// Tests of the port of horus/generate.py, against a fake of the interface
// ComfyUI (fake_test.go) and workflows in a temporary folder. The same path
// through the real adapter and a fake ComfyUI over HTTP is tested in
// cmd/horus (image_test.go).
package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

const subject = "On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. " +
	"On the right, a sky-blue server box takes them off the belt."

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"manifest.json", "workflow.json", "models.json"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func editJSON(t *testing.T, path string, edit func(m map[string]any)) {
	t.Helper()
	data, _ := os.ReadFile(path)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	edit(m)
	out, _ := json.Marshal(m)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

type env struct {
	fake *fakeComfy
	svc  *Service
	back *fakeBackend
	out  string
}

// setup makes the workflows it-illustration and absent-style (its UNET file
// is not in the options of the fake server) and short wait intervals.
func setup(t *testing.T) *env {
	t.Helper()
	tmp := t.TempDir()
	wf := filepath.Join(tmp, "workflows")
	copyDir(t, filepath.Join("..", "..", "workflows", "it-illustration"), filepath.Join(wf, "it-illustration"))
	absent := filepath.Join(wf, "absent-style")
	copyDir(t, filepath.Join(wf, "it-illustration"), absent)
	editJSON(t, filepath.Join(absent, "workflow.json"), func(m map[string]any) {
		for _, n := range m["nodes"].([]any) {
			node := n.(map[string]any)
			if node["type"] == "UNETLoader" {
				node["widgets_values"].([]any)[0] = "absent-model.safetensors"
			}
		}
	})
	editJSON(t, filepath.Join(absent, "manifest.json"), func(m map[string]any) { m["name"] = "absent-style" })

	t.Setenv("HORUS_ROOT", tmp)
	t.Setenv("HORUS_WORKFLOWS_DIR", wf)
	t.Setenv("HORUS_OUTPUT_DIR", filepath.Join(tmp, "out"))
	t.Setenv("HORUS_AUTOSTART", "0")
	settings.Reset()
	t.Cleanup(settings.Reset)

	oldPoll, oldReport, oldAbsent := PollInterval, ReportInterval, AbsentLimit
	PollInterval, ReportInterval, AbsentLimit = 5*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() { PollInterval, ReportInterval, AbsentLimit = oldPoll, oldReport, oldAbsent })

	f := newFake(t, filepath.Join("..", "..", "tests", "fixtures", "object_info.json"))
	b := &fakeBackend{comfy: f}
	return &env{fake: f, svc: New(f, b), back: b, out: filepath.Join(tmp, "images")}
}

type reports struct {
	mu   sync.Mutex
	msgs []string
}

func (r *reports) report(ctx context.Context, progress, total float64, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, message)
}

func (r *reports) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.msgs...)
}

func (e *env) opts(r *reports) Options {
	o := Options{Timeout: 5 * time.Second}
	if r != nil {
		o.Report = r.report
	}
	return o
}

func codeOf(err error) string {
	var he *herr.Error
	if errors.As(err, &he) {
		return he.Code
	}
	return ""
}

func info(t *testing.T, err error) herr.Info {
	t.Helper()
	var he *herr.Error
	if !errors.As(err, &he) {
		t.Fatalf("not a horus error: %v", err)
	}
	return he.Info()
}

func inputsOf(t *testing.T, graph map[string]any, class string) map[string]any {
	t.Helper()
	for _, n := range graph {
		node := n.(map[string]any)
		if node["class_type"] == class {
			return node["inputs"].(map[string]any)
		}
	}
	t.Fatalf("no %s node", class)
	return nil
}

