// Package register adds horus to Claude Code for the user. It ports the
// -Register part of scripts/install-horus-mcp.ps1.
//
// It runs "claude mcp add --scope user horus -- <path of horus> mcp". It first
// asks "claude mcp get horus" from a folder outside the repo, so the entry of
// .mcp.json does not count. When Claude Code already knows the server, it
// skips the add. Running it again is safe.
package register

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
)

// Options is everything that Register needs.
type Options struct {
	// Claude is the path of the claude program.
	Claude string
	// HorusPath is the path of the horus program that Claude Code starts.
	HorusPath string
	// AskDir is the folder where "claude mcp get" runs.
	AskDir string
	Stdout io.Writer
	Stderr io.Writer
}

// Register runs the two claude commands.
func Register(ctx context.Context, o Options) error {
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
	get := exec.CommandContext(ctx, o.Claude, "mcp", "get", "horus")
	get.Dir = o.AskDir
	get.Stdout, get.Stderr = io.Discard, io.Discard
	if err := get.Run(); err == nil {
		fmt.Fprintln(out, "SKIP: Claude Code already knows a server named horus (claude mcp get horus).")
		return nil
	} else if ctx.Err() != nil {
		return ctx.Err()
	}
	add := exec.CommandContext(ctx, o.Claude, "mcp", "add", "--scope", "user", "horus", "--", o.HorusPath, "mcp")
	add.Stdout, add.Stderr = o.Stdout, o.Stderr
	if err := add.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Errorf("claude mcp add failed (exit code %d).", ee.ExitCode())
		}
		return fmt.Errorf("claude mcp add failed: %w", err)
	}
	fmt.Fprintln(out, "OK: registered for Claude Code in user scope. Start a new Claude Code session to see the tools.")
	return nil
}

// Run is the command horus register. It returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("horus register", flag.ContinueOnError)
	fs.SetOutput(stderr)
	horus := fs.String("horus-path", "", "path of the horus program to register (default: this program)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		fmt.Fprintln(stderr, "horus register: The claude command was not found. Install Claude Code, or add the server by hand (docs/mcp.md).")
		return 1
	}
	path := *horus
	if path == "" {
		if path, err = os.Executable(); err != nil {
			fmt.Fprintln(stderr, "horus register:", err)
			return 1
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	o := Options{Claude: claude, HorusPath: path, AskDir: os.TempDir(), Stdout: stdout, Stderr: stderr}
	if err := Register(ctx, o); err != nil {
		fmt.Fprintln(stderr, "horus register:", err)
		return 1
	}
	return 0
}
