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
// id of the tree to the state file horus-comfyui.state.json next to the logs
// (started.go).
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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/platform"
)

// The official release asset. The size and the hash come from the GitHub release page.
const (
	assetName      = "ComfyUI_windows_portable_nvidia_cu126.7z"
	expectedBytes  = int64(1945636677)
	expectedSHA256 = "a26cf84f043b3d9fbf0d53296f57d7190482c339caf1b3b5cfda5149d454be9f"
	expandedBytes  = int64(5681590674)
	spareBytes     = int64(1 << 30)
	gib            = float64(1 << 30)
)

// pinnedURL is the download address of the pinned archive.
const pinnedURL = "https://github.com/Comfy-Org/ComfyUI/releases/download/" + version + "/" + assetName

// The log files that Start writes in the install folder, next to the state
// file (started.go).
const (
	outLogName = "horus-comfyui.out.log"
	errLogName = "horus-comfyui.err.log"
)

// portable is the Windows portable build in one install folder.
type portable struct {
	Place
	Archive

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

// Layout gives the folders and files of the install.
func (p *portable) Layout() Layout {
	dir := p.dir()
	return Layout{
		Dir:     dir,
		ComfyUI: comfyUIDir(dir),
		Python:  filepath.Join(dir, "python_embeded", "python.exe"),
		Logs:    []string{filepath.Join(dir, outLogName), filepath.Join(dir, errLogName)},
	}
}

// -- install ------------------------------------------------------------------

// Install runs the steps of the install. It returns nil after a skip.
func (p *portable) Install(ctx context.Context, out io.Writer) error {
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
	if isInstalled(installDir) {
		saySkip(out, installDir)
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
	sayInstalled(out, installDir)
	return nil
}

// withInstallDefaults gives a copy with the pinned archive and the real
// helpers in each empty field.
func (p *portable) withInstallDefaults() portable {
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

func (o portable) download(ctx context.Context, part string, out io.Writer) error {
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
func (o portable) fetch(ctx context.Context, part string) error {
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

// Start starts ComfyUI when its server does not answer, then waits for it.
func (p *portable) Start(ctx context.Context, out io.Writer) error {
	l := p.Layout()
	serverURL := p.serverURL()
	if !platform.IsFile(l.Python) || !platform.IsFile(filepath.Join(l.ComfyUI, "main.py")) {
		return &notInstalledError{fmt.Sprintf("ComfyUI was not found in %s. Run horus install-comfyui first, or pass -comfyui-dir.", l.Dir)}
	}
	if alreadyUp(ctx, out, serverURL) {
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
	path := statePath(l.Dir)
	st := state{Tree: tree, PID: cmd.Process.Pid, URL: serverURL, Started: time.Now().Format(time.RFC3339)}
	if err := writeState(path, st); err != nil {
		// Without the state file, Stop cannot end this tree: end it now.
		_ = platform.EndTree(tree)
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return p.waitUp(ctx, out, st, l.Logs, func() error {
		select {
		case werr := <-exited:
			code := 0
			var ee *exec.ExitError
			if errors.As(werr, &ee) {
				code = ee.ExitCode()
			}
			removeState(path, tree)
			return fmt.Errorf("ComfyUI stopped with exit code %d. Read the log: %s", code, errLog)
		default:
			return nil
		}
	})
}

// -- stop ---------------------------------------------------------------------

// Stop ends the tree of the ComfyUI that Start started at the same address
// (stopStarted). The id is the random name of a Job Object: when the tree is
// gone (also after a restart of the PC), EndTree reaches no process.
func (p *portable) Stop(ctx context.Context, out io.Writer) error {
	return p.stopStarted(ctx, out, p.dir(), func(st *state) (bool, error) {
		if err := platform.EndTree(st.Tree); err != nil {
			return false, fmt.Errorf("cannot stop the ComfyUI that horus started%s: %w", process(st.PID), err)
		}
		return true, nil
	})
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
