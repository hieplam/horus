// Tests of horus worker through the wiring of cmd/horus: the Postgres job
// store and broker of a test database, the real ComfyUI adapter and the fake
// ComfyUI of comfytest over HTTP. The loop has its own tests with the
// in-memory broker and a fake (internal/worker). These tests prove that the
// parts fit: the schema at the start, the gate over HTTP, the idempotent
// consumer on the Postgres broker, the heartbeat row and the worker lock.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
	"github.com/hieplam/horus/internal/postgres"
	"github.com/hieplam/horus/internal/postgres/pgtest"
	"github.com/hieplam/horus/internal/settings"
)

// lockedBuffer is the log of a worker.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// publishTwice is the Postgres broker with each message published two times.
type publishTwice struct{ *postgres.Broker }

func (b publishTwice) Publish(ctx context.Context, queue string, m jobdata.Message) error {
	if err := b.Broker.Publish(ctx, queue, m); err != nil {
		return err
	}
	return b.Broker.Publish(ctx, queue, m)
}

func eventually(t *testing.T, what string, within time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not in %s", what, within)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestWorkerOnPostgres: horus worker applies the schema to an empty database,
// writes its row, takes no delivery while ComfyUI does not answer, and then
// runs each of 3 jobs one time, though the broker delivers each message two
// times. A second worker names the first one and fails.
func TestWorkerOnPostgres(t *testing.T) {
	pgtest.Need(t)
	url := pgtest.URL(t)
	f := imageEnv(t)
	t.Setenv("HORUS_DATABASE_URL", url)
	t.Setenv("HORUS_WORKER_CONCURRENCY", "2")
	settings.Reset()
	f.SetDown(true)

	var log lockedBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runWorker(ctx, &log) }()

	db, err := postgres.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	eventually(t, "the worker applies the schema", 10*time.Second, func() bool { return db.CheckSchema(context.Background()) == nil })
	queue := jobs.New(postgres.NewStore(db), publishTwice{postgres.NewBroker(db, postgres.BrokerOptions{})}, jobs.Config{})
	eventually(t, "the row of the worker", 10*time.Second, func() bool {
		sum, err := queue.Summary(context.Background())
		return err == nil && sum.Worker != nil
	})
	sum, _ := queue.Summary(context.Background())
	if w := sum.Worker; w.Slots != 2 || w.PID != os.Getpid() || w.ID == "" {
		t.Errorf("the row of the worker: %+v", *w)
	}

	images := newImages("")
	var ids []string
	for seed := 1; seed <= 3; seed++ {
		p, err := images.Check(generate.Request{Workflow: "it-illustration", Subject: testSubject + " The picture contains no text.", Seed: int64(seed)})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(p)
		j, err := queue.Submit(context.Background(), jobdata.Job{Tool: "generate_image", Workflow: p.Workflow, Backend: "comfyui", Payload: raw})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, j.ID)
	}
	eventually(t, "the gate sees ComfyUI down", 10*time.Second, func() bool {
		return strings.Contains(log.String(), "ComfyUI does not answer: the worker takes no delivery")
	})
	for i, id := range ids {
		st, err := queue.Progress(context.Background(), id)
		if err != nil || st.State != jobdata.Queued || st.Position != i+1 || st.Total != 3 {
			t.Errorf("the job %d while ComfyUI is down: %+v, %v; want queued at %d of 3", i+1, st, err, i+1)
		}
	}
	if n, r := len(f.Jobs()), f.RefusedPrompts(); n+r != 0 {
		t.Errorf("while ComfyUI is down: %d submits and %d refused submits, want 0", n, r)
	}

	f.SetDown(false)
	eventually(t, "the 3 jobs succeeded, and the 6 deliveries ended", 30*time.Second, func() bool {
		for _, id := range ids {
			if st, err := queue.Progress(context.Background(), id); err != nil || st.State != jobdata.Succeeded {
				return false
			}
		}
		return strings.Count(log.String(), ": ran\n") == 3 && strings.Count(log.String(), ": skipped\n") == 3
	})
	if n := len(f.Jobs()); n != 3 {
		t.Errorf("%d submits for 3 jobs, each delivered two times; want 3", n)
	}

	second, stop2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop2()
	if err := runWorker(second, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "another horus worker runs: id "+sum.Worker.ID) {
		t.Errorf("a second worker: %v, want the error that names the first worker", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the worker after its stop: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the worker did not stop in 10 s")
	}
	if !strings.Contains(log.String(), "stopped: id "+sum.Worker.ID) {
		t.Errorf("the log has no stop line:\n%s", log.String())
	}
}

