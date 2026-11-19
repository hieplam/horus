// Tests of the worker, with the in-memory broker and job store of
// internal/brokertest, the real services internal/jobs and internal/generate,
// and the fake ComfyUI of fake_test.go.
package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/brokertest"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/worker"
)

const subject = "On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. " +
	"On the right, a sky-blue server box takes them off the belt."

// counting is the in-memory broker that counts the deliveries and the acks,
// and keeps the prefetch of each Consume. With twice, Publish adds each
// message two times, so the broker delivers each message two times
// (at-least-once).
type counting struct {
	*brokertest.Broker
	twice      bool
	deliveries atomic.Int32
	acks       atomic.Int32

	mu       sync.Mutex
	prefetch []int
}

func (b *counting) Publish(ctx context.Context, queue string, m jobdata.Message) error {
	if err := b.Broker.Publish(ctx, queue, m); err != nil || !b.twice {
		return err
	}
	return b.Broker.Publish(ctx, queue, m)
}

func (b *counting) Consume(ctx context.Context, queue string, prefetch int) (<-chan jobdata.Delivery, error) {
	b.mu.Lock()
	b.prefetch = append(b.prefetch, prefetch)
	b.mu.Unlock()
	in, err := b.Broker.Consume(ctx, queue, prefetch)
	if err != nil {
		return nil, err
	}
	out := make(chan jobdata.Delivery)
	go func() {
		defer close(out)
		for d := range in {
			b.deliveries.Add(1)
			ack := d.Ack
			d.Ack = func(ctx context.Context) error {
				err := ack(ctx)
				if err == nil {
					b.acks.Add(1)
				}
				return err
			}
			select {
			case out <- d:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (b *counting) prefetches() []int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]int(nil), b.prefetch...)
}

// clock is a clock that a test moves.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }

func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// syncBuffer is the log of a worker.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type env struct {
	fake   *fakeComfy
	store  *brokertest.Store
	broker *counting
	jobs   *jobs.Service
	gen    *generate.Service
	clock  *clock // nil: the store and the service use time.Now
}

type options struct {
	twice bool // the broker delivers each message two times
	clock bool // the store and the service use a clock that the test moves
}

// setup makes the services on the in-memory broker and store, with the
// workflows of this repo, a temporary output folder and short wait intervals
// of the generate service.
func setup(t *testing.T, o options) *env {
	t.Helper()
	tmp := t.TempDir()
	wf, err := filepath.Abs(filepath.Join("..", "..", "workflows"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HORUS_ROOT", tmp)
	t.Setenv("HORUS_WORKFLOWS_DIR", wf)
	t.Setenv("HORUS_OUTPUT_DIR", filepath.Join(tmp, "out"))
	t.Setenv("HORUS_AUTOSTART", "0")
	settings.Reset()
	t.Cleanup(settings.Reset)
	oldPoll, oldReport, oldAbsent := generate.PollInterval, generate.ReportInterval, generate.AbsentLimit
	generate.PollInterval, generate.ReportInterval, generate.AbsentLimit = 2*time.Millisecond, 20*time.Millisecond, time.Second
	t.Cleanup(func() {
		generate.PollInterval, generate.ReportInterval, generate.AbsentLimit = oldPoll, oldReport, oldAbsent
	})

	e := &env{fake: newFake(t), store: brokertest.NewStore(), broker: &counting{Broker: brokertest.NewBroker(), twice: o.twice}}
	cfg := jobs.Config{Poll: 10 * time.Millisecond}
	if o.clock {
		e.clock = &clock{t: time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)}
		e.store.Now, cfg.Now = e.clock.Now, e.clock.Now
	}
	e.jobs = jobs.New(e.store, e.broker, cfg)
	e.gen = generate.New(e.fake, noStart{t})
	return e
}

// noStart is the backend of the worker tests: the worker never starts ComfyUI.
type noStart struct{ t *testing.T }

func (b noStart) Start(context.Context, io.Writer) error {
	b.t.Error("the worker started ComfyUI")
	return errors.New("the worker never starts ComfyUI")
}

// config is a worker with short intervals.
func (e *env) config(id string, slots int, log *syncBuffer) worker.Config {
	c := worker.Config{ID: id, Host: "test-host", PID: 42, Version: "test", Slots: slots,
		Heartbeat: 20 * time.Millisecond, Sweep: 20 * time.Millisecond, DeleteEvery: 20 * time.Millisecond, ComfyPoll: 5 * time.Millisecond}
	if log != nil {
		c.Log = log
	}
	return c
}

// start runs a worker until the test calls stop or ends. stop gives the
// error of worker.Run.
func (e *env) start(t *testing.T, c worker.Config) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx, c, e.jobs, e.gen) }()
	var once sync.Once
	var err error
	stop = func() error {
		once.Do(func() {
			cancel()
			select {
			case err = <-done:
			case <-time.After(10 * time.Second):
				err = errors.New("the worker did not stop in 10 s")
			}
		})
		return err
	}
	t.Cleanup(func() { _ = stop() })
	return stop
}

