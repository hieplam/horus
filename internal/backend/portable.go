// The Windows portable backend: the official ComfyUI Windows portable build
// (NVIDIA, CUDA 12.6). It installs the build and starts and stops its server.
//
// Install: skip when ComfyUI is already in the install folder, check 7-Zip and
// the free disk space, download the archive (with resume), check its size and
// SHA-256, unpack it with 7z.exe into a staging folder, move the top folder to
// the install folder, delete the archive. The download uses net/http, not
// curl.exe (evidence log of #4, G6).
//
// Start: run the embedded Python with ComfyUI's main.py as the root of a
// process tree (platform.StartTree), so ComfyUI keeps running after horus
// ends. The output goes to horus-comfyui.out.log and horus-comfyui.err.log in
// the install folder. The server listens on 127.0.0.1 only. Start writes the
// id of the tree to horus-comfyui.state.json next to the logs.
//
// Stop: read that state file and end the tree with platform.EndTree. Without
// a state file for the same address, Stop refuses: horus did not start that
// ComfyUI. The id of a tree on Windows is the random name of a Job Object, so
// an old state file (ComfyUI ended, the PC restarted) can end no other
// process.
package backend

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// The official release asset. The size and the hash come from the GitHub release page.
const (
	version        = "v0.44.0"
	assetName      = "ComfyUI_windows_portable_nvidia_cu126.7z"
	expectedBytes  = int64(1945636677)
	expectedSHA256 = "a26cf84f043b3d9fbf0d53296f57d7190482c339caf1b3b5cfda5149d454be9f"
	expandedBytes  = int64(5681590674)
	spareBytes     = int64(1 << 30)
	gib            = float64(1 << 30)
)

// pinnedURL is the download address of the pinned archive.
const pinnedURL = "https://github.com/Comfy-Org/ComfyUI/releases/download/" + version + "/" + assetName

// The defaults of Start and Stop.
const (
	startTimeout = 300 * time.Second // the wait for the server; the first start can take a few minutes
	stopTimeout  = 15 * time.Second  // the wait until the server no longer answers after the end of its tree
	pollInterval = 2 * time.Second   // the time between two checks of the server (2 s in the script)
)

// The files that Start writes in the install folder.
const (
	outLogName = "horus-comfyui.out.log"
	errLogName = "horus-comfyui.err.log"
	stateName  = "horus-comfyui.state.json"
)

// Portable is the Windows portable build in one install folder. An empty
// field has its default; Dir and URL are then read from the settings at each
// call.
type Portable struct {
	Dir string // the install folder (default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)
	URL string // the address that the server answers on; the port comes from it (default: HORUS_COMFYUI_URL)

	// Install.
	ArchivePath string // where the .7z file stays during the install (default: next to Dir)
	SevenZip    string // the full path of 7z.exe (default: the PATH, then Program Files\7-Zip)
	KeepArchive bool   // do not delete the archive after a successful install

	// Start and Stop.
	Timeout      time.Duration // the wait of Start (default: startTimeout)
	StopTimeout  time.Duration // the wait of Stop (default: stopTimeout)
	PollInterval time.Duration // the time between two checks of the server (default: 2 s)

	// Test seams of Install. The zero value is the pinned archive, the real
	// disk check, time.Sleep and http.DefaultClient.
	ArchiveURL     string
	ExpectedBytes  int64
	ExpectedSHA256 string
	ExpandedBytes  int64
	SpareBytes     int64
	FreeSpace      func(path string) (int64, error) // the free bytes of the drive that holds path
	Sleep          func(time.Duration)              // the wait between two download attempts
	Client         *http.Client
}

func (p *Portable) dir() string {
	if p.Dir != "" {
		return p.Dir
	}
	return settings.ComfyDir()
}

func (p *Portable) serverURL() string {
	if p.URL != "" {
		return strings.TrimRight(p.URL, "/")
	}
	return settings.ComfyURL()
}

