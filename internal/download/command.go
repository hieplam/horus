// The command horus download-model: its flags, its output lines and its
// exit codes.
package download

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
)

// Main runs horus download-model with args (the arguments after the command
// name). It writes the progress lines to out and returns the exit
// code: 0 = the file is present and verified, 1 = anything else, 2 = a flag
// that does not parse. base gives the test seams; its URL, Dest, sizes and
// names come from the flags.
func Main(ctx context.Context, args []string, out io.Writer, base Options) int {
	fs := flag.NewFlagSet("horus download-model", flag.ContinueOnError)
	fs.SetOutput(out)
	o := base
	fs.StringVar(&o.URL, "url", "", "https address of the file")
	fs.StringVar(&o.Dest, "destination", "", "full path of the final file; the folder is made when needed")
	fs.Int64Var(&o.ExpectedBytes, "expected-bytes", 0, "exact size of the file in bytes")
	fs.StringVar(&o.SHA256, "sha256", "", "expected SHA-256 hash, 64 hexadecimal characters")
	fs.StringVar(&o.tokenVar, "token-env-var", "HF_TOKEN", "name of the variable that holds the access token (environment, then .env)")
	fs.IntVar(&o.MaxAttempts, "max-attempts", 4, "number of download attempts")
	fs.Usage = func() {
		fmt.Fprintln(out, "Usage: horus download-model -url <https address> -destination <file> -expected-bytes <n> -sha256 <hex>")
		fmt.Fprintln(out, "Downloads one file and keeps it only if its size and SHA-256 hash are correct.")
		fmt.Fprintln(out, "Exit code 0 = file is present and verified. Exit code 1 = anything else.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	o.Out = out
	var missing []string
	for _, f := range []struct {
		name string
		set  bool
	}{{"-url", o.URL != ""}, {"-destination", o.Dest != ""}, {"-expected-bytes", o.ExpectedBytes != 0}, {"-sha256", o.SHA256 != ""}} {
		if !f.set {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(out, "FAIL: missing flag(s): %v\n", missing)
		return 1
	}
	if _, err := Download(ctx, o); err != nil {
		fmt.Fprintf(out, "FAIL: %v\n", err)
		return 1
	}
	return 0
}
