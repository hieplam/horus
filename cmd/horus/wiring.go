// The wiring: cmd/horus creates the adapters and gives them to the services.
package main

import (
	"context"
	"errors"
	"io"
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
	return generate.New(comfy.New(url), backend.New(backend.Place{URL: url}, backend.Archive{}))
}

// imagesAt is the image service for the -url flag of run-illustration.
func imagesAt(url string) cli.Images { return newImages(url) }

// newBackend makes the backend of this OS for the flags of a command. An
// empty field means the setting, read at each call.
func newBackend(p cli.BackendPlace) backend.Backend {
	return backend.New(backend.Place{Dir: p.Dir, URL: p.ServerURL, Timeout: p.Timeout},
		backend.Archive{ArchivePath: p.ArchivePath, SevenZip: p.SevenZip, KeepArchive: p.KeepArchive})
}

// backendAt is the backend for the flags of install-comfyui, start-comfyui
// and stop-comfyui.
func backendAt(p cli.BackendPlace) cli.Backend { return newBackend(p) }

// takesArchive tells install-comfyui if the backend of this OS takes the
// flags of the archive (the Windows portable build).
var takesArchive = backend.TakesArchive()

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
// request in, and the kind and the reason lines of a failure out.
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
		return false, &provision.DownloadError{Kind: kinds[de.Kind], Msg: de.Msg, Reasons: de.Reasons}
	}
	return res.Skipped, err
}

// doctorAt is the doctor service, and the setup service for the model
// check, for the flags of horus doctor. The model check reads the address of
// ComfyUI from the setting, as horus workflow check does without -server-url.
func doctorAt(p cli.DoctorPlace) (cli.Doctor, cli.Setup) {
	install := newBackend(cli.BackendPlace{Dir: p.ComfyUIDir, ServerURL: p.ServerURL})
	setup := setupAt(cli.SetupPlace{ComfyUIDir: p.ComfyUIDir, ModelsDir: p.ModelsDir, TextEncoderDir: p.TextEncoderDir})
	return doctor.New(comfy.New(p.ServerURL), install, platform.Programs{}), setup
}
