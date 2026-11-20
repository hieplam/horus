// Package pgtest gives a test its own Postgres database. It reads the URL of
// a test Postgres from HORUS_TEST_DATABASE_URL. Each database has a new name
// that ends in _test, and the test drops it at its end. A test never uses the
// database horus.
//
// Without HORUS_TEST_DATABASE_URL, the test skips. The goal row
// broker-contract fails in that case (goals/2026-11-19-job-queue).
//
// Only a test file imports this package.
package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hieplam/horus/internal/postgres"
)

// Env is the variable with the URL of the test Postgres:
// postgres://user:password@host:port/postgres?sslmode=disable.
const Env = "HORUS_TEST_DATABASE_URL"

// Need skips the test when Env is not set.
func Need(t testing.TB) {
	t.Helper()
	if os.Getenv(Env) == "" {
		t.Skip(Env + " is not set: this test needs a test Postgres (docs/mcp.md#the-postgres-tests)")
	}
}

// URL makes a new empty database for the test, and gives its URL. The test
// drops the database at its end. Without Env, the test skips.
func URL(t testing.TB) string {
	t.Helper()
	Need(t)
	base, err := url.Parse(os.Getenv(Env))
	if err != nil || base.Scheme == "" {
		t.Fatalf("%s is not a URL of the form postgres://user:password@host:port/postgres", Env)
	}
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	name := "horus_" + hex.EncodeToString(b[:]) + "_test"
	admin(t, base, "CREATE DATABASE "+name)
	t.Cleanup(func() { admin(t, base, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)") })
	u := *base
	u.Path = "/" + name
	return u.String()
}

// DB gives a pool of connections to a new database with the schema horus.
// The test closes it and drops the database at its end.
func DB(t testing.TB) *postgres.DB {
	t.Helper()
	return Open(t, URL(t))
}

// Open gives a pool of connections to the database of url (from URL), and
// applies the schema horus. The test closes the pool at its end.
func Open(t testing.TB, url string) *postgres.DB {
	t.Helper()
	ctx := context.Background()
	db, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return db
}

// inThisDB limits a row of pg_locks or pg_stat_activity to the database of the
// connection. Each test has its own database, so a test never touches the
// sessions of another test that runs at the same time.
const inThisDB = "(SELECT oid FROM pg_database WHERE datname = current_database())"

// holdsWorkerLock is the SQL condition of a row of pg_locks that holds the
// one-worker lock in the database of the connection. $1 is
// postgres.WorkerLockKey.
const holdsWorkerLock = "locktype = 'advisory' AND granted AND objsubid = 1 AND database = " + inThisDB +
	" AND ((classid::bigint << 32) | objid::bigint) = $1"

// WorkerLockHolder gives the process id of the session that holds the
// one-worker lock in the database of url, or 0 when no session holds it.
func WorkerLockHolder(t testing.TB, url string) int {
	t.Helper()
	var pid int
	query(t, url, func(ctx context.Context, conn *pgx.Conn) error {
		return conn.QueryRow(ctx, "SELECT coalesce((SELECT pid FROM pg_locks WHERE "+holdsWorkerLock+" LIMIT 1), 0)",
			postgres.WorkerLockKey).Scan(&pid)
	})
	return pid
}

// EndWorkerLock ends the session that holds the one-worker lock in the
// database of url, as pg_terminate_backend does, and waits until the session
// is gone. Postgres then releases the lock. It gives the count of ended
// sessions.
func EndWorkerLock(t testing.TB, url string) int {
	t.Helper()
	var n int
	query(t, url, func(ctx context.Context, conn *pgx.Conn) error {
		return conn.QueryRow(ctx, "SELECT count(*) FILTER (WHERE pg_terminate_backend(pid, 10000)) FROM pg_locks WHERE "+holdsWorkerLock,
			postgres.WorkerLockKey).Scan(&n)
	})
	return n
}

// EndSessions ends each other session of the database of url, as a restart of
// the database does, and waits until each one is gone. It gives the count of
// ended sessions.
func EndSessions(t testing.TB, url string) int {
	t.Helper()
	var n int
	query(t, url, func(ctx context.Context, conn *pgx.Conn) error {
		return conn.QueryRow(ctx, `SELECT count(*) FILTER (WHERE pg_terminate_backend(pid, 10000)) FROM pg_stat_activity
WHERE datname = current_database() AND pid <> pg_backend_pid()`).Scan(&n)
	})
	return n
}

// query runs f on a new connection to the database of url. An error never
// shows the password.
func query(t testing.TB, url string, f func(ctx context.Context, conn *pgx.Conn) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect to the test database: %v", err)
	}
	defer conn.Close(ctx)
	if err := f(ctx, conn); err != nil {
		t.Fatalf("the test database: %v", err)
	}
}

// admin runs one statement in the database of base. An error never shows the
// password.
func admin(t testing.TB, base *url.URL, sql string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, base.String())
	if err != nil {
		t.Fatalf("connect to %s: %v", base.Redacted(), err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, sql); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}
