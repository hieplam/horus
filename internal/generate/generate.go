// Package generate makes one image from one workflow.
//
// The steps:
//  1. Check the arguments against the workflow manifest (subject length, size, out_path).
//     Check does this step only, and gives a Plan. Run does the steps 2 to 7 for a Plan.
//     Generate does all the steps. An API process of the job queue checks at once, and
//     the worker runs the plan later.
//  2. Make sure that ComfyUI answers. Start it when it does not (HORUS_AUTOSTART=0 turns
//     this off, and the worker never starts it: Options.NoStart).
//  3. Convert workflow.json to the API format with the live node definitions.
//  4. Refuse early when the server does not list a model file of the graph (WORKFLOW_NOT_INSTALLED).
//  5. Set caption (the caller's subject, exactly as written), seed, size and file prefix.
//     horus adds no style text: the caller owns the look, the palette and the design system.
//  6. Submit, give the id of the job to Options.Submitted, then wait. Report progress every few seconds.
//  7. Save the PNG to out_path.
//
// A timeout or a cancelled call removes the job from the ComfyUI queue, so a
// later call does not wait behind an orphan job.
//
// The service talks to ComfyUI only through the interface ComfyUI, and starts
// it only through the interface Backend. It declares both here. cmd/horus
// gives it the adapters internal/comfy and internal/backend. A test gives it
// fakes.
package generate

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/graph"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

// ComfyUI is what the service needs from the image backend. The adapter owns
// the wire format: it makes the client_id of a job, reads /history and
// /queue, and maps a rejected graph or a failed job to a horus error. The
// service owns the steps, the wait loop, its time limits and the image file.
type ComfyUI interface {
	// IsUp reports if the server answers. Only "no answer" is false; another
	// error passes through.
	IsUp(ctx context.Context) (bool, error)
	// Address is the address of the server, for the text of an error.
	Address() string
	// ObjectInfo is the node information of the live server.
	ObjectInfo(ctx context.Context) (graph.ObjectInfo, error)
	// Submit queues the graph and returns the id of the job.
	Submit(ctx context.Context, api *graph.API) (jobID string, err error)
	// History polls the job one time: nil when the server does not list it
	// yet, the horus error of the job when it failed.
	History(ctx context.Context, jobID string) (*graph.Job, error)
	// QueueState says where the job is in the backend queue.
	QueueState(ctx context.Context, jobID string) (graph.Queue, error)
	// Cancel stops or removes the job. Errors are ignored.
	Cancel(ctx context.Context, jobID string)
	// View downloads one output image.
	View(ctx context.Context, img graph.Image) ([]byte, error)
	// Free unloads the models. Errors are ignored.
	Free(ctx context.Context)
}

// Backend is what the service needs from the ComfyUI install: a start when
// the server does not answer. cmd/horus gives the service the backend of
// internal/backend. A test gives it a fake.
type Backend interface {
	// Start starts ComfyUI and waits until its server answers. It writes its
	// text lines to out. The error of a ComfyUI that is not installed has a
	// method NotInstalled() bool that returns true.
	Start(ctx context.Context, out io.Writer) error
}

// Service makes images with one ComfyUI.
type Service struct {
	comfy   ComfyUI
	backend Backend
}

// New makes the service. cmd/horus gives it the adapters internal/comfy and
// internal/backend.
func New(c ComfyUI, b Backend) *Service { return &Service{comfy: c, backend: b} }

// Running reports if the image backend answers now. list_workflows shows it
// as backend_running.
func (s *Service) Running(ctx context.Context) (bool, error) { return s.comfy.IsUp(ctx) }

// FreeMemory unloads the models of the image backend, so that their files
// can be deleted (an open file cannot be deleted on Windows). Errors are
// ignored.
func (s *Service) FreeMemory(ctx context.Context) { s.comfy.Free(ctx) }

// Report sends one progress report. total 0 means unknown.
type Report func(ctx context.Context, progress, total float64, message string)

