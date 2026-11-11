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
	"workflow": {"list, check, install or remove the model files of a workflow", func(args []string) int {
		return withSignal(func(ctx context.Context) int { return cli.Workflow(ctx, args, os.Stdout, os.Stderr, setupAt) })
	}},
	"download-model": {"download one model file with size and SHA-256 checks", func(args []string) int {
		return withSignal(func(ctx context.Context) int { return download.Main(ctx, args, os.Stdout, download.Options{}) })
	}},
	"install-comfyui": {"install the ComfyUI portable build, Windows only", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.InstallComfyUI(ctx, args, os.Stdout, os.Stderr, backendAt)
		})
	}},
	"start-comfyui": {"start ComfyUI and wait for its server, Windows only", func(args []string) int {
		return withSignal(func(ctx context.Context) int { return cli.StartComfyUI(ctx, args, os.Stdout, os.Stderr, backendAt) })
	}},
	"stop-comfyui": {"stop the ComfyUI that horus started, Windows only", func(args []string) int {
		return withSignal(func(ctx context.Context) int { return cli.StopComfyUI(ctx, args, os.Stdout, os.Stderr, backendAt) })
	}},
	"install-comfy-mcp": {"install comfy-mcp and comfy-cli in their own venv", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.InstallComfyMCP(ctx, args, os.Stdout, os.Stderr, platform.Programs{})
		})
	}},
	"register": {"register horus mcp with Claude Code", func(args []string) int {
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
	"errors": {"print the table of error codes (-json for JSON)", func(args []string) int {
		return cli.Errors(args, os.Stdout, os.Stderr)
	}},
	"mcp": {"serve the 5 horus tools over stdio (MCP)", func(args []string) int {
		return withSignal(func(ctx context.Context) int {
			return cli.MCP(ctx, args, os.Stderr, func(ctx context.Context) error {
				return mcpserver.Run(ctx, newImages(""), newSetup(cli.SetupPlace{}, seams{}))
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