func orDuration(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

// Layout gives the folders and files of the install.
func (p *Portable) Layout() Layout {
	dir := p.dir()
	return Layout{
		Dir:     dir,
		ComfyUI: filepath.Join(dir, "ComfyUI"),
		Python:  filepath.Join(dir, "python_embeded", "python.exe"),
		Logs:    []string{filepath.Join(dir, outLogName), filepath.Join(dir, errLogName)},
	}
}

// Installed gives the install folder, and reports if ComfyUI is installed
// there: its ComfyUI/main.py is a file.
func (p *Portable) Installed() (string, bool) {
	l := p.Layout()
	return l.Dir, platform.IsFile(filepath.Join(l.ComfyUI, "main.py"))
}

// -- install ------------------------------------------------------------------

// Install runs the steps of the install. It returns nil after a skip.
func (p *Portable) Install(ctx context.Context, out io.Writer) error {
	o := p.withInstallDefaults()
	installDir, err := filepath.Abs(o.dir())
	if err != nil {
		return err
	}
	installDir = strings.TrimRight(installDir, `\/`)
	parentDir := filepath.Dir(installDir)
	archivePath := o.ArchivePath
	if archivePath == "" {
		archivePath = filepath.Join(parentDir, assetName)
	}
	if archivePath, err = filepath.Abs(archivePath); err != nil {
		return err
	}
	partPath := archivePath + ".part"

	// 1. Already installed?
	if platform.IsFile(filepath.Join(installDir, "ComfyUI", "main.py")) {
		fmt.Fprintf(out, "SKIP: ComfyUI is already installed in %s\n", installDir)
		return nil
	}
	if entries, err := os.ReadDir(installDir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s exists, is not empty, and holds no ComfyUI. Choose another -install-dir or empty this folder.", installDir)
	}

	// 2. Tools and disk space.
	sevenZip := o.SevenZip
	if sevenZip == "" {
		sevenZip = findSevenZip()
	}
	if sevenZip == "" || !platform.IsFile(sevenZip) {
		return errors.New("7-Zip was not found. Install it (winget install 7zip.7zip) or pass -seven-zip with the full path to 7z.exe.")
	}
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return err
	}
	need := o.ExpectedBytes + o.ExpandedBytes + o.SpareBytes
	free, err := o.FreeSpace(installDir)
	if err != nil {
		return fmt.Errorf("cannot read the free space: %w", err)
	}
	if free < need {
		return fmt.Errorf("Not enough free space on %s. Need %.1f GiB (archive, unpacked files and 1 GiB spare), have %.1f GiB.",
			driveName(installDir), float64(need)/gib, float64(free)/gib)
	}

	// 3. Download (resumes a .part file after an interruption).
	archiveReady := false
	if platform.IsFile(archivePath) {
		if fileSize(archivePath) == o.ExpectedBytes && sha256File(archivePath) == o.ExpectedSHA256 {
			fmt.Fprintf(out, "Using the verified archive that is already at %s\n", archivePath)
			archiveReady = true
		} else {
			return fmt.Errorf("A file already exists at %s and does not match the official archive. Move it or pass another -archive-path.", archivePath)
		}
	}
	if !archiveReady {
		fmt.Fprintf(out, "Downloading ComfyUI %s portable (%.2f GiB) from GitHub ...\n", version, float64(o.ExpectedBytes)/gib)
		if err := o.download(ctx, partPath, out); err != nil {
			return err
		}
		// 4. Verify.
		fmt.Fprintln(out, "Checking SHA-256 ...")
		actual := sha256File(partPath)
		if actual != o.ExpectedSHA256 {
			os.Remove(partPath)
			return fmt.Errorf("SHA-256 mismatch. Expected %s, got %s. The bad download was deleted.", o.ExpectedSHA256, actual)
		}
		if err := os.Rename(partPath, archivePath); err != nil {
			return err
		}
	}

	// 5. Unpack into a staging folder, then move the top-level folder to -install-dir.
	staging := filepath.Join(parentDir, "horus-extract-"+randHex())
	if err := os.Mkdir(staging, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	fmt.Fprintln(out, "Unpacking (a few minutes) ...")
	cmd := exec.CommandContext(ctx, sevenZip, "x", archivePath, "-o"+staging, "-y")
	if err := cmd.Run(); err != nil {
		code := -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		}
		return fmt.Errorf("7-Zip failed with exit code %d. The archive is kept at %s", code, archivePath)
	}
	top := filepath.Join(staging, "ComfyUI_windows_portable")
	if !platform.IsFile(filepath.Join(top, "ComfyUI", "main.py")) {
		return errors.New(`The archive does not have the expected ComfyUI_windows_portable\ComfyUI\main.py layout.`)
	}
	if _, err := os.Stat(installDir); err == nil {
		os.Remove(installDir) // an empty folder only; checked above
	}
	if err := os.Rename(top, installDir); err != nil {
		return err
	}

	if !o.KeepArchive {
		os.Remove(archivePath)
		fmt.Fprintln(out, "Deleted the archive. Use -keep-archive to keep it next time.")
	}
	fmt.Fprintf(out, "OK: ComfyUI %s is installed in %s\n", version, installDir)
	fmt.Fprintf(out, "Next: horus start-comfyui -comfyui-dir '%s'\n", installDir)
	return nil
}

