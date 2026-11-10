// Package generate makes one image from one workflow. It ports horus/generate.py.
//
// The steps:
//  1. Check the arguments against the workflow manifest (subject length, size, out_path).
//  2. Make sure that ComfyUI answers. Start it when it does not (HORUS_AUTOSTART=0 turns this off).
//  3. Convert workflow.json to the API format with the live node definitions.
//  4. Refuse early when the server does not list a model file of the graph (WORKFLOW_NOT_INSTALLED).
//  5. Set caption (the caller's subject, exactly as written), seed, size and file prefix.
//     horus adds no style text: the caller owns the look, the palette and the design system.
//  6. Submit, then wait. Report progress every few seconds.
//  7. Save the PNG to out_path.
//
// A timeout or a cancelled call removes the job from the ComfyUI queue, so a
// later call does not wait behind an orphan job.
package generate

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/comfy"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

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

// Result is the result of generate_image, in the key order of ImageResult of server.py.
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
	JobID          string   `json:"job_id"`

	// Not part of the tool result (json "-"): the step count that ran and the
	// file name that ComfyUI reported. horus run-illustration prints them.
	Steps       int64  `json:"-"`
	ComfyOutput string `json:"-"`
}

// checked is the output of Validate.
type checked struct {
	subject       string
	seed          uint64
	path          string
	width, height int64
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

// Validate checks the arguments against the manifest of w.
func Validate(w *catalog.Workflow, r Request) (*checked, error) {
	var out outputRules
	var rules subjectRules
	if err := w.Manifest.Decode("output", &out); err != nil {
		return nil, fmt.Errorf("manifest.json of %s: output: %w", w.Name, err)
	}
	if err := w.Manifest.Decode("subject", &rules); err != nil {
		return nil, fmt.Errorf("manifest.json of %s: subject: %w", w.Name, err)
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
		return nil, invalid(fmt.Sprintf("subject has %d characters; allowed: %d to %d", n, lo, hi),
			map[string]any{"field": "subject", "min_chars": lo, "max_chars": hi, "got": n, "rules": list})
	}
	c := &checked{subject: subject}
	for _, x := range []struct {
		field string
		value any
		def   int64
		dst   *int64
	}{{"width", r.Width, out.DefaultWidth, &c.width}, {"height", r.Height, out.DefaultHeight, &c.height}} {
		value := x.value
		if value == nil {
			value = x.def
		}
		b, ok := toBig(value)
		if !ok {
			return nil, invalid(x.field+" must be an integer", map[string]any{"field": x.field})
		}
		if !b.IsInt64() || b.Int64() < out.MinSide || b.Int64() > out.MaxSide || out.MultipleOf == 0 || b.Int64()%out.MultipleOf != 0 {
			return nil, invalid(fmt.Sprintf("%s %s is not allowed", x.field, b.String()),
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
		return nil, invalid("seed must be an integer from 0 to 2^64-1", map[string]any{"field": "seed"})
	}
	c.seed = sb.Uint64()
	if r.OutPath == nil || *r.OutPath == "" {
		stamp := time.Now().UTC().Format("20060102T150405Z")
		c.path = filepath.Join(settings.OutputDir(), w.Name, fmt.Sprintf("%s-%d.png", stamp, c.seed))
	} else {
		p := *r.OutPath
		if !filepath.IsAbs(p) {
			return nil, invalid("out_path must be an absolute path", map[string]any{"field": "out_path", "got": p})
		}
		if strings.ToLower(pySuffix(p)) != ".png" {
			return nil, invalid("out_path must end with .png", map[string]any{"field": "out_path", "got": p})
		}
		c.path = filepath.Clean(p)
	}
	if _, err := os.Stat(c.path); err == nil && !r.Overwrite {
		return nil, herr.New("OUTPUT_EXISTS", c.path+" exists", map[string]any{"path": c.path})
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

func uuid4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// nodeID returns the one node of the class.
func nodeID(api *comfy.API, class string) (string, error) {
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

// round1 is round(x, 1); roundInt is round(x) of Python (half to even).
func round1(x float64) float64   { return comfy.Round1(x) }
func roundInt(x float64) float64 { return math.RoundToEven(x) }

// Options are the parts of a call that the server or a test sets.
type Options struct {
	Report Report
	Client *comfy.Client
	// Timeout is the time limit of the job. 0 means HORUS_GENERATE_TIMEOUT.
	Timeout time.Duration
}

// Generate makes one image.
func Generate(ctx context.Context, r Request, o Options) (*Result, error) {
	t0 := time.Now()
	w, err := catalog.Get(r.Workflow)
	if err != nil {
		return nil, err
	}
	req, err := Validate(w, r)
	if err != nil {
		return nil, err
	}
	report := o.Report
	if report == nil {
		report = func(context.Context, float64, float64, string) {}
	}
	c := o.Client
	if c == nil {
		c = comfy.New("")
	}
	timeout := o.Timeout
	if timeout == 0 {
		secs, err := settings.GenerateTimeout()
		if err != nil {
			return nil, err
		}
		timeout = time.Duration(secs) * time.Second
	}

	up, err := c.IsUp(ctx)
	if err != nil {
		return nil, err
	}
	if !up {
		report(ctx, 0, 0, "starting the image backend (ComfyUI); the first start takes 1 to 3 minutes")
	}
	started, err := c.EnsureUp(ctx)
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
	graph, err := os.ReadFile(graphPath)
	if err != nil {
		return nil, err
	}
	api, err := comfy.UIToAPI(graph, info)
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
	missing, err := comfy.MissingModelFiles(api, info)
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
		"caption":         req.subject,
		"seed":            req.seed,
		"width":           req.width,
		"height":          req.height,
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
		if r.Steps != nil {
			if err := api.Node(sampler).Inputs.Set("steps", *r.Steps); err != nil {
				return nil, err
			}
		}
		_ = api.Node(sampler).Inputs.Decode("steps", &steps)
	} else if r.Steps != nil {
		return nil, err
	}

	promptID, err := c.Submit(ctx, api, uuid4())
	if err != nil {
		return nil, err
	}
	report(ctx, 0, 0, fmt.Sprintf("job %s submitted", promptID))
	entry, err := wait(ctx, c, promptID, timeout, report, t0)
	if err != nil {
		if ctx.Err() != nil {
			// The call was cancelled: remove the job, with a context that still works.
			cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			c.Cancel(cctx, promptID)
			cancel()
		}
		return nil, err
	}

	images := entry.Images()
	if len(images) == 0 {
		return nil, herr.New("GENERATION_FAILED", "the job finished without an image", map[string]any{"job_id": promptID})
	}
	data, err := c.View(ctx, images[0])
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(req.path), 0o755); err != nil {
		return nil, err
	}
	tmp := req.path + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, req.path); err != nil {
		return nil, err
	}
	return &Result{
		Workflow:       w.Name,
		Path:           req.path,
		Bytes:          int64(len(data)),
		Width:          req.width,
		Height:         req.height,
		Seed:           req.seed,
		Seconds:        round1(time.Since(t0).Seconds()),
		BackendSeconds: comfy.ExecutionSeconds(entry),
		StartedBackend: started,
		JobID:          promptID,
		Steps:          steps,
		ComfyOutput:    images[0].Filename,
	}, nil
}

func seconds(d time.Duration) string {
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d/time.Second), 10)
	}
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64)
}

func wait(ctx context.Context, c *comfy.Client, promptID string, timeout time.Duration, report Report, t0 time.Time) (*comfy.Entry, error) {
	deadline := time.Now().Add(timeout)
	var lastReport time.Time // zero: no report yet, so the first poll reports
	var absentSince time.Time
	for {
		entry, err := c.History(ctx, promptID)
		if err != nil {
			return nil, err
		}
		var status comfy.Status
		if entry != nil {
			status = entry.Status
		}
		if status.StatusStr == "error" {
			return nil, comfy.JobError(promptID, status)
		}
		if entry != nil && status.Completed {
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
			if entry == nil && state == "absent" {
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
				if state == "running" {
					text = "rendering"
				} else if strings.HasPrefix(state, "pending") {
					text = "waiting in the backend queue, position " + strings.SplitN(state, ":", 2)[1]
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
