// horus run-illustration makes one image without MCP. It ports
// scripts/run-illustration.py.
//
// It calls the image service internal/generate, the same code as the tool
// generate_image, so it holds no second copy of the steps (convert the graph,
// submit, wait, download). The prompt that the model receives is the subject,
// exactly as written. To wrap a style around the subject, pass -style-file
// with a JSON file {"prefix": ..., "suffix": ...} that you own.
//
// On success it prints one JSON line and exits 0:
//
//	{"ok":true,"prompt_id":...,"out":...,"seed":...,"steps":...,"width":...,"height":...,"execution_seconds":...,"comfy_output":...}
//
// On failure it prints {"ok":false,"error":...} to stderr and exits 1.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/settings"
)

// Images is what run-illustration needs from the image service.
// cmd/horus gives it a *generate.Service.
type Images interface {
	Generate(ctx context.Context, r generate.Request, o generate.Options) (*generate.Result, error)
}

// IllustrationOptions is everything that Illustrate needs.
type IllustrationOptions struct {
	Subject  string
	Workflow string
	Out      string
	Seed     int64
	Width    any // nil: the default of the manifest
	Height   any
	// Steps overrides the step count of the sampler. nil: the value of the workflow.
	Steps *int64
	// StylePath is a JSON file {prefix, suffix}. Empty: no style.
	StylePath string
	Timeout   time.Duration
}

// IllustrationResult is the JSON line of a success.
type IllustrationResult struct {
	OK               bool     `json:"ok"`
	PromptID         string   `json:"prompt_id"`
	Out              string   `json:"out"`
	Seed             uint64   `json:"seed"`
	Steps            int64    `json:"steps"`
	Width            int64    `json:"width"`
	Height           int64    `json:"height"`
	ExecutionSeconds *float64 `json:"execution_seconds"`
	ComfyOutput      string   `json:"comfy_output"`
}

// illustrationPrompt is the subject, or the caller's prefix + subject + suffix.
func illustrationPrompt(subject, stylePath string) (string, error) {
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

// Illustrate makes one image through the image service.
func Illustrate(ctx context.Context, images Images, o IllustrationOptions) (*IllustrationResult, error) {
	prompt, err := illustrationPrompt(o.Subject, o.StylePath)
	if err != nil {
		return nil, err
	}
	out, err := filepath.Abs(o.Out)
	if err != nil {
		return nil, err
	}
	res, err := images.Generate(ctx, generate.Request{
		Workflow: o.Workflow, Subject: prompt, Seed: o.Seed, Width: o.Width, Height: o.Height,
		OutPath: &out, Overwrite: true, Steps: o.Steps,
	}, generate.Options{Timeout: o.Timeout})
	if err != nil {
		return nil, err
	}
	return &IllustrationResult{OK: true, PromptID: res.JobID, Out: res.Path, Seed: res.Seed, Steps: res.Steps,
		Width: res.Width, Height: res.Height, ExecutionSeconds: res.BackendSeconds, ComfyOutput: res.ComfyOutput}, nil
}

// illustrationWorkflow turns the value of -workflow into a workflow name. A
// plain name stays as it is. The path of a workflow.json inside a workflow
// folder becomes the name of that folder, and the parent of the folder becomes
// the workflows folder of this process, as the old script ran the file that it
// was given.
func illustrationWorkflow(v string) (string, error) {
	if !strings.ContainsAny(v, `/\`) && !strings.HasSuffix(strings.ToLower(v), ".json") {
		return v, nil
	}
	abs, err := filepath.Abs(v)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	dir := filepath.Dir(abs)
	if filepath.Base(abs) != "workflow.json" {
		return "", fmt.Errorf("%s: the path must end in workflow.json", v)
	}
	parent := filepath.Dir(dir)
	if filepath.Clean(parent) != filepath.Clean(settings.WorkflowsDir()) {
		os.Setenv("HORUS_WORKFLOWS_DIR", parent)
		settings.Reset()
	}
	return filepath.Base(dir), nil
}

// RunIllustration is the command horus run-illustration. imagesAt gives the
// image service for the value of -url (empty: the HORUS_COMFYUI_URL
// setting). It returns the exit code.
func RunIllustration(args []string, stdout, stderr io.Writer, imagesAt func(url string) Images) int {
	fs := flag.NewFlagSet("horus run-illustration", flag.ContinueOnError)
	fs.SetOutput(stderr)
	subject := fs.String("subject", "", "the whole prompt: content and style, sent as written")
	subjectFile := fs.String("subject-file", "", "a text file that holds the subject")
	out := fs.String("out", "illustration.png", "where to save the PNG")
	seed := fs.Int64("seed", 424242, "the seed")
	width := fs.Int64("width", 0, "width in pixels (default: the width of the manifest)")
	height := fs.Int64("height", 0, "height in pixels (default: the height of the manifest)")
	url := fs.String("url", "", "ComfyUI URL (default: HORUS_COMFYUI_URL, then http://127.0.0.1:8188)")
	steps := fs.Int64("steps", 0, "sampler steps (default: the value in the workflow)")
	workflow := fs.String("workflow", "it-illustration", "name of a workflow under workflows/, or the path of its workflow.json")
	style := fs.String("style-file", "", "optional JSON {prefix, suffix} that you own; default: no style")
	timeout := fs.Int("timeout", 1800, "seconds to wait for the job")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	name, err := illustrationWorkflow(*workflow)
	if err != nil {
		return fail(err)
	}
	o := IllustrationOptions{Subject: text, Workflow: name, Out: *out, Seed: *seed, StylePath: *style,
		Timeout: time.Duration(*timeout) * time.Second}
	if *steps != 0 {
		o.Steps = steps
	}
	if *width != 0 {
		o.Width = *width
	}
	if *height != 0 {
		o.Height = *height
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res, err := Illustrate(ctx, imagesAt(*url), o)
	if err != nil {
		return fail(err)
	}
	b, _ := json.Marshal(res)
	fmt.Fprintln(stdout, string(b))
	return 0
}
