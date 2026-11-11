// The setup of a machine for one workflow (horus setup): first the plan,
// then, only when the caller asks, the steps of the plan.
package provision

import (
	"context"
	"io"
	"path/filepath"

	"github.com/hieplam/horus/internal/catalog"
)

// The kinds of a step.
const (
	StepBackend = "backend" // install ComfyUI
	StepFile    = "file"    // download one model file
)

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
	Kind string // StepBackend or StepFile
	// Dir is the install folder (StepBackend).
	Dir string
	// File is the model file (StepFile).
	File FileState
}

// Plan is what horus setup found on this machine, and the steps that make
// it ready for the workflow.
type Plan struct {
	Workflow         string
	BackendDir       string
	BackendInstalled bool
	Files            []FileState
	Steps            []Step
}

// steps gives the steps for the facts: install the backend when it is
// missing, then download each file that is not present, in the order of
// models.json.
func steps(dir string, installed bool, files []FileState) []Step {
	var out []Step
	if !installed {
		out = append(out, Step{Kind: StepBackend, Dir: dir})
	}
	for _, f := range files {
		if !f.Present {
			out = append(out, Step{Kind: StepFile, File: f})
		}
	}
	return out
}

// Plan reads the facts of this machine for the workflow of req: is ComfyUI
// installed, and is each model file present with its size (no SHA-256, so
// the plan is quick). It changes nothing.
func (s *Service) Plan(ctx context.Context, req Request) (Plan, error) {
	pl := Plan{Workflow: req.Workflow}
	m, _, err := catalog.FindModelList(req.WorkflowsDir, req.Workflow, req.ModelsJSON)
	if err != nil {
		return pl, err
	}
	pl.BackendDir, pl.BackendInstalled = s.backend.Installed()
	for _, f := range m.Files {
		st := FileState{Role: f.Role, File: f.Name, Bytes: f.Bytes, Path: s.find(f)}
		if st.Path == "" {
			st.Path = filepath.Join(s.files.Dirs(f.Folder)[0], f.Name)
		} else {
			st.Present = s.files.Size(st.Path) == f.Bytes
		}
		pl.Files = append(pl.Files, st)
	}
	pl.Steps = steps(pl.BackendDir, pl.BackendInstalled, pl.Files)
	return pl, nil
}

// RunPlan runs the steps of pl in order. A backend step installs ComfyUI,
// and its lines go to w.Out. The file steps go to Install, limited to the
// files of the plan. The result is the result of Install, or nil when the
// plan has no file step. A plan with no step changes nothing.
func (s *Service) RunPlan(ctx context.Context, req Request, pl Plan, w Watch) (*Result, error) {
	out := w.Out
	if out == nil {
		out = io.Discard
	}
	var names []string
	for _, st := range pl.Steps {
		switch st.Kind {
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
