// Tests of the setup service on fakes of its adapters. The service reads a
// workflows folder in a temporary folder. The tests with the real adapters
// are in cmd/horus, and tests/workflow_parity.py compares the output of
// horus workflow with the golden files.
package provision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/graph"
)

// memFiles is a fake of the model files: every model file of folder F is in
// <root>/F, and the files are in memory.
type memFiles struct {
	root    string
	files   map[string][]byte
	mapped  [][2]string
	deleted []string
}

func newMemFiles() *memFiles {
	return &memFiles{root: filepath.Join(string(filepath.Separator), "m"), files: map[string][]byte{}}
}

func (m *memFiles) Installed() (string, bool) { return m.root, true }
func (m *memFiles) Install(context.Context, io.Writer) error {
	return errors.New("this test installs no ComfyUI")
}
func (m *memFiles) Dirs(folder string) []string { return []string{filepath.Join(m.root, folder)} }
func (m *memFiles) Own(folder string) string    { return filepath.Join(m.root, folder) }
func (m *memFiles) Mapped(string) []string      { return nil }
func (m *memFiles) Map(p [][2]string) error     { m.mapped = append(m.mapped, p...); return nil }
func (m *memFiles) Delete(path string) error {
	delete(m.files, path)
	m.deleted = append(m.deleted, path)
	return nil
}
func (m *memFiles) Size(path string) int64 {
	if b, ok := m.files[path]; ok {
		return int64(len(b))
	}
	return -1
}
func (m *memFiles) SHA256(path string) (string, error) { return sum(m.files[path]), nil }

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// fakeDownloader runs fn for each download.
type fakeDownloader struct {
	calls []Fetch
	fn    func(ctx context.Context, f Fetch, out io.Writer) (bool, error)
}

func (d *fakeDownloader) Download(ctx context.Context, f Fetch, out io.Writer) (bool, error) {
	d.calls = append(d.calls, f)
	return d.fn(ctx, f, out)
}

// fakeComfy answers /object_info with info, or with err.
type fakeComfy struct {
	info graph.ObjectInfo
	err  error
}

func (c fakeComfy) ObjectInfo(context.Context) (graph.ObjectInfo, error) { return c.info, c.err }

var (
	vaeData = []byte("stand-in vae")
	teData  = []byte("stand-in text encoder")
)

// workflows writes a workflows folder with the workflow "demo" (two model
// files) and returns it.
func workflows(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, v any) {
		b, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("models.json", map[string]any{"repo": "org/repo", "revision": "abc", "license": "test license", "files": []any{
		map[string]any{"role": "vae", "repo_path": "vae/demo-vae.safetensors", "folder": "vae", "bytes": len(vaeData), "sha256": sum(vaeData)},
		map[string]any{"role": "text_encoder", "repo_path": "te/demo-te.safetensors", "folder": "text_encoders", "bytes": len(teData), "sha256": sum(teData)},
	}})
	write("workflow.json", map[string]any{"nodes": []any{
		map[string]any{"type": "VAELoader", "widgets_values": []any{"demo-vae.safetensors"}},
		map[string]any{"type": "CLIPLoader", "widgets_values": []any{"demo-te.safetensors"}},
	}})
	return root
}

func statuses(res *Result) string {
	var s []string
	for _, it := range res.Items {
		s = append(s, it.Status+" "+it.Kind+" "+it.Role)
	}
	return strings.Join(s, ", ")
}

func TestRowState(t *testing.T) {
	for _, c := range []struct {
		row  *Row
		want string
	}{
		{&Row{Files: 3, Present: 3}, "installed"},
		{&Row{Files: 3, Present: 2}, "partially_installed"},
		{&Row{Files: 2, Present: 0}, "not_installed"},
		{&Row{Files: 0, Present: 0}, "not_installed"},
		{nil, "not_installed"},
	} {
		if got := c.row.State(); got != c.want {
			t.Errorf("%+v: %s, want %s", c.row, got, c.want)
		}
	}
}

func TestRoundGBAsTheScriptRoundsIt(t *testing.T) {
	// The values of [math]::Round(x, 2) of workflow.ps1.
	for b, want := range map[int64]float64{0: 0, 5_827_874_779: 5.83, 1_234_567_890_123: 1234.57, 999_995_000_000: 1000, 1_005_000_000: 1, 2_675_000_000: 2.68} {
		if got := roundGB(b); got != want {
			t.Errorf("roundGB(%d) = %v, want %v", b, got, want)
		}
	}
}