// The intervals of the wait loop. Tests make them short.
var (
	PollInterval   = 2 * time.Second
	ReportInterval = 10 * time.Second
	// AbsentLimit is how long a job may be neither in /history nor in /queue.
	AbsentLimit = 30 * time.Second
)

// Request holds the arguments of generate_image. Seed, Width and Height are
// nil, an int64 or a *big.Int, as the argument check of the server gives them.
type Request struct {
	Workflow  string
	Subject   string
	Seed      any
	Width     any
	Height    any
	OutPath   *string
	Overwrite bool
	// Steps overrides the step count of the sampler node for this call. The
	// tool generate_image never sets it; horus run-illustration does. nil
	// keeps the value of the workflow.
	Steps *int64
}

// Result is the result of generate_image, in the key order that the tool shows.
type Result struct {
	Workflow       string   `json:"workflow"`
	Path           string   `json:"path"`
	Bytes          int64    `json:"bytes"`
	Width          int64    `json:"width"`
	Height         int64    `json:"height"`
	Seed           uint64   `json:"seed"`
	Seconds        float64  `json:"seconds"`
	BackendSeconds *float64 `json:"backend_seconds"`
	StartedBackend bool     `json:"started_backend"`
	// JobID is the id of the job: the id of the job in ComfyUI on the direct
	// path, and the horus job id in the stack.
	JobID string `json:"job_id"`
	// BackendJobID is the id of the job in ComfyUI.
	BackendJobID string `json:"backend_job_id"`
	// QueueSeconds is the wait of the job in the horus queue, from its insert
	// to the start of its run. nil on the direct path: no horus queue.
	QueueSeconds *float64 `json:"queue_seconds"`

	// Not part of the tool result (json "-"): the step count that ran and the
	// file name that ComfyUI reported. horus run-illustration prints them.
	Steps       int64  `json:"-"`
	ComfyOutput string `json:"-"`
}

// Plan is a request that passed the argument check: the subject with its
// spaces joined, the seed, the size after the defaults and the output path.
// Check makes it, and Run runs it. The job queue keeps it as the payload of a
// job, so its JSON keeps each value exactly.
type Plan struct {
	Workflow string `json:"workflow"`
	Subject  string `json:"subject"`
	Seed     uint64 `json:"seed"`
	Width    int64  `json:"width"`
	Height   int64  `json:"height"`
	// Path is the absolute path of the PNG.
	Path string `json:"path"`
	// Steps overrides the step count of the sampler node, as Request.Steps.
	Steps *int64 `json:"steps,omitempty"`
}

type outputRules struct {
	DefaultWidth  int64 `json:"default_width"`
	DefaultHeight int64 `json:"default_height"`
	MinSide       int64 `json:"min_side"`
	MaxSide       int64 `json:"max_side"`
	MultipleOf    int64 `json:"multiple_of"`
}

type subjectRules struct {
	MinChars *int64          `json:"min_chars"`
	MaxChars *int64          `json:"max_chars"`
	Rules    json.RawMessage `json:"rules"`
}

func invalid(msg string, details map[string]any) *herr.Error {
	return herr.New("INVALID_ARGUMENT", msg, details)
}

// toBig reads an int64 or a *big.Int. ok is false for any other value.
func toBig(v any) (*big.Int, bool) {
	switch x := v.(type) {
	case int64:
		return big.NewInt(x), true
	case int:
		return big.NewInt(int64(x)), true
	case *big.Int:
		return x, true
	}
	return nil, false
}

var maxSeed = new(big.Int).Lsh(big.NewInt(1), 64)

