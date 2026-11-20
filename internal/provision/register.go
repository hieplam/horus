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
//
// With the stack (horus register -docker), the client starts horus mcp in the
// container horus-api: "claude mcp add --scope user horus -- <path of docker>
// exec -i horus-api horus mcp", with no HORUS_ROOT (spec
// docs/specs/2026-11-19-job-queue.md, "Two ways in").
//
// Over HTTP (horus register -http), the client connects to /mcp of horus api
// in the stack, with the bearer token of the variable HORUS_API_TOKEN. The
// config of the client names the variable, never the token (ADR 0040):
//
//	claude mcp add --scope user --transport http horus <url> --header "Authorization: Bearer ${HORUS_API_TOKEN}"
//	codex mcp add horus --url <url> --bearer-token-env-var HORUS_API_TOKEN
//
// Claude Code reads the variable of its own environment in the header when it
// connects, and Codex reads the variable that bearer_token_env_var names.
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
	// Container is the name of the container horus-api of the stack. Not
	// empty: the client starts "docker exec -i <Container> horus mcp", and
	// Register uses neither HorusPath nor Root.
	Container string
	// Docker is the path of the docker program. Empty: find it on the PATH.
	Docker string
	// URL is the address of /mcp of horus api, for example
	// http://127.0.0.1:8190/mcp. Not empty: the client connects over HTTP
	// with the bearer token of the variable TokenVar, and Register uses
	// neither HorusPath, Root nor Container.
	URL string
	// TokenVar is the name of the variable of the bearer token
	// (HORUS_API_TOKEN). Register writes the name into the config of the
	// client, never the value.
	TokenVar string
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
	// httpArgs is the "mcp add" argument list of a server over HTTP at url,
	// with the bearer token of the variable tokenVar.
	httpArgs func(url, tokenVar string) []string
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
			},
			httpArgs: func(url, tokenVar string) []string {
				return []string{"mcp", "add", "--scope", "user", "--transport", "http", "horus", url,
					"--header", "Authorization: Bearer ${" + tokenVar + "}"}
			}}, nil
	case ClientCodex:
		return clientInfo{name: "codex", title: "Codex", session: "Codex session",
			addArgs: func(root string) []string {
				a := []string{"mcp", "add", "horus"}
				if root != "" {
					a = append(a, "--env", "HORUS_ROOT="+root)
				}
				return a
			},
			httpArgs: func(url, tokenVar string) []string {
				return []string{"mcp", "add", "horus", "--url", url, "--bearer-token-env-var", tokenVar}
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
	command := []string{o.HorusPath, "mcp"}
	if o.URL != "" {
		if o.TokenVar == "" {
			return fmt.Errorf("the name of the variable of the bearer token is empty (a bug of the caller)")
		}
		o.Root, o.Container = "", ""
	}
	if o.Container != "" {
		o.Root = ""
		if o.Docker == "" {
			p, err := programs.LookPath("docker")
			if err != nil {
				return fmt.Errorf("The docker command was not found. Install Docker, start the stack (docs/setup.md#run-horus-as-a-stack), then run horus register -docker again.")
			}
			o.Docker = p
		}
		command = []string{o.Docker, "exec", "-i", o.Container, "horus", "mcp"}
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
	args := append(append(c.addArgs(o.Root), "--"), command...)
	if o.URL != "" {
		args = c.httpArgs(o.URL, o.TokenVar)
	}
	code, err = programs.Run(ctx, platform.Command{Path: o.Program, Args: args, Stdout: o.Stdout, Stderr: o.Stderr})
	if err != nil {
		return fmt.Errorf("%s mcp add failed: %w", c.name, err)
	}
	if code != 0 {
		return fmt.Errorf("%s mcp add failed (exit code %d).", c.name, code)
	}
	via := ""
	if o.Container != "" {
		via = " The server is horus mcp in the container " + o.Container + " (docker exec -i " + o.Container + " horus mcp)."
	}
	if o.URL != "" {
		via = " The server is /mcp of horus api at " + o.URL + ", with the bearer token of the variable " + o.TokenVar +
			". The config names the variable, not the token: set " + o.TokenVar + " in the environment of " + c.title +
			" to the value in the .env of the stack."
	}
	if c.name == "claude" {
		fmt.Fprintln(out, "OK: registered for Claude Code in user scope."+via+" Start a new Claude Code session to see the tools.")
	} else {
		fmt.Fprintln(out, "OK: registered for Codex."+via+" Start a new Codex session to see the tools.")
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
