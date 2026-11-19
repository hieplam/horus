package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/jobdata"
)

// CodeWorkerLost is the error code of a job whose worker stopped while it ran
// the job.
const CodeWorkerLost = "WORKER_LOST"

// RunFunc runs one job on its backend: the worker gives it. progress saves a
// progress report of the job and wakes its waiter. RunFunc gives the result of
// the job as JSON, or the error of the job. A *herr.Error keeps its code in
// the job row.
type RunFunc func(ctx context.Context, j jobdata.Job, progress func(p jobdata.Progress)) (json.RawMessage, error)

// Consumer is one consumer of a work queue.
type Consumer struct {
	// WorkerID is the id of this worker process. Each start of the worker
	// gets a new id.
	WorkerID string
	Queue    string
	// Slots is the count of jobs that run at the same time: the prefetch.
	Slots int
	Run   RunFunc
	// Done, when it is not nil, gets the outcome of each delivery.
	Done func(m jobdata.Message, o Outcome, err error)
}

// Outcome is what Handle did with one delivery.
type Outcome string

// The outcomes of a delivery.
const (
	// Ran: the job moved from queued to running and ran. It ended in
	// succeeded, failed or cancelled. The delivery got its ack.
	Ran Outcome = "ran"
	// Skipped: the job was not queued, for example a second delivery of a job
	// that runs or ended, or the store has no such job. Ack and nothing more:
	// the idempotent consumer.
	Skipped Outcome = "skipped"
	// Lost: the job was running on a worker that stopped before its ack. The
	// job moved to failed with WORKER_LOST. Ack.
	Lost Outcome = "worker_lost"
	// Requeued: the job store failed before the run. Nack with requeue.
	Requeued Outcome = "requeued"
	// Stopped: ctx ended while the job ran. No ack: the message comes again,
	// and the next worker ends the job with WORKER_LOST.
	Stopped Outcome = "stopped"
)

// Consume takes the deliveries of the work queue c.Queue, at most c.Slots
// without an ack, and handles each one in its own goroutine. It returns when
// ctx ends, after each handler returns.
func (s *Service) Consume(ctx context.Context, c Consumer) error {
	deliveries, err := s.broker.Consume(ctx, c.Queue, c.Slots)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for d := range deliveries {
		wg.Add(1)
		go func(d jobdata.Delivery) {
			defer wg.Done()
			o, err := s.Handle(ctx, c, d)
			if c.Done != nil {
				c.Done(d.Message, o, err)
			}
		}(d)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("jobs: the broker closed the deliveries of " + c.Queue)
}

// Handle handles one delivery as an idempotent consumer:
//
//  1. Sweep the job store, so that an expired or abandoned job never starts.
//  2. Move the job from queued to running. Only then run it.
//  3. The job was not queued: when it is running on another worker, that
//     worker stopped before its ack, so the job fails with WORKER_LOST.
//     Else do nothing. Ack.
//  4. Run the job, save its progress, and move it to succeeded, failed or
//     cancelled (a cancel request and an error). Ack.
func (s *Service) Handle(ctx context.Context, c Consumer, d jobdata.Delivery) (Outcome, error) {
	id := d.Message.JobID
	if _, _, err := s.store.Sweep(ctx); err != nil {
		return requeue(ctx, d, err)
	}
	now := s.cfg.Now()
	moved, err := s.move(ctx, id, []jobdata.State{jobdata.Queued}, jobdata.Running, jobdata.Change{At: now, WorkerID: c.WorkerID})
	if err != nil {
		return requeue(ctx, d, err)
	}
	if !moved {
		j, err := s.store.Get(ctx, id)
		switch {
		case errors.Is(err, jobdata.ErrNotFound):
			return Skipped, d.Ack(ctx)
		case err != nil:
			return requeue(ctx, d, err)
		case j.State == jobdata.Running && j.WorkerID != c.WorkerID:
			lost := &jobdata.JobError{Code: CodeWorkerLost, Message: "the worker stopped while it ran the job",
				Details: map[string]any{"worker_id": j.WorkerID}}
			ok, err := s.move(ctx, id, []jobdata.State{jobdata.Running}, jobdata.Failed, jobdata.Change{At: now, Error: lost})
			if err != nil {
				return requeue(ctx, d, err)
			}
			if ok {
				return Lost, d.Ack(ctx)
			}
		}
		return Skipped, d.Ack(ctx)
	}

	j, err := s.store.Get(ctx, id)
	var result json.RawMessage
	if err == nil {
		progress := func(p jobdata.Progress) {
			_, _ = s.move(ctx, id, []jobdata.State{jobdata.Running}, jobdata.Running,
				jobdata.Change{At: s.cfg.Now(), Progress: &p, Event: jobdata.EventProgress})
		}
		result, err = c.Run(ctx, j, progress)
	}
	if ctx.Err() != nil {
		return Stopped, ctx.Err()
	}
	end := jobdata.Change{At: s.cfg.Now()}
	to := jobdata.Succeeded
	if err == nil {
		end.Result = result
	} else {
		to = jobdata.Failed
		if cur, gerr := s.store.Get(ctx, id); gerr == nil && !cur.CancelRequested.IsZero() {
			to = jobdata.Cancelled
		}
		end.Error = jobError(err)
	}
	_, merr := s.move(ctx, id, []jobdata.State{jobdata.Running}, to, end)
	return Ran, errors.Join(merr, d.Ack(ctx))
}

// requeue gives the delivery back to the broker, so that it comes again.
func requeue(ctx context.Context, d jobdata.Delivery, err error) (Outcome, error) {
	return Requeued, errors.Join(err, d.Nack(ctx, true))
}

// jobError is the error of a job: the code, the message and the details of a
// *herr.Error, else INTERNAL with the text of the error.
func jobError(err error) *jobdata.JobError {
	var he *herr.Error
	if errors.As(err, &he) {
		return &jobdata.JobError{Code: he.Code, Message: he.Message, Details: he.Details}
	}
	return &jobdata.JobError{Code: "INTERNAL", Message: err.Error()}
}
