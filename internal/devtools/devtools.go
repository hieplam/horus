// Package devtools is the service of the tools for a workflow author: make
// a new workflow folder from the template (horus new-workflow), and copy the
// workflows into the Workflows pane of ComfyUI (horus sync-workflows).
//
// It reads and writes files only. It knows no flag and no output text: a
// handler writes the lines from the typed results.
package devtools

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// NewWorkflow makes workflowsDir/<name>/README.md from
// workflowsDir/_template/README.md. It returns the path of the README. It
// refuses a name that is not kebab-case, a missing template and a folder
// that exists.
func NewWorkflow(workflowsDir, name, description, modelFamily string) (string, error) {
	if !kebab.MatchString(name) {
		return "", fmt.Errorf("Name must be kebab-case (lowercase letters, digits, single hyphens): '%s'", name)
	}
	template := filepath.Join(workflowsDir, "_template", "README.md")
	target := filepath.Join(workflowsDir, name)
	data, err := os.ReadFile(template)
	if err != nil {
		return "", fmt.Errorf("Template not found: %s", template)
	}
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("Workflow folder already exists. Not overwriting: %s", target)
	}
	text := string(data)
	text = strings.ReplaceAll(text, "{{NAME}}", name)
	text = strings.ReplaceAll(text, "{{DESCRIPTION}}", description)
	text = strings.ReplaceAll(text, "{{MODEL_FAMILY}}", modelFamily)
	if err := os.Mkdir(target, 0o755); err != nil {
		return "", err
	}
	readme := filepath.Join(target, "README.md")
	if err := os.WriteFile(readme, []byte(text), 0o644); err != nil {
		return "", err
	}
	return readme, nil
}

// Install is the ComfyUI install that Sync writes into. cmd/horus gives it
// the backend of internal/backend.
type Install interface {
	// Installed gives the install folder, and reports if ComfyUI is
	// installed there.
	Installed() (dir string, ok bool)
}

// The states of one copy of Sync.
const (
	Copied   = "copied"   // the pane had no copy
	Same     = "ok"       // the pane copy is the repo copy
	Replaced = "replaced" // force: the pane copy differed; the old one is <file>.bak
	Conflict = "conflict" // the pane copy differs; it stays
)

// Copy is one workflow of Sync.
type Copy struct {
	File   string // the file in the pane: horus-<name>.json
	Source string // the repo copy: workflows/<name>/workflow.json, with the separator of this OS
	State  string // Copied, Same, Replaced or Conflict
}

// Sync copies each workflowsDir/<name>/workflow.json to
// <ComfyUI>/user/default/workflows/horus-<name>.json. The repo copy is the
// master. A pane copy that differs is a Conflict, and it stays; with force
// it is replaced and the old copy is kept as .bak. It returns the copies in
// the order of the folder. An error means that the sync could not start (no
// ComfyUI, no workflows folder) or a file could not be written; the copies
// before it are in the result.
func Sync(inst Install, workflowsDir string, force bool) ([]Copy, error) {
	dir, ok := inst.Installed()
	if !ok {
		return nil, fmt.Errorf("ComfyUI was not found in %s. Run horus install-comfyui first, or pass -comfyui-dir.", dir)
	}
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		return nil, fmt.Errorf("no workflows folder in the repo: %s", workflowsDir)
	}
	target := filepath.Join(dir, "ComfyUI", "user", "default", "workflows")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, err
	}
	copies := []Copy{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		source := filepath.Join(workflowsDir, e.Name(), "workflow.json")
		if fi, err := os.Stat(source); err != nil || !fi.Mode().IsRegular() {
			continue
		}
		c := Copy{File: "horus-" + e.Name() + ".json", Source: filepath.Join("workflows", e.Name(), "workflow.json")}
		dest := filepath.Join(target, c.File)
		src, err := os.ReadFile(source)
		if err != nil {
			return copies, err
		}
		old, err := os.ReadFile(dest)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if err := os.WriteFile(dest, src, 0o644); err != nil {
				return copies, err
			}
			c.State = Copied
		case err != nil:
			return copies, err
		case bytes.Equal(src, old):
			c.State = Same
		case force:
			if err := os.WriteFile(dest+".bak", old, 0o644); err != nil {
				return copies, err
			}
			if err := os.WriteFile(dest, src, 0o644); err != nil {
				return copies, err
			}
			c.State = Replaced
		default:
			c.State = Conflict
		}
		copies = append(copies, c)
	}
	return copies, nil
}
