// The setup of a machine for one workflow (horus setup): first the plan,
// then, only when the caller asks, the steps of the plan.
//
// The order of the steps is the order of the spec
// (docs/specs/2026-11-15-release-install.md, "The setup plan"): the system
// package (7-Zip on Windows, Python 3.12 on macOS), comfy-cli (macOS),
// ComfyUI, then each model file. A machine where ComfyUI is installed gets
// no package step and no comfy-cli step.
package provision

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// The kinds of a step.
const (
	StepPackage  = "package"   // install a system package with the package manager
	StepPerson   = "person"    // a person installs a system package: no package manager was found
	StepComfyCLI = "comfy-cli" // install comfy-cli in the venv of horus install-comfy-mcp
	StepBackend  = "backend"   // install ComfyUI
	StepFile     = "file"      // download one model file
)

// comfyPython is the Python version of comfy-cli (horus install-comfy-mcp).
const comfyPython = "3.12"

// FileState is one model file of the workflow on this machine.
type FileState struct {
	Role, File string
	// Path is where the file is, or where a download puts it.
	Path  string
	Bytes int64 // the size that models.json says
	// Present is true when a file of that size is at Path.
	Present bool
}

// Step is one step of a plan.
type Step struct {
	Kind string // one of the Step kinds
	// Name is what the step installs: the package name, "comfy-cli",
	// "ComfyUI" or the file name.
	Name string
	// Dir is the folder that the step writes to: the folder of the program
	// of a package (StepPackage), the venv (StepComfyCLI) or the install
	// folder (StepBackend).
	Dir string
	// Package is the system package (StepPackage and StepPerson).
	Package *platform.Package
	// Command is the program and its arguments that a StepPackage runs.
	Command []string
	// File is the model file (StepFile).
	File FileState
}

// System is what the plan needs from the programs of this machine.
// cmd/horus gives it the needs of the backend (backend.SetupNeeds) and
// platform.Programs. The zero value adds no package step and no comfy-cli
// step.
type System struct {
	Programs Programs
	// Package is the system package that the ComfyUI install needs, or nil.
	Package *platform.Package
	// ComfyCLI is true when the ComfyUI install needs the program comfy.
	ComfyCLI bool
	// VenvPath is the venv of horus install-comfy-mcp; "" means
	// settings.ComfyMCPVenv().
	VenvPath string
}

// WithSystem gives the service the programs of this machine and the needs
// of the backend, for the package and comfy-cli steps of a plan. It returns
// the service.
func (s *Service) WithSystem(sys System) *Service {
	s.sys = sys
	return s
}

func (s *Service) venv() string {
	if s.sys.VenvPath != "" {
		return s.sys.VenvPath
	}
	return settings.ComfyMCPVenv()
}

// Plan is what horus setup found on this machine, and the steps that make
// it ready for the workflow.
type Plan struct {
	Workflow         string
	BackendDir       string
	BackendInstalled bool
	// The facts of the system package and of comfy-cli. They are read only
	// when ComfyUI is not installed.
	Package     *platform.Package // the package that the install needs, or nil
	ProgramPath string            // the program of Package, or "" when it is not found
	ManagerPath string            // the package manager, or "" when it is not found
	ComfyCLI    bool              // the install needs the program comfy
	ComfyPath   string            // the program comfy, or "" when it is not found
	VenvPath    string            // where the comfy-cli step installs comfy
	Files       []FileState
	Steps       []Step
}

// steps gives the steps for the facts of pl, in the order of the spec. It
// is a pure function.
func steps(pl Plan) []Step {
	var out []Step
	if !pl.BackendInstalled {
		needComfy := pl.ComfyCLI && pl.ComfyPath == ""
		// The package is needed for the install itself (7-Zip), or for
		// comfy-cli (Python 3.12) when comfy is missing.
		if pkg := pl.Package; pkg != nil && pl.ProgramPath == "" && (!pl.ComfyCLI || needComfy) {
			if pl.ManagerPath == "" {
				out = append(out, Step{Kind: StepPerson, Name: pkg.Name, Package: pkg})
			} else {
				out = append(out, Step{Kind: StepPackage, Name: pkg.Name, Package: pkg,
					Dir: packageDir(pkg, pl.ManagerPath), Command: append([]string{pl.ManagerPath}, pkg.Args...)})
			}
		}
		if needComfy {
			out = append(out, Step{Kind: StepComfyCLI, Name: "comfy-cli", Dir: pl.VenvPath})
		}
		out = append(out, Step{Kind: StepBackend, Name: "ComfyUI", Dir: pl.BackendDir})
	}
	for _, f := range pl.Files {
		if !f.Present {
			out = append(out, Step{Kind: StepFile, Name: f.File, File: f})
		}
	}
	return out
}

