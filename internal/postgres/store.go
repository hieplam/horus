package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hieplam/horus/internal/jobdata"
)

// Store is the job store in Postgres: the tables horus.jobs, horus.job_events
// and horus.workers. It implements jobs.Store.
type Store struct {
	// Now is the clock of Touch, Sweep, Summary and of a Move without a time.
	// nil: time.Now. The service gives the time of each other change, so all
	// the times of a job come from the clock of the program, as in the
	// in-memory store.
	Now func() time.Time

	db *DB
}

// NewStore gives the job store of the database.
func NewStore(db *DB) *Store { return &Store{db: db} }

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// jobColumns are the columns of horus.jobs in the order of scanJob.
const jobColumns = `id::text, tool, workflow, backend, work_queue, resource_key, payload::text, request_hash,
idempotency_key, state, attempt, created_at, deadline, waiter_seen_at, started_at, finished_at, worker_id,
backend_job_id, cancel_requested_at, progress::text, result::text, error::text, caller`

// scanJob reads one row of jobColumns. NULL gives the zero value.
func scanJob(row pgx.Row) (jobdata.Job, error) {
	var j jobdata.Job
	var tool, workflow, backend, workQueue, resourceKey, payload, requestHash, key, workerID, backendJobID *string
	var progress, result, jerr, caller *string
	var deadline, waiter, started, finished, cancel *time.Time
	var state string
	err := row.Scan(&j.ID, &tool, &workflow, &backend, &workQueue, &resourceKey, &payload, &requestHash,
		&key, &state, &j.Attempt, &j.CreatedAt, &deadline, &waiter, &started, &finished, &workerID,
		&backendJobID, &cancel, &progress, &result, &jerr, &caller)
	if err != nil {
		return jobdata.Job{}, err
	}
	j.Tool, j.Workflow, j.Backend, j.WorkQueue = str(tool), str(workflow), str(backend), str(workQueue)
	j.ResourceKey, j.RequestHash, j.IdempotencyKey = str(resourceKey), str(requestHash), str(key)
	j.WorkerID, j.BackendJobID, j.Caller = str(workerID), str(backendJobID), str(caller)
	j.State = jobdata.State(state)
	j.Deadline, j.WaiterSeenAt, j.StartedAt, j.FinishedAt, j.CancelRequested = tm(deadline), tm(waiter), tm(started), tm(finished), tm(cancel)
	if payload != nil {
		j.Payload = json.RawMessage(*payload)
	}
	if result != nil {
		j.Result = json.RawMessage(*result)
	}
	if progress != nil {
		j.Progress = &jobdata.Progress{}
		if err := json.Unmarshal([]byte(*progress), j.Progress); err != nil {
			return jobdata.Job{}, fmt.Errorf("postgres: the progress of the job %s: %w", j.ID, err)
		}
	}
	if jerr != nil {
		j.Error = &jobdata.JobError{}
		if err := json.Unmarshal([]byte(*jerr), j.Error); err != nil {
			return jobdata.Job{}, fmt.Errorf("postgres: the error of the job %s: %w", j.ID, err)
		}
	}
	return j, nil
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func tm(p *time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	return *p
}

// The arguments of a column that can be NULL: nil is NULL.

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

// jsonOf gives the JSON of v, or nil when v is a nil pointer.
func jsonOf[T any](v *T) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Insert writes the job and its event job.queued in one transaction.
func (s *Store) Insert(ctx context.Context, j jobdata.Job) error {
	progress, err := jsonOf(j.Progress)
	if err != nil {
		return err
	}
	jerr, err := jsonOf(j.Error)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.db.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO horus.jobs (id, tool, workflow, backend, work_queue, resource_key, payload,
request_hash, idempotency_key, state, attempt, created_at, deadline, waiter_seen_at, started_at, finished_at,
worker_id, backend_job_id, cancel_requested_at, progress, result, error, caller)
VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20::jsonb,
$21::jsonb, $22::jsonb, $23)`,
			j.ID, nullStr(j.Tool), nullStr(j.Workflow), nullStr(j.Backend), nullStr(j.WorkQueue), nullStr(j.ResourceKey),
			nullJSON(j.Payload), nullStr(j.RequestHash), nullStr(j.IdempotencyKey), string(j.State), j.Attempt, j.CreatedAt,
			nullTime(j.Deadline), nullTime(j.WaiterSeenAt), nullTime(j.StartedAt), nullTime(j.FinishedAt),
			nullStr(j.WorkerID), nullStr(j.BackendJobID), nullTime(j.CancelRequested), progress, nullJSON(j.Result), jerr,
			nullStr(j.Caller))
		if err != nil {
			return fmt.Errorf("postgres: insert the job %s: %w", j.ID, err)
		}
		return addEvent(ctx, tx, j.ID, jobdata.EventType(jobdata.Queued), j.CreatedAt, nil)
	})
}

// addEvent writes one row of horus.job_events.
func addEvent(ctx context.Context, tx pgx.Tx, id, typ string, at time.Time, data json.RawMessage) error {
	_, err := tx.Exec(ctx, "INSERT INTO horus.job_events (job_id, at, type, data) VALUES ($1, $2, $3, $4::jsonb)",
		id, at, typ, nullJSON(data))
	if err != nil {
		return fmt.Errorf("postgres: the event %s of the job %s: %w", typ, id, err)
	}
	return nil
}

// Get gives the job, or jobdata.ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (jobdata.Job, error) {
	j, err := scanJob(s.db.pool.QueryRow(ctx, "SELECT "+jobColumns+" FROM horus.jobs WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) || notUUID(err) {
		return jobdata.Job{}, jobdata.ErrNotFound
	}
	return j, err
}

// Move changes the state only from one of from, with its event, in one
// transaction. It locks the row, so that of two moves from the same state at
// the same time, one moves the job and the other gives false. The columns
// that change are those of jobdata.Apply.
func (s *Store) Move(ctx context.Context, id string, from []jobdata.State, to jobdata.State, c jobdata.Change) (bool, error) {
	if c.At.IsZero() {
		c.At = s.now()
	}
	moved := false
	err := pgx.BeginFunc(ctx, s.db.pool, func(tx pgx.Tx) error {
		cur, err := scanJob(tx.QueryRow(ctx, "SELECT "+jobColumns+" FROM horus.jobs WHERE id = $1 FOR UPDATE", id))
		if notUUID(err) {
			return errNoJob // the failed statement ended the transaction: roll it back
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if !slices.Contains(from, cur.State) {
			return nil
		}
		next := jobdata.Apply(cur, to, c)
		progress, err := jsonOf(next.Progress)
		if err != nil {
			return err
		}
		jerr, err := jsonOf(next.Error)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE horus.jobs SET state = $2, attempt = $3, started_at = $4, finished_at = $5,
worker_id = $6, backend_job_id = $7, progress = $8::jsonb, result = $9::jsonb, error = $10::jsonb,
cancel_requested_at = $11 WHERE id = $1`,
			id, string(next.State), next.Attempt, nullTime(next.StartedAt), nullTime(next.FinishedAt),
			nullStr(next.WorkerID), nullStr(next.BackendJobID), progress, nullJSON(next.Result), jerr,
			nullTime(next.CancelRequested))
		if err != nil {
			return fmt.Errorf("postgres: move the job %s to %s: %w", id, to, err)
		}
		typ := c.Event
		if typ == "" {
			typ = jobdata.EventType(to)
		}
		if err := addEvent(ctx, tx, id, typ, c.At, c.EventData()); err != nil {
			return err
		}
		moved = true
		return nil
	})
	if err != nil && !errors.Is(err, errNoJob) {
		return false, err
	}
	return moved, nil
}