// validate checks the arguments against the manifest of w.
func validate(w *catalog.Workflow, r Request) (Plan, error) {
	var out outputRules
	var rules subjectRules
	if err := w.Manifest.Decode("output", &out); err != nil {
		return Plan{}, fmt.Errorf("manifest.json of %s: output: %w", w.Name, err)
	}
	if err := w.Manifest.Decode("subject", &rules); err != nil {
		return Plan{}, fmt.Errorf("manifest.json of %s: subject: %w", w.Name, err)
	}
	subject := strings.Join(strings.Fields(r.Subject), " ")
	lo, hi := int64(1), int64(4000)
	if rules.MinChars != nil {
		lo = *rules.MinChars
	}
	if rules.MaxChars != nil {
		hi = *rules.MaxChars
	}
	n := int64(utf8.RuneCountInString(subject))
	if n < lo || n > hi {
		var list any = []any{}
		if len(rules.Rules) > 0 {
			_ = json.Unmarshal(rules.Rules, &list)
		}
		return Plan{}, invalid(fmt.Sprintf("subject has %d characters; allowed: %d to %d", n, lo, hi),
			map[string]any{"field": "subject", "min_chars": lo, "max_chars": hi, "got": n, "rules": list})
	}
	c := Plan{Workflow: w.Name, Subject: subject, Steps: r.Steps}
	for _, x := range []struct {
		field string
		value any
		def   int64
		dst   *int64
	}{{"width", r.Width, out.DefaultWidth, &c.Width}, {"height", r.Height, out.DefaultHeight, &c.Height}} {
		value := x.value
		if value == nil {
			value = x.def
		}
		b, ok := toBig(value)
		if !ok {
			return Plan{}, invalid(x.field+" must be an integer", map[string]any{"field": x.field})
		}
		if !b.IsInt64() || b.Int64() < out.MinSide || b.Int64() > out.MaxSide || out.MultipleOf == 0 || b.Int64()%out.MultipleOf != 0 {
			return Plan{}, invalid(fmt.Sprintf("%s %s is not allowed", x.field, b.String()),
				map[string]any{"field": x.field, "got": json.Number(b.String()), "min": out.MinSide, "max": out.MaxSide,
					"multiple_of": out.MultipleOf})
		}
		*x.dst = b.Int64()
	}
	seed := r.Seed
	if seed == nil {
		seed = randomSeed()
	}
	sb, ok := toBig(seed)
	if !ok || sb.Sign() < 0 || sb.Cmp(maxSeed) >= 0 {
		return Plan{}, invalid("seed must be an integer from 0 to 2^64-1", map[string]any{"field": "seed"})
	}
	c.Seed = sb.Uint64()
	if r.OutPath == nil || *r.OutPath == "" {
		stamp := time.Now().UTC().Format("20060102T150405Z")
		c.Path = filepath.Join(settings.OutputDir(), w.Name, fmt.Sprintf("%s-%d.png", stamp, c.Seed))
	} else {
		p := *r.OutPath
		if !filepath.IsAbs(p) {
			return Plan{}, invalid("out_path must be an absolute path", map[string]any{"field": "out_path", "got": p})
		}
		if strings.ToLower(pySuffix(p)) != ".png" {
			return Plan{}, invalid("out_path must end with .png", map[string]any{"field": "out_path", "got": p})
		}
		c.Path = filepath.Clean(p)
	}
	if _, err := os.Stat(c.Path); err == nil && !r.Overwrite {
		return Plan{}, herr.New("OUTPUT_EXISTS", c.Path+" exists", map[string]any{"path": c.Path})
	}
	return c, nil
}

// pySuffix is PurePath.suffix: the last ".x" of the file name, but not a
// leading dot and not a name that ends with a dot.
func pySuffix(p string) string {
	name := filepath.Base(p)
	i := strings.LastIndex(name, ".")
	if i <= 0 || i == len(name)-1 {
		return ""
	}
	return name[i:]
}

// randomSeed is random.randrange(0, 2**32).
func randomSeed() int64 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return int64(binary.LittleEndian.Uint32(b[:]))
}

// nodeID returns the one node of the class.
func nodeID(api *graph.API, class string) (string, error) {
	var ids []string
	for _, id := range api.IDs() {
		if api.Node(id).ClassType == class {
			ids = append(ids, id)
		}
	}
	if len(ids) != 1 {
		return "", herr.New("INTERNAL", fmt.Sprintf("the graph must have exactly one %s node, it has %d", class, len(ids)), nil)
	}
	return ids[0], nil
}

