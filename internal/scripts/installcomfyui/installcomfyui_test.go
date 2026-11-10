package installcomfyui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The test binary is also the fake 7z: when HORUS_FAKE_7Z is set, TestMain
// acts as 7z.exe instead of running the tests. The same fake runs on Windows,
// macOS and Linux.
func TestMain(m *testing.M) {
	if os.Getenv("HORUS_FAKE_7Z") != "" {
		os.Exit(fake7z(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// fake7z records its arguments, then makes the layout of the real archive.
func fake7z(args []string) int {
	if rec := os.Getenv("HORUS_FAKE_RECORD"); rec != "" {
		f, err := os.OpenFile(rec, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			f.WriteString(strings.Join(args, " ") + "\n")
			f.Close()
		}
	}
	if os.Getenv("HORUS_FAKE_FAIL") != "" {
		return 7
	}
	var staging string
	for _, a := range args {
		if strings.HasPrefix(a, "-o") {
			staging = strings.TrimPrefix(a, "-o")
		}
	}
	file := filepath.Join(staging, "ComfyUI_windows_portable", "ComfyUI", "main.py")
	if os.Getenv("HORUS_FAKE_BADLAYOUT") != "" {
		file = filepath.Join(staging, "other", "x.txt")
	}
	os.MkdirAll(filepath.Dir(file), 0o755)
	os.WriteFile(file, []byte("print('fake')\n"), 0o644)
	return 0
}

var archiveBytes = bytes.Repeat([]byte("fake comfyui archive "), 1000)

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

type rig struct {
	opts    Options
	dir     string // folder that holds the install dir and the archive
	record  string // the file where the fake 7z writes its arguments
	out     *bytes.Buffer
	hits    *atomic.Int32 // requests that reached the local server
	ranges  chan string
	archive string
}

// newRig has a local HTTP server with a small fake archive, a temporary folder,
// the test binary as 7z, and a disk that has room.
func newRig(t *testing.T, serve []byte) *rig {
	t.Helper()
	hits := &atomic.Int32{}
	ranges := make(chan string, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case ranges <- r.Header.Get("Range"):
		default:
		}
		http.ServeContent(w, r, "a.7z", time.Time{}, bytes.NewReader(serve))
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{dir: dir, record: filepath.Join(dir, "7z.record"), out: &bytes.Buffer{}, hits: hits, ranges: ranges}
	t.Setenv("HORUS_FAKE_7Z", "1")
	t.Setenv("HORUS_FAKE_RECORD", r.record)
	r.archive = filepath.Join(dir, AssetName)
	r.opts = Options{
		InstallDir: filepath.Join(dir, "ComfyUI_windows_portable"), SevenZip: exe,
		URL: srv.URL + "/" + AssetName, ExpectedBytes: int64(len(archiveBytes)), ExpectedSHA256: sha(archiveBytes),
		ExpandedBytes: 1000, SpareBytes: 1000,
		FreeSpace: func(string) (int64, error) { return 1 << 40, nil },
		Sleep:     func(time.Duration) {}, Client: srv.Client(), Stdout: r.out,
	}
	return r
}

func (r *rig) recorded() string {
	b, _ := os.ReadFile(r.record)
	return string(b)
}

func TestWrongSHA256StopsBeforeUnpacking(t *testing.T) {
	bad := append([]byte{}, archiveBytes...)
	bad[10] ^= 0xff // same size, other content
	r := newRig(t, bad)
	err := Install(context.Background(), r.opts)
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch. Expected "+sha(archiveBytes)+", got "+sha(bad)) {
		t.Fatalf("error = %v", err)
	}
	if got := r.recorded(); got != "" {
		t.Errorf("7z ran, record: %q", got)
	}
	if _, e := os.Stat(r.archive + ".part"); e == nil {
		t.Error("the bad download is still there")
	}
	if _, e := os.Stat(r.opts.InstallDir); e == nil {
		t.Error("the install folder exists")
	}
}

func TestRightSHA256UnpacksWithTheArchivePath(t *testing.T) {
	r := newRig(t, archiveBytes)
	if err := Install(context.Background(), r.opts); err != nil {
		t.Fatal(err)
	}
	rec := r.recorded()
	lines := strings.Split(strings.TrimSpace(rec), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "x "+r.archive+" -o") || !strings.HasSuffix(lines[0], " -y") {
		t.Fatalf("7z record = %q", rec)
	}
	if _, err := os.Stat(filepath.Join(r.opts.InstallDir, "ComfyUI", "main.py")); err != nil {
		t.Errorf("main.py is not in the install folder: %v", err)
	}
	if _, err := os.Stat(r.archive); err == nil {
		t.Error("the archive was not deleted")
	}
	entries, _ := os.ReadDir(r.dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "horus-extract-") {
			t.Errorf("staging folder left: %s", e.Name())
		}
	}
	want := []string{
		"Downloading ComfyUI v0.44.0 portable (", "Checking SHA-256 ...", "Unpacking (a few minutes) ...",
		"Deleted the archive. Use -keep-archive to keep it next time.",
		"OK: ComfyUI v0.44.0 is installed in " + r.opts.InstallDir,
		"Next: horus start-comfyui -comfyui-dir '" + r.opts.InstallDir + "'",
	}
	for _, w := range want {
		if !strings.Contains(r.out.String(), w) {
			t.Errorf("output lacks %q:\n%s", w, r.out)
		}
	}
}

func TestKeepArchive(t *testing.T) {
	r := newRig(t, archiveBytes)
	r.opts.KeepArchive = true
	if err := Install(context.Background(), r.opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(r.archive); err != nil {
		t.Errorf("the archive was deleted: %v", err)
	}
}

func TestTooLittleFreeSpaceStopsWithTheMessageOfTheScript(t *testing.T) {
	r := newRig(t, archiveBytes)
	d := DefaultOptions() // the real sizes: 8.1 GiB needed
	r.opts.ExpectedBytes, r.opts.ExpandedBytes, r.opts.SpareBytes = d.ExpectedBytes, d.ExpandedBytes, d.SpareBytes
	r.opts.FreeSpace = func(string) (int64, error) { return 1 << 30, nil }
	err := Install(context.Background(), r.opts)
	want := "Need 8.1 GiB (archive, unpacked files and 1 GiB spare), have 1.0 GiB."
	if err == nil || !strings.HasPrefix(err.Error(), "Not enough free space on ") || !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("error = %v", err)
	}
	if r.hits.Load() != 0 || r.recorded() != "" {
		t.Error("the command downloaded or unpacked after the check failed")
	}
}

func TestAlreadyInstalledSkips(t *testing.T) {
	r := newRig(t, archiveBytes)
	main := filepath.Join(r.opts.InstallDir, "ComfyUI", "main.py")
	os.MkdirAll(filepath.Dir(main), 0o755)
	os.WriteFile(main, []byte("x"), 0o644)
	if err := Install(context.Background(), r.opts); err != nil {
		t.Fatal(err)
	}
	if got, want := r.out.String(), "SKIP: ComfyUI is already installed in "+r.opts.InstallDir+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
	if r.hits.Load() != 0 || r.recorded() != "" {
		t.Error("a skip must not download or unpack")
	}
}

func TestNonEmptyFolderWithoutComfyUIFails(t *testing.T) {
	r := newRig(t, archiveBytes)
	os.MkdirAll(r.opts.InstallDir, 0o755)
	os.WriteFile(filepath.Join(r.opts.InstallDir, "other.txt"), []byte("x"), 0o644)
	err := Install(context.Background(), r.opts)
	if err == nil || !strings.Contains(err.Error(), "is not empty, and holds no ComfyUI") {
		t.Fatalf("error = %v", err)
	}
}

func TestSevenZipMissingFails(t *testing.T) {
	r := newRig(t, archiveBytes)
	r.opts.SevenZip = filepath.Join(r.dir, "no-such-7z.exe")
	err := Install(context.Background(), r.opts)
	if err == nil || !strings.HasPrefix(err.Error(), "7-Zip was not found.") {
		t.Fatalf("error = %v", err)
	}
}

func TestSevenZipFailureKeepsTheArchive(t *testing.T) {
	r := newRig(t, archiveBytes)
	t.Setenv("HORUS_FAKE_FAIL", "1")
	err := Install(context.Background(), r.opts)
	if err == nil || !strings.Contains(err.Error(), "7-Zip failed with exit code 7. The archive is kept at "+r.archive) {
		t.Fatalf("error = %v", err)
	}
	if _, e := os.Stat(r.archive); e != nil {
		t.Error("the archive was deleted")
	}
	if _, e := os.Stat(r.opts.InstallDir); e == nil {
		t.Error("the install folder exists")
	}
}

func TestWrongLayoutFails(t *testing.T) {
	r := newRig(t, archiveBytes)
	t.Setenv("HORUS_FAKE_BADLAYOUT", "1")
	err := Install(context.Background(), r.opts)
	if err == nil || !strings.Contains(err.Error(), "does not have the expected") {
		t.Fatalf("error = %v", err)
	}
}

func TestVerifiedArchiveIsReusedAndOtherFileIsRefused(t *testing.T) {
	r := newRig(t, archiveBytes)
	os.WriteFile(r.archive, archiveBytes, 0o644)
	if err := Install(context.Background(), r.opts); err != nil {
		t.Fatal(err)
	}
	if r.hits.Load() != 0 || !strings.Contains(r.out.String(), "Using the verified archive that is already at "+r.archive) {
		t.Errorf("hits %d, output:\n%s", r.hits.Load(), r.out)
	}

	r2 := newRig(t, archiveBytes)
	os.WriteFile(r2.archive, []byte("not the archive"), 0o644)
	err := Install(context.Background(), r2.opts)
	if err == nil || !strings.Contains(err.Error(), "does not match the official archive") {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadResumesAPartFile(t *testing.T) {
	r := newRig(t, archiveBytes)
	os.WriteFile(r.archive+".part", archiveBytes[:5000], 0o644)
	if err := Install(context.Background(), r.opts); err != nil {
		t.Fatal(err)
	}
	if got := <-r.ranges; got != "bytes=5000-" {
		t.Errorf("Range header = %q, want bytes=5000-", got)
	}
	if !strings.Contains(r.recorded(), "x ") {
		t.Error("7z did not run after the resumed download")
	}
}

func TestDownloadRetriesAfterAFailedAttempt(t *testing.T) {
	r := newRig(t, archiveBytes)
	var calls atomic.Int32
	inner := r.opts.Client.Transport
	r.opts.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return &http.Response{StatusCode: 503, Body: http.NoBody, Request: req}, nil
		}
		return inner.RoundTrip(req)
	})}
	var slept []time.Duration
	r.opts.Sleep = func(d time.Duration) { slept = append(slept, d) }
	if err := Install(context.Background(), r.opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.out.String(), "Attempt 1 of 3 did not finish (HTTP 503, 0 bytes).") {
		t.Errorf("output:\n%s", r.out)
	}
	if len(slept) != 1 || slept[0] != 5*time.Second {
		t.Errorf("sleeps = %v, want [5s]", slept)
	}
}

