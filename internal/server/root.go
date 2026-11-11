package server

import (
	"os"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

// rootNextStep names the setting to fix when the server cannot find the
// workflows folder. Issue #40: without it, a server that starts outside the
// repo with no HORUS_ROOT answered INTERNAL ("a bug in horus").
const rootNextStep = "horus cannot find the horus repo. Ask the user to run horus register in the horus repo, " +
	"or to set HORUS_ROOT to the repo folder (the folder with workflows/) in the MCP server config. Then restart the agent."

// workflowsDirError is BACKEND_NOT_INSTALLED when the workflows folder is
// missing, and nil when it is there.
func workflowsDirError() *herr.Error {
	dir := settings.WorkflowsDir()
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		return nil
	}
	e := herr.New("BACKEND_NOT_INSTALLED", "No workflows folder: "+dir, map[string]any{
		"workflows_dir": dir,
		"horus_root":    os.Getenv("HORUS_ROOT"),
	})
	e.NextStep = rootNextStep
	return e
}
