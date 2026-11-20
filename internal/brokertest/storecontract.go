package brokertest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// NewStoreFunc makes an empty job store for one check of RunStore. now is the
// clock of the store. events gives the types of the event rows of a job, in
// order. endLock ends the session of the worker lock, as a restart of the
// database does: the holder loses the lock, and the lock is free.
type NewStoreFunc func(t testing.TB, now func() time.Time) (s jobs.Store, events func(id string) []string, endLock func())

// storeCheck is one check of the store contract.
type storeCheck struct {
	name string
	run  func(t testing.TB, s jobs.Store, events func(id string) []string, clk *clock, endLock func())
}

// storeChecks are the checks of the store contract, in the order of RunStore.
var storeChecks = []storeCheck{
	{"insert_get", checkInsertGet},
	{"move_from_expected_state", checkMove},
	{"one_move_wins", checkOneMoveWins},
	{"touch", checkTouch},
	{"position_and_queued", checkPosition},
	{"sweep", checkSweep},
	{"summary", checkSummary},
	{"worker_lock", checkWorkerLock},
	{"worker_lock_lost", checkWorkerLockLost},
	{"heartbeat", checkHeartbeat},
	{"delete_old", checkDeleteOld},
}

// StoreNames are the names of the checks of the store contract.
func StoreNames() []string {
	var out []string
	for _, c := range storeChecks {
		out = append(out, c.name)
	}
	return out
}

// RunStore runs each check of the store contract as a subtest of t, on a new
// store from newStore for each check. The in-memory store and the Postgres
// store pass it: they keep the rules of jobs.Store in the same way.
func RunStore(t *testing.T, newStore NewStoreFunc) {
	for _, c := range storeChecks {
		t.Run(c.name, func(t *testing.T) {
			clk := &clock{now: t0}
			s, events, endLock := newStore(t, clk.Now)
			c.run(t, s, events, clk, endLock)
		})
	}
}

// storeFailures runs the store check named name on a new store, and gives its
// failures instead of failing t.
func storeFailures(t *testing.T, name string, newStore NewStoreFunc) []string {
	t.Helper()
	for _, c := range storeChecks {
		if c.name == name {
			clk := &clock{now: t0}
			s, events, endLock := newStore(t, clk.Now)
			return record(t, func(r testing.TB) { c.run(r, s, events, clk, endLock) })
		}
	}
	t.Fatalf("no store check %s", name)
	return nil
}

// t0 is the time of the clock at the start of each check: 12:00 UTC. The
// times of the checks are whole seconds, because Postgres keeps microseconds.
var t0 = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

// clock is the clock of the store in a check.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// uid is the id number i: a UUID, because Postgres keeps the id as a UUID.
func uid(i int) string { return fmt.Sprintf("019aa000-0000-7000-8000-%012d", i) }

// notAUUID is an id that is not a UUID. The store has no such job.
const notAUUID = "job-1"

// queuedJob is a queued job with the id number i, made at the time created.
func queuedJob(i int, created time.Time) jobdata.Job {
	return jobdata.Job{ID: uid(i), Tool: "generate_image", Workflow: "it-illustration", Backend: "comfyui",
		WorkQueue: "jobs.comfyui", State: jobdata.Queued, CreatedAt: created, Deadline: created.Add(2 * time.Hour),
		WaiterSeenAt: created}
}

func insert(t testing.TB, s jobs.Store, j jobdata.Job) {
	t.Helper()
	if err := s.Insert(context.Background(), j); err != nil {
		t.Fatalf("Insert %s: %v", j.ID, err)
	}
}

func get(t testing.TB, s jobs.Store, id string) jobdata.Job {
	t.Helper()
	j, err := s.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	return j
}

// sameJSON tells if a and b hold the same JSON value. Two empty values are the
// same.
func sameJSON(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}

