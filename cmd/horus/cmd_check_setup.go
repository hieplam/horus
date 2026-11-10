// horus check-setup. Ports scripts/check-setup.ps1.
package main

import (
	"os"

	"github.com/hieplam/horus/internal/scripts/checksetup"
)

func init() {
	register("check-setup", "one PASS or FAIL line for each check of the setup", func(args []string) int {
		return checksetup.Run(args, os.Stdout, os.Stderr)
	})
}
