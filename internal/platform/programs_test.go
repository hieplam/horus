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

// TestRunPassesTheStandardInput runs the test binary in the role "echo": it
// copies its standard input to its standard output. A Command without Stdin
// gives the program no input.
func TestRunPassesTheStandardInput(t *testing.T) {
	in := "line one\r\nline two \u00e9\x00\n"
	var out bytes.Buffer
	code, err := Programs{}.Run(context.Background(), Command{Path: os.Args[0], Env: []string{helperEnv + "=echo"},
		Stdin: strings.NewReader(in), Stdout: &out})
	if err != nil || code != 0 || out.String() != in {
		t.Fatalf("code %d, err %v, stdout %q, want %q", code, err, out.String(), in)
	}
	out.Reset()
	code, err = Programs{}.Run(context.Background(), Command{Path: os.Args[0], Env: []string{helperEnv + "=echo"}, Stdout: &out})
	if err != nil || code != 0 || out.Len() != 0 {
		t.Fatalf("no Stdin: code %d, err %v, stdout %q", code, err, out.String())
	}
}
