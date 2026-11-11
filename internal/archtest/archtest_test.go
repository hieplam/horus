package archtest

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/hieplam/horus"

// readTree returns every .go file under root, with its path from root. Like
// the go tool, it skips testdata and folders whose name starts with . or _,
// and folders of another module.
func readTree(t *testing.T, root string) []File {
	t.Helper()
	var files []File
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p == root {
				return nil
			}
			if name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, File{Path: filepath.ToSlash(rel), Src: src})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestRules applies the rules to the module and compares the result with the
// allow lists.
func TestRules(t *testing.T) {
	files := readTree(t, filepath.Join("..", ".."))
	if len(files) < 50 {
		t.Fatalf("found only %d .go files: the walk did not start at the module root", len(files))
	}
	found, err := Scan(module, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range Rules {
		data, err := os.ReadFile("allow_" + rule + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range Compare(rule, found, ParseAllow(string(data))) {
			t.Error(p)
		}
	}
}

// planted gives, for each file under testdata/planted, the rules that it must
// break. A file with no rule must break none.
var planted = map[string][]string{
	// os-boundary: one file for each case.
	"internal/catalog/name_windows.go":   {RuleOS},
	"internal/catalog/name_unix_test.go": {RuleOS},
	"internal/catalog/goos.go":           {RuleOS},
	"internal/catalog/goos_alias.go":     {RuleOS},
	"internal/catalog/build.go":          {RuleOS},
	"internal/catalog/build_not.go":      {RuleOS},
	"internal/catalog/syscall.go":        {RuleOS},
	"internal/catalog/xsys.go":           {RuleOS},
	"internal/platform/ok_windows.go":    nil,
	"internal/backend/ok_windows.go":     nil,
	"internal/catalog/build_tag.go":      nil, // a build tag that is not an OS

	// direction: one file for each import rule.
	"internal/cli/adapter.go":            {RuleDirection}, // handler -> adapter
	"internal/cli/service.go":            nil,             // handler -> service, base
	"internal/generate/adapter.go":       {RuleDirection}, // service -> adapter
	"internal/provision/service.go":      {RuleDirection}, // service -> service
	"internal/doctor/nethttp.go":         {RuleDirection}, // service -> net/http
	"internal/devtools/osexec.go":        {RuleDirection}, // service -> os/exec
	"internal/download/adapter.go":       {RuleDirection}, // adapter -> adapter
	"internal/comfy/service.go":          {RuleDirection}, // adapter -> service
	"internal/herr/adapter.go":           {RuleDirection}, // base -> adapter
	"internal/herr/handler.go":           {RuleDirection}, // base -> handler
	"internal/generate/fake.go":          {RuleDirection}, // comfytest in a file that is not a test
	"internal/generate/fake_test.go":     nil,             // comfytest in a test file
	"internal/generate/adapter_test.go":  {RuleDirection}, // a test file follows the rules of its package
	"internal/nolayer/x.go":              {RuleDirection}, // a package with no layer
	"cmd/horus/wiring.go":                nil,             // the wiring imports every package
	"internal/catalog/catalog_x_test.go": nil,             // a package imports itself

	// token
	"internal/catalog/token.go":         {RuleToken},
	"internal/catalog/token_raw.go":     {RuleToken},
	"internal/catalog/token_env.go":     {RuleToken},
	"internal/download/token.go":        nil,
	"internal/settings/settings.go":     nil,         // the refusal
	"internal/catalog/token_invalid.go": nil,         // the error code HF_TOKEN_INVALID
	"internal/catalog/token_comment.go": nil,         // HF_TOKEN in a comment
	"cmd/horus/token_name.go":           {RuleToken}, // a name of internal/download that names the token
	"cmd/horus/token_alias.go":          {RuleToken}, // the same through an import alias
}

// TestPlanted proves that the rules flag each planted bad file, and only
// those.
func TestPlanted(t *testing.T) {
	files := readTree(t, filepath.Join("testdata", "planted"))
	got := map[string][]string{}
	for _, f := range files {
		if _, ok := planted[f.Path]; !ok {
			t.Errorf("%s: the table planted has no line for this file", f.Path)
		}
		got[f.Path] = nil
	}
	found, err := Scan(module, files)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range found {
		if !slices.Contains(got[v.File], v.Rule) {
			got[v.File] = append(got[v.File], v.Rule)
		}
	}
	for p, want := range planted {
		g, ok := got[p]
		if !ok {
			t.Errorf("%s: the planted file is missing", p)
			continue
		}
		slices.Sort(g)
		if !slices.Equal(g, want) {
			t.Errorf("%s: the rules flag %v, want %v", p, g, want)
		}
	}
}

// TestCompare proves the two answers of an allow list: a violation that is
// not on the list, and an entry that no longer breaks the rule.
func TestCompare(t *testing.T) {
	found := []Violation{{Rule: RuleToken, File: "a.go", Key: "a.go", Why: "x"}}
	if p := Compare(RuleToken, found, []string{"a.go"}); len(p) != 0 {
		t.Errorf("exact list: %v", p)
	}
	p := Compare(RuleToken, found, nil)
	if len(p) != 1 || !strings.Contains(p[0], "the allow list only becomes shorter") {
		t.Errorf("violation not on the list: %v", p)
	}
	p = Compare(RuleToken, found, []string{"a.go", "b.go"})
	if len(p) != 1 || !strings.Contains(p[0], `"b.go" no longer breaks the rule: remove this entry`) {
		t.Errorf("extra entry: %v", p)
	}
	p = Compare(RuleToken, found, []string{"a.go", "a.go"})
	if len(p) != 1 || !strings.Contains(p[0], "remove this entry") {
		t.Errorf("entry two times: %v", p)
	}
	if got := ParseAllow("# head\r\n\r\ninternal/a.go\r\n  b -> c  \n"); !slices.Equal(got, []string{"internal/a.go", "b -> c"}) {
		t.Errorf("ParseAllow: %q", got)
	}
}
