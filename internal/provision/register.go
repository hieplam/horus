// The MCP registration: add horus to Claude Code or to Codex for the user
// (horus register -client claude|codex).
//
// Claude Code: "claude mcp add --scope user horus -e HORUS_ROOT=<repo> -- <path
// of horus> mcp". Codex: "codex mcp add horus --env HORUS_ROOT=<repo> -- <path
// of horus> mcp". HORUS_ROOT lets a session in any folder find the workflows.
// It goes to the client only when there is a repo root. Register first asks
// "<client> mcp get horus" from a folder outside the repo, so the entry of
// .mcp.json does not count. When the client already knows the server, it skips
// the add. Running it again is safe.
package provision

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hieplam/horus/internal/platform"
)

// The two clients that Register serves.
const (
	ClientClaude = "claude"
	ClientCodex  = "codex"
)

// ClientNames is the text that lists the two client names.
const ClientNames = "claude, codex"

// RegisterOptions is everything that Register needs.
type RegisterOptions struct {
	// Client is ClientClaude or ClientCodex. Empty means ClientClaude.
	Client string
	// Program is the path of the client program. Empty: find it on the PATH.
	Program string
	// HorusPath is the path of the horus program that the client starts.
	HorusPath string
	// Root is the absolute path of the repo (the folder with workflows/).
	// Register passes it to the server as HORUS_ROOT. Empty means there is no
	// repo root, and Register passes no HORUS_ROOT.
	Root string
	// AskDir is the folder where "<client> mcp get" runs.
	AskDir string
	// Stdout gets the result line and the output of "mcp add";
	// Stderr gets its errors. nil drops them.
	Stdout, Stderr io.Writer
}

// clientInfo is what differs between the two clients.
type clientInfo struct {
	name, title, envFlag, session string
	// addArgs is the "mcp add" argument list, up to the "--" and the command.
	addArgs func(root string) []string
}

func clientOf(name string) (clientInfo, error) {
	switch name {
	case "", ClientClaude:
		return clientInfo{name: "claude", title: "Claude Code", session: "Claude Code session",
			addArgs: func(root string) []string {
				a := []string{"mcp", "add", "--scope", "user", "horus"}
				if root != "" {
					a = append(a, "-e", "HORUS_ROOT="+root)
				}
				return a
			}}, nil
	case ClientCodex:
		return clientInfo{name: "codex", title: "Codex", session: "Codex session",
			addArgs: func(root string) []string {
				a := []string{"mcp", "add", "horus"}
				if root != "" {
					a = append(a, "--env", "HORUS_ROOT="+root)
				}
				return a
			}}, nil
	}
	return clientInfo{}, fmt.Errorf("unknown client %q. The clients are: %s.", name, ClientNames)
}

// Register checks the root, finds the client program, and runs its two
// commands.
func Register(ctx context.Context, programs Programs, o RegisterOptions) error {
	c, err := clientOf(o.Client)
	if err != nil {
		return err
	}
	if o.Root != "" {
		if err := checkRoot(o.Root); err != nil {
			return err
		}
	}
	if o.Program == "" {
		p, err := programs.LookPath(c.name)
		if err != nil {
			return fmt.Errorf("The %s command was not found. Install %s, or add the server by hand (docs/mcp.md).", c.name, c.title)
		}
		o.Program = p
	}
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
	code, err := programs.Run(ctx, platform.Command{Path: o.Program, Args: []string{"mcp", "get", "horus"}, Dir: o.AskDir})
	if err == nil && code == 0 {
		fmt.Fprintf(out, "SKIP: %s already knows a server named horus (%s mcp get horus).\n", c.title, c.name)
		return nil
	} else if ctx.Err() != nil {
		return ctx.Err()
	}
	args := append(c.addArgs(o.Root), "--", o.HorusPath, "mcp")
	code, err = programs.Run(ctx, platform.Command{Path: o.Program, Args: args, Stdout: o.Stdout, Stderr: o.Stderr})
	if err != nil {
		return fmt.Errorf("%s mcp add failed: %w", c.name, err)
	}
	if code != 0 {
		return fmt.Errorf("%s mcp add failed (exit code %d).", c.name, code)
	}
	if c.name == "claude" {
		fmt.Fprintln(out, "OK: registered for Claude Code in user scope. Start a new Claude Code session to see the tools.")
	} else {
		fmt.Fprintln(out, "OK: registered for Codex. Start a new Codex session to see the tools.")
	}
	return nil
}

// checkRoot fails when root is not an absolute path of a folder that has a
// workflows folder. A registered entry with such a root would serve no workflow.
func checkRoot(root string) error {
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("the repo root %q is not an absolute path.", root)
	}
	if fi, err := os.Stat(filepath.Join(root, "workflows")); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s has no workflows folder, so it is not the horus repo. Run horus register inside the repo, or pass -root <repo folder>.", root)
	}
	return nil
}