func TestPartSize(t *testing.T) {
	files := newMemFiles()
	s := New(files, nil, nil, files, "")
	dest := filepath.Join(files.root, "f.bin")
	if s.PartSize("") != 0 || s.PartSize(dest) != 0 {
		t.Error("no file must be 0")
	}
	files.files[dest] = make([]byte, 7)
	if s.PartSize(dest) != 7 {
		t.Error("the file itself")
	}
	files.files[dest+".part"] = make([]byte, 3)
	if s.PartSize(dest) != 3 {
		t.Error("the .part file comes first")
	}
}

// TestListCountsTheFilesOnDisk: a size check only.
func TestListCountsTheFilesOnDisk(t *testing.T) {
	dir := workflows(t)
	files := newMemFiles()
	files.files[filepath.Join(files.root, "vae", "demo-vae.safetensors")] = vaeData
	rows, err := New(files, nil, nil, files, "").List(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "demo" || rows[0].Present != 1 || rows[0].Files != 2 ||
		rows[0].Installed != "no (1 of 2 files)" || rows[0].State() != "partially_installed" || rows[0].Roles != "vae, text_encoder" {
		t.Fatalf("rows %+v", rows)
	}
	if _, err := New(files, nil, nil, files, "").List(context.Background(), filepath.Join(dir, "nope")); err == nil ||
		!strings.HasPrefix(err.Error(), "No workflows folder: ") {
		t.Errorf("a missing folder: %v", err)
	}
}

// TestInstallDownloadsEachFileAndKeepsTheKindOfAFailure: the items of an
// install, the lines of a failed download in its detail, and its typed
// error.
func TestInstallDownloadsEachFileAndKeepsTheKindOfAFailure(t *testing.T) {
	dir := workflows(t)
	files := newMemFiles()
	dl := &fakeDownloader{fn: func(_ context.Context, f Fetch, out io.Writer) (bool, error) {
		if strings.HasSuffix(f.Dest, "demo-te.safetensors") {
			fmt.Fprint(out, "Downloading demo-te.safetensors\nAttempt 1 of 4 did not finish (the server returned error: 401, 0 of 21 bytes).\n")
			return false, &DownloadError{Kind: DownloadUnauthorized, Msg: "Download did not finish after 4 attempts."}
		}
		files.files[f.Dest] = vaeData
		return false, nil
	}}
	var started, downloading, failed []string
	res, err := New(files, dl, fakeComfy{}, files, "").Install(context.Background(), Request{WorkflowsDir: dir, Workflow: "demo"}, Watch{
		Started:     func(license string, n int) { started = append(started, fmt.Sprint(license, n)) },
		Downloading: func(role, dest string) { downloading = append(downloading, role+" "+filepath.Base(dest)) },
		Failed:      func(err error) { failed = append(failed, err.Error()) },
	})
	if err == nil || err.Error() != "1 of 2 file(s) are not ready." || res.OK || res.Message != err.Error() {
		t.Fatalf("err %v, result %+v", err, res)
	}
	if got := statuses(res); got != "DOWNLOADED download vae, FAILED download text_encoder" {
		t.Errorf("items %s", got)
	}
	te := res.Items[1]
	if te.Detail != "Attempt 1 of 4 did not finish (the server returned error: 401, 0 of 21 bytes). | FAIL: Download did not finish after 4 attempts." {
		t.Errorf("detail %q", te.Detail)
	}
	var de *DownloadError
	if !errors.As(te.Err, &de) || de.Kind != DownloadUnauthorized {
		t.Errorf("the error of the item: %v", te.Err)
	}
	if strings.Join(started, "|") != "test license2" || strings.Join(downloading, "|") != "vae demo-vae.safetensors|text_encoder demo-te.safetensors" ||
		strings.Join(failed, "|") != "Download did not finish after 4 attempts." {
		t.Errorf("watch: %q %q %q", started, downloading, failed)
	}
	if res.Checked {
		t.Error("a failed download must not run the check")
	}
}

// TestInstallStopsWhenTheContextEnds: a cancel during the first download
// starts no second download and maps no folder.
func TestInstallStopsWhenTheContextEnds(t *testing.T) {
	dir := workflows(t)
	files := newMemFiles()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dl := &fakeDownloader{fn: func(ctx context.Context, f Fetch, out io.Writer) (bool, error) {
		files.files[f.Dest+".part"] = []byte("part")
		cancel()
		<-ctx.Done()
		return false, &DownloadError{Kind: DownloadFailed, Msg: "Download stopped: " + ctx.Err().Error()}
	}}
	files.mapped = nil
	res, err := New(files, dl, fakeComfy{}, files, "").Install(ctx, Request{WorkflowsDir: dir, Workflow: "demo"}, Watch{})
	if !errors.Is(err, context.Canceled) || len(dl.calls) != 1 || len(files.mapped) != 0 {
		t.Fatalf("err %v, %d downloads, mapped %v", err, len(dl.calls), files.mapped)
	}
	if got := statuses(res); got != "FAILED download vae" {
		t.Errorf("items %s", got)
	}
	if files.Size(dl.calls[0].Dest+".part") != 4 {
		t.Error("the .part file must stay")
	}
}

func TestInstallRefusesWithoutComfyUI(t *testing.T) {
	files := &noComfyUI{newMemFiles()}
	_, err := New(files, nil, fakeComfy{}, files, "").Install(context.Background(), Request{WorkflowsDir: workflows(t), Workflow: "demo"}, Watch{})
	if !errors.Is(err, ErrNoComfyUI) || !strings.HasPrefix(err.Error(), "ComfyUI was not found in ") {
		t.Fatalf("err %v", err)
	}
}

type noComfyUI struct{ *memFiles }

func (n *noComfyUI) Installed() (string, bool) { return "/c", false }

func TestMappings(t *testing.T) {
	own := map[string]string{"vae": "/c/models/vae", "loras": "/c/models/loras", "te": "/c/models/te"}
	mapped := map[string][]string{"loras": {"/Y/LORAS"}}
	got := mappings([][2]string{{"vae", "/x/vae"}, {"vae", "/X/VAE"}, {"te", "/c/models/te"}, {"loras", "/y/loras"}, {"unet", "/x/unet"}}, own, mapped)
	want := "[{vae /x/vae true} {loras /y/loras false} {unet /x/unet true}]"
	if fmt.Sprint(got) != want {
		t.Errorf("got  %v\nwant %s", got, want)
	}
}

func TestRemoveDecision(t *testing.T) {
	f := catalog.ListedFile{Role: "vae", Name: "v.safetensors", Folder: "vae", Bytes: 10}
	for _, c := range []struct {
		name   string
		path   string
		size   int64
		users  []string
		force  bool
		status string
		del    bool
	}{
		{"absent", "", 0, nil, false, StatusAbsent, false},
		{"other size", "/m/v", 11, nil, false, StatusRefused, false},
		{"shared", "/m/v", 10, []string{"b"}, false, StatusKeptShared, false},
		{"shared with force", "/m/v", 10, []string{"b"}, true, "", true},
		{"own", "/m/v", 10, nil, false, "", true},
	} {
		it, del := removeDecision(f, c.path, c.size, c.users, c.force)
		if it.Status != c.status || del != c.del {
			t.Errorf("%s: %s %v, want %s %v", c.name, it.Status, del, c.status, c.del)
		}
	}
}

// TestCheckWithAServerThatAnswersNoJSON: an answer that is not a JSON object
// is a server with no node class and no file, as workflow.ps1 read it. No
// answer skips the server checks.
func TestCheckWithAServerThatAnswersNoJSON(t *testing.T) {
	dir := workflows(t)
	files := newMemFiles()
	files.files[filepath.Join(files.root, "vae", "demo-vae.safetensors")] = vaeData
	files.files[filepath.Join(files.root, "text_encoders", "demo-te.safetensors")] = teData
	var syn graph.ObjectInfo
	jsonErr := json.Unmarshal([]byte("<html>"), &syn)
	res, err := New(files, nil, fakeComfy{err: jsonErr}, files, "").Check(context.Background(), Request{WorkflowsDir: dir, Workflow: "demo"})
	if err == nil || statuses(res) != "OK file vae, OK file text_encoder, OK graph , NO_NODE server , NO_NODE server , NOT_SEEN server vae, NOT_SEEN server text_encoder" {
		t.Errorf("not JSON: %v, %s", err, statuses(res))
	}
	res, err = New(files, nil, fakeComfy{err: errors.New("no answer")}, files, "http://h:1/").Check(context.Background(), Request{WorkflowsDir: dir, Workflow: "demo"})
	if err != nil || statuses(res) != "OK file vae, OK file text_encoder, OK graph , SKIP server " || res.Items[3].Detail != "no answer at http://h:1" {
		t.Errorf("no answer: %v, %s, %+v", err, statuses(res), res.Items)
	}
	info := graph.ObjectInfo{"vaeloader": json.RawMessage(`{"input":{"required":{"vae_name":[["sub\\demo-vae.safetensors"]]}}}`),
		"CLIPLoader": json.RawMessage(`{"input":{"required":{"clip_name":[["DEMO-TE.safetensors"]]}}}`)}
	res, err = New(files, nil, fakeComfy{info: info}, files, "").Check(context.Background(), Request{WorkflowsDir: dir, Workflow: "demo", Quick: true})
	if err != nil || statuses(res) != "OK file vae, OK file text_encoder, OK graph , OK server , OK server vae, OK server text_encoder" {
		t.Errorf("a server with the files: %v, %s", err, statuses(res))
	}
}
