//go:build goal

// The parts of the oracles of generate_image through the job queue (task
// #202): a fake ComfyUI on the host that runs jobs, the test stack with a
// shared folder that horus stack-mounts writes, the peak tool against the
// stack, and the reads of horus.jobs. Each row starts what it needs and
// removes it at its end.
package goal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// goalSubject is the subject of the calls of the rows.
const goalSubject = "On the left, a mint-green server box sends sun-yellow envelopes onto a lilac conveyor belt. " +
	"On the right, a sky-blue server box takes them off the belt."

// sharedContainer is the container folder of the shared folder of a row.
const sharedContainer = "/shared/goal"

// jobFake is a fake ComfyUI on 127.0.0.1 of the host that runs jobs, one at
// a time as ComfyUI does. Each job runs for run. It answers the requests of
// the image service: /system_stats, /object_info (tests/fixtures), /prompt,
// /history, /queue, /view, /interrupt, the delete of /queue and /free.
type jobFake struct {
	mu      sync.Mutex
	port    int
	srv     *http.Server
	run     time.Duration
	t0      time.Time
	info    []byte
	png     []byte
	jobs    []*fakeJob // in submit order
	maxHeld int        // the most jobs that the fake held at one time, running and waiting
}

type fakeJob struct {
	id, state          string // queued, running, done, interrupted, deleted
	seed               int64
	submit, start, end time.Time
}

func startJobFake(t *testing.T, run time.Duration) *jobFake {
	t.Helper()
	info, err := os.ReadFile(filepath.Join(root, "tests", "fixtures", "object_info.json"))
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	f := &jobFake{run: run, t0: time.Now(), info: info, png: pngData.Bytes()}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("the fake ComfyUI does not start: %v", err)
	}
	f.port = ln.Addr().(*net.TCPAddr).Port
	f.srv = &http.Server{Handler: http.HandlerFunc(f.serve)}
	go func() { _ = f.srv.Serve(ln) }()
	t.Cleanup(func() { _ = f.srv.Close() })
	return f
}

