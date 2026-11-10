// horus new-workflow. Ports scripts/new-workflow.ps1.
package main

import (
	"os"

	"github.com/hieplam/horus/internal/scripts/newworkflow"
)

func init() {
	register("new-workflow", "make a new workflow folder from the template", func(args []string) int {
		return newworkflow.Run(args, os.Stdout, os.Stderr)
	})
}
