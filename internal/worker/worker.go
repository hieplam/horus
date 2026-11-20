// Package worker is the handler of horus worker: the loop of the one worker
// of the job queue. It joins the services internal/jobs and
// internal/generate. It takes the deliveries of the work queue jobs.comfyui
// into its slots, and runs the plan of each job (generate.Check made it in
// the API process) with generate.Run.
//
//	slots      HORUS_WORKER_CONCURRENCY: the jobs that run at the same time, and the prefetch of Consume
//	gate       no delivery starts while ComfyUI does not answer, or while the worker has no worker lock; the queued jobs keep their place
//	consumer   jobs.Service.Handle: a run only after the move from queued to running (the idempotent consumer)
//	cancel     a cancel request of a running job ends its run (jobs.Service.Handle): the event, and the job store every 5 s
//	heartbeat  every 5 s: a check of the worker lock, then the row of this worker in horus.workers
//	lock       one worker: a second horus worker prints the first one and exits. When the session of the
//	           lock ends (a restart of horus-db), the worker takes the lock again every 2 s. When another
//	           worker took the lock in that time, this worker exits (ADR 0037)
//	sweep      every 30 s, and at each delivery: expired after the deadline, abandoned after 60 s with no waiter
//	retention  at the start and one time each day: delete the jobs that ended more than HORUS_JOB_RETENTION_DAYS ago
//
// The worker never starts ComfyUI (generate.Options.NoStart): the owner starts
// it. The worker saves backend_job_id before the wait (generate.Options.Submitted),
// and generate.Run writes the PNG as file.part, then renames it.
//
// The spec: docs/specs/2026-11-19-job-queue.md.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// The backend and the tool of the jobs that this worker runs.
const (
	Backend   = jobdata.BackendComfyUI
	ToolImage = jobdata.ToolImage
)

// The defaults of Config.
const (
	DefaultSlots       = 1
	DefaultRetention   = 30 * 24 * time.Hour
	DefaultHeartbeat   = 5 * time.Second
	DefaultSweep       = 30 * time.Second
	DefaultDeleteEvery = 24 * time.Hour
	DefaultComfyPoll   = 2 * time.Second
	DefaultLockRetry   = 2 * time.Second
)

// lockTimeout is the longest time of one check of the worker lock, and of one
// try to take the lock again.
const lockTimeout = 5 * time.Second

// Config holds the settings of one worker process. A zero field gets its
// default.
type Config struct {
	// ID is the id of this worker process. Each start gets a new id, so that a
	// job of an earlier process is a job of a worker that is gone.
	ID      string
	Host    string
	PID     int
	Version string
	// Slots is HORUS_WORKER_CONCURRENCY. Default 1.
	Slots int
	// Retention is HORUS_JOB_RETENTION_DAYS as a duration. Default 30 days.
	Retention time.Duration
	// Heartbeat is how often the worker checks its worker lock and writes its
	// row. Default 5 s.
	Heartbeat time.Duration
	// Sweep is how often the worker sweeps the job store. Default 30 s.
	Sweep time.Duration
	// DeleteEvery is how often the worker deletes the old jobs. Default 24 h.
	DeleteEvery time.Duration
	// ComfyPoll is how often the gate asks ComfyUI again while it does not
	// answer. Default 2 s.
	ComfyPoll time.Duration
	// LockRetry is how often the worker tries to take the worker lock again
	// after the session of its lock ended. Default 2 s.
	LockRetry time.Duration
	// Log gets one line for each start, each delivery, each change of the
	// gate and of the worker lock, and each failed step. nil: no log.
	Log io.Writer
}

func (c Config) withDefaults() Config {
	if c.Slots <= 0 {
		c.Slots = DefaultSlots
	}
	if c.Retention <= 0 {
		c.Retention = DefaultRetention
	}
	if c.Heartbeat <= 0 {
		c.Heartbeat = DefaultHeartbeat
	}
	if c.Sweep <= 0 {
		c.Sweep = DefaultSweep
	}
	if c.DeleteEvery <= 0 {
		c.DeleteEvery = DefaultDeleteEvery
	}
	if c.ComfyPoll <= 0 {
		c.ComfyPoll = DefaultComfyPoll
	}
	if c.LockRetry <= 0 {
		c.LockRetry = DefaultLockRetry
	}
	if c.Log == nil {
		c.Log = io.Discard
	}
	return c
}

// LockedError is the error of Run when another worker holds the worker lock:
// at the start, or when this worker takes the lock again after its session
// ended. First is the worker row with the newest heartbeat, or nil.
type LockedError struct {
	First *jobdata.Worker
}

