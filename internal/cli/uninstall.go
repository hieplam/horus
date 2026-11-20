// horus uninstall: remove what horus installed. It prints the plan: one line
// for each item, with REMOVE or KEEP, the path, and the reason of a KEEP.
// Without -yes it changes nothing. With -yes it removes each REMOVE item, in
// this order: it stops the ComfyUI that horus started, then removes the model
// files (also of a speech workflow), ComfyUI, the venv, the speech venv and
// the file of the speech worker, the MCP registration of Claude Code and
// Codex, the files of horus in the user folder, the PATH entry of the install
// script, and the program file. -models limits it to the model files, -mcp
// to the MCP registration. The work is in the service internal/provision.
//
// Exit code 0 = the plan is printed (without -yes), or each REMOVE item is
// gone (with -yes); 1 = a REMOVE item is still there, or the plan failed;
// 2 = a flag or an argument that does not parse.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// Uninstaller is what horus uninstall needs from the setup service.
// cmd/horus gives it a *provision.Service.
type Uninstaller interface {
	UninstallPlan(ctx context.Context, req provision.UninstallRequest) (*provision.Uninstall, error)
	RunUninstall(ctx context.Context, u *provision.Uninstall, out io.Writer) error
}

// RunUninstall runs horus uninstall with args (the arguments after the
// command name). at gives the service for the place of the flags.
func RunUninstall(ctx context.Context, args []string, out, errOut io.Writer, at func(SetupPlace) Uninstaller) int {
	var place SetupPlace
	var req provision.UninstallRequest
	var yes bool
	fs := newFlags("horus uninstall", errOut)
	fs.BoolVar(&yes, "yes", false, "remove each REMOVE item of the plan; without it, only print the plan")
	fs.BoolVar(&req.Models, "models", false, "only the model files of the workflows")
	fs.BoolVar(&req.MCP, "mcp", false, "only the MCP registration of Claude Code and Codex")
	fs.StringVar(&place.ComfyUIDir, "comfyui-dir", "", "the ComfyUI install folder "+dirHelp)
	fs.StringVar(&place.ModelsDir, "models-dir", "", "the models folder (default: HORUS_MODELS_DIR, then <comfyui-dir>/ComfyUI/models)")
	fs.StringVar(&place.TextEncoderDir, "text-encoder-dir", "", "the text encoder folder (default: HORUS_TEXT_ENCODER_DIR)")
	fs.StringVar(&place.ServerURL, "server-url", "", "the ComfyUI address (default: HORUS_COMFYUI_URL, then http://127.0.0.1:8188)")
	fs.StringVar(&req.WorkflowsDir, "workflows-dir", "", "use this folder instead of the repo's workflows folder")
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, "Usage: horus uninstall [-yes] [-models] [-mcp] [flags]")
		fmt.Fprintln(w, "Print what horus installed on this machine. With -yes, remove it: the model files, ComfyUI, the venv")
		fmt.Fprintln(w, "and the speech venv (only a folder with the install record horus-install.json), the file of the speech")
		fmt.Fprintln(w, "worker, the MCP registration, the files of horus in the user folder, the PATH entry of the install script,")
		fmt.Fprintln(w, "and this program. .env and outputs/ stay. A system package (7-Zip, Python, FFmpeg) stays.")
		fs.PrintDefaults()
	}
	if code, done := parse(fs, args); done {
		return code
	}
	if fs.NArg() > 0 {
		fs.Usage()
		return 2
	}
	if req.WorkflowsDir == "" {
		req.WorkflowsDir = settings.WorkflowsDir()
	}
	p := &printer{out: out}
	svc := at(place)
	u, err := svc.UninstallPlan(ctx, req)
	if err != nil {
		p.say("FAIL: %v", err)
		return 1
	}
	p.say("Uninstall plan:")
	for _, it := range u.Items {
		p.say("%s", itemText(it))
	}
	if u.Removes() == 0 {
		p.say("Nothing to remove.")
		return 0
	}
	if !yes {
		p.say("Nothing changed. Run horus uninstall%s -yes to remove the %d REMOVE item(s).", partFlags(req), u.Removes())
		return 0
	}
	p.say("")
	p.say("Running the plan.")
	err = svc.RunUninstall(ctx, u, out)
	for _, it := range u.Items {
		if !it.Remove {
			continue
		}
		if it.Done {
			p.say("OK: removed %s: %s", it.What, it.Path)
			continue
		}
		why := "it is still there"
		if it.Err != nil {
			why = it.Err.Error()
		}
		p.say("FAIL: %s: %s: %s", it.What, it.Path, why)
	}
	if err != nil {
		p.say("FAIL: %v", err)
		return 1
	}
	p.say("OK: %d REMOVE item(s) are gone.", u.Removes())
	if hasPart(u, provision.PartProgram) {
		p.say("Open a new terminal: the PATH of this terminal is old.")
	}
	return 0
}

// itemText is the line of one item of the plan.
func itemText(it provision.UninstallItem) string {
	word := "KEEP  "
	if it.Remove {
		word = "REMOVE"
	}
	line := fmt.Sprintf("%s  %s  (%s", word, it.Path, it.What)
	if it.Why != "" {
		line += ": " + it.Why
	}
	if len(it.Command) > 0 {
		line += "; runs " + strings.Join(it.Command, " ")
	}
	return line + ")"
}

// partFlags gives the flags -models and -mcp of the request, for the text.
func partFlags(r provision.UninstallRequest) string {
	s := ""
	if r.Models {
		s += " -models"
	}
	if r.MCP {
		s += " -mcp"
	}
	return s
}

func hasPart(u *provision.Uninstall, part string) bool {
	for _, it := range u.Items {
		if it.Part == part && it.Remove {
			return true
		}
	}
	return false
}
