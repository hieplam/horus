// Package newworkflow makes workflows/<name>/README.md from
// workflows/_template/README.md. It ports scripts/new-workflow.ps1.
//
// Exit code 0 on success. Exit code 1 if the name is not kebab-case, the
// template is missing, or the workflow folder already exists.
package newworkflow

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hieplam/horus/internal/settings"
)

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Create makes the folder and its README.md in workflowsDir. It returns the
// path of the README.
func Create(workflowsDir, name, description, modelFamily string) (string, error) {
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

// Run is the command horus new-workflow. It returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("horus new-workflow", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "folder name of the new workflow, in kebab-case (required)")
	description := fs.String("description", "", "one line that says what the workflow makes (required)")
	family := fs.String("model-family", "TBD", "the model family, for example \"FLUX 3 Image Klein\"")
	wfDir := fs.String("workflows-dir", "", "the workflows folder (default: HORUS_WORKFLOWS_DIR, then <repo>/workflows)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *name == "" || *description == "" {
		fmt.Fprintln(stderr, "horus new-workflow: -name and -description are required")
		return 2
	}
	dir := *wfDir
	if dir == "" {
		dir = settings.WorkflowsDir()
	}
	readme, err := Create(dir, *name, *description, *family)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Created %s\n", readme)
	fmt.Fprintln(stdout, "Next: add workflow.json through the ComfyUI UI export or a template. Do not hand-edit the graph.")
	return 0
}
