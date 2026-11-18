// Package goals holds the goal suites: the folder goals/<spec stem>/ has one
// test function for each goal row of the spec docs/specs/<spec stem>.md.
// AGENTS.md, section "Goal suites", gives the rules.
//
// This file is the link check. go test ./... runs it on each pull request.
// It reads the specs and the suites as text, so it runs no goal row and it
// needs no build tag.
package goals

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// finishedMark starts the block that a finished spec has. The suite of a
// finished spec is frozen.
const finishedMark = "> **Record of a finished phase.**"

// buildLine is the line that keeps a suite file out of go test ./...
const buildLine = "//go:build goal"

// spec is what the link check reads from one spec file.
type spec struct {
	rows     []string // the goal rows of the table "Ratified goal"
	finished bool
}

// readSpec reads the goal rows and the finished block from the text of a spec.
// A goal row is a table line of the section "Ratified goal" whose first cell
// is a name in backticks.
func readSpec(text string) spec {
	var s spec
	inGoal := false
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimRight(ln, "\r")
		switch {
		case strings.HasPrefix(ln, finishedMark):
			s.finished = true
		case strings.HasPrefix(ln, "## "):
			inGoal = strings.HasPrefix(ln, "## Ratified goal")
		case inGoal && strings.HasPrefix(ln, "| `"):
			if name, _, ok := strings.Cut(ln[3:], "`"); ok && name != "" {
				s.rows = append(s.rows, name)
			}
		}
	}
	return s
}

// funcName gives the test function of a goal row: same-behaviour is
// TestSameBehaviour.
func funcName(row string) string {
	name := "Test"
	for _, part := range strings.Split(row, "-") {
		if part != "" {
			name += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return name
}

// check gives one text for each break of a rule in the repo at root, sorted.
func check(root string) ([]string, error) {
	specs := map[string]spec{}
	paths, err := filepath.Glob(filepath.Join(root, "docs", "specs", "*.md"))
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		text, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		specs[strings.TrimSuffix(filepath.Base(p), ".md")] = readSpec(string(text))
	}
	entries, err := os.ReadDir(filepath.Join(root, "goals"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var problems []string
	suites := map[string]bool{}
	for _, e := range entries {
		stem := e.Name()
		if !e.IsDir() || stem == "testdata" || strings.HasPrefix(stem, ".") || strings.HasPrefix(stem, "_") {
			continue
		}
		suites[stem] = true
		s, ok := specs[stem]
		if !ok {
			problems = append(problems, fmt.Sprintf("goals/%s: the suite has no spec docs/specs/%s.md", stem, stem))
			continue
		}
		found, err := checkSuite(filepath.Join(root, "goals", stem), stem, s)
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	for stem, s := range specs {
		if !s.finished && len(s.rows) > 0 && !suites[stem] {
			problems = append(problems, fmt.Sprintf("docs/specs/%s.md: the open spec has no suite goals/%s/", stem, stem))
		}
	}
	sort.Strings(problems)
	return problems, nil
}

// checkSuite checks the files of one suite against the rows of its spec.
func checkSuite(dir, stem string, s spec) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	var problems []string
	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf("goals/%s: ", stem)+fmt.Sprintf(format, args...))
	}
	usesT := map[string]bool{} // each test function of the suite: does its body use t
	for _, p := range files {
		name := filepath.Base(p)
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(name, "_test.go") {
			add("%s is not a _test.go file", name)
		}
		if !hasBuildLine(string(src)) {
			add("%s has no line %s before the package line", name, buildLine)
		}
		af, err := parser.ParseFile(token.NewFileSet(), p, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, imp := range af.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if first, _, _ := strings.Cut(path, "/"); strings.Contains(first, ".") {
				add("%s imports %s: a suite imports only the standard library", name, path)
			}
		}
		for _, d := range af.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "TestMain" {
				continue
			}
			usesT[fn.Name.Name] = bodyUses(fn)
		}
	}
	rowOf := map[string]string{}
	for _, row := range s.rows {
		fn := funcName(row)
		rowOf[fn] = row
		used, ok := usesT[fn]
		switch {
		case !ok:
			add("the row `%s` has no function %s", row, fn)
		case !used:
			add("%s never uses t: an empty row function passes", fn)
		}
	}
	for fn := range usesT {
		if _, ok := rowOf[fn]; !ok {
			add("%s matches no row of docs/specs/%s.md", fn, stem)
		}
	}
	return problems, nil
}

// hasBuildLine tells if the line //go:build goal is before the package line.
func hasBuildLine(src string) bool {
	for _, ln := range strings.Split(src, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == buildLine {
			return true
		}
		if strings.HasPrefix(ln, "package ") {
			return false
		}
	}
	return false
}

// bodyUses tells if the body of a test function uses its parameter.
func bodyUses(fn *ast.FuncDecl) bool {
	params := fn.Type.Params.List
	if fn.Body == nil || len(params) != 1 || len(params[0].Names) != 1 || params[0].Names[0].Name == "_" {
		return false
	}
	name, used := params[0].Names[0].Name, false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			used = true
		}
		return !used
	})
	return used
}

// ---------------------------------------------------------------------------
// The tests. Each one builds a planted repo in a temporary folder.
// ---------------------------------------------------------------------------

