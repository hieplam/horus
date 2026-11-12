// horus setup <workflow>: make this machine ready for one workflow. It
// prints the plan: is ComfyUI installed, and is each model file of the
// workflow present (a size check). Without -yes it changes nothing. With
// -yes it runs the plan: it installs ComfyUI when it is missing, then
// downloads the missing model files (as horus workflow install does). The
// work is in the service internal/provision.
//
// Exit code 0 = the plan is printed (without -yes) or done (with -yes),
// 1 = anything else, 2 = a flag or an argument that does not parse.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// SetupSteps is what horus setup needs from the setup service. cmd/horus
// gives it a *provision.Service.
type SetupSteps interface {
	Plan(ctx context.Context, req provision.Request) (provision.Plan, error)
	RunPlan(ctx context.Context, req provision.Request, pl provision.Plan, w provision.Watch) (*provision.Result, error)
}

// RunSetup runs horus setup with args (the arguments after the command
// name). at gives the setup service for the place of the flags.
func RunSetup(ctx context.Context, args []string, out, errOut io.Writer, at func(SetupPlace) SetupSteps) int {
	var place SetupPlace
	var req provision.Request
	fs := newFlags("horus setup", errOut)
	fs.BoolVar(&req.Yes, "yes", false, "run the plan: install ComfyUI and download the model files; without it, only print the plan")
	fs.StringVar(&place.ComfyUIDir, "comfyui-dir", "", "the ComfyUI install folder "+dirHelp)
	fs.StringVar(&place.ModelsDir, "models-dir", "", "the models folder (default: HORUS_MODELS_DIR, then <comfyui-dir>/ComfyUI/models)")
	fs.StringVar(&place.TextEncoderDir, "text-encoder-dir", "", "the text encoder folder (default: HORUS_TEXT_ENCODER_DIR)")
	fs.StringVar(&place.ServerURL, "server-url", "", "the ComfyUI address (default: HORUS_COMFYUI_URL, then http://127.0.0.1:8188)")
	fs.StringVar(&req.WorkflowsDir, "workflows-dir", "", "use this folder instead of the repo's workflows folder")
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, "Usage: horus setup <workflow> [-yes] [flags]")
		fmt.Fprintln(w, "Print what this machine needs for the workflow. With -yes, install it: ComfyUI first, then the model files.")
		fs.PrintDefaults()
	}
	// Flags may come before and after the workflow name.
	var pos []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	if len(pos) != 1 {
		fs.Usage()
		return 2
	}
	req.Workflow = pos[0]
	if req.WorkflowsDir == "" {
		req.WorkflowsDir = filepath.Join(settings.Root(), "workflows")
	}
	p := &printer{out: out}
	svc := at(place)
	pl, err := svc.Plan(ctx, req)
	if err != nil {
		p.say("FAIL: %v", err)
		return 1
	}
	printPlan(p, pl)
	if len(pl.Steps) == 0 {
		return 0
	}
	if !req.Yes {
		p.say("Nothing changed. Run horus setup %s -yes to run the plan.", req.Workflow)
		return 0
	}
	p.say("")
	p.say("Running the plan.")
	res, err := svc.RunPlan(ctx, req, pl, provision.Watch{
		Started: func(license string, files int) {
			p.say("License: %s", license)
			p.say("Installing %d model file(s) for workflow '%s'.", files, req.Workflow)
		},
		Downloading: func(role, dest string) { p.say("-- %s -> %s", role, dest) },
		Out:         out,
		Failed:      func(err error) { p.say("FAIL: %v", err) },
	})
	if res != nil {
		p.install(res)
	}
	if err != nil {
		p.say("FAIL: %v", err)
		return 1
	}
	p.say("OK: this machine is set up for workflow '%s'.", req.Workflow)
	return 0
}

// printPlan writes the facts and the steps of a plan.
func printPlan(p *printer, pl provision.Plan) {
	p.say("Setup of workflow '%s':", pl.Workflow)
	state := func(ok bool) string {
		if ok {
			return "PRESENT "
		}
		return "MISSING "
	}
	p.say("%s  ComfyUI in %s", state(pl.BackendInstalled), pl.BackendDir)
	for _, f := range pl.Files {
		p.say("%s  %s %s (%s GB) %s", state(f.Present), f.Role, f.File, gb2(f.Bytes), f.Path)
	}
	if len(pl.Steps) == 0 {
		p.say("Plan: 0 steps. Nothing is missing.")
		return
	}
	p.say("Plan: %d step(s):", len(pl.Steps))
	for i, st := range pl.Steps {
		if st.Kind == provision.StepBackend {
			p.say("  %d. install ComfyUI in %s", i+1, st.Dir)
			continue
		}
		p.say("  %d. download %s %s (%s GB) to %s", i+1, st.File.Role, st.File.File, gb2(st.File.Bytes), st.File.Path)
	}
}
