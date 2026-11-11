// Package archtest holds the rules of the spec docs/specs/2026-11-11-restructure.md
// that a test can check on the source text:
//
//   - os-boundary: code for one operating system is only in internal/platform
//     and internal/backend.
//   - direction: a package imports only the layers that the import rules allow.
//   - token: only internal/download names the variable HF_TOKEN, and no
//     other package uses a name of internal/download that has "token" in it.
//
// The rules read file names and file text only. They do not use the build
// constraints of the current system, so they give the same result on every
// operating system. The test in archtest_test.go walks the module and
// fails for each break. There is no allow list, so no file can be exempt
// from a rule except the places that layers.go names.
package archtest

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// The names of the rules.
const (
	RuleOS        = "os-boundary"
	RuleDirection = "direction"
	RuleToken     = "token"
)

// Rules are the names of all rules.
var Rules = []string{RuleOS, RuleDirection, RuleToken}

// File is one Go source file: its path from the module root, with slashes,
// and its text.
type File struct {
	Path string
	Src  []byte
}

// Violation is one break of a rule. Key names what breaks it: the file path
// for os-boundary and token, "<package> -> <import>" for direction.
type Violation struct {
	Rule string
	File string
	Key  string
	Why  string
}

// Scan applies the three rules to the files of the module with the given
// module path. It returns an error only when a file does not parse.
func Scan(module string, files []File) ([]Violation, error) {
	var out []Violation
	for _, f := range files {
		fset := token.NewFileSet()
		af, err := parser.ParseFile(fset, f.Path, f.Src, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Path, err)
		}
		out = append(out, osBoundary(f, af)...)
		out = append(out, direction(module, f, af)...)
		out = append(out, tokenRule(module, f, af)...)
	}
	return out, nil
}

// Problems gives one text for each break of a rule, once for each rule and
// key, sorted. There is no allow list: every break fails the test.
func Problems(found []Violation) []string {
	var problems []string
	seen := map[string]bool{}
	for _, v := range found {
		if seen[v.Rule+" "+v.Key] {
			continue
		}
		seen[v.Rule+" "+v.Key] = true
		problems = append(problems, fmt.Sprintf("%s: %s: %s (%s). Fix the code: archtest has no allow list", v.Rule, v.Key, v.Why, v.File))
	}
	sort.Strings(problems)
	return problems
}

// inside tells if the file or package p is one of the places, or in one.
func inside(p string, places []string) bool {
	for _, pl := range places {
		if p == pl || strings.HasPrefix(p, pl+"/") {
			return true
		}
	}
	return false
}

func osBoundary(f File, af *ast.File) []Violation {
	if inside(path.Dir(f.Path), osExempt) {
		return nil
	}
	var why []string
	name := strings.TrimSuffix(strings.TrimSuffix(path.Base(f.Path), ".go"), "_test")
	parts := strings.Split(name, "_")
	for _, p := range parts[1:] {
		if slices.Contains(osNames, p) {
			why = append(why, "the file name has the OS name "+p)
			break
		}
	}
	if tag := buildOS(f.Src); tag != "" {
		why = append(why, "a build constraint names the OS "+tag)
	}
	for _, imp := range af.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if p == "syscall" || p == "golang.org/x/sys" || strings.HasPrefix(p, "golang.org/x/sys/") {
			why = append(why, "it imports "+p)
		}
	}
	if usesGOOS(af) {
		why = append(why, "it uses runtime.GOOS")
	}
	if len(why) == 0 {
		return nil
	}
	return []Violation{{Rule: RuleOS, File: f.Path, Key: f.Path, Why: strings.Join(why, ", ")}}
}

// buildOS returns the first OS name in a //go:build or // +build line of the
// file header, or "".
func buildOS(src []byte) string {
	for _, ln := range strings.Split(string(src), "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "package ") {
			break
		}
		if !constraint.IsGoBuild(ln) && !constraint.IsPlusBuild(ln) {
			continue
		}
		expr, err := constraint.Parse(ln)
		if err != nil {
			continue
		}
		found := ""
		expr.Eval(func(tag string) bool {
			if found == "" && slices.Contains(osNames, tag) {
				found = tag
			}
			return false
		})
		if found != "" {
			return found
		}
	}
	return ""
}

func usesGOOS(af *ast.File) bool {
	local := ""
	for _, imp := range af.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == "runtime" {
			local = "runtime"
			if imp.Name != nil {
				local = imp.Name.Name
			}
		}
	}
	if local == "" {
		return false
	}
	found := false
	ast.Inspect(af, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "GOOS" {
			if id, ok := s.X.(*ast.Ident); ok && id.Name == local {
				found = true
			}
		}
		return !found
	})
	return found
}

func direction(module string, f File, af *ast.File) []Violation {
	pkg := path.Dir(f.Path)
	isTest := strings.HasSuffix(f.Path, "_test.go")
	layer, ok := layers[pkg]
	if !ok {
		return []Violation{{Rule: RuleDirection, File: f.Path, Key: pkg + " -> ?", Why: "the package has no layer in internal/archtest/layers.go"}}
	}
	var out []Violation
	add := func(imp, why string) {
		out = append(out, Violation{Rule: RuleDirection, File: f.Path, Key: pkg + " -> " + imp, Why: why})
	}
	for _, spec := range af.Imports {
		imp, _ := strconv.Unquote(spec.Path.Value)
		if layer == service && slices.Contains(serviceBans, imp) {
			add(imp, "a service must not import "+imp)
			continue
		}
		rel, internal := strings.CutPrefix(imp, module+"/")
		if !internal || rel == pkg {
			continue
		}
		target, known := layers[rel]
		if !known {
			continue // its own files report that it has no layer
		}
		if target == test {
			if !isTest {
				add(rel, "only a test file can import the test package "+rel)
			}
			continue
		}
		allowed, limited := mayImport[layer]
		if limited && !slices.Contains(allowed, target) {
			add(rel, fmt.Sprintf("the %s layer must not import the %s layer", layer, target))
		}
	}
	return out
}

// tokenText is a string that names the variable of the Hugging Face token:
// the name alone, or the name and "=" as in a line of a .env file.
var tokenText = regexp.MustCompile(`^\s*HF_TOKEN\s*(=|$)`)

func tokenRule(module string, f File, af *ast.File) []Violation {
	if inside(f.Path, tokenExempt) {
		return nil
	}
	// The local name of the import of the download package, if any.
	dl := ""
	for _, imp := range af.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p == module+"/internal/download" {
			dl = "download"
			if imp.Name != nil {
				dl = imp.Name.Name
			}
		}
	}
	why := ""
	ast.Inspect(af, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BasicLit:
			if s, err := strconv.Unquote(x.Value); err == nil && x.Kind == token.STRING && tokenText.MatchString(s) {
				why = "a string names HF_TOKEN outside internal/download"
			}
		case *ast.SelectorExpr:
			// A name of the download package that names the token, for
			// example a constant that holds the name of the variable.
			if id, ok := x.X.(*ast.Ident); ok && dl != "" && id.Name == dl && strings.Contains(strings.ToLower(x.Sel.Name), "token") {
				why = "it uses " + dl + "." + x.Sel.Name + " of internal/download"
			}
		}
		return why == ""
	})
	if why == "" {
		return nil
	}
	return []Violation{{Rule: RuleToken, File: f.Path, Key: f.Path, Why: why}}
}
