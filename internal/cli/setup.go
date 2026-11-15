// horus setup <workflow>: make this machine ready for one workflow. It
// prints the plan: is ComfyUI installed, and is each model file of the
// workflow present (a size check). Without -yes it changes nothing. With
// -yes it runs the plan: the system package (7-Zip on Windows, Python 3.12
// on macOS), comfy-cli (macOS), ComfyUI, then the missing model files (as
// horus workflow install does). A package step runs only with
// -with-packages: without it, -yes changes nothing. When no package manager
// is found, the package step is a step for a person, and -yes changes
// nothing. The work is in the service internal/provision.
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
	"strings"

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
	fs.BoolVar(&req.WithPackages, "with-packages", false, "with -yes: also run the package steps (7-Zip with winget, Python 3.12 with brew)")
	fs.StringVar(&place.ComfyUIDir, "comfyui-dir", "", "the ComfyUI install folder "+dirHelp)
	fs.StringVar(&place.ModelsDir, "models-dir", "", "the models folder (default: HORUS_MODELS_DIR, then <comfyui-dir>/ComfyUI/models)")
	fs.StringVar(&place.TextEncoderDir, "text-encoder-dir", "", "the text encoder folder (default: HORUS_TEXT_ENCODER_DIR)")
	fs.StringVar(&place.ServerURL, "server-url", "", "the ComfyUI address (default: HORUS_COMFYUI_URL, then http://127.0.0.1:8188)")
	fs.StringVar(&req.WorkflowsDir, "workflows-dir", "", "use this folder instead of the repo's workflows folder")
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, "Usage: horus setup <workflow> [-yes [-with-packages]] [flags]")
		fmt.Fprintln(w, "Print what this machine needs for the workflow. With -yes, install it: the system package, comfy-cli, ComfyUI, then the model files.")
		fmt.Fprintln(w, "A step that installs a system package runs only with -with-packages.")
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
		req.WorkflowsDir = settings.WorkflowsDir()
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
		if hasKind(pl, provision.StepPerson) {
			p.say("Nothing changed. Install the package by hand (step above), then run horus setup %s again.", req.Workflow)
		} else if hasKind(pl, provision.StepPackage) {
			p.say("Nothing changed. Run horus setup %s -yes -with-packages to run the plan.", req.Workflow)
		} else {
			p.say("Nothing changed. Run horus setup %s -yes to run the plan.", req.Workflow)
		}
		return 0
	}
	if refused(p, req, pl.Refusal(req.WithPackages)) {
		return 1
	}
	p.say("")
	p.say("Running the plan.")
	res, err := svc.RunPlan(ctx, req, pl, provision.Watch{
		Step: func(st provision.Step) {
			if st.Kind != provision.StepPackage && st.Kind != provision.StepComfyCLI {
				return
			}
			p.say("-- %s", stepText(st))
		},
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
	if refused(p, req, err) {
		return 1
	}
	if err != nil {
		p.say("FAIL: %v", err)
		return 1
	}
	p.say("OK: this machine is set up for workflow '%s'.", req.Workflow)
	return 0
}

// refused writes the lines of a plan that the service refuses before any
// step (provision.Plan.Refusal), and reports if err is such a refusal.
func refused(p *printer, req provision.Request, err error) bool {
	var person *provision.PersonStepError
	var approval *provision.PackageApprovalError
	switch {
	case errors.As(err, &person):
		pkg := person.Package
		p.say("FAIL: %s was not found, so horus cannot install %s. Nothing changed.", pkg.Manager, pkg.Name)
		p.say("Install %s by hand from %s, then run horus setup %s -yes again.", pkg.Name, pkg.URL, req.Workflow)
		return true
	case errors.As(err, &approval):
		p.say("Nothing changed. The plan installs a system package with its package manager:")
		for _, st := range approval.Steps {
			p.say("  %s: %s", st.Name, strings.Join(st.Command, " "))
		}
		p.say("To go on, do one of these:")
		p.say("  - run horus setup %s -yes -with-packages", req.Workflow)
		for _, st := range approval.Steps {
			p.say("  - install %s by hand from %s, then run horus setup %s -yes", st.Name, st.Package.URL, req.Workflow)
		}
		return true
	}
	return false
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
	if pkg := pl.Package; pkg != nil {
		p.say("%s  %s (%s) %s", state(pl.ProgramPath != ""), pkg.Name, pkg.Program, pl.ProgramPath)
		p.say("%s  package manager %s %s", state(pl.ManagerPath != ""), pkg.Manager, pl.ManagerPath)
	}
	if pl.ComfyCLI {
		p.say("%s  comfy-cli (comfy) %s", state(pl.ComfyPath != ""), pl.ComfyPath)
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
		p.say("  %d. %s", i+1, stepText(st))
	}
}

// stepText is the line of one step of a plan.
func stepText(st provision.Step) string {
	switch st.Kind {
	case provision.StepPackage:
		return fmt.Sprintf("install the system package %s (needs -with-packages): %s", st.Name, strings.Join(st.Command, " "))
	case provision.StepPerson:
		return fmt.Sprintf("a step for a person: %s was not found. Install %s by hand from %s", st.Package.Manager, st.Name, st.Package.URL)
	case provision.StepComfyCLI:
		return fmt.Sprintf("install comfy-cli (as horus install-comfy-mcp does) in %s", st.Dir)
	case provision.StepBackend:
		return fmt.Sprintf("install ComfyUI in %s", st.Dir)
	}
	return fmt.Sprintf("download %s %s (%s GB) to %s", st.File.Role, st.File.File, gb2(st.File.Bytes), st.File.Path)
}

// hasKind tells if the plan has a step of the kind.
func hasKind(pl provision.Plan, kind string) bool {
	for _, st := range pl.Steps {
		if st.Kind == kind {
			return true
		}
	}
	return false
}
