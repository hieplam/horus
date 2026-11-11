// horus workflow: list, check, install or remove the model files of a workflow.
// Ports scripts/workflow.ps1; the handler is in internal/cli, the work in
// internal/provision.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/hieplam/horus/internal/cli"
)

func init() {
	register("workflow", "list, check, install or remove the model files of a workflow", func(args []string) int {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return cli.Workflow(ctx, args, os.Stdout, os.Stderr, setupAt)
	})
}

// setupAt is the setup service for the flags of horus workflow.
func setupAt(place cli.SetupPlace) cli.Setup { return newSetup(place, seams{}) }
