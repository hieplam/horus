// generate_image through the job queue of the stack (spec
// docs/specs/2026-11-19-job-queue.md, "One job, step by step" and "MCP
// contract"; ADR 0025, ADR 0027 and ADR 0039). With HORUS_DATABASE_URL, the
// API process never calls ComfyUI for a job:
//
//  1. It changes the out_path of the host to the container path (Paths), and
//     checks the arguments with generate.Check: the plan of the run.
//  2. It computes the request fingerprint and the idempotency key of recipe
//     version 1 (jobs.Recipe). The dedupe is off: each call makes a job.
//  3. It submits the job (insert, publish one message), or gives QUEUE_FULL.
//  4. It waits for the job (jobs.Service.Wait), with the progress text of the
//     queue and then of the run. A cancel of the call, or the end of the
//     session, cancels the job: a queued job never runs, and the worker stops
//     a running one.
//  5. It gives the result of the job with the host path, or the error of the
//     job.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/graph"
	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// Queue is the job queue of the stack, for the tool generate_image.
type Queue struct {
	Jobs  *jobs.Service
	Paths jobs.Paths
	// QueueTimeout is HORUS_QUEUE_TIMEOUT, for the details of an expired job.
	QueueTimeout time.Duration
}

// cancelWithin is the most time of the cancel of a job after the call ended.
const cancelWithin = 10 * time.Second

// QueueState is the field queue of list_workflows in the stack.
type QueueState struct {
	Waiting int `json:"waiting"`
	Running int `json:"running"`
	Slots   int `json:"slots"`
}

// state gives the counts of the job queue now. The slots are those of the
// worker with the newest heartbeat, or 0 without a worker row.
func (q *Queue) state(ctx context.Context) (*QueueState, error) {
	sum, err := q.Jobs.Summary(ctx)
	if err != nil {
		return nil, err
	}
	s := &QueueState{Waiting: sum.Queued, Running: sum.Running}
	if sum.Worker != nil {
		s.Slots = sum.Worker.Slots
	}
	return s, nil
}

// noShared is the error of a call when the stack has no shared folder, or
// when the output folder of the stack is not in one.
func noShared(why string) *herr.Error {
	e := herr.New("BACKEND_NOT_INSTALLED", "horus in the stack cannot write a PNG that this machine can open: "+why, nil)
	e.NextStep = "Ask the user to run horus stack-mounts in the horus clone, then docker compose up -d " +
		"(docs/setup.md, section Run horus as a stack). Then call again."
	return e
}

