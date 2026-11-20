package jobs

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
)

// QueueFullError is the error of Submit when the rule of QUEUE_FULL refuses
// the call. The handler gives it to the caller as QUEUE_FULL, with
// RetryAfter as details.retry_after_seconds.
type QueueFullError struct {
	QueueCheck
	Queued, Max     int
	Start, Deadline time.Time
}

func (e *QueueFullError) Error() string {
	if e.Reason == FullCount {
		return fmt.Sprintf("the queue holds %d jobs, the most is %d", e.Queued, e.Max)
	}
	return fmt.Sprintf("the estimated start %s is after the deadline %s",
		e.Start.UTC().Format(time.RFC3339), e.Deadline.UTC().Format(time.RFC3339))
}

// Submit makes a job from j and puts it in the queue. The caller fills the
// fields of the request: Tool, Workflow, Backend, ResourceKey, Payload,
// RequestHash, IdempotencyKey and Caller. Submit fills ID, WorkQueue, State,
// CreatedAt, Deadline and WaiterSeenAt, inserts the job and publishes one
// message to its work queue. A full queue gives a *QueueFullError, and no job.
func (s *Service) Submit(ctx context.Context, j jobdata.Job) (jobdata.Job, error) {
	now := s.cfg.Now()
	queued, err := s.store.Queued(ctx)
	if err != nil {
		return jobdata.Job{}, err
	}
	sum, err := s.store.Summary(ctx)
	if err != nil {
		return jobdata.Job{}, err
	}
	run := meanRun(sum, s.cfg.DefaultRun)
	start := EstimateStart(now, queued+1, s.cfg.Slots, runningStarts(sum), run)
	deadline := now.Add(s.cfg.QueueTimeout)
	if c := CheckQueue(queued, s.cfg.QueueMax, start, deadline, run, s.cfg.Slots); c.Full {
		return jobdata.Job{}, &QueueFullError{QueueCheck: c, Queued: queued, Max: s.cfg.QueueMax, Start: start, Deadline: deadline}
	}
	id, err := NewID(now, rand.Reader)
	if err != nil {
		return jobdata.Job{}, err
	}
	j.ID = id
	j.WorkQueue = WorkQueue(j.Backend)
	j.State = jobdata.Queued
	j.Attempt = 0
	j.CreatedAt = now
	j.Deadline = deadline
	j.WaiterSeenAt = now
	if err := s.store.Insert(ctx, j); err != nil {
		return jobdata.Job{}, err
	}
	m := jobdata.Message{V: jobdata.MessageVersion, JobID: j.ID, Tool: j.Tool, Backend: j.Backend}
	if err := s.broker.Publish(ctx, j.WorkQueue, m); err != nil {
		// No message: no worker can run the job. End it now.
		_, _ = s.move(ctx, j.ID, []jobdata.State{jobdata.Queued}, jobdata.Failed, jobdata.Change{At: s.cfg.Now(),
			Error: &jobdata.JobError{Code: "INTERNAL", Message: "the broker refused the message: " + err.Error()}})
		return jobdata.Job{}, err
	}
	s.notify(ctx, j.ID, jobdata.EventType(jobdata.Queued), now)
	return j, nil
}

// Status is what a waiter tells about a job now.
type Status struct {
	State jobdata.State
	// Position and Total: the place of a queued job and the count of queued
	// jobs. 0 for a job that is not queued.
	Position, Total int
	// Start is the estimated start of a queued job.
	Start time.Time
	// Message is the progress text.
	Message string
}

// Progress tells the state of the job now, with its place in the queue and
// its estimated start when it is queued.
func (s *Service) Progress(ctx context.Context, id string) (Status, error) {
	j, err := s.store.Get(ctx, id)
	if err != nil {
		return Status{}, err
	}
	return s.status(ctx, j, s.cfg.Now())
}

func (s *Service) status(ctx context.Context, j jobdata.Job, now time.Time) (Status, error) {
	st := Status{State: j.State}
	switch {
	case j.State == jobdata.Queued:
		pos, total, err := s.store.Position(ctx, j.ID)
		if err != nil {
			return Status{}, err
		}
		if pos == 0 {
			st.Message = "starting"
			return st, nil
		}
		sum, err := s.store.Summary(ctx)
		if err != nil {
			return Status{}, err
		}
		st.Position, st.Total = pos, total
		st.Start = EstimateStart(now, pos, s.cfg.Slots, runningStarts(sum), meanRun(sum, s.cfg.DefaultRun))
		st.Message = QueueText(pos, total, st.Start.Sub(now))
	case j.State == jobdata.Running:
		st.Message = "running"
		if j.Progress != nil && j.Progress.Message != "" {
			st.Message = j.Progress.Message
		}
	default:
		st.Message = "the job ended: " + string(j.State)
	}
	return st, nil
}

