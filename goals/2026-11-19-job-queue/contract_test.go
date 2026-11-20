//go:build goal

// The parts of the oracles of the rows same-contract and worker-lost (task
// #203): the contract test tests/mcp_contract.py on the host and on the test
// stack, and a worker that Docker kills while it runs a job.
package goal

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// contractLine is the last line of tests/mcp_contract.py.
var contractLine = regexp.MustCompile(`(?m)^contract: (\d+) of (\d+) passed`)

// contractCounts reads "contract: N of M passed" from the output of the
// contract test. It fails the row when the line is not there.
func contractCounts(t *testing.T, way, out string) (passedChecks, total int) {
	t.Helper()
	m := contractLine.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("the contract test %s printed no line \"contract: N of M passed\":\n%s", way, out)
	}
	passedChecks, _ = strconv.Atoi(m[1])
	total, _ = strconv.Atoi(m[2])
	return passedChecks, total
}

// hostContract runs tests/mcp_contract.py on the direct path of the host
// with a fresh program, and gives its exit code and output.
func hostContract(t *testing.T) (int, string) {
	t.Helper()
	return run(t, root, pythonOf(t), filepath.Join("tests", "mcp_contract.py"), "--server-cmd", buildHorus(t), "mcp")
}

// stackContract runs tests/mcp_contract.py over stdio into a new test stack
// (--stack horus_test) and gives its exit code and output. The test stack
// has HORUS_GENERATE_TIMEOUT 12, as the checks of the time limit need. The
// contract test starts its fake ComfyUI on a free port at once, and waits
// until horus ready exits 0, so it runs while the stack starts.
func stackContract(t *testing.T) (int, string) {
	t.Helper()
	py := pythonOf(t)
	port := freePort(t)
	st := newQueueStack(t, port, "HORUS_GENERATE_TIMEOUT=12")
	cmd := &background{cmd: exec.Command(py, filepath.Join("tests", "mcp_contract.py"), "--stack", testStack, "--port", strconv.Itoa(port),
		"--shared-dir", st.shared)}
	cmd.cmd.Dir = root
	done := cmd.start(t)
	upWait(t, st.env)
	select {
	case code := <-done:
		return code, cmd.output()
	case <-time.After(10 * time.Minute):
		cmd.kill()
		<-done
		t.Fatalf("tests/mcp_contract.py --stack did not end in 10 minutes:\n%s", cmd.output())
		return 0, ""
	}
}

// background is a program that runs while the test does something else.
type background struct {
	cmd *exec.Cmd
	out lockedBuffer
}

// start starts the program, and gives a channel that gets its exit code.
func (b *background) start(t *testing.T) <-chan int {
	t.Helper()
	b.cmd.Stdout, b.cmd.Stderr = &b.out, &b.out
	if err := b.cmd.Start(); err != nil {
		t.Fatalf("%s did not start: %v", b.cmd.Path, err)
	}
	done := make(chan int, 1)
	go func() {
		err := b.cmd.Wait()
		var exit *exec.ExitError
		switch {
		case errors.As(err, &exit):
			done <- exit.ExitCode()
		case err != nil:
			done <- -1
		default:
			done <- 0
		}
	}()
	return done
}

func (b *background) output() string { return b.out.String() }

func (b *background) kill() { _ = b.cmd.Process.Kill() }

// TestSameContract runs the contract test on the host and over stdio into the
// test stack. Each run passes all its checks. The stack run has the check
// that the stack ran the call: the PNG in the shared folder and the job row of
// the worker. The same first check runs in both: the server is horus and has
// the same 6 tools. The run over HTTP comes with task #204.
func TestSameContract(t *testing.T) {
	code, out := hostContract(t)
	hostPassed, hostTotal := contractCounts(t, "on the host", out)
	if code != 0 || hostPassed != hostTotal {
		t.Fatalf("the contract test on the host: exit code %d, %d of %d checks pass:\n%s", code, hostPassed, hostTotal, out)
	}
	t.Logf("the host run: %d of %d checks pass", hostPassed, hostTotal)

	code, out = stackContract(t)
	stackPassed, stackTotal := contractCounts(t, "over stdio into the stack", out)
	if code != 0 || stackPassed != stackTotal || stackTotal == 0 {
		t.Fatalf("the contract test over stdio into the stack: exit code %d, %d of %d checks pass:\n%s", code, stackPassed, stackTotal, out)
	}
	for _, line := range []string{"PASS  tools/list has exactly the 6 tools", "PASS  the stack ran the call: "} {
		if !strings.Contains(out, line) {
			t.Errorf("the contract test over stdio into the stack has no line %q:\n%s", line, out)
		}
	}
	t.Logf("the stdio run on the stack: %d of %d checks pass", stackPassed, stackTotal)

	t.Fatal("not built: /mcp: the HTTP run of tests/mcp_contract.py (task #204, MCP over HTTP)")
}

