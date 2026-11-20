package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/postgres"
	"github.com/hieplam/horus/internal/postgres/pgtest"
)

// TestWorkerLockHeld: the check reads the one-worker lock and never takes it.
// It is false before a worker takes the lock, true while a worker holds it,
// and false again after the release. A worker that starts after many checks
// still gets the lock.
func TestWorkerLockHeld(t *testing.T) {
	ctx := context.Background()
	db := open(t, pgtest.URL(t))
	store := postgres.NewStore(db)
	held := func(want bool, when string) {
		t.Helper()
		got, err := db.WorkerLockHeld(ctx)
		if err != nil || got != want {
			t.Fatalf("%s: WorkerLockHeld = %v, %v, want %v", when, got, err, want)
		}
	}
	held(false, "no worker")
	held(false, "a second check")
	lock, err := store.WorkerLock(ctx)
	if err != nil {
		t.Fatalf("a worker after two checks does not get the lock: %v", err)
	}
	held(true, "a worker holds the lock")
	lock.Release()
	held(false, "the worker released the lock")
}

// TestOpenWithTheDatabaseDown: Open makes the pool while the database does
// not answer, and the first query reports the error without the password. A
// process of the stack that starts before horus-db then works when it is up.
func TestOpenWithTheDatabaseDown(t *testing.T) {
	const secret = "s3cret-pw-for-the-open-test"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.Open(ctx, "postgres://horus:"+secret+"@127.0.0.1:1/horus?sslmode=disable&connect_timeout=2")
	if err != nil {
		t.Fatalf("Open of a closed port: %v, want a pool and no error", err)
	}
	defer db.Close()
	err = db.CheckSchema(ctx)
	if err == nil {
		t.Fatal("CheckSchema on a closed port: no error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("the error shows the password: %v", err)
	}
	if _, err := db.WorkerLockHeld(ctx); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("WorkerLockHeld on a closed port: %v, want an error without the password", err)
	}
}