// submit checks a request with the seed i, as an API process does, and
// submits its job.
func (e *env) submit(t *testing.T, i int) (jobdata.Job, generate.Plan) {
	t.Helper()
	p, err := e.gen.Check(generate.Request{Workflow: "it-illustration", Subject: subject, Seed: int64(i)})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := e.jobs.Submit(context.Background(), jobdata.Job{Tool: worker.ToolImage, Workflow: p.Workflow,
		Backend: worker.Backend, Payload: raw, Caller: "test"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	return j, p
}

func (e *env) get(t *testing.T, id string) jobdata.Job {
	t.Helper()
	j, err := e.store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	return j
}

func (e *env) state(t *testing.T, id string) jobdata.State { return e.get(t, id).State }

func (e *env) summaryWorker(t *testing.T) *jobdata.Worker {
	t.Helper()
	sum, err := e.jobs.Summary(context.Background())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	return sum.Worker
}

// waitFor waits until cond is true, for at most 10 s.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not in 10 s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func count(list []string, s string) int {
	n := 0
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

// TestDuplicateDelivery is the oracle of the goal row duplicate-delivery. The
// in-memory broker delivers each of 6 messages two times, and the fake
// ComfyUI counts one submit for each job. The worker has 2 slots, so a second
// delivery comes while its job runs on this worker, or after the job ended.
// The goal suite reads the line "duplicate-delivery: ...".
func TestDuplicateDelivery(t *testing.T) {
	e := setup(t, options{twice: true})
	const n = 6
	var ids []string
	var plans []generate.Plan
	for i := 1; i <= n; i++ {
		j, p := e.submit(t, i)
		ids, plans = append(ids, j.ID), append(plans, p)
	}
	stop := e.start(t, e.config("worker-1", 2, nil))
	waitFor(t, "an ack of each of the 12 deliveries", func() bool { return e.broker.acks.Load() == 2*n })
	if err := stop(); err != nil {
		t.Fatalf("the worker: %v", err)
	}

	submits := e.fake.submits()
	perSeed := map[uint64]int{}
	for _, id := range submits {
		perSeed[e.fake.seed(id)]++
	}
	once := 0
	for i, id := range ids {
		seed := uint64(i + 1)
		j := e.get(t, id)
		events := e.store.Events(id)
		var res generate.Result
		_ = json.Unmarshal(j.Result, &res)
		switch {
		case perSeed[seed] != 1:
			t.Errorf("the job %d: %d submits, want 1", i+1, perSeed[seed])
		case j.State != jobdata.Succeeded || j.Attempt != 1 || j.WorkerID != "worker-1":
			t.Errorf("the job %d: %s with the attempt %d on %q, want succeeded with the attempt 1 on worker-1", i+1, j.State, j.Attempt, j.WorkerID)
		case j.BackendJobID == "" || e.fake.seed(j.BackendJobID) != seed:
			t.Errorf("the job %d: backend_job_id %q is not its job in ComfyUI", i+1, j.BackendJobID)
		case count(events, "job.running") != 1 || count(events, "job.submitted") != 1 || count(events, "job.succeeded") != 1:
			t.Errorf("the job %d: events %v, want one job.running, one job.submitted and one job.succeeded", i+1, events)
		case res.Path != plans[i].Path:
			t.Errorf("the job %d: result path %q, want %q", i+1, res.Path, plans[i].Path)
		default:
			once++
		}
		if _, err := os.Stat(plans[i].Path); err != nil {
			t.Errorf("the job %d: no PNG at %s: %v", i+1, plans[i].Path, err)
		}
		if _, err := os.Stat(plans[i].Path + ".part"); !os.IsNotExist(err) {
			t.Errorf("the job %d: the file .part is left", i+1)
		}
	}
	deliveries := e.broker.deliveries.Load()
	t.Logf("duplicate-delivery: %d messages delivered two times: %d deliveries, %d submits for %d jobs, %d jobs with exactly one submit",
		n, deliveries, len(submits), n, once)
	if deliveries != 2*n {
		t.Errorf("%d deliveries, want %d", deliveries, 2*n)
	}
	if len(submits) != n || once != n {
		t.Errorf("%d submits for %d jobs and %d jobs with exactly one submit, want %d and %d", len(submits), n, once, n, n)
	}
}

// TestConcurrencySetting is the unit level of the goal row
// concurrency-setting: with N slots, the worker holds at most N jobs in
// ComfyUI at the same time, and it reaches N. The prefetch of Consume is N.
// Each job holds its slot until the test releases it.
func TestConcurrencySetting(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("slots_%d", n), func(t *testing.T) {
			e := setup(t, options{})
			e.fake.setMode(modeHold)
			const total = 6
			var ids []string
			for i := 1; i <= total; i++ {
				j, _ := e.submit(t, i)
				ids = append(ids, j.ID)
			}
			e.start(t, e.config("worker-1", n, nil))
			for left := total; left > 0; {
				want := min(n, left)
				waitFor(t, fmt.Sprintf("%d jobs in ComfyUI", want), func() bool { return len(e.fake.held()) >= want })
				// A worker that takes more jobs than its slots shows it in this time.
				time.Sleep(100 * time.Millisecond)
				if a := e.fake.Active(); a > n {
					t.Fatalf("ComfyUI holds %d jobs, and the worker has %d slots", a, n)
				}
				left -= len(e.fake.held())
				e.fake.release()
			}
			waitFor(t, "the 6 jobs succeeded", func() bool {
				for _, id := range ids {
					if e.state(t, id) != jobdata.Succeeded {
						return false
					}
				}
				return true
			})
			most := e.fake.mostActive()
			t.Logf("concurrency-setting: HORUS_WORKER_CONCURRENCY=%d: most jobs in ComfyUI at one time %d, %d submits for %d jobs",
				n, most, len(e.fake.submits()), total)
			if most != n {
				t.Errorf("most jobs in ComfyUI at one time: %d, want %d", most, n)
			}
			if p := e.broker.prefetches(); !slices.Equal(p, []int{n}) {
				t.Errorf("the prefetch of Consume: %v, want [%d]", p, n)
			}
			if s := len(e.fake.submits()); s != total {
				t.Errorf("%d submits, want %d", s, total)
			}
		})
	}
}

