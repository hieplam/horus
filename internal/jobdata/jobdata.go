// Package jobdata holds the data of the job queue: a job, its state, a change
// of its state, a message of the broker, a delivery, an event and the summary
// of the job store. It also holds the readiness report of horus.
//
// The package is in the base layer. The service internal/jobs declares the
// interfaces Broker and Store with these types, and an adapter (for example
// internal/postgres) implements them without an import of the service. The
// spec: docs/specs/2026-11-19-job-queue.md.
package jobdata

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// State is the state of a job.
type State string

// The states of a job. A job starts in Queued. The other states after
// Running are end states: the job never moves again.
const (
	Queued    State = "queued"
	Running   State = "running"
	Succeeded State = "succeeded"
	Failed    State = "failed"
	Cancelled State = "cancelled"
	Expired   State = "expired"
	Abandoned State = "abandoned"
)

// States are all the states, in the order of the spec.
var States = []State{Queued, Running, Succeeded, Failed, Cancelled, Expired, Abandoned}

// Ended tells if the state is an end state.
func (s State) Ended() bool {
	switch s {
	case Succeeded, Failed, Cancelled, Expired, Abandoned:
		return true
	}
	return false
}

// AbandonAfter is the time with no waiter after which the sweep moves a
// queued job to Abandoned.
const AbandonAfter = 60 * time.Second

// Errors that a store returns. An adapter returns them, so that the service
// can tell them apart without the text of the error.
var (
	// ErrNotFound: the job store has no job with this id.
	ErrNotFound = errors.New("jobdata: no job with this id")
	// ErrWorkerLocked: another worker holds the worker lock.
	ErrWorkerLocked = errors.New("jobdata: another worker holds the worker lock")
)

// Progress is the last progress report of a running job.
type Progress struct {
	Value   float64 `json:"value"`
	Total   float64 `json:"total"`
	Message string  `json:"message"`
}

// JobError is the error of a job that failed. Code is a code of the horus
// error table (internal/herr), for example WORKER_LOST.
type JobError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Job is one row of horus.jobs. An empty string, a zero time, a nil slice and
// a nil pointer are NULL in the row.
type Job struct {
	ID              string          `json:"id"` // UUIDv7
	Tool            string          `json:"tool"`
	Workflow        string          `json:"workflow"`
	Backend         string          `json:"backend"`
	WorkQueue       string          `json:"work_queue"`
	ResourceKey     string          `json:"resource_key"`
	Payload         json.RawMessage `json:"payload"`
	RequestHash     string          `json:"request_hash"`
	IdempotencyKey  string          `json:"idempotency_key"`
	State           State           `json:"state"`
	Attempt         int             `json:"attempt"`
	CreatedAt       time.Time       `json:"created_at"`
	Deadline        time.Time       `json:"deadline"`
	WaiterSeenAt    time.Time       `json:"waiter_seen_at"`
	StartedAt       time.Time       `json:"started_at"`
	FinishedAt      time.Time       `json:"finished_at"`
	WorkerID        string          `json:"worker_id"`
	BackendJobID    string          `json:"backend_job_id"`
	CancelRequested time.Time       `json:"cancel_requested_at"`
	Progress        *Progress       `json:"progress"`
	Result          json.RawMessage `json:"result"`
	Error           *JobError       `json:"error"`
	Caller          string          `json:"caller"`
}

// SweepState gives the state that the sweep sets on the job at the time now:
// Expired after the deadline, Abandoned after AbandonAfter with no waiter.
// ok is false when the sweep does not change the job. The sweep changes only
// a queued job.
func (j Job) SweepState(now time.Time) (s State, ok bool) {
	if j.State != Queued {
		return "", false
	}
	if !j.Deadline.IsZero() && now.After(j.Deadline) {
		return Expired, true
	}
	if !j.WaiterSeenAt.IsZero() && now.Sub(j.WaiterSeenAt) > AbandonAfter {
		return Abandoned, true
	}
	return "", false
}

// The types of an event that are not a state.
const (
	EventProgress        = "job.progress"
	EventCancelRequested = "job.cancel_requested"
)

// EventType is the type of the event of a move to the state s: "job." + s.
func EventType(s State) string { return "job." + string(s) }

