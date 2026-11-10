// horus sync-workflows. Ports scripts/sync-workflows.ps1.
package main

import (
	"os"

	"github.com/hieplam/horus/internal/scripts/syncworkflows"
)

func init() {
	register("sync-workflows", "copy the workflows into the ComfyUI page", func(args []string) int {
		return syncworkflows.Run(args, os.Stdout, os.Stderr)
	})
}
