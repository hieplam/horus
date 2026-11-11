package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/provision"
)

// fakeSteps returns pl from Plan, and records the calls of RunPlan.
type fakeSteps struct {
	pl    provision.Plan
	err   error
	place SetupPlace
	req   provision.Request
	runs  int
}

func (f *fakeSteps) at(p SetupPlace) SetupSteps { f.place = p; return f }
func (f *fakeSteps) Plan(_ context.Context, r provision.Request) (provision.Plan, error) {
	f.req = r
	return f.pl, f.err
}
func (f *fakeSteps) RunPlan(_ context.Context, r provision.Request, pl provision.Plan, w provision.Watch) (*provision.Result, error) {
	f.runs++
	fmt.Fprintln(w.Out, "installing ComfyUI")
	w.Started("lic", 1)
	w.Downloading("vae", "/m/vae/v.safetensors")
	return &provision.Result{OK: true, Workflow: r.Workflow}, nil
}

func emptyPlan() provision.Plan {
	vae := provision.FileState{Role: "vae", File: "v.safetensors", Path: "/m/vae/v.safetensors", Bytes: 336211292}
	return provision.Plan{Workflow: "w", BackendDir: "/c", Files: []provision.FileState{vae},
		Steps: []provision.Step{{Kind: provision.StepBackend, Dir: "/c"}, {Kind: provision.StepFile, File: vae}}}
}

func runSetup(f *fakeSteps, args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := RunSetup(context.Background(), args, &out, &errOut, f.at)
	return code, out.String(), errOut.String()
}

// TestSetupPrintsThePlanAndRunsNothingWithoutYes: the plan of an empty
// machine is printed, and without -yes no step runs.
func TestSetupPrintsThePlanAndRunsNothingWithoutYes(t *testing.T) {
	f := &fakeSteps{pl: emptyPlan()}
	code, out, _ := runSetup(f, "w", "-comfyui-dir", "/c", "-models-dir", "/m")
	want := "Setup of workflow 'w':\n" +
		"MISSING   ComfyUI in /c\n" +
		"MISSING   vae v.safetensors (0.34 GB) /m/vae/v.safetensors\n" +
		"Plan: 2 step(s):\n" +
		"  1. install ComfyUI in /c\n" +
		"  2. download vae v.safetensors (0.34 GB) to /m/vae/v.safetensors\n" +
		"Nothing changed. Run horus setup w -yes to run the plan.\n"
	if code != 0 || out != want {
		t.Fatalf("exit %d:\n%s\nwant:\n%s", code, out, want)
	}
	if f.runs != 0 {
		t.Fatalf("RunPlan ran %d times without -yes", f.runs)
	}
	if f.req.Workflow != "w" || f.req.Yes || f.place.ComfyUIDir != "/c" || f.place.ModelsDir != "/m" || f.req.WorkflowsDir == "" {
		t.Errorf("request %+v, place %+v", f.req, f.place)
	}
}

// TestSetupWithYesRunsThePlan: with -yes the handler runs the plan once and
// prints its lines.
func TestSetupWithYesRunsThePlan(t *testing.T) {
	f := &fakeSteps{pl: emptyPlan()}
	code, out, _ := runSetup(f, "-yes", "w")
	if code != 0 || f.runs != 1 {
		t.Fatalf("exit %d, %d runs:\n%s", code, f.runs, out)
	}
	for _, w := range []string{"Running the plan.\ninstalling ComfyUI\nLicense: lic\nInstalling 1 model file(s) for workflow 'w'.\n-- vae -> /m/vae/v.safetensors\n",
		"OK: this machine is set up for workflow 'w'.\n"} {
		if !strings.Contains(out, w) {
			t.Errorf("no %q in:\n%s", w, out)
		}
	}
}

// TestSetupOfAReadyMachine: a plan with no step says that nothing is
// missing, and -yes runs nothing.
func TestSetupOfAReadyMachine(t *testing.T) {
	f := &fakeSteps{pl: provision.Plan{Workflow: "w", BackendDir: "/c", BackendInstalled: true,
		Files: []provision.FileState{{Role: "vae", File: "v.safetensors", Path: "/m/vae/v.safetensors", Bytes: 5_827_874_779, Present: true}}}}
	code, out, _ := runSetup(f, "w", "-yes")
	want := "Setup of workflow 'w':\nPRESENT   ComfyUI in /c\nPRESENT   vae v.safetensors (5.83 GB) /m/vae/v.safetensors\nPlan: 0 steps. Nothing is missing.\n"
	if code != 0 || out != want || f.runs != 0 {
		t.Fatalf("exit %d, %d runs:\n%s\nwant:\n%s", code, f.runs, out, want)
	}
}

func TestSetupFlagAndPlanErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"a", "b"}, {"-nope", "w"}} {
		f := &fakeSteps{}
		if code, _, _ := runSetup(f, args...); code != 2 {
			t.Errorf("%q: exit %d", args, code)
		}
	}
	f := &fakeSteps{err: errors.New("no models.json")}
	if code, out, _ := runSetup(f, "w"); code != 1 || out != "FAIL: no models.json\n" {
		t.Errorf("exit %d: %q", code, out)
	}
	if code, _, _ := runSetup(&fakeSteps{}, "-h"); code != 0 {
		t.Errorf("-h: exit %d", code)
	}
}

// failSteps fails the run of the plan.
type failSteps struct{ fakeSteps }

func (f *failSteps) RunPlan(context.Context, provision.Request, provision.Plan, provision.Watch) (*provision.Result, error) {
	return nil, errors.New("no disk")
}

func TestSetupReportsAFailedRun(t *testing.T) {
	f := &failSteps{fakeSteps{pl: emptyPlan()}}
	var out bytes.Buffer
	code := RunSetup(context.Background(), []string{"w", "-yes"}, &out, io.Discard, func(SetupPlace) SetupSteps { return f })
	if code != 1 || !strings.HasSuffix(out.String(), "Running the plan.\nFAIL: no disk\n") {
		t.Fatalf("exit %d:\n%s", code, out.String())
	}
}