// pyQuote is repr() of a str with plain characters: 'name'.
func pyQuote(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "'", `\'`) + "'"
}

// roundInt rounds x to the nearest whole number, and a half to the even one.
func roundInt(x float64) float64 { return math.RoundToEven(x) }

// Options are the parts of a call that the server, the worker or a test sets.
type Options struct {
	Report Report
	// Timeout is the time limit of the job. 0 means HORUS_GENERATE_TIMEOUT.
	Timeout time.Duration
	// NoStart: never start ComfyUI. A ComfyUI that does not answer gives
	// BACKEND_UNAVAILABLE at once. The worker sets it: the owner starts
	// ComfyUI.
	NoStart bool
	// Submitted, when it is not nil, gets the id of the job in ComfyUI after
	// the submit and before the wait. The worker saves it as backend_job_id.
	Submitted func(ctx context.Context, backendJobID string)
}

// Generate makes one image: Check, then Run.
func (s *Service) Generate(ctx context.Context, r Request, o Options) (*Result, error) {
	t0 := time.Now()
	w, p, err := check(r)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, w, p, o, t0)
}

// Check checks the arguments against the manifest of the workflow, and gives
// the plan of the run. It does not call ComfyUI. The default output path and
// a random seed are fixed here, so that a later Run makes the same file.
func (s *Service) Check(r Request) (Plan, error) {
	_, p, err := check(r)
	return p, err
}

// Run runs a plan that Check made: steps 2 to 7. A workflow that is gone
// gives WORKFLOW_NOT_FOUND.
func (s *Service) Run(ctx context.Context, p Plan, o Options) (*Result, error) {
	t0 := time.Now()
	w, err := catalog.Get(p.Workflow)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, w, p, o, t0)
}

// check is step 1: the workflow, its kind and the arguments.
func check(r Request) (*catalog.Workflow, Plan, error) {
	w, err := catalog.Get(r.Workflow)
	if err != nil {
		return nil, Plan{}, err
	}
	if k := w.Kind(); k != catalog.KindImage {
		e := invalid(fmt.Sprintf("workflow %s is of the kind %s: generate_image makes images only", pyQuote(w.Name), pyQuote(k)),
			map[string]any{"field": "workflow", "kind": k})
		e.NextStep = fmt.Sprintf("Call generate_speech with workflow=%s, or choose a workflow of the kind image from list_workflows.", pyQuote(w.Name))
		return nil, Plan{}, e
	}
	p, err := validate(w, r)
	if err != nil {
		return nil, Plan{}, err
	}
	return w, p, nil
}

