// Package download downloads one file and keeps it only if its size and
// SHA-256 hash are correct. It ports scripts/download-model.ps1. Go's net/http
// replaces curl.exe.
//
//   - It writes to "<dest>.part" first. The final name appears only after the
//     SHA-256 hash matches.
//   - If the destination file already exists and matches the size and hash, it
//     prints SKIP and succeeds.
//   - If the destination file exists and does NOT match, it refuses. It never
//     overwrites or deletes a file that it did not write.
//   - It needs the rest of the file plus 1 GiB of free space on the drive.
//   - It retries a stopped download and resumes the .part file with a Range
//     header.
//   - The Hugging Face token comes from the environment variable HF_TOKEN
//     (horus download-model -token-env-var names another one), or from the
//     .env file in the repo root. Only this package reads it, and no other
//     package can name the variable: neither the name nor the option that
//     sets it is exported. It goes only in a header, only over https, and only to the host
//     huggingface.co or a host that ends with .huggingface.co. A redirect to
//     another host does not get it. The token is never in a URL, an argument or
//     an output line.
package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/settings"
)

// SpareBytes is the free space that must stay on the drive after the download.
const SpareBytes = 1 << 30

const gib = float64(1 << 30)

// Kind is the outcome of a failed download. horus/models.py reads four of them
// from the text output (HTTP 401, HTTP 403, not enough free space, refusal to
// overwrite). The Go API gives them as a value.
type Kind int

const (
	// KindFailed: the download did not finish, or an argument is wrong.
	KindFailed Kind = iota
	// KindUnauthorized: the server answered HTTP 401 (token missing or wrong).
	KindUnauthorized
	// KindForbidden: the server answered HTTP 403 (license not accepted).
	KindForbidden
	// KindDiskFull: the free space is under the rest of the file plus 1 GiB.
	KindDiskFull
	// KindConflict: a different file with the same name is on disk.
	KindConflict
	// KindBadHash: the downloaded bytes have a different SHA-256. The .part
	// file is deleted.
	KindBadHash
)

func (k Kind) String() string {
	return [...]string{"failed", "unauthorized", "forbidden", "disk-full", "conflict", "bad-hash"}[k]
}

