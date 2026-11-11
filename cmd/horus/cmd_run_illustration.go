// horus run-illustration. Ports scripts/run-illustration.py.
package main

import (
	"os"

	"github.com/hieplam/horus/internal/cli"
)

func init() {
	register("run-illustration", "make one image without MCP", func(args []string) int {
		return cli.RunIllustration(args, os.Stdout, os.Stderr, imagesAt)
	})
}

// imagesAt is the image service for the -url flag of run-illustration.
func imagesAt(url string) cli.Images { return newImages(url) }
