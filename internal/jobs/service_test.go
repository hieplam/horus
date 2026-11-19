package jobs_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/brokertest"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

var t0 = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

// clock is a clock that a test moves.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type env struct {
	svc        *jobs.Service
	store      *brokertest.Store
	broker     *brokertest.Broker
	clock      *clock
	deliveries <-chan jobdata.Delivery // the one consumer of delivery
}

func newEnv(t *testing.T, cfg jobs.Config) *env {
	t.Helper()
	c := &clock{t: t0}
	st := brokertest.NewStore()
	st.Now = c.Now
	b := brokertest.NewBroker()
	cfg.Now = c.Now
	if cfg.Poll == 0 {
		cfg.Poll = 10 * time.Millisecond
	}
	return &env{svc: jobs.New(st, b, cfg), store: st, broker: b, clock: c}
}

func request() jobdata.Job {
	return jobdata.Job{Tool: "generate_image", Workflow: "it-illustration", Backend: "comfyui",
		Payload: json.RawMessage(`{"subject":"a falcon"}`), RequestHash: "h", Caller: "test"}
}

func submit(t *testing.T, e *env) jobdata.Job {
	t.Helper()
	j, err := e.svc.Submit(context.Background(), request())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	return j
}

func get(t *testing.T, e *env, id string) jobdata.Job {
	t.Helper()
	j, err := e.store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	return j
}

// delivery takes the next delivery of the work queue, from one consumer for
// the whole test.
func delivery(t *testing.T, e *env) jobdata.Delivery {
	t.Helper()
	if e.deliveries == nil {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		ch, err := e.broker.Consume(ctx, "jobs.comfyui", 1)
		if err != nil {
			t.Fatal(err)
		}
		e.deliveries = ch
	}
	select {
	case d := <-e.deliveries:
		return d
	case <-time.After(5 * time.Second):
		t.Fatal("no delivery")
	}
	return jobdata.Delivery{}
}

