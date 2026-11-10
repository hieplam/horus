// The tool uninstall_workflow. Part of the port of horus/server.py.
package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
)

// UninstallResult is the result of uninstall_workflow, in the key order of server.py.
type UninstallResult struct {
	Workflow   string        `json:"workflow"`
	Files      []InstallFile `json:"files"`
	FreedBytes int64         `json:"freed_bytes"`
}

func uninstallWorkflow() tool {
	return tool{
		name:  "uninstall_workflow",
		title: "Uninstall an image workflow",
		description: "Delete the model files of one image workflow to free disk space. A file that another installed workflow also " +
			"uses is kept unless remove_shared=true. Without confirm=true nothing is deleted: the call returns the error " +
			"CONFIRMATION_REQUIRED with the list of files and sizes. Show it to the user, then call again with confirm=true.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptr(true), IdempotentHint: true, OpenWorldHint: ptr(false)},
		args: []field{
			f("workflow", tString).d("Name from list_workflows."),
			f("confirm", tBool).d("True only after the user agreed to the plan from the first reply.").dflt(false),
			f("remove_shared", tBool).d("Also delete files that other workflows use.").dflt(false),
		},
		output: envelope("UninstallResult", obj{
			"UninstallResult": model("UninstallResult",
				f("workflow", tString),
				f("files", tList(ref("InstallFile"))),
				f("freed_bytes", tInt),
			),
			"InstallFile": installFileModel,
		}),
		body: func(ctx context.Context, c *call) (any, error) {
			if _, err := catalog.Get(c.args.str("workflow")); err != nil {
				return nil, err
			}
			// Next: Comfy().free (task G2) and models.remove (task G5).
			return nil, notPorted("uninstall_workflow", "the removal of the model files", "G5")
		},
	}
}
