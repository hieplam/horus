// Tests of a machine with only the program (the row binary-only of the spec
// docs/specs/2026-11-15-release-install.md): a temporary home folder, no
// HORUS_ROOT, and a current folder outside a clone. The program must offer
// its built-in workflows.
package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/cli"
	"github.com/hieplam/horus/internal/mcpserver"
	"github.com/hieplam/horus/internal/settings"
)

// onlyTheProgram makes this process a machine with only the program, and
// returns the temporary home folder. Every folder of the settings is in it.
func onlyTheProgram(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, k := range []string{"HORUS_HOME", "HORUS_ROOT", "HORUS_WORKFLOWS_DIR", "HORUS_OUTPUT_DIR", "HORUS_MODELS_DIR",
		"HORUS_TEXT_ENCODER_DIR", "HORUS_COMFYUI_DIR", "HORUS_COMFYUI_URL"} {
		t.Setenv(k, "")
	}
	t.Setenv("HORUS_AUTOSTART", "0")
	cwd := filepath.Join(home, "elsewhere")
	must(t, os.MkdirAll(cwd, 0o755))
	t.Chdir(cwd)
	settings.Reset()
	t.Cleanup(settings.Reset)
	if r := settings.Root(); r != "" {
		t.Fatalf("the temporary folder is inside a horus clone: %s", r)
	}
	return home
}

// TestWorkflowListWithoutAClonePrintsTheBuiltInWorkflows: horus workflow
// list names it-illustration and exits 0. Before the built-in workflows it
// printed "No workflows folder" and exited 1.
func TestWorkflowListWithoutAClonePrintsTheBuiltInWorkflows(t *testing.T) {
	home := onlyTheProgram(t)
	var out, errOut bytes.Buffer
	code := cli.Workflow(context.Background(), []string{"list"}, &out, &errOut, setupAt)
	text := out.String()
	if code != 0 || strings.Contains(text, "No workflows folder") {
		t.Fatalf("exit %d\n%s%s", code, text, errOut.String())
	}
	if !hasLine(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "it-illustration") {
		t.Fatalf("no line it-illustration:\n%s", text)
	}
	copyOf := filepath.Join(home, "horus", "builtin-workflows")
	if got := settings.WorkflowsDir(); got != copyOf {
		t.Errorf("the workflows folder is %s, want the copy in the user folder %s", got, copyOf)
	}
}

// TestListWorkflowsOfTheMCPServerWithoutAClonePrintsTheBuiltInWorkflows:
// list_workflows of horus mcp gives it-illustration and describe_workflow
// gives its subject rules. The address of ComfyUI comes from the .env file of
// the temporary user folder, so backend_running is true.
func TestListWorkflowsOfTheMCPServerWithoutAClonePrintsTheBuiltInWorkflows(t *testing.T) {
	home := onlyTheProgram(t)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "{}")
	}))
	t.Cleanup(up.Close)
	must(t, os.MkdirAll(filepath.Join(home, "horus"), 0o755))
	must(t, os.WriteFile(filepath.Join(home, "horus", ".env"), []byte("HORUS_COMFYUI_URL="+up.URL+"\n"), 0o600))
	settings.Reset()

	cs := connect(t, mcpserver.New(newImages(""), newSetup(cli.SetupPlace{}, seams{}), nil), nil)
	env := envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{}))
	r, _ := env["result"].(map[string]any)
	if env["ok"] != true || r == nil {
		t.Fatalf("list_workflows: %v", env)
	}
	if r["backend_running"] != true {
		t.Errorf("backend_running is %v: the .env file of the user folder was not read", r["backend_running"])
	}
	var names []string
	rows, _ := r["workflows"].([]any)
	for _, x := range rows {
		names = append(names, x.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "it-illustration,vi-speech" {
		t.Fatalf("list_workflows names %v, want it-illustration and vi-speech", names)
	}
	kinds := map[string]string{}
	for _, x := range rows {
		row := x.(map[string]any)
		kinds[row["name"].(string)], _ = row["kind"].(string)
	}
	if kinds["it-illustration"] != "image" || kinds["vi-speech"] != "speech" {
		t.Fatalf("list_workflows kinds %v, want image and speech", kinds)
	}

	env = envelopeOf(t, callTool(t, cs, "describe_workflow", map[string]any{"workflow": "vi-speech"}))
	r, _ = env["result"].(map[string]any)
	voices, _ := r["voices"].([]any)
	if env["ok"] != true || len(voices) != 25 || r["default_voice"] != "Hải Đăng" || r["graph"] != nil {
		t.Fatalf("describe_workflow vi-speech: %v", env)
	}

	env = envelopeOf(t, callTool(t, cs, "describe_workflow", map[string]any{"workflow": "it-illustration"}))
	r, _ = env["result"].(map[string]any)
	subject, _ := r["subject"].(map[string]any)
	if env["ok"] != true || subject == nil || subject["rules"] == nil {
		t.Fatalf("describe_workflow: %v", env)
	}
	// The example images are in the copy, so describe_workflow gives their paths.
	examples, _ := subject["examples"].([]any)
	if len(examples) == 0 {
		t.Fatal("describe_workflow gives no examples")
	}
	for _, x := range examples {
		p, _ := x.(map[string]any)["image"].(string)
		if !strings.HasPrefix(p, filepath.Join(home, "horus", "builtin-workflows")) {
			t.Errorf("example image %q is not in the copy of the built-in workflows", p)
		} else if _, err := os.Stat(p); err != nil {
			t.Errorf("example image: %v", err)
		}
	}
}