func jsonBytes(v any) []byte {
	if reflect.ValueOf(v).IsNil() {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

// sameJob checks each field of the job got against want. A time is the same
// instant, and JSON is the same value.
func sameJob(t testing.TB, what string, got, want jobdata.Job) {
	t.Helper()
	var diff []string
	field := func(name string, same bool) {
		if !same {
			diff = append(diff, name)
		}
	}
	field("id", got.ID == want.ID)
	field("tool", got.Tool == want.Tool)
	field("workflow", got.Workflow == want.Workflow)
	field("backend", got.Backend == want.Backend)
	field("work_queue", got.WorkQueue == want.WorkQueue)
	field("resource_key", got.ResourceKey == want.ResourceKey)
	field("payload", sameJSON(got.Payload, want.Payload))
	field("request_hash", got.RequestHash == want.RequestHash)
	field("idempotency_key", got.IdempotencyKey == want.IdempotencyKey)
	field("state", got.State == want.State)
	field("attempt", got.Attempt == want.Attempt)
	field("created_at", got.CreatedAt.Equal(want.CreatedAt))
	field("deadline", got.Deadline.Equal(want.Deadline))
	field("waiter_seen_at", got.WaiterSeenAt.Equal(want.WaiterSeenAt))
	field("started_at", got.StartedAt.Equal(want.StartedAt))
	field("finished_at", got.FinishedAt.Equal(want.FinishedAt))
	field("worker_id", got.WorkerID == want.WorkerID)
	field("backend_job_id", got.BackendJobID == want.BackendJobID)
	field("cancel_requested_at", got.CancelRequested.Equal(want.CancelRequested))
	field("progress", reflect.DeepEqual(got.Progress, want.Progress))
	field("result", sameJSON(got.Result, want.Result))
	field("error", sameJSON(jsonBytes(got.Error), jsonBytes(want.Error)))
	field("caller", got.Caller == want.Caller)
	if len(diff) > 0 {
		t.Errorf("%s: the job %s differs in %s:\n got  %+v\n want %+v", what, want.ID, strings.Join(diff, ", "), got, want)
	}
}

func eventsAre(t testing.TB, events func(string) []string, id string, want ...string) {
	t.Helper()
	if got := events(id); !slices.Equal(got, want) {
		t.Errorf("the events of the job %s: %q, want %q", id, got, want)
	}
}

func notFound(t testing.TB, what string, err error) {
	t.Helper()
	if !errors.Is(err, jobdata.ErrNotFound) {
		t.Errorf("%s: error %v, want jobdata.ErrNotFound", what, err)
	}
}

// checkInsertGet: Get gives each field that Insert wrote, an empty field
// stays empty, and Insert writes the event job.queued. A second Insert with
// the same id fails and changes nothing. An unknown id gives ErrNotFound.
func checkInsertGet(t testing.TB, s jobs.Store, events func(string) []string, _ *clock, _ func()) {
	full := jobdata.Job{ID: uid(1), Tool: "generate_image", Workflow: "it-illustration", Backend: "comfyui",
		WorkQueue: "jobs.comfyui", ResourceKey: "flux-3-klein", Payload: json.RawMessage(`{"subject":"a cat on a desk","seed":7}`),
		RequestHash: "ab12", IdempotencyKey: "k1:cd34", State: jobdata.Running, Attempt: 2,
		CreatedAt: t0.Add(-10 * time.Minute), Deadline: t0.Add(2 * time.Hour), WaiterSeenAt: t0.Add(-5 * time.Second),
		StartedAt: t0.Add(-time.Minute), FinishedAt: t0, WorkerID: "worker-1", BackendJobID: "prompt-9",
		CancelRequested: t0.Add(-30 * time.Second), Progress: &jobdata.Progress{Value: 3, Total: 10, Message: "step 3 of 10"},
		Result: json.RawMessage(`{"path":"out/cat.png"}`),
		Error:  &jobdata.JobError{Code: "WORKER_LOST", Message: "the worker stopped", Details: map[string]any{"worker_id": "worker-0"}},
		Caller: "agent-1"}
	empty := jobdata.Job{ID: uid(2), State: jobdata.Queued, CreatedAt: t0}
	for _, j := range []jobdata.Job{full, empty} {
		insert(t, s, j)
		sameJob(t, "Get after Insert", get(t, s, j.ID), j)
	}
	if err := s.Insert(context.Background(), queuedJob(2, t0)); err == nil {
		t.Error("a second Insert with the same id: no error")
	}
	sameJob(t, "Get after a second Insert with the same id", get(t, s, uid(2)), empty)
	for _, id := range []string{uid(999), notAUUID} {
		_, err := s.Get(context.Background(), id)
		notFound(t, "Get "+id, err)
	}
	eventsAre(t, events, uid(1), "job.queued")
	eventsAre(t, events, uid(2), "job.queued")
}

// checkMove: Move changes the job only from one of the states from, as
// jobdata.Apply does, and writes one event of the type of the change. A move
// from another state and a move of an unknown id give false and change
// nothing. A change with no time gets the time of the clock.
func checkMove(t testing.TB, s jobs.Store, events func(string) []string, clk *clock, _ func()) {
	ctx := context.Background()
	move := func(id string, from []jobdata.State, to jobdata.State, c jobdata.Change, want bool) {
		t.Helper()
		before := get(t, s, id)
		ok, err := s.Move(ctx, id, from, to, c)
		if err != nil {
			t.Fatalf("Move %s from %v to %s: %v", id, from, to, err)
		}
		if ok != want {
			t.Fatalf("Move %s from %v to %s, with the job in %s: %v, want %v", id, from, to, before.State, ok, want)
		}
		expect := before
		if want {
			if c.At.IsZero() {
				c.At = clk.Now()
			}
			expect = jobdata.Apply(before, to, c)
		}
		sameJob(t, fmt.Sprintf("Get after Move from %v to %s", from, to), get(t, s, id), expect)
	}
	q, r := []jobdata.State{jobdata.Queued}, []jobdata.State{jobdata.Running}
	sec := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Second) }

	insert(t, s, queuedJob(1, t0.Add(-time.Minute)))
	move(uid(1), r, jobdata.Succeeded, jobdata.Change{At: sec(0)}, false)
	move(uid(1), q, jobdata.Running, jobdata.Change{At: sec(1), WorkerID: "worker-1"}, true)
	move(uid(1), q, jobdata.Running, jobdata.Change{At: sec(2), WorkerID: "worker-2"}, false)
	move(uid(1), r, jobdata.Running, jobdata.Change{At: sec(3), Event: jobdata.EventProgress,
		Progress: &jobdata.Progress{Value: 3, Total: 10, Message: "step 3 of 10"}}, true)
	move(uid(1), r, jobdata.Running, jobdata.Change{At: sec(4), BackendJobID: "prompt-9"}, true)
	move(uid(1), r, jobdata.Running, jobdata.Change{At: sec(5), CancelRequested: true, Event: jobdata.EventCancelRequested}, true)
	move(uid(1), []jobdata.State{jobdata.Queued, jobdata.Running}, jobdata.Succeeded,
		jobdata.Change{At: sec(6), Result: json.RawMessage(`{"path":"out/cat.png"}`)}, true)
	move(uid(1), r, jobdata.Failed, jobdata.Change{At: sec(7)}, false)
	eventsAre(t, events, uid(1), "job.queued", "job.running", "job.progress", "job.running", "job.cancel_requested", "job.succeeded")

	insert(t, s, queuedJob(2, t0.Add(-time.Minute)))
	move(uid(2), q, jobdata.Failed, jobdata.Change{At: sec(8), Error: &jobdata.JobError{Code: "WORKER_LOST",
		Message: "the worker stopped", Details: map[string]any{"worker_id": "worker-0"}}}, true)
	eventsAre(t, events, uid(2), "job.queued", "job.failed")

	insert(t, s, queuedJob(3, t0.Add(-time.Minute)))
	clk.Set(sec(9))
	move(uid(3), q, jobdata.Cancelled, jobdata.Change{}, true)
	if got := get(t, s, uid(3)).FinishedAt; !got.Equal(sec(9)) {
		t.Errorf("a move with no time: finished_at %s, want the time of the clock %s", got, sec(9))
	}

	for _, id := range []string{uid(999), notAUUID} {
		ok, err := s.Move(ctx, id, q, jobdata.Running, jobdata.Change{At: sec(10)})
		if ok || err != nil {
			t.Errorf("Move of the unknown job %s: %v, %v, want false and no error", id, ok, err)
		}
	}
}