func (f *jobFake) serve(w http.ResponseWriter, r *http.Request) {
	reply := func(v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	p := r.URL.Path
	switch {
	case r.Method == http.MethodGet && p == "/system_stats":
		reply(map[string]any{"system": map[string]any{"comfyui_version": "fake"}, "devices": []any{}})
	case r.Method == http.MethodGet && p == "/object_info":
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(f.info)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/history/"):
		reply(f.history(strings.TrimPrefix(p, "/history/")))
	case r.Method == http.MethodGet && p == "/queue":
		reply(f.queue())
	case r.Method == http.MethodGet && p == "/view":
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(f.png)
	case r.Method == http.MethodPost && p == "/prompt":
		var body struct {
			Prompt map[string]struct {
				ClassType string                     `json:"class_type"`
				Inputs    map[string]json.RawMessage `json:"inputs"`
			} `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var seed int64
		for _, n := range body.Prompt {
			if n.ClassType == "KSampler" {
				_ = json.Unmarshal(n.Inputs["seed"], &seed)
			}
		}
		id, number := f.submit(seed)
		reply(map[string]any{"prompt_id": id, "number": number, "node_errors": map[string]any{}})
	case r.Method == http.MethodPost && p == "/interrupt":
		var body struct {
			PromptID string `json:"prompt_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.interrupt(body.PromptID)
		reply(map[string]any{})
	case r.Method == http.MethodPost && p == "/queue":
		var body struct {
			Delete []string `json:"delete"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.delete(body.Delete)
		reply(map[string]any{})
	case r.Method == http.MethodPost && p == "/free":
		reply(map[string]any{})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// advance moves the jobs with the clock: a running job ends at its end time,
// and the next queued job starts at that time.
func (f *jobFake) advance(now time.Time) {
	for {
		var running, next *fakeJob
		for _, j := range f.jobs {
			switch {
			case j.state == "running":
				running = j
			case j.state == "queued" && next == nil:
				next = j
			}
		}
		switch {
		case running != nil && !now.Before(running.end):
			running.state = "done"
			if next != nil {
				next.state, next.start, next.end = "running", running.end, running.end.Add(f.run)
			}
		case running == nil && next != nil:
			next.state, next.start, next.end = "running", next.submit, next.submit.Add(f.run)
		default:
			return
		}
	}
}

// count notes the most jobs that the fake holds.
func (f *jobFake) count() {
	held := 0
	for _, j := range f.jobs {
		if j.state == "running" || j.state == "queued" {
			held++
		}
	}
	f.maxHeld = max(f.maxHeld, held)
}

func (f *jobFake) submit(seed int64) (string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	f.advance(now)
	j := &fakeJob{id: "job-" + strconv.Itoa(len(f.jobs)+1), state: "queued", seed: seed, submit: now}
	f.jobs = append(f.jobs, j)
	f.advance(now)
	f.count()
	return j.id, len(f.jobs)
}

func (f *jobFake) find(id string) *fakeJob {
	for _, j := range f.jobs {
		if j.id == id {
			return j
		}
	}
	return nil
}

func (f *jobFake) interrupt(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	f.advance(now)
	for _, j := range f.jobs {
		if j.state == "running" && (id == "" || id == j.id) {
			j.state, j.end = "interrupted", now
		}
	}
	f.advance(now)
}

func (f *jobFake) delete(ids []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advance(time.Now())
	for _, id := range ids {
		if j := f.find(id); j != nil && j.state == "queued" {
			j.state = "deleted"
		}
	}
}

func (f *jobFake) history(id string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advance(time.Now())
	j := f.find(id)
	if j == nil || j.state != "done" {
		return map[string]any{}
	}
	ms := func(at time.Time) int64 { return at.Sub(f.t0).Milliseconds() }
	return map[string]any{id: map[string]any{
		"status": map[string]any{"status_str": "success", "completed": true, "messages": []any{
			[]any{"execution_start", map[string]any{"timestamp": ms(j.start)}},
			[]any{"execution_success", map[string]any{"timestamp": ms(j.end)}}}},
		"outputs": map[string]any{"9": map[string]any{"images": []any{
			map[string]any{"filename": id + ".png", "subfolder": "", "type": "output"}}}}}}
}

func (f *jobFake) queue() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advance(time.Now())
	running, pending := []any{}, []any{}
	for i, j := range f.jobs {
		row := []any{i + 1, j.id, map[string]any{}}
		switch j.state {
		case "running":
			running = append(running, row)
		case "queued":
			pending = append(pending, row)
		}
	}
	return map[string]any{"queue_running": running, "queue_pending": pending}
}

// submits gives the count of the jobs with the seed that reached the fake.
func (f *jobFake) submits(seed int64) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, j := range f.jobs {
		if j.seed == seed {
			n++
		}
	}
	return n
}

// idle tells if the fake holds no job.
func (f *jobFake) idle() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advance(time.Now())
	for _, j := range f.jobs {
		if j.state == "running" || j.state == "queued" {
			return false
		}
	}
	return true
}

// goEnv is one value of go env, for example GOOS or GOEXE. The suite imports
// only the standard library, and runtime.GOOS is a fact of internal/platform.
func goEnv(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command("go", "env", name).Output()
	if err != nil {
		t.Fatalf("go env %s: %v", name, err)
	}
	return strings.TrimSpace(string(out))
}

// buildHorus builds the program horus of this repo into a temporary folder.
func buildHorus(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "horus"+goEnv(t, "GOEXE"))
	if code, out := run(t, root, "go", "build", "-o", exe, "./cmd/horus"); code != 0 {
		t.Fatalf("go build ./cmd/horus: exit code %d\n%s", code, out)
	}
	return exe
}

// queueStack is the test stack of a row, with one shared folder and one
// models folder.
type queueStack struct {
	env []string
	// shared is the shared folder on the host: HORUS_SHARED_DIRS pairs it
	// with the container folder /shared/goal.
	shared string
	// models is the models folder of the test on the host: horus
	// stack-mounts mounts it into horus-api as HORUS_MODELS_DIR. It is a new
	// test folder, so a test stack never mounts the model files of the
	// machine, also when the environment of the test sets HORUS_MODELS_DIR.
	models string
	// workflows is the workflows folder of the test on the host, or "":
	// tests/stack-sandbox.yaml mounts it into the stack as
	// HORUS_WORKFLOWS_DIR (newSandboxStack).
	workflows string
}

// newQueueStack prepares the test stack for a fake ComfyUI on port: a new
// shared folder and a new models folder, the override file that horus
// stack-mounts writes for them, and the settings of the stack (NAME=value) in
// settings. The row starts the stack with upWait.
func newQueueStack(t *testing.T, port int, settings ...string) *queueStack {
	t.Helper()
	return queueStackWith(t, port, false, settings)
}

// newSandboxStack is newQueueStack with a new workflows folder too: the
// compose file tests/stack-sandbox.yaml mounts it into horus-api and
// horus-worker as HORUS_WORKFLOWS_DIR. The contract test writes its
// workflows there, and its stand-in model files into the models folder.
func newSandboxStack(t *testing.T, port int, settings ...string) *queueStack {
	t.Helper()
	return queueStackWith(t, port, true, settings)
}

func queueStackWith(t *testing.T, port int, sandbox bool, settings []string) *queueStack {
	t.Helper()
	exe := buildHorus(t)
	st := &queueStack{shared: t.TempDir(), models: t.TempDir()}
	override := filepath.Join(t.TempDir(), "override.yaml")
	cmd := exec.Command(exe, "stack-mounts", "-o", override)
	cmd.Env = append(os.Environ(), "HORUS_SHARED_DIRS="+st.shared+"="+sharedContainer, "HORUS_MODELS_DIR="+st.models)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("horus stack-mounts: %v\n%s", err, out)
	}
	files := []string{override}
	if sandbox {
		st.workflows = t.TempDir()
		file, err := filepath.Abs(filepath.Join(root, "tests", "stack-sandbox.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
		settings = append(settings, "HORUS_TEST_WORKFLOWS_DIR="+st.workflows)
	}
	st.env = prepareStack(t, append(stackEnvFiles(t, port, files...), settings...))
	return st
}

// e2eStack starts a fake ComfyUI whose jobs run for runTime, and the test
// stack with one slot, and waits until the stack is ready.
func e2eStack(t *testing.T, runTime time.Duration) (*jobFake, *queueStack) {
	t.Helper()
	f := startJobFake(t, runTime)
	st := newQueueStack(t, f.port)
	upWait(t, st.env)
	return f, st
}

// query runs SQL in horus-db of the test stack, and gives the rows, each
// row as its fields.
func query(t *testing.T, sql string) [][]string {
	t.Helper()
	code, out := run(t, root, "docker", "exec", dbName, "psql", "-U", "horus", "-d", "horus", "-At", "-F", "\t", "-c", sql)
	if code != 0 {
		t.Fatalf("psql %q: exit code %d\n%s", sql, code, out)
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimRight(strings.ReplaceAll(out, "\r\n", "\n"), "\n"), "\n") {
		if line != "" {
			rows = append(rows, strings.Split(line, "\t"))
		}
	}
	return rows
}

// stateOfSeed gives the state of the job of the call with the seed.
func stateOfSeed(t *testing.T, seed int) string {
	t.Helper()
	rows := query(t, fmt.Sprintf("select state from horus.jobs where payload->>'seed' = '%d'", seed))
	if len(rows) != 1 {
		return fmt.Sprintf("%d jobs", len(rows))
	}
	return rows[0][0]
}

// waitFor runs check every 200 ms until it gives "" or the time is over, and
// gives its last answer.
func waitFor(within time.Duration, check func() string) string {
	deadline := time.Now().Add(within)
	for {
		why := check()
		if why == "" || time.Now().After(deadline) {
			return why
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// pythonOf is the Python of the test tools: HORUS_TEST_PYTHON, then the venv
// .scratch/test-venv of the repo, then python3 and python. It must have the
// package mcp (tests/requirements.txt).
func pythonOf(t *testing.T) string {
	t.Helper()
	win, _ := filepath.Abs(filepath.Join(root, ".scratch", "test-venv", "Scripts", "python.exe"))
	unix, _ := filepath.Abs(filepath.Join(root, ".scratch", "test-venv", "bin", "python"))
	for _, name := range []string{os.Getenv("HORUS_TEST_PYTHON"), win, unix, "python3", "python"} {
		if name != "" && exec.Command(name, "-c", "import mcp").Run() == nil {
			return name
		}
	}
	t.Fatal("no Python with the package mcp: set HORUS_TEST_PYTHON, or run pip install -r tests/requirements.txt (docs/mcp.md, \"Test the server\")")
	return ""
}

// freePort gives a free port of 127.0.0.1.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// peakResult is the JSON line of tests/peak_load.py.
type peakResult struct {
	PNG               int     `json:"png"`
	GenerationTimeout int     `json:"generation_timeout"`
	QueueFull         int     `json:"queue_full"`
	WaitForError      float64 `json:"wait_for_error_seconds"`
	GPUForNobody      float64 `json:"gpu_seconds_for_nobody"`
	MaxJobs           int     `json:"max_jobs_in_comfyui"`
	MaxRunning        int     `json:"max_running"`
	Left              []int   `json:"left"`
	JobsInComfyUI     int     `json:"jobs_in_comfyui"`
}

// peakRun runs the peak tool against a new test stack whose worker has slots
// slots. The settings of the stack are those of --scale 10: one image takes
// 24 s, HORUS_GENERATE_TIMEOUT 180, HORUS_QUEUE_TIMEOUT 720 and
// HORUS_RUN_ESTIMATE 24. args are the arguments of the tool. It gives the
// JSON line and the lines of the calls.
func peakRun(t *testing.T, slots int, args ...string) (peakResult, string) {
	t.Helper()
	py := pythonOf(t)
	port := freePort(t)
	st := newQueueStack(t, port, "HORUS_WORKER_CONCURRENCY="+strconv.Itoa(slots), "HORUS_GENERATE_TIMEOUT=180",
		"HORUS_QUEUE_TIMEOUT=720", "HORUS_RUN_ESTIMATE=24")
	// The tool starts its fake, then waits until the stack is ready: the
	// stack reaches the fake as its ComfyUI.
	cmd := exec.Command(py, append([]string{filepath.Join("tests", "peak_load.py"), "--target", "stack", "--stack", testStack,
		"--port", strconv.Itoa(port), "--out-dir", st.shared}, args...)...)
	cmd.Dir = root
	var stdout bytes.Buffer
	stderr := &lockedBuffer{}
	cmd.Stdout, cmd.Stderr = &stdout, stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("tests/peak_load.py did not start: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	upWait(t, st.env)
	var err error
	select {
	case err = <-done:
	case <-time.After(20 * time.Minute):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("tests/peak_load.py did not end in 20 minutes:\n%s", stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var res peakResult
	if jerr := json.Unmarshal([]byte(lines[len(lines)-1]), &res); err != nil || jerr != nil {
		t.Fatalf("tests/peak_load.py: %v, last line %q:\n%s", errors.Join(err, jerr), lines[len(lines)-1], stderr.String())
	}
	var calls []string
	for _, line := range strings.Split(stderr.String(), "\n") {
		if strings.HasPrefix(line, "call ") || strings.HasPrefix(line, "fake: ") {
			calls = append(calls, strings.TrimRight(line, "\r"))
		}
	}
	return res, strings.Join(append(calls, lines[len(lines)-1]), "\n")
}

// peakTwelve is the run of the rows peak-timeouts and bounded-backend-queue:
// 12 calls at the same time, the agents of the calls 3 and 6 leave after
// 60 / 10 s, one slot. The first row that needs it runs it, and the other
// row reads the same run.
var peakTwelve struct {
	once sync.Once
	res  peakResult
	log  string
	ran  bool
}

func twelveCalls(t *testing.T) (peakResult, string) {
	t.Helper()
	peakTwelve.once.Do(func() {
		peakTwelve.res, peakTwelve.log = peakRun(t, 1, "--calls", "12", "--leave", "3,6")
		peakTwelve.ran = true
	})
	if !peakTwelve.ran {
		t.Fatal("the peak run of 12 calls failed in an earlier row: see its output")
	}
	return peakTwelve.res, peakTwelve.log
}

// recipeText is the fingerprint text of recipe version 1 (ADR 0027), as the
// oracle computes it from the request: no spaces, the keys in this order.
type recipeText struct {
	V           int     `json:"v"`
	Tool        string  `json:"tool"`
	Workflow    string  `json:"workflow"`
	WorkflowRev string  `json:"workflow_rev"`
	Subject     string  `json:"subject"`
	Seed        *string `json:"seed"`
	Width       int64   `json:"width"`
	Height      int64   `json:"height"`
	OutPath     *string `json:"out_path"`
	Overwrite   bool    `json:"overwrite"`
}

// fingerprintOf is the request fingerprint of a call of generate_image with
// the built-in workflow it-illustration: the SHA-256 of the text of recipe
// version 1. outPath is a host path of this machine.
func fingerprintOf(t *testing.T, subject string, seed uint64, outPath string, overwrite bool) string {
	t.Helper()
	dir := filepath.Join(root, "workflows", "it-illustration")
	manifest, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := os.ReadFile(filepath.Join(dir, "workflow.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Output struct {
			Width  int64 `json:"default_width"`
			Height int64 `json:"default_height"`
		} `json:"output"`
	}
	if err := json.Unmarshal(manifest, &m); err != nil {
		t.Fatal(err)
	}
	rev := sha256.New()
	rev.Write(manifest)
	rev.Write([]byte{0})
	rev.Write(graph)
	s := strconv.FormatUint(seed, 10)
	p := filepath.Clean(outPath)
	if goos := goEnv(t, "GOOS"); goos == "windows" || goos == "darwin" {
		p = strings.ToLower(p) // the file systems of Windows and macOS do not tell the case apart
	}
	text := recipeText{V: 1, Tool: "generate_image", Workflow: "it-illustration", WorkflowRev: hex.EncodeToString(rev.Sum(nil)),
		Subject: strings.Join(strings.Fields(subject), " "), Seed: &s, Width: m.Output.Width, Height: m.Output.Height,
		OutPath: &p, Overwrite: overwrite}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(text); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes.TrimSuffix(b.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:])
}

// isPNG tells if the file at path starts with the PNG signature.
func isPNG(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
}
