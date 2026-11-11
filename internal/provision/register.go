// The MCP registration: add horus to Claude Code for the user (horus
// register). It ports the -Register part of scripts/install-horus-mcp.ps1.
//
// It runs "claude mcp add --scope user horus -e HORUS_ROOT=<repo> -- <path of
// horus> mcp". HORUS_ROOT lets a session in any folder find the workflows. It
// first asks "claude mcp get horus" from a folder outside the repo, so the
// entry of .mcp.json does not count. When Claude Code already knows the
// server, it skips the add. Running it again is safe.
package provision

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hieplam/horus/internal/platform"
)

// RegisterOptions is everything that Register needs.
type RegisterOptions struct {
	// Claude is the path of the claude program. Empty: find claude on the
	// PATH.
	Claude string
	// HorusPath is the path of the horus program that Claude Code starts.
	HorusPath string
	// Root is the absolute path of the repo (the folder with workflows/).
	// Register passes it to the server as HORUS_ROOT.
	Root string
	// AskDir is the folder where "claude mcp get" runs.
	AskDir string
	// Stdout gets the result line and the output of "claude mcp add";
	// Stderr gets its errors. nil drops them.
	Stdout, Stderr io.Writer
}

// Register checks the root, finds claude, and runs the two claude commands.
func Register(ctx context.Context, programs Programs, o RegisterOptions) error {
	if err := CheckRoot(o.Root); err != nil {
		return err
	}
	if o.Claude == "" {
		p, err := programs.LookPath("claude")
		if err != nil {
			return fmt.Errorf("The claude command was not found. Install Claude Code, or add the server by hand (docs/mcp.md).")
		}
		o.Claude = p
	}
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
	code, err := programs.Run(ctx, platform.Command{Path: o.Claude, Args: []string{"mcp", "get", "horus"}, Dir: o.AskDir})
	if err == nil && code == 0 {
		fmt.Fprintln(out, "SKIP: Claude Code already knows a server named horus (claude mcp get horus).")
		return nil
	} else if ctx.Err() != nil {
		return ctx.Err()
	}
	code, err = programs.Run(ctx, platform.Command{Path: o.Claude,
		Args:   []string{"mcp", "add", "--scope", "user", "horus", "-e", "HORUS_ROOT=" + o.Root, "--", o.HorusPath, "mcp"},
		Stdout: o.Stdout, Stderr: o.Stderr})
	if err != nil {
		return fmt.Errorf("claude mcp add failed: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("claude mcp add failed (exit code %d).", code)
	}
	fmt.Fprintln(out, "OK: registered for Claude Code in user scope. Start a new Claude Code session to see the tools.")
	return nil
}

// CheckRoot fails when root is not an absolute path of a folder that has a
// workflows folder. A registered entry with such a root would serve no workflow.
func CheckRoot(root string) error {
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("the repo root %q is not an absolute path.", root)
	}
	if fi, err := os.Stat(filepath.Join(root, "workflows")); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s has no workflows folder, so it is not the horus repo. Run horus register inside the repo, or pass -root <repo folder>.", root)
	}
	return nil
}