// checkOneMoveWins: of 8 moves of one job from queued to running at the same
// time, exactly one moves it (the idempotent consumer).
func checkOneMoveWins(t testing.TB, s jobs.Store, events func(string) []string, _ *clock, _ func()) {
	insert(t, s, queuedJob(1, t0))
	const n = 8
	var wg sync.WaitGroup
	results := make(chan bool, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.Move(context.Background(), uid(1), []jobdata.State{jobdata.Queued}, jobdata.Running,
				jobdata.Change{At: t0, WorkerID: fmt.Sprintf("worker-%d", i)})
			if err != nil {
				t.Errorf("Move %d: %v", i, err)
			}
			results <- ok
		}()
	}
	wg.Wait()
	close(results)
	moved := 0
	for ok := range results {
		if ok {
			moved++
		}
	}
	if moved != 1 {
		t.Errorf("%d of %d moves from queued to running at the same time moved the job, want 1", moved, n)
	}
	if j := get(t, s, uid(1)); j.State != jobdata.Running || j.Attempt != 1 {
		t.Errorf("the job is %s with the attempt %d, want running with the attempt 1", j.State, j.Attempt)
	}
	eventsAre(t, events, uid(1), "job.queued", "job.running")
}

// checkTouch: Touch writes the time of the clock to waiter_seen_at, and
// changes nothing else. An unknown id gives ErrNotFound.
func checkTouch(t testing.TB, s jobs.Store, _ func(string) []string, clk *clock, _ func()) {
	j := queuedJob(1, t0.Add(-time.Minute))
	insert(t, s, j)
	clk.Set(t0.Add(10 * time.Second))
	if err := s.Touch(context.Background(), j.ID); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	j.WaiterSeenAt = t0.Add(10 * time.Second)
	sameJob(t, "Get after Touch", get(t, s, j.ID), j)
	for _, id := range []string{uid(999), notAUUID} {
		notFound(t, "Touch "+id, s.Touch(context.Background(), id))
	}
}