// errNoJob ends the transaction of a Move whose id is not a UUID.
var errNoJob = errors.New("postgres: no such job")

// Touch writes the time now to waiter_seen_at.
func (s *Store) Touch(ctx context.Context, id string) error {
	tag, err := s.db.pool.Exec(ctx, "UPDATE horus.jobs SET waiter_seen_at = $2 WHERE id = $1", id, s.now())
	if notUUID(err) || err == nil && tag.RowsAffected() == 0 {
		return jobdata.ErrNotFound
	}
	return err
}

// Position gives the place of a queued job among the queued jobs, oldest
// first, and the count of queued jobs. pos is 0 when the job is not queued.
func (s *Store) Position(ctx context.Context, id string) (pos, total int, err error) {
	var state *string
	err = s.db.pool.QueryRow(ctx, `SELECT
    (SELECT state FROM horus.jobs WHERE id = $1),
    (SELECT count(*) FROM horus.jobs WHERE state = 'queued'),
    (SELECT count(*) FROM horus.jobs q JOIN horus.jobs me ON me.id = $1
        WHERE me.state = 'queued' AND q.state = 'queued' AND (q.created_at, q.id) <= (me.created_at, me.id))`,
		id).Scan(&state, &total, &pos)
	if notUUID(err) || err == nil && state == nil {
		return 0, 0, jobdata.ErrNotFound
	}
	if err != nil {
		return 0, 0, err
	}
	return pos, total, nil
}