// generate is the body of generate_image in the stack.
func (q *Queue) generate(ctx context.Context, images Images, c *call) (any, error) {
	t0 := time.Now()
	req := generate.Request{
		Workflow:  c.args.str("workflow"),
		Subject:   c.args.str("subject"),
		Seed:      c.args["seed"],
		Width:     c.args["width"],
		Height:    c.args["height"],
		Overwrite: c.args.boolean("overwrite"),
	}
	if len(q.Paths.Dirs) == 0 {
		return nil, noShared("HORUS_SHARED_DIRS has no shared folder")
	}
	host := ""
	if out := c.args.strPtr("out_path"); out != nil && *out != "" {
		container, h, ok := q.Paths.ToContainer(*out)
		if !ok {
			e := herr.New("INVALID_ARGUMENT", "out_path must be an absolute path in a shared folder of horus: "+strings.Join(q.Paths.HostDirs(), ", "),
				map[string]any{"field": "out_path", "got": *out, "shared_dirs": q.Paths.HostDirs()})
			e.NextStep = "Pass an out_path in one of the folders of details.shared_dirs, or no out_path. Then call again."
			return nil, e
		}
		host, req.OutPath = h, &container
	}
	plan, err := images.Check(req)
	if err != nil {
		return nil, err
	}
	hostOut := host
	if hostOut == "" {
		h, ok := q.Paths.ToHost(plan.Path)
		if !ok {
			return nil, noShared("the output folder " + plan.Path + " is not in a shared folder")
		}
		hostOut = h
	}
	fp, err := q.fingerprint(c, plan, host)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}
	resourceKey, err := workflowResourceKey(plan.Workflow)
	if err != nil {
		return nil, err
	}
	j, st, err := q.Jobs.SubmitStatus(ctx, jobdata.Job{
		Tool: jobdata.ToolImage, Workflow: plan.Workflow, Backend: jobdata.BackendComfyUI,
		ResourceKey: resourceKey, Payload: payload,
		RequestHash: fp.Hash, IdempotencyKey: fp.Key, Caller: callerOf(ctx, c),
	})
	var full *jobs.QueueFullError
	if errors.As(err, &full) {
		return nil, queueFull(full)
	}
	if err != nil {
		return nil, err
	}
	logf("INFO", "call generate_image: job %s queued", j.ID)
	report := rising(c.report)
	// The first message is the place at the insert: a free worker can start
	// the job before the first read of the waiter.
	report(ctx, 0, 0, st.Message)
	done, err := q.Jobs.Wait(ctx, j.ID, jobs.Report(report))
	if err != nil {
		if ctx.Err() != nil {
			// The call ended: the agent cancelled it or left. Cancel the job
			// with a context that still works.
			cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cancelWithin)
			state, cerr := q.Jobs.Cancel(cctx, j.ID)
			cancel()
			logf("INFO", "call generate_image ended: job %s cancelled, state %s %v", j.ID, state, cerr)
		}
		return nil, err
	}
	return q.result(done, hostOut, t0)
}

// rising gives the reports of r with progress values that only rise, as MCP
// needs: a value that is not above the last one becomes the last one plus
// 0.001.
func rising(r Reporter) Reporter {
	last := -1.0
	return func(ctx context.Context, progress, total float64, message string) {
		if progress <= last {
			progress = last + 0.001
		}
		last = progress
		r(ctx, progress, total, message)
	}
}

// fingerprint is the recipe of ADR 0027 for the call: the arguments as the
// caller gave them, with host the cleaned host path of out_path ("" when the
// caller gave none).
func (q *Queue) fingerprint(c *call, plan generate.Plan, host string) (jobs.Fingerprint, error) {
	w, err := catalog.Get(plan.Workflow)
	if err != nil {
		return jobs.Fingerprint{}, err
	}
	manifest, err := os.ReadFile(filepath.Join(w.Dir, "manifest.json"))
	if err != nil {
		return jobs.Fingerprint{}, err
	}
	gp, err := w.GraphPath()
	if err != nil {
		return jobs.Fingerprint{}, err
	}
	graphFile, err := os.ReadFile(gp)
	if err != nil {
		return jobs.Fingerprint{}, err
	}
	r := jobs.Request{
		Tool: jobdata.ToolImage, Workflow: plan.Workflow, WorkflowRev: jobs.WorkflowRev(manifest, graphFile),
		Subject: c.args.str("subject"), DefaultWidth: plan.Width, DefaultHeight: plan.Height,
		OutPath: host, PathStyle: q.Paths.Style, Overwrite: c.args.boolean("overwrite"),
	}
	if s, ok := c.args["seed"]; ok && s != nil {
		seed := plan.Seed // Check checked the range: 0 to 2^64-1
		r.Seed = &seed
	}
	r.Width, r.Height = c.args.intPtr("width"), c.args.intPtr("height")
	if k := c.args.strPtr("idempotency_key"); k != nil {
		r.CallerKey = *k
	}
	return jobs.Recipe(r)
}

// workflowResourceKey is the resource key of a workflow: one key for its
// model files (jobs.ResourceKey).
func workflowResourceKey(name string) (string, error) {
	w, err := catalog.Get(name)
	if err != nil {
		return "", err
	}
	m, err := w.Models()
	if err != nil {
		return "", err
	}
	sums := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		sums = append(sums, f.SHA256)
	}
	return jobs.ResourceKey(name, sums), nil
}

