// horus start-comfyui. Ports scripts/start-comfyui.ps1.
package main

import (
	"os"

	"github.com/hieplam/horus/internal/scripts/startcomfyui"
)

func init() {
	register("start-comfyui", "start ComfyUI and wait for its server, Windows only", func(args []string) int {
		return startcomfyui.Run(args, os.Stdout, os.Stderr)
	})
}
