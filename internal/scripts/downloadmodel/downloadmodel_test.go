// Tests of the port of scripts/download-model.ps1. Each test runs against a
// local fake HTTPS server on an ephemeral port, in a temporary folder. No test
// reaches the network: the transport sends every host name to the fake server.
package downloadmodel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeToken is a made-up value. It is not a real token and has no token shape.
const fakeToken = "made-up-test-token-value-0000"

const tokenVar = "HORUS_TEST_DOWNLOAD_TOKEN"

// seen is one request that the fake server got.
type seen struct {
	Host, URI, Auth, Range string
}

type fake struct {
	t       *testing.T
	srv     *httptest.Server
	mu      sync.Mutex
	reqs    []seen
	handler func(w http.ResponseWriter, r *http.Request, n int) // n counts the requests from 1
}

// newFake starts the fake server. Every host name dials to it.
func newFake(t *testing.T, h func(w http.ResponseWriter, r *http.Request, n int)) *fake {
	f := &fake{t: t, handler: h}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.reqs = append(f.reqs, seen{r.Host, r.RequestURI, r.Header.Get("Authorization"), r.Header.Get("Range")})
		n := len(f.reqs)
		f.mu.Unlock()
		f.handler(w, r, n)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fake) requests() []seen {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]seen(nil), f.reqs...)
}

// transport dials every address to the fake server. The test certificate is
// not made for huggingface.co, so this test-only transport skips the check.
func (f *fake) transport() http.RoundTripper {
	addr := f.srv.Listener.Addr().String()
	return &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

// serve answers with body, and with a 206 answer for a Range header.
func serve(w http.ResponseWriter, r *http.Request, body []byte) {
	start := 0
	if rg := r.Header.Get("Range"); rg != "" {
		s, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
		if s >= len(body) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		start = s
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, len(body)-1, len(body)))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)-s))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}
	w.Write(body[start:])
}

func content(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + i/251)
	}
	return b
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// job is one download into a fresh temporary folder, with no token, no sleep
// and plenty of free space, unless the test changes it.
type job struct {
	Options
	out    *bytes.Buffer
	sleeps []time.Duration
}

func newJob(t *testing.T, f *fake, url string, body []byte) *job {
	t.Setenv(tokenVar, "")
	j := &job{out: &bytes.Buffer{}}
	j.Options = Options{
		URL:           url,
		Dest:          filepath.Join(t.TempDir(), "models", "vae", "model.safetensors"),
		ExpectedBytes: int64(len(body)),
		SHA256:        sum(body),
		TokenEnvVar:   tokenVar,
		Root:          t.TempDir(), // an empty folder: no .env
		Transport:     f.transport(),
		FreeSpace:     func(string) (uint64, error) { return 1 << 40, nil },
	}
	j.Out = j.out
	j.Sleep = func(d time.Duration) { j.sleeps = append(j.sleeps, d) }
	return j
}

func (j *job) run(t *testing.T) (Result, error) {
	t.Helper()
	return Download(context.Background(), j.Options)
}

func kindOf(err error) (Kind, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind, true
	}
	return 0, false
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s exists, want it absent (err=%v)", filepath.Base(path), err)
	}
}

func mustHold(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
	if sum(got) != sum(want) {
		t.Errorf("%s: SHA-256 %s, want %s", filepath.Base(path), sum(got), sum(want))
	}
}

// pyOutcome is the text check of _download_error in horus/models.py, on the
// text that the item detail gets: the FAIL line and the "Attempt" lines.
func pyOutcome(text string) string {
	switch {
	case strings.Contains(text, "error: 401"):
		return "HF_TOKEN_INVALID"
	case strings.Contains(text, "error: 403"):
		return "LICENSE_NOT_ACCEPTED"
	case strings.Contains(text, "Not enough free space"):
		return "DISK_FULL"
	case strings.Contains(text, "Refusing to overwrite"):
		return "FILE_CONFLICT"
	}
	return "DOWNLOAD_FAILED"
}