// withInstallDefaults gives a copy with the pinned archive and the real
// helpers in each empty field.
func (p *Portable) withInstallDefaults() Portable {
	o := *p
	if o.ArchiveURL == "" {
		o.ArchiveURL = pinnedURL
	}
	if o.ExpectedBytes == 0 {
		o.ExpectedBytes = expectedBytes
	}
	if o.ExpectedSHA256 == "" {
		o.ExpectedSHA256 = expectedSHA256
	}
	if o.ExpandedBytes == 0 {
		o.ExpandedBytes = expandedBytes
	}
	if o.SpareBytes == 0 {
		o.SpareBytes = spareBytes
	}
	if o.FreeSpace == nil {
		o.FreeSpace = freeSpace
	}
	if o.Sleep == nil {
		o.Sleep = time.Sleep
	}
	if o.Client == nil {
		o.Client = http.DefaultClient
	}
	return o
}

// freeSpace returns the free bytes that the current user can use on the
// drive of path. path itself can be missing: the check reads the root of its
// drive.
func freeSpace(path string) (int64, error) {
	n, err := platform.FreeSpace(filepath.VolumeName(path) + `\`)
	return int64(n), err
}

func (o Portable) download(ctx context.Context, part string, out io.Writer) error {
	const attempts = 3
	for attempt := 1; attempt <= attempts; attempt++ {
		if fileSize(part) > o.ExpectedBytes {
			os.Remove(part)
		}
		var derr error
		if fileSize(part) < o.ExpectedBytes {
			derr = o.fetch(ctx, part)
		}
		have := fileSize(part)
		if derr == nil && have == o.ExpectedBytes {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		why := "no error"
		if derr != nil {
			why = derr.Error()
		}
		fmt.Fprintf(out, "Attempt %d of %d did not finish (%s, %d bytes).\n", attempt, attempts, why, have)
		if have > o.ExpectedBytes {
			os.Remove(part)
		}
		if attempt < attempts {
			o.Sleep(time.Duration(5*attempt) * time.Second)
		}
	}
	return fmt.Errorf("Download did not finish. The partial file is kept so the next run can resume it: %s", part)
}

// fetch downloads the rest of the file. It sends a Range header when a part exists.
func (o Portable) fetch(ctx context.Context, part string) error {
	have := fileSize(part)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.ArchiveURL, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	flags := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK:
		flags |= os.O_TRUNC // the server ignored the Range header: start again
	default:
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// -- start --------------------------------------------------------------------

// state is the content of horus-comfyui.state.json: the tree of the ComfyUI
// that Start started.
type state struct {
	Tree    string `json:"tree"`    // the id of platform.StartTree
	PID     int    `json:"pid"`     // the process of the root, for the text of Stop
	URL     string `json:"url"`     // the address that this ComfyUI answers on
	Started string `json:"started"` // the time of the start (RFC 3339)
}

func (p *Portable) statePath() string { return filepath.Join(p.dir(), stateName) }

// Start starts ComfyUI when its server does not answer, then waits for it.
func (p *Portable) Start(ctx context.Context, out io.Writer) error {
	l := p.Layout()
	serverURL := p.serverURL()
	if !platform.IsFile(l.Python) || !platform.IsFile(filepath.Join(l.ComfyUI, "main.py")) {
		return &notInstalledError{fmt.Sprintf("ComfyUI was not found in %s. Run horus install-comfyui first, or pass -comfyui-dir.", l.Dir)}
	}
	if answers(ctx, serverURL) {
		fmt.Fprintf(out, "ComfyUI already answers at %s\n", serverURL)
		return nil
	}
	port, err := portOf(serverURL)
	if err != nil {
		return err
	}
	outLog, errLog := l.Logs[0], l.Logs[1]
	outF, err := os.Create(outLog)
	if err != nil {
		return err
	}
	defer outF.Close()
	errF, err := os.Create(errLog)
	if err != nil {
		return err
	}
	defer errF.Close()

	// No context here: the process must outlive this call.
	cmd := exec.Command(l.Python, "-s", filepath.Join("ComfyUI", "main.py"),
		"--windows-standalone-build", "--listen", "127.0.0.1", "--port", port)
	cmd.Dir = l.Dir
	cmd.Stdout, cmd.Stderr = outF, errF
	// A tree of its own: ComfyUI keeps running after this process ends.
	tree, err := platform.StartTree(cmd)
	if err != nil {
		return fmt.Errorf("cannot start %s: %w", l.Python, err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	st := state{Tree: tree, PID: cmd.Process.Pid, URL: serverURL, Started: time.Now().Format(time.RFC3339)}
	if err := writeState(p.statePath(), st); err != nil {
		// Without the state file, Stop cannot end this tree: end it now.
		_ = platform.EndTree(tree)
		return fmt.Errorf("cannot write %s: %w", p.statePath(), err)
	}
	fmt.Fprintf(out, "Started ComfyUI (process %d). Waiting for %s ...\n", cmd.Process.Pid, serverURL)

	timeout := orDuration(p.Timeout, startTimeout)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(orDuration(p.PollInterval, pollInterval))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("ComfyUI did not answer within %d seconds. Read the logs: %s and %s", int(timeout.Seconds()), outLog, errLog)
		case <-tick.C:
		}
		if answers(ctx, serverURL) {
			fmt.Fprintf(out, "OK: ComfyUI answers at %s\n", serverURL)
			return nil
		}
		select {
		case werr := <-exited:
			code := 0
			var ee *exec.ExitError
			if errors.As(werr, &ee) {
				code = ee.ExitCode()
			}
			removeState(p.statePath(), tree)
			return fmt.Errorf("ComfyUI stopped with exit code %d. Read the log: %s", code, errLog)
		default:
		}
	}
}

// portOf gives the port of the address, as a decimal number.
func portOf(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", fmt.Errorf("bad -server-url: %w", err)
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("bad port in -server-url: %q", port)
	}
	return port, nil
}

// -- stop ---------------------------------------------------------------------

// Stop ends the ComfyUI that Start started at the same address. Without a
// state file for that address it ends nothing: when a server answers there,
// horus did not start it, and Stop refuses with an error.
func (p *Portable) Stop(ctx context.Context, out io.Writer) error {
	serverURL := p.serverURL()
	path := p.statePath()
	st, err := readState(path)
	if err != nil {
		return err
	}
	if st == nil || st.URL != serverURL {
		if !answers(ctx, serverURL) {
			fmt.Fprintf(out, "No ComfyUI that horus started runs at %s. Nothing to stop.\n", serverURL)
			return nil
		}
		why := fmt.Sprintf("%s holds no ComfyUI that horus started", path)
		if st != nil {
			why = fmt.Sprintf("horus started the ComfyUI at %s, not this one", st.URL)
		}
		return fmt.Errorf("horus did not start the ComfyUI that answers at %s (%s), so it does not stop it. Stop it where it was started.", serverURL, why)
	}
	upBefore := answers(ctx, serverURL)
	// The id is the random name of a Job Object: when the tree is gone (also
	// after a restart of the PC), EndTree reaches no process.
	if err := platform.EndTree(st.Tree); err != nil {
		return fmt.Errorf("cannot stop the ComfyUI that horus started (process %d): %w", st.PID, err)
	}
	removeState(path, st.Tree)
	if !upBefore {
		fmt.Fprintf(out, "The ComfyUI that horus started at %s (process %d) was not running any more.\n", serverURL, st.PID)
		return nil
	}
	if !p.waitDown(ctx, serverURL) {
		return fmt.Errorf("the ComfyUI that horus started (process %d) is gone, but a server still answers at %s. horus did not start it, so it does not stop it.", st.PID, serverURL)
	}
	fmt.Fprintf(out, "OK: stopped the ComfyUI that horus started at %s (process %d).\n", serverURL, st.PID)
	return nil
}

// waitDown waits until the server no longer answers. It reports false when
// it still answers after the wait of Stop.
func (p *Portable) waitDown(ctx context.Context, serverURL string) bool {
	deadline := time.Now().Add(orDuration(p.StopTimeout, stopTimeout))
	every := orDuration(p.PollInterval, pollInterval)
	if every > 500*time.Millisecond {
		every = 500 * time.Millisecond
	}
	for {
		if !answers(ctx, serverURL) {
			return true
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return false
		}
		time.Sleep(every)
	}
}

func writeState(path string, st state) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// readState reads the state file. No file gives nil and no error.
func readState(path string) (*state, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st state
	if err := json.Unmarshal(data, &st); err != nil || st.Tree == "" {
		return nil, fmt.Errorf("%s is not a state file of horus. Delete it, then stop ComfyUI where it was started.", path)
	}
	return &st, nil
}

// removeState deletes the state file when it still names the tree.
func removeState(path, tree string) {
	if st, err := readState(path); err == nil && st != nil && st.Tree == tree {
		os.Remove(path)
	}
}

// answers reports whether GET <serverURL>/system_stats gives a 2xx answer.
func answers(ctx context.Context, serverURL string) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/system_stats", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// -- files --------------------------------------------------------------------

func fileSize(p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func sha256File(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

func randHex() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func driveName(p string) string {
	if v := filepath.VolumeName(p); v != "" {
		return v + `\`
	}
	return "/"
}

func findSevenZip() string {
	if p, err := exec.LookPath("7z.exe"); err == nil {
		return p
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if base := os.Getenv(env); base != "" {
			if c := filepath.Join(base, "7-Zip", "7z.exe"); platform.IsFile(c) {
				return c
			}
		}
	}
	return ""
}
