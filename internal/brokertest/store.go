package brokertest

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
)

// Store is the in-memory job store of the tests. It keeps the rules of the
// interface jobs.Store: a move only from the expected state, with its event.
type Store struct {
	// Now is the clock of the store. nil: time.Now.
	Now func() time.Time

	mu      sync.Mutex
	jobs    map[string]*jobdata.Job
	order   []string // the ids in the order of Insert
	events  []EventRow
	lock    int // the number of the lock that holds the worker lock; 0: free
	locks   int // the count of locks that WorkerLock gave
	workers map[string]jobdata.Worker
}

// EventRow is one row of horus.job_events.
type EventRow struct {
	ID    int64
	JobID string
	At    time.Time
	Type  string
	Data  json.RawMessage
}

// NewStore makes an empty in-memory job store.
func NewStore() *Store {
	return &Store{jobs: map[string]*jobdata.Job{}, workers: map[string]jobdata.Worker{}}
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// event adds an event row. Hold mu.
func (s *Store) event(id, typ string, at time.Time, data json.RawMessage) {
	s.events = append(s.events, EventRow{ID: int64(len(s.events) + 1), JobID: id, At: at, Type: typ, Data: data})
}

// Insert writes the job and its event job.queued.
func (s *Store) Insert(ctx context.Context, j jobdata.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[j.ID]; ok {
		return errors.New("brokertest: a job with the id " + j.ID + " exists")
	}
	c := j
	s.jobs[j.ID] = &c
	s.order = append(s.order, j.ID)
	s.event(j.ID, jobdata.EventType(jobdata.Queued), j.CreatedAt, nil)
	return nil
}

// Get gives a copy of the job, or jobdata.ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (jobdata.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return jobdata.Job{}, jobdata.ErrNotFound
	}
	return *j, nil
}

// Move changes the state only from one of from, with its event.
func (s *Store) Move(ctx context.Context, id string, from []jobdata.State, to jobdata.State, c jobdata.Change) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok || !slices.Contains(from, j.State) {
		return false, nil
	}
	if c.At.IsZero() {
		c.At = s.now()
	}
	*j = jobdata.Apply(*j, to, c)
	typ := c.Event
	if typ == "" {
		typ = jobdata.EventType(to)
	}
	s.event(id, typ, c.At, c.EventData())
	return true, nil
}

// Touch writes the time now to waiter_seen_at.
func (s *Store) Touch(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return jobdata.ErrNotFound
	}
	j.WaiterSeenAt = s.now()
	return nil
}

// queued gives the queued jobs, oldest first. Hold mu.
func (s *Store) queued() []*jobdata.Job {
	var out []*jobdata.Job
	for _, id := range s.order {
		if j := s.jobs[id]; j.State == jobdata.Queued {
			out = append(out, j)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].CreatedAt.Before(out[b].CreatedAt) })
	return out
}

// Position gives the place of a queued job and the count of queued jobs.
func (s *Store) Position(ctx context.Context, id string) (pos, total int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[id]; !ok {
		return 0, 0, jobdata.ErrNotFound
	}
	q := s.queued()
	for i, j := range q {
		if j.ID == id {
			pos = i + 1
		}
	}
	return pos, len(q), nil
}

// Queued gives the count of queued jobs.
func (s *Store) Queued(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queued()), nil
}

// Sweep moves each queued job to the state of jobdata.Job.SweepState.
func (s *Store) Sweep(ctx context.Context) (expired, abandoned int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for _, j := range s.queued() {
		to, ok := j.SweepState(now)
		if !ok {
			continue
		}
		*j = jobdata.Apply(*j, to, jobdata.Change{At: now})
		s.event(j.ID, jobdata.EventType(to), now, nil)
		if to == jobdata.Expired {
			expired++
		} else {
			abandoned++
		}
	}
	return expired, abandoned, nil
}

