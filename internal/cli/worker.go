// horus worker: the handler of the flags of the one worker of the job queue.
// The loop is in internal/worker. cmd/horus connects the job store and the
// broker of HORUS_DATABASE_URL and gives serve.
//
// Exit code 0 = the worker stopped at a signal, 1 = the worker failed or
// another worker runs, 2 = a flag that does not parse.
package cli

import (
	"context"
	"fmt"
	"io"
)

// Worker is horus worker. serve runs the worker until ctx ends, and writes its
// log lines to out.
func Worker(ctx context.Context, args []string, out, errOut io.Writer, serve func(ctx context.Context, out io.Writer) error) int {
	fs := newFlags("horus worker", errOut)
	fs.Usage = func() {
		w := fs.Output()
		fmt.Fprintln(w, "Usage: horus worker")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Runs the jobs of the job queue on ComfyUI, until Ctrl+C or a stop signal.")
		fmt.Fprintln(w, "It needs HORUS_DATABASE_URL: the job store and the broker in Postgres. It applies")
		fmt.Fprintln(w, "the schema horus at its start. Only one worker runs: a second horus worker prints")
		fmt.Fprintln(w, "the first one and exits with 1.")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Settings: HORUS_DATABASE_URL, HORUS_COMFYUI_URL, HORUS_WORKER_CONCURRENCY (default 1),")
		fmt.Fprintln(w, "HORUS_GENERATE_TIMEOUT (default 1800), HORUS_JOB_RETENTION_DAYS (default 30).")
		fmt.Fprintln(w, "The worker never starts ComfyUI, and takes no job while ComfyUI does not answer.")
	}
	if code, done := parse(fs, args); done {
		return code
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errOut, "horus worker: no arguments allowed, got %q\n", fs.Args())
		return 2
	}
	if err := serve(ctx, out); err != nil && ctx.Err() == nil {
		fmt.Fprintf(errOut, "horus worker: %v\n", err)
		return 1
	}
	return 0
}
