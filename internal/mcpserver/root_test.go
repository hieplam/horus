package mcpserver

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/settings"
)

// Issue #40: a server outside the repo without HORUS_ROOT has no workflows
// folder. Every tool must name the setting to fix, not answer INTERNAL.
func TestNoWorkflowsFolderNamesHorusRoot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HORUS_ROOT", tmp)
	t.Setenv("HORUS_WORKFLOWS_DIR", filepath.Join(tmp, "workflows"))
	t.Setenv("HORUS_AUTOSTART", "0")
	settings.Reset()
	t.Cleanup(settings.Reset)
	cs := connect(t, New(&fakeImages{}, nil), nil)
	for name, args := range map[string]map[string]any{
		"list_workflows":     {},
		"describe_workflow":  {"workflow": "it-illustration"},
		"generate_image":     {"workflow": "it-illustration", "subject": testSubject},
		"install_workflow":   {"workflow": "it-illustration"},
		"uninstall_workflow": {"workflow": "it-illustration"},
	} {
		res := callTool(t, cs, name, args)
		e, _ := envelopeOf(t, res)["error"].(map[string]any)
		if !res.IsError || e["code"] != "BACKEND_NOT_INSTALLED" || e["action_by"] != "human" {
			t.Errorf("%s: %v", name, e)
			continue
		}
		if ns, _ := e["next_step"].(string); !strings.Contains(ns, "HORUS_ROOT") || !strings.Contains(ns, "horus register") {
			t.Errorf("%s: next_step %q does not name HORUS_ROOT and horus register", name, ns)
		}
	}
}
