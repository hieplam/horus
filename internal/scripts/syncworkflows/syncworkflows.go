// Package syncworkflows copies the repo workflows into the Workflows pane of
// ComfyUI. It ports scripts/sync-workflows.ps1.
//
// For each workflows/<name>/workflow.json it writes
// <ComfyUIDir>/ComfyUI/user/default/workflows/horus-<name>.json. The repo copy
// is the master. When a pane copy differs, Sync reports a CONFLICT and leaves
// it alone. With Force it replaces the copy and keeps the old one as .bak.
package syncworkflows

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hieplam/horus/internal/settings"
)

// Options is everything that Sync needs.
type Options struct {
	ComfyUIDir   string
	WorkflowsDir string
	Force        bool
	Stdout       io.Writer
}

// Sync copies the workflows. It returns the number of conflicts. An error
// means that the sync could not start (no ComfyUI, no workflows folder).
func Sync(o Options) (int, error) {
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
	if !isFile(filepath.Join(o.ComfyUIDir, "ComfyUI", "main.py")) {
		return 0, fmt.Errorf("ComfyUI was not found in %s. Run horus install-comfyui first, or pass -comfyui-dir.", o.ComfyUIDir)
	}
	entries, err := os.ReadDir(o.WorkflowsDir)
	if err != nil {
		return 0, fmt.Errorf("no workflows folder in the repo: %s", o.WorkflowsDir)
	}
	target := filepath.Join(o.ComfyUIDir, "ComfyUI", "user", "default", "workflows")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return 0, err
	}
	conflicts, count := 0, 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		source := filepath.Join(o.WorkflowsDir, e.Name(), "workflow.json")
		if !isFile(source) {
			continue
		}
		count++
		file := "horus-" + e.Name() + ".json"
		dest := filepath.Join(target, file)
		src, err := os.ReadFile(source)
		if err != nil {
			return conflicts, err
		}
		old, err := os.ReadFile(dest)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if err := os.WriteFile(dest, src, 0o644); err != nil {
				return conflicts, err
			}
			fmt.Fprintf(out, "copied   %s\n", file)
		case err != nil:
			return conflicts, err
		case bytes.Equal(src, old):
			fmt.Fprintf(out, "ok       %s\n", file)
		case o.Force:
			if err := os.WriteFile(dest+".bak", old, 0o644); err != nil {
				return conflicts, err
			}
			if err := os.WriteFile(dest, src, 0o644); err != nil {
				return conflicts, err
			}
			fmt.Fprintf(out, "replaced %s (old copy kept as .bak)\n", file)
		default:
			conflicts++
			fmt.Fprintf(out, "CONFLICT %s differs from %s. Compare both, then run again with -force to use the repo copy.\n",
				file, filepath.Join("workflows", e.Name(), "workflow.json"))
		}
	}
	if count == 0 {
		fmt.Fprintln(out, "No workflows/*/workflow.json files found. Nothing to sync.")
	}
	return conflicts, nil
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// Run is the command horus sync-workflows. It returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("horus sync-workflows", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("comfyui-dir", "", "the portable folder (default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)")
	force := fs.Bool("force", false, "replace a pane copy that differs from the repo copy; the old copy is kept as .bak")
	wfDir := fs.String("workflows-dir", "", "the workflows folder (default: HORUS_WORKFLOWS_DIR, then <repo>/workflows)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	o := Options{ComfyUIDir: *dir, WorkflowsDir: *wfDir, Force: *force, Stdout: stdout}
	if o.ComfyUIDir == "" {
		o.ComfyUIDir = settings.ComfyDir()
	}
	if o.WorkflowsDir == "" {
		o.WorkflowsDir = settings.WorkflowsDir()
	}
	conflicts, err := Sync(o)
	if err != nil {
		fmt.Fprintln(stderr, "horus sync-workflows:", err)
		return 1
	}
	if conflicts > 0 {
		return 1
	}
	return 0
}
