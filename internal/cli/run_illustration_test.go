// Tests of the handler of horus run-illustration that need no image service.
// The tests that make an image run in cmd/horus, through the wiring.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hieplam/horus/internal/generate"
)

// noImages is an image service that must not be called.
func noImages(t *testing.T) func(string) Images {
	return func(string) Images {
		t.Error("the command asked for the image service")
		return nil
	}
}

func TestFlagErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, args := range [][]string{nil, {"-subject", "a", "-subject-file", "b"}, {"-nope"}} {
		if code := RunIllustration(args, &stdout, &stderr, noImages(t)); code != 2 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if code := RunIllustration([]string{"-subject-file", filepath.Join(t.TempDir(), "absent.txt")}, &stdout, &stderr, noImages(t)); code != 1 {
		t.Errorf("absent subject file: exit %d", code)
	}
}

// fakeImages records the request and gives a fixed result.
type fakeImages struct {
	req  generate.Request
	opts generate.Options
}

func (f *fakeImages) Generate(ctx context.Context, r generate.Request, o generate.Options) (*generate.Result, error) {
	f.req, f.opts = r, o
	secs := 3.5
	return &generate.Result{Path: *r.OutPath, Seed: 7, Width: 1024, Height: 768, BackendSeconds: &secs, JobID: "job-1",
		Steps: 4, ComfyOutput: "job-1.png"}, nil
}

// TestFlagsBecomeTheRequest: -url picks the service, and the other flags
// become the request: an absolute out path, overwrite, the steps and the time
// limit. Flags that are not given leave nil, so the manifest decides.
func TestFlagsBecomeTheRequest(t *testing.T) {
	var f fakeImages
	var gotURL string
	imagesAt := func(u string) Images { gotURL = u; return &f }
	var stdout, stderr bytes.Buffer
	code := RunIllustration([]string{"-url", "http://fake:1", "-subject", "a b", "-steps", "4", "-timeout", "60", "-out", "rel.png"},
		&stdout, &stderr, imagesAt)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	abs, _ := filepath.Abs("rel.png")
	if gotURL != "http://fake:1" || f.req.Workflow != "it-illustration" || f.req.Subject != "a b" || f.req.Seed != int64(424242) ||
		f.req.Width != nil || f.req.Height != nil || *f.req.OutPath != abs || !f.req.Overwrite || *f.req.Steps != 4 ||
		f.opts.Timeout.Seconds() != 60 {
		t.Fatalf("url %q request %+v options %+v", gotURL, f.req, f.opts)
	}
	want := `{"ok":true,"prompt_id":"job-1","out":` + jsonString(abs) + `,"seed":7,"steps":4,"width":1024,"height":768,` +
		`"execution_seconds":3.5,"comfy_output":"job-1.png"}` + "\n"
	if !reflect.DeepEqual(stdout.String(), want) {
		t.Fatalf("got  %s\nwant %s", stdout.String(), want)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