func (e *LockedError) Error() string {
	if e.First == nil {
		return "another horus worker holds the worker lock, and it wrote no heartbeat yet"
	}
	w := e.First
	return fmt.Sprintf("another horus worker runs: id %s, host %s, pid %d, version %s, %d slots, started at %s, last heartbeat at %s",
		w.ID, w.Host, w.PID, w.Version, w.Slots, w.StartedAt.UTC().Format(time.RFC3339), w.HeartbeatAt.UTC().Format(time.RFC3339))
}

// loop is one run of the worker.
type loop struct {
	cfg  Config
	jobs *jobs.Service
	gen  *generate.Service
	row  jobdata.Worker

	// lock is the worker lock of this worker. Only the goroutine of the
	// heartbeat changes it, and Run releases it after that goroutine ends.
	lock jobdata.Lock
	// fail stops Run with its cause: a *LockedError when another worker took
	// the lock.
	fail context.CancelCauseFunc

	logMu    sync.Mutex
	down     atomic.Bool // the gate saw ComfyUI not answer, and logged it
	noLock   atomic.Bool // the session of the lock ended, and the worker does not hold the lock again yet
	lockWait atomic.Bool // the gate holds a delivery because of noLock, and logged it
}

func (l *loop) logf(format string, args ...any) {
	l.logMu.Lock()
	defer l.logMu.Unlock()
	fmt.Fprintf(l.cfg.Log, "horus worker: "+format+"\n", args...)
}

// Run is the loop of horus worker. It takes the worker lock, writes the
// heartbeat, sweeps, deletes the old jobs, and consumes the work queue
// jobs.comfyui until ctx ends. Then it returns nil. A worker lock that another
// worker holds gives a *LockedError at once. At each heartbeat, Run checks the
// lock, and takes it again when its session ended (keepLock). When another
// worker took the lock in that time, Run stops its jobs and gives a
// *LockedError.
func Run(ctx context.Context, cfg Config, j *jobs.Service, g *generate.Service) error {
	cfg = cfg.withDefaults()
	l := &loop{cfg: cfg, jobs: j, gen: g}
	lock, err := j.WorkerLock(ctx)
	if errors.Is(err, jobdata.ErrWorkerLocked) {
		return l.locked(ctx)
	}
	if err != nil {
		return err
	}
	l.lock = lock
	defer func() { l.lock.Release() }()

	l.row = jobdata.Worker{ID: cfg.ID, Host: cfg.Host, PID: cfg.PID, Version: cfg.Version, Slots: cfg.Slots, StartedAt: time.Now()}
	if err := j.Heartbeat(ctx, l.row); err != nil {
		return err
	}
	l.logf("started: id %s, %d slots, work queue %s", cfg.ID, cfg.Slots, jobs.WorkQueue(Backend))
	l.sweep(ctx)
	l.deleteOld(ctx)

	rctx, fail := context.WithCancelCause(ctx)
	defer fail(nil)
	l.fail = fail
	bctx, stop := context.WithCancel(rctx)
	var wg sync.WaitGroup
	every := func(d time.Duration, f func(context.Context)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.NewTicker(d)
			defer t.Stop()
			for {
				select {
				case <-bctx.Done():
					return
				case <-t.C:
					f(bctx)
				}
			}
		}()
	}
	every(cfg.Heartbeat, l.heartbeat)
	every(cfg.Sweep, l.sweep)
	every(cfg.DeleteEvery, l.deleteOld)

	err = j.Consume(rctx, jobs.Consumer{
		WorkerID: cfg.ID,
		Queue:    jobs.WorkQueue(Backend),
		Slots:    cfg.Slots,
		Run:      l.run,
		Ready:    l.ready,
		Done:     l.done,
	})
	stop()
	wg.Wait()
	var le *LockedError
	if cause := context.Cause(rctx); errors.As(cause, &le) {
		l.logf("stopped: id %s: %v", cfg.ID, cause)
		return cause
	}
	if ctx.Err() != nil {
		l.logf("stopped: id %s", cfg.ID)
		return nil
	}
	return err
}

// locked gives the *LockedError of a worker lock that another worker holds,
// with the worker row of the newest heartbeat. The row of this worker is not
// the other worker: then First is nil.
func (l *loop) locked(ctx context.Context) error {
	sum, err := l.jobs.Summary(ctx)
	if err != nil {
		return errors.Join(&LockedError{}, err)
	}
	if sum.Worker != nil && sum.Worker.ID == l.cfg.ID {
		return &LockedError{}
	}
	return &LockedError{First: sum.Worker}
}

// heartbeat checks the worker lock, then writes the row of this worker with
// the time now. A worker without the lock writes no row.
func (l *loop) heartbeat(ctx context.Context) {
	if !l.keepLock(ctx) {
		return
	}
	if err := l.jobs.Heartbeat(ctx, l.row); err != nil && ctx.Err() == nil {
		l.logf("the heartbeat failed: %v", err)
	}
}