// Case 1.
func TestFullDownloadWithRightSizeAndSHA256(t *testing.T) {
	body := content(300_000)
	f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) { serve(w, r, body) })
	j := newJob(t, f, "https://huggingface.co/org/repo/resolve/main/model.safetensors", body)
	res, err := j.run(t)
	if err != nil {
		t.Fatalf("Download: %v\n%s", err, j.out)
	}
	if res.Skipped || res.Path != j.Dest {
		t.Errorf("result %+v", res)
	}
	mustHold(t, j.Dest, body)
	mustNotExist(t, j.Dest+".part")
	wantOut := "Downloading model.safetensors (0.00 GiB) ...\n" +
		"Checking SHA-256 of model.safetensors (this takes a while for big files) ...\n" +
		"OK model.safetensors verified: 300000 bytes, SHA-256 " + sum(body) + "\n"
	if j.out.String() != wantOut {
		t.Errorf("output:\n%s\nwant:\n%s", j.out, wantOut)
	}
	if len(f.requests()) != 1 {
		t.Errorf("%d requests, want 1", len(f.requests()))
	}

	// A second run finds the verified file and prints SKIP.
	j.out.Reset()
	res, err = j.run(t)
	if err != nil || !res.Skipped {
		t.Fatalf("second run: %+v %v", res, err)
	}
	if !strings.Contains(j.out.String(), "SKIP model.safetensors : already present and verified.") {
		t.Errorf("second run output:\n%s", j.out)
	}
	if len(f.requests()) != 1 {
		t.Errorf("the second run made a request")
	}
}

// Case 2.
func TestResumeFromPartFile(t *testing.T) {
	body := content(200_000)
	f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) { serve(w, r, body) })
	j := newJob(t, f, "https://huggingface.co/a/b/resolve/main/model.safetensors", body)
	os.MkdirAll(filepath.Dir(j.Dest), 0o755)
	if err := os.WriteFile(j.Dest+".part", body[:75_000], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := j.run(t); err != nil {
		t.Fatalf("Download: %v\n%s", err, j.out)
	}
	reqs := f.requests()
	if len(reqs) != 1 || reqs[0].Range != "bytes=75000-" {
		t.Errorf("requests %+v, want one with Range bytes=75000-", reqs)
	}
	mustHold(t, j.Dest, body)
	mustNotExist(t, j.Dest+".part")
}

// Case 2, second half: a server that ignores the Range header sends the whole
// file again, and the .part file starts again from zero.
func TestResumeWhenServerIgnoresRange(t *testing.T) {
	body := content(50_000)
	f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) { w.Write(body) })
	j := newJob(t, f, "https://huggingface.co/a/b/resolve/main/model.safetensors", body)
	os.MkdirAll(filepath.Dir(j.Dest), 0o755)
	os.WriteFile(j.Dest+".part", body[:1000], 0o644)
	if _, err := j.run(t); err != nil {
		t.Fatalf("Download: %v\n%s", err, j.out)
	}
	mustHold(t, j.Dest, body)
}

// Case 3.
func TestRetryAfterBrokenConnection(t *testing.T) {
	body := content(400_000)
	f := newFake(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if n == 1 {
			// Promise the whole file, send a part of it, then break the connection.
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Write(body[:123_456])
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}
		serve(w, r, body)
	})
	j := newJob(t, f, "https://huggingface.co/a/b/resolve/main/model.safetensors", body)
	if _, err := j.run(t); err != nil {
		t.Fatalf("Download: %v\n%s", err, j.out)
	}
	reqs := f.requests()
	if len(reqs) != 2 || reqs[0].Range != "" || reqs[1].Range != "bytes=123456-" {
		t.Errorf("requests %+v, want a full request, then Range bytes=123456-", reqs)
	}
	if !strings.Contains(j.out.String(), "Attempt 1 of 4 did not finish (") ||
		!strings.Contains(j.out.String(), ", 123456 of 400000 bytes).") {
		t.Errorf("output:\n%s", j.out)
	}
	if len(j.sleeps) != 1 || j.sleeps[0] != 5*time.Second {
		t.Errorf("sleeps %v, want [5s]", j.sleeps)
	}
	mustHold(t, j.Dest, body)
}

// Case 4.
func TestWrongSizeIsNotPutInPlace(t *testing.T) {
	body := content(10_000)
	for name, served := range map[string][]byte{"short": body[:9_000], "long": append(append([]byte{}, body...), 1, 2, 3)} {
		t.Run(name, func(t *testing.T) {
			f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) { serve(w, r, served) })
			j := newJob(t, f, "https://huggingface.co/a/b/resolve/main/model.safetensors", body)
			_, err := j.run(t)
			if k, ok := kindOf(err); !ok || k != KindFailed {
				t.Fatalf("err %v, want KindFailed", err)
			}
			if !strings.HasPrefix(err.Error(), "Download did not finish after 4 attempts.") {
				t.Errorf("message %q", err)
			}
			if len(f.requests()) != 4 || len(j.sleeps) != 3 {
				t.Errorf("%d requests and sleeps %v, want 4 attempts", len(f.requests()), j.sleeps)
			}
			mustNotExist(t, j.Dest)
		})
	}
}