// packageDir is the folder where the package manager puts the program of
// pkg: the first of its folders, else the folder of the manager.
func packageDir(pkg *platform.Package, manager string) string {
	if len(pkg.Dirs) > 0 {
		return pkg.Dirs[0]
	}
	if pkg.InManagerDir {
		return filepath.Dir(manager)
	}
	return ""
}

// Plan reads the facts of this machine for the workflow of req: is ComfyUI
// installed, and is each model file present with its size (no SHA-256, so
// the plan is quick). When ComfyUI is not installed, it also looks for the
// system package, its package manager and comfy. It changes nothing.
func (s *Service) Plan(ctx context.Context, req Request) (Plan, error) {
	pl := Plan{Workflow: req.Workflow}
	m, _, err := catalog.FindModelList(req.WorkflowsDir, req.Workflow, req.ModelsJSON)
	if err != nil {
		return pl, err
	}
	pl.BackendDir, pl.BackendInstalled = s.backend.Installed()
	if !pl.BackendInstalled && s.sys.Programs != nil {
		pl.ComfyCLI = s.sys.ComfyCLI
		if pl.ComfyCLI {
			pl.VenvPath = s.venv()
			pl.ComfyPath = s.findComfy(pl.VenvPath)
		}
		if pkg := s.sys.Package; pkg != nil {
			pl.Package = pkg
			pl.ManagerPath = s.lookPath(pkg.Manager)
			pl.ProgramPath = s.findProgram(pkg, pl.ManagerPath)
		}
	}
	for _, f := range m.Files {
		st := FileState{Role: f.Role, File: f.Name, Bytes: f.Bytes, Path: s.find(f)}
		if st.Path == "" {
			st.Path = filepath.Join(s.files.Dirs(f.Folder)[0], f.Name)
		} else {
			st.Present = s.files.Size(st.Path) == f.Bytes
		}
		pl.Files = append(pl.Files, st)
	}
	pl.Steps = steps(pl)
	return pl, nil
}

func (s *Service) lookPath(name string) string {
	if p, err := s.sys.Programs.LookPath(name); err == nil {
		return p
	}
	return ""
}

// findProgram gives the path of the program of pkg: the PATH first, then the
// folders where the package manager puts it.
func (s *Service) findProgram(pkg *platform.Package, manager string) string {
	if p := s.lookPath(pkg.Program); p != "" {
		return p
	}
	dirs := pkg.Dirs
	if pkg.InManagerDir && manager != "" {
		dirs = append(dirs[:len(dirs):len(dirs)], filepath.Dir(manager))
	}
	for _, d := range dirs {
		for _, name := range []string{platform.ExeName(pkg.Program), pkg.Program} {
			if p := filepath.Join(d, name); platform.IsFile(p) {
				return p
			}
		}
	}
	return ""
}

// findComfy gives the path of the program comfy: the PATH first, then the
// venv of horus install-comfy-mcp (as the backend of macOS finds it).
func (s *Service) findComfy(venv string) string {
	if p := s.lookPath("comfy"); p != "" {
		return p
	}
	if p := platform.VenvBin(venv, "comfy"); platform.IsFile(p) {
		return p
	}
	return ""
}

// PersonStepError is the answer of RunPlan for a plan with a step for a
// person: no package manager was found. RunPlan changed nothing.
type PersonStepError struct{ Package *platform.Package }

func (e *PersonStepError) Error() string {
	return fmt.Sprintf("%s was not found, so horus cannot install %s. Install %s by hand from %s, then run the setup again.",
		e.Package.Manager, e.Package.Name, e.Package.Name, e.Package.URL)
}

// PackageApprovalError is the answer of RunPlan for a plan with a package
// step when the request does not approve package steps (WithPackages).
// RunPlan changed nothing.
type PackageApprovalError struct{ Steps []Step }

