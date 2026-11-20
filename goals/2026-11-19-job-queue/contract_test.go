//go:build goal

// The parts of the oracles of the rows same-contract and worker-lost (tasks
// #203 and #204): the contract test tests/mcp_contract.py on the host, over
// stdio into the test stack and over HTTP to /mcp of the test stack, and a
// worker that Docker kills while it runs a job.
package goal

import (
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

// contractRun is the exit code and the output of one run of the contract
// test.
type contractRun struct {
	code int
	out  string
}

// stackContracts runs tests/mcp_contract.py two times on one new test stack:
// over stdio (--stack horus_test), then over HTTP (--http, with the token of
// the stack in the environment of the test, never on its command line). The
// test stack has HORUS_GENERATE_TIMEOUT 12, as the checks of the time limit
// need. The contract test starts its fake ComfyUI on a free port at once, and
// waits until horus ready exits 0, so the first run starts while the stack
// starts. Each run writes its own PNG file in the shared folder.
func stackContracts(t *testing.T) (stdio, overHTTP contractRun) {
	t.Helper()
	py := pythonOf(t)
	port := freePort(t)
	st := newQueueStack(t, port, "HORUS_GENERATE_TIMEOUT=12")
	args := []string{filepath.Join("tests", "mcp_contract.py"), "--stack", testStack, "--port", strconv.Itoa(port), "--shared-dir", st.shared}
	one := func(env []string, args []string, meanwhile func()) contractRun {
		cmd := &background{cmd: exec.Command(py, args...)}
		cmd.cmd.Dir, cmd.cmd.Env = root, env
		done := cmd.start(t)
		if meanwhile != nil {
			meanwhile()
		}
		select {
		case code := <-done:
			return contractRun{code, cmd.output()}
		case <-time.After(10 * time.Minute):
			cmd.kill()
			<-done
			t.Fatalf("tests/mcp_contract.py %v did not end in 10 minutes:\n%s", args[1:], cmd.output())
			return contractRun{}
		}
	}
	stdio = one(nil, args, func() { upWait(t, st.env) })
	overHTTP = one(append(os.Environ(), "HORUS_API_TOKEN="+tokenOf(t, st.env)), append(args, "--http", httpURL), nil)
	return stdio, overHTTP
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

// TestSameContract runs the contract test on the host, over stdio into the
// test stack, and over HTTP to /mcp of the test stack. Each run passes all its
// checks. The two runs on the stack have the same checks, and the check that
// the stack ran the call: the PNG in the shared folder and the job row of the
// worker. The same first check runs in all three: the server is horus and has
// the same 6 tools.
func TestSameContract(t *testing.T) {
	code, out := hostContract(t)
	hostPassed, hostTotal := contractCounts(t, "on the host", out)
	if code != 0 || hostPassed != hostTotal {
		t.Fatalf("the contract test on the host: exit code %d, %d of %d checks pass:\n%s", code, hostPassed, hostTotal, out)
	}
	t.Logf("the host run: %d of %d checks pass", hostPassed, hostTotal)

	stdio, overHTTP := stackContracts(t)
	totals := map[string]int{}
	for _, r := range []struct {
		way string
		run contractRun
	}{{"over stdio into the stack", stdio}, {"over HTTP to /mcp of the stack", overHTTP}} {
		passedChecks, total := contractCounts(t, r.way, r.run.out)
		if r.run.code != 0 || passedChecks != total || total == 0 {
			t.Fatalf("the contract test %s: exit code %d, %d of %d checks pass:\n%s", r.way, r.run.code, passedChecks, total, r.run.out)
		}
		for _, line := range []string{"PASS  tools/list has exactly the 6 tools", "PASS  the stack ran the call: "} {
			if !strings.Contains(r.run.out, line) {
				t.Errorf("the contract test %s has no line %q:\n%s", r.way, line, r.run.out)
			}
		}
		totals[r.way] = total
		t.Logf("the run %s: %d of %d checks pass", r.way, passedChecks, total)
	}
	if a, b := totals["over stdio into the stack"], totals["over HTTP to /mcp of the stack"]; a != b {
		t.Errorf("the two runs on the stack have %d and %d checks, want the same checks", a, b)
	}
}

// TestWorkerLost kills the process of horus-worker with SIGKILL while it runs
// a job, as a crash or the out-of-memory killer does: a container with
// --pid=host sends kill -9 to the host process id of the worker. Docker sees
// an exit with code 137 that no person asked for, so its restart policy
// starts the container again. (The command docker kill is a stop that a
// person asks for: Docker 29.8 never restarts the container after it, also
// with restart: always. A kill -9 from inside the container is ignored,
// because the worker is process 1 there.) The waiting
// call gets WORKER_LOST in 90 s or less, the container is running again with
// a new start time, and a new call then ends with a PNG at its host path.
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
	_, firstStart := run(t, root, "docker", "inspect", "-f", "{{.State.StartedAt}}", workerName)
	code, pid := run(t, root, "docker", "inspect", "-f", "{{.State.Pid}}", workerName)
	pid = strings.TrimSpace(pid)
	if code != 0 || pid == "" || pid == "0" {
		t.Fatalf("docker inspect %s: no process id (exit code %d): %q", workerName, code, pid)
	}
	if code, out := run(t, root, "docker", "run", "--rm", "--privileged", "--pid=host", "alpine:3.22", "kill", "-9", pid); code != 0 {
		t.Fatalf("kill -9 %s of %s from a container with --pid=host: exit code %d\n%s", pid, workerName, code, out)
	}
	killed := time.Now()
	var got answer
	select {
	case got = <-lost:
	case <-time.After(150 * time.Second):
		t.Fatal("the waiting call got no answer in 150 s after SIGKILL of horus-worker")
	}
	took := got.at.Sub(killed)
	switch {
	case got.err != nil:
		t.Fatalf("the waiting call: %v", got.err)
	case got.e.OK || got.e.Error == nil || got.e.Error.Code != "WORKER_LOST" || got.e.Error.ActionBy != "retry":
		t.Fatalf("the waiting call after SIGKILL of the worker: %+v, want WORKER_LOST with action_by retry", got.e)
	case took > 90*time.Second:
		t.Fatalf("WORKER_LOST came %s after SIGKILL of the worker, want 90 s or less", took.Round(time.Second))
	}
	t.Logf("calls that end with WORKER_LOST: 1, %s after SIGKILL of the worker: %s", took.Round(100*time.Millisecond), got.e.Error.Message)

	_, why := eventually(60*time.Second, func() string {
		code, out := run(t, root, "docker", "inspect", "-f", "{{.State.Running}} {{.State.StartedAt}}", workerName)
		state, start, _ := strings.Cut(strings.TrimSpace(out), " ")
		if code != 0 || state != "true" || start == strings.TrimSpace(firstStart) {
			return fmt.Sprintf("docker inspect %s: %q (exit code %d), want running with a new start time (first start %s)", workerName, strings.TrimSpace(out), code, strings.TrimSpace(firstStart))
		}
		return ""
	})
	if why != "" {
		t.Fatalf("Docker did not restart horus-worker: %s", why)
	}
	t.Logf("Docker restarted %s: running, with a new start time", workerName)

	next := filepath.Join(st.shared, "after.png")
	var e envelope
	_, why = eventually(60*time.Second, func() string {
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
