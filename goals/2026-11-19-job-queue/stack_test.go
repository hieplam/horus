//go:build goal

// The parts of the oracles that run the test stack horus_test: a fake ComfyUI
// on the host, the stack of compose.yaml, and a minimal MCP client over the
// stdio of docker exec. Each row starts what it needs and stops it at its end,
// so that it runs on a clean PC with one command. The rows never touch the
// stack horus, and never a container that they did not start.
package goal

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The test stack: its compose project, its containers and its port on the host.
const (
	testStack   = "horus_test"
	testAPIPort = "18190"
	apiName     = testStack + "-api"
	workerName  = testStack + "-worker"
	dbName      = testStack + "-db"
)

// fakeComfy is a fake ComfyUI on the host. GET /system_stats answers 200.
// Each other request counts as a job that reached ComfyUI, and gets 500.
type fakeComfy struct {
	port int
	mu   sync.Mutex
	srv  *http.Server
	jobs atomic.Int32
	// broken makes GET /system_stats answer 500: the port answers, and
	// ComfyUI does not work.
	broken atomic.Bool
}

// startFake starts the fake on 127.0.0.1 of a free port. Docker Desktop
// gives a container the loopback of the host at host.docker.internal.
func startFake(t *testing.T) *fakeComfy {
	t.Helper()
	f := &fakeComfy{}
	if err := f.listen("127.0.0.1:0"); err != nil {
		t.Fatalf("the fake ComfyUI does not start: %v", err)
	}
	t.Cleanup(f.stop)
	return f
}

