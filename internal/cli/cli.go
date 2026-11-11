// Package cli holds the handlers of the horus commands: one file for each
// command, flags in, text out. A handler imports services and base packages
// only. cmd/horus gives it the services.
package cli

import (
	"flag"
	"io"
)

// newFlags makes the flag set of one command. Its errors and its usage go
// to errOut.
func newFlags(name string, errOut io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(errOut)
	return fs
}
