// The tool list_workflows. Part of the port of horus/server.py.
package server

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
)

// WorkflowSummary is one row of list_workflows, in the key order of server.py.
type WorkflowSummary struct {
	Name           string          `json:"name"`
	Title          json.RawMessage `json:"title"`
	Summary        json.RawMessage `json:"summary"`
	GoodFor        json.RawMessage `json:"good_for"`
	NotGoodFor     json.RawMessage `json:"not_good_for"`
	Output         any             `json:"output"`
	TypicalSeconds json.RawMessage `json:"typical_seconds"`
	InstallState   string          `json:"install_state"`
	DownloadBytes  int64           `json:"download_bytes"`
}

// ListResult is the result of list_workflows.
type ListResult struct {
	Workflows      []WorkflowSummary `json:"workflows"`
	BackendRunning bool              `json:"backend_running"`
}

func listWorkflows() tool {
	return tool{
		name:  "list_workflows",
		title: "List image workflows",
		description: "List the image workflows (model + graph) on this machine. Each entry has the name to pass " +
			"to the other tools, a summary, what the workflow is good and bad for, the output size, the measured " +
			"time per image, and install_state. Call this first.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: ptr(false)},
		output: envelope("ListResult", obj{
			"ListResult": model("ListResult",
				f("workflows", tList(ref("WorkflowSummary"))),
				f("backend_running", tBool).d("False: the next generate_image starts the backend first (1 to 3 minutes more)."),
			),
			"WorkflowSummary": model("WorkflowSummary",
				f("name", tString).d("Pass this to the other tools."),
				f("title", tString),
				f("summary", tString),
				f("good_for", tList(tString)),
				f("not_good_for", tList(tString)),
				f("output", tDict).d("format, default_width, default_height"),
				f("typical_seconds", tList(tInt)).null().d("Measured time for one image, [fastest, slowest]."),
				f("install_state", tInstallState),
				f("download_bytes", tInt).d("Total size of the model files."),
			),
		}),
		body: func(ctx context.Context, c *call) (any, error) {
			if _, err := catalog.All(); err != nil {
				return nil, err
			}
			// Next: models.list_status (task G5) and Comfy().is_up (task G2).
			return nil, notPorted("list_workflows", "the install state of each workflow", "G5")
		},
	}
}
