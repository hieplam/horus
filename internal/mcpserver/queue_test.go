// Tests of generate_image through the job queue of the stack, with the real
// service internal/jobs on the in-memory broker and job store of
// internal/brokertest. A fake worker consumes the work queue jobs.comfyui and
// writes a PNG at the container path of the plan. The image service is the
// fake of tool_generate_image_test.go: its Generate must never run.
package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/brokertest"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
	"github.com/hieplam/horus/internal/settings"
)

// hostShared is the shared folder of the host in the tests. The container
// folder is a real temporary folder, so that the fake worker can write.
const hostShared = `C:\Users\me\Pictures\horus`

// stackEnv is a server in the stack with its job queue.
type stackEnv struct {
	images    *fakeImages
	store     *brokertest.Store
	svc       *jobs.Service
	container string // the container folder of the shared folder
	cs        *mcp.ClientSession
	progress  chan string
}

func newStackEnv(t *testing.T, cfg jobs.Config) *stackEnv {
	t.Helper()
	useRepoWorkflows(t)
	e := &stackEnv{images: &fakeImages{running: true}, store: brokertest.NewStore(), container: filepath.ToSlash(t.TempDir()),
		progress: make(chan string, 256)}
	t.Setenv("HORUS_OUTPUT_DIR", e.container)
	if cfg.Poll == 0 {
		cfg.Poll = 10 * time.Millisecond
	}
	if cfg.Touch == 0 {
		cfg.Touch = 50 * time.Millisecond
	}
	e.svc = jobs.New(e.store, brokertest.NewBroker(), cfg)
	q := &Queue{Jobs: e.svc, QueueTimeout: cfg.QueueTimeout,
		Paths: PathsOf([]settings.SharedDir{{Host: hostShared, Container: e.container}}, "windows")}
	ready := func(context.Context, ...string) jobdata.Readiness {
		return jobdata.ReadinessOf([]jobdata.CheckResult{{Dependency: jobdata.DependencyDB, Up: true}, {Dependency: jobdata.DependencyComfyUI, Up: true},
			{Dependency: jobdata.DependencyWorker, Up: true}, {Dependency: jobdata.DependencyAPI, Up: true}})
	}
	e.cs = connect(t, NewWith(e.images, noFilesSetup{}, nil, Options{Ready: ready, Queue: q}), &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) { e.progress <- r.Params.Message },
	})
	return e
}

// worker starts a fake worker with one slot. run makes the result of a job;
// nil run writes the PNG of the plan.
func (e *stackEnv) worker(t *testing.T, run jobs.RunFunc) {
	t.Helper()
	if run == nil {
		run = func(ctx context.Context, j jobdata.Job, progress func(jobdata.Progress)) (json.RawMessage, error) {
			var p generate.Plan
			if err := json.Unmarshal(j.Payload, &p); err != nil {
				return nil, err
			}
			progress(jobdata.Progress{Message: "rendering, 3 s"})
			if err := os.MkdirAll(filepath.Dir(p.Path), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(p.Path, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
				return nil, err
			}
			bs := 2.5
			return json.Marshal(generate.Result{Workflow: p.Workflow, Path: p.Path, Bytes: 8, Width: p.Width, Height: p.Height,
				Seed: p.Seed, Seconds: 3, BackendSeconds: &bs, JobID: "prompt-1", BackendJobID: "prompt-1"})
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = e.svc.Consume(ctx, jobs.Consumer{WorkerID: "w1", Queue: jobs.WorkQueue(jobdata.BackendComfyUI), Slots: 1, Run: run})
	}()
	if err := e.store.Heartbeat(context.Background(), jobdata.Worker{ID: "w1", Slots: 1}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); wg.Wait() })
}

