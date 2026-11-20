package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/jobdata"
)

// readiness is a fake of the 4 checks: the dependencies in down fail. It
// records the only of each call.
func readiness(only *[][]string, down ...string) Readiness {
	return func(_ context.Context, o ...string) jobdata.Readiness {
		*only = append(*only, o)
		var rs []jobdata.CheckResult
		for _, d := range jobdata.Dependencies {
			if len(o) > 0 && !slices.Contains(o, d) {
				continue
			}
			rs = append(rs, jobdata.CheckResult{Dependency: d, Up: !slices.Contains(down, d), Detail: "detail of " + d})
		}
		return jobdata.ReadinessOf(rs)
	}
}

// TestReady: horus ready prints the 4 lines and exits 0 only when all pass.
// A check that fails names its dependency and exits 1.
func TestReady(t *testing.T) {
	var only [][]string
	var out, errOut bytes.Buffer
	if code := Ready(context.Background(), nil, &out, &errOut, readiness(&only)); code != 0 {
		t.Fatalf("all up: exit %d, %s", code, errOut.String())
	}
	if n := strings.Count(out.String(), "\nok   "); n != 4 || !strings.HasPrefix(out.String(), "horus ready: 4 of 4\n") {
		t.Errorf("all up: the output\n%s", out.String())
	}
	out.Reset()
	if code := Ready(context.Background(), nil, &out, &errOut, readiness(&only, jobdata.DependencyComfyUI)); code != 1 {
		t.Fatalf("ComfyUI down: exit %d", code)
	}
	if !strings.Contains(out.String(), "FAIL comfyui: detail of comfyui") || !strings.HasPrefix(out.String(), "horus ready: 3 of 4\n") {
		t.Errorf("ComfyUI down: the output\n%s", out.String())
	}
}

// TestReadyOnly: -only runs and prints the check of one dependency, and its
// result is the exit code. An unknown name exits 2 and runs no check.
func TestReadyOnly(t *testing.T) {
	var only [][]string
	var out, errOut bytes.Buffer
	code := Ready(context.Background(), []string{"-only", "horus-worker"}, &out, &errOut, readiness(&only, jobdata.DependencyComfyUI))
	if code != 0 || out.String() != "ok   horus-worker: detail of horus-worker\n" || !slices.Equal(only[0], []string{"horus-worker"}) {
		t.Errorf("-only horus-worker: exit %d, output %q, only %v", code, out.String(), only)
	}
	out.Reset()
	if code := Ready(context.Background(), []string{"-only", "horus-worker"}, &out, &errOut, readiness(&only, "horus-worker")); code != 1 ||
		!strings.HasPrefix(out.String(), "FAIL horus-worker") {
		t.Errorf("-only horus-worker, worker down: exit %d, output %q", code, out.String())
	}
	only = nil
	if code := Ready(context.Background(), []string{"-only", "nope"}, &out, &errOut, readiness(&only)); code != 2 || len(only) != 0 ||
		!strings.Contains(errOut.String(), "horus-db, comfyui, horus-worker, horus-api") {
		t.Errorf("-only nope: exit %d, %d runs, %s", code, len(only), errOut.String())
	}
}

// TestMCPStartsOnlyWhenReady: in the stack, horus mcp starts no session when
// a dependency is down: it writes the report and exits 1. On the direct path
// (no readiness) it serves.
func TestMCPStartsOnlyWhenReady(t *testing.T) {
	var only [][]string
	served := 0
	serve := func(context.Context) error { served++; return nil }
	var errOut bytes.Buffer
	if code := MCP(context.Background(), nil, &errOut, readiness(&only, jobdata.DependencyDB), serve); code != 1 || served != 0 {
		t.Fatalf("horus-db down: exit %d, %d sessions", code, served)
	}
	if !strings.Contains(errOut.String(), "horus is not ready") || !strings.Contains(errOut.String(), "FAIL horus-db") {
		t.Errorf("horus-db down: the error output\n%s", errOut.String())
	}
	if code := MCP(context.Background(), nil, &errOut, readiness(&only), serve); code != 0 || served != 1 {
		t.Errorf("all up: exit %d, %d sessions", code, served)
	}
	if code := MCP(context.Background(), nil, &errOut, nil, serve); code != 0 || served != 2 {
		t.Errorf("the direct path: exit %d, %d sessions", code, served)
	}
}

// TestAPIListen: horus api listens on 127.0.0.1:<HORUS_API_PORT> by default,
// and on -listen when it is given. A failed server exits 1.
func TestAPIListen(t *testing.T) {
	t.Setenv("HORUS_API_PORT", "18199")
	var got string
	serve := func(_ context.Context, listen string, _ io.Writer) error { got = listen; return nil }
	var out, errOut bytes.Buffer
	if code := API(context.Background(), nil, &out, &errOut, serve); code != 0 || got != "127.0.0.1:18199" {
		t.Errorf("default: exit %d, listen %q", code, got)
	}
	if code := API(context.Background(), []string{"-listen", "0.0.0.0:8190"}, &out, &errOut, serve); code != 0 || got != "0.0.0.0:8190" {
		t.Errorf("-listen: exit %d, listen %q", code, got)
	}
	fail := func(context.Context, string, io.Writer) error { return errors.New("address in use") }
	if code := API(context.Background(), nil, &out, &errOut, fail); code != 1 || !strings.Contains(errOut.String(), "address in use") {
		t.Errorf("a failed server: exit %d, %s", code, errOut.String())
	}
	t.Setenv("HORUS_API_PORT", "99999")
	if code := API(context.Background(), nil, &out, &errOut, serve); code != 1 {
		t.Errorf("HORUS_API_PORT 99999: exit %d", code)
	}
}

// TestRegisterDocker: -docker registers docker exec -i <stack>-api horus mcp,
// with no HORUS_ROOT, for Claude Code and for Codex.
func TestRegisterDocker(t *testing.T) {
	root := repoDir(t)
	useRoot(t, root)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-docker"}, "/bin/claude mcp get horus\n/bin/claude mcp add --scope user horus -- /bin/docker exec -i horus-api horus mcp"},
		{[]string{"-docker", "-stack", "horus_test"}, "/bin/claude mcp get horus\n/bin/claude mcp add --scope user horus -- /bin/docker exec -i horus_test-api horus mcp"},
		{[]string{"-docker", "-client", "codex"}, "/bin/codex mcp get horus\n/bin/codex mcp add horus -- /bin/docker exec -i horus-api horus mcp"},
	} {
		f := &fakePrograms{claude: "/bin/claude", codex: "/bin/codex", docker: "/bin/docker"}
		var out, errOut bytes.Buffer
		if code := Register(context.Background(), c.args, &out, &errOut, f); code != 0 {
			t.Fatalf("%v: exit %d: %s", c.args, code, errOut.String())
		}
		if f.commands() != c.want {
			t.Errorf("%v: commands:\n%s\nwant:\n%s", c.args, f.commands(), c.want)
		}
		if !strings.Contains(out.String(), "docker exec -i ") {
			t.Errorf("%v: the output does not name docker exec: %q", c.args, out.String())
		}
	}
	f := &fakePrograms{claude: "/bin/claude"}
	var out, errOut bytes.Buffer
	if code := Register(context.Background(), []string{"-docker"}, &out, &errOut, f); code != 1 ||
		!strings.Contains(errOut.String(), "The docker command was not found") || len(f.runs) != 0 {
		t.Errorf("no docker: exit %d, %s, %d runs", code, errOut.String(), len(f.runs))
	}
}
