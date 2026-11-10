// horus install-comfy-mcp. Ports scripts/install-comfy-mcp.ps1.
package main

import (
	"os"

	"github.com/hieplam/horus/internal/scripts/installcomfymcp"
)

func init() {
	register("install-comfy-mcp", "install comfy-mcp and comfy-cli in their own venv", func(args []string) int {
		return installcomfymcp.Run(args, os.Stdout, os.Stderr)
	})
}
