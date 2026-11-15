package mcpserver

import (
	"os"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

// workflowsDirError is BACKEND_NOT_INSTALLED when the workflows folder is
// missing, and nil when it is there. Without HORUS_WORKFLOWS_DIR and
// HORUS_ROOT the folder is the copy of the built-in workflows, so this error
// comes only from a setting that names a folder that does not exist. Issue
// #40: the next_step names the setting to fix, not INTERNAL ("a bug in
// horus").
func workflowsDirError() *herr.Error {
	dir := settings.WorkflowsDir()
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		return nil
	}
	e := herr.New("BACKEND_NOT_INSTALLED", "No workflows folder: "+dir, map[string]any{
		"workflows_dir": dir,
		"horus_root":    os.Getenv("HORUS_ROOT"),
	})
	e.NextStep = rootNextStep(dir)
	return e
}

// rootNextStep names the setting that points to the folder dir that does
// not exist, and where the setting is.
func rootNextStep(dir string) string {
	setting := "HORUS_ROOT"
	if settings.Get("HORUS_WORKFLOWS_DIR", "") != "" {
		setting = "HORUS_WORKFLOWS_DIR"
	}
	return "The setting " + setting + " names a folder without workflows (" + dir + "). " +
		"Ask the user to fix or remove HORUS_WORKFLOWS_DIR and HORUS_ROOT in the MCP server config or in " +
		settings.EnvPath() + ". Without both, horus uses its built-in workflows. " +
		"To use a horus repo, run horus register in the repo. Then restart the agent."
}