func (f *fakeComfy) listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	f.port = ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/system_stats" {
			if f.broken.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error": "the fake ComfyUI of the goal suite is broken"}`))
				return
			}
			_, _ = w.Write([]byte(`{"system": {"comfyui_version": "fake"}, "devices": []}`))
			return
		}
		f.jobs.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "the fake ComfyUI of the goal suite runs no job"}`))
	})}
	f.mu.Lock()
	f.srv = srv
	f.mu.Unlock()
	go func() { _ = srv.Serve(ln) }()
	return nil
}

// stop closes the fake: a request gets "connection refused".
func (f *fakeComfy) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.srv != nil {
		_ = f.srv.Close()
		f.srv = nil
	}
}

// start opens the fake again on the same port.
func (f *fakeComfy) start(t *testing.T) {
	t.Helper()
	if err := f.listen("127.0.0.1:" + strconv.Itoa(f.port)); err != nil {
		t.Fatalf("the fake ComfyUI does not start again on port %d: %v", f.port, err)
	}
}

// stackEnv is the environment of docker compose for the test stack. The
// password of the database and the bearer token of /mcp (HORUS_API_TOKEN) are
// new for each run, and they go only into the environment, never onto a
// command line or into a log.
func stackEnv(t *testing.T, f *fakeComfy) []string { return stackEnvFiles(t, f.port) }

// randomHex gives 16 random bytes as hex.
func randomHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// tokenOf gives HORUS_API_TOKEN of the environment env of a test stack.
func tokenOf(t *testing.T, env []string) string {
	t.Helper()
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], "HORUS_API_TOKEN="); ok && v != "" {
			return v
		}
	}
	t.Fatal("the environment of the test stack has no HORUS_API_TOKEN")
	return ""
}

// stackEnvFiles is stackEnv for a fake ComfyUI on port, with the compose
// files compose.yaml and then each of files. COMPOSE_FILE names the files, so
// docker compose never reads a compose.override.yaml of the owner in the
// repo root.
func stackEnvFiles(t *testing.T, port int, files ...string) []string {
	t.Helper()
	compose, err := filepath.Abs(filepath.Join(root, "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sep := string(os.PathListSeparator)
	return append(os.Environ(), "HORUS_STACK="+testStack, "HORUS_API_PORT="+testAPIPort,
		"HORUS_COMFYUI_URL=http://host.docker.internal:"+strconv.Itoa(port), "HORUS_DB_PASSWORD="+randomHex(t),
		"HORUS_API_TOKEN="+randomHex(t),
		"COMPOSE_FILE="+strings.Join(append([]string{compose}, files...), sep), "COMPOSE_PATH_SEPARATOR="+sep)
}

// runEnv is run with the environment env.
func runEnv(t *testing.T, env []string, name string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = root, env
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), out.String()
	}
	if err != nil {
		t.Fatalf("%s did not start: %v", name, err)
	}
	return 0, out.String()
}

// compose runs docker compose -p horus_test with args.
func compose(t *testing.T, env []string, args ...string) (int, string) {
	t.Helper()
	return runEnv(t, env, "docker", append([]string{"compose", "-p", testStack}, args...)...)
}

// newStack checks Docker, removes a test stack of an earlier run, and builds
// the image of horus. The cleanup removes the stack, its volume and its
// network. The image build is not a part of the timed start.
func newStack(t *testing.T, f *fakeComfy) []string {
	t.Helper()
	return prepareStack(t, stackEnv(t, f))
}

// prepareStack is newStack for the environment env of docker compose.
func prepareStack(t *testing.T, env []string) []string {
	t.Helper()
	if code, out := run(t, root, "docker", "version", "--format", "{{.Server.Version}}"); code != 0 {
		t.Fatalf("Docker does not answer: start Docker Desktop. docker version: exit code %d\n%s", code, out)
	}
	compose(t, env, "down", "-v", "--remove-orphans")
	t.Cleanup(func() {
		if code, out := compose(t, env, "down", "-v", "--remove-orphans"); code != 0 {
			t.Errorf("docker compose -p %s down -v: exit code %d\n%s", testStack, code, out)
		}
	})
	if code, out := compose(t, env, "build"); code != 0 {
		t.Fatalf("docker compose -p %s build: exit code %d\n%s", testStack, code, out)
	}
	return env
}

// upWait runs docker compose up -d --wait, and gives its time.
func upWait(t *testing.T, env []string) time.Duration {
	t.Helper()
	start := time.Now()
	code, out := compose(t, env, "up", "-d", "--wait", "--wait-timeout", "120")
	took := time.Since(start)
	if code != 0 {
		_, ps := compose(t, env, "ps", "-a")
		_, logs := compose(t, env, "logs", "--no-color", "--tail", "30")
		t.Fatalf("docker compose -p %s up -d --wait: exit code %d after %s\n%s\n%s\n%s", testStack, code, took.Round(time.Second), out, ps, logs)
	}
	return took
}

// psRow is one line of docker compose ps --format json.
type psRow struct {
	Name, Service, State, Health string
}

func ps(t *testing.T, env []string) []psRow {
	t.Helper()
	code, out := compose(t, env, "ps", "-a", "--format", "json")
	if code != 0 {
		t.Fatalf("docker compose ps: exit code %d\n%s", code, out)
	}
	var rows []psRow
	if err := json.Unmarshal([]byte(out), &rows); err == nil {
		return rows
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var r psRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("docker compose ps: a line that is not JSON: %q", line)
		}
		rows = append(rows, r)
	}
	return rows
}

// session is one MCP session over the stdio of docker exec -i horus_test-api
// horus mcp: newline-delimited JSON-RPC.
type session struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	lines  chan string
	stderr *lockedBuffer
	exited chan struct{}
	exit   int
	mu     sync.Mutex // one write at a time, and the id
	id     int
	// progress gets each progress notification of the session, with the
	// time when it came. A full channel drops the message.
	progress chan progressNote
}

// progressNote is one progress notification and the time when it came.
type progressNote struct {
	at      time.Time
	message string
}

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

// errNoSession is the error of openSession when the process ends before the
// answer to initialize: its exit code and its error output.
type errNoSession struct {
	exit   int
	stderr string
}

func (e *errNoSession) Error() string {
	return fmt.Sprintf("the session did not start: exit code %d: %s", e.exit, strings.TrimSpace(e.stderr))
}

// openSession starts docker exec -i horus_test-api horus mcp and sends
// initialize. It gives *errNoSession when the process ends first. Each of
// env (NAME=value) is a setting of that horus mcp process (docker exec -e).
func openSession(t *testing.T, within time.Duration, env ...string) (*session, error) {
	t.Helper()
	s := &session{lines: make(chan string, 64), stderr: &lockedBuffer{}, exited: make(chan struct{}),
		progress: make(chan progressNote, 1024)}
	args := []string{"exec", "-i"}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	s.cmd = exec.Command("docker", append(args, apiName, "horus", "mcp")...)
	s.cmd.Stderr = s.stderr
	in, err := s.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	// exec copies stdout into pw, and Wait returns after the last byte, so
	// the reader below gets each line before the pipe closes.
	pr, pw := io.Pipe()
	s.cmd.Stdout = pw
	if err := s.cmd.Start(); err != nil {
		return nil, err
	}
	s.in = in
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 1<<20), 16<<20)
		for sc.Scan() {
			s.lines <- sc.Text()
		}
		close(s.lines)
	}()
	go func() {
		err := s.cmd.Wait()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			s.exit = exit.ExitCode()
		}
		_ = pw.Close()
		close(s.exited)
	}()
	_, err = s.request("initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "goal-suite", "version": "0"}}, within)
	if err != nil {
		s.close()
		return nil, err
	}
	if err := s.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		s.close()
		return nil, err
	}
	return s, nil
}

func (s *session) send(msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.in.Write(append(b, '\n'))
	return err
}

// request sends one request and waits for its answer. Only one request of a
// session waits at a time.
func (s *session) request(method string, params any, within time.Duration) (json.RawMessage, error) {
	s.mu.Lock()
	s.id++
	id := s.id
	s.mu.Unlock()
	if err := s.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		<-s.exited
		return nil, &errNoSession{exit: s.exit, stderr: s.stderr.String()}
	}
	deadline := time.After(within)
	for {
		select {
		case line, ok := <-s.lines:
			if !ok {
				<-s.exited
				return nil, &errNoSession{exit: s.exit, stderr: s.stderr.String()}
			}
			var m struct {
				ID     *int            `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if json.Unmarshal([]byte(line), &m) != nil {
				continue
			}
			if m.Method == "notifications/progress" {
				var p struct {
					Message string `json:"message"`
				}
				_ = json.Unmarshal(m.Params, &p)
				select {
				case s.progress <- progressNote{at: time.Now(), message: p.Message}:
				default:
				}
				continue
			}
			if m.ID == nil || *m.ID != id {
				continue // another notification, or a line that is not ours
			}
			if len(m.Error) > 0 {
				return nil, fmt.Errorf("%s: JSON-RPC error %s", method, m.Error)
			}
			return m.Result, nil
		case <-deadline:
			return nil, fmt.Errorf("%s: no answer in %s", method, within)
		}
	}
}