// TestWorkerLost is the unit level of the goal row worker-lost. The worker A
// saves backend_job_id before it waits. A second delivery of the job to A
// while the job runs on A does not touch the job. Then A stops while the job
// runs: the delivery gets no ack. The next worker B gets the delivery again,
// ends the job with WORKER_LOST without a run, and runs the next job.
func TestWorkerLost(t *testing.T) {
	e := setup(t, options{})
	e.fake.setMode(modeHold)
	lost, _ := e.submit(t, 1)
	stopA := e.start(t, e.config("worker-a", 2, nil))
	waitFor(t, "the job runs in ComfyUI", func() bool { return len(e.fake.held()) == 1 })
	waitFor(t, "backend_job_id is saved before the end of the job", func() bool { return e.get(t, lost.ID).BackendJobID == "job-1" })
	if j := e.get(t, lost.ID); j.State != jobdata.Running || j.WorkerID != "worker-a" {
		t.Fatalf("the job: %s on %q, want running on worker-a", j.State, j.WorkerID)
	}

	m := jobdata.Message{V: jobdata.MessageVersion, JobID: lost.ID, Tool: lost.Tool, Backend: lost.Backend}
	if err := e.broker.Broker.Publish(context.Background(), lost.WorkQueue, m); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the ack of the second delivery", func() bool { return e.broker.acks.Load() == 1 })
	if j := e.get(t, lost.ID); j.State != jobdata.Running || j.WorkerID != "worker-a" || len(e.fake.submits()) != 1 {
		t.Fatalf("after a second delivery to the same worker: %s on %q with %d submits, want running on worker-a with 1 submit",
			j.State, j.WorkerID, len(e.fake.submits()))
	}

	if err := stopA(); err != nil {
		t.Fatalf("the worker A: %v", err)
	}
	if j := e.get(t, lost.ID); j.State != jobdata.Running {
		t.Fatalf("the job after the worker A stopped: %s, want running", j.State)
	}
	e.fake.setMode(modeOK)
	next, plan := e.submit(t, 2)
	e.start(t, e.config("worker-b", 1, nil))
	waitFor(t, "the next job ends", func() bool { return e.state(t, next.ID).Ended() })

	j := e.get(t, lost.ID)
	code := ""
	if j.Error != nil {
		code = j.Error.Code
	}
	t.Logf("worker-lost: the job of the stopped worker: %s with the error %s; the next job: %s", j.State, code, e.state(t, next.ID))
	if j.State != jobdata.Failed || j.Error == nil || j.Error.Code != "WORKER_LOST" || j.Error.Details["worker_id"] != "worker-a" {
		t.Errorf("the job of the stopped worker: %s with the error %+v, want failed with WORKER_LOST of worker-a", j.State, j.Error)
	}
	if s := e.state(t, next.ID); s != jobdata.Succeeded {
		t.Errorf("the next job: %s, want succeeded", s)
	}
	if _, err := os.Stat(plan.Path); err != nil {
		t.Errorf("the next job: no PNG: %v", err)
	}
	if s := e.fake.submits(); len(s) != 2 {
		t.Errorf("%d submits, want 2: the worker B ran the lost job again", len(s))
	}
}

