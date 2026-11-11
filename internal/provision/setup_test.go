// Tests of the plan of horus setup, on the models.json of the workflow
// it-illustration of this repo (read only) and on fakes of the adapters
// whose folders are in a temporary folder.
package provision

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/graph"
)

// sizedFiles is a fake of the model files: every model file of folder F is
// in <root>/F. A file exists when sizes has its path. It never writes to the
// disk, and it records each call that changes a file.
type sizedFiles struct {
	root    string
	sizes   map[string]int64
	changes []string
}

func (f *sizedFiles) Dirs(folder string) []string { return []string{filepath.Join(f.root, folder)} }
func (f *sizedFiles) Own(folder string) string    { return filepath.Join(f.root, folder) }
func (f *sizedFiles) Mapped(string) []string      { return nil }
func (f *sizedFiles) Map(p [][2]string) error {
	f.changes = append(f.changes, fmt.Sprint("map ", p))
	return nil
}
func (f *sizedFiles) Delete(p string) error { f.changes = append(f.changes, "delete "+p); return nil }
func (f *sizedFiles) Size(p string) int64 {
	if n, ok := f.sizes[p]; ok {
		return n
	}
	return -1
}
func (f *sizedFiles) SHA256(string) (string, error) { return "", errors.New("no SHA-256 in this fake") }

// recordBackend is a fake of the ComfyUI install. Install writes "install"
// to the log and marks ComfyUI as installed.
type recordBackend struct {
	dir       string
	installed bool
	log       *[]string
}

func (b *recordBackend) Installed() (string, bool) { return b.dir, b.installed }
func (b *recordBackend) Install(_ context.Context, out io.Writer) error {
	*b.log = append(*b.log, "install")
	fmt.Fprintln(out, "installing ComfyUI")
	b.installed = true
	return nil
}

// noAnswer is a ComfyUI that does not answer: the check skips the server.
type noAnswer struct{}

func (noAnswer) ObjectInfo(context.Context) (graph.ObjectInfo, error) {
	return nil, errors.New("no answer")
}

// itIllustration is the size of each model file of it-illustration, by its
// folder and name.
var itIllustration = map[string]int64{
	filepath.Join("diffusion_models", "flux3-image-klein-int8-convrot.safetensors"): 5827874779,
	filepath.Join("text_encoders", "qwen3vl_4b_int8_convrot.safetensors"):           4858559240,
	filepath.Join("vae", "flux2-vae.safetensors"):                                   336211292,
}

type setupRig struct {
	svc     *Service
	files   *sizedFiles
	backend *recordBackend
	dl      *fakeDownloader
	log     []string
	req     Request
}

func newSetupRig(t *testing.T, installed, present bool) *setupRig {
	t.Helper()
	tmp := t.TempDir()
	r := &setupRig{files: &sizedFiles{root: filepath.Join(tmp, "models"), sizes: map[string]int64{}},
		req: Request{WorkflowsDir: filepath.Join("..", "..", "workflows"), Workflow: "it-illustration"}}
	r.backend = &recordBackend{dir: filepath.Join(tmp, "comfy"), installed: installed, log: &r.log}
	if present {
		for p, n := range itIllustration {
			r.files.sizes[filepath.Join(r.files.root, p)] = n
		}
	}
	r.dl = &fakeDownloader{fn: func(_ context.Context, f Fetch, _ io.Writer) (bool, error) {
		r.log = append(r.log, "download "+filepath.Base(f.Dest))
		r.files.sizes[f.Dest] = f.Bytes
		return false, nil
	}}
	r.svc = New(r.files, r.dl, noAnswer{}, r.backend, "")
	return r
}

func kinds(steps []Step) string {
	var s []string
	for _, st := range steps {
		if st.Kind == StepFile {
			s = append(s, st.Kind+" "+st.File.Role)
		} else {
			s = append(s, st.Kind)
		}
	}
	return strings.Join(s, ", ")
}

