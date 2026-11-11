// horus download-model: download one model file with size and SHA-256 checks.
// Ports scripts/download-model.ps1; the work is in internal/download.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/hieplam/horus/internal/download"
)

func init() {
	register("download-model", "download one model file with size and SHA-256 checks", func(args []string) int {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return download.Main(ctx, args, os.Stdout, download.Options{})
	})
}