// keepLock gives true while this worker holds the worker lock. When the
// session of the lock ended (a restart of horus-db), the gate takes no new
// delivery, and keepLock takes the lock again on a new session: at once, then
// every Config.LockRetry. When another worker holds the lock, keepLock stops
// Run with a *LockedError. It gives false when ctx ends or Run stops.
func (l *loop) keepLock(ctx context.Context) bool {
	cctx, cancel := context.WithTimeout(ctx, lockTimeout)
	err := l.lock.Check(cctx)
	cancel()
	if err == nil {
		return true
	}
	if ctx.Err() != nil {
		return false
	}
	lost := time.Now()
	l.noLock.Store(true)
	l.logf("the worker lock is lost: %v: the worker takes no delivery until it holds the lock again", err)
	l.lock.Release()
	logged := false
	for {
		tctx, cancel := context.WithTimeout(ctx, lockTimeout)
		lock, err := l.jobs.WorkerLock(tctx)
		cancel()
		switch {
		case err == nil:
			l.lock = lock
			l.noLock.Store(false)
			l.lockWait.Store(false)
			l.logf("the worker holds the worker lock again, %s after the lock was lost", time.Since(lost).Round(time.Millisecond))
			return true
		case errors.Is(err, jobdata.ErrWorkerLocked):
			l.fail(l.locked(ctx))
			return false
		case ctx.Err() != nil:
			return false
		case !logged:
			l.logf("the worker cannot take the worker lock again: %v: it tries again every %s", err, l.cfg.LockRetry)
			logged = true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(l.cfg.LockRetry):
		}
	}
}

// sweep moves the queued jobs after their deadline and the queued jobs with
// no waiter to their end state.
func (l *loop) sweep(ctx context.Context) {
	expired, abandoned, err := l.jobs.Sweep(ctx)
	switch {
	case err != nil && ctx.Err() == nil:
		l.logf("the sweep failed: %v", err)
	case expired+abandoned > 0:
		l.logf("the sweep: %d expired, %d abandoned", expired, abandoned)
	}
}

// deleteOld deletes the jobs that ended more than Config.Retention ago.
func (l *loop) deleteOld(ctx context.Context) {
	n, err := l.jobs.DeleteOld(ctx, l.cfg.Retention)
	switch {
	case err != nil && ctx.Err() == nil:
		l.logf("the deletion of old jobs failed: %v", err)
	case n > 0:
		l.logf("deleted %d jobs that ended more than %s ago", n, l.cfg.Retention)
	}
}

// ready is the gate: it returns when this worker holds the worker lock and
// ComfyUI answers, or with the error of ctx. While the worker has no lock or
// ComfyUI does not answer, no delivery starts, so each queued job keeps its
// place.
func (l *loop) ready(ctx context.Context) error {
	for {
		if l.noLock.Load() {
			if l.lockWait.CompareAndSwap(false, true) {
				l.logf("a delivery waits until the worker holds the worker lock again")
			}
		} else {
			up, err := l.gen.Running(ctx)
			if err == nil && up {
				if l.down.CompareAndSwap(true, false) {
					l.logf("ComfyUI answers again: the worker takes deliveries")
				}
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if l.down.CompareAndSwap(false, true) {
				l.logf("ComfyUI does not answer: the worker takes no delivery until it answers")
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(l.cfg.ComfyPoll):
		}
	}
}

// run runs one job: the plan of its payload, on ComfyUI.
func (l *loop) run(ctx context.Context, j jobdata.Job, progress func(jobdata.Progress)) (json.RawMessage, error) {
	if j.Tool != ToolImage {
		return nil, herr.New("INTERNAL", "the worker has no run for the tool "+j.Tool, map[string]any{"job_id": j.ID, "tool": j.Tool})
	}
	var p generate.Plan
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.Workflow == "" {
		return nil, herr.New("INTERNAL", "the payload of the job is not a plan of generate_image", map[string]any{"job_id": j.ID})
	}
	res, err := l.gen.Run(ctx, p, generate.Options{
		NoStart: true,
		Report: func(_ context.Context, value, total float64, message string) {
			progress(jobdata.Progress{Value: value, Total: total, Message: message})
		},
		Submitted: func(ctx context.Context, backendJobID string) {
			if err := l.jobs.Submitted(ctx, j.ID, backendJobID); err != nil && ctx.Err() == nil {
				l.logf("the job %s: save backend_job_id %s: %v", j.ID, backendJobID, err)
			}
		},
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(res)
}

// done logs the outcome of one delivery.
func (l *loop) done(m jobdata.Message, o jobs.Outcome, err error) {
	if err != nil {
		l.logf("the job %s: %s: %v", m.JobID, o, err)
		return
	}
	l.logf("the job %s: %s", m.JobID, o)
}
