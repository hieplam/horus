// horus doctor (also horus check-setup): one PASS or FAIL line for each check
// of the setup. It ports the flags and the output of
// scripts/check-setup.ps1. The checks are in the service internal/doctor.
// With -workflow, the handler then runs the check of horus workflow check on
// the setup service, and writes its lines with an indent.
//
// Exit code 0 = all checks passed, 1 = a check failed, 2 = a flag that does
// not parse.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hieplam/horus/internal/doctor"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// Doctor is what horus doctor needs from the doctor service. cmd/horus gives
// it a *doctor.Service.
type Doctor interface {
	Check(ctx context.Context, o doctor.Options) []doctor.Result
}

// DoctorPlace is the ComfyUI and the folders that the flags of horus doctor
// pick. An empty field means the setting.
type DoctorPlace struct {
	ComfyUIDir     string // -comfyui-dir
	ServerURL      string // -server-url
	ModelsDir      string // -models-dir, for the model check
	TextEncoderDir string // -text-encoder-dir, for the model check
}

// modelIndent comes before each line of the model check.
const modelIndent = "      "

// RunDoctor runs horus doctor, or horus check-setup when name is
// "check-setup": the two names have the same flags and the same output. at
// gives the doctor service, and the setup service for the model check, for
// the place of the flags.
func RunDoctor(ctx context.Context, name string, args []string, out, errOut io.Writer, at func(DoctorPlace) (Doctor, Setup)) int {
	var place DoctorPlace
	var o doctor.Options
	var workflow string
	var quick bool
	fs := newFlags("horus "+name, errOut)
	fs.StringVar(&place.ComfyUIDir, "comfyui-dir", "", "the portable folder (default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)")
	fs.StringVar(&place.ServerURL, "server-url", "http://127.0.0.1:8188", "address of the ComfyUI server")
	fs.StringVar(&o.VenvPath, "venv-path", "", "folder of the comfy-mcp venv (default: <home>/.venvs/comfy-mcp)")
	fs.StringVar(&workflow, "workflow", "", "also check the model files of workflows/<name>/models.json")
	fs.BoolVar(&quick, "quick", false, "with -workflow: check the size of each file only (the default also checks the SHA-256, and hashing 11 GB takes minutes)")
	fs.StringVar(&place.ModelsDir, "models-dir", "", "with -workflow: models folder (default: HORUS_MODELS_DIR)")
	fs.StringVar(&place.TextEncoderDir, "text-encoder-dir", "", "with -workflow: folder of the text encoder on another drive (default: HORUS_TEXT_ENCODER_DIR)")
	if code, done := parse(fs, args); done {
		return code
	}
	home, _ := os.UserHomeDir()
	o.ServerURL, o.RepoRoot, o.Home = place.ServerURL, settings.Root(), home
	if o.VenvPath == "" {
		o.VenvPath = filepath.Join(home, ".venvs", "comfy-mcp")
	}
	doc, setup := at(place)
	rs := doc.Check(ctx, o)
	for _, r := range rs {
		resultLine(out, r.OK, r.Name, r.Detail)
	}
	n := doctor.Failed(rs)
	if workflow != "" && !modelCheck(ctx, setup, workflow, quick, out) {
		n++
	}
	if n > 0 {
		fmt.Fprintf(out, "%d check(s) failed.\n", n)
		return 1
	}
	fmt.Fprintln(out, "All checks passed.")
	return 0
}

// resultLine writes the PASS or FAIL line of one check.
func resultLine(out io.Writer, ok bool, name, detail string) {
	word := "FAIL"
	if ok {
		word = "PASS"
	}
	fmt.Fprintf(out, "%s  %s - %s\n", word, name, detail)
}

// modelCheck checks the model files of workflow on the setup service, as
// horus workflow check does. It writes the lines of that check with an
// indent, then the PASS or FAIL line, and reports if the check passed.
// quick: the size of each file only, no SHA-256.
func modelCheck(ctx context.Context, svc Setup, workflow string, quick bool, out io.Writer) bool {
	req := provision.Request{Workflow: workflow, Quick: quick, WorkflowsDir: filepath.Join(settings.Root(), "workflows")}
	res, err := svc.Check(ctx, req)
	p := &printer{out: out, indent: modelIndent}
	p.check(res)
	name := fmt.Sprintf("models for '%s'", workflow)
	switch {
	case err != nil:
		p.say("FAIL: %v", err)
		resultLine(out, false, name, "see lines above. Fix: horus workflow install "+workflow)
	case quick:
		p.say("OK: workflow '%s' is ready.", workflow)
		resultLine(out, true, name, "all files present, size checked (the SHA-256 is not checked; drop -quick)")
	default:
		p.say("OK: workflow '%s' is ready.", workflow)
		resultLine(out, true, name, "all files present and verified (SHA-256 checked)")
	}
	return err == nil
}