// TestNoDeliveryWhileComfyUIDown: while ComfyUI does not answer, the worker
// starts no delivery. The queued jobs keep their place and reach no submit.
// When ComfyUI answers again, the jobs run.
func TestNoDeliveryWhileComfyUIDown(t *testing.T) {
	e := setup(t, options{})
	var log syncBuffer
	e.fake.setDown(true)
	a, _ := e.submit(t, 1)
	b, _ := e.submit(t, 2)
	e.start(t, e.config("worker-1", 2, &log))
	// The gate asks ComfyUI every 5 ms. A worker without the gate runs the
	// jobs in this time.
	time.Sleep(200 * time.Millisecond)
	for i, id := range []string{a.ID, b.ID} {
		st, err := e.jobs.Progress(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if st.State != jobdata.Queued || st.Position != i+1 || st.Total != 2 {
			j := e.get(t, id)
			t.Errorf("the job %d while ComfyUI is down: %s at %d of %d (error %+v), want queued at %d of 2",
				i+1, st.State, st.Position, st.Total, j.Error, i+1)
		}
	}
	if s, r := len(e.fake.submits()), e.fake.refusedSubmits(); s+r != 0 {
		t.Errorf("while ComfyUI is down: %d submits and %d refused submits, want 0", s, r)
	}
	if !strings.Contains(log.String(), "ComfyUI does not answer: the worker takes no delivery") {
		t.Errorf("the log has no line of the gate:\n%s", log.String())
	}
	e.fake.setDown(false)
	waitFor(t, "the 2 jobs succeeded", func() bool {
		return e.state(t, a.ID) == jobdata.Succeeded && e.state(t, b.ID) == jobdata.Succeeded
	})
	if s := len(e.fake.submits()); s != 2 {
		t.Errorf("%d submits, want 2", s)
	}
	if !strings.Contains(log.String(), "ComfyUI answers again") {
		t.Errorf("the log has no line of the gate when ComfyUI answers again:\n%s", log.String())
	}
}

// TestOneWorker: the worker writes its row in horus.workers and a newer
// heartbeat every Config.Heartbeat. A second worker finds the worker lock
// taken, names the first worker and returns. After the first worker stops, a
// new worker takes the lock.
func TestOneWorker(t *testing.T) {
	e := setup(t, options{})
	stop1 := e.start(t, e.config("worker-1", 3, nil))
	waitFor(t, "the row of worker-1", func() bool { w := e.summaryWorker(t); return w != nil && w.ID == "worker-1" })
	first := *e.summaryWorker(t)
	if first.Host != "test-host" || first.PID != 42 || first.Version != "test" || first.Slots != 3 || first.StartedAt.IsZero() {
		t.Errorf("the row of worker-1: %+v", first)
	}
	waitFor(t, "a newer heartbeat", func() bool { return e.summaryWorker(t).HeartbeatAt.After(first.HeartbeatAt) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := worker.Run(ctx, e.config("worker-2", 1, nil), e.jobs, e.gen)
	var le *worker.LockedError
	if !errors.As(err, &le) || le.First == nil || le.First.ID != "worker-1" ||
		!strings.Contains(err.Error(), "another horus worker runs: id worker-1, host test-host, pid 42") {
		t.Fatalf("a second worker: %v, want a *LockedError that names worker-1", err)
	}
	if ctx.Err() != nil {
		t.Fatal("the second worker waited for the lock")
	}

	if err := stop1(); err != nil {
		t.Fatalf("the first worker: %v", err)
	}
	stop3 := e.start(t, e.config("worker-3", 1, nil))
	waitFor(t, "the row of worker-3", func() bool { w := e.summaryWorker(t); return w != nil && w.ID == "worker-3" })
	if err := stop3(); err != nil {
		t.Fatalf("the third worker: %v", err)
	}
}

// TestSweep: the sweep of the worker moves a queued job with no waiter for
// more than 60 s to abandoned, and a queued job after its deadline to
// expired. ComfyUI is down, so no delivery starts and only the sweep of the
// worker moves the jobs. When ComfyUI answers again, neither job reaches it.
func TestSweep(t *testing.T) {
	e := setup(t, options{clock: true})
	e.fake.setDown(true)
	abandoned, _ := e.submit(t, 1)
	e.start(t, e.config("worker-1", 2, nil))
	e.clock.Add(jobdata.AbandonAfter + time.Second)
	waitFor(t, "the job with no waiter is abandoned", func() bool { return e.state(t, abandoned.ID) == jobdata.Abandoned })

	expired, _ := e.submit(t, 2)
	e.clock.Add(jobs.DefaultQueueTimeout + time.Second)
	if err := e.jobs.Touch(context.Background(), expired.ID); err != nil { // a waiter is there: only the deadline passed
		t.Fatal(err)
	}
	waitFor(t, "the job after its deadline is expired", func() bool { return e.state(t, expired.ID) == jobdata.Expired })

	e.fake.setDown(false)
	waitFor(t, "the ack of the 2 deliveries", func() bool { return e.broker.acks.Load() == 2 })
	if s := len(e.fake.submits()); s != 0 {
		t.Errorf("%d submits of an abandoned or expired job, want 0", s)
	}
}

// TestDeleteOld: the worker deletes the jobs that ended more than
// Config.Retention ago at its start and every Config.DeleteEvery, and keeps
// the newer jobs.
func TestDeleteOld(t *testing.T) {
	e := setup(t, options{clock: true})
	ctx := context.Background()
	retention := 30 * 24 * time.Hour
	ended := func(n int, age time.Duration) jobdata.Job {
		at := e.clock.Now().Add(-age)
		return jobdata.Job{ID: fmt.Sprintf("019aa000-0000-7000-8000-%012d", n), Tool: worker.ToolImage, Backend: worker.Backend,
			State: jobdata.Succeeded, CreatedAt: at.Add(-time.Minute), StartedAt: at.Add(-30 * time.Second), FinishedAt: at, Attempt: 1}
	}
	old, young := ended(1, retention+time.Hour), ended(2, retention-time.Hour)
	for _, j := range []jobdata.Job{old, young} {
		if err := e.store.Insert(ctx, j); err != nil {
			t.Fatal(err)
		}
	}
	c := e.config("worker-1", 1, nil)
	c.Retention = retention
	e.start(t, c)
	gone := func(id string) func() bool {
		return func() bool { _, err := e.store.Get(ctx, id); return errors.Is(err, jobdata.ErrNotFound) }
	}
	waitFor(t, "the old job is deleted at the start", gone(old.ID))
	later := ended(3, retention+time.Hour)
	if err := e.store.Insert(ctx, later); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "an old job is deleted by the next run", gone(later.ID))
	if s := e.state(t, young.ID); s != jobdata.Succeeded {
		t.Errorf("the job that ended %s ago: %s, want kept", retention-time.Hour, s)
	}
}
