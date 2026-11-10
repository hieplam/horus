// horus errors: the table of error codes. Ports the table ERRORS of
// horus/errors.py, for a reader that is not Go (the contract test after G10).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hieplam/horus/internal/herr"
)

func init() { register("errors", "print the table of error codes (-json for JSON)", runErrors) }

func runErrors(args []string) int {
	fs := flag.NewFlagSet("horus errors", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print the table as one JSON object: code -> retryable, action_by, next_step")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *asJSON {
		os.Stdout.Write(herr.TableJSON())
		return 0
	}
	for _, s := range herr.Codes {
		retry := "no"
		if s.Retryable {
			retry = "yes"
		}
		fmt.Printf("%-30s retryable=%-3s action_by=%-6s %s\n", s.Code, retry, s.ActionBy, s.NextStep)
	}
	return 0
}
