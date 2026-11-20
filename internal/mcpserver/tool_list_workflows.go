// The tool list_workflows.
package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/provision"
)

// WorkflowSummary is one row of list_workflows, in the key order that the tool shows.
type WorkflowSummary struct {
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Title          json.RawMessage `json:"title"`
	Summary        json.RawMessage `json:"summary"`
	GoodFor        json.RawMessage `json:"good_for"`
	NotGoodFor     json.RawMessage `json:"not_good_for"`
	Output         any             `json:"output"`
	TypicalSeconds json.RawMessage `json:"typical_seconds"`
	InstallState   string          `json:"install_state"`
	DownloadBytes  int64           `json:"download_bytes"`
}

// ListResult is the result of list_workflows. Ready and Queue are nil on the
// direct path.
type ListResult struct {
	Workflows      []WorkflowSummary  `json:"workflows"`
	BackendRunning bool               `json:"backend_running"`
	Ready          *jobdata.Readiness `json:"ready"`
	Queue          *QueueState        `json:"queue"`
}

// listWorkflows is the tool. With q (the stack), the result also has the
// readiness of the 4 dependencies and the counts of the queue.
func listWorkflows(images Images, setup Setup, q *Queue) tool {
	return tool{
		name:  "list_workflows",
		title: "List workflows",
		description: "List the workflows on this machine. Each entry has the name to pass to the other tools, its kind " +
			"(image or speech), a summary, what the workflow is good and bad for, the output size of an image workflow, " +
			"the measured time, and install_state. Make an image with generate_image and speech with generate_speech. " +
			"Call this first.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: ptr(false)},
		output: envelope("ListResult", obj{
			"ListResult": model("ListResult",
				f("workflows", tList(ref("WorkflowSummary"))),
				f("backend_running", tBool).d("False: the next generate_image starts the backend first (1 to 3 minutes more)."),
				f("ready", ref("Readiness")).null().d("In the Docker stack: the 4 dependencies of horus and their state. Null: horus runs on this machine without the stack."),
				f("queue", ref("QueueState")).null().d("In the Docker stack: the jobs that wait, the jobs that run, and the slots of the worker. Null: no horus queue."),
			),
			"Readiness": model("Readiness",
				f("ready", tBool).d("True only when the 4 dependencies are up."),
				f("up", tInt).d("The count of the dependencies that are up."),
				f("checks", tList(ref("CheckResult"))),
				f("down", tList(tString)).d("The dependencies that are down."),
			),
			"CheckResult": model("CheckResult",
				f("dependency", tEnum(jobdata.Dependencies...)),
				f("up", tBool),
				f("detail", tString),
			),
			"QueueState": model("QueueState",
				f("waiting", tInt).d("Jobs in the horus queue that wait for a slot."),
				f("running", tInt).d("Jobs that run now."),
				f("slots", tInt).d("The jobs that the worker runs at the same time (HORUS_WORKER_CONCURRENCY)."),
			),
			"WorkflowSummary": model("WorkflowSummary",
				f("name", tString).d("Pass this to the other tools."),
				f("kind", tKind).d("What the workflow makes. image: use generate_image. speech: use generate_speech."),
				f("title", tString),
				f("summary", tString),
				f("good_for", tList(tString)),
				f("not_good_for", tList(tString)),
				f("output", tDict).null().d("Image workflow: format, default_width, default_height. Speech workflow: null."),
				f("typical_seconds", tList(tInt)).null().d("Measured time for one image or one file, [fastest, slowest]."),
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
					Name: s.Name, Kind: s.Kind, Title: s.Title, Summary: s.Summary, GoodFor: s.GoodFor, NotGoodFor: s.NotGoodFor,
					Output: s.Output, TypicalSeconds: s.TypicalSeconds, InstallState: row.State(), DownloadBytes: size,
				})
			}
			res := ListResult{Workflows: rows, BackendRunning: running}
			if q != nil {
				res.Ready = c.ready
				if res.Queue, err = q.state(ctx); err != nil {
					return nil, err
				}
			}
			return res, nil
		},
	}
}