// Queued gives the count of queued jobs.
func (s *Store) Queued(ctx context.Context) (int, error) {
	var n int
	err := s.db.pool.QueryRow(ctx, "SELECT count(*) FROM horus.jobs WHERE state = 'queued'").Scan(&n)
	return n, err
}

// Sweep moves each queued job to the state of jobdata.Job.SweepState at the
// time now, with its event, in one statement: expired after its deadline,
// abandoned after jobdata.AbandonAfter with no waiter.
func (s *Store) Sweep(ctx context.Context) (expired, abandoned int, err error) {
	now := s.now()
	rows, err := s.db.pool.Query(ctx, `WITH swept AS (
    UPDATE horus.jobs SET finished_at = $1,
        state = CASE WHEN deadline IS NOT NULL AND deadline < $1 THEN 'expired' ELSE 'abandoned' END
    WHERE state = 'queued' AND ((deadline IS NOT NULL AND deadline < $1) OR (waiter_seen_at IS NOT NULL AND waiter_seen_at < $2))
    RETURNING id, state
), events AS (
    INSERT INTO horus.job_events (job_id, at, type) SELECT id, $1, 'job.' || state FROM swept
)
SELECT state, count(*) FROM swept GROUP BY state`, now, now.Add(-jobdata.AbandonAfter))
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var state string
		var n int
		if err := rows.Scan(&state, &n); err != nil {
			return 0, 0, err
		}
		switch jobdata.State(state) {
		case jobdata.Expired:
			expired = n
		case jobdata.Abandoned:
			abandoned = n
		}
	}
	return expired, abandoned, rows.Err()
}

// endedStates is the SQL list of the end states.
const endedStates = "('succeeded', 'failed', 'cancelled', 'expired', 'abandoned')"

