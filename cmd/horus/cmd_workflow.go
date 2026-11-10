// horus workflow: list, check, install or remove the model files of a workflow.
// Ports scripts/workflow.ps1; the work is in internal/scripts/workflow.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/hieplam/horus/internal/scripts/workflow"
)

func init() {
	register("workflow", "list, check, install or remove the model files of a workflow", func(args []string) int {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return workflow.Main(ctx, args, os.Stdout, workflow.Options{})
	})
}
