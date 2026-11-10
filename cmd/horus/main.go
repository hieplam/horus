// Command horus: one program for the horus MCP server and the setup commands.
// It replaces the Python package horus/ and the scripts in scripts/.
//
//	horus <command> [flags]
//
// Each command lives in its own file cmd_<name>.go and registers itself in
// init. A command that is not ported yet is one line in cmd_not_ported.go.
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
)

// command is one subcommand. run gets the arguments after the command name
// and returns the exit code.
type command struct {
	name    string
	summary string
	run     func(args []string) int
}

var commands = map[string]command{}

func register(name, summary string, run func(args []string) int) {
	if _, dup := commands[name]; dup {
		panic("horus: command registered twice: " + name)
	}
	commands[name] = command{name, summary, run}
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