// checkPosition: the place of a queued job is its place among the queued jobs
// by the time they were made, oldest first, and not by the order of Insert. A
// job that is not queued has the place 0.
func checkPosition(t testing.TB, s jobs.Store, _ func(string) []string, _ *clock, _ func()) {
	ctx := context.Background()
	insert(t, s, queuedJob(3, t0.Add(-10*time.Second)))
	insert(t, s, queuedJob(1, t0.Add(-30*time.Second)))
	insert(t, s, queuedJob(2, t0.Add(-20*time.Second)))
	insert(t, s, queuedJob(4, t0.Add(-40*time.Second)))
	if ok, err := s.Move(ctx, uid(4), []jobdata.State{jobdata.Queued}, jobdata.Running, jobdata.Change{At: t0}); !ok || err != nil {
		t.Fatalf("Move to running: %v, %v", ok, err)
	}
	for i, want := range []int{1, 2, 3, 0} {
		pos, total, err := s.Position(ctx, uid(i+1))
		if err != nil || pos != want || total != 3 {
			t.Errorf("Position of the job %d: %d of %d, %v; want %d of 3", i+1, pos, total, err, want)
		}
	}
	if n, err := s.Queued(ctx); err != nil || n != 3 {
		t.Errorf("Queued: %d, %v; want 3", n, err)
	}
	for _, id := range []string{uid(999), notAUUID} {
		_, _, err := s.Position(ctx, id)
		notFound(t, "Position "+id, err)
	}
}

