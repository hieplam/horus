// horus new-workflow and horus sync-workflows: the handlers of the tools for
// a workflow author. They port the flags and the output of
// scripts/new-workflow.ps1 and scripts/sync-workflows.ps1. The work is in the
// service internal/devtools.
package cli

import (
	"fmt"
	"io"

	"github.com/hieplam/horus/internal/devtools"
	"github.com/hieplam/horus/internal/settings"
)

// NewWorkflow is horus new-workflow. Exit code 0 on success, 1 when the name
// is not kebab-case, the template is missing or the folder exists, 2 for a
// flag.
func NewWorkflow(args []string, out, errOut io.Writer) int {
	fs := newFlags("horus new-workflow", errOut)
	name := fs.String("name", "", "folder name of the new workflow, in kebab-case (required)")
	description := fs.String("description", "", "one line that says what the workflow makes (required)")
	family := fs.String("model-family", "TBD", "the model family, for example \"FLUX 3 Image Klein\"")
	wfDir := fs.String("workflows-dir", "", "the workflows folder (default: HORUS_WORKFLOWS_DIR, then <repo>/workflows)")
	if code, done := parse(fs, args); done {
		return code
	}
	if *name == "" || *description == "" {
		fmt.Fprintln(errOut, "horus new-workflow: -name and -description are required")
		return 2
	}
	dir := *wfDir
	if dir == "" {
		dir = settings.WorkflowsDir()
	}
	readme, err := devtools.NewWorkflow(dir, *name, *description, *family)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	fmt.Fprintf(out, "Created %s\n", readme)
	fmt.Fprintln(out, "Next: add workflow.json through the ComfyUI UI export or a template. Do not hand-edit the graph.")
	return 0
}

// SyncWorkflows is horus sync-workflows. installAt gives the ComfyUI install
// of the flag -comfyui-dir ("" = the setting). Exit code 0 when every copy is
// the repo copy, 1 for a conflict or an error, 2 for a flag.
func SyncWorkflows(args []string, out, errOut io.Writer, installAt func(comfyUIDir string) devtools.Install) int {
	fs := newFlags("horus sync-workflows", errOut)
	dir := fs.String("comfyui-dir", "", "the portable folder (default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)")
	force := fs.Bool("force", false, "replace a pane copy that differs from the repo copy; the old copy is kept as .bak")
	wfDir := fs.String("workflows-dir", "", "the workflows folder (default: HORUS_WORKFLOWS_DIR, then <repo>/workflows)")
	if code, done := parse(fs, args); done {
		return code
	}
	workflows := *wfDir
	if workflows == "" {
		workflows = settings.WorkflowsDir()
	}
	copies, err := devtools.Sync(installAt(*dir), workflows, *force)
	conflicts := 0
	for _, c := range copies {
		switch c.State {
		case devtools.Copied:
			fmt.Fprintf(out, "copied   %s\n", c.File)
		case devtools.Same:
			fmt.Fprintf(out, "ok       %s\n", c.File)
		case devtools.Replaced:
			fmt.Fprintf(out, "replaced %s (old copy kept as .bak)\n", c.File)
		default:
			conflicts++
			fmt.Fprintf(out, "CONFLICT %s differs from %s. Compare both, then run again with -force to use the repo copy.\n", c.File, c.Source)
		}
	}
	if err != nil {
		fmt.Fprintln(errOut, "horus sync-workflows:", err)
		return 1
	}
	if len(copies) == 0 {
		fmt.Fprintln(out, "No workflows/*/workflow.json files found. Nothing to sync.")
	}
	if conflicts > 0 {
		return 1
	}
	return 0
}