// Error is the error of Download. Its text is the text after "FAIL: ".
type Error struct {
	Kind Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func fail(k Kind, format string, a ...any) *Error { return &Error{k, fmt.Sprintf(format, a...)} }

// defaultTokenVar is the name of the variable that holds the Hugging Face
// token. This package reads the token itself; no other package passes it.
const defaultTokenVar = "HF_TOKEN"

// Options is one download.
type Options struct {
	URL           string // https address of the file
	Dest          string // path of the final file; the folder is made when needed
	ExpectedBytes int64  // exact size of the file in bytes
	SHA256        string // expected SHA-256, 64 hexadecimal characters
	MaxAttempts   int    // default 4
	// tokenVar is the variable that holds the token; default HF_TOKEN. Only
	// the flag -token-env-var of horus download-model and the tests of this
	// package set it.
	tokenVar string

	// Out gets the text lines of download-model.ps1. Default: io.Discard.
	Out io.Writer
	// Root is the folder of the .env file. Default: settings.Root().
	Root string
	// Transport sends the requests. Default: http.DefaultTransport. The client
	// around it always applies the redirect rule of the token.
	Transport http.RoundTripper
	// Sleep waits between attempts (5 s times the attempt number). Default: time.Sleep.
	Sleep func(time.Duration)
	// FreeSpace returns the bytes that the caller can still write in the folder.
	// Default: the free space of the drive.
	FreeSpace func(dir string) (uint64, error)
}

// Result is a successful download.
type Result struct {
	Skipped bool   // the file was already present and verified
	Path    string // the full path of the file
}

var hexSHA = regexp.MustCompile(`^[A-Fa-f0-9]{64}$`)

// Download downloads one file as download-model.ps1 does. On failure the error
// is an *Error.
func Download(ctx context.Context, o Options) (Result, error) {
	if o.Out == nil {
		o.Out = io.Discard
	}
	if o.tokenVar == "" {
		o.tokenVar = defaultTokenVar
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 4
	}
	if o.Sleep == nil {
		o.Sleep = time.Sleep
	}
	if o.FreeSpace == nil {
		o.FreeSpace = platform.FreeSpace
	}
	if o.Transport == nil {
		o.Transport = http.DefaultTransport
	}
	say := func(format string, a ...any) { fmt.Fprintf(o.Out, format+"\n", a...) }

	if o.ExpectedBytes <= 0 {
		return Result{}, fail(KindFailed, "-expected-bytes must be greater than 0.")
	}
	if !hexSHA.MatchString(o.SHA256) {
		return Result{}, fail(KindFailed, "-sha256 must be 64 hexadecimal characters.")
	}
	u, err := url.Parse(o.URL)
	if err != nil || u.Host == "" {
		return Result{}, fail(KindFailed, "Not a valid address: %s", o.URL)
	}
	if u.Scheme != "https" {
		return Result{}, fail(KindFailed, "Only https addresses are allowed. Got: %s", u.Scheme)
	}
	if u.User != nil {
		// net/http would send the user part as a Basic Authorization header.
		return Result{}, fail(KindFailed, "The address must not hold a user name or a password.")
	}
	dest, err := filepath.Abs(o.Dest)
	if err != nil {
		return Result{}, fail(KindFailed, "%v", err)
	}
	part := dest + ".part"
	want := strings.ToLower(o.SHA256)
	name := filepath.Base(dest)

	// 1. The file is already there.
	if fi, err := os.Stat(dest); err == nil && !fi.IsDir() {
		if fi.Size() != o.ExpectedBytes {
			return Result{}, fail(KindConflict, "Refusing to overwrite %s : it has %d bytes, expected %d. Move or delete it yourself.",
				dest, fi.Size(), o.ExpectedBytes)
		}
		say("Checking SHA-256 of the existing file %s ...", name)
		got, err := fileSHA256(dest)
		if err != nil {
			return Result{}, fail(KindFailed, "%v", err)
		}
		if got == want {
			say("SKIP %s : already present and verified.", name)
			return Result{Skipped: true, Path: dest}, nil
		}
		return Result{}, fail(KindConflict, "Refusing to overwrite %s : SHA-256 is %s, expected %s. Move or delete it yourself.",
			dest, got, want)
	}

	// 2. Free space: the rest of the file plus 1 GiB.
	already := size(part)
	need := uint64(max(0, o.ExpectedBytes-already)) + SpareBytes
	dir := filepath.Dir(dest)
	if free, err := o.FreeSpace(existingAncestor(dir)); err != nil {
		say("Warning: cannot read free space for this path (network path?). Continuing.")
	} else if free < need {
		return Result{}, fail(KindDiskFull, "Not enough free space on %s. Need %.2f GiB (file plus 1 GiB spare), have %.2f GiB.",
			existingAncestor(dir), float64(need)/gib, float64(free)/gib)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, fail(KindFailed, "%v", err)
	}

	// 3. Token, only for Hugging Face hosts.
	token := readToken(o.tokenVar, o.Root)
	if token != "" && !TokenAllowed(u) {
		say("Warning: the host %s is not huggingface.co. The token is not sent.", u.Hostname())
	}
	client := newClient(o.Transport)

	// 4. Download with retries. Each try resumes the .part file.
	say("Downloading %s (%.2f GiB) ...", name, float64(o.ExpectedBytes)/gib)
	complete := false
	lastStatus := 0
	for attempt := 1; attempt <= o.MaxAttempts; attempt++ {
		have := size(part)
		if have > o.ExpectedBytes {
			if err := os.Remove(part); err != nil {
				return Result{}, fail(KindFailed, "%v", err)
			}
			have = 0
		}
		if have == o.ExpectedBytes {
			complete = true
			break
		}
		status, err := fetch(ctx, client, u, token, part, have, o.ExpectedBytes)
		if status != 0 {
			lastStatus = status
		}
		have = size(part)
		if err == nil && have == o.ExpectedBytes {
			complete = true
			break
		}
		if ctx.Err() != nil {
			return Result{}, fail(KindFailed, "Download stopped: %v. The partial file is kept for the next run: %s", ctx.Err(), part)
		}
		say("Attempt %d of %d did not finish (%s, %d of %d bytes).", attempt, o.MaxAttempts, why(err, status), have, o.ExpectedBytes)
		if status >= 400 {
			say("The server answered with an error. HTTP 401 = token missing or wrong. HTTP 403 = license not accepted for this account.")
		}
		if attempt < o.MaxAttempts {
			o.Sleep(time.Duration(5*attempt) * time.Second)
		}
	}
	if !complete {
		k := KindFailed
		switch lastStatus {
		case http.StatusUnauthorized:
			k = KindUnauthorized
		case http.StatusForbidden:
			k = KindForbidden
		}
		return Result{}, fail(k, "Download did not finish after %d attempts. The partial file is kept for the next run: %s", o.MaxAttempts, part)
	}

	// 5. Verify, then rename.
	say("Checking SHA-256 of %s (this takes a while for big files) ...", name)
	got, err := fileSHA256(part)
	if err != nil {
		return Result{}, fail(KindFailed, "%v", err)
	}
	if got != want {
		os.Remove(part)
		return Result{}, fail(KindBadHash, "SHA-256 mismatch for %s : expected %s, got %s. The bad .part file was deleted.", name, want, got)
	}
	// os.Rename replaces a file on every system, so check again: a file that
	// appeared during the download is not this package's file.
	if _, err := os.Lstat(dest); err == nil {
		return Result{}, fail(KindConflict, "Refusing to overwrite %s : the file appeared during the download. Move or delete it yourself.", dest)
	}
	if err := os.Rename(part, dest); err != nil {
		return Result{}, fail(KindFailed, "%v", err)
	}
	say("OK %s verified: %d bytes, SHA-256 %s", name, o.ExpectedBytes, got)
	return Result{Path: dest}, nil
}

// statusError is an HTTP answer that is not 200 or 206.
type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprintf("the server returned error: %d", e.code) }

