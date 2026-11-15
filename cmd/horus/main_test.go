// Tests of the command table of horus.
package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// portMap is one command for each row of the command table of Phase 1,
// plus errors. install-workflow-models is dropped (spec, design decisions).
var portMap = []string{"mcp", "workflow", "download-model", "check-setup", "install-comfyui", "start-comfyui",
	"install-comfy-mcp", "sync-workflows", "new-workflow", "register", "run-illustration", "errors",
	"stop-comfyui",    // stop-comfyui: new in Phase 2 (task T7)
	"doctor", "setup", // new in Phase 2 (task T8)
	"comfy-mcp", // new in Phase 3 (task T5)
	"version"}   // new in Phase 4 (task T1)

func TestEveryPortMapRowHasACommand(t *testing.T) {
	for _, name := range portMap {
		if _, ok := commands[name]; !ok {
			t.Errorf("no command %s", name)
		}
	}
	if len(commands) != len(portMap) {
		t.Errorf("%d commands, want %d", len(commands), len(portMap))
	}
}

func TestExitCodes(t *testing.T) {
	if code := run(nil); code != 2 {
		t.Errorf("no command: exit %d, want 2", code)
	}
	if code := run([]string{"nope"}); code != 2 {
		t.Errorf("unknown command: exit %d, want 2", code)
	}
	if code := run([]string{"-h"}); code != 0 {
		t.Errorf("-h: exit %d, want 0", code)
	}
	if code := run([]string{"errors", "-json"}); code != 0 {
		t.Errorf("errors -json: exit %d, want 0", code)
	}
}

// Issue #41: -h prints the flags and exits 0 for every command. A wrong flag
// still exits 2.
func TestHelpExitsZero(t *testing.T) {
	for name := range commands {
		if got := run([]string{name, "-h"}); got != 0 {
			t.Errorf("horus %s -h exited %d, want 0", name, got)
		}
	}
	if got := run([]string{"errors", "-no-such-flag"}); got != 2 {
		t.Errorf("a wrong flag exited %d, want 2", got)
	}
}

// TestVersionPrintsTheVersionOfTheLinkerFlag builds the program with the
// version flag of the release build, and runs it. A build without the flag
// has the version dev.
func TestVersionPrintsTheVersionOfTheLinkerFlag(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "horus-test"+exeSuffix())
	for _, c := range []struct{ flag, want string }{
		{"", "horus dev\n"},
		{"-X github.com/hieplam/horus/internal/version.Version=1.2.3", "horus 1.2.3\n"},
	} {
		build := exec.Command("go", "build", "-ldflags", c.flag, "-o", exe, ".")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build with flag %q: %v\n%s", c.flag, err, out)
		}
		got, err := exec.Command(exe, "version").Output()
		if err != nil {
			t.Fatalf("horus version: %v", err)
		}
		if string(got) != c.want {
			t.Errorf("flag %q: horus version printed %q, want %q", c.flag, got, c.want)
		}
	}
}

// exeSuffix is the file name suffix of a program on this system (go env GOEXE),
// so that the test has no code for one operating system.
func exeSuffix() string {
	out, err := exec.Command("go", "env", "GOEXE").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
