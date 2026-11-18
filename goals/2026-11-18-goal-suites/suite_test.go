//go:build goal

// Package goal holds the goal rows of docs/specs/2026-11-18-goal-suites.md:
// one test function for each row. Run it with
//
//	go test -tags goal -v ./goals/2026-11-18-goal-suites
package goal

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// root is the repo root. go test starts a test in the folder of its package.
var root = filepath.Join("..", "..")

// run starts a program in the folder dir with the text stdin on standard
// input. It returns the exit code and the output. A program that does not
// start fails the row.
func run(t *testing.T, dir, stdin string, name string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), out.String()
	}
	if err != nil {
		t.Fatalf("%s did not start: %v", name, err)
	}
	return 0, out.String()
}

// python is the name of the Python program of this machine.
func python(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		if exec.Command(name, "-c", "").Run() == nil {
			return name
		}
	}
	t.Fatal("no python3 and no python on the PATH")
	return ""
}

// linkTests runs the named tests of the link check (package goals). Each of
// them must report "--- PASS".
func linkTests(t *testing.T, names ...string) {
	t.Helper()
	_, out := run(t, root, "", "go", "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", "./goals")
	for _, name := range names {
		if !regexp.MustCompile(`(?m)^--- PASS: ` + name + ` `).MatchString(out) {
			t.Errorf("the test %s of the link check did not pass", name)
		}
	}
	if t.Failed() {
		t.Logf("output of go test ./goals:\n%s", out)
	}
}

// write makes the files of a planted repo under dir.
func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// read gives the text of a file of the repo.
func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("missing: %s", name)
	}
	return string(b)
}

// planted is a repo with one frozen suite and one open suite.
var planted = map[string]string{
	"docs/specs/a-frozen.md":       "# Spec\n\n> **Record of a finished phase.** The work is done.\n",
	"docs/specs/b-open.md":         "# Spec\n",
	"goals/a-frozen/row_test.go":   "//go:build goal\n\npackage goal\n",
	"goals/b-open/row_test.go":     "//go:build goal\n\npackage goal\n",
	"internal/other/other_test.go": "package other\n",
}

const frozenScript = "scripts/frozen-suites.py"

func TestLink(t *testing.T) {
	linkTests(t, "TestRepo", "TestOpenSpecWithoutSuiteFails", "TestSuiteWithoutSpecFails",
		"TestRowWithoutFunctionFails", "TestFunctionWithoutRowFails", "TestFinishedSpecWithoutSuitePasses")
}

func TestOutsideOnly(t *testing.T) {
	linkTests(t, "TestSuiteThatImportsAHorusPackageFails")
}

func TestEmptyRow(t *testing.T) {
	linkTests(t, "TestRowFunctionThatNeverUsesTFails")
}

func TestFrozenGate(t *testing.T) {
	script, err := filepath.Abs(filepath.Join(root, frozenScript))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("not built: %s is missing", frozenScript)
	}
	dir := t.TempDir()
	write(t, dir, planted)
	py := python(t)
	for file, want := range map[string]int{
		"goals/a-frozen/row_test.go":   2,
		"goals/a-frozen/new_test.go":   2, // a new file in a frozen suite
		"goals/b-open/row_test.go":     0,
		"internal/other/other_test.go": 0,
		"docs/specs/a-frozen.md":       0,
	} {
		input := `{"tool_name":"Edit","tool_input":{"file_path":"` + filepath.ToSlash(filepath.Join(dir, file)) + `"}}`
		code, out := run(t, dir, input, py, script, "--hook", "--root", dir)
		if code != want {
			t.Errorf("hook for %s: exit code %d, want %d\n%s", file, code, want, out)
		}
	}
	settings := read(t, ".claude/settings.json")
	pre := strings.Index(settings, `"PreToolUse"`)
	if pre < 0 || !strings.Contains(settings[pre:], frozenScript+`\" --hook`) {
		t.Errorf(".claude/settings.json has no PreToolUse hook that runs %s --hook", frozenScript)
	}
}

func TestCiCompiles(t *testing.T) {
	ci := read(t, ".github/workflows/ci.yml")
	for _, step := range []string{"go vet -tags goal ./goals/...", frozenScript + " --base"} {
		if !strings.Contains(ci, step) {
			t.Errorf("ci.yml has no step with: %s", step)
		}
	}
	script, err := filepath.Abs(filepath.Join(root, frozenScript))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("not built: %s is missing", frozenScript)
	}
	py := python(t)
	for file, want := range map[string]int{"goals/a-frozen/row_test.go": 1, "goals/b-open/row_test.go": 0} {
		dir := t.TempDir()
		write(t, dir, planted)
		git := func(args ...string) {
			t.Helper()
			args = append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)
			if code, out := run(t, dir, "", "git", args...); code != 0 {
				t.Fatalf("git %v: %s", args, out)
			}
		}
		git("init", "-q")
		git("add", ".")
		git("commit", "-q", "-m", "base")
		write(t, dir, map[string]string{file: "//go:build goal\n\npackage goal\n\n// changed\n"})
		git("commit", "-q", "-a", "-m", "change")
		code, out := run(t, dir, "", py, script, "--base", "HEAD^", "--root", dir)
		if code != want {
			t.Errorf("--base with a change of %s: exit code %d, want %d\n%s", file, code, want, out)
		}
	}
}

func TestWorkflow(t *testing.T) {
	agents := read(t, "AGENTS.md")
	for _, text := range []string{"## Goal suites", "go test -tags goal -v ./goals/<spec stem>", "Never edit a frozen suite"} {
		if !strings.Contains(agents, text) {
			t.Errorf("AGENTS.md has no text: %s", text)
		}
	}
	old := "goal_" + "check" // the name of the Python goal tool of Phases 1 to 4
	code, out := run(t, root, "", "git", "grep", "-l", old, "--", ".", ":!docs/specs", ":!docs/adr")
	if code == 0 {
		t.Errorf("these files name the old tool %s.py:\n%s", old, out)
	} else if code != 1 {
		t.Fatalf("git grep failed: %s", out)
	}
}