// TestGenerateWritesThePNG reads the first 8 bytes of the file, so a result
// without a file fails.
func TestGenerateWritesThePNG(t *testing.T) {
	e := setup(t)
	out := filepath.Join(e.out, "queue.png")
	var r reports
	res, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: "  " + subject + "\n",
		Seed: int64(7), Width: int64(1024), Height: int64(768), OutPath: &out}, e.opts(&r))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil || len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("no PNG at out_path: %v", err)
	}
	if _, err := os.Stat(out + ".part"); !os.IsNotExist(err) {
		t.Fatal("the .part file is left")
	}
	if res.Path != out || res.Seed != 7 || res.Width != 1024 || res.Height != 768 || res.Bytes != int64(len(data)) ||
		res.Workflow != "it-illustration" || res.JobID != "job-1" || res.StartedBackend {
		t.Fatalf("result: %+v", res)
	}
	if res.BackendSeconds == nil || *res.BackendSeconds != 3.5 {
		t.Fatalf("backend_seconds: %v", res.BackendSeconds)
	}
	graph := e.fake.Job("job-1").Graph
	if got := inputsOf(t, graph, "CLIPTextEncode")["text"]; got != subject {
		t.Fatalf("caption is not the subject as written (spaces joined): %q", got)
	}
	if got := inputsOf(t, graph, "KSampler")["seed"]; got != 7.0 {
		t.Fatalf("seed: %v", got)
	}
	latent := inputsOf(t, graph, "EmptyFlux2LatentImage")
	if latent["width"] != 1024.0 || latent["height"] != 768.0 {
		t.Fatalf("size: %v", latent)
	}
	if p, _ := inputsOf(t, graph, "SaveImage")["filename_prefix"].(string); !strings.HasPrefix(p, "horus-it-illustration-") {
		t.Fatalf("filename_prefix: %v", p)
	}
	msgs := r.all()
	if len(msgs) == 0 || msgs[0] != "job job-1 submitted" {
		t.Fatalf("reports: %v", msgs)
	}

	// The same out_path again: OUTPUT_EXISTS, and no new job.
	_, err = e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject, OutPath: &out}, e.opts(nil))
	if i := info(t, err); i.Code != "OUTPUT_EXISTS" || i.Details["path"] != out || len(e.fake.Jobs()) != 1 {
		t.Fatalf("got %+v", i)
	}
	// overwrite=true replaces it.
	if _, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject, OutPath: &out, Overwrite: true}, e.opts(nil)); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultPathAndSeed(t *testing.T) {
	e := setup(t)
	res, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(settings.OutputDir(), "it-illustration")
	if filepath.Dir(res.Path) != dir || !strings.HasSuffix(res.Path, "-"+big.NewInt(0).SetUint64(res.Seed).String()+".png") {
		t.Fatalf("path %s", res.Path)
	}
	if res.Seed >= 1<<32 {
		t.Fatalf("a random seed is under 2^32: %d", res.Seed)
	}
	if res.Width != 1536 || res.Height != 1024 {
		t.Fatalf("default size: %dx%d", res.Width, res.Height)
	}
}

