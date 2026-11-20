//go:build goal

// Package goal holds the goal rows of docs/specs/2026-11-19-job-queue.md:
// one test function for each row. Run it with
//
//	go test -tags goal -v ./goals/2026-11-19-job-queue
package goal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

func TestPeakTimeouts(t *testing.T) {
	t.Fatal("not built: tests/peak_load.py against the stack: 12 calls, 2 agents leave, 10 PNG, 0 GENERATION_TIMEOUT, 0 GPU seconds for nobody")
}

func TestBoundedBackendQueue(t *testing.T) {
	t.Fatal("not built: the fake ComfyUI of the peak tool records the most horus jobs that it held at one time")
}

// TestStackStartsTogether: with a fake ComfyUI on the host, one command,
// docker compose up -d --wait, starts the 3 containers of the test stack in
// their order (horus-db, horus-worker, horus-api), each one healthy, and
// horus ready in horus-api exits 0 with 4 ok lines, in 120 s or less from the
// start of the command. The build of the image comes before, and is not
// timed.
func TestStackStartsTogether(t *testing.T) {
	fake := startFake(t)
	env := newStack(t, fake)
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
}

// oneStop is one dependency of the row ready-when-four-up: how to stop it and
// start it again.
type oneStop struct {
	dep         string // the name in the readiness report
	stop, start func(t *testing.T)
}

// TestReadyWhenFourUp stops each of the 4 dependencies in turn, then the
// whole stack. Each time, over stdio (docker exec -i horus_test-api horus
// mcp):
//
//   - a call in a session that is open gets NOT_READY that names the
//     dependency, and a new session does not start and names it, in 10 s or
//     less. When horus-api stops, docker exec fails and names its container;
//   - no request reaches ComfyUI while horus is not ready;
//   - after the dependency starts again, a new session starts and a call gets
//     no NOT_READY, in 30 s or less, with no action on horus.
//
// The HTTP session of each step (/mcp) comes with task #204.
func TestReadyWhenFourUp(t *testing.T) {
	fake := startFake(t)
	env := newStack(t, fake)
	upWait(t, env)
	apiID := containerID(t, apiName)
	docker := func(args ...string) func(t *testing.T) {
		return func(t *testing.T) {
			t.Helper()
			if code, out := run(t, root, "docker", args...); code != 0 {
				t.Fatalf("docker %s: exit code %d\n%s", strings.Join(args, " "), code, out)
			}
		}
	}
	steps := []oneStop{
		{"horus-db", docker("stop", dbName), docker("start", dbName)},
		{"horus-worker", docker("stop", workerName), docker("start", workerName)},
		{"comfyui", func(*testing.T) { fake.stop() }, fake.start},
		{"horus-api", docker("stop", apiName), docker("start", apiName)},
	}
	call := map[string]any{"workflow": "it-illustration", "subject": "A falcon over a server rack. The picture contains no text."}
	named := 0
	for _, step := range steps {
		open, err := openSession(t, 60*time.Second)
		if err != nil {
			t.Fatalf("%s: before the stop, a session does not start: %v", step.dep, err)
		}
		if e, err := open.call("list_workflows", map[string]any{}, 30*time.Second); err != nil || !e.OK {
			t.Fatalf("%s: before the stop, list_workflows: %+v %v", step.dep, e, err)
		}
		jobsBefore := fake.jobs.Load()
		step.stop(t)
		took, why := eventually(10*time.Second, func() string {
			if step.dep != "horus-api" {
				e, err := open.call("generate_image", call, 15*time.Second)
				if err != nil {
					return "the call in the open session: " + err.Error()
				}
				if down := downNames(e); !hasAll(down, step.dep) {
					return fmt.Sprintf("the call in the open session: %+v, want NOT_READY with %s in details.down", e, step.dep)
				}
			}
			s, err := openSession(t, 15*time.Second)
			if err == nil {
				s.close()
				return "a new session started"
			}
			var no *errNoSession
			if !errors.As(err, &no) {
				return "a new session: " + err.Error()
			}
			want := "FAIL " + step.dep
			if step.dep == "horus-api" {
				want = "container " + apiID + " is not running"
			}
			if no.exit == 0 || !strings.Contains(no.stderr, want) {
				return fmt.Sprintf("a new session ended with exit code %d and does not say %q: %s", no.exit, want, no.stderr)
			}
			return ""
		})
		open.close()
		if why != "" {
			t.Fatalf("%s is stopped: not named in 10 s: %s", step.dep, why)
		}
		if n := fake.jobs.Load() - jobsBefore; n != 0 {
			t.Errorf("%s is stopped: %d requests reached ComfyUI while horus was not ready", step.dep, n)
		}
		named++
		t.Logf("%s stopped: named in %s", step.dep, took.Round(100*time.Millisecond))

		step.start(t)
		took, why = eventually(30*time.Second, func() string {
			s, err := openSession(t, 15*time.Second)
			if err != nil {
				return "a new session: " + err.Error()
			}
			defer s.close()
			e, err := s.call("list_workflows", map[string]any{}, 15*time.Second)
			if err != nil || !e.OK {
				return fmt.Sprintf("list_workflows: %+v %v", e, err)
			}
			return ""
		})
		if why != "" {
			t.Fatalf("%s started again: horus does not serve again in 30 s: %s", step.dep, why)
		}
		t.Logf("%s started again: horus serves again in %s", step.dep, took.Round(100*time.Millisecond))
	}

	if code, out := compose(t, env, "stop"); code != 0 {
		t.Fatalf("docker compose -p %s stop: exit code %d\n%s", testStack, code, out)
	}
	took, why := eventually(10*time.Second, func() string {
		s, err := openSession(t, 15*time.Second)
		if err == nil {
			s.close()
			return "a new session started"
		}
		var no *errNoSession
		if !errors.As(err, &no) || no.exit == 0 || !strings.Contains(no.stderr, "is not running") {
			return "a new session: " + err.Error()
		}
		return ""
	})
	if why != "" {
		t.Fatalf("the whole stack is stopped: %s", why)
	}
	t.Logf("the whole stack stopped: no session in %s", took.Round(100*time.Millisecond))
	start := time.Now()
	upWait(t, env)
	_, why = eventually(30*time.Second, func() string {
		s, err := openSession(t, 15*time.Second)
		if err != nil {
			return "a new session: " + err.Error()
		}
		defer s.close()
		if e, err := s.call("list_workflows", map[string]any{}, 15*time.Second); err != nil || !e.OK {
			return fmt.Sprintf("list_workflows: %+v %v", e, err)
		}
		return ""
	})
	if back := time.Since(start); why != "" || back > 30*time.Second {
		t.Fatalf("the whole stack started again: horus does not serve again in 30 s (%s): %s", back.Round(time.Second), why)
	}
	t.Logf("the whole stack started again: horus serves again in %s", time.Since(start).Round(100*time.Millisecond))
	t.Logf("dependencies whose stop horus finds over stdio: %d of 4", named)

	t.Fatal("not built: /mcp: the HTTP session of each step (task #204, MCP over HTTP)")
}

