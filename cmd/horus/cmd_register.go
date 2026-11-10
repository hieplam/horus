// horus register. Ports the -Register part of scripts/install-horus-mcp.ps1.
package main

import (
	"os"

	registercmd "github.com/hieplam/horus/internal/scripts/register"
)

func init() {
	register("register", "register horus mcp with Claude Code", func(args []string) int {
		return registercmd.Run(args, os.Stdout, os.Stderr)
	})
}
