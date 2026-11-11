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
	"github.com/hieplam/horus/internal/download"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/modelfiles"
	"github.com/hieplam/horus/internal/provision"
)

// newImages makes the image service on the ComfyUI adapter and the backend.
// An empty url means the HORUS_COMFYUI_URL setting, read at each request.
// The backend starts ComfyUI at the same address, in the HORUS_COMFYUI_DIR
// folder.
func newImages(url string) *generate.Service {
	return generate.New(comfy.New(url), backend.New(&backend.Portable{URL: url}))
}

// newBackend makes the backend of this OS for the flags of a command. An
// empty field means the setting, read at each call.
func newBackend(p cli.BackendPlace) backend.Backend {
	return backend.New(&backend.Portable{Dir: p.Dir, URL: p.ServerURL, ArchivePath: p.ArchivePath,
		SevenZip: p.SevenZip, KeepArchive: p.KeepArchive, Timeout: p.Timeout})
}

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
	install := backend.New(&backend.Portable{Dir: place.ComfyUIDir, URL: place.ServerURL})
	return provision.New(files, downloader{s.download}, comfy.New(place.ServerURL), install, place.ServerURL)
}

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
