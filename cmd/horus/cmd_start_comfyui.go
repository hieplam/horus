// horus start-comfyui. Ports scripts/start-comfyui.ps1; the handler is in
// internal/cli, the work in internal/backend.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/hieplam/horus/internal/cli"
)

func init() {
	register("start-comfyui", "start ComfyUI and wait for its server, Windows only", func(args []string) int {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return cli.StartComfyUI(ctx, args, os.Stdout, os.Stderr, backendAt)
	})
}
