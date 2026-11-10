// The commands of the port map (issue #4) that are not ported yet. Each one
// prints "not implemented yet" to stderr and exits 2.
//
// The task that ports a command deletes its line here and adds the file
// cmd_<name>.go with its own register call.
package main

import (
	"fmt"
	"os"
)

func init() {
	notPorted("workflow", "list, check, install or remove the model files of a workflow (task G4)")
}

func notPorted(name, summary string) {
	register(name, summary, func(args []string) int {
		fmt.Fprintf(os.Stderr, "horus %s: not implemented yet\n", name)
		return 2
	})
}