// TestWorkerTakesTheLockAgain: when the session of the worker lock ends, the
// worker holds the lock again on a new session in 10 s or less, and writes its
// heartbeat again. This is the readiness check of horus-worker: a heartbeat
// newer than 15 s, and a session that holds the worker lock. First only the
// session of the lock ends (pg_terminate_backend). Then each session of the
// worker ends, as a restart of horus-db does. A second worker still finds the
// lock taken.
func TestWorkerTakesTheLockAgain(t *testing.T) {
	pgtest.Need(t)
	url := pgtest.URL(t)
	imageEnv(t)
	t.Setenv("HORUS_DATABASE_URL", url)
	settings.Reset()

	var log lockedBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runWorker(ctx, &log) }()

	var first int
	eventually(t, "the worker takes the worker lock", 10*time.Second, func() bool {
		first = pgtest.WorkerLockHolder(t, url)
		return first != 0
	})
	db := pgtest.Open(t, url)
	queue := jobs.New(postgres.NewStore(db), postgres.NewBroker(db, postgres.BrokerOptions{}), jobs.Config{})

	// again ends the sessions with end, then waits until a new session holds
	// the worker lock and the row of the worker has a heartbeat after that.
	again := func(what string, end func() int) {
		t.Helper()
		before := pgtest.WorkerLockHolder(t, url)
		if n := end(); n == 0 {
			t.Fatalf("%s: no session ended", what)
		}
		ended := time.Now()
		var holder int
		for holder = pgtest.WorkerLockHolder(t, url); holder == 0 || holder == before; holder = pgtest.WorkerLockHolder(t, url) {
			if time.Since(ended) > 30*time.Second {
				t.Fatalf("%s: no session holds the worker lock 30 s after its session ended\nthe log of the worker:\n%s", what, log.String())
			}
			time.Sleep(50 * time.Millisecond)
		}
		if took := time.Since(ended); took > 10*time.Second {
			t.Errorf("%s: a session holds the worker lock again after %s, want 10 s or less", what, took.Round(time.Millisecond))
		} else {
			t.Logf("%s: the session %d holds the worker lock again after %s", what, holder, took.Round(time.Millisecond))
		}
		eventually(t, what+": a heartbeat after the new lock", 10*time.Second, func() bool {
			sum, err := queue.Summary(context.Background())
			return err == nil && sum.Worker != nil && sum.Worker.HeartbeatAt.After(ended)
		})
	}
	again("the session of the worker lock ends", func() int { return pgtest.EndWorkerLock(t, url) })
	again("each session of the worker ends", func() int { return pgtest.EndSessions(t, url) })

	sum, err := queue.Summary(context.Background())
	if err != nil || sum.Worker == nil {
		t.Fatalf("the summary: %+v, %v", sum.Worker, err)
	}
	second, stop2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop2()
	if err := runWorker(second, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "another horus worker runs: id "+sum.Worker.ID) {
		t.Errorf("a second worker after the lock came back: %v, want the error that names the first worker", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the worker after its stop: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the worker did not stop in 10 s")
	}
}

// TestWorkerNeedsTheDatabase: without HORUS_DATABASE_URL, horus worker exits
// with 1 and names the setting. A bad HORUS_WORKER_CONCURRENCY also exits 1.
func TestWorkerNeedsTheDatabase(t *testing.T) {
	t.Setenv("HORUS_ROOT", t.TempDir())
	t.Setenv("HORUS_DATABASE_URL", "")
	settings.Reset()
	t.Cleanup(settings.Reset)
	err := runWorker(context.Background(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "HORUS_DATABASE_URL is not set") {
		t.Errorf("no database URL: %v", err)
	}
	if code := run([]string{"worker"}); code != 1 {
		t.Errorf("horus worker without HORUS_DATABASE_URL: exit %d, want 1", code)
	}
	t.Setenv("HORUS_DATABASE_URL", "postgres://user@127.0.0.1:1/horus_test")
	t.Setenv("HORUS_WORKER_CONCURRENCY", "0")
	err = runWorker(context.Background(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "HORUS_WORKER_CONCURRENCY must be a whole number of 1 or more") {
		t.Errorf("HORUS_WORKER_CONCURRENCY=0: %v", err)
	}
	if code := run([]string{"worker", "extra"}); code != 2 {
		t.Errorf("horus worker extra: exit %d, want 2", code)
	}
}