// Change is what Store.Move writes with the new state, in one transaction
// with its event. A field with its zero value does not change the job.
type Change struct {
	// At is the time of the move. The store writes it as the time of the
	// event, as started_at when the job enters Running from another state, as
	// finished_at when the job enters an end state, and as
	// cancel_requested_at with CancelRequested.
	At time.Time
	// Event is the type of the event row. Empty: EventType of the new state.
	Event           string
	WorkerID        string
	BackendJobID    string
	Progress        *Progress
	Result          json.RawMessage
	Error           *JobError
	CancelRequested bool
}

// EventData is the data of the event row of the change: the progress, the
// result and the error that the change sets. nil when it sets none.
func (c Change) EventData() json.RawMessage {
	data := struct {
		Progress *Progress       `json:"progress,omitempty"`
		Result   json.RawMessage `json:"result,omitempty"`
		Error    *JobError       `json:"error,omitempty"`
	}{c.Progress, c.Result, c.Error}
	if data.Progress == nil && len(data.Result) == 0 && data.Error == nil {
		return nil
	}
	b, _ := json.Marshal(data)
	return b
}

// Apply gives the job after a move to the state to with the change c. The
// caller checks the state before: Apply does not. An adapter writes the same
// columns in SQL.
func Apply(j Job, to State, c Change) Job {
	if to == Running && j.State != Running {
		j.StartedAt = c.At
		j.Attempt++
	}
	if to.Ended() {
		j.FinishedAt = c.At
	}
	j.State = to
	if c.WorkerID != "" {
		j.WorkerID = c.WorkerID
	}
	if c.BackendJobID != "" {
		j.BackendJobID = c.BackendJobID
	}
	if c.Progress != nil {
		p := *c.Progress
		j.Progress = &p
	}
	if len(c.Result) > 0 {
		j.Result = c.Result
	}
	if c.Error != nil {
		e := *c.Error
		j.Error = &e
	}
	if c.CancelRequested {
		j.CancelRequested = c.At
	}
	return j
}

// MessageVersion is the v of a Message and of an Event.
const MessageVersion = 1

// Message is one message in a work queue of the broker. The job store holds
// the arguments and the state of the job: a message holds only its id.
type Message struct {
	V       int    `json:"v"`
	JobID   string `json:"job_id"`
	Tool    string `json:"tool"`
	Backend string `json:"backend"`
}

// Delivery is one message that the broker gives to a consumer. Until Ack or
// Nack, the message counts toward the prefetch of the consumer.
type Delivery struct {
	Message Message
	// Ack tells the broker that the consumer handled the message. The broker
	// deletes it.
	Ack func(ctx context.Context) error
	// Nack gives the message back. With requeue, the broker delivers it
	// again. Without requeue, the broker deletes it.
	Nack func(ctx context.Context, requeue bool) error
}

// Event is a signal to the processes that listen: a job changed. The job
// store keeps each event in horus.job_events. Seq is the id of that row when
// the sender knows it, else 0. A listener reads the job store for the facts.
type Event struct {
	V     int       `json:"v"`
	JobID string    `json:"job_id"`
	Type  string    `json:"type"`
	At    time.Time `json:"at"`
	Seq   int64     `json:"seq"`
}

// Worker is one row of horus.workers.
type Worker struct {
	ID                string    `json:"id"`
	Host              string    `json:"host"`
	PID               int       `json:"pid"`
	Version           string    `json:"version"`
	Slots             int       `json:"slots"`
	StartedAt         time.Time `json:"started_at"`
	HeartbeatAt       time.Time `json:"heartbeat_at"`
	LoadedResourceKey string    `json:"loaded_resource_key"`
}

// Summary is what the job store tells about the jobs now: the counts and the
// lists of the Horus UI. "Today" is since 00:00 in the time zone of the store.
type Summary struct {
	Queued         int `json:"queued"`
	Running        int `json:"running"`
	SucceededToday int `json:"succeeded_today"`
	FailedToday    int `json:"failed_today"`
	// SameKeyToday is the count of the jobs of today whose idempotency key
	// another job of today has: the measurement for the dedupe (ADR 0027).
	SameKeyToday int `json:"same_key_today"`
	// MeanRunSeconds is the mean run time of the last 10 succeeded jobs, from
	// started_at to finished_at. 0 when no job succeeded.
	MeanRunSeconds float64 `json:"mean_run_seconds"`
	RunningJobs    []Job   `json:"running_jobs"` // oldest start first
	QueuedJobs     []Job   `json:"queued_jobs"`  // oldest first
	Recent         []Job   `json:"recent"`       // the last 10 ended jobs, newest first
	Worker         *Worker `json:"worker"`       // the newest worker row, or nil
}
