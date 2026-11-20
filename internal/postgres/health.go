package postgres

import (
	"context"
	"fmt"
)

// WorkerLockHeld tells if a session of this database holds the one-worker
// lock now (WorkerLock). It reads pg_locks and takes no lock, so it never
// makes a worker fail to start. It is a part of the readiness check of
// horus-worker.
//
// Postgres shows an advisory lock with a bigint key as two oid values:
// classid is the high 32 bits of the key, and objid the low 32 bits.
func (db *DB) WorkerLockHeld(ctx context.Context) (bool, error) {
	var held bool
	err := db.pool.QueryRow(ctx, `SELECT EXISTS (
    SELECT 1 FROM pg_locks
    WHERE locktype = 'advisory' AND granted AND objsubid = 1
      AND database = (SELECT oid FROM pg_database WHERE datname = current_database())
      AND (classid::bigint << 32) | objid::bigint = $1)`, lockWorker).Scan(&held)
	if err != nil {
		return false, fmt.Errorf("postgres: read the worker lock: %w", err)
	}
	return held, nil
}