// call calls generate_image with a progressToken and gives the envelope.
func (e *stackEnv) call(t *testing.T, ctx context.Context, args map[string]any) map[string]any {
	t.Helper()
	params := &mcp.CallToolParams{Name: "generate_image", Arguments: args}
	params.SetProgressToken("tok")
	res, err := e.cs.CallTool(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	return envelopeOf(t, res)
}

// start calls generate_image in a goroutine that the test does not wait for.
// The call ends with ctx.
func (e *stackEnv) start(ctx context.Context, args map[string]any) {
	go func() {
		params := &mcp.CallToolParams{Name: "generate_image", Arguments: args}
		params.SetProgressToken("tok")
		_, _ = e.cs.CallTool(ctx, params)
	}()
}

// onlyJob gives the one job of the job store.
func (e *stackEnv) onlyJob(t *testing.T) jobdata.Job {
	t.Helper()
	sum, err := e.store.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	all := append(append(append([]jobdata.Job{}, sum.QueuedJobs...), sum.RunningJobs...), sum.Recent...)
	if len(all) != 1 {
		t.Fatalf("the job store has %d jobs, want 1", len(all))
	}
	j, err := e.store.Get(context.Background(), all[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func queueError(t *testing.T, env map[string]any) map[string]any {
	t.Helper()
	e, ok := env["error"].(map[string]any)
	if !ok || env["ok"] != false {
		t.Fatalf("want an error, got %v", env)
	}
	return e
}

var uuidV7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestQueueGenerateImage: a call with a host path in the shared folder goes
// through the job queue. The job row holds the plan with the container path,
// the fingerprint and the key of recipe version 1. The worker writes the PNG
// at the container path, and the result gives the host path, the horus job
// id, the id of the job in ComfyUI and the wait in the queue. The progress
// starts with the place in the queue. The image service makes no image.
func TestQueueGenerateImage(t *testing.T) {
	e := newStackEnv(t, jobs.Config{DefaultRun: 4 * time.Minute})
	host := `c:\users\me\pictures\HORUS\covers\one.png`
	envc := make(chan map[string]any, 1)
	go func() {
		envc <- e.call(t, context.Background(), map[string]any{"workflow": "it-illustration", "subject": testSubject, "seed": 7, "out_path": host})
	}()
	// No worker yet: the first progress is the place in the queue.
	if first := <-e.progress; first != "waiting in the horus queue, position 1 of 1, start in less than 1 min" {
		t.Errorf("first progress %q", first)
	}
	e.worker(t, nil)
	env := <-envc
	r, ok := env["result"].(map[string]any)
	if !ok || env["ok"] != true {
		t.Fatalf("envelope %v", env)
	}
	// The result gives the cleaned host path: the drive letter in upper case.
	if r["path"] != `C:\users\me\pictures\HORUS\covers\one.png` || r["backend_job_id"] != "prompt-1" || r["seed"] != 7.0 || r["started_backend"] != false {
		t.Errorf("result %v", r)
	}
	id, _ := r["job_id"].(string)
	if !uuidV7.MatchString(id) {
		t.Errorf("job_id %q is not a UUIDv7 of horus", id)
	}
	if qs, ok := r["queue_seconds"].(float64); !ok || qs < 0 {
		t.Errorf("queue_seconds %v", r["queue_seconds"])
	}
	if _, err := os.Stat(e.container + "/covers/one.png"); err != nil {
		t.Errorf("the PNG is not at the container path: %v", err)
	}
	if n := len(e.images.requests()); n != 0 {
		t.Errorf("the image service made %d images, want 0: the API process never calls ComfyUI for a job", n)
	}

	j := e.onlyJob(t)
	var plan generate.Plan
	if err := json.Unmarshal(j.Payload, &plan); err != nil || filepath.ToSlash(plan.Path) != e.container+"/covers/one.png" || plan.Seed != 7 {
		t.Errorf("payload %s %v", j.Payload, err)
	}
	if j.Tool != "generate_image" || j.Backend != "comfyui" || j.WorkQueue != "jobs.comfyui" || j.Caller != "test 0" {
		t.Errorf("job row %+v", j)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(j.RequestHash) || j.IdempotencyKey != "f1:"+j.RequestHash {
		t.Errorf("request_hash %q, idempotency_key %q: want the key f1:<hash> of a call with a seed", j.RequestHash, j.IdempotencyKey)
	}
	// The recipe of the call: the cleaned host path in lower case, the
	// subject as written, the seed as text.
	want, err := jobs.Recipe(jobs.Request{Tool: "generate_image", Workflow: "it-illustration", WorkflowRev: workflowRev(t, "it-illustration"),
		Subject: testSubject, Seed: ptr(uint64(7)), DefaultWidth: plan.Width, DefaultHeight: plan.Height,
		OutPath: host, PathStyle: jobs.PathWindows})
	if err != nil || want.Hash != j.RequestHash {
		t.Errorf("request_hash %s, want %s of %s", j.RequestHash, want.Hash, want.Text)
	}
}

func workflowRev(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join("..", "..", "workflows", name)
	m, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := os.ReadFile(filepath.Join(dir, "workflow.json"))
	if err != nil {
		t.Fatal(err)
	}
	return jobs.WorkflowRev(m, g)
}

// TestQueueKeys: a call with idempotency_key has the key k1:, and a call
// with no seed and no key has no key. The default output path is in the
// shared folder, and the result gives its host path.
func TestQueueKeys(t *testing.T) {
	e := newStackEnv(t, jobs.Config{})
	e.worker(t, nil)
	env := e.call(t, context.Background(), map[string]any{"workflow": "it-illustration", "subject": testSubject, "idempotency_key": "cover-42"})
	r, _ := env["result"].(map[string]any)
	path, _ := r["path"].(string)
	if env["ok"] != true || !strings.HasPrefix(path, hostShared+`\it-illustration\`) || !strings.HasSuffix(path, ".png") {
		t.Fatalf("the default path is not the host path in the shared folder: %v", env)
	}
	sum := sha256.Sum256([]byte("generate_image\ncover-42"))
	if j := e.onlyJob(t); j.IdempotencyKey != "k1:"+hex.EncodeToString(sum[:]) {
		t.Errorf("idempotency_key %q", j.IdempotencyKey)
	}

	e2 := newStackEnv(t, jobs.Config{})
	e2.worker(t, nil)
	if env := e2.call(t, context.Background(), map[string]any{"workflow": "it-illustration", "subject": testSubject}); env["ok"] != true {
		t.Fatalf("%v", env)
	}
	if j := e2.onlyJob(t); j.IdempotencyKey != "" || j.RequestHash == "" {
		t.Errorf("no seed and no key: idempotency_key %q, request_hash %q", j.IdempotencyKey, j.RequestHash)
	}
}

// TestQueueOutPathOutside: an out_path outside the shared folders gets
// INVALID_ARGUMENT that lists them, and makes no job.
func TestQueueOutPathOutside(t *testing.T) {
	e := newStackEnv(t, jobs.Config{})
	env := e.call(t, context.Background(), map[string]any{"workflow": "it-illustration", "subject": testSubject, "out_path": `D:\elsewhere\a.png`})
	er := queueError(t, env)
	d, _ := er["details"].(map[string]any)
	dirs, _ := d["shared_dirs"].([]any)
	if er["code"] != "INVALID_ARGUMENT" || d["field"] != "out_path" || len(dirs) != 1 || dirs[0] != hostShared {
		t.Errorf("error %v", er)
	}
	if n, _ := e.store.Queued(context.Background()); n != 0 {
		t.Errorf("%d jobs queued", n)
	}
}

// TestQueueFull: with HORUS_QUEUE_MAX jobs queued, a call gets QUEUE_FULL at
// once, with details.retry_after_seconds.
func TestQueueFull(t *testing.T) {
	e := newStackEnv(t, jobs.Config{QueueMax: 1, DefaultRun: 30 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.start(ctx, map[string]any{"workflow": "it-illustration", "subject": testSubject})
	<-e.progress // the first job waits in the queue: no worker
	start := time.Now()
	er := queueError(t, e.call(t, context.Background(), map[string]any{"workflow": "it-illustration", "subject": testSubject}))
	d, _ := er["details"].(map[string]any)
	if er["code"] != "QUEUE_FULL" || er["action_by"] != "retry" || d["retry_after_seconds"] != 30.0 || d["reason"] != "count" {
		t.Errorf("error %v", er)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("QUEUE_FULL after %s, want at once", took)
	}
}

// TestQueueCancel: a cancel of the call moves the queued job to cancelled at
// once. No worker ever sees it.
func TestQueueCancel(t *testing.T) {
	e := newStackEnv(t, jobs.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		params := &mcp.CallToolParams{Name: "generate_image", Arguments: map[string]any{"workflow": "it-illustration", "subject": testSubject}}
		params.SetProgressToken("tok")
		_, _ = e.cs.CallTool(ctx, params)
	}()
	<-e.progress
	cancel()
	<-done
	deadline := time.Now().Add(5 * time.Second)
	for e.onlyJob(t).State != jobdata.Cancelled {
		if time.Now().After(deadline) {
			t.Fatalf("the job is %s 5 s after the cancel, want cancelled", e.onlyJob(t).State)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestQueueJobErrors: the error of a failed job reaches the caller with its
// code. GENERATION_TIMEOUT of a run has details.phase running, details.job_id
// is the horus job id and details.backend_job_id the id in ComfyUI. A job
// that waits past its deadline gets GENERATION_TIMEOUT with details.phase
// queued, and never runs.
func TestQueueJobErrors(t *testing.T) {
	e := newStackEnv(t, jobs.Config{})
	e.worker(t, func(ctx context.Context, j jobdata.Job, _ func(jobdata.Progress)) (json.RawMessage, error) {
		return nil, herr.New("GENERATION_TIMEOUT", "the job did not finish in 1800 s", map[string]any{"job_id": "prompt-9"})
	})
	er := queueError(t, e.call(t, context.Background(), map[string]any{"workflow": "it-illustration", "subject": testSubject}))
	d, _ := er["details"].(map[string]any)
	if er["code"] != "GENERATION_TIMEOUT" || d["phase"] != "running" || d["backend_job_id"] != "prompt-9" || !uuidV7.MatchString(d["job_id"].(string)) {
		t.Errorf("error %v", er)
	}

	lost := newStackEnv(t, jobs.Config{})
	lost.worker(t, func(ctx context.Context, j jobdata.Job, _ func(jobdata.Progress)) (json.RawMessage, error) {
		return nil, herr.New("WORKER_LOST", "the worker stopped while it ran the job", nil)
	})
	if er := queueError(t, lost.call(t, context.Background(), map[string]any{"workflow": "it-illustration", "subject": testSubject})); er["code"] != "WORKER_LOST" || er["retryable"] != true {
		t.Errorf("error %v", er)
	}

	late := newStackEnv(t, jobs.Config{QueueTimeout: 200 * time.Millisecond, DefaultRun: time.Millisecond})
	go func() {
		time.Sleep(400 * time.Millisecond)
		_, _, _ = late.store.Sweep(context.Background()) // the sweep of the worker
	}()
	er = queueError(t, late.call(t, context.Background(), map[string]any{"workflow": "it-illustration", "subject": testSubject}))
	d, _ = er["details"].(map[string]any)
	if er["code"] != "GENERATION_TIMEOUT" || d["phase"] != "queued" || late.onlyJob(t).StartedAt != (time.Time{}) {
		t.Errorf("error %v", er)
	}
}

// TestQueueSpeechAndList: in the stack, generate_speech refuses with a cause
// and starts nothing, and list_workflows gives the readiness and the queue.
// On the direct path both fields are null.
func TestQueueSpeechAndList(t *testing.T) {
	e := newStackEnv(t, jobs.Config{})
	res := callTool(t, e.cs, "generate_speech", map[string]any{"workflow": "vi-speech", "text": "Xin chào các bạn."})
	er := queueError(t, envelopeOf(t, res))
	if er["code"] != "BACKEND_UNAVAILABLE" || er["retryable"] != false || !strings.Contains(er["message"].(string), "stack") {
		t.Errorf("generate_speech in the stack: %v", er)
	}
	e.worker(t, func(ctx context.Context, j jobdata.Job, _ func(jobdata.Progress)) (json.RawMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.start(ctx, map[string]any{"workflow": "it-illustration", "subject": testSubject})
	<-e.progress
	deadline := time.Now().Add(5 * time.Second)
	for {
		env := envelopeOf(t, callTool(t, e.cs, "list_workflows", map[string]any{}))
		r, _ := env["result"].(map[string]any)
		ready, _ := r["ready"].(map[string]any)
		q, _ := r["queue"].(map[string]any)
		if ready["ready"] == true && q["running"] == 1.0 && q["waiting"] == 0.0 && q["slots"] == 1.0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("list_workflows in the stack: %v", env)
		}
		time.Sleep(20 * time.Millisecond)
	}

	useRepoWorkflows(t)
	direct := connect(t, New(&fakeImages{}, noFilesSetup{}, nil), nil)
	r, _ := envelopeOf(t, callTool(t, direct, "list_workflows", map[string]any{}))["result"].(map[string]any)
	if v, ok := r["ready"]; !ok || v != nil {
		t.Errorf("direct path: ready %v", v)
	}
	if v, ok := r["queue"]; !ok || v != nil {
		t.Errorf("direct path: queue %v", v)
	}
}