const (
	openSpec     = "# Spec\n\n## Ratified goal (owner)\n\n| Row | Outcome |\n|---|---|\n| `same-behaviour` | x |\n| `link` | y |\n\n## Scope\n\n| `not-a-row` | z |\n"
	finishedSpec = "# Spec\n\n" + finishedMark + " Done.\n\n" + "## Ratified goal (owner)\n\n| Row | Outcome |\n|---|---|\n| `same-behaviour` | x |\n| `link` | y |\n"
	header       = buildLine + "\n\npackage goal\n\nimport \"testing\"\n\n"
	goodSuite    = header + "func TestSameBehaviour(t *testing.T) { t.Fatal(\"not built\") }\n\nfunc TestLink(t *testing.T) { helper(t) }\n\nfunc helper(t *testing.T) {}\n"
)

// planted writes the files under a temporary folder and returns the problems
// of the link check for that folder.
func planted(t *testing.T, files map[string]string) []string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	problems, err := check(root)
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

// wantOne fails unless the problems are exactly one text that holds each part.
func wantOne(t *testing.T, problems []string, parts ...string) {
	t.Helper()
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1: %q", len(problems), problems)
	}
	for _, part := range parts {
		if !strings.Contains(problems[0], part) {
			t.Errorf("the problem %q does not hold %q", problems[0], part)
		}
	}
}

// TestRepo applies the link check to this repo.
func TestRepo(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "go.mod")); err != nil {
		t.Fatalf("the parent folder is not the repo root: %v", err)
	}
	problems, err := check("..")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func TestGoodSuitePasses(t *testing.T) {
	for name, specText := range map[string]string{"open": openSpec, "finished": finishedSpec} {
		got := planted(t, map[string]string{"docs/specs/2026-01-02-x.md": specText, "goals/2026-01-02-x/row_test.go": goodSuite})
		if len(got) != 0 {
			t.Errorf("%s spec: got %q, want no problem", name, got)
		}
	}
}

func TestOpenSpecWithoutSuiteFails(t *testing.T) {
	got := planted(t, map[string]string{"docs/specs/2026-01-02-x.md": openSpec})
	wantOne(t, got, "docs/specs/2026-01-02-x.md", "the open spec has no suite goals/2026-01-02-x/")
}

func TestFinishedSpecWithoutSuitePasses(t *testing.T) {
	if got := planted(t, map[string]string{"docs/specs/2026-01-02-x.md": finishedSpec}); len(got) != 0 {
		t.Errorf("got %q, want no problem", got)
	}
}

func TestSuiteWithoutSpecFails(t *testing.T) {
	got := planted(t, map[string]string{"goals/2026-01-02-x/row_test.go": goodSuite})
	wantOne(t, got, "goals/2026-01-02-x", "has no spec docs/specs/2026-01-02-x.md")
}

func TestRowWithoutFunctionFails(t *testing.T) {
	suite := header + "func TestLink(t *testing.T) { t.Fatal(\"not built\") }\n"
	got := planted(t, map[string]string{"docs/specs/2026-01-02-x.md": openSpec, "goals/2026-01-02-x/row_test.go": suite})
	wantOne(t, got, "the row `same-behaviour` has no function TestSameBehaviour")
}

func TestFunctionWithoutRowFails(t *testing.T) {
	suite := goodSuite + "\nfunc TestNotARow(t *testing.T) { t.Fatal(\"x\") }\n"
	got := planted(t, map[string]string{"docs/specs/2026-01-02-x.md": openSpec, "goals/2026-01-02-x/row_test.go": suite})
	wantOne(t, got, "TestNotARow matches no row")
}

func TestRowFunctionThatNeverUsesTFails(t *testing.T) {
	for name, fn := range map[string]string{
		"an empty body":     "func TestLink(t *testing.T) {}\n",
		"a body without t":  "func TestLink(t *testing.T) { println(1) }\n",
		"a blank parameter": "func TestLink(_ *testing.T) { println(1) }\n",
	} {
		suite := header + "func TestSameBehaviour(t *testing.T) { t.Fatal(\"not built\") }\n\n" + fn
		got := planted(t, map[string]string{"docs/specs/2026-01-02-x.md": openSpec, "goals/2026-01-02-x/row_test.go": suite})
		t.Run(name, func(t *testing.T) { wantOne(t, got, "TestLink never uses t") })
	}
}

func TestSuiteThatImportsAHorusPackageFails(t *testing.T) {
	suite := strings.Replace(goodSuite, "import \"testing\"", "import (\n\t\"testing\"\n\n\t\"github.com/hieplam/horus/internal/herr\"\n)\n\nvar _ = herr.Code(\"\")", 1)
	got := planted(t, map[string]string{"docs/specs/2026-01-02-x.md": openSpec, "goals/2026-01-02-x/row_test.go": suite})
	wantOne(t, got, "imports github.com/hieplam/horus/internal/herr", "only the standard library")
}

func TestFileWithoutTheBuildLineFails(t *testing.T) {
	suite := strings.TrimPrefix(goodSuite, buildLine+"\n\n")
	got := planted(t, map[string]string{"docs/specs/2026-01-02-x.md": openSpec, "goals/2026-01-02-x/row_test.go": suite})
	wantOne(t, got, "row_test.go has no line //go:build goal")
}

func TestFuncName(t *testing.T) {
	for row, want := range map[string]string{"link": "TestLink", "same-behaviour": "TestSameBehaviour", "mac-image": "TestMacImage"} {
		if got := funcName(row); got != want {
			t.Errorf("funcName(%q) = %q, want %q", row, got, want)
		}
	}
}
