// Tests of the command table of horus.
package main

import "testing"

// portMap is one command for each row of the port map in issue #4 (Phase 1),
// plus errors. install-workflow-models is dropped (spec, design decisions).
var portMap = []string{"mcp", "workflow", "download-model", "check-setup", "install-comfyui", "start-comfyui",
	"install-comfy-mcp", "sync-workflows", "new-workflow", "register", "run-illustration", "errors",
	"stop-comfyui"} // stop-comfyui: new in Phase 2 (task T7)

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
