// The tool generate_image. Part of the port of horus/server.py.
package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ImageResult is the result of generate_image, in the key order of server.py.
type ImageResult struct {
	Workflow       string   `json:"workflow"`
	Path           string   `json:"path"`
	Bytes          int64    `json:"bytes"`
	Width          int64    `json:"width"`
	Height         int64    `json:"height"`
	Seed           int64    `json:"seed"`
	Seconds        float64  `json:"seconds"`
	BackendSeconds *float64 `json:"backend_seconds"`
	StartedBackend bool     `json:"started_backend"`
	JobID          string   `json:"job_id"`
}

func generateImage() tool {
	return tool{
		name:  "generate_image",
		title: "Generate an image",
		description: "Make one image with the given workflow and save it as a PNG on this machine. The call waits until the image is " +
			"done (often 3 to 8 minutes; the first call after a cold start takes longer) and reports progress. " +
			"The subject is the whole prompt: what the picture shows and how it looks (style, palette, light, " +
			"composition). horus sends it to the model exactly as written and adds nothing. The result has the absolute path of the PNG; open " +
			"it and check it. The same subject and seed give the same image.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptr(false), IdempotentHint: false, OpenWorldHint: ptr(false)},
		args: []field{
			f("workflow", tString).d("Name from list_workflows, for example 'it-illustration'."),
			f("subject", tString).d("The whole prompt, sent to the model as written: content and style. Follow subject.rules of describe_workflow."),
			f("out_path", tString).null().d("Absolute path of the PNG to write. Default: a new file in the horus output folder.").dflt(nil),
			f("seed", tInt).null().d("Integer for a repeatable image. Default: a random seed (returned in the result).").dflt(nil),
			f("width", tInt).null().d("Pixels. Default and limits: describe_workflow output.").dflt(nil),
			f("height", tInt).null().d("Pixels. Default and limits: describe_workflow output.").dflt(nil),
			f("overwrite", tBool).d("Replace out_path when it exists.").dflt(false),
		},
		output: envelope("ImageResult", obj{
			"ImageResult": model("ImageResult",
				f("workflow", tString),
				f("path", tString).d("Absolute path of the PNG on this machine."),
				f("bytes", tInt),
				f("width", tInt),
				f("height", tInt),
				f("seed", tInt).d("Pass the same seed and subject again to get the same image."),
				f("seconds", tNumber).d("Wall time of this call."),
				f("backend_seconds", tNumber).null().d("Time the GPU spent on the job."),
				f("started_backend", tBool).d("True: this call had to start the backend first."),
				f("job_id", tString),
			),
		}),
		body: func(ctx context.Context, c *call) (any, error) {
			// Next: generate.generate_image (task G2), with c.report for progress.
			return nil, notPorted("generate_image", "the image job", "G2")
		},
	}
}
