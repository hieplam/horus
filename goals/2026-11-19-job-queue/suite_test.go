//go:build goal

// Package goal holds the goal rows of docs/specs/2026-11-19-job-queue.md:
// one test function for each row. Run it with
//
//	go test -tags goal -v ./goals/2026-11-19-job-queue
package goal

import "testing"

// The task that builds the outcome of a row gives the row its oracle.

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

func TestBrokerContract(t *testing.T) {
	t.Fatal("not built: internal/brokertest on the in-memory broker and on the Postgres broker")
}

func TestDuplicateDelivery(t *testing.T) {
	t.Fatal("not built: the Go test of internal/worker with each of 6 messages delivered two times")
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
	t.Fatal("not built: the recipe of version 1 with 12 fixed vectors, and the two columns of each job row")
}

func TestUiMatchesMockup(t *testing.T) {
	t.Fatal("not built: the Horus UI next to docs/specs/2026-11-19-job-queue-ui.html, and the JSON summary against SQL counts")
}
