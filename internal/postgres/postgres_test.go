package postgres_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hieplam/horus/internal/brokertest"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/postgres"
	"github.com/hieplam/horus/internal/postgres/pgtest"
)

// open connects to the database of the URL and applies the schema.
func open(t testing.TB, url string) *postgres.DB {
	t.Helper()
	db, err := postgres.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}

// sqlConn is one connection to the database of the URL, for the SQL of a test.
func sqlConn(t testing.TB, url string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(context.Background()) })
	return c
}

func strings1(t testing.TB, c *pgx.Conn, sql string, args ...any) []string {
	t.Helper()
	rows, err := c.Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestSchema: the tables of the spec ("Data in horus-db"), with the columns of
// the spec in its order, the index on (idempotency_key, created_at), and the
// schema version of the program. A second Migrate changes nothing.
func TestSchema(t *testing.T) {
	url := pgtest.URL(t)
	db := open(t, url)
	c := sqlConn(t, url)
	want := map[string]string{
		"jobs": "id tool workflow backend work_queue resource_key payload request_hash idempotency_key state attempt " +
			"created_at deadline waiter_seen_at started_at finished_at worker_id backend_job_id cancel_requested_at " +
			"progress result error caller",
		"job_events":     "id job_id at type data",
		"workers":        "id host pid version slots started_at heartbeat_at loaded_resource_key",
		"messages":       "id queue body created_at visible_at deliveries",
		"schema_version": "version applied_at",
	}
	tables := strings1(t, c, "SELECT table_name::text FROM information_schema.tables WHERE table_schema = 'horus' ORDER BY 1")
	if got := strings.Join(tables, " "); got != "job_events jobs messages schema_version workers" {
		t.Errorf("the tables of the schema horus: %s", got)
	}
	for table, cols := range want {
		got := strings1(t, c, `SELECT column_name::text FROM information_schema.columns
WHERE table_schema = 'horus' AND table_name = $1 ORDER BY ordinal_position`, table)
		if strings.Join(got, " ") != cols {
			t.Errorf("the columns of horus.%s:\n got  %s\n want %s", table, strings.Join(got, " "), cols)
		}
	}
	def := strings1(t, c, "SELECT indexdef FROM pg_indexes WHERE schemaname = 'horus' AND indexname = 'jobs_idempotency_key'")
	if len(def) != 1 || !strings.HasSuffix(def[0], "(idempotency_key, created_at)") {
		t.Errorf("the index of the idempotency key: %q, want one index on (idempotency_key, created_at)", def)
	}
	if err := db.CheckSchema(context.Background()); err != nil {
		t.Errorf("CheckSchema after Migrate: %v", err)
	}
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("a second Migrate: %v", err)
	}
	versions := strings1(t, c, "SELECT version::text FROM horus.schema_version ORDER BY version")
	if !slices.Equal(versions, []string{"1"}) || postgres.SchemaVersion != 1 {
		t.Errorf("the schema versions after two Migrate: %q, and SchemaVersion %d; want [1] and 1", versions, postgres.SchemaVersion)
	}
}

// TestMigrateAtTheSameTime: 4 workers that start at the same time on an
// empty database each apply the schema with no error, and the database
// records the version one time (the advisory lock of the schema).
func TestMigrateAtTheSameTime(t *testing.T) {
	url := pgtest.URL(t)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db, err := postgres.Connect(context.Background(), url)
			if err != nil {
				t.Errorf("Connect %d: %v", i, err)
				return
			}
			defer db.Close()
			if err := db.Migrate(context.Background()); err != nil {
				t.Errorf("Migrate %d: %v", i, err)
			}
		}()
	}
	wg.Wait()
	versions := strings1(t, sqlConn(t, url), "SELECT version::text FROM horus.schema_version")
	if !slices.Equal(versions, []string{"1"}) {
		t.Errorf("the schema versions: %q, want [1]", versions)
	}
}