// Case 5.
func TestWrongSHA256IsNotPutInPlace(t *testing.T) {
	body := content(20_000)
	other := content(20_000)
	other[100] ^= 0xff
	f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) { serve(w, r, other) })
	j := newJob(t, f, "https://huggingface.co/a/b/resolve/main/model.safetensors", body)
	_, err := j.run(t)
	if k, ok := kindOf(err); !ok || k != KindBadHash {
		t.Fatalf("err %v, want KindBadHash", err)
	}
	want := "SHA-256 mismatch for model.safetensors : expected " + sum(body) + ", got " + sum(other) + ". The bad .part file was deleted."
	if err.Error() != want {
		t.Errorf("message\n%s\nwant\n%s", err, want)
	}
	mustNotExist(t, j.Dest)
	mustNotExist(t, j.Dest+".part")
}

// Case 6.
func TestDifferentFileWithSameNameIsNotOverwritten(t *testing.T) {
	body := content(5_000)
	for name, onDisk := range map[string][]byte{"other size": []byte("a file of the user"), "same size, other hash": content(5_000)[1:]} {
		t.Run(name, func(t *testing.T) {
			if len(onDisk) == len(body)-1 {
				onDisk = append(onDisk, 9)
			}
			f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) { serve(w, r, body) })
			j := newJob(t, f, "https://huggingface.co/a/b/resolve/main/model.safetensors", body)
			os.MkdirAll(filepath.Dir(j.Dest), 0o755)
			os.WriteFile(j.Dest, onDisk, 0o644)
			_, err := j.run(t)
			if k, ok := kindOf(err); !ok || k != KindConflict {
				t.Fatalf("err %v, want KindConflict", err)
			}
			if got := pyOutcome("FAIL: " + err.Error()); got != "FILE_CONFLICT" {
				t.Errorf("models.py reads %s from %q", got, err)
			}
			mustHold(t, j.Dest, onDisk)
			if len(f.requests()) != 0 {
				t.Errorf("a request was made")
			}
		})
	}
}

// Case 7.
func TestFreeSpaceUnderSizePlusOneGiBIsRefused(t *testing.T) {
	body := content(8_000)
	f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) { serve(w, r, body) })
	j := newJob(t, f, "https://huggingface.co/a/b/resolve/main/model.safetensors", body)
	var asked string
	j.FreeSpace = func(dir string) (uint64, error) { asked = dir; return uint64(len(body)) + SpareBytes - 1, nil }
	_, err := j.run(t)
	if k, ok := kindOf(err); !ok || k != KindDiskFull {
		t.Fatalf("err %v, want KindDiskFull", err)
	}
	if got := pyOutcome("FAIL: " + err.Error()); got != "DISK_FULL" {
		t.Errorf("models.py reads %s from %q", got, err)
	}
	if len(f.requests()) != 0 {
		t.Errorf("a request was made")
	}
	mustNotExist(t, filepath.Dir(j.Dest)) // the folder was not made either
	if fi, err := os.Stat(asked); err != nil || !fi.IsDir() {
		t.Errorf("free space was read for %q, which is not a folder", asked)
	}

	// The rest of a .part file counts, not the whole file.
	os.MkdirAll(filepath.Dir(j.Dest), 0o755)
	os.WriteFile(j.Dest+".part", body[:3_000], 0o644)
	j.FreeSpace = func(string) (uint64, error) { return uint64(len(body)-3_000) + SpareBytes, nil }
	if _, err := j.run(t); err != nil {
		t.Fatalf("exactly the rest plus 1 GiB: %v", err)
	}
	mustHold(t, j.Dest, body)
}

// Case 8.
func TestHTTP401And403GiveTwoOutcomes(t *testing.T) {
	for code, want := range map[int]struct {
		kind Kind
		py   string
	}{401: {KindUnauthorized, "HF_TOKEN_INVALID"}, 403: {KindForbidden, "LICENSE_NOT_ACCEPTED"}} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) { w.WriteHeader(code) })
			body := content(100)
			j := newJob(t, f, "https://huggingface.co/a/b/resolve/main/model.safetensors", body)
			_, err := j.run(t)
			if k, ok := kindOf(err); !ok || k != want.kind {
				t.Fatalf("err %v (kind %v), want %v", err, k, want.kind)
			}
			// The detail of workflow.ps1 keeps the lines that start with "Attempt " and "FAIL:".
			var detail []string
			for _, l := range strings.Split(j.out.String()+"FAIL: "+err.Error(), "\n") {
				if strings.HasPrefix(l, "Attempt ") || strings.HasPrefix(l, "FAIL:") {
					detail = append(detail, l)
				}
			}
			if got := pyOutcome(strings.Join(detail, " | ")); got != want.py {
				t.Errorf("models.py reads %s, want %s, from %q", got, want.py, detail)
			}
			// The hint line names both codes, so it alone is no evidence of either.
			if got := pyOutcome("The server answered with an error. HTTP 401 = token missing or wrong. HTTP 403 = license not accepted for this account."); got != "DOWNLOAD_FAILED" {
				t.Errorf("the hint line reads as %s", got)
			}
			mustNotExist(t, j.Dest)
		})
	}
}

