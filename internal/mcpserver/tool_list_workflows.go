// The tool list_workflows.
package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/provision"
)

// WorkflowSummary is one row of list_workflows, in the key order that the tool shows.
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

func listWorkflows(images Images, setup Setup) tool {
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
			flows, err := catalog.All()
			if err != nil {
				return nil, err
			}
			status, err := listStatus(ctx, setup)
			if err != nil {
				return nil, err
			}
			running, err := images.Running(ctx)
			if err != nil {
				return nil, err
			}
			rows := []WorkflowSummary{}
			for _, w := range flows {
				s, err := catalog.SummaryOf(w)
				if err != nil {
					return nil, err
				}
				var row *provision.Row
				if r, ok := status[w.Name]; ok {
					row = &r
				}
				size := int64(0)
				if row != nil {
					size = row.Bytes
				}
				if size == 0 { // no size in the row: use the size in models.json
					lic, err := w.License()
					if err != nil {
						return nil, err
					}
					size = lic.DownloadBytes
				}
				rows = append(rows, WorkflowSummary{
					Name: s.Name, Title: s.Title, Summary: s.Summary, GoodFor: s.GoodFor, NotGoodFor: s.NotGoodFor,
					Output: s.Output, TypicalSeconds: s.TypicalSeconds, InstallState: row.State(), DownloadBytes: size,
				})
			}
			return ListResult{Workflows: rows, BackendRunning: running}, nil
		},
	}
}
