// The tool describe_workflow. Part of the port of horus/server.py.
package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/models"
)

// DescribeResult is the result of describe_workflow, in the key order of server.py.
type DescribeResult struct {
	Name           string          `json:"name"`
	Title          json.RawMessage `json:"title"`
	Summary        json.RawMessage `json:"summary"`
	GoodFor        json.RawMessage `json:"good_for"`
	NotGoodFor     json.RawMessage `json:"not_good_for"`
	Output         any             `json:"output"`
	Subject        *catalog.Object `json:"subject"`
	Timing         json.RawMessage `json:"timing"`
	TypicalSeconds json.RawMessage `json:"typical_seconds"`
	License        catalog.License `json:"license"`
	InstallState   string          `json:"install_state"`
	Files          []InstallFile   `json:"files"`
}

func describeResult(d catalog.Detail, state string, files []InstallFile) DescribeResult {
	if files == nil {
		files = []InstallFile{}
	}
	return DescribeResult{
		Name: d.Name, Title: d.Title, Summary: d.Summary.Summary, GoodFor: d.GoodFor, NotGoodFor: d.NotGoodFor,
		Output: d.Output, Subject: d.Subject, Timing: d.Timing, TypicalSeconds: d.TypicalSeconds,
		License: d.License, InstallState: state, Files: files,
	}
}

// fileStates is _status_rows of server.py: the state of each model file. A
// failed check (a missing file) is a normal answer here, so it reads the
// items and not the ok of the result.
func fileStates(ctx context.Context, w *catalog.Workflow) ([]InstallFile, error) {
	run, err := models.RunScript(context.WithoutCancel(ctx), "check", w.Name, []string{"-quick"}, nil, nil)
	if err != nil {
		return nil, err
	}
	var items []models.Item
	if res := run.Result(); res != nil {
		items = res.Items
	}
	return fileRows(items, "file", map[string]string{"OK": "present", "MISSING": "missing", "BAD_SIZE": "wrong_size"}), nil
}

func describeWorkflow() tool {
	return tool{
		name:  "describe_workflow",
		title: "Describe an image workflow",
		description: "Everything a caller needs to use one image workflow: the allowed sizes, how to write the subject (rules " +
			"and example subjects with their images), the license and the install state of each model file. The workflow " +
			"has no style of its own. Read subject.rules before the first generate_image with this workflow.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: ptr(false)},
		args: []field{
			f("workflow", tString).d("Name from list_workflows, for example 'it-illustration'."),
		},
		output: envelope("DescribeResult", obj{
			"DescribeResult": model("DescribeResult",
				f("name", tString),
				f("title", tString),
				f("summary", tString),
				f("good_for", tList(tString)),
				f("not_good_for", tList(tString)),
				f("output", tDict).d("format, default size, min_side, max_side, multiple_of"),
				f("subject", tDict).d("min_chars, max_chars, how_to_write, rules, examples (subject, image path, seed)"),
				f("timing", tDict).null(),
				f("typical_seconds", tList(tInt)).null(),
				f("license", tDict).d("name, url, download_bytes"),
				f("install_state", tInstallState),
				f("files", tList(ref("InstallFile"))),
			),
			"InstallFile": installFileModel,
		}),
		body: func(ctx context.Context, c *call) (any, error) {
			w, err := catalog.Get(c.args.str("workflow"))
			if err != nil {
				return nil, err
			}
			files, err := fileStates(ctx, w)
			if err != nil {
				return nil, err
			}
			present := 0
			for _, x := range files {
				if x.State == "present" {
					present++
				}
			}
			state := "not_installed"
			if len(files) > 0 && present == len(files) {
				state = "installed"
			} else if present > 0 {
				state = "partially_installed"
			}
			d, err := catalog.DetailOf(w)
			if err != nil {
				return nil, err
			}
			return describeResult(d, state, files), nil
		},
	}
}