// Case 9: the token header goes only to huggingface.co hosts over https.
func TestTokenGoesOnlyToHuggingFaceHosts(t *testing.T) {
	body := content(1_000)
	for _, c := range []struct {
		url  string
		send bool
	}{
		{"https://huggingface.co/a/b/resolve/main/m.bin", true},
		{"https://cdn-lfs.huggingface.co/a/m.bin", true},
		{"https://HuggingFace.co/a/m.bin", true},
		{"https://huggingface.co:443/a/m.bin", true},
		{"https://example.net/m.bin", false},
		{"https://evil-huggingface.co/m.bin", false},
		{"https://huggingface.co.evil.example/m.bin", false},
		{"https://evil.example/huggingface.co/m.bin", false},
		{"https://evil.example/m.bin?h=.huggingface.co", false},
	} {
		t.Run(c.url, func(t *testing.T) {
			f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) { serve(w, r, body) })
			j := newJob(t, f, c.url, body)
			t.Setenv(tokenVar, fakeToken)
			if _, err := j.run(t); err != nil {
				t.Fatalf("Download: %v\n%s", err, j.out)
			}
			reqs := f.requests()
			if len(reqs) != 1 {
				t.Fatalf("requests %+v", reqs)
			}
			got := reqs[0].Auth == "Bearer "+fakeToken
			if got != c.send || (!c.send && reqs[0].Auth != "") {
				t.Errorf("Authorization %q, want sent=%v", reqs[0].Auth, c.send)
			}
			warned := strings.Contains(j.out.String(), "is not huggingface.co. The token is not sent.")
			if warned == c.send {
				t.Errorf("warning line printed=%v, want %v:\n%s", warned, !c.send, j.out)
			}
		})
	}
	// Plain http never gets the token, also for huggingface.co: the address is refused.
	f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) {})
	j := newJob(t, f, "http://huggingface.co/m.bin", body)
	t.Setenv(tokenVar, fakeToken)
	if _, err := j.run(t); err == nil || err.Error() != "Only https addresses are allowed. Got: http" {
		t.Errorf("http address: %v", err)
	}
	// An address with a user part is refused, because net/http sends it as a
	// Basic Authorization header to that host.
	// The address is built in parts, so the lint of the public repo does not read it as an email address.
	j = newJob(t, f, "https://huggingface.co"+"@"+"evil.example/m.bin", body)
	if _, err := j.run(t); err == nil || err.Error() != "The address must not hold a user name or a password." {
		t.Errorf("address with a user part: %v", err)
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("%d requests, want 0", n)
	}
}

// Case 9: a redirect to another host does not get the token. A redirect to a
// huggingface.co host does.
func TestTokenIsDroppedOnRedirectToAnotherHost(t *testing.T) {
	body := content(2_000)
	f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		switch r.Host {
		case "huggingface.co":
			http.Redirect(w, r, "https://cdn-lfs.huggingface.co/blob", http.StatusFound)
		case "cdn-lfs.huggingface.co":
			http.Redirect(w, r, "https://cas-bridge.xethub.example/blob?sig=1", http.StatusTemporaryRedirect)
		default:
			serve(w, r, body)
		}
	})
	j := newJob(t, f, "https://huggingface.co/a/b/resolve/main/model.safetensors", body)
	t.Setenv(tokenVar, fakeToken)
	if _, err := j.run(t); err != nil {
		t.Fatalf("Download: %v\n%s", err, j.out)
	}
	reqs := f.requests()
	if len(reqs) != 3 {
		t.Fatalf("requests %+v, want 3", reqs)
	}
	for i, want := range []string{"Bearer " + fakeToken, "Bearer " + fakeToken, ""} {
		if reqs[i].Auth != want {
			t.Errorf("request %d to %s: Authorization %q, want %q", i+1, reqs[i].Host, reqs[i].Auth, want)
		}
	}
	mustHold(t, j.Dest, body)

	// A redirect from https to plain http on a huggingface.co host does not get it either.
	if TokenAllowed(mustURL(t, "http://cdn-lfs.huggingface.co/blob")) {
		t.Error("TokenAllowed accepts plain http")
	}
}

