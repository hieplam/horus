// horus install-comfyui, horus start-comfyui and horus stop-comfyui: the
// handlers of the ComfyUI install. They port the flags and the output of
// scripts/install-comfyui.ps1 and scripts/start-comfyui.ps1. The work is in
// the backend (internal/backend), which cmd/horus gives them.
//
// Exit code 0 = done, 1 = the step failed, 2 = a flag that does not parse,
// or a step that this OS does not have yet.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

// Backend is what the three commands need from the ComfyUI install.
// cmd/horus gives them the backend of internal/backend.
type Backend interface {
	Install(ctx context.Context, out io.Writer) error
	Start(ctx context.Context, out io.Writer) error
	Stop(ctx context.Context, out io.Writer) error
}

// BackendPlace is the install that the flags pick. An empty field means the
// setting or the default of the backend.
type BackendPlace struct {
	Dir         string        // -install-dir or -comfyui-dir
	ServerURL   string        // -server-url
	ArchivePath string        // -archive-path
	SevenZip    string        // -seven-zip
	KeepArchive bool          // -keep-archive
	Timeout     time.Duration // -timeout-seconds
}

const dirHelp = "(default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)"

// InstallComfyUI is horus install-comfyui.
func InstallComfyUI(ctx context.Context, args []string, out, errOut io.Writer, at func(BackendPlace) Backend) int {
	var place BackendPlace
	fs := flag.NewFlagSet("horus install-comfyui", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.StringVar(&place.Dir, "install-dir", "", "target folder "+dirHelp)
	fs.StringVar(&place.ArchivePath, "archive-path", "", "where to keep the downloaded .7z file during the install (default: next to -install-dir)")
	fs.StringVar(&place.SevenZip, "seven-zip", "", "full path to 7z.exe (default: search PATH, then Program Files\\7-Zip)")
	fs.BoolVar(&place.KeepArchive, "keep-archive", false, "do not delete the archive after a successful install")
	if code, done := parse(fs, args); done {
		return code
	}
	return finish("horus install-comfyui", at(place).Install(ctx, out), errOut)
}

// StartComfyUI is horus start-comfyui.
func StartComfyUI(ctx context.Context, args []string, out, errOut io.Writer, at func(BackendPlace) Backend) int {
	var place BackendPlace
	fs := flag.NewFlagSet("horus start-comfyui", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.StringVar(&place.Dir, "comfyui-dir", "", "the portable folder, with ComfyUI/ and python_embeded/ "+dirHelp)
	fs.StringVar(&place.ServerURL, "server-url", "", "address that the server must answer on; the port is taken from it (default: HORUS_COMFYUI_URL, then http://127.0.0.1:8188)")
	timeout := fs.Int("timeout-seconds", 300, "how long to wait for the server; the first start can take a few minutes")
	if code, done := parse(fs, args); done {
		return code
	}
	place.Timeout = time.Duration(*timeout) * time.Second
	return finish("horus start-comfyui", at(place).Start(ctx, out), errOut)
}

// StopComfyUI is horus stop-comfyui. It stops only a ComfyUI that horus
// started, with every process that it started.
func StopComfyUI(ctx context.Context, args []string, out, errOut io.Writer, at func(BackendPlace) Backend) int {
	var place BackendPlace
	fs := flag.NewFlagSet("horus stop-comfyui", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.StringVar(&place.Dir, "comfyui-dir", "", "the portable folder "+dirHelp)
	fs.StringVar(&place.ServerURL, "server-url", "", "the address of the ComfyUI to stop (default: HORUS_COMFYUI_URL, then http://127.0.0.1:8188)")
	if code, done := parse(fs, args); done {
		return code
	}
	return finish("horus stop-comfyui", at(place).Stop(ctx, out), errOut)
}

// parse reads the flags. done is true when the command ends with code: -h
// gives 0, a flag that does not parse gives 2.
func parse(fs *flag.FlagSet, args []string) (code int, done bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, true
		}
		return 2, true
	}
	return 0, false
}

// finish writes the error of the step and gives the exit code.
func finish(name string, err error, errOut io.Writer) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, platform.ErrNotSupported):
		fmt.Fprintln(errOut, name+":", err)
		return 2
	default:
		fmt.Fprintln(errOut, name+":", err)
		return 1
	}
}
