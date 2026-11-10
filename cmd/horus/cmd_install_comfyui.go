// horus install-comfyui. Ports scripts/install-comfyui.ps1.
package main

import (
	"os"

	"github.com/hieplam/horus/internal/scripts/installcomfyui"
)

func init() {
	register("install-comfyui", "install the ComfyUI portable build, Windows only", func(args []string) int {
		return installcomfyui.Run(args, os.Stdout, os.Stderr)
	})
}
