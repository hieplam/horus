// Tests of the model tools on the setup service with fakes of its adapters,
// and of the mapping of a typed result to the horus error codes. The tests
// with the real adapters are in cmd/horus (workflow_test.go).
package mcpserver

import (
	"bytes"
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
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/graph"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// lockedBuffer is a log target that the test reads while the server writes.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuffer) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

// diskFiles is a fake of the model files adapter: every model file of
// folder F is in <root>/F, and ComfyUI is installed.
type diskFiles struct{ root string }

func (d diskFiles) Installed() (string, bool) { return d.root, true }
func (d diskFiles) Install(context.Context, io.Writer) error {
	return errors.New("the MCP tools never install ComfyUI")
}
func (d diskFiles) Dirs(folder string) []string { return []string{filepath.Join(d.root, folder)} }
func (d diskFiles) Own(folder string) string    { return filepath.Join(d.root, folder) }
func (d diskFiles) Mapped(string) []string      { return nil }
func (d diskFiles) Map([][2]string) error       { return nil }
func (d diskFiles) Delete(path string) error    { return os.Remove(path) }
func (d diskFiles) SHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), err
}
func (d diskFiles) Size(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return -1
	}
	return fi.Size()
}

// slowDownloader writes part bytes to <dest>.part, says so on ready, and
// then waits for the end of ctx, as a download of a big file does.
type slowDownloader struct {
	part  int
	ready chan string
}

func (s *slowDownloader) Download(ctx context.Context, f provision.Fetch, out io.Writer) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(f.Dest), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(f.Dest+".part", make([]byte, s.part), 0o644); err != nil {
		return false, err
	}
	select { // never blocks: a second download after a cancel stays in the buffer for the test to see
	case s.ready <- f.Dest:
	default:
	}
	<-ctx.Done()
	return false, &provision.DownloadError{Kind: provision.DownloadFailed, Msg: "Download stopped: " + ctx.Err().Error()}
}

// downComfy is a ComfyUI that does not answer.
type downComfy struct{}

func (downComfy) ObjectInfo(context.Context) (graph.ObjectInfo, error) {
	return nil, errors.New("no answer")
}

// setupSandbox copies workflows/it-illustration into a temporary folder,
// with a models.json of small sizes, and no model file on disk.
func setupSandbox(t *testing.T) (models string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join("..", "..", "workflows", "it-illustration")
	dst := filepath.Join(dir, "workflows", "it-illustration")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"manifest.json", "workflow.json"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dst, name), data, 0o644)
	}
	var m map[string]any
	data, _ := os.ReadFile(filepath.Join(src, "models.json"))
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, x := range m["files"].([]any) {
		f := x.(map[string]any)
		body := []byte("stand-in for " + f["repo_path"].(string))
		sum := sha256.Sum256(body)
		f["bytes"], f["sha256"] = len(body), hex.EncodeToString(sum[:])
	}
	out, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(dst, "models.json"), out, 0o644)
	t.Setenv("HORUS_ROOT", dir)
	t.Setenv("HORUS_WORKFLOWS_DIR", filepath.Join(dir, "workflows"))
	t.Setenv("HORUS_AUTOSTART", "0")
	settings.Reset()
	t.Cleanup(settings.Reset)
	return filepath.Join(dir, "models")
}