// why is the reason in an "Attempt ... did not finish" line. For an HTTP error
// it holds "error: <code>", the text that horus/models.py reads.
func why(err error, status int) string {
	var se statusError
	switch {
	case errors.As(err, &se):
		return se.Error()
	case err == nil:
		return fmt.Sprintf("HTTP %d", status)
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err // the URL is already in no line; keep it so
	}
	return strings.ReplaceAll(err.Error(), "\n", " ")
}

// fetch makes one request and appends to part. It returns the HTTP status (0
// when no answer came).
func fetch(ctx context.Context, client *http.Client, u *url.URL, token, part string, have, total int64) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}
	if token != "" && TokenAllowed(u) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	flags := os.O_WRONLY | os.O_CREATE
	switch {
	case resp.StatusCode == http.StatusPartialContent && have > 0:
		start, ok := rangeStart(resp.Header.Get("Content-Range"))
		if !ok || start != have {
			return resp.StatusCode, fmt.Errorf("the server resumed at a wrong offset (%s)", resp.Header.Get("Content-Range"))
		}
		flags |= os.O_APPEND
	case resp.StatusCode == http.StatusOK:
		flags |= os.O_TRUNC // the server ignored the Range header: start again
		have = 0
	default:
		return resp.StatusCode, statusError{resp.StatusCode}
	}
	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return resp.StatusCode, err
	}
	// Read one byte more than the rest, so that a too long answer is seen.
	n, cerr := io.Copy(f, io.LimitReader(resp.Body, total-have+1))
	if err := f.Close(); cerr == nil {
		cerr = err
	}
	if cerr == nil && have+n > total {
		cerr = fmt.Errorf("the server sent more than %d bytes", total)
	}
	return resp.StatusCode, cerr
}

func rangeStart(cr string) (int64, bool) {
	// "bytes <start>-<end>/<size>"
	rest, ok := strings.CutPrefix(cr, "bytes ")
	if !ok {
		return 0, false
	}
	s, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

// TokenAllowed reports if a request to u may carry the token: https, and the
// host is huggingface.co or ends with .huggingface.co. It reads the parsed
// host, so evil-huggingface.co and huggingface.co.evil.example are refused.
func TokenAllowed(u *url.URL) bool {
	if u.Scheme != "https" {
		return false
	}
	h := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	return h == "huggingface.co" || strings.HasSuffix(h, ".huggingface.co")
}

// newClient follows redirects (as curl --location did) and removes the token
// header from each request whose address may not carry it.
func newClient(rt http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: rt,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if !TokenAllowed(req.URL) {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}
}

// readToken reads the token: the environment variable, then the .env file in
// the repo root. It is the only place in horus that reads HF_TOKEN.
func readToken(name, root string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	if root == "" {
		root = settings.Root()
	}
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		return ""
	}
	return settings.ParseEnv(data)[name]
}

func size(path string) int64 {
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return fi.Size()
	}
	return 0
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// existingAncestor is the nearest folder at or above dir that exists, so that
// the free space of a folder that is not made yet can be read.
func existingAncestor(dir string) string {
	for d := dir; ; {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			return dir
		}
		d = p
	}
}