func TestArgumentChecks(t *testing.T) {
	e := setup(t)
	rel := "image.png"
	jpg := filepath.Join(e.out, "image.jpg")
	dot := filepath.Join(e.out, ".png")
	huge, _ := new(big.Int).SetString("18446744073709551616", 10)
	for _, c := range []struct {
		name  string
		req   Request
		field string
	}{
		{"short subject", Request{Subject: "a box"}, "subject"},
		{"long subject", Request{Subject: strings.Repeat("a", 5000)}, "subject"},
		{"width not a multiple", Request{Subject: subject, Width: int64(1000)}, "width"},
		{"height too small", Request{Subject: subject, Height: int64(0)}, "height"},
		{"width too big", Request{Subject: subject, Width: huge}, "width"},
		{"negative seed", Request{Subject: subject, Seed: int64(-1)}, "seed"},
		{"seed 2^64", Request{Subject: subject, Seed: huge}, "seed"},
		{"relative out_path", Request{Subject: subject, OutPath: &rel}, "out_path"},
		{"not .png", Request{Subject: subject, OutPath: &jpg}, "out_path"},
		{"only a dot name", Request{Subject: subject, OutPath: &dot}, "out_path"},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.req.Workflow = "it-illustration"
			_, err := e.svc.Generate(context.Background(), c.req, e.opts(nil))
			i := info(t, err)
			if i.Code != "INVALID_ARGUMENT" || i.Details["field"] != c.field || i.ActionBy != "caller" {
				t.Fatalf("got %+v", i)
			}
			if c.field == "subject" && i.Details["rules"] == nil {
				t.Fatal("a subject error carries the rules")
			}
		})
	}
	if len(e.fake.Jobs()) != 0 {
		t.Fatal("a refused call submitted a job")
	}
	maxSeed, _ := new(big.Int).SetString("18446744073709551615", 10)
	res, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject, Seed: maxSeed}, e.opts(nil))
	if err != nil || res.Seed != 1<<64-1 {
		t.Fatalf("seed 2^64-1: %v %v", res, err)
	}
	if _, err := e.svc.Generate(context.Background(), Request{Workflow: "nope", Subject: subject}, e.opts(nil)); codeOf(err) != "WORKFLOW_NOT_FOUND" {
		t.Fatalf("unknown workflow: %v", err)
	}
}

func TestWorkflowNotInstalled(t *testing.T) {
	e := setup(t)
	_, err := e.svc.Generate(context.Background(), Request{Workflow: "absent-style", Subject: subject}, e.opts(nil))
	i := info(t, err)
	if i.Code != "WORKFLOW_NOT_INSTALLED" || !reflect.DeepEqual(i.Details["missing"], []string{"absent-model.safetensors"}) ||
		i.NextStep != "Call install_workflow with workflow='absent-style', then call generate_image again." || len(e.fake.Jobs()) != 0 {
		t.Fatalf("got %+v", i)
	}
}

func TestNodeCountMismatch(t *testing.T) {
	e := setup(t)
	editJSON(t, filepath.Join(settings.WorkflowsDir(), "it-illustration", "manifest.json"), func(m map[string]any) {
		m["graph"].(map[string]any)["node_count"] = 11
	})
	_, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil))
	if i := info(t, err); i.Code != "INTERNAL" || i.Message != "converted 10 of 11 nodes" {
		t.Fatalf("got %+v", i)
	}
}

func TestOutOfMemory(t *testing.T) {
	e := setup(t)
	e.fake.SetMode(OOM)
	_, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil))
	if i := info(t, err); i.Code != "OUT_OF_MEMORY" || !i.Retryable || i.ActionBy != "caller" {
		t.Fatalf("got %+v", i)
	}
}

func TestNoImage(t *testing.T) {
	e := setup(t)
	e.fake.SetMode(NoImage)
	_, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil))
	if i := info(t, err); i.Code != "GENERATION_FAILED" || i.Message != "the job finished without an image" {
		t.Fatalf("got %+v", i)
	}
}

// TestTimeoutCancelsTheJob: a running job and a pending job over the time
// limit are cancelled, and the reports name their state. The adapter
// interrupts a running job and deletes a pending one (internal/comfy,
// TestClientJobCalls).
func TestTimeoutCancelsTheJob(t *testing.T) {
	e := setup(t)
	o := e.opts(nil)
	o.Timeout = 150 * time.Millisecond
	var r reports
	o.Report = r.report

	e.fake.SetMode(Slow)
	_, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, o)
	i := info(t, err)
	if i.Code != "GENERATION_TIMEOUT" || i.Details["job_id"] != "job-1" || !reflect.DeepEqual(e.fake.Cancels(), []string{"job-1"}) {
		t.Fatalf("running: %+v cancels=%v", i, e.fake.Cancels())
	}
	if !hasReport(r.all(), "rendering, ") {
		t.Fatalf("reports: %v", r.all())
	}

	e.fake.SetMode(Pending)
	_, err = e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, o)
	if codeOf(err) != "GENERATION_TIMEOUT" || !reflect.DeepEqual(e.fake.Cancels(), []string{"job-1", "job-2"}) {
		t.Fatalf("pending: %v cancels=%v", err, e.fake.Cancels())
	}
	if !hasReport(r.all(), "waiting in the backend queue, position 2, ") {
		t.Fatalf("reports: %v", r.all())
	}
}