// checkSweep: the sweep moves a queued job after its deadline to expired, and
// a queued job with no waiter for more than jobdata.AbandonAfter to
// abandoned, at the time of the clock, with the event. Expired comes first.
// It changes no other job, and a second sweep changes nothing.
func checkSweep(t testing.TB, s jobs.Store, events func(string) []string, _ *clock, _ func()) {
	ctx := context.Background()
	job := func(i int, deadline, waiter time.Time) jobdata.Job {
		j := queuedJob(i, t0.Add(-3*time.Hour))
		j.Deadline, j.WaiterSeenAt = deadline, waiter
		return j
	}
	later := t0.Add(time.Hour)
	jobsIn := []jobdata.Job{
		job(1, t0.Add(-time.Second), t0.Add(-time.Second)),       // expired
		job(2, t0.Add(-time.Second), t0.Add(-2*time.Minute)),     // expired, not abandoned
		job(3, later, t0.Add(-jobdata.AbandonAfter-time.Second)), // abandoned
		job(4, later, t0.Add(-jobdata.AbandonAfter)),             // stays: exactly AbandonAfter
		job(5, later, time.Time{}),                               // stays: no waiter time
		job(6, t0, t0),                                           // stays: the deadline is now
		job(7, t0.Add(-time.Hour), t0.Add(-time.Hour)),           // running: stays
	}
	for _, j := range jobsIn {
		insert(t, s, j)
	}
	if ok, err := s.Move(ctx, uid(7), []jobdata.State{jobdata.Queued}, jobdata.Running, jobdata.Change{At: t0.Add(-time.Hour)}); !ok || err != nil {
		t.Fatalf("Move to running: %v, %v", ok, err)
	}
	expired, abandoned, err := s.Sweep(ctx)
	if err != nil || expired != 2 || abandoned != 1 {
		t.Errorf("Sweep: %d expired, %d abandoned, %v; want 2 and 1", expired, abandoned, err)
	}
	for i, want := range []jobdata.State{jobdata.Expired, jobdata.Expired, jobdata.Abandoned, jobdata.Queued,
		jobdata.Queued, jobdata.Queued, jobdata.Running} {
		j := get(t, s, uid(i+1))
		if j.State != want {
			t.Errorf("the job %d after Sweep: %s, want %s", i+1, j.State, want)
		}
		if want.Ended() && !j.FinishedAt.Equal(t0) {
			t.Errorf("the job %d after Sweep: finished_at %s, want the time of the clock %s", i+1, j.FinishedAt, t0)
		}
	}
	eventsAre(t, events, uid(1), "job.queued", "job.expired")
	eventsAre(t, events, uid(3), "job.queued", "job.abandoned")
	eventsAre(t, events, uid(4), "job.queued")
	if expired, abandoned, err := s.Sweep(ctx); err != nil || expired+abandoned != 0 {
		t.Errorf("a second Sweep: %d expired, %d abandoned, %v; want 0 and 0", expired, abandoned, err)
	}
}

