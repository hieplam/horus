// horus api and horus ready: the handlers of the flags of the stack (spec
// docs/specs/2026-11-19-job-queue.md, "Readiness"). cmd/horus gives each one
// the readiness of the 4 dependencies, and horus api its server.
//
// Exit codes: 0 = ready (horus ready) or stopped at a signal (horus api),
// 1 = not ready or a failure, 2 = a flag that does not parse.
package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/settings"
)

// Readiness gives the readiness report of horus in the stack. With only, it
// runs only the checks of those dependencies.
type Readiness func(ctx context.Context, only ...string) jobdata.Readiness

// Ready is horus ready: the 4 checks of the stack, one line for each. It
// exits 0 only when all pass. -only runs the check of one dependency: the
// health check of the container horus-worker is horus ready -only
// horus-worker.
func Ready(ctx context.Context, args []string, out, errOut io.Writer, ready Readiness) int {
	fs := newFlags("horus ready", errOut)
	only := fs.String("only", "", "run only the check of this dependency: "+strings.Join(jobdata.Dependencies, ", "))
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, "Usage: horus ready [-only <dependency>]")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Runs the 4 checks of horus in the stack, and prints one ok or FAIL line for each:")
		fmt.Fprintln(w, "  horus-db      the database answers and has the schema version of this program")
		fmt.Fprintln(w, "  comfyui       GET <HORUS_COMFYUI_URL>/system_stats answers 200 in 2 s")
		fmt.Fprintln(w, "  horus-worker  the worker wrote its heartbeat in the last 15 s and holds the worker lock")
		fmt.Fprintln(w, "  horus-api     horus api answers GET http://127.0.0.1:<HORUS_API_PORT>/livez")
		fmt.Fprintln(w, "Exit code 0 only when all pass. It needs HORUS_DATABASE_URL: run it in the stack,")
		fmt.Fprintln(w, "for example docker exec horus-api horus ready.")
		fmt.Fprintln(w, "")
		fs.PrintDefaults()
	}
	if code, done := parse(fs, args); done {
		return code
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errOut, "horus ready: no arguments allowed, got %q\n", fs.Args())
		return 2
	}
	if *only == "" {
		r := ready(ctx)
		fmt.Fprint(out, r.Text())
		if r.Ready {
			return 0
		}
		return 1
	}
	if !slices.Contains(jobdata.Dependencies, *only) {
		fmt.Fprintf(errOut, "horus ready: unknown dependency %q. The dependencies are: %s.\n", *only, strings.Join(jobdata.Dependencies, ", "))
		return 2
	}
	r := ready(ctx, *only)
	for _, c := range r.Checks {
		if c.Dependency == *only {
			fmt.Fprintln(out, c.Line())
			if c.Up {
				return 0
			}
		}
	}
	return 1
}

// API is horus api: the main process of the container horus-api. serve runs
// the HTTP server on the address listen until ctx ends, and writes its log
// lines to out.
func API(ctx context.Context, args []string, out, errOut io.Writer, serve func(ctx context.Context, listen string, out io.Writer) error) int {
	fs := newFlags("horus api", errOut)
	port, err := settings.APIPort()
	if err != nil {
		fmt.Fprintln(errOut, "horus api:", err)
		return 1
	}
	listen := fs.String("listen", "127.0.0.1:"+strconv.Itoa(port), "the address of the HTTP server. In the stack: 0.0.0.0:8190 in the container, and compose.yaml publishes it only on 127.0.0.1 of the host")
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, "Usage: horus api [-listen <host:port>]")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Serves the HTTP routes of horus in the stack, until Ctrl+C or a stop signal:")
		fmt.Fprintln(w, "  GET /readyz  the 4 checks of horus ready as JSON: 200 when all pass, else 503")
		fmt.Fprintln(w, "  GET /livez   200 while horus api serves")
		fmt.Fprintln(w, "It needs HORUS_DATABASE_URL: the job store and the broker in Postgres.")
		fmt.Fprintln(w, "")
		fs.PrintDefaults()
	}
	if code, done := parse(fs, args); done {
		return code
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errOut, "horus api: no arguments allowed, got %q\n", fs.Args())
		return 2
	}
	if err := serve(ctx, *listen, out); err != nil && ctx.Err() == nil {
		fmt.Fprintf(errOut, "horus api: %v\n", err)
		return 1
	}
	return 0
}