func TestJobLeftTheQueue(t *testing.T) {
	e := setup(t)
	e.fake.SetMode(Absent)
	_, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil))
	if i := info(t, err); i.Code != "GENERATION_FAILED" || !strings.Contains(i.NextStep, "cleared its queue") {
		t.Fatalf("got %+v", i)
	}
}

func TestCancelledCallCancelsTheJob(t *testing.T) {
	e := setup(t)
	e.fake.SetMode(Slow)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// Cancel in the wait loop: after the first poll of /history for the job.
		for len(e.fake.Jobs()) == 0 || e.fake.Polls(e.fake.Jobs()[0]) == 0 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	_, err := e.svc.Generate(ctx, Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil))
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(e.fake.Cancels(), []string{"job-1"}) {
		t.Fatalf("got %v cancels=%v", err, e.fake.Cancels())
	}
}

func TestBackendDownAutostartOff(t *testing.T) {
	e := setup(t)
	o := e.opts(nil)
	e.fake.SetDown()
	var r reports
	o.Report = r.report
	_, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, o)
	if i := info(t, err); i.Code != "BACKEND_UNAVAILABLE" || i.ActionBy != "human" {
		t.Fatalf("got %+v", i)
	}
	if msgs := r.all(); len(msgs) != 1 || !strings.HasPrefix(msgs[0], "starting the image backend") {
		t.Fatalf("reports: %v", msgs)
	}
	if i := info(t, err); i.Message != "ComfyUI does not answer at http://fake, and HORUS_AUTOSTART is off" || e.back.Starts() != 0 {
		t.Fatalf("message %q, %d starts", i.Message, e.back.Starts())
	}
}

func autostartOn(t *testing.T) {
	t.Helper()
	t.Setenv("HORUS_AUTOSTART", "1")
	settings.Reset()
}

// TestDownBackendStartsOneTime: a generate call on a ComfyUI that is down
// calls Start of the backend exactly one time, then makes the image. The
// next call finds ComfyUI up and starts nothing.
func TestDownBackendStartsOneTime(t *testing.T) {
	e := setup(t)
	autostartOn(t)
	e.fake.SetDown()
	var r reports
	o := e.opts(&r)
	res, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject, OutPath: ptr(filepath.Join(e.out, "a.png"))}, o)
	if err != nil {
		t.Fatal(err)
	}
	if e.back.Starts() != 1 || !res.StartedBackend {
		t.Fatalf("%d starts, started_backend %v; want 1 start and true", e.back.Starts(), res.StartedBackend)
	}
	if msgs := r.all(); len(msgs) == 0 || !strings.HasPrefix(msgs[0], "starting the image backend") {
		t.Fatalf("reports: %v", msgs)
	}
	res, err = e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject, OutPath: ptr(filepath.Join(e.out, "b.png"))}, o)
	if err != nil || e.back.Starts() != 1 || res.StartedBackend {
		t.Fatalf("second call: %v, %d starts, started_backend %v", err, e.back.Starts(), res.StartedBackend)
	}
}

// TestUpBackendStartsNothing: when ComfyUI answers, the backend is not called.
func TestUpBackendStartsNothing(t *testing.T) {
	e := setup(t)
	autostartOn(t)
	if _, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil)); err != nil {
		t.Fatal(err)
	}
	if e.back.Starts() != 0 {
		t.Fatalf("%d starts", e.back.Starts())
	}
}

