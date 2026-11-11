// horus doctor (also horus check-setup): one PASS or FAIL line for each check
// of the setup. It ports the flags and the output of
// scripts/check-setup.ps1. The checks are in the service internal/doctor.
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

// RunDoctor runs horus doctor, or horus check-setup when name is
// "check-setup": the two names have the same flags and the same output.
func RunDoctor(ctx context.Context, name string, args []string, out, errOut io.Writer, at func(DoctorPlace) Doctor) int {
	var place DoctorPlace
	var o doctor.Options
	fs := newFlags("horus "+name, errOut)
	fs.StringVar(&place.ComfyUIDir, "comfyui-dir", "", "the portable folder (default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)")
	fs.StringVar(&place.ServerURL, "server-url", "http://127.0.0.1:8188", "address of the ComfyUI server")
	fs.StringVar(&o.VenvPath, "venv-path", "", "folder of the comfy-mcp venv (default: <home>/.venvs/comfy-mcp)")
	fs.StringVar(&o.Workflow, "workflow", "", "also check the model files of workflows/<name>/models.json")
	fs.BoolVar(&o.Quick, "quick", false, "with -workflow: check the size of each file only (the default also checks the SHA-256, and hashing 11 GB takes minutes)")
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
	rs := at(place).Check(ctx, o)
	for _, r := range rs {
		for _, l := range r.Lines {
			fmt.Fprintf(out, "      %s\n", l)
		}
		word := "FAIL"
		if r.OK {
			word = "PASS"
		}
		fmt.Fprintf(out, "%s  %s - %s\n", word, r.Name, r.Detail)
	}
	if n := doctor.Failed(rs); n > 0 {
		fmt.Fprintf(out, "%d check(s) failed.\n", n)
		return 1
	}
	fmt.Fprintln(out, "All checks passed.")
	return 0
}