// checkSummary: the counts and the lists of the Horus UI. "Today" starts at
// 00:00 of the day of the clock.
func checkSummary(t testing.TB, s jobs.Store, _ func(string) []string, _ *clock, _ func()) {
	ended := func(i int, state jobdata.State, started, finished time.Time, key string) jobdata.Job {
		j := queuedJob(i, started.Add(-time.Minute))
		j.State, j.StartedAt, j.FinishedAt, j.IdempotencyKey, j.Attempt = state, started, finished, key, 1
		return j
	}
	running := func(i int, created, started time.Time) jobdata.Job {
		j := queuedJob(i, created)
		j.State, j.StartedAt, j.Attempt = jobdata.Running, started, 1
		return j
	}
	q2 := queuedJob(2, t0.Add(-2*time.Minute))
	q2.IdempotencyKey = "k1:same"
	all := []jobdata.Job{
		queuedJob(1, t0.Add(-3*time.Minute)), q2,
		running(4, t0.Add(-9*time.Minute), t0.Add(-40*time.Second)), // inserted before the job 3, starts after it
		running(3, t0.Add(-10*time.Minute), t0.Add(-50*time.Second)),
	}
	// The jobs 101 to 111 succeeded today. The job i runs i-100 seconds.
	for i := 1; i <= 11; i++ {
		started := t0.Add(-2*time.Hour + time.Duration(i)*5*time.Minute)
		key := ""
		switch i {
		case 1:
			key = "k1:same"
		case 2:
			key = "f1:alone"
		}
		all = append(all, ended(100+i, jobdata.Succeeded, started, started.Add(time.Duration(i)*time.Second), key))
	}
	yesterday := t0.Add(-24 * time.Hour)
	all = append(all,
		ended(201, jobdata.Succeeded, yesterday, yesterday.Add(100*time.Second), "k1:same"), // yesterday
		ended(202, jobdata.Failed, t0.Add(-31*time.Minute), t0.Add(-30*time.Minute), ""),
		ended(203, jobdata.Failed, t0.Add(-13*time.Hour-time.Minute), t0.Add(-13*time.Hour), ""), // 23:00 yesterday
		ended(204, jobdata.Cancelled, t0.Add(-21*time.Minute), t0.Add(-20*time.Minute), ""),
	)
	for _, j := range all {
		insert(t, s, j)
	}
	sum, err := s.Summary(context.Background())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	counts := func(name string, got, want int) {
		if got != want {
			t.Errorf("Summary %s: %d, want %d", name, got, want)
		}
	}
	counts("queued", sum.Queued, 2)
	counts("running", sum.Running, 2)
	counts("succeeded_today", sum.SucceededToday, 11)
	counts("failed_today", sum.FailedToday, 1)
	counts("same_key_today", sum.SameKeyToday, 2)
	if math.Abs(sum.MeanRunSeconds-6.5) > 1e-9 {
		t.Errorf("Summary mean_run_seconds: %v, want 6.5 (the last 10 succeeded jobs run 2 to 11 s)", sum.MeanRunSeconds)
	}
	ids := func(name string, list []jobdata.Job, want ...int) {
		var got, w []string
		for _, j := range list {
			got = append(got, j.ID)
		}
		for _, i := range want {
			w = append(w, uid(i))
		}
		if !slices.Equal(got, w) {
			t.Errorf("Summary %s: %q, want %q", name, got, w)
		}
	}
	ids("running_jobs", sum.RunningJobs, 3, 4)
	ids("queued_jobs", sum.QueuedJobs, 1, 2)
	ids("recent", sum.Recent, 204, 202, 111, 110, 109, 108, 107, 106, 105, 104)
	if sum.Worker != nil {
		t.Errorf("Summary worker: %+v, want nil: the store has no worker row", sum.Worker)
	}
	if len(sum.RunningJobs) > 0 {
		sameJob(t, "Summary running_jobs[0]", sum.RunningJobs[0], all[3])
	}
}

// checkWorkerLock: one holder of the worker lock at a time. Check gives nil
// while the lock holds it, and ErrLockLost after the release. A release lets
// the next one take the lock, and a second release does nothing.
func checkWorkerLock(t testing.TB, s jobs.Store, _ func(string) []string, _ *clock, _ func()) {
	ctx := context.Background()
	first, err := s.WorkerLock(ctx)
	if err != nil {
		t.Fatalf("the first WorkerLock: %v", err)
	}
	if err := first.Check(ctx); err != nil {
		t.Errorf("Check of the first lock while it holds the lock: %v, want nil", err)
	}
	if l2, err := s.WorkerLock(ctx); !errors.Is(err, jobdata.ErrWorkerLocked) {
		if l2.Release != nil {
			l2.Release()
		}
		t.Errorf("a second WorkerLock while the first holds it: error %v, want jobdata.ErrWorkerLocked", err)
	}
	first.Release()
	if err := first.Check(ctx); !errors.Is(err, jobdata.ErrLockLost) {
		t.Errorf("Check of the first lock after its release: %v, want jobdata.ErrLockLost", err)
	}
	again, err := s.WorkerLock(ctx)
	if err != nil {
		t.Fatalf("WorkerLock after the release: %v", err)
	}
	first.Release()
	if err := again.Check(ctx); err != nil {
		t.Errorf("Check of the next lock after a second release of the first: %v, want nil", err)
	}
	again.Release()
	again.Release()
}

