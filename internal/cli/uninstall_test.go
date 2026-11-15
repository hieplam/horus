package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/provision"
)

// fakeUninstall gives a fixed plan and records the calls. It touches no
// file. Each test passes -workflows-dir, so no setting is read.
type fakeUninstall struct {
	u    *provision.Uninstall
	req  provision.UninstallRequest
	runs int
	fail bool
}

func (f *fakeUninstall) at(SetupPlace) Uninstaller { return f }
func (f *fakeUninstall) UninstallPlan(_ context.Context, r provision.UninstallRequest) (*provision.Uninstall, error) {
	f.req = r
	return f.u, nil
}
func (f *fakeUninstall) RunUninstall(_ context.Context, u *provision.Uninstall, _ io.Writer) error {
	f.runs++
	for i := range u.Items {
		if u.Items[i].Remove {
			u.Items[i].Done = !f.fail
		}
	}
	if f.fail {
		u.Items[0].Err = errors.New("access denied")
		return errors.New("1 of 2 REMOVE item(s) are still there. See the FAIL lines.")
	}
	return nil
}

func uninstallPlan() *provision.Uninstall {
	return &provision.Uninstall{Items: []provision.UninstallItem{
		{Part: provision.PartModels, Remove: true, What: "model file v.safetensors of workflow w", Path: "/m/vae/v.safetensors"},
		{Part: provision.PartComfyUI, What: "the ComfyUI install folder", Path: "/c", Why: "it has no install record"},
		{Part: provision.PartMCP, Remove: true, What: "the MCP server horus of Codex", Path: "/bin/codex", Command: []string{"/bin/codex", "mcp", "remove", "horus"}},
		{Part: provision.PartUser, What: "the .env file", Path: "/h/horus/.env", Why: "data of the user"},
	}}
}

func runUninstall(f *fakeUninstall, args ...string) (int, string) {
	var out bytes.Buffer
	code := RunUninstall(context.Background(), append(args, "-workflows-dir", "/w"), &out, &out, f.at)
	return code, out.String()
}

func TestUninstallPrintsThePlanAndRunsNothingWithoutYes(t *testing.T) {
	f := &fakeUninstall{u: uninstallPlan()}
	code, out := runUninstall(f)
	want := "Uninstall plan:\n" +
		"REMOVE  /m/vae/v.safetensors  (model file v.safetensors of workflow w)\n" +
		"KEEP    /c  (the ComfyUI install folder: it has no install record)\n" +
		"REMOVE  /bin/codex  (the MCP server horus of Codex; runs /bin/codex mcp remove horus)\n" +
		"KEEP    /h/horus/.env  (the .env file: data of the user)\n" +
		"Nothing changed. Run horus uninstall -yes to remove the 2 REMOVE item(s).\n"
	if code != 0 || out != want || f.runs != 0 {
		t.Fatalf("exit %d, runs %d, output:\n%s\nwant:\n%s", code, f.runs, out, want)
	}
}

func TestUninstallYesRunsThePlanAndPrintsEachItem(t *testing.T) {
	f := &fakeUninstall{u: uninstallPlan()}
	code, out := runUninstall(f, "-yes")
	for _, w := range []string{"Running the plan.\n", "OK: removed model file v.safetensors of workflow w: /m/vae/v.safetensors\n",
		"OK: removed the MCP server horus of Codex: /bin/codex\n", "OK: 2 REMOVE item(s) are gone.\n"} {
		if !strings.Contains(out, w) {
			t.Errorf("the output lacks %q:\n%s", w, out)
		}
	}
	if code != 0 || f.runs != 1 {
		t.Errorf("exit %d, runs %d", code, f.runs)
	}

	f = &fakeUninstall{u: uninstallPlan(), fail: true}
	code, out = runUninstall(f, "-yes")
	if code != 1 || !strings.Contains(out, "FAIL: model file v.safetensors of workflow w: /m/vae/v.safetensors: access denied\n") {
		t.Errorf("a failed item: exit %d:\n%s", code, out)
	}
}

func TestUninstallFlagsLimitTheParts(t *testing.T) {
	f := &fakeUninstall{u: uninstallPlan()}
	if code, out := runUninstall(f, "-models", "-mcp"); code != 0 || !strings.Contains(out, "Run horus uninstall -models -mcp -yes") {
		t.Errorf("exit %d:\n%s", code, out)
	}
	if !f.req.Models || !f.req.MCP || f.req.WorkflowsDir != "/w" {
		t.Errorf("request = %+v", f.req)
	}
	if code, _ := runUninstall(f, "extra"); code != 2 {
		t.Errorf("an argument: exit %d, want 2", code)
	}
	if code, _ := runUninstall(f, "-no-such-flag"); code != 2 {
		t.Errorf("a wrong flag: exit %d, want 2", code)
	}
	f = &fakeUninstall{u: &provision.Uninstall{}}
	if code, out := runUninstall(f, "-yes"); code != 0 || !strings.HasSuffix(out, "Nothing to remove.\n") || f.runs != 0 {
		t.Errorf("an empty plan: exit %d, runs %d:\n%s", code, f.runs, out)
	}
}
