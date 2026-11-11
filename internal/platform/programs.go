// Programs: find a program on the PATH and run it to its end.
package platform

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
)

// Command is one run of a program.
type Command struct {
	Path string   // the program: a path, or a name on the PATH
	Args []string // the arguments after the program
	Dir  string   // the working folder; "" = the folder of this process
	// Env is added to the environment of this process, as "NAME=value".
	Env []string
	// Stdout and Stderr get the output of the program. nil drops it.
	Stdout, Stderr io.Writer
}

// Programs finds and runs the programs of this machine. Its zero value is
// ready to use. A service declares the methods that it needs, and a test
// gives the service a fake.
type Programs struct{}

// LookPath gives the path of the program name on the PATH.
func (Programs) LookPath(name string) (string, error) { return exec.LookPath(name) }

// Run runs the program and waits for its end. It returns the exit code of
// the program. The error is not nil only when the program did not run to its
// end for another reason: it was not found, it did not start, or the wait
// failed. The end of ctx ends the program.
func (Programs) Run(ctx context.Context, c Command) (int, error) {
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.Dir = c.Dir
	if len(c.Env) > 0 {
		cmd.Env = append(os.Environ(), c.Env...)
	}
	cmd.Stdout, cmd.Stderr = c.Stdout, c.Stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}
