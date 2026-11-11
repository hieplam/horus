// horus install-comfyui. Ports scripts/install-comfyui.ps1; the handler is in
// internal/cli, the work in internal/backend.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/hieplam/horus/internal/cli"
)

func init() {
	register("install-comfyui", "install the ComfyUI portable build, Windows only", func(args []string) int {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return cli.InstallComfyUI(ctx, args, os.Stdout, os.Stderr, backendAt)
	})
}

// backendAt is the backend for the flags of install-comfyui, start-comfyui
// and stop-comfyui.
func backendAt(p cli.BackendPlace) cli.Backend { return newBackend(p) }
