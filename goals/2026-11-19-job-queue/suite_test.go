//go:build goal

// Package goal holds the goal rows of docs/specs/2026-11-19-job-queue.md:
// one test function for each row. Run it with
//
//	go test -tags goal -v ./goals/2026-11-19-job-queue
package goal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// The task that builds the outcome of a row gives the row its oracle.

// root is the repo root. go test starts a test in the folder of its package.
var root = filepath.Join("..", "..")

// run starts a program in the folder dir. It returns the exit code and the
// output. A program that does not start fails the row.
func run(t *testing.T, dir, name string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
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

// passed tells if the go test output has "--- PASS" for the test name.
func passed(out, name string) bool {
	return regexp.MustCompile(`(?m)^\s*--- PASS: ` + regexp.QuoteMeta(name) + ` `).MatchString(out)
}

// TestPeakTimeouts runs the peak tool against the test stack: 12 agents call
// generate_image at the same time, and the agents of the calls 3 and 6 leave
// after 60 / 10 s. Each of the 10 agents that stay gets its PNG at its host
// path in the shared folder, no call ends with GENERATION_TIMEOUT, and the
// fake ComfyUI runs no job of an agent that left.
func TestPeakTimeouts(t *testing.T) {
	res, log := twelveCalls(t)
	t.Logf("the peak run:\n%s", log)
	if !slices.Equal(res.Left, []int{3, 6}) {
		t.Fatalf("the agents that left: %v, want [3 6]: a run with no leave proves nothing about the leave", res.Left)
	}
	if res.JobsInComfyUI != 10 {
		t.Fatalf("jobs that reached ComfyUI: %d, want 10 (the 10 calls that stay, no job of an agent that left)", res.JobsInComfyUI)
	}
	if res.PNG != 10 || res.GenerationTimeout != 0 || res.GPUForNobody != 0 {
		t.Fatalf("png %d of 10, GENERATION_TIMEOUT %d, GPU seconds for nobody %.1f; want 10, 0 and 0",
			res.PNG, res.GenerationTimeout, res.GPUForNobody)
	}
	t.Logf("png: 10 of 10; GENERATION_TIMEOUT: %d; GPU seconds for nobody: %.1f", res.GenerationTimeout, res.GPUForNobody)
}

// TestBoundedBackendQueue reads the same run: the fake ComfyUI of the peak
// tool records the most jobs that it held at one time, running and waiting.
// With one slot, ComfyUI never holds more than one horus job.
func TestBoundedBackendQueue(t *testing.T) {
	res, log := twelveCalls(t)
	if res.JobsInComfyUI != 10 || res.PNG != 10 {
		t.Fatalf("jobs that reached ComfyUI: %d, PNG: %d, want 10 and 10: the bound is measured under the full load:\n%s",
			res.JobsInComfyUI, res.PNG, log)
	}
	if res.MaxJobs != 1 {
		t.Fatalf("the most jobs in ComfyUI at one time: %d, want 1 with 1 slot:\n%s", res.MaxJobs, log)
	}
	t.Logf("the most jobs in ComfyUI at one time: %d, with 1 slot (%d PNG)", res.MaxJobs, res.PNG)
}

// TestStackStartsTogether: with a fake ComfyUI on the host, one command,
// docker compose up -d --wait, starts the 3 containers of the test stack in
// their order (horus-db, horus-worker, horus-api), each one healthy, and
// horus ready in horus-api exits 0 with 4 ok lines, in 120 s or less from the
// start of the command. The build of the image comes before, and is not
// timed. Before the start, the row measures the health check of horus-db on
// a new volume: horus-db is never healthy while the database refuses TCP,
// because Docker Compose starts horus-worker when horus-db is healthy.
//
// Then the row proves that horus is ready because ComfyUI answers. It stops
// the fake ComfyUI: horus ready exits 1 with the line FAIL comfyui and 3 ok
// lines, and horus-api becomes unhealthy in docker compose ps. It starts the
// fake again: horus ready exits 0 with 4 ok lines, and horus-api is healthy
// again. A fake that answers GET /system_stats with 500 is down too.
func TestStackStartsTogether(t *testing.T) {
	fake := startFake(t)
	env := stackEnv(t, fake)
	checkDBHealthNeedsTCP(t, env)
	prepareStack(t, env)
	start := time.Now()
	took := upWait(t, env)

	rows := ps(t, env)
	healthy := 0
	names := []string{}
	for _, r := range rows {
		names = append(names, r.Name)
		if r.State == "running" && r.Health == "healthy" {
			healthy++
		}
	}
	if len(rows) != 3 || healthy != 3 || !hasAll(names, dbName, workerName, apiName) {
		t.Fatalf("docker compose ps: %+v, want the 3 containers %s, %s and %s, each running and healthy", rows, dbName, workerName, apiName)
	}

	code, out := run(t, root, "docker", "inspect", "-f", "{{.State.StartedAt}}", dbName, workerName, apiName)
	if code != 0 {
		t.Fatalf("docker inspect: exit code %d\n%s", code, out)
	}
	var started []time.Time
	for _, line := range strings.Fields(out) {
		at, err := time.Parse(time.RFC3339Nano, line)
		if err != nil {
			t.Fatalf("docker inspect: a start time that does not parse: %q", line)
		}
		started = append(started, at)
	}
	if len(started) != 3 || !started[0].Before(started[1]) || !started[1].Before(started[2]) {
		t.Errorf("the start order: %s, %s, %s started at %v; want horus-db, then horus-worker, then horus-api",
			dbName, workerName, apiName, started)
	}

	code, out = run(t, root, "docker", "exec", apiName, "horus", "ready")
	ready := time.Since(start)
	oks := strings.Count(out, "\nok   ")
	if code != 0 || oks != 4 {
		t.Fatalf("docker exec %s horus ready: exit code %d, %d ok lines, want 0 and 4:\n%s", apiName, code, oks, out)
	}
	if ready > 120*time.Second {
		t.Fatalf("3 healthy and horus ready exit 0 after %s, more than 120 s", ready.Round(time.Second))
	}
	t.Logf("containers healthy after one command: %d of 3; up -d --wait took %s; horus ready exit 0 after %s",
		healthy, took.Round(100*time.Millisecond), ready.Round(100*time.Millisecond))

	// horus becomes ready when ComfyUI answers. So while the fake ComfyUI does
	// not answer, horus ready exits 1 with the line FAIL comfyui and an ok
	// line for each of the 3 other dependencies. A horus ready that runs no
	// check fails here.
	fake.stop()
	if _, _, why := horusReady(t, []string{"comfyui"}); why != "" {
		t.Fatalf("the fake ComfyUI does not answer: %s", why)
	}
	// The health check of horus-api is horus ready, with an interval of 5 s,
	// a time limit of 4 s and 3 retries (compose.yaml). So Docker gives
	// horus-api as unhealthy in 3 * (5 + 4) s or less: "3 healthy" needs
	// ComfyUI. The 2 other containers do not need ComfyUI, and stay healthy.
	unhealthy, why := eventuallyBy(27*time.Second, func() string {
		if h := healthOf(t, env, apiName); h != "unhealthy" {
			return fmt.Sprintf("the health of %s in docker compose ps is %q, want unhealthy", apiName, h)
		}
		return ""
	})
	if why != "" {
		t.Fatalf("the fake ComfyUI does not answer for %s: %s", unhealthy.Round(time.Second), why)
	}
	for _, name := range []string{dbName, workerName} {
		if h := healthOf(t, env, name); h != "healthy" {
			t.Fatalf("the fake ComfyUI does not answer: the health of %s in docker compose ps is %q, want healthy", name, h)
		}
	}
	if _, _, why := horusReady(t, []string{"comfyui"}); why != "" {
		t.Fatalf("the fake ComfyUI does not answer, and %s is unhealthy: %s", apiName, why)
	}

	// The fake ComfyUI answers again: horus ready exits 0 with 4 ok lines at
	// once, and horus-api is healthy again at its next health check (5 s, and
	// 4 s for the check).
	fake.start(t)
	if _, _, why := horusReady(t, nil); why != "" {
		t.Fatalf("the fake ComfyUI answers again: %s", why)
	}
	again, why := eventuallyBy(9*time.Second, func() string {
		if h := healthOf(t, env, apiName); h != "healthy" {
			return fmt.Sprintf("the health of %s in docker compose ps is %q, want healthy", apiName, h)
		}
		return ""
	})
	if why != "" {
		t.Fatalf("the fake ComfyUI answers again for %s: %s", again.Round(time.Second), why)
	}
	t.Logf("the fake ComfyUI does not answer: horus ready exit 1 with FAIL comfyui and 3 ok lines, %s unhealthy after %s; "+
		"it answers again: horus ready exit 0 with 4 ok lines, %s healthy after %s",
		apiName, unhealthy.Round(100*time.Millisecond), apiName, again.Round(100*time.Millisecond))

	// "ComfyUI answers" is GET /system_stats with 200, not an open port: a
	// fake that answers 500 is down for horus ready.
	fake.broken.Store(true)
	if _, _, why := horusReady(t, []string{"comfyui"}); why != "" {
		t.Fatalf("the fake ComfyUI answers GET /system_stats with 500: %s", why)
	}
	fake.broken.Store(false)
	if _, _, why := horusReady(t, nil); why != "" {
		t.Fatalf("the fake ComfyUI answers GET /system_stats with 200 again: %s", why)
	}
	t.Log("the fake ComfyUI answers GET /system_stats with 500: horus ready exit 1 with FAIL comfyui; with 200 again: exit 0")
}

// oneStop is one dependency of the row ready-when-four-up: how to stop it and
// start it again, and the dependencies that the readiness report names while
// it is stopped.
type oneStop struct {
	dep string // the name in the readiness report
	// down is the whole list of the report while only dep is stopped, in the
	// order of the report. nil for horus-api: no report comes.
	down        []string
	stop, start func(t *testing.T)
}

// TestReadyWhenFourUp stops each of the 4 dependencies in turn, then the
// whole stack. Each time, over stdio (docker exec -i horus_test-api horus
// mcp) and over HTTP (/mcp of horus api on 127.0.0.1:18190, with the token of
// the test stack):
//
//   - a call in a session that is open gets NOT_READY that names the
//     dependency, and a new session does not start and names it, in 10 s or
//     less. Over stdio, the new session exits with the FAIL line of the
//     dependency. Over HTTP, it gets 503 with the readiness report that names
//     the dependency. When horus-api stops, docker exec fails and names its
//     container, and /mcp gives no HTTP answer;
//   - each answer names only the dependencies that the stop takes down: the
//     row compares the whole list, and a dependency that is up has an ok line
//     and is not in the list. A report that names each of the 4 dependencies,
//     or always the same one, fails;
//   - horus ready in horus-api and /readyz give the same report in the same
//     10 s: exit code 1 and 503;
//   - while the dependency is stopped, each tool of the server gets
//     NOT_READY with the same list, in the two open sessions;
//   - no request reaches ComfyUI while horus is not ready;
//   - after the dependency starts again, a new session starts and a call gets
//     no NOT_READY, in 30 s or less, over stdio and over HTTP, with no action
//     on horus. In the same 30 s, the two sessions that were open before the
//     stop serve again, horus ready exits 0 and /readyz answers 200.
//
// The list of each stop, by the design of the checks (docs/mcp.md, "The stack
// and its readiness"): the check of horus-worker reads the row of the worker
// and the worker lock in horus-db. So horus cannot check horus-worker without
// horus-db, and a stop of horus-db gives horus-db and horus-worker. The checks
// of ComfyUI and of horus-api do not use horus-db, and the check of horus-db
// uses no other dependency. So a stop of horus-worker gives only horus-worker,
// and a stop of ComfyUI gives only comfyui.
func TestReadyWhenFourUp(t *testing.T) {
	fake := startFake(t)
	env := newStack(t, fake)
	upWait(t, env)
	apiID := containerID(t, apiName)
	token := tokenOf(t, env)
	docker := func(args ...string) func(t *testing.T) {
		return func(t *testing.T) {
			t.Helper()
			if code, out := run(t, root, "docker", args...); code != 0 {
				t.Fatalf("docker %s: exit code %d\n%s", strings.Join(args, " "), code, out)
			}
		}
	}
	steps := []oneStop{
		{"horus-db", []string{"horus-db", "horus-worker"}, docker("stop", dbName), docker("start", dbName)},
		{"horus-worker", []string{"horus-worker"}, docker("stop", workerName), docker("start", workerName)},
		{"comfyui", []string{"comfyui"}, func(*testing.T) { fake.stop() }, fake.start},
		{"horus-api", nil, docker("stop", apiName), docker("start", apiName)},
	}
	call := map[string]any{"workflow": "it-illustration", "subject": "A falcon over a server rack. The picture contains no text."}
	notRunning := "container " + apiID + " is not running"

	// stdioDown checks over stdio that the stop of step is named: the call in
	// the open session, and a new session. It gives the lists that it read.
	stdioDown := func(step oneStop, open *session) (lists, why string) {
		dep := step.dep
		var inCall []string
		if dep != "horus-api" {
			e, err := open.call("generate_image", call, 15*time.Second)
			if err != nil {
				return "", "the call in the open stdio session: " + err.Error()
			}
			inCall = downNames(e)
			if !hasAll(inCall, dep) {
				return "", fmt.Sprintf("the call in the open stdio session: %+v, want NOT_READY with %s in details.down", e, dep)
			}
			if why := wrongNotReady(e, step.down); why != "" {
				return "", "the call in the open stdio session: " + why
			}
		}
		s, err := openSession(t, 15*time.Second)
		if err == nil {
			s.close()
			return "", "a new stdio session started"
		}
		var no *errNoSession
		if !errors.As(err, &no) {
			return "", "a new stdio session: " + err.Error()
		}
		want := "FAIL " + dep
		if dep == "horus-api" {
			want = notRunning
		}
		if no.exit == 0 || !strings.Contains(no.stderr, want) {
			return "", fmt.Sprintf("a new stdio session ended with exit code %d and does not say %q: %s", no.exit, want, no.stderr)
		}
		if dep == "horus-api" {
			return "docker exec fails, the container is not running", ""
		}
		if no.exit != 1 {
			return "", fmt.Sprintf("a new stdio session ended with exit code %d, want 1: %s", no.exit, no.stderr)
		}
		failed, why := wrongText(no.stderr, step.down)
		if why != "" {
			return "", fmt.Sprintf("the error output of a new stdio session: %s:\n%s", why, no.stderr)
		}
		return fmt.Sprintf("details.down %v, FAIL lines of a new session %v", inCall, failed), ""
	}
	// httpDown checks the same over HTTP.
	httpDown := func(step oneStop, open *httpSession) (lists, why string) {
		dep := step.dep
		var inCall []string
		if dep != "horus-api" {
			e, err := open.call("generate_image", call, 15*time.Second)
			if err != nil {
				return "", "the call in the open HTTP session: " + err.Error()
			}
			inCall = downNames(e)
			if !hasAll(inCall, dep) {
				return "", fmt.Sprintf("the call in the open HTTP session: %+v, want NOT_READY with %s in details.down", e, dep)
			}
			if why := wrongNotReady(e, step.down); why != "" {
				return "", "the call in the open HTTP session: " + why
			}
		}
		s, err := openHTTP(token, 15*time.Second)
		if err == nil {
			s.close()
			return "", "a new HTTP session started"
		}
		var no *errNoHTTP
		if !errors.As(err, &no) {
			return "", "a new HTTP session: " + err.Error()
		}
		if dep == "horus-api" {
			if no.status != 0 {
				return "", fmt.Sprintf("a new HTTP session while horus-api is stopped: %s, want no HTTP answer", no)
			}
			return "no HTTP answer", ""
		}
		r, ok := reportOf(no)
		if !ok || r.Ready || !hasAll(r.Down, dep) {
			return "", fmt.Sprintf("a new HTTP session: %s, want 503 with the readiness report that names %s", no, dep)
		}
		if why := wrongReport(r, step.down); why != "" {
			return "", fmt.Sprintf("the 503 of a new HTTP session: %s: %s", why, no)
		}
		return fmt.Sprintf("details.down %v, down of the 503 of a new session %v", inCall, r.Down), ""
	}
	// commandDown checks horus ready in horus-api: exit code 1 and the FAIL
	// lines of the stop. When horus-api is stopped, docker exec fails.
	commandDown := func(step oneStop) (lists, why string) {
		if step.dep == "horus-api" {
			code, out := run(t, root, "docker", "exec", apiName, "horus", "ready")
			if code == 0 || !strings.Contains(out, notRunning) {
				return "", fmt.Sprintf("docker exec %s horus ready: exit code %d, and it does not say %q: %s", apiName, code, notRunning, out)
			}
			return "docker exec fails, the container is not running", ""
		}
		_, out, why := horusReady(t, step.down)
		if why != "" {
			return "", why
		}
		failed, _ := wrongText(out, step.down)
		return fmt.Sprintf("exit code 1, FAIL lines %v", failed), ""
	}
	// pageDown checks /readyz: 503 with the report of the stop. When
	// horus-api is stopped, it gives no HTTP answer.
	pageDown := func(step oneStop) (lists, why string) {
		if step.dep == "horus-api" {
			status, _, body, err := getReadyz()
			if err == nil || status != 0 {
				return "", fmt.Sprintf("GET /readyz while horus-api is stopped: HTTP %d, want no HTTP answer: %s", status, body)
			}
			return "no HTTP answer", ""
		}
		down, why := readyz(step.down)
		if why != "" {
			return "", why
		}
		return fmt.Sprintf("503, down %v", down), ""
	}
	// serves checks that horus serves again: a new session starts and
	// list_workflows works, over stdio and over HTTP, horus ready exits 0 with
	// 4 ok lines, and /readyz answers 200. open and hopen are the sessions
	// that were open before the stop (nil: the stop ended them): a call in
	// each one works again, with no action on horus.
	serves := func(open *session, hopen *httpSession) string {
		s, err := openSession(t, 15*time.Second)
		if err != nil {
			return "a new stdio session: " + err.Error()
		}
		defer s.close()
		if e, err := s.call("list_workflows", map[string]any{}, 15*time.Second); err != nil || !e.OK {
			return fmt.Sprintf("list_workflows over stdio: %+v %v", e, err)
		}
		h, err := openHTTP(token, 15*time.Second)
		if err != nil {
			return "a new HTTP session: " + err.Error()
		}
		defer h.close()
		if e, err := h.call("list_workflows", map[string]any{}, 15*time.Second); err != nil || !e.OK {
			return fmt.Sprintf("list_workflows over HTTP: %+v %v", e, err)
		}
		if open != nil {
			if e, err := open.call("list_workflows", map[string]any{}, 15*time.Second); err != nil || !e.OK {
				return fmt.Sprintf("list_workflows in the stdio session that was open before the stop: %+v %v", e, err)
			}
		}
		if hopen != nil {
			if e, err := hopen.call("list_workflows", map[string]any{}, 15*time.Second); err != nil || !e.OK {
				return fmt.Sprintf("list_workflows in the HTTP session that was open before the stop: %+v %v", e, err)
			}
		}
		if _, _, why := horusReady(t, nil); why != "" {
			return why
		}
		if _, why := readyz(nil); why != "" {
			return why
		}
		return ""
	}

	named, namedHTTP := 0, 0
	for _, step := range steps {
		open, err := openSession(t, 60*time.Second)
		if err != nil {
			t.Fatalf("%s: before the stop, a stdio session does not start: %v", step.dep, err)
		}
		if e, err := open.call("list_workflows", map[string]any{}, 30*time.Second); err != nil || !e.OK {
			t.Fatalf("%s: before the stop, list_workflows over stdio: %+v %v", step.dep, e, err)
		}
		hopen, err := openHTTP(token, 60*time.Second)
		if err != nil {
			t.Fatalf("%s: before the stop, an HTTP session does not start: %v", step.dep, err)
		}
		if e, err := hopen.call("list_workflows", map[string]any{}, 30*time.Second); err != nil || !e.OK {
			t.Fatalf("%s: before the stop, list_workflows over HTTP: %+v %v", step.dep, e, err)
		}
		tools, err := toolNames(open)
		if err != nil || !hasAll(tools, "generate_image", "list_workflows") {
			t.Fatalf("%s: before the stop, tools/list over stdio: %v %v, want the tools of horus", step.dep, tools, err)
		}
		if _, _, why := horusReady(t, nil); why != "" {
			t.Fatalf("%s: before the stop: %s", step.dep, why)
		}
		if _, why := readyz(nil); why != "" {
			t.Fatalf("%s: before the stop: %s", step.dep, why)
		}
		jobsBefore := fake.jobs.Load()
		step.stop(t)
		// The 4 ways run at the same time, so that each one has its own
		// time from the stop.
		var wg sync.WaitGroup
		var stdioTook, httpTook, commandTook, pageTook time.Duration
		var stdioWhy, httpWhy, commandWhy, pageWhy string
		var stdioLists, httpLists, commandLists, pageLists string
		wg.Add(4)
		go func() {
			defer wg.Done()
			stdioTook, stdioWhy = eventuallyBy(10*time.Second, func() (why string) {
				stdioLists, why = stdioDown(step, open)
				return why
			})
		}()
		go func() {
			defer wg.Done()
			httpTook, httpWhy = eventuallyBy(10*time.Second, func() (why string) {
				httpLists, why = httpDown(step, hopen)
				return why
			})
		}()
		go func() {
			defer wg.Done()
			commandTook, commandWhy = eventuallyBy(10*time.Second, func() (why string) {
				commandLists, why = commandDown(step)
				return why
			})
		}()
		go func() {
			defer wg.Done()
			pageTook, pageWhy = eventuallyBy(10*time.Second, func() (why string) {
				pageLists, why = pageDown(step)
				return why
			})
		}()
		wg.Wait()
		if step.dep == "horus-api" {
			// The stop ended the two sessions.
			open.close()
			hopen.close()
			open, hopen = nil, nil
		}
		closeOpen := func() {
			if open != nil {
				open.close()
				hopen.close()
			}
		}
		if stdioWhy != "" {
			closeOpen()
			t.Fatalf("%s is stopped: not named over stdio in 10 s: %s", step.dep, stdioWhy)
		}
		if httpWhy != "" {
			closeOpen()
			t.Fatalf("%s is stopped: not named over HTTP in 10 s: %s", step.dep, httpWhy)
		}
		if commandWhy != "" {
			closeOpen()
			t.Fatalf("%s is stopped: not named by horus ready in 10 s: %s", step.dep, commandWhy)
		}
		if pageWhy != "" {
			closeOpen()
			t.Fatalf("%s is stopped: not named by /readyz in 10 s: %s", step.dep, pageWhy)
		}
		if open != nil {
			// The gate is in front of each tool, not only of generate_image.
			// The dependency is still stopped, and the row has no time limit
			// for these calls.
			for _, tool := range tools {
				e, err := open.call(tool, map[string]any{}, 15*time.Second)
				if err == nil {
					if why := wrongNotReady(e, step.down); why != "" {
						err = errors.New(why)
					}
				}
				if err != nil {
					closeOpen()
					t.Fatalf("%s is stopped: the tool %s in the open stdio session: %v", step.dep, tool, err)
				}
				e, err = hopen.call(tool, map[string]any{}, 15*time.Second)
				if err == nil {
					if why := wrongNotReady(e, step.down); why != "" {
						err = errors.New(why)
					}
				}
				if err != nil {
					closeOpen()
					t.Fatalf("%s is stopped: the tool %s in the open HTTP session: %v", step.dep, tool, err)
				}
			}
		}
		if n := fake.jobs.Load() - jobsBefore; n != 0 {
			t.Errorf("%s is stopped: %d requests reached ComfyUI while horus was not ready", step.dep, n)
		}
		named++
		namedHTTP++
		t.Logf("%s stopped: named over stdio in %s, over HTTP in %s, by horus ready in %s, by /readyz in %s",
			step.dep, stdioTook.Round(100*time.Millisecond), httpTook.Round(100*time.Millisecond),
			commandTook.Round(100*time.Millisecond), pageTook.Round(100*time.Millisecond))
		t.Logf("%s stopped: the lists: over stdio: %s; over HTTP: %s; horus ready: %s; /readyz: %s",
			step.dep, stdioLists, httpLists, commandLists, pageLists)
		if open != nil {
			t.Logf("%s stopped: NOT_READY with the same list from each of the %d tools, over stdio and over HTTP: %v", step.dep, len(tools), tools)
		}

		step.start(t)
		took, why := eventuallyBy(30*time.Second, func() string { return serves(open, hopen) })
		closeOpen()
		if why != "" {
			t.Fatalf("%s started again: horus does not serve again in 30 s: %s", step.dep, why)
		}
		t.Logf("%s started again: horus serves again over stdio and over HTTP in %s", step.dep, took.Round(100*time.Millisecond))
	}

	if code, out := compose(t, env, "stop"); code != 0 {
		t.Fatalf("docker compose -p %s stop: exit code %d\n%s", testStack, code, out)
	}
	took, why := eventuallyBy(10*time.Second, func() string {
		s, err := openSession(t, 15*time.Second)
		if err == nil {
			s.close()
			return "a new stdio session started"
		}
		var no *errNoSession
		if !errors.As(err, &no) || no.exit == 0 || !strings.Contains(no.stderr, "is not running") {
			return "a new stdio session: " + err.Error()
		}
		h, err := openHTTP(token, 15*time.Second)
		if err == nil {
			h.close()
			return "a new HTTP session started"
		}
		var noHTTP *errNoHTTP
		if !errors.As(err, &noHTTP) || noHTTP.status != 0 {
			return fmt.Sprintf("a new HTTP session: %v, want no HTTP answer", err)
		}
		return ""
	})
	if why != "" {
		t.Fatalf("the whole stack is stopped: %s", why)
	}
	t.Logf("the whole stack stopped: no session over stdio or over HTTP in %s", took.Round(100*time.Millisecond))
	start := time.Now()
	upWait(t, env)
	_, why = eventuallyBy(30*time.Second, func() string { return serves(nil, nil) })
	if back := time.Since(start); why != "" || back > 30*time.Second {
		t.Fatalf("the whole stack started again: horus does not serve again in 30 s (%s): %s", back.Round(time.Second), why)
	}
	t.Logf("the whole stack started again: horus serves again over stdio and over HTTP in %s", time.Since(start).Round(100*time.Millisecond))
	t.Logf("dependencies whose stop horus finds: %d of 4 over stdio, %d of 4 over HTTP", named, namedHTTP)
}

// TestOutPathMapping runs two calls on the test stack. A call with a host
// path in the shared folder gets its PNG at that host path, and the result
// gives the host path, not the container path. A call with a path outside
// the shared folders gets INVALID_ARGUMENT that lists them, and makes no job
// in ComfyUI.
func TestOutPathMapping(t *testing.T) {
	f, st := e2eStack(t, 2*time.Second)
	s, err := openSession(t, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	pass := 0
	host := filepath.Join(st.shared, "covers", "one.png")
	e, err := s.call("generate_image", map[string]any{"workflow": "it-illustration", "subject": goalSubject, "seed": 51, "out_path": host}, 120*time.Second)
	switch {
	case err != nil:
		t.Errorf("a host path in the shared folder: %v", err)
	case !e.OK || e.Result["path"] != host:
		t.Errorf("a host path in the shared folder: %+v, want ok with the path %s", e, host)
	case !isPNG(host):
		t.Errorf("a host path in the shared folder: no PNG at %s", host)
	default:
		pass++
		t.Logf("the PNG is at the host path %s, and the result gives it", host)
	}

	outside := filepath.Join(t.TempDir(), "outside.png")
	e, err = s.call("generate_image", map[string]any{"workflow": "it-illustration", "subject": goalSubject, "seed": 52, "out_path": outside}, 60*time.Second)
	var dirs []any
	if e.Error != nil {
		dirs, _ = e.Error.Details["shared_dirs"].([]any)
	}
	switch {
	case err != nil:
		t.Errorf("a path outside the shared folders: %v", err)
	case e.OK || e.Error.Code != "INVALID_ARGUMENT" || len(dirs) != 1 || dirs[0] != st.shared:
		t.Errorf("a path outside the shared folders: %+v, want INVALID_ARGUMENT with details.shared_dirs [%s]", e, st.shared)
	case f.submits(52) != 0:
		t.Errorf("a path outside the shared folders: %d jobs reached ComfyUI", f.submits(52))
	default:
		pass++
		t.Logf("a path outside the shared folders: INVALID_ARGUMENT, details.shared_dirs %v", dirs)
	}
	t.Logf("cases that pass: %d of 2", pass)
}

// TestConcurrencySetting runs the peak tool against the test stack with
// HORUS_WORKER_CONCURRENCY = 1, 2 and 3, and a fake ComfyUI that runs 3 jobs
// at the same time. The fake records the most jobs that run at the same
// time: it is N, for each N.
func TestConcurrencySetting(t *testing.T) {
	pass := 0
	for n := 1; n <= 3; n++ {
		res, log := peakRun(t, n, "--calls", "6", "--parallel", "3")
		if res.MaxRunning != n || res.PNG != 6 {
			t.Errorf("HORUS_WORKER_CONCURRENCY=%d: the most jobs at one time %d, png %d of 6; want %d and 6:\n%s", n, res.MaxRunning, res.PNG, n, log)
			continue
		}
		pass++
		t.Logf("HORUS_WORKER_CONCURRENCY=%d: the most jobs at one time %d, png 6 of 6", n, res.MaxRunning)
	}
	t.Logf("values of N that pass: %d of 3", pass)
}

// brokerChecks are the checks of the contract suite internal/brokertest:
// each one is a subtest of the test of each broker.
var brokerChecks = []string{"order", "at_most_prefetch", "ack_deletes", "again_after_consumer_stops",
	"two_consumers_never_share", "nack_requeue", "nack_drop", "queues_are_separate", "notify_each_listener",
	"listen_from_its_start", "long_delivery", "depth"}

func TestBrokerContract(t *testing.T) {
	if os.Getenv("HORUS_TEST_DATABASE_URL") == "" {
		t.Fatal("HORUS_TEST_DATABASE_URL is not set: the Postgres broker runs the contract suite only on a test Postgres. " +
			"Start one and set the variable to its URL (docs/mcp.md#the-postgres-tests)")
	}
	code, out := run(t, root, "go", "test", "-count=1", "-v", "./internal/brokertest/...", "./internal/postgres/...")
	if code != 0 {
		t.Fatalf("go test ./internal/brokertest/... ./internal/postgres/... exit code %d:\n%s", code, out)
	}
	for _, broker := range []string{"TestMemoryBroker", "TestPostgresBroker"} {
		if !passed(out, broker) {
			t.Fatalf("%s does not pass:\n%s", broker, out)
		}
		for _, c := range brokerChecks {
			if !passed(out, broker+"/"+c) {
				t.Errorf("%s: the check %s does not pass", broker, c)
			}
		}
	}
}

// duplicateLine is the line of the Go test TestDuplicateDelivery of
// internal/worker: its counts of the deliveries and of the submits that the
// fake ComfyUI received.
var duplicateLine = regexp.MustCompile(`duplicate-delivery: (\d+) messages delivered two times: (\d+) deliveries, (\d+) submits for (\d+) jobs, (\d+) jobs with exactly one submit`)

// TestDuplicateDelivery runs the Go test of internal/worker: the in-memory
// broker delivers each of 6 messages two times to the worker, and the fake
// ComfyUI counts the submits. The row reads the counts of the line of the
// test, so a test that passes without the line, or with other counts, fails
// the row.
func TestDuplicateDelivery(t *testing.T) {
	code, out := run(t, root, "go", "test", "-count=1", "-run", "TestDuplicateDelivery", "-v", "./internal/worker")
	if code != 0 || !passed(out, "TestDuplicateDelivery") {
		t.Fatalf("go test -run TestDuplicateDelivery ./internal/worker: exit code %d:\n%s", code, out)
	}
	m := duplicateLine.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("the test printed no line duplicate-delivery:\n%s", out)
	}
	want := []string{"6", "12", "6", "6", "6"}
	for i, name := range []string{"messages", "deliveries", "submits", "jobs", "jobs with exactly one submit"} {
		if m[i+1] != want[i] {
			t.Errorf("%s: %s, want %s", name, m[i+1], want[i])
		}
	}
	t.Logf("jobs with exactly one submit: %s of %s", m[5], m[4])
}

// TestHttpGuard sends 5 requests to /mcp of the test stack (POST initialize,
// as a new agent session does). Each of the first 4 has exactly one fault: no
// token, a wrong token, a wrong Host header, a foreign Origin header. They get
// 401, 401, 403 and 403. The 5th request has no fault: it gets 200 with the
// answer of horus to initialize. The tokens are random values of this run.
func TestHttpGuard(t *testing.T) {
	fake := startFake(t)
	env := newStack(t, fake)
	upWait(t, env)
	token := tokenOf(t, env)
	cases := []struct {
		fault string
		edit  func(r *http.Request)
		want  int
	}{
		{"no token", func(r *http.Request) { r.Header.Del("Authorization") }, http.StatusUnauthorized},
		{"a wrong token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+randomHex(t)) }, http.StatusUnauthorized},
		{"a wrong Host header", func(r *http.Request) { r.Host = "horus.example.com:" + testAPIPort }, http.StatusForbidden},
		{"a foreign Origin header", func(r *http.Request) { r.Header.Set("Origin", "http://horus.example.com") }, http.StatusForbidden},
		{"no fault", nil, http.StatusOK},
	}
	var got []string
	refused := 0
	for _, c := range cases {
		req, err := mcpRequest(token, "", initMessage(), c.edit)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			t.Fatalf("%s: no HTTP answer: %v", c.fault, err)
		}
		var server string
		if resp.StatusCode == http.StatusOK {
			raw, err := rpcAnswer(resp, 1)
			var init struct {
				ServerInfo struct {
					Name string `json:"name"`
				} `json:"serverInfo"`
			}
			if err == nil {
				_ = json.Unmarshal(raw, &init)
			}
			server = init.ServerInfo.Name
		}
		resp.Body.Close()
		got = append(got, strconv.Itoa(resp.StatusCode))
		switch {
		case resp.StatusCode != c.want:
			t.Errorf("%s: HTTP %d, want %d", c.fault, resp.StatusCode, c.want)
		case c.want == http.StatusUnauthorized && !strings.HasPrefix(resp.Header.Get("WWW-Authenticate"), "Bearer"):
			t.Errorf("%s: 401 without WWW-Authenticate: Bearer", c.fault)
		case c.want == http.StatusOK && server != "horus":
			t.Errorf("%s: 200, but the answer to initialize names the server %q, want horus", c.fault, server)
		case c.want != http.StatusOK:
			refused++
		}
		t.Logf("%s: HTTP %d", c.fault, resp.StatusCode)
	}
	t.Logf("the 5 requests: %s; bad requests refused: %d of 4", strings.Join(got, ", "), refused)
}

// queueText is the progress text of a queued job.
var queueText = regexp.MustCompile(`^waiting in the horus queue, position (\d+) of (\d+), start in (about \d+ min|less than 1 min)$`)

// progressRunEstimate is HORUS_RUN_ESTIMATE of the stack of TestQueueProgress,
// in seconds. Before a job succeeded, horus uses it as the run time of one job
// in the estimated start. With 250 s, the estimated start of call 2 stays
// "about 4 min" and that of call 3 stays "about 8 min" while the calls 2 and 3
// come from 0 to 40 s after the start of the run of call 1.
const progressRunEstimate = 250

// startWords is the estimated start of a queue text for a start that is in
// away from now: the whole minutes, rounded to the nearest one, and "less
// than 1 min" below 30 s.
func startWords(in time.Duration) string {
	if in >= 30*time.Second {
		return fmt.Sprintf("about %d min", int((in+30*time.Second)/time.Minute))
	}
	return "less than 1 min"
}

// startRange gives each estimated start of a queue text for a start that is
// from lo to hi away. The function knows the time of an event of horus only
// between two of its own clock reads, so it computes the text for both ends.
// The two ends give one text, but when a clock read is at a turn of a minute.
func startRange(lo, hi time.Duration) []string {
	var out []string
	for d := lo; ; d += time.Second {
		d = min(d, hi)
		if w := startWords(d); !slices.Contains(out, w) {
			out = append(out, w)
		}
		if d >= hi {
			return out
		}
	}
}

// queuePlace is the place, the total and the estimated start of a queue text.
type queuePlace struct {
	pos, total int
	start      string // "about 8 min" or "less than 1 min"
	minutes    int    // 0 for "less than 1 min"
}

// placeOf reads a queue text. ok is false for a text of another kind.
func placeOf(message string) (p queuePlace, ok bool) {
	m := queueText.FindStringSubmatch(message)
	if m == nil {
		return queuePlace{}, false
	}
	p.pos, _ = strconv.Atoi(m[1])
	p.total, _ = strconv.Atoi(m[2])
	p.start = m[3]
	if f := strings.Fields(m[3]); f[0] == "about" {
		p.minutes, _ = strconv.Atoi(f[1])
	}
	return p, true
}

// queueTexts gives the queue texts of notes, in their order.
func queueTexts(notes []progressNote) []string {
	texts := []string{}
	for _, n := range notes {
		if queueText.MatchString(n.message) {
			texts = append(texts, n.message)
		}
	}
	return texts
}

// TestQueueProgress makes 3 calls on the test stack, one after the other,
// while the first one runs. The stack has one slot, a job runs for 15 s, and
// HORUS_RUN_ESTIMATE is progressRunEstimate. Each call gets its place in the
// queue and its estimated start as progress:
//
//   - Call 1 comes to an empty queue and a free slot: "position 1 of 1, start
//     in less than 1 min".
//   - Call 2 comes while call 1 runs: its first message is "position 1 of 1",
//     and its start is one run estimate after the start of call 1. After
//     call 3 comes, its text is "position 1 of 2".
//   - Call 3 comes while call 1 runs and call 2 waits: its first message is
//     "position 2 of 2", and its start is two run estimates after the start
//     of call 1. After call 1 ends, call 2 runs and call 3 gets "position 1 of
//     1", with a start that is at most the run time of call 1 away.
//
// The third call gets a message at least every 10 s, and then its PNG.
func TestQueueProgress(t *testing.T) {
	const jobRun = 15 * time.Second
	estimate := progressRunEstimate * time.Second
	f := startJobFake(t, jobRun)
	st := newQueueStack(t, f.port, "HORUS_WORKER_CONCURRENCY=1", "HORUS_RUN_ESTIMATE="+strconv.Itoa(progressRunEstimate))
	upWait(t, st.env)
	var sessions []*session
	for i := 0; i < 3; i++ {
		s, err := openSession(t, 60*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer s.close()
		sessions = append(sessions, s)
	}
	args := func(n int) map[string]any {
		return map[string]any{"workflow": "it-illustration", "subject": goalSubject, "seed": 80 + n,
			"out_path": filepath.Join(st.shared, fmt.Sprintf("progress-%d.png", n))}
	}
	results := make([]chan envelope, 3)
	firsts := make([]progressNote, 2) // the first progress message of the calls 1 and 2
	sent := make([]time.Time, 2)      // the time before the calls 1 and 2 went out
	var runSeen time.Time             // a time when call 1 was in ComfyUI: its run started before
	for n := 1; n <= 2; n++ {
		results[n-1] = make(chan envelope, 1)
		sent[n-1] = time.Now()
		go func(n int) {
			e, err := sessions[n-1].callToken("generate_image", args(n), fmt.Sprintf("p%d", n), 180*time.Second)
			if err != nil {
				e = envelope{}
			}
			results[n-1] <- e
		}(n)
		// The next call starts when this one is in the queue of horus.
		select {
		case note := <-sessions[n-1].progress:
			firsts[n-1] = note
		case <-time.After(60 * time.Second):
			t.Fatalf("call %d: no progress in 60 s", n)
		}
		if n == 1 {
			if why := waitFor(30*time.Second, func() string {
				if f.submits(81) == 0 {
					return "call 1 did not reach ComfyUI"
				}
				return ""
			}); why != "" {
				t.Fatal(why)
			}
			runSeen = time.Now()
		}
	}
	start := time.Now()
	e3, err := sessions[2].callToken("generate_image", args(3), "p3", 180*time.Second)
	answered := time.Now()
	if err != nil || !e3.OK {
		t.Fatalf("call 3: %+v %v", e3, err)
	}
	var notes []progressNote
	for len(sessions[2].progress) > 0 {
		notes = append(notes, <-sessions[2].progress)
	}
	// The messages of the 3 calls. The run output shows their queue texts.
	all := [][]progressNote{{firsts[0]}, {firsts[1]}, notes}
	for n := 1; n <= 2; n++ {
		e := <-results[n-1]
		if !e.OK {
			t.Errorf("call %d: %+v", n, e)
		}
		for len(sessions[n-1].progress) > 0 {
			all[n-1] = append(all[n-1], <-sessions[n-1].progress)
		}
	}
	for n := 1; n <= 3; n++ {
		t.Logf("call %d: its queue texts, %d of its %d messages: %q", n, len(queueTexts(all[n-1])), len(all[n-1]), queueTexts(all[n-1]))
	}

	// Call 3: the place and the gaps.
	positions := 0
	lastPos := 0      // the position of the last queue message: it never rises
	lastMinutes := -1 // the minutes of the estimated start of the last queue message: they never rise
	last := start
	for _, n := range notes {
		if gap := n.at.Sub(last); gap > 10*time.Second {
			t.Errorf("call 3: %s without a progress message before %q, want at most 10 s", gap.Round(10*time.Millisecond), n.message)
		}
		last = n.at
		if m := queueText.FindStringSubmatch(n.message); m != nil {
			positions++
			pos, _ := strconv.Atoi(m[1])
			total, _ := strconv.Atoi(m[2])
			if pos < 1 || pos > total || total > 2 {
				t.Errorf("call 3: %q, want a position from 1 to its total, and a total of 2 or less", n.message)
			}
			if lastPos != 0 && pos > lastPos {
				t.Errorf("call 3: %q after position %d, want a position that never rises", n.message, lastPos)
			}
			lastPos = pos
			p, _ := placeOf(n.message)
			if lastMinutes >= 0 && p.minutes > lastMinutes {
				t.Errorf("call 3: %q after a start in %d min, want an estimated start that never moves away: no job came before call 3", n.message, lastMinutes)
			}
			lastMinutes = p.minutes
		}
	}
	if len(notes) > 0 {
		if gap := answered.Sub(last); gap > 10*time.Second {
			t.Errorf("call 3: %s from the last progress message to its answer, want at most 10 s", gap.Round(10*time.Millisecond))
		}
		// Call 1 runs and call 2 waits before call 3: its place is 2 of 2.
		if m := queueText.FindStringSubmatch(notes[0].message); m == nil || m[1] != "2" || m[2] != "2" {
			t.Fatalf("call 3: first message %q, want position 2 of 2: call 1 runs and call 2 waits before it", notes[0].message)
		}
	}
	if positions < 2 || len(notes) == 0 || !queueText.MatchString(notes[0].message) {
		var texts []string
		for _, n := range notes {
			texts = append(texts, n.message)
		}
		t.Fatalf("call 3: %d messages with its place in the queue, want 2 or more, the first one first: %q", positions, texts)
	}
	withPosition := 0
	for n := 1; n <= 3; n++ {
		if len(queueTexts(all[n-1])) > 0 {
			withPosition++
		}
	}
	if withPosition != 3 {
		t.Errorf("calls with a position message: %d of 3, want each call: the first message is the place at the insert", withPosition)
	}

	// The estimated start of a message of a call while call 1 runs: the start
	// of the run of call 1, plus one run estimate for each job before the
	// call, minus the time when horus made the message. The run of call 1
	// started from sent[0] to runSeen. horus made the message from the time
	// before the call went out to the time when the message came.
	startWhileRun := func(n, before int, out time.Time, note progressNote) {
		t.Helper()
		lo := time.Duration(before)*estimate - note.at.Sub(sent[0])
		hi := time.Duration(before)*estimate - out.Sub(runSeen)
		want := startRange(lo, hi)
		p, ok := placeOf(note.message)
		if !ok || !slices.Contains(want, p.start) {
			t.Errorf("call %d: message %q while call 1 runs, want the estimated start %q: %d jobs before it, HORUS_RUN_ESTIMATE %d s for each one, and the run of call 1 started from %s to %s before the message",
				n, note.message, want, before, progressRunEstimate,
				out.Sub(runSeen).Round(100*time.Millisecond), note.at.Sub(sent[0]).Round(100*time.Millisecond))
		}
	}
	// end1 is the end of the run of call 1 on the fake ComfyUI. A message that
	// came before it is a message that horus made while call 1 ran.
	end1 := f.endOf(81)
	if end1.IsZero() {
		t.Fatal("the fake ComfyUI did not end the run of call 1")
	}

	// Call 1: an empty queue and a free slot. Each of its queue texts is the
	// same one.
	const alone = "waiting in the horus queue, position 1 of 1, start in less than 1 min"
	if firsts[0].message != alone {
		t.Errorf("call 1: first message %q, want %q: no job runs and no job waits", firsts[0].message, alone)
	}
	for _, text := range queueTexts(all[0]) {
		if text != alone {
			t.Errorf("call 1: %q, want only %q: no job is before call 1", text, alone)
		}
	}

	// Call 2: call 1 runs, and no job waits before it.
	if p, ok := placeOf(firsts[1].message); !ok || p.pos != 1 || p.total != 1 {
		t.Errorf("call 2: first message %q, want position 1 of 1: call 1 runs, and no job waits before call 2", firsts[1].message)
	}
	startWhileRun(2, 1, sent[1], firsts[1])
	// Then call 3 comes behind it: position 1 of 2, while call 1 runs.
	behind := 0
	lastMinutes = -1
	for i, n := range all[1] {
		p, ok := placeOf(n.message)
		if !ok {
			continue
		}
		if p.pos != 1 || p.total < 1 || p.total > 2 {
			t.Errorf("call 2: %q, want position 1 of 1 or 1 of 2: only call 1 is before call 2", n.message)
		}
		if i > 0 && p.pos == 1 && p.total == 2 {
			behind++
		}
		if i > 0 && n.at.Before(end1) {
			startWhileRun(2, 1, sent[1], n)
		}
		if lastMinutes >= 0 && p.minutes > lastMinutes {
			t.Errorf("call 2: %q after a start in %d min, want an estimated start that never moves away", n.message, lastMinutes)
		}
		lastMinutes = p.minutes
	}
	if behind == 0 {
		t.Errorf("call 2: no message with position 1 of 2 after its first one, want one: call 3 waits behind call 2 while call 1 runs: %q", queueTexts(all[1]))
	}

	// Call 3: two jobs before it at its insert, and while call 1 runs.
	startWhileRun(3, 2, start, notes[0])
	for _, n := range notes[1:] {
		if p, ok := placeOf(n.message); ok && p.pos == 2 && n.at.Before(end1) {
			startWhileRun(3, 2, start, n)
		}
	}
	// Then call 1 ends and call 2 runs: call 3 is 1 of 1. That text can not
	// come before the end of the run of call 1 on the fake ComfyUI. Its
	// estimated start is at most one run away, and horus now uses the run time
	// of call 1 from the job store.
	rows := query(t, "select extract(epoch from finished_at - started_at) from horus.jobs where payload->>'seed' = '81' and state = 'succeeded'")
	if len(rows) != 1 {
		t.Fatalf("the job store has %d succeeded jobs of call 1, want 1", len(rows))
	}
	seconds, err := strconv.ParseFloat(rows[0][0], 64)
	if err != nil || seconds <= 0 {
		t.Fatalf("the job store gives the run time %q of call 1, want a time above 0 s", rows[0][0])
	}
	run1 := time.Duration(seconds * float64(time.Second))
	next := 0 // the messages of call 3 with position 1 of 1
	for _, n := range notes {
		p, ok := placeOf(n.message)
		if !ok || p.pos != 1 {
			continue
		}
		if p.total != 1 {
			t.Errorf("call 3: %q, want position 1 of 1: no job waits behind call 3", n.message)
			continue
		}
		next++
		if n.at.Before(end1) {
			t.Errorf("call 3: %q came %s before the end of the run of call 1, want it after: call 2 waits before call 3 until then",
				n.message, end1.Sub(n.at).Round(10*time.Millisecond))
		}
		if want := startRange(0, run1); !slices.Contains(want, p.start) {
			t.Errorf("call 3: %q, want the estimated start %q: call 2 runs, and the run of call 1 took %s", n.message, want, run1.Round(100*time.Millisecond))
		}
	}
	if next == 0 {
		t.Errorf("call 3: no message with position 1 of 1, want one after call 1 ends: its place falls from 2 to 1: %q", queueTexts(notes))
	}
	t.Logf("call 3: first message %q; %d messages with its place in the queue, %d of them with position 1 of 1, %d messages in all, each gap at most 10 s",
		notes[0].message, positions, next, len(notes))
	t.Logf("calls with a position message: %d of 3", withPosition)
}

// TestCancelAndDeadline makes calls on the test stack while one job runs on
// the fake ComfyUI. A cancel of a queued call moves its job to cancelled. A
// call whose estimated start is after its HORUS_QUEUE_TIMEOUT gets
// QUEUE_FULL in less than 2 s. A queued job past its deadline never starts,
// and its call gets GENERATION_TIMEOUT with details.phase queued. ComfyUI
// gets no submit for those 3 calls.
func TestCancelAndDeadline(t *testing.T) {
	f, st := e2eStack(t, 20*time.Second)
	args := func(seed int) map[string]any {
		return map[string]any{"workflow": "it-illustration", "subject": goalSubject, "seed": seed,
			"out_path": filepath.Join(st.shared, fmt.Sprintf("cancel-%d.png", seed))}
	}
	a, err := openSession(t, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	aDone := make(chan envelope, 1)
	go func() {
		e, _ := a.call("generate_image", args(71), 180*time.Second)
		aDone <- e
	}()
	if why := waitFor(30*time.Second, func() string {
		if f.submits(71) == 0 {
			return "the first call did not reach ComfyUI"
		}
		return ""
	}); why != "" {
		t.Fatal(why)
	}

	// A cancel removes a queued job.
	b, err := openSession(t, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	go func() { _, _ = b.callToken("generate_image", args(72), "b", 60*time.Second) }()
	select {
	case n := <-b.progress:
		if !queueText.MatchString(n.message) {
			t.Fatalf("the call to cancel is not queued: %q", n.message)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("the call to cancel: no progress in 60 s")
	}
	if err := b.cancel(b.lastID()); err != nil {
		t.Fatal(err)
	}
	if why := waitFor(5*time.Second, func() string {
		if s := stateOfSeed(t, 72); s != "cancelled" {
			return "the job of the cancelled call is " + s
		}
		return ""
	}); why != "" {
		t.Error(why)
	}

	// A late start gets QUEUE_FULL at once.
	c, err := openSession(t, 60*time.Second, "HORUS_QUEUE_TIMEOUT=5", "HORUS_RUN_ESTIMATE=60")
	if err != nil {
		t.Fatal(err)
	}
	defer c.close()
	began := time.Now()
	e, err := c.call("generate_image", args(73), 30*time.Second)
	took := time.Since(began)
	if err != nil || e.OK || e.Error.Code != "QUEUE_FULL" || took >= 2*time.Second {
		t.Errorf("a call whose start is after its deadline: %+v %v after %s, want QUEUE_FULL in less than 2 s", e, err, took)
	} else {
		t.Logf("QUEUE_FULL after %s: %s", took.Round(time.Millisecond), e.Error.Message)
	}

	// A job past its deadline never starts.
	d, err := openSession(t, 60*time.Second, "HORUS_QUEUE_TIMEOUT=3", "HORUS_RUN_ESTIMATE=1")
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	e, err = d.call("generate_image", args(74), 90*time.Second)
	if err != nil || e.OK || e.Error.Code != "GENERATION_TIMEOUT" || e.Error.Details["phase"] != "queued" {
		t.Errorf("a job past its deadline: %+v %v, want GENERATION_TIMEOUT with details.phase queued", e, err)
	}
	if s := stateOfSeed(t, 74); s != "expired" {
		t.Errorf("the job past its deadline is %s, want expired", s)
	}

	if e := <-aDone; !e.OK {
		t.Errorf("the first call: %+v", e)
	}
	if why := waitFor(60*time.Second, func() string {
		if !f.idle() {
			return "the fake ComfyUI still holds a job"
		}
		return ""
	}); why != "" {
		t.Error(why)
	}
	nobody := f.submits(72) + f.submits(73) + f.submits(74)
	if nobody != 0 || f.submits(71) != 1 {
		t.Errorf("submits: the first call %d, the cancelled, late and expired calls %d; want 1 and 0", f.submits(71), nobody)
	}
	t.Logf("submits of jobs that nobody wants: %d", nobody)
}

// TestIdempotencyRecorded runs the 12 fixed vectors of the recipe, then an
// end-to-end run on the test stack: two equal calls make 2 job rows, each
// with the request fingerprint that the oracle computes by recipe version 1
// and the key f1:<fingerprint> (the dedupe is off). A call with
// idempotency_key gets the key k1:<SHA-256 of the tool and the key>.
func TestIdempotencyRecorded(t *testing.T) {
	code, out := run(t, root, "go", "test", "-count=1", "-run", "^TestRecipeVectors$", "-v", "./internal/jobs")
	n := len(regexp.MustCompile(`(?m)^\s+--- PASS: TestRecipeVectors/`).FindAllString(out, -1))
	if code != 0 || n != 12 {
		t.Fatalf("the vectors of recipe version 1: %d of 12 pass, exit code %d:\n%s", n, code, out)
	}
	t.Logf("vectors that pass: %d of 12", n)

	_, st := e2eStack(t, time.Second)
	s, err := openSession(t, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	same := filepath.Join(st.shared, "same.png")
	equal := map[string]any{"workflow": "it-illustration", "subject": goalSubject, "seed": 61, "out_path": same, "overwrite": true}
	for i := 1; i <= 2; i++ {
		if e, err := s.call("generate_image", equal, 120*time.Second); err != nil || !e.OK {
			t.Fatalf("equal call %d: %+v %v", i, e, err)
		}
	}
	keyed := map[string]any{"workflow": "it-illustration", "subject": goalSubject, "out_path": filepath.Join(st.shared, "keyed.png"),
		"idempotency_key": "goal-key-1"}
	if e, err := s.call("generate_image", keyed, 120*time.Second); err != nil || !e.OK {
		t.Fatalf("the call with idempotency_key: %+v %v", e, err)
	}
	rows := query(t, "select coalesce(request_hash, ''), coalesce(idempotency_key, '') from horus.jobs order by created_at")
	if len(rows) != 3 {
		t.Fatalf("horus.jobs has %d rows, want 3: %v", len(rows), rows)
	}
	want := fingerprintOf(t, goalSubject, 61, same, true)
	for i := 0; i < 2; i++ {
		if rows[i][0] != want || rows[i][1] != "f1:"+want {
			t.Errorf("the row of equal call %d: request_hash %s, idempotency_key %s; want %s and f1:%s", i+1, rows[i][0], rows[i][1], want, want)
		}
	}
	sum := sha256.Sum256([]byte("generate_image\ngoal-key-1"))
	if k := "k1:" + hex.EncodeToString(sum[:]); rows[2][1] != k || len(rows[2][0]) != 64 {
		t.Errorf("the row of the call with idempotency_key: request_hash %q, idempotency_key %q; want a hash and %s", rows[2][0], rows[2][1], k)
	}
	if rows[0][1] == rows[1][1] {
		t.Logf("2 rows with the same key %s; the dedupe is off", rows[0][1])
	}
}

// uiLine is the line of the Go test TestHorusUI of cmd/horus for one
// fixture: the parts of the reference that the page has, and the counts of
// the page and of the JSON summary that equal the counts of SQL.
var uiLine = regexp.MustCompile(`ui-matches-mockup: fixture (\w+): (\d+) of (\d+) parts, (\d+) of (\d+) counts equal`)

// uiCounts is the least count of checks of one fixture: a test that checks
// fewer counts fails the row.
const uiCounts = 22

// TestUiMatchesMockup runs the Go test TestHorusUI of cmd/horus: horus api
// on a new test database with each of the two fixtures of
// internal/webui/uifixture (the example data of the reference
// docs/specs/2026-11-19-job-queue-ui.html, and other numbers). For both
// fixtures, the page has the 11 parts of the reference, and each count of the
// page and of the JSON summary equals the count that SQL reads. A page with
// the numbers of one fixture fails the other. As in the stack, the job rows
// hold container paths and the browser is in another time zone than horus
// api: the page shows the host path of the newest PNG, and with the time
// zone of the browser it shows the end time of that zone and counts "today"
// from its 00:00 (5 of the checks of each fixture). The screenshots at 1920 x 1080
// next to the reference are in docs/specs/2026-11-19-job-queue-ui/, and the
// judgment of each part is in the pull request of task #205.
func TestUiMatchesMockup(t *testing.T) {
	if os.Getenv("HORUS_TEST_DATABASE_URL") == "" {
		t.Fatal("HORUS_TEST_DATABASE_URL is not set: the Horus UI reads its fixtures from a test Postgres. " +
			"Start one and set the variable to its URL (docs/mcp.md#the-postgres-tests)")
	}
	code, out := run(t, root, "go", "test", "-count=1", "-run", "^TestHorusUI$", "-v", "./cmd/horus")
	if code != 0 || !passed(out, "TestHorusUI") {
		t.Fatalf("go test -run TestHorusUI ./cmd/horus: exit code %d:\n%s", code, out)
	}
	lines := map[string][]string{}
	for _, m := range uiLine.FindAllStringSubmatch(out, -1) {
		lines[m[1]] = m
	}
	matched := 0
	for _, name := range []string{"reference", "other"} {
		m, ok := lines[name]
		if !ok {
			t.Errorf("the test printed no line ui-matches-mockup for the fixture %s:\n%s", name, out)
			continue
		}
		parts, of, equal, counts := atoi(m[2]), atoi(m[3]), atoi(m[4]), atoi(m[5])
		if parts != 11 || of != 11 {
			t.Errorf("fixture %s: %d of %d parts, want 11 of 11", name, parts, of)
		}
		if equal != counts || counts < uiCounts {
			t.Errorf("fixture %s: %d of %d counts equal, want all of at least %d", name, equal, counts, uiCounts)
		}
		if name == "reference" {
			matched = parts
		}
	}
	t.Logf("parts that match: %d of 11", matched)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