func TestSizeText(t *testing.T) {
	for n, want := range map[int64]string{172: "0.00 MB", 99_994_999: "99.99 MB", 100_000_000: "0.1 GB", 11_022_645_311: "11.0 GB", 5_050_000_000: "5.0 GB"} {
		if got := sizeText(n); got != want {
			t.Errorf("sizeText(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestInstallReportsProgressAndACancelStopsTheDownload runs an install
// through the MCP server on the setup service, in the same process, against
// a slow fake downloader. The tool reports the size of the .part file. A
// cancel of the call ends the download through the context: the tool
// returns at once, and the .part file stays for the next install.
func TestInstallReportsProgressAndACancelStopsTheDownload(t *testing.T) {
	models := setupSandbox(t)
	log := &lockedBuffer{}
	logTo(log)
	t.Cleanup(func() { logTo(os.Stderr) })
	oldTick := progressInterval
	progressInterval = 20 * time.Millisecond
	t.Cleanup(func() { progressInterval = oldTick })

	dl := &slowDownloader{part: 2_500_000, ready: make(chan string, 1)}
	setup := provision.New(diskFiles{models}, dl, downComfy{}, diskFiles{models}, "")
	got := make(chan string, 100)
	cs := connect(t, New(&fakeImages{}, setup), &mcp.ClientOptions{ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
		select {
		case got <- fmt.Sprintf("%v/%v %s", r.Params.Progress, r.Params.Total, r.Params.Message):
		default:
		}
	}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	params := &mcp.CallToolParams{Name: "install_workflow", Arguments: map[string]any{"workflow": "it-illustration", "accept_license": true}}
	params.SetProgressToken("tok-install")
	done := make(chan error, 1)
	go func() {
		_, err := cs.CallTool(ctx, params)
		done <- err
	}()
	var dest string
	select {
	case dest = <-dl.ready:
	case err := <-done:
		t.Fatalf("the call ended before the download started: %v\n%s", err, log)
	case <-time.After(30 * time.Second):
		t.Fatalf("the download did not start\n%s", log)
	}
	want := "2.5e+06/172 downloading " + filepath.Base(dest) + ": 0.00 GB"
	deadline := time.After(30 * time.Second)
	for seen := false; !seen; {
		select {
		case m := <-got:
			seen = m == want
		case err := <-done:
			t.Fatalf("the call ended before a progress report: %v\n%s", err, log)
		case <-deadline:
			t.Fatalf("no progress report %q\n%s", want, log)
		}
	}
	cancelled := time.Now()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("CallTool: %v", err)
	}
	for !strings.Contains(log.String(), "call install_workflow crashed\ncontext canceled") {
		if time.Since(cancelled) > 10*time.Second {
			t.Fatalf("the tool did not return 10 s after the cancel\n%s", log)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if fi, err := os.Stat(dest + ".part"); err != nil || fi.Size() != 2_500_000 {
		t.Errorf("the .part file must stay: %v", err)
	}
	select {
	case d := <-dl.ready:
		t.Errorf("the install started the next download after the cancel: %s", d)
	default:
	}
}

func item(status, kind, detail string) provision.Item {
	return provision.Item{Status: status, Kind: kind, Role: "diffusion", File: "x.safetensors", Path: "/m/x.safetensors", Bytes: 10, Detail: detail}
}

func failedItem(kind provision.DownloadKind, detail string) provision.Item {
	it := item("FAILED", "download", detail)
	it.Err = &provision.DownloadError{Kind: kind, Msg: "m"}
	return it
}

func codeOf(t *testing.T, err error) *herr.Error {
	t.Helper()
	var he *herr.Error
	if !errors.As(err, &he) {
		t.Fatalf("want a *herr.Error, got %v", err)
	}
	return he
}

// TestInstallMapsEachStatusToItsCode maps each outcome of an install to its
// error code, on the typed result: the kind of a failed download, and the
// statuses of the check.
func TestInstallMapsEachStatusToItsCode(t *testing.T) {
	setupSandbox(t)
	w, err := catalog.Get("it-illustration")
	if err != nil {
		t.Fatal(err)
	}
	lic, _ := w.License()
	cases := []struct {
		name    string
		items   []provision.Item
		message string
		err     error
		code    string
		check   func(e *herr.Error) bool
	}{
		{"401", []provision.Item{item("PRESENT", "download", ""), failedItem(provision.DownloadUnauthorized, "Attempt 1 of 4 did not finish (the server returned error: 401) | FAIL: x")}, "", nil, "HF_TOKEN_INVALID",
			func(e *herr.Error) bool {
				return e.Details["file"] == "x.safetensors" && e.Details["path"] == "/m/x.safetensors" && strings.Contains(e.Details["detail"].(string), "error: 401")
			}},
		{"403", []provision.Item{failedItem(provision.DownloadForbidden, "the server returned error: 403")}, "", nil, "LICENSE_NOT_ACCEPTED",
			func(e *herr.Error) bool {
				u, _ := e.Details["license_url"].(*string)
				return e.Details["license"] == lic.Name && u != nil && lic.URL != nil && *u == *lic.URL
			}},
		{"disk full", []provision.Item{failedItem(provision.DownloadDiskFull, "FAIL: Not enough free space")}, "", nil, "DISK_FULL", nil},
		{"conflict", []provision.Item{failedItem(provision.DownloadConflict, "FAIL: Refusing to overwrite")}, "", nil, "FILE_CONFLICT", nil},
		{"bad hash download", []provision.Item{failedItem(provision.DownloadBadHash, "FAIL: SHA-256 mismatch")}, "", nil, "DOWNLOAD_FAILED", nil},
		{"other download", []provision.Item{failedItem(provision.DownloadFailed, "FAIL: connection reset")}, "", nil, "DOWNLOAD_FAILED",
			func(e *herr.Error) bool { return e.Details["detail"] == "FAIL: connection reset" }},
		{"error of no kind", []provision.Item{{Status: "FAILED", Kind: "download", File: "x", Err: errors.New("disk error")}}, "", nil, "DOWNLOAD_FAILED", nil},
		{"no node", []provision.Item{item("MISSING", "file", ""), {Status: "NO_NODE", Kind: "server", File: "VAELoader"}}, "", nil, "NODE_PACK_MISSING",
			func(e *herr.Error) bool { return fmt.Sprint(e.Details["missing"]) == "[VAELoader]" }},
		{"bad size", []provision.Item{item("BAD_SIZE", "file", ""), {Status: "BAD_HASH", Kind: "file", File: "y", Path: "/m/y"}}, "", nil, "FILE_CONFLICT",
			func(e *herr.Error) bool {
				return e.Details["path"] == "/m/x.safetensors" && len(e.Details["files"].([]provision.Item)) == 2
			}},
		{"bad hash", []provision.Item{{Status: "BAD_HASH", Kind: "file", File: "y", Path: "/m/y"}}, "", nil, "FILE_CONFLICT",
			func(e *herr.Error) bool { return e.Details["path"] == "/m/y" }},
		{"not seen", []provision.Item{item("NOT_SEEN", "server", "")}, "", nil, "BACKEND_RESTART_REQUIRED",
			func(e *herr.Error) bool { return fmt.Sprint(e.Details["files"]) == "[x.safetensors]" }},
		{"missing", []provision.Item{item("MISSING", "file", "")}, "", nil, "WORKFLOW_NOT_INSTALLED",
			func(e *herr.Error) bool { return fmt.Sprint(e.Details["missing"]) == "[x.safetensors]" }},
		{"unlisted", []provision.Item{item("UNLISTED", "graph", "")}, "", nil, "INTERNAL",
			func(e *herr.Error) bool {
				return e.Message == "the workflow loads a model file that models.json does not list"
			}},
		{"no ComfyUI", nil, "ComfyUI was not found in /c. Run horus install-comfyui first, or pass -comfyui-dir.",
			fmt.Errorf("x: %w", provision.ErrNoComfyUI), "BACKEND_NOT_INSTALLED",
			func(e *herr.Error) bool { return strings.HasPrefix(e.Message, "ComfyUI was not found in /c") }},
		{"something else", []provision.Item{item("OK", "file", "")}, "the yaml file is bad", errors.New("the yaml file is bad"), "INTERNAL",
			func(e *herr.Error) bool { return e.Message == "the yaml file is bad" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.err
			if err == nil {
				err = errors.New("failed")
			}
			res := &provision.Result{Action: "install", Workflow: "it-illustration", Message: c.message, Items: c.items}
			e := codeOf(t, installError(res, err, w))
			if e.Code != c.code || (c.check != nil && !c.check(e)) {
				t.Fatalf("got %s %q %v, want %s", e.Code, e.Message, e.Details, c.code)
			}
		})
	}
}

func TestRemoveMapsEachStatusToItsCode(t *testing.T) {
	cases := []struct {
		name  string
		items []provision.Item
		code  string
	}{
		{"in use", []provision.Item{item("DELETED", "file", ""), item("DELETE_FAILED", "file", "in use")}, "FILE_IN_USE"},
		{"refused", []provision.Item{item("REFUSED", "file", "size differs")}, "FILE_CONFLICT"},
		{"something else", nil, "INTERNAL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := codeOf(t, removeError(&provision.Result{Action: "remove", Message: "m", Items: c.items}))
			if e.Code != c.code {
				t.Fatalf("got %s, want %s", e.Code, c.code)
			}
		})
	}
}

// TestUninstallPlanAndRemoveOnTheService: a plan deletes nothing and frees
// no memory; a confirmed remove frees the memory first.
func TestUninstallPlanAndRemoveOnTheService(t *testing.T) {
	models := setupSandbox(t)
	w, _ := catalog.Get("it-illustration")
	m, _ := w.Models()
	var paths []string
	for _, f := range m.Files {
		parts := strings.Split(f.RepoPath, "/")
		p := filepath.Join(models, f.Folder, parts[len(parts)-1])
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("stand-in for "+f.RepoPath), 0o644)
		paths = append(paths, p)
	}
	images := &fakeImages{}
	cs := connect(t, New(images, provision.New(diskFiles{models}, nil, downComfy{}, diskFiles{models}, "")), nil)
	env := envelopeOf(t, callTool(t, cs, "uninstall_workflow", map[string]any{"workflow": "it-illustration"}))
	if e, _ := env["error"].(map[string]any); e["code"] != "CONFIRMATION_REQUIRED" || images.frees() != 0 {
		t.Fatalf("plan: %v, %d frees", env, images.frees())
	}
	env = envelopeOf(t, callTool(t, cs, "uninstall_workflow", map[string]any{"workflow": "it-illustration", "confirm": true}))
	if env["ok"] != true || images.frees() != 1 {
		t.Fatalf("remove: %v, %d frees", env, images.frees())
	}
	for _, p := range paths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s is still there", p)
		}
	}
}

// stateSetup is a setup service with present of the 3 files of
// it-illustration on disk, for list_workflows and describe_workflow.
type stateSetup struct{ present int }

func (s stateSetup) List(context.Context, string) ([]provision.Row, error) {
	return []provision.Row{{Name: "it-illustration", Files: 3, Present: s.present, Bytes: 3}}, nil
}
func (s stateSetup) Check(_ context.Context, req provision.Request) (*provision.Result, error) {
	res := &provision.Result{Action: "check", Workflow: req.Workflow, Checked: true, CheckFiles: 3}
	for i, role := range []string{"diffusion", "text_encoder", "vae"} {
		status := provision.StatusMissing
		if i < s.present {
			status = provision.StatusOK
		}
		res.Items = append(res.Items, provision.Item{Status: status, Kind: "file", Role: role, File: role + ".safetensors", Bytes: 1})
	}
	return res, nil
}
func (stateSetup) Install(context.Context, provision.Request, provision.Watch) (*provision.Result, error) {
	return nil, errors.New("not in this test")
}
func (stateSetup) Remove(context.Context, provision.Request) (*provision.Result, error) {
	return nil, errors.New("not in this test")
}
func (stateSetup) PartSize(string) int64 { return 0 }

// TestBothToolsGiveTheSameInstallState: list_workflows and describe_workflow
// give the same install_state for the same disk (issue #88).
func TestBothToolsGiveTheSameInstallState(t *testing.T) {
	useRepoWorkflows(t)
	for _, c := range []struct {
		present int
		want    string
	}{{0, "not_installed"}, {1, "partially_installed"}, {3, "installed"}} {
		cs := connect(t, New(&fakeImages{}, stateSetup{present: c.present}), nil)
		list, _ := envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{}))["result"].(map[string]any)
		listed := ""
		flows, _ := list["workflows"].([]any)
		for _, w := range flows {
			if m, _ := w.(map[string]any); m["name"] == "it-illustration" {
				listed, _ = m["install_state"].(string)
			}
		}
		desc, _ := envelopeOf(t, callTool(t, cs, "describe_workflow", map[string]any{"workflow": "it-illustration"}))["result"].(map[string]any)
		described, _ := desc["install_state"].(string)
		if listed != c.want || described != c.want {
			t.Errorf("%d of 3 files: list_workflows %q, describe_workflow %q, want %q", c.present, listed, described, c.want)
		}
	}
}