// Touch writes waiter_seen_at of the job: a waiter still waits for it.
func (s *Service) Touch(ctx context.Context, id string) error { return s.store.Touch(ctx, id) }

// Wait waits until the job ends and gives the job. It wakes on each event of
// the job and reads the job store every Config.Poll, so a lost event delays
// it by at most Config.Poll. Every Config.Touch it writes waiter_seen_at and
// reports the status: a timer wakes it at that time, so a report never waits
// for the next poll. The progress value of a report is the seconds since the
// job was made, so that it only rises. When ctx ends, Wait gives ctx.Err()
// and leaves the job as it is: the caller cancels it.
func (s *Service) Wait(ctx context.Context, id string, report Report) (jobdata.Job, error) {
	if report == nil {
		report = func(context.Context, float64, float64, string) {}
	}
	lctx, cancel := context.WithCancel(ctx)
	defer cancel()
	events, err := s.broker.Listen(lctx)
	if err != nil {
		return jobdata.Job{}, err
	}
	poll := time.NewTicker(s.cfg.Poll)
	defer poll.Stop()
	due := time.NewTimer(s.cfg.Touch) // the time of the next report
	defer due.Stop()
	var touched time.Time
	for {
		j, err := s.store.Get(ctx, id)
		if err != nil {
			return jobdata.Job{}, err
		}
		if j.State.Ended() {
			return j, nil
		}
		if now := s.cfg.Now(); touched.IsZero() || now.Sub(touched) >= s.cfg.Touch {
			if err := s.store.Touch(ctx, id); err != nil {
				return jobdata.Job{}, err
			}
			st, err := s.status(ctx, j, now)
			if err != nil {
				return jobdata.Job{}, err
			}
			report(ctx, math.Round(now.Sub(j.CreatedAt).Seconds()), 0, st.Message)
			touched = now
			due.Reset(s.cfg.Touch)
		}
		if err := waitWake(ctx, id, events, poll.C, due.C); err != nil {
			return jobdata.Job{}, err
		}
	}
}

// waitWake returns at the next event of the job id, the next tick or the
// time due, or with ctx.Err() when ctx ends. After the event channel closes,
// only the tick and due wake it. A nil due never wakes it.
func waitWake(ctx context.Context, id string, events <-chan jobdata.Event, tick, due <-chan time.Time) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick:
			return nil
		case <-due:
			return nil
		case e, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if e.JobID == id {
				return nil
			}
		}
	}
}

// Cancel cancels the job. A queued job moves to cancelled, and the worker
// never runs it. A running job gets cancel_requested_at: the worker stops it
// and moves it to cancelled. Cancel gives the state after the cancel. A job
// that ended keeps its state.
func (s *Service) Cancel(ctx context.Context, id string) (jobdata.State, error) {
	now := s.cfg.Now()
	ok, err := s.move(ctx, id, []jobdata.State{jobdata.Queued}, jobdata.Cancelled, jobdata.Change{At: now})
	if err != nil {
		return "", err
	}
	if ok {
		return jobdata.Cancelled, nil
	}
	j, err := s.store.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if j.State == jobdata.Running && j.CancelRequested.IsZero() {
		if _, err := s.move(ctx, id, []jobdata.State{jobdata.Running}, jobdata.Running,
			jobdata.Change{At: now, CancelRequested: true, Event: jobdata.EventCancelRequested}); err != nil {
			return "", err
		}
	}
	return j.State, nil
}

// move moves the job in the job store, and notifies the event when the job
// moved. A move that the rules do not allow is a bug of the service.
func (s *Service) move(ctx context.Context, id string, from []jobdata.State, to jobdata.State, c jobdata.Change) (bool, error) {
	for _, f := range from {
		if !CanMove(f, to) {
			return false, fmt.Errorf("jobs: the move from %s to %s is not allowed", f, to)
		}
	}
	ok, err := s.store.Move(ctx, id, from, to, c)
	if err != nil || !ok {
		return ok, err
	}
	typ := c.Event
	if typ == "" {
		typ = jobdata.EventType(to)
	}
	s.notify(ctx, id, typ, c.At)
	return true, nil
}

// notify sends the event of a change. An event is only a signal: the job
// store has the facts, and each waiter reads it every Config.Poll. So a
// failed notify is not an error. The service does not know the id of the
// event row, so Seq is 0.
func (s *Service) notify(ctx context.Context, id, typ string, at time.Time) {
	_ = s.broker.Notify(ctx, jobdata.Event{V: jobdata.MessageVersion, JobID: id, Type: typ, At: at})
}