// TestWorkerLost kills horus-worker with docker kill while it runs a job. The
// waiting call gets WORKER_LOST in 90 s or less. Docker restarts the
// container (its restart count is 1 or more, and it is running), and a new
// call then ends with a PNG at its host path.
func TestWorkerLost(t *testing.T) {
	f, st := e2eStack(t, 20*time.Second)
	args := func(seed int, name string) map[string]any {
		return map[string]any{"workflow": "it-illustration", "subject": goalSubject, "seed": seed,
			"out_path": filepath.Join(st.shared, name)}
	}
	a, err := openSession(t, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	type answer struct {
		e   envelope
		err error
		at  time.Time
	}
	lost := make(chan answer, 1)
	go func() {
		e, err := a.call("generate_image", args(91, "lost.png"), 180*time.Second)
		lost <- answer{e, err, time.Now()}
	}()
	if why := waitFor(60*time.Second, func() string {
		if f.submits(91) == 0 {
			return "the job did not reach the fake ComfyUI"
		}
		return ""
	}); why != "" {
		t.Fatal(why)
	}
	if code, out := run(t, root, "docker", "kill", workerName); code != 0 {
		t.Fatalf("docker kill %s: exit code %d\n%s", workerName, code, out)
	}
	killed := time.Now()
	var got answer
	select {
	case got = <-lost:
	case <-time.After(150 * time.Second):
		t.Fatal("the waiting call got no answer in 150 s after docker kill of horus-worker")
	}
	took := got.at.Sub(killed)
	switch {
	case got.err != nil:
		t.Fatalf("the waiting call: %v", got.err)
	case got.e.OK || got.e.Error == nil || got.e.Error.Code != "WORKER_LOST" || got.e.Error.ActionBy != "retry":
		t.Fatalf("the waiting call after docker kill: %+v, want WORKER_LOST with action_by retry", got.e)
	case took > 90*time.Second:
		t.Fatalf("WORKER_LOST came %s after docker kill, want 90 s or less", took.Round(time.Second))
	}
	t.Logf("calls that end with WORKER_LOST: 1, %s after docker kill: %s", took.Round(100*time.Millisecond), got.e.Error.Message)

	code, out := run(t, root, "docker", "inspect", "-f", "{{.State.Running}} {{.RestartCount}}", workerName)
	var running bool
	var restarts int
	if code == 0 {
		_, _ = fmt.Sscanf(strings.TrimSpace(out), "%t %d", &running, &restarts)
	}
	if code != 0 || !running || restarts < 1 {
		t.Fatalf("docker inspect %s: %q (exit code %d), want running with a restart count of 1 or more: Docker did not restart horus-worker", workerName, strings.TrimSpace(out), code)
	}
	t.Logf("Docker restarted %s: running, restart count %d", workerName, restarts)

	next := filepath.Join(st.shared, "after.png")
	var e envelope
	_, why := eventually(60*time.Second, func() string {
		s, err := openSession(t, 15*time.Second)
		if err != nil {
			return "a new session: " + err.Error()
		}
		defer s.close()
		e, err = s.call("generate_image", args(92, "after.png"), 180*time.Second)
		if err != nil {
			return "the next call: " + err.Error()
		}
		if down := downNames(e); len(down) > 0 {
			return fmt.Sprintf("NOT_READY: %v", down)
		}
		return ""
	})
	if why != "" || !e.OK || e.Result["path"] != next || !isPNG(next) {
		t.Fatalf("the next job after the restart: %+v (%s), want ok with a PNG at %s", e, why, next)
	}
	t.Logf("the next job after the restart ended with a PNG at %s", next)
}