func TestDownloadGivesUpAfterThreeAttempts(t *testing.T) {
	r := newRig(t, archiveBytes)
	r.opts.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500, Body: http.NoBody, Request: req}, nil
	})}
	err := Install(context.Background(), r.opts)
	if err == nil || !strings.Contains(err.Error(), "Download did not finish. The partial file is kept so the next run can resume it: ") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(r.out.String(), "Attempt 3 of 3 did not finish") || r.recorded() != "" {
		t.Errorf("output:\n%s\nrecord %q", r.out, r.recorded())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPinnedValues(t *testing.T) {
	d := DefaultOptions()
	if d.ExpectedBytes != 1945636677 || d.ExpectedSHA256 != "a26cf84f043b3d9fbf0d53296f57d7190482c339caf1b3b5cfda5149d454be9f" {
		t.Errorf("pinned size or hash changed: %d %s", d.ExpectedBytes, d.ExpectedSHA256)
	}
	if d.URL != "https://github.com/Comfy-Org/ComfyUI/releases/download/v0.44.0/ComfyUI_windows_portable_nvidia_cu126.7z" {
		t.Errorf("URL = %s", d.URL)
	}
}

func TestNotSupportedOffWindows(t *testing.T) {
	old := goos
	defer func() { goos = old }()
	for _, name := range []string{"linux", "darwin"} {
		goos = name
		var out, errOut bytes.Buffer
		code := Run(nil, &out, &errOut)
		if code != 2 || !strings.Contains(errOut.String(), "not supported on this OS yet") || out.Len() != 0 {
			t.Errorf("%s: exit %d, stderr %q", name, code, errOut.String())
		}
	}
}

func TestBadFlagExits2(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"-nope"}, &out, &errOut); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}