func TestSubmit(t *testing.T) {
	e := newEnv(t, jobs.Config{QueueTimeout: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, _ := e.broker.Listen(ctx)
	j := submit(t, e)
	got := get(t, e, j.ID)
	if got.State != jobdata.Queued || got.WorkQueue != "jobs.comfyui" || !got.CreatedAt.Equal(t0) ||
		!got.Deadline.Equal(t0.Add(time.Hour)) || !got.WaiterSeenAt.Equal(t0) || got.Attempt != 0 || got.RequestHash != "h" {
		t.Errorf("the job row: %+v", got)
	}
	if len(j.ID) != 36 || j.ID[14] != '7' {
		t.Errorf("id %q is not a UUIDv7", j.ID)
	}
	if ev := e.store.Events(j.ID); !slices.Equal(ev, []string{"job.queued"}) {
		t.Errorf("events %v", ev)
	}
	d := delivery(t, e)
	if want := (jobdata.Message{V: 1, JobID: j.ID, Tool: "generate_image", Backend: "comfyui"}); d.Message != want {
		t.Errorf("message %+v, want %+v", d.Message, want)
	}
	select {
	case ev := <-events:
		if ev.JobID != j.ID || ev.Type != "job.queued" || !ev.At.Equal(t0) {
			t.Errorf("event %+v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Error("no event job.queued")
	}
}

func TestSubmitQueueFullCount(t *testing.T) {
	e := newEnv(t, jobs.Config{QueueMax: 2, DefaultRun: time.Minute})
	submit(t, e)
	submit(t, e)
	_, err := e.svc.Submit(context.Background(), request())
	var full *jobs.QueueFullError
	if !errors.As(err, &full) || full.Reason != jobs.FullCount || full.RetryAfter != time.Minute || full.Queued != 2 {
		t.Fatalf("third call: %v %+v", err, full)
	}
	if n, _ := e.store.Queued(context.Background()); n != 2 {
		t.Errorf("the refused call made a job: %d queued", n)
	}
}

func TestSubmitQueueFullDeadline(t *testing.T) {
	e := newEnv(t, jobs.Config{QueueTimeout: 5 * time.Minute, DefaultRun: 4 * time.Minute})
	submit(t, e) // starts now
	submit(t, e) // starts after 4 min
	_, err := e.svc.Submit(context.Background(), request())
	var full *jobs.QueueFullError
	if !errors.As(err, &full) || full.Reason != jobs.FullDeadline || full.RetryAfter != 3*time.Minute ||
		!full.Start.Equal(t0.Add(8*time.Minute)) {
		t.Fatalf("third call, start after 8 min and a deadline of 5 min: %v %+v", err, full)
	}
}

// failPublish is a broker that refuses each message.
type failPublish struct{ *brokertest.Broker }

func (failPublish) Publish(context.Context, string, jobdata.Message) error { return errors.New("down") }

func TestSubmitPublishFails(t *testing.T) {
	st := brokertest.NewStore()
	svc := jobs.New(st, failPublish{brokertest.NewBroker()}, jobs.Config{})
	if _, err := svc.Submit(context.Background(), request()); err == nil {
		t.Fatal("no error")
	}
	sum, _ := st.Summary(context.Background())
	if sum.Queued != 0 || len(sum.Recent) != 1 || sum.Recent[0].State != jobdata.Failed || sum.Recent[0].Error.Code != "INTERNAL" {
		t.Errorf("a job without a message must end: %+v", sum)
	}
}

func TestProgressOfQueuedJobs(t *testing.T) {
	e := newEnv(t, jobs.Config{DefaultRun: 4 * time.Minute})
	submit(t, e)
	submit(t, e)
	third := submit(t, e)
	st, err := e.svc.Progress(context.Background(), third.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Position != 3 || st.Total != 3 || !st.Start.Equal(t0.Add(8*time.Minute)) ||
		st.Message != "waiting in the horus queue, position 3 of 3, start in about 8 min" {
		t.Errorf("status %+v", st)
	}
}

// ok is a run that succeeds with a result.
func ok(calls *atomic.Int32) jobs.RunFunc {
	return func(ctx context.Context, j jobdata.Job, progress func(jobdata.Progress)) (json.RawMessage, error) {
		calls.Add(1)
		return json.RawMessage(`{"path":"out.png"}`), nil
	}
}

// consume runs the consumer until the test ends, and gives the outcomes.
func consume(t *testing.T, e *env, c jobs.Consumer) <-chan jobs.Outcome {
	t.Helper()
	out := make(chan jobs.Outcome, 32)
	c.Queue = "jobs.comfyui"
	if c.Slots == 0 {
		c.Slots = 1
	}
	c.Done = func(_ jobdata.Message, o jobs.Outcome, err error) {
		if err != nil && o != jobs.Stopped {
			t.Errorf("outcome %s: %v", o, err)
		}
		out <- o
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = e.svc.Consume(ctx, c)
	}()
	t.Cleanup(func() { cancel(); <-done })
	return out
}

func outcome(t *testing.T, ch <-chan jobs.Outcome) jobs.Outcome {
	t.Helper()
	select {
	case o := <-ch:
		return o
	case <-time.After(5 * time.Second):
		t.Fatal("no outcome")
	}
	return ""
}

// TestDuplicateDeliveryRunsOnce: the same message comes two times, with 2
// slots. The job runs one time: the idempotent consumer.
func TestDuplicateDeliveryRunsOnce(t *testing.T) {
	e := newEnv(t, jobs.Config{Slots: 2})
	j := submit(t, e)
	m := jobdata.Message{V: 1, JobID: j.ID, Tool: j.Tool, Backend: j.Backend}
	if err := e.broker.Publish(context.Background(), "jobs.comfyui", m); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	outs := consume(t, e, jobs.Consumer{WorkerID: "w1", Slots: 2, Run: ok(&calls)})
	got := []jobs.Outcome{outcome(t, outs), outcome(t, outs)}
	slices.Sort(got)
	if !slices.Equal(got, []jobs.Outcome{jobs.Ran, jobs.Skipped}) {
		t.Errorf("outcomes %v", got)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("the job ran %d times", n)
	}
	if g := get(t, e, j.ID); g.State != jobdata.Succeeded || string(g.Result) != `{"path":"out.png"}` || g.WorkerID != "w1" || g.Attempt != 1 {
		t.Errorf("the job: %+v", g)
	}
	if ev := e.store.Events(j.ID); !slices.Equal(ev, []string{"job.queued", "job.running", "job.succeeded"}) {
		t.Errorf("events %v", ev)
	}
}

// TestWorkerLost: a delivery of a job that runs on a worker that stopped
// ends the job with WORKER_LOST, and runs nothing.
func TestWorkerLost(t *testing.T) {
	e := newEnv(t, jobs.Config{})
	j := submit(t, e)
	if _, err := e.store.Move(context.Background(), j.ID, []jobdata.State{jobdata.Queued}, jobdata.Running,
		jobdata.Change{At: t0, WorkerID: "old"}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	o, err := e.svc.Handle(context.Background(), jobs.Consumer{WorkerID: "new", Run: ok(&calls)}, delivery(t, e))
	if o != jobs.Lost || err != nil {
		t.Fatalf("outcome %s %v", o, err)
	}
	g := get(t, e, j.ID)
	if g.State != jobdata.Failed || g.Error == nil || g.Error.Code != jobs.CodeWorkerLost || g.Error.Details["worker_id"] != "old" || calls.Load() != 0 {
		t.Errorf("the job: %+v, runs %d", g, calls.Load())
	}
}

// TestSkipsJobsThatAreNotQueued: a cancelled job, an expired job, an abandoned
// job and a job that the store does not have never run.
func TestSkipsJobsThatAreNotQueued(t *testing.T) {
	e := newEnv(t, jobs.Config{QueueTimeout: time.Hour})
	cancelled := submit(t, e)
	if s, err := e.svc.Cancel(context.Background(), cancelled.ID); s != jobdata.Cancelled || err != nil {
		t.Fatalf("Cancel: %s %v", s, err)
	}
	var calls atomic.Int32
	c := jobs.Consumer{WorkerID: "w", Run: ok(&calls)}
	if o, err := e.svc.Handle(context.Background(), c, delivery(t, e)); o != jobs.Skipped || err != nil {
		t.Errorf("a cancelled job: %s %v", o, err)
	}

	expired := submit(t, e)
	e.clock.Add(time.Hour + time.Second)
	if o, _ := e.svc.Handle(context.Background(), c, delivery(t, e)); o != jobs.Skipped {
		t.Errorf("an expired job: %s", o)
	}
	if s := get(t, e, expired.ID).State; s != jobdata.Expired {
		t.Errorf("the job after its deadline: %s", s)
	}

	abandoned := submit(t, e)
	e.clock.Add(jobdata.AbandonAfter + time.Second)
	if o, _ := e.svc.Handle(context.Background(), c, delivery(t, e)); o != jobs.Skipped {
		t.Errorf("an abandoned job: %s", o)
	}
	if s := get(t, e, abandoned.ID).State; s != jobdata.Abandoned {
		t.Errorf("the job with no waiter: %s", s)
	}

	_ = e.broker.Publish(context.Background(), "jobs.comfyui", jobdata.Message{V: 1, JobID: "nobody"})
	if o, err := e.svc.Handle(context.Background(), c, delivery(t, e)); o != jobs.Skipped || err != nil {
		t.Errorf("a job that the store does not have: %s %v", o, err)
	}
	if calls.Load() != 0 {
		t.Errorf("%d runs", calls.Load())
	}
}

func TestRunProgressAndFailure(t *testing.T) {
	e := newEnv(t, jobs.Config{})
	j := submit(t, e)
	var seen string
	run := func(ctx context.Context, j jobdata.Job, progress func(jobdata.Progress)) (json.RawMessage, error) {
		progress(jobdata.Progress{Value: 9, Total: 15, Message: "rendering, step 9 of 15"})
		st, _ := e.svc.Progress(ctx, j.ID)
		seen = st.Message
		return nil, herr.New("OUT_OF_MEMORY", "no VRAM", map[string]any{"width": 1536})
	}
	o, err := e.svc.Handle(context.Background(), jobs.Consumer{WorkerID: "w", Run: run}, delivery(t, e))
	if o != jobs.Ran || err != nil {
		t.Fatalf("outcome %s %v", o, err)
	}
	if seen != "rendering, step 9 of 15" {
		t.Errorf("the progress of a running job: %q", seen)
	}
	g := get(t, e, j.ID)
	if g.State != jobdata.Failed || g.Error.Code != "OUT_OF_MEMORY" || g.Error.Message != "no VRAM" || g.Error.Details["width"] != 1536 {
		t.Errorf("the job: %+v %+v", g, g.Error)
	}
	if ev := e.store.Events(j.ID); !slices.Equal(ev, []string{"job.queued", "job.running", "job.progress", "job.failed"}) {
		t.Errorf("events %v", ev)
	}
}

// TestCancelRunning: a cancel of a running job asks the worker to stop it. The
// run stops, and the job ends in cancelled.
func TestCancelRunning(t *testing.T) {
	e := newEnv(t, jobs.Config{})
	j := submit(t, e)
	run := func(ctx context.Context, j jobdata.Job, progress func(jobdata.Progress)) (json.RawMessage, error) {
		if s, err := e.svc.Cancel(ctx, j.ID); s != jobdata.Running || err != nil {
			t.Errorf("Cancel of a running job: %s %v", s, err)
		}
		if g := get(t, e, j.ID); g.CancelRequested.IsZero() {
			t.Error("no cancel_requested_at")
		}
		return nil, errors.New("stopped on request")
	}
	if o, err := e.svc.Handle(context.Background(), jobs.Consumer{WorkerID: "w", Run: run}, delivery(t, e)); o != jobs.Ran || err != nil {
		t.Fatalf("outcome %s %v", o, err)
	}
	if s := get(t, e, j.ID).State; s != jobdata.Cancelled {
		t.Errorf("state %s", s)
	}
	if s, err := e.svc.Cancel(context.Background(), j.ID); s != jobdata.Cancelled || err != nil {
		t.Errorf("Cancel of an ended job: %s %v", s, err)
	}
	if ev := e.store.Events(j.ID); !slices.Equal(ev, []string{"job.queued", "job.running", "job.cancel_requested", "job.cancelled"}) {
		t.Errorf("events %v", ev)
	}
}

// TestStoppedWorkerLeavesTheMessage: when the worker stops while a job runs,
// the delivery gets no ack. The message comes again, and the next worker ends
// the job with WORKER_LOST.
func TestStoppedWorkerLeavesTheMessage(t *testing.T) {
	e := newEnv(t, jobs.Config{})
	j := submit(t, e)
	started := make(chan struct{})
	run := func(ctx context.Context, j jobdata.Job, progress func(jobdata.Progress)) (json.RawMessage, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		done <- e.svc.Consume(ctx, jobs.Consumer{WorkerID: "first", Queue: "jobs.comfyui", Slots: 1, Run: run})
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Consume: %v", err)
	}
	outs := consume(t, e, jobs.Consumer{WorkerID: "second", Run: run})
	if o := outcome(t, outs); o != jobs.Lost {
		t.Errorf("the next worker: %s", o)
	}
	if g := get(t, e, j.ID); g.State != jobdata.Failed || g.Error.Code != jobs.CodeWorkerLost {
		t.Errorf("the job: %+v", g)
	}
}

// reports collects the progress reports of Wait.
type reports struct {
	mu   sync.Mutex
	msgs []string
	ch   chan string
}

func newReports() *reports { return &reports{ch: make(chan string, 64)} }

func (r *reports) report(_ context.Context, progress, total float64, message string) {
	r.mu.Lock()
	r.msgs = append(r.msgs, message)
	r.mu.Unlock()
	r.ch <- message
}

func TestWait(t *testing.T) {
	e := newEnv(t, jobs.Config{DefaultRun: 4 * time.Minute})
	j := submit(t, e)
	r := newReports()
	type res struct {
		j   jobdata.Job
		err error
	}
	done := make(chan res, 1)
	go func() {
		got, err := e.svc.Wait(context.Background(), j.ID, r.report)
		done <- res{got, err}
	}()
	if m := <-r.ch; m != "waiting in the horus queue, position 1 of 1, start in less than 1 min" {
		t.Errorf("first report %q", m)
	}
	var calls atomic.Int32
	consume(t, e, jobs.Consumer{WorkerID: "w", Run: ok(&calls)})
	select {
	case got := <-done:
		if got.err != nil || got.j.State != jobdata.Succeeded || string(got.j.Result) != `{"path":"out.png"}` {
			t.Errorf("Wait: %+v %v", got.j, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return")
	}
}

// TestWaitWakesOnEvent: with a poll of one hour, only the event can wake the
// waiter.
func TestWaitWakesOnEvent(t *testing.T) {
	e := newEnv(t, jobs.Config{Poll: time.Hour})
	j := submit(t, e)
	r := newReports()
	done := make(chan error, 1)
	go func() {
		_, err := e.svc.Wait(context.Background(), j.ID, r.report)
		done <- err
	}()
	<-r.ch
	var calls atomic.Int32
	consume(t, e, jobs.Consumer{WorkerID: "w", Run: ok(&calls)})
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the event did not wake the waiter")
	}
}

// dropEvents is a broker that loses each event.
type dropEvents struct{ *brokertest.Broker }

func (dropEvents) Notify(context.Context, jobdata.Event) error { return nil }

// TestWaitWithoutEvents: when each event is lost, the waiter still sees the
// end of the job, from the job store.
func TestWaitWithoutEvents(t *testing.T) {
	c := &clock{t: t0}
	st := brokertest.NewStore()
	st.Now = c.Now
	b := dropEvents{brokertest.NewBroker()}
	svc := jobs.New(st, b, jobs.Config{Now: c.Now, Poll: 10 * time.Millisecond})
	j, err := svc.Submit(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := svc.Wait(context.Background(), j.ID, nil)
		done <- err
	}()
	if _, err := st.Move(context.Background(), j.ID, []jobdata.State{jobdata.Queued}, jobdata.Cancelled, jobdata.Change{At: t0}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the waiter did not read the job store")
	}
}

// TestWaitTouches: the waiter writes waiter_seen_at, so that the sweep does
// not abandon a job that has a waiter. Its context ends the wait, and the job
// stays queued.
func TestWaitTouches(t *testing.T) {
	e := newEnv(t, jobs.Config{Touch: time.Second})
	j := submit(t, e)
	r := newReports()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := e.svc.Wait(ctx, j.ID, r.report)
		done <- err
	}()
	<-r.ch
	e.clock.Add(50 * time.Second)
	<-r.ch // the next touch, after Config.Touch
	e.clock.Add(50 * time.Second)
	if _, abandoned, _ := e.store.Sweep(context.Background()); abandoned != 0 {
		t.Error("the sweep abandoned a job that has a waiter")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Wait: %v", err)
	}
	if s := get(t, e, j.ID).State; s != jobdata.Queued {
		t.Errorf("state %s", s)
	}
}

// TestReadyGatesTheRun: Handle runs nothing while Consumer.Ready waits, and
// the job stays queued. When ctx ends during the wait, the outcome is
// Stopped, the delivery gets no ack and the job is still queued. When Ready
// returns, the same delivery runs the job.
func TestReadyGatesTheRun(t *testing.T) {
	e := newEnv(t, jobs.Config{})
	j := submit(t, e)
	var calls atomic.Int32
	up := make(chan struct{})
	ready := func(ctx context.Context) error {
		select {
		case <-up:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c := jobs.Consumer{WorkerID: "w", Run: ok(&calls), Ready: ready}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	d := delivery(t, e)
	if o, err := e.svc.Handle(ctx, c, d); o != jobs.Stopped || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Handle while Ready waits: %s %v, want stopped and the error of ctx", o, err)
	}
	if g := get(t, e, j.ID); g.State != jobdata.Queued || calls.Load() != 0 {
		t.Fatalf("the job after Ready waited: %s with %d runs, want queued with 0 runs", g.State, calls.Load())
	}
	close(up)
	if o, err := e.svc.Handle(context.Background(), c, d); o != jobs.Ran || err != nil {
		t.Fatalf("Handle after Ready: %s %v", o, err)
	}
	if g := get(t, e, j.ID); g.State != jobdata.Succeeded || calls.Load() != 1 {
		t.Errorf("the job after Ready: %s with %d runs, want succeeded with 1 run", g.State, calls.Load())
	}
}

// TestSubmitted: Submitted saves backend_job_id of a running job with the
// event job.submitted, and changes no job that is not running.
func TestSubmitted(t *testing.T) {
	e := newEnv(t, jobs.Config{})
	j := submit(t, e)
	if err := e.svc.Submitted(context.Background(), j.ID, "prompt-1"); err != nil {
		t.Fatal(err)
	}
	if g := get(t, e, j.ID); g.BackendJobID != "" {
		t.Errorf("Submitted of a queued job: backend_job_id %q, want empty", g.BackendJobID)
	}
	if _, err := e.store.Move(context.Background(), j.ID, []jobdata.State{jobdata.Queued}, jobdata.Running,
		jobdata.Change{At: t0, WorkerID: "w"}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Submitted(context.Background(), j.ID, "prompt-2"); err != nil {
		t.Fatal(err)
	}
	if g := get(t, e, j.ID); g.BackendJobID != "prompt-2" || g.State != jobdata.Running {
		t.Errorf("Submitted of a running job: %s with backend_job_id %q, want running with prompt-2", g.State, g.BackendJobID)
	}
	if ev := e.store.Events(j.ID); !slices.Equal(ev, []string{"job.queued", "job.running", "job.submitted"}) {
		t.Errorf("events %v", ev)
	}
}