func (e *PackageApprovalError) Error() string {
	var parts []string
	for _, st := range e.Steps {
		parts = append(parts, fmt.Sprintf("%s (%s)", st.Name, strings.Join(st.Command, " ")))
	}
	return "the plan installs a system package and a person did not approve it: " + strings.Join(parts, ", ")
}

// Refusal gives the error of RunPlan for a plan that it refuses before any
// step: a step for a person (*PersonStepError), or a package step when
// withPackages is false (*PackageApprovalError). It is nil when RunPlan can
// run the plan. It is a pure function.
func (pl Plan) Refusal(withPackages bool) error {
	var pkgs []Step
	for _, st := range pl.Steps {
		switch st.Kind {
		case StepPerson:
			return &PersonStepError{Package: st.Package}
		case StepPackage:
			pkgs = append(pkgs, st)
		}
	}
	if len(pkgs) > 0 && !withPackages {
		return &PackageApprovalError{Steps: pkgs}
	}
	return nil
}

// RunPlan runs the steps of pl in order. Before it runs a step, it refuses
// a plan with a step for a person (*PersonStepError), and a plan with a
// package step when req.WithPackages is false (*PackageApprovalError). Both
// change nothing.
//
// A package step runs the package manager, and its lines go to w.Out. Then
// RunPlan looks for the program again in the folders of the package. A
// comfy-cli step does the work of horus install-comfy-mcp. A backend step
// installs ComfyUI. The file steps go to Install, limited to the files of
// the plan. The result is the result of Install, or nil when the plan has
// no file step. A plan with no step changes nothing.
func (s *Service) RunPlan(ctx context.Context, req Request, pl Plan, w Watch) (*Result, error) {
	if err := pl.Refusal(req.WithPackages); err != nil {
		return nil, err
	}
	out := w.Out
	if out == nil {
		out = io.Discard
	}
	program := pl.ProgramPath // the program of the package: the Python of comfy-cli on macOS
	var names []string
	for _, st := range pl.Steps {
		if st.Kind != StepFile && w.Step != nil {
			w.Step(st)
		}
		switch st.Kind {
		case StepPackage:
			p, err := s.installPackage(ctx, st, out)
			if err != nil {
				return nil, err
			}
			program = p
		case StepComfyCLI:
			o := ComfyMCPOptions{VenvPath: st.Dir, PythonVersion: comfyPython, Stdout: out, Stderr: out}
			if program != "" {
				o.Launcher = []string{program}
			}
			if err := InstallComfyMCP(ctx, s.sys.Programs, o); err != nil {
				return nil, err
			}
		case StepBackend:
			if err := s.backend.Install(ctx, out); err != nil {
				return nil, err
			}
		case StepFile:
			names = append(names, st.File.File)
		}
	}
	if len(names) == 0 {
		return nil, nil
	}
	req.Only = names
	return s.Install(ctx, req, w)
}

// installPackage runs the command of a package step, then gives the path of
// the program of the package. It looks in the folders of the package,
// because the PATH of this process is older than the install.
func (s *Service) installPackage(ctx context.Context, st Step, out io.Writer) (string, error) {
	fmt.Fprintf(out, "Running %s ...\n", strings.Join(st.Command, " "))
	code, err := s.sys.Programs.Run(ctx, platform.Command{Path: st.Command[0], Args: st.Command[1:], Stdout: out, Stderr: out})
	if err != nil || code != 0 {
		return "", fmt.Errorf("%s failed (%s). Install %s by hand from %s, then run the setup again.",
			strings.Join(st.Command, " "), exitText(code, err), st.Package.Name, st.Package.URL)
	}
	p := s.findProgram(st.Package, st.Command[0])
	if p == "" {
		return "", fmt.Errorf("%s ended, but %s was not found on the PATH or in %s. Open a new terminal and run the setup again.",
			strings.Join(st.Command, " "), st.Package.Program, strings.Join(searchDirs(st), ", "))
	}
	fmt.Fprintf(out, "OK: %s is installed: %s\n", st.Package.Name, p)
	return p, nil
}

// searchDirs are the folders that findProgram reads for the package of st.
func searchDirs(st Step) []string {
	dirs := append([]string(nil), st.Package.Dirs...)
	if st.Package.InManagerDir {
		dirs = append(dirs, filepath.Dir(st.Command[0]))
	}
	return dirs
}
