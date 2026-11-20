// Tests of the setup service of the stack (ADR 0041): in a container
// without ComfyUI, the plan of a workflow holds only its model files, in
// HORUS_MODELS_DIR, and the install writes only there.
package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// TestStackSetupPlansOnlyModelFiles: the sandbox has no ComfyUI folder, as a
// container of the stack. The plan has one file step for each model file,
// each in HORUS_MODELS_DIR, and no ComfyUI step and no package step. The
// install downloads the files into HORUS_MODELS_DIR and writes no
// extra_model_paths.yaml. Then the workflow is installed, and the plan is
// empty. With one file removed, the plan has exactly that one file step.
func TestStackSetupPlansOnlyModelFiles(t *testing.T) {
	s := newSandbox(t)
	must(t, os.RemoveAll(s.comfy))
	t.Setenv("HORUS_WORKFLOWS_DIR", s.workflows)
	t.Setenv("HORUS_MODELS_DIR", s.models)
	t.Setenv("HORUS_COMFYUI_DIR", s.comfy)
	t.Setenv("HORUS_COMFYUI_URL", s.comfyUI.URL)
	settings.Reset()
	svc := stackSetupFor(s.seams(1 << 50))
	req := provision.Request{WorkflowsDir: s.workflows, Workflow: "demo"}
	ctx := context.Background()

	pl, err := svc.Plan(ctx, req)
	must(t, err)
	if len(pl.Steps) != 2 {
		t.Fatalf("the plan has %d steps, want the 2 model files: %+v", len(pl.Steps), pl.Steps)
	}
	for _, st := range pl.Steps {
		if st.Kind != provision.StepFile || filepath.Dir(filepath.Dir(st.File.Path)) != full(t, s.models) {
			t.Errorf("step %s %s at %s, want a model file in %s", st.Kind, st.Name, st.File.Path, s.models)
		}
	}

	res, err := svc.Install(ctx, req, provision.Watch{})
	if err != nil {
		t.Fatalf("install: %v: %s", err, statuses(res))
	}
	for _, p := range []string{filepath.Join(s.models, "vae", "demo-vae.safetensors"), filepath.Join(s.models, "text_encoders", "demo-te.safetensors")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("the install did not write %s: %v", p, err)
		}
	}
	if _, err := os.Stat(s.comfy); !os.IsNotExist(err) {
		t.Errorf("the install made the ComfyUI folder %s (extra_model_paths.yaml): %v", s.comfy, err)
	}
	rows, err := svc.List(ctx, s.workflows)
	must(t, err)
	if len(rows) != 1 || rows[0].State() != "installed" {
		t.Errorf("after the install: %+v, want demo installed", rows)
	}
	if pl, _ = svc.Plan(ctx, req); len(pl.Steps) != 0 {
		t.Errorf("after the install the plan has %d steps: %+v", len(pl.Steps), pl.Steps)
	}

	must(t, os.Remove(filepath.Join(s.models, "vae", "demo-vae.safetensors")))
	if pl, _ = svc.Plan(ctx, req); len(pl.Steps) != 1 || pl.Steps[0].Kind != provision.StepFile || pl.Steps[0].Name != "demo-vae.safetensors" {
		t.Errorf("one file removed: the plan %+v, want the one file step demo-vae.safetensors", pl.Steps)
	}
}
