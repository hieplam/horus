// The tool install_workflow. Part of the port of horus/server.py.
package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
)

// InstallResult is the result of install_workflow, in the key order of server.py.
type InstallResult struct {
	Workflow         string        `json:"workflow"`
	InstallState     string        `json:"install_state"`
	AlreadyInstalled bool          `json:"already_installed"`
	Files            []InstallFile `json:"files"`
	Seconds          float64       `json:"seconds"`
}

func installWorkflow() tool {
	return tool{
		name:  "install_workflow",
		title: "Install an image workflow",
		description: "Download and verify (SHA-256) the model files of one image workflow, so that generate_image can use it. " +
			"Files that are already present are kept. Downloads are large (several GB) and can take 10 to 60 minutes. " +
			"The models have their own license: the first call without accept_license=true returns the error " +
			"LICENSE_CONFIRMATION_REQUIRED with the license and the size. Ask the user, then call again with " +
			"accept_license=true. The call waits until the install is done and reports progress.",
		annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: ptr(false), IdempotentHint: true, OpenWorldHint: ptr(true)},
		args: []field{
			f("workflow", tString).d("Name from list_workflows."),
			f("accept_license", tBool).d("True only after the user accepted the license shown in the first reply.").dflt(false),
		},
		output: envelope("InstallResult", obj{
			"InstallResult": model("InstallResult",
				f("workflow", tString),
				f("install_state", tConst("installed")),
				f("already_installed", tBool),
				f("files", tList(ref("InstallFile"))),
				f("seconds", tNumber),
			),
			"InstallFile": installFileModel,
		}),
		body: func(ctx context.Context, c *call) (any, error) {
			if _, err := catalog.Get(c.args.str("workflow")); err != nil {
				return nil, err
			}
			// Next: models.list_status, models.check and models.install (task G5).
			return nil, notPorted("install_workflow", "the install of the model files", "G5")
		},
	}
}