// TestPlanOfAnEmptyMachine: no ComfyUI and no model file give 1 backend step
// and 3 file steps, and the plan changes nothing.
func TestPlanOfAnEmptyMachine(t *testing.T) {
	r := newSetupRig(t, false, false)
	pl, err := r.svc.Plan(context.Background(), r.req)
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(pl.Steps); got != "backend, file diffusion, file text_encoder, file vae" {
		t.Fatalf("steps: %s", got)
	}
	if pl.Steps[0].Dir != r.backend.dir || pl.BackendInstalled || pl.BackendDir != r.backend.dir {
		t.Errorf("backend step: %+v", pl)
	}
	want := filepath.Join(r.files.root, "vae", "flux2-vae.safetensors")
	if f := pl.Steps[3].File; f.Path != want || f.Bytes != 336211292 || f.File != "flux2-vae.safetensors" || f.Present {
		t.Errorf("file step: %+v, want path %s", f, want)
	}
	if len(r.log) != 0 || len(r.dl.calls) != 0 || len(r.files.changes) != 0 {
		t.Errorf("the plan changed something: log %q, downloads %d, changes %q", r.log, len(r.dl.calls), r.files.changes)
	}
}

// TestPlanOfAReadyMachine: ComfyUI and every model file present give 0
// steps, and running that plan does nothing.
func TestPlanOfAReadyMachine(t *testing.T) {
	r := newSetupRig(t, true, true)
	pl, err := r.svc.Plan(context.Background(), r.req)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Steps) != 0 || len(pl.Files) != 3 || !pl.BackendInstalled {
		t.Fatalf("plan: %+v", pl)
	}
	for _, f := range pl.Files {
		if !f.Present {
			t.Errorf("not present: %+v", f)
		}
	}
	res, err := r.svc.RunPlan(context.Background(), r.req, pl, Watch{})
	if res != nil || err != nil || len(r.log) != 0 || len(r.files.changes) != 0 {
		t.Fatalf("an empty plan ran: %v %v, log %q, changes %q", res, err, r.log, r.files.changes)
	}
}

// TestPlanOfAFileWithAnotherSize: a file with another size is a file step.
func TestPlanOfAFileWithAnotherSize(t *testing.T) {
	r := newSetupRig(t, true, true)
	r.files.sizes[filepath.Join(r.files.root, "vae", "flux2-vae.safetensors")] = 7
	pl, err := r.svc.Plan(context.Background(), r.req)
	if err != nil || kinds(pl.Steps) != "file vae" {
		t.Fatalf("steps %q, err %v", kinds(pl.Steps), err)
	}
}

// TestRunPlanInstallsTheBackendThenTheFiles: with -yes the service installs
// ComfyUI first, then downloads the files of the plan, in that order.
func TestRunPlanInstallsTheBackendThenTheFiles(t *testing.T) {
	r := newSetupRig(t, false, false)
	pl, err := r.svc.Plan(context.Background(), r.req)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	res, err := r.svc.RunPlan(context.Background(), r.req, pl, Watch{Out: &out})
	if err != nil {
		t.Fatalf("err %v, result %+v", err, res)
	}
	want := "install|download flux3-image-klein-int8-convrot.safetensors|download qwen3vl_4b_int8_convrot.safetensors|download flux2-vae.safetensors"
	if got := strings.Join(r.log, "|"); got != want {
		t.Fatalf("order:\n%s\nwant:\n%s", got, want)
	}
	if !res.OK || statuses(res) == "" || !strings.Contains(out.String(), "installing ComfyUI") {
		t.Errorf("result %+v, out %q", res, out.String())
	}
	if pl, _ := r.svc.Plan(context.Background(), r.req); len(pl.Steps) != 0 {
		t.Errorf("after the run the plan still has steps: %s", kinds(pl.Steps))
	}
}

// TestRunPlanStopsWhenTheBackendFails: a failed install of ComfyUI starts
// no download.
func TestRunPlanStopsWhenTheBackendFails(t *testing.T) {
	r := newSetupRig(t, false, false)
	pl, _ := r.svc.Plan(context.Background(), r.req)
	failing := &failBackend{r.backend}
	svc := New(r.files, r.dl, noAnswer{}, failing, "")
	if _, err := svc.RunPlan(context.Background(), r.req, pl, Watch{}); err == nil || err.Error() != "no disk" {
		t.Fatalf("err %v", err)
	}
	if len(r.dl.calls) != 0 {
		t.Errorf("%d downloads after a failed install", len(r.dl.calls))
	}
}

type failBackend struct{ *recordBackend }

func (failBackend) Install(context.Context, io.Writer) error { return errors.New("no disk") }

// TestPlanOfAnUnknownWorkflow is the error of the models.json reader.
func TestPlanOfAnUnknownWorkflow(t *testing.T) {
	r := newSetupRig(t, true, true)
	r.req.Workflow = "no-such-workflow"
	if _, err := r.svc.Plan(context.Background(), r.req); err == nil {
		t.Fatal("no error")
	}
}