// Case 10: the token is in no URL, no argument and no output line. The
// command reads it from the environment and from .env; it has no flag for it.
func TestTokenIsInNoURLArgumentOrOutput(t *testing.T) {
	body := content(3_000)
	for _, from := range []string{"environment", ".env"} {
		for _, code := range []int{200, 401, 403} {
			t.Run(fmt.Sprintf("%s/%d", from, code), func(t *testing.T) {
				f := newFake(t, func(w http.ResponseWriter, r *http.Request, _ int) {
					switch {
					case code != 200:
						w.WriteHeader(code)
					case r.Host == "huggingface.co":
						http.Redirect(w, r, "https://other.example/x", http.StatusFound)
					default:
						serve(w, r, body)
					}
				})
				j := newJob(t, f, "", body)
				if from == "environment" {
					t.Setenv(tokenVar, fakeToken)
				} else {
					os.WriteFile(filepath.Join(j.Root, ".env"), []byte(tokenVar+"=\""+fakeToken+"\"\n"), 0o600)
				}
				args := []string{"-url", "https://huggingface.co/a/b/resolve/main/model.safetensors",
					"-destination", j.Dest, "-expected-bytes", strconv.Itoa(len(body)), "-sha256", sum(body),
					"-token-env-var", tokenVar, "-max-attempts", "2"}
				var out bytes.Buffer
				exit := Main(context.Background(), args, &out, j.Options)
				if want := map[int]int{200: 0, 401: 1, 403: 1}[code]; exit != want {
					t.Errorf("exit %d, want %d\n%s", exit, want, &out)
				}
				reqs := f.requests()
				if len(reqs) == 0 || reqs[0].Auth != "Bearer "+fakeToken {
					t.Fatalf("the token was not read from %s: %+v", from, reqs)
				}
				for _, r := range reqs {
					if strings.Contains(r.URI, fakeToken) || strings.Contains(r.Host, fakeToken) {
						t.Errorf("the token is in the URL %s%s", r.Host, r.URI)
					}
				}
				for _, a := range args {
					if strings.Contains(a, fakeToken) {
						t.Errorf("the token is in the argument %q", a)
					}
				}
				if strings.Contains(out.String(), fakeToken) {
					t.Errorf("the token is in the output:\n%s", &out)
				}
				if code == 200 {
					mustHold(t, j.Dest, body)
				}
			})
		}
	}
	// The usage text has no token value either.
	t.Setenv(tokenVar, fakeToken)
	var out bytes.Buffer
	if exit := Main(context.Background(), []string{"-h"}, &out, Options{}); exit != 0 || strings.Contains(out.String(), fakeToken) {
		t.Errorf("-h: exit %d, output:\n%s", exit, &out)
	}
}

// The flags of the command, and the exit codes of download-model.ps1.
func TestCommandFlagsAndExitCodes(t *testing.T) {
	var out bytes.Buffer
	if exit := Main(context.Background(), []string{"-url", "https://huggingface.co/x"}, &out, Options{}); exit != 1 ||
		!strings.Contains(out.String(), "FAIL: missing flag(s): [-destination -expected-bytes -sha256]") {
		t.Errorf("missing flags: exit %d\n%s", exit, &out)
	}
	out.Reset()
	if exit := Main(context.Background(), []string{"-no-such-flag"}, &out, Options{}); exit != 2 {
		t.Errorf("unknown flag: exit %d", exit)
	}
	out.Reset()
	args := []string{"-url", "https://huggingface.co/x", "-destination", filepath.Join(t.TempDir(), "m"),
		"-expected-bytes", "10", "-sha256", "abc"}
	if exit := Main(context.Background(), args, &out, Options{}); exit != 1 ||
		out.String() != "FAIL: -sha256 must be 64 hexadecimal characters.\n" {
		t.Errorf("bad hash: exit %d\n%s", exit, &out)
	}
	out.Reset()
	args[5] = "-5"
	args[7] = strings.Repeat("a", 64)
	if exit := Main(context.Background(), args, &out, Options{}); exit != 1 ||
		out.String() != "FAIL: -expected-bytes must be greater than 0.\n" {
		t.Errorf("negative size: exit %d\n%s", exit, &out)
	}
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
