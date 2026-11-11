// Package installcomfyui installs the official ComfyUI Windows portable build
// (NVIDIA, CUDA 12.6). It ports scripts/install-comfyui.ps1.
//
// Steps: skip when ComfyUI is already in the install folder, check 7-Zip and
// the free disk space, download the archive (with resume), check its size and
// SHA-256, unpack it with 7z.exe into a staging folder, move the top folder to
// the install folder, delete the archive.
//
// Decision (evidence log, G6): the download uses net/http, not curl.exe.
package installcomfyui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/settings"
)

// The official release asset. The size and the hash come from the GitHub release page.
const (
	Version        = "v0.44.0"
	AssetName      = "ComfyUI_windows_portable_nvidia_cu126.7z"
	ExpectedBytes  = int64(1945636677)
	ExpectedSHA256 = "a26cf84f043b3d9fbf0d53296f57d7190482c339caf1b3b5cfda5149d454be9f"
	ExpandedBytes  = int64(5681590674)
	SpareBytes     = int64(1 << 30)
	gib            = float64(1 << 30)
)

// URL is the download address of the pinned archive.
const URL = "https://github.com/Comfy-Org/ComfyUI/releases/download/" + Version + "/" + AssetName

// Options is everything that Install needs. DefaultOptions has the pinned values.
type Options struct {
	InstallDir  string
	ArchivePath string
	SevenZip    string
	KeepArchive bool

	URL            string
	ExpectedBytes  int64
	ExpectedSHA256 string
	ExpandedBytes  int64
	SpareBytes     int64

	// FreeSpace returns the free bytes of the drive that holds path.
	FreeSpace func(path string) (int64, error)
	// Sleep waits between two download attempts.
	Sleep  func(time.Duration)
	Client *http.Client
	Stdout io.Writer
}

// DefaultOptions has the pinned archive and the real disk check.
func DefaultOptions() Options {
	return Options{
		URL: URL, ExpectedBytes: ExpectedBytes, ExpectedSHA256: ExpectedSHA256,
		ExpandedBytes: ExpandedBytes, SpareBytes: SpareBytes,
		FreeSpace: freeSpace, Sleep: time.Sleep, Client: http.DefaultClient, Stdout: os.Stdout,
	}
}

var goos = runtime.GOOS

// Run is the command horus install-comfyui. It returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("horus install-comfyui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	installDir := fs.String("install-dir", "", "target folder (default: HORUS_COMFYUI_DIR, then <home>/ComfyUI_windows_portable)")
	archive := fs.String("archive-path", "", "where to keep the downloaded .7z file during the install (default: next to -install-dir)")
	sevenZip := fs.String("seven-zip", "", "full path to 7z.exe (default: search PATH, then Program Files\\7-Zip)")
	keep := fs.Bool("keep-archive", false, "do not delete the archive after a successful install")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if goos != "windows" {
		fmt.Fprintln(stderr, "horus install-comfyui: not supported on this OS yet")
		return 2
	}
	o := DefaultOptions()
	o.Stdout = stdout
	o.InstallDir, o.ArchivePath, o.SevenZip, o.KeepArchive = *installDir, *archive, *sevenZip, *keep
	if o.InstallDir == "" {
		o.InstallDir = settings.ComfyDir()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := Install(ctx, o); err != nil {
		fmt.Fprintln(stderr, "horus install-comfyui:", err)
		return 1
	}
	return 0
}

// Install runs the steps. It returns nil after a skip.
func Install(ctx context.Context, o Options) error {
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
	installDir, err := filepath.Abs(o.InstallDir)
	if err != nil {
		return err
	}
	installDir = strings.TrimRight(installDir, `\/`)
	parentDir := filepath.Dir(installDir)
	archivePath := o.ArchivePath
	if archivePath == "" {
		archivePath = filepath.Join(parentDir, AssetName)
	}
	if archivePath, err = filepath.Abs(archivePath); err != nil {
		return err
	}
	partPath := archivePath + ".part"

	// 1. Already installed?
	if isFile(filepath.Join(installDir, "ComfyUI", "main.py")) {
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
	if sevenZip == "" || !isFile(sevenZip) {
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
	if isFile(archivePath) {
		if fileSize(archivePath) == o.ExpectedBytes && sha256File(archivePath) == o.ExpectedSHA256 {
			fmt.Fprintf(out, "Using the verified archive that is already at %s\n", archivePath)
			archiveReady = true
		} else {
			return fmt.Errorf("A file already exists at %s and does not match the official archive. Move it or pass another -archive-path.", archivePath)
		}
	}
	if !archiveReady {
		fmt.Fprintf(out, "Downloading ComfyUI %s portable (%.2f GiB) from GitHub ...\n", Version, float64(o.ExpectedBytes)/gib)
		if err := o.download(ctx, partPath); err != nil {
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
	if !isFile(filepath.Join(top, "ComfyUI", "main.py")) {
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
	fmt.Fprintf(out, "OK: ComfyUI %s is installed in %s\n", Version, installDir)
	fmt.Fprintf(out, "Next: horus start-comfyui -comfyui-dir '%s'\n", installDir)
	return nil
}

func (o Options) download(ctx context.Context, part string) error {
	out := o.Stdout
	if out == nil {
		out = io.Discard
	}
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
func (o Options) fetch(ctx context.Context, part string) error {
	have := fileSize(part)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.URL, nil)
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

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

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
			if c := filepath.Join(base, "7-Zip", "7z.exe"); isFile(c) {
				return c
			}
		}
	}
	return ""
}