// Summary gives the counts and the lists of the Horus UI, from one snapshot of
// the database. "Today" is since 00:00 of the day of the clock, in its time
// zone.
func (s *Store) Summary(ctx context.Context) (jobdata.Summary, error) {
	now := s.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	sum := jobdata.Summary{}
	err := pgx.BeginTxFunc(ctx, s.db.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT
    count(*) FILTER (WHERE state = 'queued'),
    count(*) FILTER (WHERE state = 'running'),
    count(*) FILTER (WHERE state = 'succeeded' AND finished_at >= $1),
    count(*) FILTER (WHERE state = 'failed' AND finished_at >= $1)
FROM horus.jobs`, today).Scan(&sum.Queued, &sum.Running, &sum.SucceededToday, &sum.FailedToday)
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `SELECT coalesce(sum(n), 0) FROM (
    SELECT count(*) AS n FROM horus.jobs WHERE idempotency_key IS NOT NULL AND created_at >= $1
    GROUP BY idempotency_key HAVING count(*) > 1) AS same`, today).Scan(&sum.SameKeyToday)
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `SELECT coalesce(avg(extract(epoch FROM finished_at - started_at)), 0)::float8 FROM (
    SELECT finished_at, started_at FROM horus.jobs WHERE state = 'succeeded'
    ORDER BY finished_at DESC LIMIT 10) AS last`).Scan(&sum.MeanRunSeconds)
		if err != nil {
			return err
		}
		lists := []struct {
			into  *[]jobdata.Job
			where string
		}{
			{&sum.RunningJobs, "state = 'running' ORDER BY started_at, id"},
			{&sum.QueuedJobs, "state = 'queued' ORDER BY created_at, id"},
			{&sum.Recent, "state IN " + endedStates + " ORDER BY finished_at DESC, id DESC LIMIT 10"},
		}
		for _, l := range lists {
			rows, err := tx.Query(ctx, "SELECT "+jobColumns+" FROM horus.jobs WHERE "+l.where)
			if err != nil {
				return err
			}
			list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (jobdata.Job, error) { return scanJob(r) })
			if err != nil {
				return err
			}
			*l.into = append([]jobdata.Job{}, list...)
		}
		w, err := newestWorker(ctx, tx)
		sum.Worker = w
		return err
	})
	if err != nil {
		return jobdata.Summary{}, fmt.Errorf("postgres: the summary of the job store: %w", err)
	}
	return sum, nil
}

// newestWorker gives the worker row with the newest heartbeat, or nil.
func newestWorker(ctx context.Context, tx pgx.Tx) (*jobdata.Worker, error) {
	var w jobdata.Worker
	var host, version, loaded *string
	var pid, slots *int
	var started, beat *time.Time
	err := tx.QueryRow(ctx, `SELECT id, host, pid, version, slots, started_at, heartbeat_at, loaded_resource_key
FROM horus.workers ORDER BY heartbeat_at DESC NULLS LAST, id LIMIT 1`).Scan(&w.ID, &host, &pid, &version, &slots, &started, &beat, &loaded)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	w.Host, w.Version, w.LoadedResourceKey = str(host), str(version), str(loaded)
	if pid != nil {
		w.PID = *pid
	}
	if slots != nil {
		w.Slots = *slots
	}
	w.StartedAt, w.HeartbeatAt = tm(started), tm(beat)
	return &w, nil
}

// Heartbeat writes the row of the worker w.ID in horus.workers, with
// heartbeat_at at the time now. The first call inserts the row, and each
// later call writes each column again.
func (s *Store) Heartbeat(ctx context.Context, w jobdata.Worker) error {
	_, err := s.db.pool.Exec(ctx, `INSERT INTO horus.workers (id, host, pid, version, slots, started_at, heartbeat_at, loaded_resource_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET host = excluded.host, pid = excluded.pid, version = excluded.version,
    slots = excluded.slots, started_at = excluded.started_at, heartbeat_at = excluded.heartbeat_at,
    loaded_resource_key = excluded.loaded_resource_key`,
		w.ID, nullStr(w.Host), w.PID, nullStr(w.Version), w.Slots, nullTime(w.StartedAt), s.now(), nullStr(w.LoadedResourceKey))
	if err != nil {
		return fmt.Errorf("postgres: the heartbeat of the worker %s: %w", w.ID, err)
	}
	return nil
}

// DeleteOld deletes each ended job whose finished_at is more than age before
// the time now, and each worker row whose heartbeat_at is that old, in one
// transaction. The events of a job go with it (ON DELETE CASCADE).
func (s *Store) DeleteOld(ctx context.Context, age time.Duration) (int, error) {
	before := s.now().Add(-age)
	var n int64
	err := pgx.BeginFunc(ctx, s.db.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "DELETE FROM horus.jobs WHERE state IN "+endedStates+" AND finished_at < $1", before)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
		_, err = tx.Exec(ctx, "DELETE FROM horus.workers WHERE heartbeat_at < $1", before)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("postgres: delete the old jobs: %w", err)
	}
	return int(n), nil
}

// Events gives the event rows of the job, oldest first. Seq is the id of the
// row.
func (s *Store) Events(ctx context.Context, id string) ([]jobdata.Event, error) {
	rows, err := s.db.pool.Query(ctx, "SELECT id, job_id::text, type, at FROM horus.job_events WHERE job_id = $1 ORDER BY id", id)
	if err != nil {
		if notUUID(err) {
			return nil, nil
		}
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (jobdata.Event, error) {
		e := jobdata.Event{V: jobdata.MessageVersion}
		err := r.Scan(&e.Seq, &e.JobID, &e.Type, &e.At)
		return e, err
	})
}

// WorkerLock takes the one-worker lock: a session advisory lock on its own
// connection. It gives jobdata.ErrWorkerLocked when another session holds it.
// When the session ends (a restart of the database, or pg_terminate_backend),
// Postgres releases the lock. Check asks the session of the lock if it holds
// the lock, and gives an error that wraps jobdata.ErrLockLost when the session
// ended. The release unlocks and closes the connection.
func (s *Store) WorkerLock(ctx context.Context) (jobdata.Lock, error) {
	conn, err := s.db.dial(ctx)
	if err != nil {
		return jobdata.Lock{}, fmt.Errorf("postgres: the connection of the worker lock: %w", err)
	}
	var ok bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockWorker).Scan(&ok); err != nil {
		_ = conn.Close(context.Background())
		return jobdata.Lock{}, fmt.Errorf("postgres: take the worker lock: %w", err)
	}
	if !ok {
		_ = conn.Close(context.Background())
		return jobdata.Lock{}, jobdata.ErrWorkerLocked
	}
	var mu sync.Mutex // a pgx.Conn runs one statement at a time
	var once sync.Once
	return jobdata.Lock{
		Check: func(ctx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			var held bool
			err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid = pg_backend_pid() AND locktype = 'advisory'
    AND granted AND objsubid = 1 AND ((classid::bigint << 32) | objid::bigint) = $1)`, lockWorker).Scan(&held)
			if err != nil {
				return fmt.Errorf("postgres: the session of the worker lock ended: %w: %w", jobdata.ErrLockLost, err)
			}
			if !held {
				return fmt.Errorf("postgres: the session of the worker lock holds no lock: %w", jobdata.ErrLockLost)
			}
			return nil
		},
		Release: func() {
			once.Do(func() {
				mu.Lock()
				defer mu.Unlock()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if !conn.IsClosed() {
					_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockWorker)
				}
				_ = conn.Close(ctx)
			})
		},
	}, nil
}
