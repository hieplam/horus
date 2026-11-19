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
	ctx := context.Background()
	db, err := postgres.Connect(ctx, URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return db
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
