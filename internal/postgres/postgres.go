// Package postgres is the adapter of the job queue on Postgres: the job store
// (Store) and the Postgres broker (Broker). The service internal/jobs declares
// the interfaces jobs.Store and jobs.Broker, and this package implements them
// with github.com/jackc/pgx/v5. It never imports the service.
//
// The schema horus comes from the SQL files in schema/, which the program
// holds. The worker applies it at start with Migrate, under an advisory lock.
// CheckSchema tells if a database has the schema version of this program.
//
// The broker uses only the table horus.messages and the channels
// horus_messages and horus_events. The job store never reads horus.messages
// (ADR 0026). The spec: docs/specs/2026-11-19-job-queue.md, the sections "The
// broker interface", "Job states" and "Data in horus-db".
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SchemaVersion is the version of the schema horus that this program uses:
// the number of the newest file in schema/.
const SchemaVersion = 1

// The keys of the advisory locks of horus: "horus" in ASCII, then a number.
const (
	lockSchema int64 = 0x68_6f_72_75_73_01 // Migrate holds it in its transaction
	lockWorker int64 = 0x68_6f_72_75_73_02 // the one-worker lock: a session lock
)

// WorkerLockKey is the key of the one-worker lock (Store.WorkerLock). In
// pg_locks, classid holds its high 32 bits and objid its low 32 bits.
const WorkerLockKey = lockWorker

//go:embed schema/*.sql
var schemaFiles embed.FS

// migration is one file of schema/: NNNN_name.sql.
type migration struct {
	version int
	name    string
	sql     string
}

// migrations gives the files of schema/, oldest first.
func migrations() ([]migration, error) {
	names, err := fs.Glob(schemaFiles, "schema/*.sql")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, name := range names {
		base := strings.TrimPrefix(name, "schema/")
		num, _, ok := strings.Cut(base, "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil {
			return nil, fmt.Errorf("postgres: the schema file %s has no number NNNN_ at its start", base)
		}
		b, err := schemaFiles.ReadFile(name)
		if err != nil {
			return nil, err
		}
		out = append(out, migration{version: v, name: base, sql: string(b)})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].version < out[b].version })
	return out, nil
}

// DB is a pool of connections to one Postgres database.
type DB struct {
	pool *pgxpool.Pool
}

// Connect opens a pool of connections to the database of the URL
// (postgres://user:password@host:port/name), and checks one connection. An
// error never holds the password.
func Connect(ctx context.Context, url string) (*DB, error) {
	db, err := Open(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := db.pool.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	return db, nil
}

// Open makes a pool of connections to the database of the URL, and opens no
// connection yet. Each query opens a connection when it needs one, so a
// process that starts while the database is down works when the database is
// up again: horus api and horus mcp in the stack use it, and their readiness
// check of horus-db reports the error. A password can also come from the
// variable PGPASSWORD, so the URL needs no password. An error never holds the
// password.
func Open(ctx context.Context, url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("postgres: the database URL does not parse: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Close closes each connection of the pool.
func (db *DB) Close() { db.pool.Close() }

// dial opens one connection outside the pool. LISTEN and the session advisory
// lock need one session for all their time.
func (db *DB) dial(ctx context.Context) (*pgx.Conn, error) {
	return pgx.ConnectConfig(ctx, db.pool.Config().ConnConfig.Copy())
}

// Migrate applies each file of schema/ whose number is above the version of
// the database, in one transaction under the advisory lock of the schema. So
// two workers that start at the same time apply each file one time. A
// database with a version above SchemaVersion gives an error: a newer program
// made it.
func (db *DB) Migrate(ctx context.Context) error {
	files, err := migrations()
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockSchema); err != nil {
			return fmt.Errorf("postgres: the lock of the schema: %w", err)
		}
		if _, err := tx.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS horus;
CREATE TABLE IF NOT EXISTS horus.schema_version (
    version    integer PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
)`); err != nil {
			return fmt.Errorf("postgres: the table horus.schema_version: %w", err)
		}
		var have int
		if err := tx.QueryRow(ctx, "SELECT coalesce(max(version), 0) FROM horus.schema_version").Scan(&have); err != nil {
			return fmt.Errorf("postgres: read the schema version: %w", err)
		}
		if have > SchemaVersion {
			return fmt.Errorf("postgres: the database has the schema version %d, and this program knows only %d: use a newer horus", have, SchemaVersion)
		}
		for _, m := range files {
			if m.version <= have {
				continue
			}
			if _, err := tx.Exec(ctx, m.sql); err != nil {
				return fmt.Errorf("postgres: apply the schema file %s: %w", m.name, err)
			}
			if _, err := tx.Exec(ctx, "INSERT INTO horus.schema_version (version) VALUES ($1)", m.version); err != nil {
				return fmt.Errorf("postgres: record the schema version %d: %w", m.version, err)
			}
		}
		return nil
	})
}

// CheckSchema gives nil when the database has the schema version of this
// program. It is a part of the readiness check of horus-db.
func (db *DB) CheckSchema(ctx context.Context) error {
	var have int
	err := db.pool.QueryRow(ctx, "SELECT coalesce(max(version), 0) FROM horus.schema_version").Scan(&have)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && (pe.Code == "42P01" || pe.Code == "3F000") { // undefined_table, invalid_schema_name
		return errors.New("postgres: the database has no schema horus: the worker applies it at start")
	}
	if err != nil {
		return fmt.Errorf("postgres: read the schema version: %w", err)
	}
	if have != SchemaVersion {
		return fmt.Errorf("postgres: the database has the schema version %d, and this program needs %d", have, SchemaVersion)
	}
	return nil
}

// notUUID tells if err is the error of Postgres for an id that is not a UUID.
// The job store has no such job.
func notUUID(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "22P02" // invalid_text_representation
}

// ms gives the duration in whole milliseconds, for "$n * interval '1 millisecond'".
func ms(d time.Duration) int64 { return d.Milliseconds() }