// TestStartOfAComfyUIThatIsNotInstalled: BACKEND_NOT_INSTALLED, with the
// lines of the start and its error in start_output_tail.
func TestStartOfAComfyUIThatIsNotInstalled(t *testing.T) {
	e := setup(t)
	autostartOn(t)
	e.fake.SetDown()
	e.back.lines = "line 1\n"
	e.back.start = func() error {
		return notInstalled{"ComfyUI was not found in C:/nowhere. Run horus install-comfyui first."}
	}
	_, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil))
	i := info(t, err)
	tail, _ := i.Details["start_output_tail"].([]string)
	if i.Code != "BACKEND_NOT_INSTALLED" || i.Message != "horus could not start ComfyUI at http://fake" || len(tail) != 2 ||
		tail[0] != "line 1" || !strings.HasPrefix(tail[1], "ComfyUI was not found") || e.back.Starts() != 1 {
		t.Fatalf("got %+v, %d starts", i, e.back.Starts())
	}
}

// TestFailedStartKeepsTheLastEightLines: another failed start is
// BACKEND_UNAVAILABLE with the last 8 lines. A start that returns but leaves
// ComfyUI down is BACKEND_UNAVAILABLE too.
func TestFailedStartKeepsTheLastEightLines(t *testing.T) {
	e := setup(t)
	autostartOn(t)
	e.fake.SetDown()
	for i := 1; i <= 11; i++ {
		e.back.lines += fmt.Sprintf("line %d\r\n", i)
	}
	e.back.start = func() error { return errors.New("line 12") }
	_, err := e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil))
	i := info(t, err)
	tail, _ := i.Details["start_output_tail"].([]string)
	if i.Code != "BACKEND_UNAVAILABLE" || len(tail) != 8 || tail[0] != "line 5" || tail[7] != "line 12" {
		t.Fatalf("fail: %+v", i)
	}

	e.back.lines, e.back.start = "OK: ComfyUI answers\n", func() error { return nil } // still down
	_, err = e.svc.Generate(context.Background(), Request{Workflow: "it-illustration", Subject: subject}, e.opts(nil))
	if i := info(t, err); i.Code != "BACKEND_UNAVAILABLE" || e.back.Starts() != 2 {
		t.Fatalf("still down: %+v, %d starts", i, e.back.Starts())
	}
}

// TestRunningAndFreeMemoryAskTheBackend: the two calls that the tools
// list_workflows and uninstall_workflow make through the service.
func TestRunningAndFreeMemoryAskTheBackend(t *testing.T) {
	e := setup(t)
	if up, err := e.svc.Running(context.Background()); !up || err != nil {
		t.Fatalf("up: %v %v", up, err)
	}
	e.svc.FreeMemory(context.Background())
	if e.fake.Frees() != 1 {
		t.Fatalf("frees: %d", e.fake.Frees())
	}
	e.fake.SetDown()
	if up, err := e.svc.Running(context.Background()); up || err != nil {
		t.Fatalf("down: %v %v", up, err)
	}
}

func TestPySuffixAndQuote(t *testing.T) {
	for p, want := range map[string]string{"a.png": ".png", ".png": "", "a.": "", "a.b.PNG": ".PNG", "a": ""} {
		if got := pySuffix(p); got != want {
			t.Errorf("%s: got %q", p, got)
		}
	}
	if pyQuote("it-illustration") != "'it-illustration'" {
		t.Error(pyQuote("it-illustration"))
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// hasReport finds a report "<text><elapsed> s". The elapsed seconds depend on
// the speed of the machine, so only the form is checked.
func hasReport(list []string, text string) bool {
	for _, x := range list {
		rest := strings.TrimPrefix(x, text)
		if rest != x && strings.HasSuffix(rest, " s") {
			if _, err := strconv.Atoi(strings.TrimSuffix(rest, " s")); err == nil {
				return true
			}
		}
	}
	return false
}