// run is steps 2 to 7. t0 is the start of the call, for Result.Seconds.
func (s *Service) run(ctx context.Context, w *catalog.Workflow, req Plan, o Options, t0 time.Time) (*Result, error) {
	report := o.Report
	if report == nil {
		report = func(context.Context, float64, float64, string) {}
	}
	c := s.comfy
	timeout := o.Timeout
	if timeout == 0 {
		secs, err := settings.GenerateTimeout()
		if err != nil {
			return nil, err
		}
		timeout = time.Duration(secs) * time.Second
	}

	started, err := s.ensureUp(ctx, report, o.NoStart)
	if err != nil {
		return nil, err
	}
	info, err := c.ObjectInfo(ctx)
	if err != nil {
		return nil, err
	}
	graphPath, err := w.GraphPath()
	if err != nil {
		return nil, err
	}
	workflow, err := os.ReadFile(graphPath)
	if err != nil {
		return nil, err
	}
	api, err := graph.UIToAPI(workflow, info)
	if err != nil {
		return nil, err
	}
	g, err := w.Manifest.Object("graph")
	if err != nil {
		return nil, err
	}
	var expected int64
	if g.Has("node_count") {
		_ = g.Decode("node_count", &expected)
	}
	if expected != 0 && int64(api.Len()) != expected {
		return nil, herr.New("INTERNAL", fmt.Sprintf("converted %d of %d nodes", api.Len(), expected), map[string]any{"workflow": w.Name})
	}
	missing, err := graph.MissingModelFiles(api, info)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		e := herr.New("WORKFLOW_NOT_INSTALLED", "the backend does not list model files of this workflow",
			map[string]any{"workflow": w.Name, "missing": missing})
		e.NextStep = fmt.Sprintf("Call install_workflow with workflow=%s, then call generate_image again.", pyQuote(w.Name))
		return nil, e
	}

	values := map[string]any{
		"caption":         req.Subject,
		"seed":            req.Seed,
		"width":           req.Width,
		"height":          req.Height,
		"filename_prefix": fmt.Sprintf("horus-%s-%d", w.Name, time.Now().Unix()),
	}
	inputs, err := g.Object("inputs")
	if err != nil || inputs == nil {
		return nil, fmt.Errorf("manifest.json of %s has no graph.inputs", w.Name)
	}
	for _, key := range inputs.Keys() {
		var where struct {
			ClassType string `json:"class_type"`
			Input     string `json:"input"`
		}
		if err := inputs.Decode(key, &where); err != nil {
			return nil, fmt.Errorf("manifest.json of %s: graph.inputs.%s: %w", w.Name, key, err)
		}
		v, ok := values[key]
		if !ok {
			return nil, fmt.Errorf("KeyError: %s", pyQuote(key))
		}
		id, err := nodeID(api, where.ClassType)
		if err != nil {
			return nil, err
		}
		if err := api.Node(id).Inputs.Set(where.Input, v); err != nil {
			return nil, err
		}
	}

	// The step count: set it when the caller asked, and read it for the result.
	// A workflow without a KSampler node has no step count (0).
	var steps int64
	if sampler, err := nodeID(api, "KSampler"); err == nil {
		if req.Steps != nil {
			if err := api.Node(sampler).Inputs.Set("steps", *req.Steps); err != nil {
				return nil, err
			}
		}
		_ = api.Node(sampler).Inputs.Decode("steps", &steps)
	} else if req.Steps != nil {
		return nil, err
	}

	promptID, err := c.Submit(ctx, api)
	if err != nil {
		return nil, err
	}
	if o.Submitted != nil {
		o.Submitted(ctx, promptID)
	}
	report(ctx, 0, 0, fmt.Sprintf("job %s submitted", promptID))
	job, err := wait(ctx, c, promptID, timeout, report, t0)
	if err != nil {
		if ctx.Err() != nil {
			// The call was cancelled: remove the job, with a context that still works.
			cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			c.Cancel(cctx, promptID)
			cancel()
		}
		return nil, err
	}

	images := job.Images
	if len(images) == 0 {
		return nil, herr.New("GENERATION_FAILED", "the job finished without an image", map[string]any{"job_id": promptID})
	}
	data, err := c.View(ctx, images[0])
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(req.Path), 0o755); err != nil {
		return nil, err
	}
	tmp := req.Path + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, req.Path); err != nil {
		return nil, err
	}
	return &Result{
		Workflow:       w.Name,
		Path:           req.Path,
		Bytes:          int64(len(data)),
		Width:          req.Width,
		Height:         req.Height,
		Seed:           req.Seed,
		Seconds:        graph.Round1(time.Since(t0).Seconds()),
		BackendSeconds: job.BackendSeconds,
		StartedBackend: started,
		JobID:          promptID,
		BackendJobID:   promptID,
		Steps:          steps,
		ComfyOutput:    images[0].Filename,
	}, nil
}

// startTailLines is how many of the last lines of a failed start go into
// the detail start_output_tail.
const startTailLines = 8