// Summary gives the counts and the lists of the Horus UI.
func (s *Store) Summary(ctx context.Context) (jobdata.Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	sum := jobdata.Summary{WorkflowToday: map[string]int{}, RunningJobs: []jobdata.Job{}, QueuedJobs: []jobdata.Job{}, Recent: []jobdata.Job{}}
	keys := map[string]int{}
	var ended, succeeded []jobdata.Job
	for _, id := range s.order {
		j := *s.jobs[id]
		switch {
		case j.State == jobdata.Running:
			sum.Running++
			sum.RunningJobs = append(sum.RunningJobs, j)
		case j.State.Ended():
			ended = append(ended, j)
			if j.State == jobdata.Succeeded {
				succeeded = append(succeeded, j)
			}
			if !j.FinishedAt.Before(today) {
				switch j.State {
				case jobdata.Succeeded:
					sum.SucceededToday++
					sum.WorkflowToday[j.Workflow]++
				case jobdata.Failed:
					sum.FailedToday++
					sum.WorkflowToday[j.Workflow]++
				}
			}
		}
		if j.IdempotencyKey != "" && !j.CreatedAt.Before(today) {
			keys[j.IdempotencyKey]++
		}
	}
	for _, j := range s.queued() {
		sum.Queued++
		sum.QueuedJobs = append(sum.QueuedJobs, *j)
	}
	for _, n := range keys {
		if n > 1 {
			sum.SameKeyToday += n
		}
	}
	sum.Worker = s.newestWorker()
	sort.SliceStable(sum.RunningJobs, func(a, b int) bool { return sum.RunningJobs[a].StartedAt.Before(sum.RunningJobs[b].StartedAt) })
	byEnd := func(list []jobdata.Job) {
		sort.SliceStable(list, func(a, b int) bool { return list[a].FinishedAt.After(list[b].FinishedAt) })
	}
	byEnd(ended)
	sum.Recent = append(sum.Recent, ended[:min(10, len(ended))]...)
	byEnd(succeeded)
	if n := min(10, len(succeeded)); n > 0 {
		var total time.Duration
		for _, j := range succeeded[:n] {
			total += j.FinishedAt.Sub(j.StartedAt)
		}
		sum.MeanRunSeconds = total.Seconds() / float64(n)
	}
	return sum, nil
}

// newestWorker gives a copy of the worker row with the newest heartbeat, or
// nil. Of two rows with the same heartbeat, the smaller id comes first. Hold
// mu.
func (s *Store) newestWorker() *jobdata.Worker {
	var best *jobdata.Worker
	for _, w := range s.workers {
		if best == nil || w.HeartbeatAt.After(best.HeartbeatAt) ||
			w.HeartbeatAt.Equal(best.HeartbeatAt) && w.ID < best.ID {
			c := w
			best = &c
		}
	}
	return best
}

// Heartbeat writes the row of the worker w.ID, with heartbeat_at at the time
// now.
func (s *Store) Heartbeat(ctx context.Context, w jobdata.Worker) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.HeartbeatAt = s.now()
	s.workers[w.ID] = w
	return nil
}

// DeleteOld deletes each ended job that finished more than age before the time
// now, with its events, and each worker row whose heartbeat is that old.
func (s *Store) DeleteOld(ctx context.Context, age time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := s.now().Add(-age)
	gone := map[string]bool{}
	order := s.order[:0:0]
	for _, id := range s.order {
		if j := s.jobs[id]; j.State.Ended() && j.FinishedAt.Before(before) {
			gone[id] = true
			delete(s.jobs, id)
			continue
		}
		order = append(order, id)
	}
	s.order = order
	events := s.events[:0:0]
	for _, e := range s.events {
		if !gone[e.JobID] {
			events = append(events, e)
		}
	}
	s.events = events
	for id, w := range s.workers {
		if w.HeartbeatAt.Before(before) {
			delete(s.workers, id)
		}
	}
	return len(gone), nil
}

// WorkerLock takes the one-worker lock, or gives jobdata.ErrWorkerLocked.
// Each lock has its own number, as each Postgres lock has its own session.
func (s *Store) WorkerLock(ctx context.Context) (jobdata.Lock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock != 0 {
		return jobdata.Lock{}, jobdata.ErrWorkerLocked
	}
	s.locks++
	mine := s.locks
	s.lock = mine
	var once sync.Once
	return jobdata.Lock{
		Check: func(context.Context) error {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.lock != mine {
				return jobdata.ErrLockLost
			}
			return nil
		},
		Release: func() {
			once.Do(func() {
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.lock == mine {
					s.lock = 0
				}
			})
		},
	}, nil
}

// EndLockSession ends the session of the worker lock, as a restart of
// horus-db does: the holder loses the lock, and the lock is free.
func (s *Store) EndLockSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lock = 0
}

// Events gives the types of the event rows of a job, in order.
func (s *Store) Events(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, e := range s.events {
		if e.JobID == id {
			out = append(out, e.Type)
		}
	}
	return out
}