// queueFull is the error QUEUE_FULL of a refused submit.
func queueFull(e *jobs.QueueFullError) *herr.Error {
	return herr.New("QUEUE_FULL", e.Error(), map[string]any{
		"reason":              e.Reason,
		"retry_after_seconds": int64(e.RetryAfter / time.Second),
		"queued":              e.Queued,
		"queue_max":           e.Max,
		"estimated_start":     e.Start.UTC().Format(time.RFC3339),
		"deadline":            e.Deadline.UTC().Format(time.RFC3339),
	})
}

// result gives the result of the job that ended, with the host path of the
// PNG, or the error of the job.
func (q *Queue) result(j jobdata.Job, host string, t0 time.Time) (any, error) {
	switch j.State {
	case jobdata.Succeeded:
		var r generate.Result
		if err := json.Unmarshal(j.Result, &r); err != nil {
			return nil, herr.New("INTERNAL", "the result of the job is not a result of generate_image", map[string]any{"job_id": j.ID})
		}
		if r.BackendJobID == "" {
			r.BackendJobID = r.JobID
		}
		if r.BackendJobID == "" {
			r.BackendJobID = j.BackendJobID
		}
		r.JobID, r.Path = j.ID, host
		r.Seconds = graph.Round1(time.Since(t0).Seconds())
		if !j.StartedAt.IsZero() {
			s := graph.Round1(j.StartedAt.Sub(j.CreatedAt).Seconds())
			r.QueueSeconds = &s
		}
		return &r, nil
	case jobdata.Expired:
		return nil, herr.New("GENERATION_TIMEOUT", fmt.Sprintf("the job waited in the horus queue longer than %s s and never started", wholeSeconds(q.QueueTimeout)),
			map[string]any{"phase": "queued", "job_id": j.ID, "queue_timeout_seconds": json.Number(wholeSeconds(q.QueueTimeout))})
	case jobdata.Failed:
		return nil, jobFailure(j)
	case jobdata.Cancelled:
		return nil, herr.New("CANCELLED", "the job was cancelled in the horus queue", map[string]any{"job_id": j.ID})
	case jobdata.Abandoned:
		return nil, herr.New("GENERATION_FAILED", "the job never started: the queue saw no waiter for it for 60 s", map[string]any{"job_id": j.ID})
	}
	return nil, herr.New("INTERNAL", "the job ended in the state "+string(j.State), map[string]any{"job_id": j.ID})
}

// jobFailure is the error of a failed job: the code, the message and the
// details of its row. details.job_id is the horus job id, and
// details.backend_job_id the id of the job in ComfyUI. GENERATION_TIMEOUT
// gets details.phase running: the run took longer than
// HORUS_GENERATE_TIMEOUT.
func jobFailure(j jobdata.Job) *herr.Error {
	if j.Error == nil {
		return herr.New("INTERNAL", "the job failed without an error", map[string]any{"job_id": j.ID})
	}
	details := map[string]any{}
	for k, v := range j.Error.Details {
		details[k] = v
	}
	if id, ok := details["job_id"]; ok {
		details["backend_job_id"] = id // the run named the id of the job in ComfyUI
	}
	details["job_id"] = j.ID
	code := j.Error.Code
	if !herr.Has(code) {
		details["code"] = code
		code = "INTERNAL"
	}
	if code == "GENERATION_TIMEOUT" {
		if _, ok := details["phase"]; !ok {
			details["phase"] = "running"
		}
	}
	return herr.New(code, j.Error.Message, details)
}

// wholeSeconds is a duration as whole seconds when it has no fraction.
func wholeSeconds(d time.Duration) string {
	if d%time.Second == 0 {
		return fmt.Sprint(int64(d / time.Second))
	}
	return fmt.Sprint(d.Seconds())
}
