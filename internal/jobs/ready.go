package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
)

// Health is what the readiness checks of horus-db and horus-worker need from
// the database, apart from Store. internal/postgres implements it.
type Health interface {
	// CheckSchema gives nil when the database answers and has the schema
	// version of this program.
	CheckSchema(ctx context.Context) error
	// WorkerLockHeld tells if a worker holds the one-worker lock now. It
	// takes no lock.
	WorkerLockHeld(ctx context.Context) (bool, error)
}

// WorkerFresh is the most age of the heartbeat of a worker that is up. The
// worker writes its heartbeat every 5 s.
const WorkerFresh = 15 * time.Second

// WithHealth gives the service the checks of the database. Checks needs it.
func (s *Service) WithHealth(h Health) *Service {
	s.health = h
	return s
}

// Checks runs the readiness checks of the job queue, in the order of the
// report: horus-db (the database answers and has the schema version of the
// program), then horus-worker (WorkerCheck). Without WithHealth, both fail.
func (s *Service) Checks(ctx context.Context) []jobdata.CheckResult {
	if s.health == nil {
		why := "the job queue has no database check (a bug of the wiring)"
		return []jobdata.CheckResult{{Dependency: jobdata.DependencyDB, Detail: why}, {Dependency: jobdata.DependencyWorker, Detail: why}}
	}
	db := jobdata.CheckResult{Dependency: jobdata.DependencyDB, Up: true, Detail: "answers, and has the schema version of this program"}
	if err := s.health.CheckSchema(ctx); err != nil {
		db = jobdata.CheckResult{Dependency: jobdata.DependencyDB, Detail: err.Error()}
		// The worker check reads the same database: name the cause once.
		return []jobdata.CheckResult{db, {Dependency: jobdata.DependencyWorker, Detail: "cannot read its row, because horus-db is down"}}
	}
	held, err := s.health.WorkerLockHeld(ctx)
	if err != nil {
		return []jobdata.CheckResult{db, {Dependency: jobdata.DependencyWorker, Detail: err.Error()}}
	}
	sum, err := s.store.Summary(ctx)
	if err != nil {
		return []jobdata.CheckResult{db, {Dependency: jobdata.DependencyWorker, Detail: err.Error()}}
	}
	return []jobdata.CheckResult{db, WorkerCheck(sum.Worker, held, s.cfg.Now())}
}

// WorkerCheck is the readiness rule of horus-worker: the newest worker row
// has a heartbeat newer than WorkerFresh at the time now, and a worker holds
// the one-worker lock.
func WorkerCheck(w *jobdata.Worker, lockHeld bool, now time.Time) jobdata.CheckResult {
	c := jobdata.CheckResult{Dependency: jobdata.DependencyWorker}
	switch {
	case w == nil:
		c.Detail = "no worker row: horus worker did not start"
	case w.HeartbeatAt.IsZero() || now.Sub(w.HeartbeatAt) > WorkerFresh:
		c.Detail = fmt.Sprintf("the last heartbeat of the worker %s is older than %s", w.ID, WorkerFresh)
		if !w.HeartbeatAt.IsZero() {
			c.Detail = fmt.Sprintf("the last heartbeat of the worker %s is %s old, more than %s",
				w.ID, now.Sub(w.HeartbeatAt).Round(time.Second), WorkerFresh)
		}
	case !lockHeld:
		c.Detail = fmt.Sprintf("no worker holds the worker lock (the newest row is the worker %s)", w.ID)
	default:
		c.Up = true
		c.Detail = fmt.Sprintf("the worker %s, %d slots, heartbeat %s ago", w.ID, w.Slots, now.Sub(w.HeartbeatAt).Round(time.Second))
	}
	return c
}