// envelope is the envelope of a tool result.
type envelope struct {
	OK     bool           `json:"ok"`
	Result map[string]any `json:"result"`
	Error  *struct {
		Code     string         `json:"code"`
		ActionBy string         `json:"action_by"`
		Message  string         `json:"message"`
		Details  map[string]any `json:"details"`
	} `json:"error"`
}

// call calls a tool and gives its envelope.
func (s *session) call(tool string, args map[string]any, within time.Duration) (envelope, error) {
	return s.callToken(tool, args, "", within)
}

// callToken calls a tool with the progress token token ("" sends none), and
// gives its envelope.
func (s *session) callToken(tool string, args map[string]any, token string, within time.Duration) (envelope, error) {
	params := map[string]any{"name": tool, "arguments": args}
	if token != "" {
		params["_meta"] = map[string]any{"progressToken": token}
	}
	raw, err := s.request("tools/call", params, within)
	if err != nil {
		return envelope{}, err
	}
	var res struct {
		StructuredContent envelope `json:"structuredContent"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return envelope{}, fmt.Errorf("%s: the result is not a tool result: %s", tool, raw)
	}
	return res.StructuredContent, nil
}

// lastID is the id of the last request of the session.
func (s *session) lastID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

// cancel sends the MCP cancel of the request id: the client cancels the call.
func (s *session) cancel(id int) error {
	return s.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/cancelled",
		"params": map[string]any{"requestId": id, "reason": "the goal suite cancels the call"}})
}

func (s *session) close() {
	go func() {
		for range s.lines { // read the rest, so that the copy of stdout ends
		}
	}()
	_ = s.in.Close()
	select {
	case <-s.exited:
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
		<-s.exited
	}
}

// downNames gives details.down of a NOT_READY envelope.
func downNames(e envelope) []string {
	if e.Error == nil || e.Error.Code != "NOT_READY" {
		return nil
	}
	list, _ := e.Error.Details["down"].([]any)
	var out []string
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// containerID gives the full id of a container of the test stack.
func containerID(t *testing.T, name string) string {
	t.Helper()
	code, out := run(t, root, "docker", "inspect", "-f", "{{.Id}}", name)
	if code != 0 {
		t.Fatalf("docker inspect %s: exit code %d\n%s", name, code, out)
	}
	return strings.TrimSpace(out)
}

// eventually runs check until it gives "" or the time is over. It gives the
// time that it took, and the last answer of check.
func eventually(within time.Duration, check func() string) (time.Duration, string) {
	start := time.Now()
	for {
		why := check()
		if why == "" || time.Since(start) > within {
			return time.Since(start), why
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// eventuallyBy is eventually with the limit of a goal row: when check gives ""
// after within, it gives a reason that names the time, because a check that
// ends late is not a pass of a row with a time threshold.
func eventuallyBy(within time.Duration, check func() string) (time.Duration, string) {
	took, why := eventually(within, check)
	if why == "" && took > within {
		why = fmt.Sprintf("passed after %s, more than %s", took.Round(100*time.Millisecond), within)
	}
	return took, why
}

// hasAll tells if list holds each of want.
func hasAll(list []string, want ...string) bool {
	for _, w := range want {
		if !slices.Contains(list, w) {
			return false
		}
	}
	return true
}