// checkWorkerLockLost: when the session of the lock ends (a restart of the
// database), Check of the lock gives ErrLockLost, and the lock is free. The
// next WorkerLock takes it on a new session, and while it holds the lock, a
// third WorkerLock gives ErrWorkerLocked. The release of the lost lock does
// not free the lock of the next holder.
func checkWorkerLockLost(t testing.TB, s jobs.Store, _ func(string) []string, _ *clock, endLock func()) {
	ctx := context.Background()
	lost, err := s.WorkerLock(ctx)
	if err != nil {
		t.Fatalf("the first WorkerLock: %v", err)
	}
	defer lost.Release()
	if err := lost.Check(ctx); err != nil {
		t.Fatalf("Check before the session ends: %v, want nil", err)
	}
	endLock()
	if err := lost.Check(ctx); !errors.Is(err, jobdata.ErrLockLost) {
		t.Errorf("Check after the session of the lock ended: %v, want jobdata.ErrLockLost", err)
	}
	next, err := s.WorkerLock(ctx)
	if err != nil {
		t.Fatalf("WorkerLock after the session of the lock ended: %v, want the lock on a new session", err)
	}
	defer next.Release()
	if err := next.Check(ctx); err != nil {
		t.Errorf("Check of the lock on the new session: %v, want nil", err)
	}
	lost.Release()
	if err := next.Check(ctx); err != nil {
		t.Errorf("Check of the new lock after the release of the lost lock: %v, want nil", err)
	}
	if l3, err := s.WorkerLock(ctx); !errors.Is(err, jobdata.ErrWorkerLocked) {
		if l3.Release != nil {
			l3.Release()
		}
		t.Errorf("a third WorkerLock while the new lock holds it: error %v, want jobdata.ErrWorkerLocked", err)
	}
}

// sameWorker checks the worker row of the summary against want.
func sameWorker(t testing.TB, what string, got *jobdata.Worker, want jobdata.Worker) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: Summary worker <nil>, want %+v", what, want)
		return
	}
	if got.ID != want.ID || got.Host != want.Host || got.PID != want.PID || got.Version != want.Version ||
		got.Slots != want.Slots || !got.StartedAt.Equal(want.StartedAt) || !got.HeartbeatAt.Equal(want.HeartbeatAt) ||
		got.LoadedResourceKey != want.LoadedResourceKey {
		t.Errorf("%s: Summary worker\n got  %+v\n want %+v", what, *got, want)
	}
}

// summaryWorker gives the worker row of the summary.
func summaryWorker(t testing.TB, s jobs.Store) *jobdata.Worker {
	t.Helper()
	sum, err := s.Summary(context.Background())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	return sum.Worker
}

