// The commands of the port map (issue #4) that are not ported yet. Each one
// prints "not implemented yet" to stderr and exits 2.
//
// The task that ports a command deletes its line here and adds the file
// cmd_<name>.go with its own register call.
package main

import (
	"fmt"
	"os"
)

func init() {
	notPorted("workflow", "list, check, install or remove the model files of a workflow (task G4)")
	notPorted("download-model", "download one model file with size and SHA-256 checks (task G3)")
	notPorted("check-setup", "check the setup of this machine (task G7)")
	notPorted("install-comfyui", "install the ComfyUI portable build, Windows only (task G6)")
	notPorted("start-comfyui", "start ComfyUI, Windows only (task G6)")
	notPorted("install-comfy-mcp", "install comfy-mcp and comfy-cli (task G6)")
	notPorted("sync-workflows", "copy the workflows into the ComfyUI page (task G7)")
	notPorted("new-workflow", "make a new workflow folder from the template (task G7)")
	notPorted("register", "register horus mcp with Claude Code (task G7)")
	notPorted("run-illustration", "make one image without MCP (task G7)")
}

func notPorted(name, summary string) {
	register(name, summary, func(args []string) int {
		fmt.Fprintf(os.Stderr, "horus %s: not implemented yet\n", name)
		return 2
	})
}
