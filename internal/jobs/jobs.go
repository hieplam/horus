// Package jobs is the service of the job queue: a call that makes an output
// becomes a job, and one worker runs the jobs.
//
// The API side (an API process of horus-api):
//
//	Submit  checks the rule of QUEUE_FULL, inserts the job and publishes one message.
//	Wait    waits until the job ends, and reports its place in the queue or its progress.
//	Touch   writes waiter_seen_at, so that the sweep does not abandon the job.
//	Cancel  moves a queued job to cancelled, or asks the worker to stop a running job.
//
// The worker side: Consume takes the deliveries of one work queue, and Handle
// handles one delivery as an idempotent consumer. It runs a delivery only
// when the job store moves the job from queued to running. A second delivery
// of the same job acks and does nothing.
//
// The pure functions: the allowed moves of a state (CanMove), the recipe of
// the fingerprint and the key (Fingerprint, Key; ADR 0027), the estimated
// start (EstimateStart) and the rule of QUEUE_FULL (CheckQueue).
//
// The service declares the interfaces Broker and Store. internal/postgres
// implements both. internal/brokertest holds the contract suite of a broker,
// the in-memory broker and the in-memory job store of the tests. The spec:
// docs/specs/2026-11-19-job-queue.md.
package jobs

import (
	"context"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
)

// Broker moves messages and events. internal/postgres implements it now.
type Broker interface {
	Publish(ctx context.Context, queue string, m jobdata.Message) error
	// Consume gives at most prefetch messages without an ack.
	// A message can come two times (at-least-once).
	// The channel closes when ctx ends. Then each message of the consumer
	// without an ack comes again to another consumer.
	Consume(ctx context.Context, queue string, prefetch int) (<-chan jobdata.Delivery, error)
	// Notify sends an event to the processes that listen now. It keeps nothing.
	Notify(ctx context.Context, e jobdata.Event) error
	// Listen gives the events that Notify sends after Listen returns. The
	// channel closes when ctx ends.
	Listen(ctx context.Context) (<-chan jobdata.Event, error)
	// Depth gives the count of the messages of the queue that wait for a
	// delivery. A message that a consumer holds without an ack does not
	// count. The Horus UI shows it.
	Depth(ctx context.Context, queue string) (int, error)
}

// Store keeps the jobs. It is the record of truth.
type Store interface {
	// Insert writes the job and its event job.queued in one transaction.
	Insert(ctx context.Context, j jobdata.Job) error
	// Get gives jobdata.ErrNotFound when the store has no job with the id.
	Get(ctx context.Context, id string) (jobdata.Job, error)
	// Move changes the state only from one of from, with its event, in one
	// transaction. false: the job was not in from. A job that is not in the
	// store gives false and no error. The columns that change are those of
	// jobdata.Apply, and the event row has the type c.Event (or
	// jobdata.EventType(to)), the time c.At and the data c.EventData().
	Move(ctx context.Context, id string, from []jobdata.State, to jobdata.State, c jobdata.Change) (bool, error)
	// Touch writes the time now to waiter_seen_at.
	Touch(ctx context.Context, id string) error
	// Position gives the place of a queued job among the queued jobs, oldest
	// first (1 is the next to start), and the count of queued jobs. pos is 0
	// when the job is not queued.
	Position(ctx context.Context, id string) (pos, total int, err error)
	// Queued gives the count of queued jobs.
	Queued(ctx context.Context) (int, error)
	// Sweep moves each queued job to the state of jobdata.Job.SweepState at
	// the time now: expired after its deadline, abandoned after
	// jobdata.AbandonAfter with no waiter.
	Sweep(ctx context.Context) (expired, abandoned int, err error)
	Summary(ctx context.Context) (jobdata.Summary, error)
	// WorkerLock takes the one-worker lock. It gives jobdata.ErrWorkerLocked
	// when another worker holds it. Lock.Check tells if the lock is still
	// held: the lock goes when its session ends, for example at a restart of
	// the database (ADR 0037).
	WorkerLock(ctx context.Context) (jobdata.Lock, error)
	// Heartbeat writes the row of the worker w.ID in horus.workers, with
	// heartbeat_at at the time now. The first call inserts the row. The
	// summary gives the row with the newest heartbeat.
	Heartbeat(ctx context.Context, w jobdata.Worker) error
	// DeleteOld deletes each ended job whose finished_at is more than age
	// before the time now, with its events, and each worker row whose
	// heartbeat_at is more than age before the time now. It gives the count
	// of deleted jobs. It never deletes a queued or a running job.
	DeleteOld(ctx context.Context, age time.Duration) (int, error)
}

// Config holds the settings of the service. A zero field gets its default.
type Config struct {
	// QueueMax is HORUS_QUEUE_MAX: the count of queued jobs at which a new
	// call gets QUEUE_FULL. Default 20.
	QueueMax int
	// QueueTimeout is HORUS_QUEUE_TIMEOUT: the longest wait in the queue.
	// Default 7200 s.
	QueueTimeout time.Duration
	// Slots is HORUS_WORKER_CONCURRENCY: the count of jobs that the worker
	// runs at the same time. Default 1.
	Slots int
	// DefaultRun is the run time of a job that the estimate uses before a job
	// succeeded. Default 240 s.
	DefaultRun time.Duration
	// Poll is how often a waiter reads the job store when no event comes, and
	// reports its progress. Default 5 s.
	Poll time.Duration
	// Touch is how often a waiter writes waiter_seen_at. Default 10 s. A
	// waiter reports its progress at each Poll.
	Touch time.Duration
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

// The defaults of Config.
const (
	DefaultQueueMax     = 20
	DefaultQueueTimeout = 7200 * time.Second
	DefaultSlots        = 1
	DefaultRunTime      = 240 * time.Second
	DefaultPoll         = 5 * time.Second
	DefaultTouch        = 10 * time.Second
)

func (c Config) withDefaults() Config {
	if c.QueueMax <= 0 {
		c.QueueMax = DefaultQueueMax
	}
	if c.QueueTimeout <= 0 {
		c.QueueTimeout = DefaultQueueTimeout
	}
	if c.Slots <= 0 {
		c.Slots = DefaultSlots
	}
	if c.DefaultRun <= 0 {
		c.DefaultRun = DefaultRunTime
	}
	if c.Poll <= 0 {
		c.Poll = DefaultPoll
	}
	if c.Touch <= 0 {
		c.Touch = DefaultTouch
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// Service is the service of the job queue.
type Service struct {
	store  Store
	broker Broker
	cfg    Config
	health Health // the readiness checks of the database (ready.go); nil without WithHealth
}

// New makes the service. cmd/horus gives it the adapters of internal/postgres.
// A test gives it the in-memory broker and store of internal/brokertest.
func New(s Store, b Broker, c Config) *Service {
	return &Service{store: s, broker: b, cfg: c.withDefaults()}
}

// Report sends one progress report. total 0 means unknown.
type Report func(ctx context.Context, progress, total float64, message string)
