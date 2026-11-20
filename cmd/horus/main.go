// Command horus: one program for the horus MCP server and the setup commands.
//
//	horus <command> [flags]
//
// This package holds the command table (this file) and the wiring
// (wiring.go). The handler of each command is in internal/cli, except
// download-model, whose flags name the token variable and so stay in
// internal/download.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"

	"github.com/hieplam/horus/internal/cli"
	"github.com/hieplam/horus/internal/download"
	"github.com/hieplam/horus/internal/mcpserver"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// command is one subcommand. run gets the arguments after the command name
// and returns the exit code.
type command struct {
	summary string
	run     func(args []string) int
}

// commands is the command table. check-setup and doctor are one command with
// two names (spec, "Command names").
var commands = map[string]command{
	"doctor": {"one PASS or FAIL line for each check of the setup", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.RunDoctor(ctx, "doctor", args, os.Stdout, os.Stderr, doctorAt)
		})
	}},
	"check-setup": {"one PASS or FAIL line for each check of the setup", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.RunDoctor(ctx, "check-setup", args, os.Stdout, os.Stderr, doctorAt)
		})
	}},
	"setup": {"print what this machine needs for a workflow; -yes installs it", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.RunSetup(ctx, args, os.Stdout, os.Stderr, setupStepsAt)
		})
	}},
	"uninstall": {"print what horus installed; -yes removes it (-models, -mcp: one part)", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.RunUninstall(ctx, args, os.Stdout, os.Stderr, uninstallAt)
		})
	}},
	"workflow": {"list, check, install or remove the model files of a workflow", func(args []string) int {
		return withSignal(func(ctx context.Context) int { return cli.Workflow(ctx, args, os.Stdout, os.Stderr, setupAt) })
	}},
	"download-model": {"download one model file with size and SHA-256 checks", func(args []string) int {
		return withSignal(func(ctx context.Context) int { return download.Main(ctx, args, os.Stdout, download.Options{}) })
	}},
	"install-comfyui": {"install ComfyUI: the portable build on Windows, comfy-cli on macOS", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.InstallComfyUI(ctx, args, os.Stdout, os.Stderr, backendAt, takesArchive)
		})
	}},
	"start-comfyui": {"start ComfyUI and wait for its server", func(args []string) int {
		return withSignal(func(ctx context.Context) int { return cli.StartComfyUI(ctx, args, os.Stdout, os.Stderr, backendAt) })
	}},
	"stop-comfyui": {"stop the ComfyUI that horus started", func(args []string) int {
		return withSignal(func(ctx context.Context) int { return cli.StopComfyUI(ctx, args, os.Stdout, os.Stderr, backendAt) })
	}},
	"install-comfy-mcp": {"install comfy-mcp and comfy-cli in their own venv", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.InstallComfyMCP(ctx, args, os.Stdout, os.Stderr, platform.Programs{})
		})
	}},
	"comfy-mcp": {"run the comfy MCP server (comfy-mcp) over stdio, for .mcp.json", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.ComfyMCP(ctx, args, os.Stdin, os.Stdout, os.Stderr, platform.Programs{})
		})
	}},
	"register": {"register horus mcp with Claude Code or Codex (-docker or -http: horus in the stack)", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.Register(ctx, args, os.Stdout, os.Stderr, platform.Programs{})
		})
	}},
	"new-workflow": {"make a new workflow folder from the template", func(args []string) int {
		return cli.NewWorkflow(args, os.Stdout, os.Stderr)
	}},
	"sync-workflows": {"copy the workflows into the ComfyUI page", func(args []string) int {
		return cli.SyncWorkflows(args, os.Stdout, os.Stderr, installAt)
	}},
	"run-illustration": {"make one image without MCP", func(args []string) int {
		return cli.RunIllustration(args, os.Stdout, os.Stderr, imagesAt)
	}},
	"version": {"print the version of this program", func(args []string) int {
		return cli.Version(args, os.Stdout, os.Stderr)
	}},
	"errors": {"print the table of error codes (-json for JSON)", func(args []string) int {
		return cli.Errors(args, os.Stdout, os.Stderr)
	}},
	"worker": {"run the jobs of the job queue on ComfyUI (needs HORUS_DATABASE_URL)", func(args []string) int {
		return withStop(func(ctx context.Context) int { return cli.Worker(ctx, args, os.Stdout, os.Stderr, runWorker) })
	}},
	"api": {"serve /mcp, the Horus UI, /readyz and /livez of horus in the stack (needs HORUS_DATABASE_URL)", func(args []string) int {
		return withStop(func(ctx context.Context) int { return cli.API(ctx, args, os.Stdout, os.Stderr, runAPI) })
	}},
	"stack-mounts": {"write compose.override.yaml: the shared folders of the stack (HORUS_SHARED_DIRS)", func(args []string) int {
		return cli.StackMounts(args, os.Stdout, os.Stderr)
	}},
	"ready": {"the 4 checks of horus in the stack, one line each; exit 0 when all pass", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			stack, done := stackOf(ctx, localAPI())
			defer done()
			return cli.Ready(ctx, args, os.Stdout, os.Stderr, stack.Ready)
		})
	}},
	"mcp": {"serve the 6 horus tools over stdio (MCP)", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			if settings.DatabaseURL() == "" {
				return cli.MCP(ctx, args, os.Stderr, nil, func(ctx context.Context) error {
					speech := newSpeech(ctx)
					defer speech.Close() // the speech worker ends with horus mcp
					return mcpserver.Run(ctx, newImages(""), newSetup(cli.SetupPlace{}, seams{}), speech, mcpserver.Options{})
				})
			}
			// In the stack (HORUS_DATABASE_URL is set), the session starts
			// only when horus is ready, each call checks the readiness, and
			// generate_image goes through the job queue.
			opts, done, err := stackMCP(ctx)
			if err != nil {
				fmt.Fprintln(os.Stderr, "horus mcp:", err)
				return 1
			}
			defer done()
			return cli.MCP(ctx, args, os.Stderr, cli.Readiness(opts.Ready), func(ctx context.Context) error {
				return mcpserver.Run(ctx, stackImages(), newSetup(cli.SetupPlace{}, seams{}), nil, opts)
			})
		})
	}},
}

// withSignal runs f with a context that Ctrl+C ends.
func withSignal(f func(ctx context.Context) int) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return f(ctx)
}

// withStop runs f with a context that Ctrl+C or a stop signal of the OS ends
// (SIGTERM of docker stop on Linux and macOS).
func withStop(f func(ctx context.Context) int) int {
	ctx, stop := signal.NotifyContext(context.Background(), platform.StopSignals...)
	defer stop()
	return f(ctx)
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: horus <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	names := make([]string, 0, len(commands))
	for n := range commands {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "  %-19s %s\n", n, commands[n].summary)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run horus <command> -h for the flags of a command.")
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "-h", "-help", "--help", "help":
		usage(os.Stdout)
		return 0
	}
	c, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "horus: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 2
	}
	return c.run(args[1:])
}

func main() { os.Exit(run(os.Args[1:])) }
