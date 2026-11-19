//go:build goal

// Package goal holds the goal rows of docs/specs/2026-11-19-job-queue.md:
// one test function for each row. Run it with
//
//	go test -tags goal -v ./goals/2026-11-19-job-queue
package goal

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
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

func TestStackStartsTogether(t *testing.T) {
	t.Fatal("not built: compose.yaml with the profile test, 3 healthy containers and horus ready exit 0 in 120 s")
}

func TestReadyWhenFourUp(t *testing.T) {
	t.Fatal("not built: horus ready, NOT_READY, and the stop of each of the 4 dependencies over stdio and over HTTP")
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
	"listen_from_its_start", "long_delivery"}

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

func TestUiMatchesMockup(t *testing.T) {
	t.Fatal("not built: the Horus UI next to docs/specs/2026-11-19-job-queue-ui.html, and the JSON summary against SQL counts")
}
