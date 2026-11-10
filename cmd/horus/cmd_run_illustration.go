// horus run-illustration. Ports scripts/run-illustration.py.
package main

import (
	"os"

	"github.com/hieplam/horus/internal/scripts/runillustration"
)

func init() {
	register("run-illustration", "make one image without MCP", func(args []string) int {
		return runillustration.Run(args, os.Stdout, os.Stderr)
	})
}
