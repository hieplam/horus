// The tool describe_workflow.
package mcpserver

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/provision"
)

// DescribeResult is the result of describe_workflow, in the key order that the tool shows.
type DescribeResult struct {
	Name       string          `json:"name"`
	Kind       string          `json:"kind"`
	Title      json.RawMessage `json:"title"`
	Summary    json.RawMessage `json:"summary"`
	GoodFor    json.RawMessage `json:"good_for"`
	NotGoodFor json.RawMessage `json:"not_good_for"`
	Output     any             `json:"output"`
	Subject    *catalog.Object `json:"subject"`
	// The fields of a speech workflow. An image workflow has none of them.
	Text           *catalog.Object `json:"text,omitempty"`
	Voices         json.RawMessage `json:"voices,omitempty"`
	DefaultVoice   json.RawMessage `json:"default_voice,omitempty"`
	Tags           json.RawMessage `json:"tags,omitempty"`
	Formats        json.RawMessage `json:"formats,omitempty"`
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
	r := DescribeResult{
		Name: d.Name, Kind: d.Kind, Title: d.Title, Summary: d.Summary.Summary, GoodFor: d.GoodFor, NotGoodFor: d.NotGoodFor,
		Output: d.Output, Subject: d.Subject, Timing: d.Timing, TypicalSeconds: d.TypicalSeconds,
		License: d.License, InstallState: state, Files: files,
	}
	if sp := d.Speech; sp != nil {
		r.Text, r.Voices, r.DefaultVoice, r.Tags, r.Formats = sp.Text, sp.Voices, sp.DefaultVoice, sp.Tags, sp.Formats
	}
	return r
}

// fileStates gives the state of each model file, and the install state of the
// workflow. A failed check (a missing file) is a normal answer here, so it
// reads the items and not the ok of the result. The install state counts each
// part that the check lists: a model file, and for a speech workflow also the
// speech venv, ffmpeg and ffprobe.
// In the stack (m), each path is the host path.
func fileStates(ctx context.Context, setup Setup, w *catalog.Workflow, m *Models) ([]InstallFile, string) {
	req := request(w)
	req.Quick = true
	res, _ := setup.Check(context.WithoutCancel(ctx), req)
	m.hostItems(res)
	parts, present := 0, 0
	for _, it := range res.Items {
		if it.Kind == "file" || it.Kind == "venv" || it.Kind == "program" {
			parts++
			if it.Status == provision.StatusOK {
				present++
			}
		}
	}
	files := fileRows(res.Items, "file", map[string]string{"OK": "present", "MISSING": "missing", "BAD_SIZE": "wrong_size"})
	return files, provision.InstallState(present, parts)
}

func describeWorkflow(setup Setup, m *Models) tool {
	return tool{
		name:  "describe_workflow",
		title: "Describe a workflow",
		description: "Everything a caller needs to use one workflow. Its kind is image or speech. An image workflow shows " +
			"the allowed sizes and how to write the subject (rules and example subjects with their images). A speech " +
			"workflow shows the limits of the text, the voices, the default voice, the tags that the model performs and " +
			"the formats. Both show the license and the install state of each model file. The workflow has no style of " +
			"its own. Read subject.rules before the first generate_image with an image workflow.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: ptr(false)},
		args: []field{
			f("workflow", tString).d("Name from list_workflows, for example 'it-illustration'."),
		},
		output: envelope("DescribeResult", obj{
			"DescribeResult": model("DescribeResult",
				f("name", tString),
				f("kind", tKind).d("What the workflow makes. image: use generate_image. speech: use generate_speech."),
				f("title", tString),
				f("summary", tString),
				f("good_for", tList(tString)),
				f("not_good_for", tList(tString)),
				f("output", tDict).null().d("Image workflow: format, default size, min_side, max_side, multiple_of. Speech workflow: null."),
				f("subject", tDict).null().d("Image workflow: min_chars, max_chars, how_to_write, rules, examples (subject, image path, seed). Speech workflow: null."),
				f("text", tDict).d("Speech workflow only: min_chars, max_chars and the rules of the text.").factory(),
				f("voices", tList(tString)).d("Speech workflow only: the voice names that generate_speech accepts.").factory(),
				f("default_voice", tString).d("Speech workflow only: the voice that generate_speech uses when it gets none.").factory(),
				f("tags", tList(tString)).d("Speech workflow only: the [tag] markers that the model performs. Another [tag] in the text is refused.").factory(),
				f("formats", tList(tString)).d("Speech workflow only: the audio formats of the result.").factory(),
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
			files, state := fileStates(ctx, setup, w, m)
			d, err := catalog.DetailOf(w)
			if err != nil {
				return nil, err
			}
			return describeResult(d, state, files), nil
		},
	}
}