func TestOutPathMapping(t *testing.T) {
	t.Fatal("not built: HORUS_SHARED_DIRS, the host path of a PNG, and INVALID_ARGUMENT for a path outside the shared folders")
}

func TestConcurrencySetting(t *testing.T) {
	t.Fatal("not built: horus worker with HORUS_WORKER_CONCURRENCY = 1, 2 and 3 in the peak tool")
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

func TestSameContract(t *testing.T) {
	t.Fatal("not built: tests/mcp_contract.py on the host, over stdio into the stack, and over HTTP")
}

func TestHttpGuard(t *testing.T) {
	t.Fatal("not built: /mcp with the bearer token and the checks of the Host and Origin headers")
}

func TestQueueProgress(t *testing.T) {
	t.Fatal("not built: the progress message with the place in the queue and the estimated start")
}

func TestCancelAndDeadline(t *testing.T) {
	t.Fatal("not built: cancel of a queued job, QUEUE_FULL for a late start, and the deadline of a queued job")
}

func TestWorkerLost(t *testing.T) {
	t.Fatal("not built: WORKER_LOST after docker kill of horus-worker, and the next job after the restart")
}

func TestIdempotencyRecorded(t *testing.T) {
	code, out := run(t, root, "go", "test", "-count=1", "-run", "^TestRecipeVectors$", "-v", "./internal/jobs")
	n := len(regexp.MustCompile(`(?m)^\s+--- PASS: TestRecipeVectors/`).FindAllString(out, -1))
	if code != 0 || n != 12 {
		t.Fatalf("the vectors of recipe version 1: %d of 12 pass, exit code %d:\n%s", n, code, out)
	}
	t.Fatal("not built: request_hash and idempotency_key in each job row of Postgres, and the end-to-end run: two equal calls make 2 rows with the same key")
}

// uiLine is the line of the Go test TestHorusUI of cmd/horus for one
// fixture: the parts of the reference that the page has, and the counts of
// the page and of the JSON summary that equal the counts of SQL.
var uiLine = regexp.MustCompile(`ui-matches-mockup: fixture (\w+): (\d+) of (\d+) parts, (\d+) of (\d+) counts equal`)

// uiCounts is the least count of checks of one fixture: a test that checks
// fewer counts fails the row.
const uiCounts = 17

// TestUiMatchesMockup runs the Go test TestHorusUI of cmd/horus: horus api
// on a new test database with each of the two fixtures of
// internal/webui/uifixture (the example data of the reference
// docs/specs/2026-11-19-job-queue-ui.html, and other numbers). For both
// fixtures, the page has the 11 parts of the reference, and each count of the
// page and of the JSON summary equals the count that SQL reads. A page with
// the numbers of one fixture fails the other. The screenshots at 1920 x 1080
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
