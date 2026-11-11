// The tool uninstall_workflow. Part of the port of horus/server.py.
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/comfy"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/models"
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
			w, err := catalog.Get(c.args.str("workflow"))
			if err != nil {
				return nil, err
			}
			confirm := c.args.boolean("confirm")
			if confirm {
				comfy.New("").Free(ctx) // an open model file cannot be deleted on Windows
			}
			res, err := models.Remove(ctx, w, confirm, c.args.boolean("remove_shared"))
			if err != nil {
				return nil, err
			}
			files := fileRows(res.Items, "file", map[string]string{
				"DELETED": "deleted", "WOULD_DELETE": "would_delete", "KEPT_SHARED": "kept_shared", "ABSENT": "absent"})
			if !confirm {
				would := int64(0)
				plan := false
				for _, f := range files {
					if f.State == "would_delete" {
						plan = true
						would += f.Bytes
					}
				}
				if plan {
					return nil, herr.New("CONFIRMATION_REQUIRED", "nothing was deleted: the user must confirm the plan",
						map[string]any{"files": files, "would_free_bytes": would})
				}
			}
			freed := int64(0)
			for _, f := range files {
				if f.State == "deleted" {
					freed += f.Bytes
				}
			}
			return UninstallResult{Workflow: w.Name, Files: files, FreedBytes: freed}, nil
		},
	}
}