// TestCheckSchema: a database without the schema, and a database that a
// newer program migrated, fail the check. Migrate refuses the newer one.
func TestCheckSchema(t *testing.T) {
	ctx := context.Background()
	url := pgtest.URL(t)
	db, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.CheckSchema(ctx); err == nil || !strings.Contains(err.Error(), "no schema horus") {
		t.Errorf("CheckSchema of an empty database: %v, want the error no schema horus", err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlConn(t, url).Exec(ctx, "INSERT INTO horus.schema_version (version) VALUES (2)"); err != nil {
		t.Fatal(err)
	}
	if err := db.CheckSchema(ctx); err == nil || !strings.Contains(err.Error(), "schema version 2") {
		t.Errorf("CheckSchema of a newer schema: %v, want an error with the version 2", err)
	}
	if err := db.Migrate(ctx); err == nil || !strings.Contains(err.Error(), "use a newer horus") {
		t.Errorf("Migrate of a newer schema: %v, want an error that asks for a newer horus", err)
	}
}

// TestConnectError: an error of Connect does not show the password.
func TestConnectError(t *testing.T) {
	const secret = "s3cret-pw-for-the-test"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := postgres.Connect(ctx, "postgres://horus:"+secret+"@127.0.0.1:1/horus?sslmode=disable&connect_timeout=2")
	if err == nil {
		t.Fatal("Connect to a closed port: no error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("the error shows the password: %v", err)
	}
}

// TestSummaryWorker: Summary gives the worker row with the newest heartbeat.
func TestSummaryWorker(t *testing.T) {
	url := pgtest.URL(t)
	s := postgres.NewStore(open(t, url))
	at := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	_, err := sqlConn(t, url).Exec(context.Background(), `INSERT INTO horus.workers
(id, host, pid, version, slots, started_at, heartbeat_at, loaded_resource_key) VALUES
('worker-old', 'horus-worker', 7, '0.9.0', 1, $1, $2, NULL),
('worker-new', 'horus-worker', 8, '0.10.0', 2, $1, $3, 'flux-3-klein')`, at, at.Add(time.Second), at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := s.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := jobdata.Worker{ID: "worker-new", Host: "horus-worker", PID: 8, Version: "0.10.0", Slots: 2,
		StartedAt: at, HeartbeatAt: at.Add(time.Minute), LoadedResourceKey: "flux-3-klein"}
	if w := sum.Worker; w == nil || w.ID != want.ID || w.Host != want.Host || w.PID != want.PID ||
		w.Version != want.Version || w.Slots != want.Slots || !w.StartedAt.Equal(want.StartedAt) ||
		!w.HeartbeatAt.Equal(want.HeartbeatAt) || w.LoadedResourceKey != want.LoadedResourceKey {
		t.Errorf("Summary worker: %+v, want %+v", w, want)
	}
}

// TestBrokerDefaults: the program uses the times of the spec: a visibility
// time of 60 s, an extension every 20 s, and a read of the queue every 5 s.
func TestBrokerDefaults(t *testing.T) {
	got := postgres.Options(postgres.NewBroker(nil, postgres.BrokerOptions{}))
	want := postgres.BrokerOptions{Visibility: 60 * time.Second, Extend: 20 * time.Second, Poll: 5 * time.Second}
	if got != want {
		t.Errorf("the options of the broker: %+v, want %+v", got, want)
	}
}

// lateSecond is a broker whose second Consume starts after delay. The contract
// check of two consumers starts them one after the other, and a slow start of
// the second one must not change the result of the stub check.
type lateSecond struct {
	*postgres.Broker
	delay time.Duration
	calls atomic.Int32
}

func (l *lateSecond) Consume(ctx context.Context, queue string, prefetch int) (<-chan jobdata.Delivery, error) {
	if l.calls.Add(1) == 2 {
		time.Sleep(l.delay)
	}
	return l.Broker.Consume(ctx, queue, prefetch)
}

// stubLockKey is the key of the advisory lock that the stub claim waits on.
const stubLockKey = 180227

// TestStubNoSkipLocked is a stub check: a claim without FOR UPDATE SKIP LOCKED
// gives one message to two consumers, and the contract check of two
// consumers fails. The stub claim waits on a shared advisory lock inside its
// statement. The test holds the exclusive lock of the same key, waits until
// two claims wait on it (an event in pg_locks, not a time window), and then
// releases it. So the first claims of the two consumers run at the same time
// on each run, also when the second consumer starts late.
func TestStubNoSkipLocked(t *testing.T) {
	url := pgtest.URL(t)
	base := postgres.NewBroker(open(t, url), postgres.BrokerOptions{Visibility: time.Second})
	stub := postgres.ClaimSQL
	for _, r := range []struct{ old, new string }{
		{"FOR UPDATE SKIP LOCKED", ""},
		{"WHERE queue = $1", "WHERE queue = $1 AND (SELECT true FROM pg_advisory_xact_lock_shared(" + strconv.Itoa(stubLockKey) + "))"},
	} {
		if !strings.Contains(stub, r.old) {
			t.Fatalf("the claim SQL has no %q", r.old)
		}
		stub = strings.Replace(stub, r.old, r.new, 1)
	}
	postgres.SetClaim(base, stub)

	ctx := context.Background()
	hold := sqlConn(t, url)
	if _, err := hold.Exec(ctx, "SELECT pg_advisory_lock($1)", stubLockKey); err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			_, _ = hold.Exec(ctx, "SELECT pg_advisory_unlock($1)", stubLockKey)
		}
	}
	defer release()

	result := make(chan []string, 1)
	go func() {
		result <- brokertest.Failures(t, "two_consumers_never_share", &lateSecond{Broker: base, delay: time.Second})
	}()

	watch := sqlConn(t, url)
	deadline := time.Now().Add(brokertest.Timeout)
	for {
		var waiting int
		err := watch.QueryRow(ctx, "SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND objid = $1 AND mode = 'ShareLock' AND NOT granted", int64(stubLockKey)).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d claims wait on the lock in %s, want 2", waiting, brokertest.Timeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
	release()

	got := <-result
	if len(got) == 0 || !strings.Contains(got[0], "came 2 times, want 1") {
		t.Errorf("a claim without SKIP LOCKED: the failures %q, want a message that came 2 times", got)
	}
}

// TestStubNoExtension is a stub check: a consumer that does not extend the
// visibility time while a delivery runs gives the message again, and the
// contract check of a long delivery fails. The extension comes after 1 h, so
// it never comes in the check.
func TestStubNoExtension(t *testing.T) {
	b := postgres.NewBroker(open(t, pgtest.URL(t)), postgres.BrokerOptions{Visibility: time.Second, Extend: time.Hour})
	got := brokertest.Failures(t, "long_delivery", b)
	if len(got) == 0 || !strings.Contains(got[0], "a delivery of job-1 came") {
		t.Errorf("a broker with no extension: the failures %q, want a delivery of job-1 that came again", got)
	}
}

func message(i int) jobdata.Message {
	return jobdata.Message{V: 1, JobID: "job-" + string(rune('0'+i)), Tool: "generate_image", Backend: "comfyui"}
}

func nextDelivery(t testing.TB, ch <-chan jobdata.Delivery, within time.Duration) jobdata.Delivery {
	t.Helper()
	select {
	case d, ok := <-ch:
		if !ok {
			t.Fatal("the deliveries closed")
		}
		return d
	case <-time.After(within):
		t.Fatalf("no delivery in %s", within)
	}
	return jobdata.Delivery{}
}

// TestStopMakesMessagesVisible: when a consumer stops, its messages come to
// another consumer at once, not after the visibility time of 60 s. An ack
// after the stop gives an error and deletes nothing.
func TestStopMakesMessagesVisible(t *testing.T) {
	ctx := context.Background()
	b := postgres.NewBroker(open(t, pgtest.URL(t)), postgres.BrokerOptions{})
	if err := b.Publish(ctx, "jobs.stop", message(1)); err != nil {
		t.Fatal(err)
	}
	cctx, stop := context.WithCancel(ctx)
	ch, err := b.Consume(cctx, "jobs.stop", 1)
	if err != nil {
		t.Fatal(err)
	}
	d := nextDelivery(t, ch, 10*time.Second)
	stop()
	for range ch {
	}
	if err := d.Ack(ctx); err == nil {
		t.Error("an ack after the consumer stopped: no error")
	}
	ctx2, stop2 := context.WithCancel(ctx)
	defer stop2()
	ch2, err := b.Consume(ctx2, "jobs.stop", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := nextDelivery(t, ch2, 2*time.Second).Ack(ctx); err != nil {
		t.Errorf("the ack of the second consumer: %v", err)
	}
}

// TestAckAfterTheVisibilityTime: a delivery whose visibility time passed with
// no extension comes to another consumer. Then the ack of the first delivery
// gives an error and deletes nothing, and the ack of the second one works.
func TestAckAfterTheVisibilityTime(t *testing.T) {
	ctx := context.Background()
	b := postgres.NewBroker(open(t, pgtest.URL(t)), postgres.BrokerOptions{Visibility: 300 * time.Millisecond, Extend: time.Hour})
	if err := b.Publish(ctx, "jobs.late", message(1)); err != nil {
		t.Fatal(err)
	}
	c1, stop1 := context.WithCancel(ctx)
	defer stop1()
	ch1, err := b.Consume(c1, "jobs.late", 1)
	if err != nil {
		t.Fatal(err)
	}
	d1 := nextDelivery(t, ch1, 10*time.Second)
	time.Sleep(600 * time.Millisecond)
	c2, stop2 := context.WithCancel(ctx)
	defer stop2()
	ch2, err := b.Consume(c2, "jobs.late", 1)
	if err != nil {
		t.Fatal(err)
	}
	d2 := nextDelivery(t, ch2, 2*time.Second)
	if err := d1.Ack(ctx); err == nil || !strings.Contains(err.Error(), "came again to another consumer") {
		t.Errorf("the ack of the first delivery: %v, want the error that the message came again", err)
	}
	if err := d2.Ack(ctx); err != nil {
		t.Errorf("the ack of the second delivery: %v", err)
	}
}

// TestListenAfterTheConnectionBreaks: when Postgres ends the LISTEN
// connection of a listener, the listener opens a new one, and an event that
// Notify sends after that comes.
func TestListenAfterTheConnectionBreaks(t *testing.T) {
	ctx := context.Background()
	url := pgtest.URL(t)
	b := postgres.NewBroker(open(t, url), postgres.BrokerOptions{Poll: 200 * time.Millisecond})
	lctx, stop := context.WithCancel(ctx)
	defer stop()
	events, err := b.Listen(lctx)
	if err != nil {
		t.Fatal(err)
	}
	c := sqlConn(t, url)
	ended := strings1(t, c, `SELECT pg_terminate_backend(pid)::text FROM pg_stat_activity
WHERE datname = current_database() AND query = 'LISTEN horus_events'`)
	if !slices.Equal(ended, []string{"true"}) {
		t.Fatalf("pg_terminate_backend of the LISTEN connection: %q, want one true", ended)
	}
	deadline := time.After(10 * time.Second)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case e := <-events:
			if e.JobID == "job-after" {
				return
			}
		case <-tick.C:
			if err := b.Notify(ctx, jobdata.Event{V: 1, JobID: "job-after", Type: "job.running", At: time.Now()}); err != nil {
				t.Fatal(err)
			}
		case <-deadline:
			t.Fatal("no event in 10 s after the LISTEN connection broke")
		}
	}
}
