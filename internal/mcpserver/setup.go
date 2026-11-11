// What the model tools need from the setup service internal/provision, and
// the mapping of its typed results to the horus error codes. Part of the port
// of horus/models.py.
package mcpserver

import (
	"context"
	"errors"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// Setup is what the tools list_workflows, describe_workflow, install_workflow
// and uninstall_workflow need from the setup service. cmd/horus gives the
// server a *provision.Service. The server calls it in its own process.
type Setup interface {
	List(ctx context.Context, workflowsDir string) ([]provision.Row, error)
	Check(ctx context.Context, req provision.Request) (*provision.Result, error)
	Install(ctx context.Context, req provision.Request, w provision.Watch) (*provision.Result, error)
	Remove(ctx context.Context, req provision.Request) (*provision.Result, error)
	// PartSize is the bytes of a download so far.
	PartSize(dest string) int64
}

// request is the request of one workflow, in the workflows folder of the
// settings.
func request(w *catalog.Workflow) provision.Request {
	return provision.Request{WorkflowsDir: settings.WorkflowsDir(), Workflow: w.Name}
}

// listStatus returns name -> row. A size check only, so it is fast. It runs
// to the end, also when the call is cancelled.
func listStatus(ctx context.Context, setup Setup) (map[string]provision.Row, error) {
	rows, err := setup.List(context.WithoutCancel(ctx), settings.WorkflowsDir())
	if err != nil {
		return nil, herr.New("INTERNAL", "horus workflow list failed", map[string]any{"output_tail": []string{"FAIL: " + err.Error()}})
	}
	out := make(map[string]provision.Row, len(rows))
	for _, r := range rows {
		out[r.Name] = r
	}
	return out, nil
}

func byStatus(items []provision.Item, statuses ...string) []provision.Item {
	var out []provision.Item
	for _, i := range items {
		for _, s := range statuses {
			if i.Status == s {
				out = append(out, i)
			}
		}
	}
	return out
}

func fileNames(items []provision.Item) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.File)
	}
	return out
}

// downloadError maps a FAILED download item to its error code, by the kind
// of the error of the downloader.
func downloadError(item provision.Item, w *catalog.Workflow) error {
	facts := func() map[string]any {
		return map[string]any{"file": item.File, "path": item.Path, "detail": item.Detail}
	}
	kind := provision.DownloadFailed
	var de *provision.DownloadError
	if errors.As(item.Err, &de) {
		kind = de.Kind
	}
	switch kind {
	case provision.DownloadUnauthorized:
		return herr.New("HF_TOKEN_INVALID", "Hugging Face refused the token (HTTP 401)", facts())
	case provision.DownloadForbidden:
		lic, err := w.License()
		if err != nil {
			return err
		}
		d := facts()
		d["license"], d["license_url"] = lic.Name, lic.URL
		return herr.New("LICENSE_NOT_ACCEPTED", "Hugging Face refused the download (HTTP 403)", d)
	case provision.DownloadDiskFull:
		return herr.New("DISK_FULL", "not enough free disk space for the download", facts())
	case provision.DownloadConflict:
		return herr.New("FILE_CONFLICT", "a different file with the same name is on disk", facts())
	}
	return herr.New("DOWNLOAD_FAILED", "the download did not finish", facts())
}

// checkError maps the items of a failed check to its error code, or nil.
func checkError(items []provision.Item) error {
	if bad := byStatus(items, provision.StatusNoNode); len(bad) > 0 {
		return herr.New("NODE_PACK_MISSING", "the backend lacks node classes of this workflow", map[string]any{"missing": fileNames(bad)})
	}
	if bad := append(byStatus(items, provision.StatusBadSize), byStatus(items, provision.StatusBadHash)...); len(bad) > 0 {
		return herr.New("FILE_CONFLICT", "a model file on disk differs from models.json", map[string]any{"path": bad[0].Path, "files": bad})
	}
	if bad := byStatus(items, provision.StatusNotSeen); len(bad) > 0 {
		return herr.New("BACKEND_RESTART_REQUIRED", "the backend does not list the new model files yet", map[string]any{"files": fileNames(bad)})
	}
	if bad := byStatus(items, provision.StatusMissing); len(bad) > 0 {
		return herr.New("WORKFLOW_NOT_INSTALLED", "model files are missing", map[string]any{"missing": fileNames(bad)})
	}
	if bad := byStatus(items, provision.StatusUnlisted); len(bad) > 0 {
		return herr.New("INTERNAL", "the workflow loads a model file that models.json does not list", map[string]any{"files": fileNames(bad)})
	}
	return nil
}

// installError maps a failed install to its error code.
func installError(res *provision.Result, err error, w *catalog.Workflow) error {
	if failed := byStatus(res.Items, provision.StatusFailed); len(failed) > 0 {
		return downloadError(failed[0], w)
	}
	if e := checkError(res.Items); e != nil {
		return e
	}
	if errors.Is(err, provision.ErrNoComfyUI) {
		return herr.New("BACKEND_NOT_INSTALLED", res.Message, nil)
	}
	return herr.New("INTERNAL", res.Message, map[string]any{"items": res.Items})
}

// removeError maps a failed remove to its error code.
func removeError(res *provision.Result) error {
	if bad := byStatus(res.Items, provision.StatusDelFailed); len(bad) > 0 {
		return herr.New("FILE_IN_USE", "some model files could not be deleted", map[string]any{"files": bad})
	}
	if refused := byStatus(res.Items, provision.StatusRefused); len(refused) > 0 {
		return herr.New("FILE_CONFLICT", "a file on disk differs from models.json, so horus does not delete it",
			map[string]any{"path": refused[0].Path, "files": refused})
	}
	return herr.New("INTERNAL", res.Message, map[string]any{"items": res.Items})
}
