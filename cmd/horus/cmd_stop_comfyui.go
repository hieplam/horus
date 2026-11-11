// horus stop-comfyui. Stops the ComfyUI that horus started; the handler is in
// internal/cli, the work in internal/backend.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/hieplam/horus/internal/cli"
)

func init() {
	register("stop-comfyui", "stop the ComfyUI that horus started, Windows only", func(args []string) int {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return cli.StopComfyUI(ctx, args, os.Stdout, os.Stderr, backendAt)
	})
}
