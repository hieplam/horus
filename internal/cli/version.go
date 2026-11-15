// horus version: the version of the program.
package cli

import (
	"fmt"
	"io"

	"github.com/hieplam/horus/internal/version"
)

// Version is horus version. It prints one line: horus <version>.
func Version(args []string, out, errOut io.Writer) int {
	fs := newFlags("horus version", errOut)
	if code, done := parse(fs, args); done {
		return code
	}
	fmt.Fprintf(out, "horus %s\n", version.Version)
	return 0
}