// checkHeartbeat: Heartbeat writes the worker row with heartbeat_at at the
// time of the clock, not the time in its argument. A second call writes each
// column again. The summary gives the row with the newest heartbeat.
func checkHeartbeat(t testing.TB, s jobs.Store, _ func(string) []string, clk *clock, _ func()) {
	ctx := context.Background()
	beat := func(w jobdata.Worker) {
		t.Helper()
		if err := s.Heartbeat(ctx, w); err != nil {
			t.Fatalf("Heartbeat %s: %v", w.ID, err)
		}
	}
	if w := summaryWorker(t, s); w != nil {
		t.Errorf("before a heartbeat: Summary worker %+v, want <nil>", *w)
	}
	first := jobdata.Worker{ID: "worker-1", Host: "horus-worker", PID: 7, Version: "0.9.0", Slots: 2,
		StartedAt: t0.Add(-time.Minute), HeartbeatAt: t0.Add(-time.Hour)}
	beat(first)
	want := first
	want.HeartbeatAt = t0
	sameWorker(t, "after the first heartbeat", summaryWorker(t, s), want)

	clk.Set(t0.Add(5 * time.Second))
	first.Slots, first.LoadedResourceKey = 3, "flux-3-klein"
	beat(first)
	want = first
	want.HeartbeatAt = t0.Add(5 * time.Second)
	sameWorker(t, "after the second heartbeat", summaryWorker(t, s), want)

	clk.Set(t0.Add(10 * time.Second))
	second := jobdata.Worker{ID: "worker-2", Host: "horus-worker", PID: 8, Version: "0.9.0", Slots: 1, StartedAt: t0.Add(9 * time.Second)}
	beat(second)
	want = second
	want.HeartbeatAt = t0.Add(10 * time.Second)
	sameWorker(t, "after a heartbeat of a second worker", summaryWorker(t, s), want)

	clk.Set(t0.Add(15 * time.Second))
	beat(first)
	want = first
	want.HeartbeatAt = t0.Add(15 * time.Second)
	sameWorker(t, "after a newer heartbeat of the first worker", summaryWorker(t, s), want)
}

// checkDeleteOld: DeleteOld deletes each ended job that finished more than
// age before the time of the clock, with its events, and each worker row with
// a heartbeat that old. It keeps a job that finished exactly age before, a
// newer job, and a queued or a running job of any age.
func checkDeleteOld(t testing.TB, s jobs.Store, events func(string) []string, clk *clock, _ func()) {
	ctx := context.Background()
	const age = 30 * 24 * time.Hour
	old := t0.Add(-age - time.Second)
	ended := func(i int, state jobdata.State, finished time.Time) jobdata.Job {
		j := queuedJob(i, finished.Add(-time.Minute))
		j.State, j.FinishedAt = state, finished
		if state != jobdata.Abandoned {
			j.StartedAt, j.Attempt = finished.Add(-30*time.Second), 1
		}
		return j
	}
	running := queuedJob(7, old.Add(-time.Minute))
	running.State, running.StartedAt, running.Attempt = jobdata.Running, old, 1
	for _, j := range []jobdata.Job{
		ended(1, jobdata.Succeeded, old),          // deleted
		ended(2, jobdata.Failed, old),             // deleted
		ended(3, jobdata.Abandoned, old),          // deleted
		ended(4, jobdata.Succeeded, t0.Add(-age)), // kept: exactly age
		ended(5, jobdata.Succeeded, t0.Add(-time.Hour)),
		queuedJob(6, old.Add(-time.Minute)), // kept: queued
		running,                             // kept: running
	} {
		insert(t, s, j)
	}
	clk.Set(old)
	if err := s.Heartbeat(ctx, jobdata.Worker{ID: "worker-old", Slots: 1, StartedAt: old}); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	clk.Set(t0)
	n, err := s.DeleteOld(ctx, age)
	if err != nil || n != 3 {
		t.Errorf("DeleteOld: %d deleted, %v; want 3 (the jobs 1, 2 and 3)", n, err)
	}
	for i := 1; i <= 3; i++ {
		_, err := s.Get(ctx, uid(i))
		notFound(t, fmt.Sprintf("Get of the old job %d after DeleteOld", i), err)
		eventsAre(t, events, uid(i))
	}
	for i := 4; i <= 7; i++ {
		get(t, s, uid(i))
	}
	eventsAre(t, events, uid(4), "job.queued")
	if w := summaryWorker(t, s); w != nil {
		t.Errorf("after DeleteOld: Summary worker %+v, want <nil>: its heartbeat is older than the age", *w)
	}
	young := jobdata.Worker{ID: "worker-young", Slots: 1, StartedAt: t0}
	if err := s.Heartbeat(ctx, young); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if n, err := s.DeleteOld(ctx, age); err != nil || n != 0 {
		t.Errorf("a second DeleteOld: %d deleted, %v; want 0", n, err)
	}
	young.HeartbeatAt = t0
	sameWorker(t, "after a second DeleteOld", summaryWorker(t, s), young)
}
