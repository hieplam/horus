package platform

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunGivesTheExitCodeAndTheOutput runs the test binary in the role
// "run" (see TestMain): it prints its arguments and one variable, prints its
// folder to stderr, and exits 5.
func TestRunGivesTheExitCodeAndTheOutput(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code, err := Programs{}.Run(context.Background(), Command{Path: os.Args[0], Args: []string{"a", "b"}, Dir: dir,
		Env: []string{helperEnv + "=run", "HORUS_RUN_EXTRA=x"}, Stdout: &out, Stderr: &errOut})
	if err != nil || code != 5 {
		t.Fatalf("code %d, err %v", code, err)
	}
	if got := strings.TrimSpace(out.String()); got != "out a b x" {
		t.Errorf("stdout %q", got)
	}
	if got := strings.TrimSpace(errOut.String()); got != "err "+filepath.Base(dir) {
		t.Errorf("stderr %q", got)
	}
	code, err = Programs{}.Run(context.Background(), Command{Path: filepath.Join(dir, "no-such-program")})
	if err == nil || code != -1 {
		t.Errorf("a program that does not exist: code %d, err %v", code, err)
	}
	if _, err := (Programs{}).LookPath("no-such-program-of-horus"); err == nil {
		t.Error("LookPath found a program that does not exist")
	}
}
