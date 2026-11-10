// Package runillustration makes one image without MCP. It ports
// scripts/run-illustration.py.
//
// It calls internal/generate, the same code as the tool generate_image, so it
// holds no second copy of the steps (convert the graph, submit, wait, download).
// The prompt that the model receives is the subject, exactly as written. To
// wrap a style around the subject, pass -style-file with a JSON file
// {"prefix": ..., "suffix": ...} that you own.
//
// On success it prints one JSON line and exits 0:
//
//	{"ok":true,"prompt_id":...,"out":...,"seed":...,"width":...,"height":...,"execution_seconds":...}
//
// On failure it prints {"ok":false,"error":...} to stderr and exits 1.
package runillustration

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/hieplam/horus/internal/comfy"
	"github.com/hieplam/horus/internal/generate"
)

// Options is everything that Illustrate needs.
type Options struct {
	Subject  string
	Workflow string
	Out      string
	Seed     int64
	Width    any // nil: the default of the manifest
	Height   any
	URL      string
	// StylePath is a JSON file {prefix, suffix}. Empty: no style.
	StylePath string
	Timeout   time.Duration
}

// Result is the JSON line of a success.
type Result struct {
	OK               bool     `json:"ok"`
	PromptID         string   `json:"prompt_id"`
	Out              string   `json:"out"`
	Seed             uint64   `json:"seed"`
	Width            int64    `json:"width"`
	Height           int64    `json:"height"`
	ExecutionSeconds *float64 `json:"execution_seconds"`
}

// buildPrompt is the subject, or the caller's prefix + subject + suffix.
func buildPrompt(subject, stylePath string) (string, error) {
	if stylePath == "" {
		return subject, nil
	}
	data, err := os.ReadFile(stylePath)
	if err != nil {
		return "", err
	}
	var style struct {
		Prefix *string `json:"prefix"`
		Suffix *string `json:"suffix"`
	}
	if err := json.Unmarshal(data, &style); err != nil {
		return "", fmt.Errorf("%s: %w", stylePath, err)
	}
	if style.Prefix == nil || style.Suffix == nil {
		return "", fmt.Errorf("%s must have the keys prefix and suffix", stylePath)
	}
	return *style.Prefix + " " + subject + " " + *style.Suffix, nil
}

// Illustrate makes one image through internal/generate.
func Illustrate(ctx context.Context, o Options) (*Result, error) {
	prompt, err := buildPrompt(o.Subject, o.StylePath)
	if err != nil {
		return nil, err
	}
	out, err := filepath.Abs(o.Out)
	if err != nil {
		return nil, err
	}
	res, err := generate.Generate(ctx, generate.Request{
		Workflow: o.Workflow, Subject: prompt, Seed: o.Seed, Width: o.Width, Height: o.Height,
		OutPath: &out, Overwrite: true,
	}, generate.Options{Client: comfy.New(o.URL), Timeout: o.Timeout})
	if err != nil {
		return nil, err
	}
	return &Result{OK: true, PromptID: res.JobID, Out: res.Path, Seed: res.Seed, Width: res.Width,
		Height: res.Height, ExecutionSeconds: res.BackendSeconds}, nil
}

// Run is the command horus run-illustration. It returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("horus run-illustration", flag.ContinueOnError)
	fs.SetOutput(stderr)
	subject := fs.String("subject", "", "the whole prompt: content and style, sent as written")
	subjectFile := fs.String("subject-file", "", "a text file that holds the subject")
	out := fs.String("out", "illustration.png", "where to save the PNG")
	seed := fs.Int64("seed", 424242, "the seed")
	width := fs.Int64("width", 0, "width in pixels (default: the width of the manifest)")
	height := fs.Int64("height", 0, "height in pixels (default: the height of the manifest)")
	url := fs.String("url", "", "ComfyUI URL (default: HORUS_COMFYUI_URL, then http://127.0.0.1:8188)")
	workflow := fs.String("workflow", "it-illustration", "name of the workflow under workflows/")
	style := fs.String("style-file", "", "optional JSON {prefix, suffix} that you own; default: no style")
	timeout := fs.Int("timeout", 1800, "seconds to wait for the job")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(err error) int {
		b, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
		fmt.Fprintln(stderr, string(b))
		return 1
	}
	if (*subject == "") == (*subjectFile == "") {
		fmt.Fprintln(stderr, "horus run-illustration: pass exactly one of -subject and -subject-file")
		return 2
	}
	text := *subject
	if *subjectFile != "" {
		data, err := os.ReadFile(*subjectFile)
		if err != nil {
			return fail(err)
		}
		text = string(data)
	}
	o := Options{Subject: text, Workflow: *workflow, Out: *out, Seed: *seed, URL: *url, StylePath: *style,
		Timeout: time.Duration(*timeout) * time.Second}
	if *width != 0 {
		o.Width = *width
	}
	if *height != 0 {
		o.Height = *height
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res, err := Illustrate(ctx, o)
	if err != nil {
		return fail(err)
	}
	b, _ := json.Marshal(res)
	fmt.Fprintln(stdout, string(b))
	return 0
}
