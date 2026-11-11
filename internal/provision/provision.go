// Package provision is the service that sets up the model files of a
// workflow: list, check, install and remove. It ports the rules of
// scripts/workflow.ps1. The command horus workflow (internal/cli) and the
// MCP tools (internal/mcpserver) call it in their own process.
//
// It decides: which file is missing, which file to download and where, which
// folder to add to extra_model_paths.yaml, which file may be deleted, and
// which status each item gets. It does the input and output through the
// interfaces below: the model files (ModelFiles), the downloader (Downloader),
// the running ComfyUI (ComfyUI) and the ComfyUI install (Backend). It reads
// the workflows folder through internal/catalog. It knows no flag, no MCP and no output text: a handler
// writes the lines from the typed Result.
package provision

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hieplam/horus/internal/graph"
	"github.com/hieplam/horus/internal/settings"
)

// ModelFiles is what the service needs from the model files on the disk.
// cmd/horus gives it a *modelfiles.Store. A test gives it a fake.
type ModelFiles interface {
	// Dirs gives the folders where a model file of this folder is looked
	// for, in order, as full paths. A new download goes to the first one.
	Dirs(folder string) []string
	// Own gives ComfyUI's own models folder for this folder (a full path).
	Own(folder string) string
	// Mapped gives the folders that extra_model_paths.yaml maps to this folder.
	Mapped(folder string) []string
	// Map adds each pair {folder, dir} to extra_model_paths.yaml, after one
	// backup of the file.
	Map(pairs [][2]string) error
	// Size gives the size of the file at path, or -1 when no file is there.
	Size(path string) int64
	// SHA256 gives the SHA-256 of a file, in lower-case hexadecimal.
	SHA256(path string) (string, error)
	// Delete deletes a file, also a read-only file.
	Delete(path string) error
}

// Fetch is one file to download.
type Fetch struct {
	URL    string // https address
	Dest   string // path of the final file
	Bytes  int64  // exact size
	SHA256 string // expected SHA-256
}

// Downloader downloads one file with resume, size check and SHA-256 check.
// It writes its text lines to out. skipped is true when the file was
// already present and verified. A failure is a *DownloadError. The end of
// ctx stops the download, and the partial file stays for the next run.
// cmd/horus gives the service the downloader of internal/download.
type Downloader interface {
	Download(ctx context.Context, f Fetch, out io.Writer) (skipped bool, err error)
}

// DownloadKind is why a download failed.
type DownloadKind int

// The kinds of a failed download.
const (
	DownloadFailed       DownloadKind = iota // it did not finish, or an argument is wrong
	DownloadUnauthorized                     // HTTP 401: the token is missing or wrong
	DownloadForbidden                        // HTTP 403: the license is not accepted
	DownloadDiskFull                         // not enough free space
	DownloadConflict                         // a different file with the same name is on disk
	DownloadBadHash                          // the downloaded bytes have a different SHA-256
)

// DownloadError is a failed download. Its text is the text after "FAIL: ".
type DownloadError struct {
	Kind DownloadKind
	Msg  string
}

func (e *DownloadError) Error() string { return e.Msg }

// ComfyUI is what the check needs from the running ComfyUI: its
// /object_info. An error means that no answer came. cmd/horus gives the
// service a *comfy.Client.
type ComfyUI interface {
	ObjectInfo(ctx context.Context) (graph.ObjectInfo, error)
}

// Backend is what the service needs from the ComfyUI install. cmd/horus
// gives the service the backend of internal/backend, the one place that
// knows the layout of the install.
type Backend interface {
	// Installed gives the install folder, and reports if ComfyUI is
	// installed there.
	Installed() (dir string, ok bool)
}

// Service sets up the model files of the workflows.
type Service struct {
	files     ModelFiles
	dl        Downloader
	comfy     ComfyUI
	backend   Backend
	serverURL string
}

// New makes the service. serverURL is the address of comfy, for the text of
// a check that gets no answer; "" means the HORUS_COMFYUI_URL setting.
func New(files ModelFiles, dl Downloader, comfy ComfyUI, backend Backend, serverURL string) *Service {
	return &Service{files: files, dl: dl, comfy: comfy, backend: backend, serverURL: serverURL}
}

func (s *Service) server() string {
	if s.serverURL != "" {
		return trimSlash(s.serverURL)
	}
	return settings.ComfyURL()
}

// Request names one workflow and the options of an action.
type Request struct {
	WorkflowsDir string   // the folder of the workflows
	Workflow     string   // the name of the workflow
	ModelsJSON   string   // a models.json to use; "" = <WorkflowsDir>/<Workflow>/models.json
	Only         []string // only these roles or file names
	Quick        bool     // check: the size only, no SHA-256
	Yes          bool     // remove: delete the files; false = only the plan
	Force        bool     // remove: also delete a file that another workflow lists
}

// Watch sees an install while it runs. Each field can be nil.
type Watch struct {
	// Started: the install starts with this many files, under this license.
	Started func(license string, files int)
	// Downloading: the download of one file to dest starts.
	Downloading func(role, dest string)
	// Out gets the text lines of the downloader.
	Out io.Writer
	// Failed: the download of the file failed.
	Failed func(err error)
}

// ErrNoComfyUI is in the chain of the error of an install that found no
// ComfyUI in its folder.
var ErrNoComfyUI = errors.New("ComfyUI was not found")

// failure is an error whose text is the message of the result.
type failure struct {
	msg string
	is  error
}

func (f failure) Error() string { return f.msg }
func (f failure) Unwrap() error { return f.is }

func failf(format string, a ...any) error { return failure{msg: fmt.Sprintf(format, a...)} }

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
