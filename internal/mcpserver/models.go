// The models folder of the stack (spec docs/specs/2026-11-19-job-queue.md,
// "Settings" and "MCP contract"; ADR 0029 and ADR 0041). In the stack,
// install_workflow downloads only model files, into HORUS_MODELS_DIR:
// ComfyUI runs on the host, and the owner sets it up. horus stack-mounts
// mounts the host folder of HORUS_MODELS_DIR into horus-api. The tools
// list_workflows, describe_workflow, install_workflow and uninstall_workflow
// read and change the model files only there, and show their host paths.
package mcpserver

import (
	"fmt"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// Models is the models folder of the stack.
type Models struct {
	// Dir is HORUS_MODELS_DIR in horus-api: the container folder of the
	// model files. "": the stack has no models folder.
	Dir   string
	paths Paths // the pair of the host folder and Dir
}

// ModelsOf gives the models folder of the stack: dir is HORUS_MODELS_DIR in
// the container, host is its host folder (HORUS_MODELS_HOST_DIR), and hostOS
// is the system of the host (HORUS_HOST_OS). Without host, the tools show the
// container paths.
func ModelsOf(dir, host, hostOS string) *Models {
	m := &Models{Dir: dir}
	if dir != "" && host != "" {
		m.paths = PathsOf([]settings.SharedDir{{Host: host, Container: dir}}, hostOS)
	}
	return m
}

// hostPath gives the host path of the container path p, or p when p is not
// in the models folder. On the direct path (m nil), it gives p.
func (m *Models) hostPath(p string) string {
	if m == nil || p == "" {
		return p
	}
	if h, ok := m.paths.ToHost(p); ok {
		return h
	}
	return p
}

// hostItems changes the path of each item of res to its host path.
func (m *Models) hostItems(res *provision.Result) {
	if m == nil || res == nil {
		return
	}
	for i := range res.Items {
		res.Items[i].Path = m.hostPath(res.Items[i].Path)
	}
}

// hostPlan gives a copy of pl with the host path of each model file, for the
// answers of a tool. The copy is never run.
func (m *Models) hostPlan(pl provision.Plan) provision.Plan {
	if m == nil {
		return pl
	}
	out := pl
	out.Files = append([]provision.FileState(nil), pl.Files...)
	for i := range out.Files {
		out.Files[i].Path = m.hostPath(out.Files[i].Path)
	}
	out.Steps = append([]provision.Step(nil), pl.Steps...)
	for i := range out.Steps {
		out.Steps[i].File.Path = m.hostPath(out.Steps[i].File.Path)
	}
	return out
}

// refusal gives the error of the tool (install_workflow or
// uninstall_workflow) for the workflow w in the stack, or nil. A speech
// workflow does not run in the stack (ADR 0039), and the stack changes model
// files only in its models folder. On the direct path (m nil), it is nil.
func (m *Models) refusal(tool string, w *catalog.Workflow) error {
	if m == nil {
		return nil
	}
	if w.Kind() == catalog.KindSpeech {
		e := herr.New("BACKEND_UNAVAILABLE",
			fmt.Sprintf("horus in the Docker stack runs image workflows only: %s of the speech workflow %s does not run in the stack", tool, w.Name),
			map[string]any{"tool": tool, "workflow": w.Name, "stack": true})
		e.Retryable = ptr(false)
		e.NextStep = "Tell the user that horus in the Docker stack cannot set up speech. For speech, the user registers the horus " +
			"program of this machine (horus register) and sets up the speech workflow there (horus setup <workflow> -yes)."
		return e
	}
	if m.Dir == "" {
		e := herr.New("BACKEND_NOT_INSTALLED",
			"horus in the stack has no models folder: HORUS_MODELS_DIR is not set in horus-api, so "+tool+" cannot see the model files",
			map[string]any{"setting": "HORUS_MODELS_DIR", "tool": tool})
		e.NextStep = "Ask the user to set HORUS_MODELS_DIR in the .env of the stack to the host folder of the model files that ComfyUI " +
			"reads, run horus stack-mounts in the horus clone, then docker compose up -d (docs/setup.md, section Run horus as a stack). " +
			"Then call again."
		return e
	}
	return nil
}

// setupInStack is the error of install_workflow in the stack when the plan
// holds a step that is not a model file. horus in the stack never installs
// ComfyUI or a system package: the setup service of the stack gives no such
// step, so this is a bug.
func setupInStack(pl provision.Plan) error {
	var names []string
	for _, st := range pl.Steps {
		if st.Kind != provision.StepFile {
			names = append(names, st.Kind+" "+st.Name)
		}
	}
	return herr.New("INTERNAL", fmt.Sprintf("the plan of the stack holds %d setup step(s): %v; horus in the stack installs only model files", len(names), names),
		map[string]any{"steps": stepRows(pl)})
}