// ensureUp makes sure that ComfyUI answers. When it does not, and
// HORUS_AUTOSTART is on, it starts ComfyUI through the backend one time. It
// reports true when this call started ComfyUI. A failed start is
// BACKEND_NOT_INSTALLED for a ComfyUI that is not installed, else
// BACKEND_UNAVAILABLE, with the last lines of the start in the details. With
// noStart, a ComfyUI that does not answer is BACKEND_UNAVAILABLE at once.
func (s *Service) ensureUp(ctx context.Context, report Report, noStart bool) (bool, error) {
	c := s.comfy
	up, err := c.IsUp(ctx)
	if err != nil || up {
		return false, err
	}
	if noStart {
		return false, herr.New("BACKEND_UNAVAILABLE", fmt.Sprintf("ComfyUI does not answer at %s, and the worker never starts it", c.Address()), nil)
	}
	report(ctx, 0, 0, "starting the image backend (ComfyUI); the first start takes 1 to 3 minutes")
	if !settings.Autostart() {
		return false, herr.New("BACKEND_UNAVAILABLE", fmt.Sprintf("ComfyUI does not answer at %s, and HORUS_AUTOSTART is off", c.Address()), nil)
	}
	var out bytes.Buffer
	startErr := s.backend.Start(ctx, &out)
	if startErr != nil && ctx.Err() != nil {
		return false, ctx.Err()
	}
	if startErr == nil {
		if up, err = c.IsUp(ctx); err != nil {
			return false, err
		}
		if up {
			return true, nil
		}
	}
	text := out.String()
	if startErr != nil {
		text += startErr.Error()
	}
	code := "BACKEND_UNAVAILABLE"
	var ni interface{ NotInstalled() bool }
	if errors.As(startErr, &ni) && ni.NotInstalled() {
		code = "BACKEND_NOT_INSTALLED"
	}
	return false, herr.New(code, "horus could not start ComfyUI at "+c.Address(), map[string]any{"start_output_tail": tail(text, startTailLines)})
}

// tail gives the last n lines of text, without line ends and without the
// empty last line.
func tail(text string, n int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{}
	}
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func seconds(d time.Duration) string {
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d/time.Second), 10)
	}
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64)
}

// wait polls the job every PollInterval until it is done, it fails, the time
// limit passes or the call is cancelled. A failed job gives the error that
// the adapter mapped.
func wait(ctx context.Context, c ComfyUI, promptID string, timeout time.Duration, report Report, t0 time.Time) (*graph.Job, error) {
	deadline := time.Now().Add(timeout)
	var lastReport time.Time // zero: no report yet, so the first poll reports
	var absentSince time.Time
	for {
		entry, err := c.History(ctx, promptID)
		if err != nil {
			return nil, err
		}
		if entry != nil && entry.Done {
			return entry, nil
		}
		now := time.Now()
		if now.After(deadline) {
			c.Cancel(ctx, promptID)
			return nil, herr.New("GENERATION_TIMEOUT", fmt.Sprintf("the job did not finish in %s s", seconds(timeout)),
				map[string]any{"job_id": promptID, "timeout_seconds": json.Number(seconds(timeout))})
		}
		due := lastReport.IsZero() || now.Sub(lastReport) >= ReportInterval
		if due || entry == nil {
			state, err := c.QueueState(ctx, promptID)
			if err != nil {
				return nil, err
			}
			if entry == nil && state.Absent() {
				if absentSince.IsZero() {
					absentSince = now
				}
				if now.Sub(absentSince) > AbsentLimit {
					e := herr.New("GENERATION_FAILED", "the job left the backend queue without a result", map[string]any{"job_id": promptID})
					e.NextStep = "The backend restarted or someone cleared its queue. Call generate_image again."
					return nil, e
				}
			} else {
				absentSince = time.Time{}
			}
			if due {
				elapsed := roundInt(now.Sub(t0).Seconds())
				text := "finishing"
				if state.Running {
					text = "rendering"
				} else if state.Pending() {
					text = fmt.Sprintf("waiting in the backend queue, position %d", state.Position)
				}
				report(ctx, elapsed, 0, fmt.Sprintf("%s, %d s", text, int64(elapsed)))
				lastReport = now
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(PollInterval):
		}
	}
}
