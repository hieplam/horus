// The wiring: cmd/horus creates the adapters and gives them to the services.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/backend"
	"github.com/hieplam/horus/internal/cli"
	"github.com/hieplam/horus/internal/comfy"
	"github.com/hieplam/horus/internal/devtools"
	"github.com/hieplam/horus/internal/doctor"
	"github.com/hieplam/horus/internal/download"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/modelfiles"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/provision"
)

// newImages makes the image service on the ComfyUI adapter and the backend.
// An empty url means the HORUS_COMFYUI_URL setting, read at each request.
// The backend starts ComfyUI at the same address, in the HORUS_COMFYUI_DIR
// folder.
func newImages(url string) *generate.Service {
	return generate.New(comfy.New(url), backend.New(&backend.Portable{URL: url}))
}

// imagesAt is the image service for the -url flag of run-illustration.
func imagesAt(url string) cli.Images { return newImages(url) }

// newBackend makes the backend of this OS for the flags of a command. An
// empty field means the setting, read at each call.
func newBackend(p cli.BackendPlace) backend.Backend {
	return backend.New(&backend.Portable{Dir: p.Dir, URL: p.ServerURL, ArchivePath: p.ArchivePath,
		SevenZip: p.SevenZip, KeepArchive: p.KeepArchive, Timeout: p.Timeout})
}

// backendAt is the backend for the flags of install-comfyui, start-comfyui
// and stop-comfyui.
func backendAt(p cli.BackendPlace) cli.Backend { return newBackend(p) }

// installAt is the ComfyUI install for the flag -comfyui-dir of
// sync-workflows.
func installAt(dir string) devtools.Install { return newBackend(cli.BackendPlace{Dir: dir}) }

// seams are the test seams of the setup service. The zero value is the real
// behaviour.
type seams struct {
	download download.Options // the base options of each download
	now      func() time.Time // the time in the name of a backup of extra_model_paths.yaml
}

// newSetup makes the setup service on the model files, the downloader, the
// ComfyUI and the install of place. An empty field of place means the
// setting, read at each call.
func newSetup(place cli.SetupPlace, s seams) *provision.Service {
	files := &modelfiles.Store{ComfyUIDir: place.ComfyUIDir, ModelsDir: place.ModelsDir, TextEncoderDir: place.TextEncoderDir, Now: s.now}
	install := newBackend(cli.BackendPlace{Dir: place.ComfyUIDir, ServerURL: place.ServerURL})
	return provision.New(files, downloader{s.download}, comfy.New(place.ServerURL), install, place.ServerURL)
}

// setupAt is the setup service for the flags of horus workflow.
func setupAt(place cli.SetupPlace) cli.Setup { return newSetup(place, seams{}) }

// setupStepsAt is the setup service for the flags of horus setup.
func setupStepsAt(place cli.SetupPlace) cli.SetupSteps { return newSetup(place, seams{}) }

// downloader gives internal/provision the downloader of internal/download.
// The service cannot name the types of an adapter, so this glue copies the
// request in and the kind of a failure out.
type downloader struct{ base download.Options }

// kinds maps the kind of a failed download to the kind of the service.
var kinds = map[download.Kind]provision.DownloadKind{
	download.KindFailed:       provision.DownloadFailed,
	download.KindUnauthorized: provision.DownloadUnauthorized,
	download.KindForbidden:    provision.DownloadForbidden,
	download.KindDiskFull:     provision.DownloadDiskFull,
	download.KindConflict:     provision.DownloadConflict,
	download.KindBadHash:      provision.DownloadBadHash,
}

func (d downloader) Download(ctx context.Context, f provision.Fetch, out io.Writer) (bool, error) {
	o := d.base
	o.URL, o.Dest, o.ExpectedBytes, o.SHA256, o.Out = f.URL, f.Dest, f.Bytes, f.SHA256, out
	res, err := download.Download(ctx, o)
	var de *download.Error
	if errors.As(err, &de) {
		return false, &provision.DownloadError{Kind: kinds[de.Kind], Msg: de.Msg}
	}
	return res.Skipped, err
}

// doctorAt is the doctor service for the flags of horus doctor.
func doctorAt(p cli.DoctorPlace) cli.Doctor {
	install := newBackend(cli.BackendPlace{Dir: p.ComfyUIDir, ServerURL: p.ServerURL})
	models := workflowCheck{place: cli.SetupPlace{ComfyUIDir: p.ComfyUIDir, ModelsDir: p.ModelsDir, TextEncoderDir: p.TextEncoderDir}, setupAt: setupAt}
	return doctor.New(comfy.New(p.ServerURL), install, platform.Programs{}, models)
}

// workflowCheck gives internal/doctor the model check: the command horus
// workflow check, in this process, with its text lines. It is the check
// that the doctor ran as a child process "horus workflow check" before.
type workflowCheck struct {
	place   cli.SetupPlace
	setupAt func(cli.SetupPlace) cli.Setup
}

func (w workflowCheck) Check(ctx context.Context, workflow string, quick bool) ([]string, bool, error) {
	args := []string{"check", workflow}
	if quick {
		args = append(args, "-quick")
	}
	var buf bytes.Buffer
	code := cli.Workflow(ctx, args, &buf, &buf, func(cli.SetupPlace) cli.Setup { return w.setupAt(w.place) })
	var lines []string
	sc := bufio.NewScanner(&buf)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	return lines, code == 0, nil
}
