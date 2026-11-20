// The tool generate_image.
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/generate"
)

// ImageResult is the result of generate_image, in the key order that the tool shows.
type ImageResult = generate.Result

// generateImage is the tool. With q (the stack), the call goes through the
// job queue (queue.go). Without q, the direct path calls the image service.
func generateImage(images Images, q *Queue) tool {
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
			f("idempotency_key", tString).null().d("Optional key of this request, for a later call again with the same request. " +
				"horus keeps it with the job in the stack, and makes a new image for each call.").dflt(nil),
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
				f("job_id", tString).d("The id of the job: the horus job id in the stack, else the id of the job in the backend."),
				f("backend_job_id", tString).d("The id of the job in the backend (ComfyUI)."),
				f("queue_seconds", tNumber).null().d("Time the job waited in the horus queue before its run. Null: no horus queue."),
			),
		}),
		body: func(ctx context.Context, c *call) (any, error) {
			if q != nil {
				return q.generate(ctx, images, c)
			}
			res, err := images.Generate(ctx, generate.Request{
				Workflow:  c.args.str("workflow"),
				Subject:   c.args.str("subject"),
				Seed:      c.args["seed"],
				Width:     c.args["width"],
				Height:    c.args["height"],
				OutPath:   c.args.strPtr("out_path"),
				Overwrite: c.args.boolean("overwrite"),
			}, generate.Options{Report: generate.Report(c.report)})
			if err != nil {
				return nil, err
			}
			return res, nil
		},
	}
}
