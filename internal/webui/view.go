package webui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// View is what the Horus UI shows: the page renders it, and GET /api/summary
// gives it as JSON. Build makes it from the reads of one request.
type View struct {
	At    time.Time         `json:"at"`
	Ready jobdata.Readiness `json:"ready"`
	// Pills are the readiness strip: the count, then the 4 dependencies.
	Pills []Pill `json:"-"`

	// The counts of the 5 tiles. Waiting, Running, SucceededToday and
	// FailedToday are the counts of the job store.
	Waiting        int `json:"waiting"`
	Running        int `json:"running"`
	Slots          int `json:"slots"`
	SucceededToday int `json:"succeeded_today"`
	FailedToday    int `json:"failed_today"`
	WorkflowCount  int `json:"workflow_count"`
	Installed      int `json:"installed"`
	SameKeyToday   int `json:"same_key_today"`

	RunningJobs []RunningRow  `json:"running_jobs"` // oldest start first
	Queue       []QueueRow    `json:"queue"`        // oldest first
	Recent      []RecentRow   `json:"recent"`       // the last 10 ended jobs, newest first
	Worker      WorkerView    `json:"worker"`
	Workflows   []WorkflowRow `json:"workflows"`

	// Errors are the reads that failed: the page shows them, and the counts
	// of a failed read are 0.
	Errors []string `json:"errors"`

	// The texts of the page.
	SlotText     string `json:"-"`
	WorkflowText string `json:"-"`
}

// Pill is one pill of the readiness strip.
type Pill struct {
	Text, Class, Title string
}

// RunningRow is one running job.
type RunningRow struct {
	ID       string  `json:"id"`
	Workflow string  `json:"workflow"`
	Caller   string  `json:"caller"`
	Subject  string  `json:"subject"`
	Step     float64 `json:"step"`
	Steps    float64 `json:"steps"`
	// RunSeconds is the time since the start, WaitedSeconds the time in the
	// queue before the start.
	RunSeconds    int `json:"run_seconds"`
	WaitedSeconds int `json:"waited_seconds"`

	Meta, Percent, BarTitle, Line, Limit string `json:"-"`
}

// QueueRow is one queued job.
type QueueRow struct {
	Position      int    `json:"position"`
	ID            string `json:"id"`
	Workflow      string `json:"workflow"`
	Caller        string `json:"caller"`
	Subject       string `json:"subject"`
	WaitedSeconds int    `json:"waited_seconds"`
	// StartInSeconds is the estimated start, rounded to 10 s.
	StartInSeconds int `json:"start_in_seconds"`

	Waited, StartIn string `json:"-"`
}

// RecentRow is one ended job.
type RecentRow struct {
	ID           string        `json:"id"`
	State        jobdata.State `json:"state"`
	Workflow     string        `json:"workflow"`
	FinishedAt   time.Time     `json:"finished_at"`
	QueueSeconds int           `json:"queue_seconds"`
	// RunSeconds is -1 for a job that never started.
	RunSeconds int `json:"run_seconds"`

	Ended, Pill, PillClass, Detail, DetailTitle, Queue, Run string `json:"-"`
	DetailMono                                              bool   `json:"-"`
}

// WorkerView is the panel "worker".
type WorkerView struct {
	ID       string `json:"id"`
	Up       bool   `json:"up"`
	Slots    int    `json:"slots"`
	Messages int    `json:"messages"`

	State, Models, Broker, JobStore, ComfyUI, SameKey string `json:"-"`
}

// WorkflowRow is one workflow that list_workflows shows to agents.
type WorkflowRow struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	InstallState string `json:"install_state"`
	// Today is the count of its jobs that ended as succeeded or failed since
	// 00:00.
	Today int `json:"today"`

	Backend, Pill, PillClass string `json:"-"`
}

// Flow is one workflow and its install state, as list_workflows gives them.
type Flow struct {
	Name, Kind, InstallState string
}

// Reads are the results of the reads of one request.
type Reads struct {
	Ready      jobdata.Readiness
	Summary    jobdata.Summary
	SummaryErr error
	Depth      int
	DepthErr   error
	Flows      []Flow
	FlowsErr   error
	// ComfyVersion is the version that ComfyUI reports, or "".
	ComfyVersion string
}

// Settings are the settings that the page shows.
type Settings struct {
	// DatabaseURL is HORUS_DATABASE_URL. The page shows its host and its
	// database, and never its password.
	DatabaseURL string
	// ComfyURL is HORUS_COMFYUI_URL.
	ComfyURL string
	// Slots is HORUS_WORKER_CONCURRENCY: the page uses it before the worker
	// writes its row.
	Slots int
	// RunLimit is HORUS_GENERATE_TIMEOUT.
	RunLimit time.Duration
	// Broker is the name of the broker, for example Postgres.
	Broker string
	// Paths are the shared folders of the stack (HORUS_SHARED_DIRS). The
	// page shows the host path of a PNG whose job row holds a container
	// path. Without a shared folder, the page shows the path of the row.
	Paths jobs.Paths
}

// Backend is the backend of the jobs of generate_image.
const Backend = "comfyui"

// pillOrder is the order of the dependencies in the readiness strip, with
// their names on the page.
var pillOrder = []struct{ dep, name string }{
	{jobdata.DependencyDB, "horus-db"},
	{jobdata.DependencyWorker, "horus-worker"},
	{jobdata.DependencyAPI, "horus-api"},
	{jobdata.DependencyComfyUI, "ComfyUI"},
}

// Build makes the view from the reads at the time now. It is a pure
// function: the page, the JSON and the tests use it.
func Build(r Reads, s Settings, now time.Time) View {
	red := redactor(s.DatabaseURL)
	v := View{At: now, Ready: r.Ready, Errors: []string{},
		RunningJobs: []RunningRow{}, Queue: []QueueRow{}, Recent: []RecentRow{}, Workflows: []WorkflowRow{}}
	v.Ready.Checks = append([]jobdata.CheckResult{}, r.Ready.Checks...)
	for i := range v.Ready.Checks {
		v.Ready.Checks[i].Detail = red(v.Ready.Checks[i].Detail)
	}
	v.Pills = pills(v.Ready)

	sum := r.Summary
	if r.SummaryErr != nil {
		v.Errors = append(v.Errors, red("the job store: "+r.SummaryErr.Error()))
		sum = jobdata.Summary{}
	}
	v.Waiting, v.Running = sum.Queued, sum.Running
	v.SucceededToday, v.FailedToday, v.SameKeyToday = sum.SucceededToday, sum.FailedToday, sum.SameKeyToday
	v.Slots = s.Slots
	if sum.Worker != nil && sum.Worker.Slots > 0 {
		v.Slots = sum.Worker.Slots
	}
	if v.Slots < 1 {
		v.Slots = 1
	}
	v.SlotText = slotText(len(sum.RunningJobs), v.Slots)

	for _, j := range sum.RunningJobs {
		v.RunningJobs = append(v.RunningJobs, runningRow(j, s.RunLimit, now))
	}
	run := time.Duration(sum.MeanRunSeconds * float64(time.Second))
	if run <= 0 {
		run = jobs.DefaultRunTime
	}
	var starts []time.Time
	for _, j := range sum.RunningJobs {
		starts = append(starts, j.StartedAt)
	}
	for i, j := range sum.QueuedJobs {
		start := jobs.EstimateStart(now, i+1, v.Slots, starts, run)
		v.Queue = append(v.Queue, queueRow(i+1, j, start, now))
	}
	for _, j := range sum.Recent {
		v.Recent = append(v.Recent, recentRow(j, now, red))
	}

	v.Worker = worker(r, s, sum, red, now)
	if r.DepthErr != nil {
		v.Errors = append(v.Errors, red("the broker: "+r.DepthErr.Error()))
	}

	if r.FlowsErr != nil {
		v.Errors = append(v.Errors, red("the workflows: "+r.FlowsErr.Error()))
	}
	for _, f := range r.Flows {
		row := WorkflowRow{Name: f.Name, Kind: f.Kind, InstallState: f.InstallState, Today: sum.WorkflowToday[f.Name]}
		row.Backend = "ComfyUI"
		if f.Kind == catalog.KindSpeech {
			row.Backend = "speech worker"
		}
		switch f.InstallState {
		case "installed":
			row.Pill, row.PillClass = "yes", "ok"
			v.Installed++
		case "partially_installed":
			row.Pill, row.PillClass = "partly", "grey"
		default:
			row.Pill, row.PillClass = "no", "grey"
		}
		v.Workflows = append(v.Workflows, row)
	}
	v.WorkflowCount = len(v.Workflows)
	word := "workflows"
	if v.WorkflowCount == 1 {
		word = "workflow"
	}
	v.WorkflowText = fmt.Sprintf("%s, %d installed", word, v.Installed)
	return v
}

func pills(r jobdata.Readiness) []Pill {
	class := "bad"
	if r.Ready {
		class = "ok"
	}
	out := []Pill{{Text: fmt.Sprintf("Ready: %d of %d", r.Up, len(r.Checks)), Class: class,
		Title: "horus serves only when its 4 dependencies are up"}}
	for _, p := range pillOrder {
		c := jobdata.CheckResult{Dependency: p.dep, Detail: "no check result"}
		for _, x := range r.Checks {
			if x.Dependency == p.dep {
				c = x
			}
		}
		pill := Pill{Text: p.name + " down", Class: "bad", Title: c.Detail}
		if c.Up {
			pill.Text, pill.Class = p.name+" up", "ok"
		}
		out = append(out, pill)
	}
	return out
}

// slotText is the note of the panel "running": "slot 1 of 1".
func slotText(running, slots int) string {
	switch running {
	case 0:
		return fmt.Sprintf("0 of %d slots", slots)
	case 1:
		return fmt.Sprintf("slot 1 of %d", slots)
	}
	return fmt.Sprintf("slots 1 to %d of %d", running, slots)
}

// plan reads the payload of a job of generate_image. A payload of another
// tool gives a zero plan.
func plan(j jobdata.Job) generate.Plan {
	var p generate.Plan
	_ = json.Unmarshal(j.Payload, &p)
	return p
}

func runningRow(j jobdata.Job, limit time.Duration, now time.Time) RunningRow {
	r := RunningRow{ID: j.ID, Workflow: j.Workflow, Caller: j.Caller, Subject: plan(j).Subject}
	ran := nonNegative(now.Sub(j.StartedAt))
	waited := nonNegative(j.StartedAt.Sub(j.CreatedAt))
	r.RunSeconds, r.WaitedSeconds = int(ran/time.Second), int(waited/time.Second)
	r.Meta = "job " + shortID(j.ID)
	if j.Caller != "" {
		r.Meta += " · " + j.Caller
	}
	step := "starting"
	r.Percent = "0%"
	if p := j.Progress; p != nil {
		r.Step, r.Steps = p.Value, p.Total
		switch {
		case p.Total > 0:
			step = fmt.Sprintf("step %g of %g", p.Value, p.Total)
			pct := p.Value / p.Total * 100
			pct = max(0, min(100, pct))
			r.Percent = fmt.Sprintf("%.0f%%", pct)
			r.BarTitle = "ComfyUI reports " + step
		case p.Message != "":
			step = p.Message
		}
	}
	if r.BarTitle == "" {
		r.BarTitle = "ComfyUI reports no step yet"
	}
	r.Line = fmt.Sprintf("%s · running %s · waited %s", step, words(ran), words(waited))
	if left := limit - ran; limit > 0 && left > 0 {
		r.Limit = fmt.Sprintf("run limit in %d min", int(left/time.Minute))
	} else if limit > 0 {
		r.Limit = "past the run limit"
	}
	return r
}

func queueRow(pos int, j jobdata.Job, start, now time.Time) QueueRow {
	waited := nonNegative(now.Sub(j.CreatedAt))
	in := roundTo10s(nonNegative(start.Sub(now)))
	return QueueRow{Position: pos, ID: j.ID, Workflow: j.Workflow, Caller: j.Caller, Subject: plan(j).Subject,
		WaitedSeconds: int(waited / time.Second), StartInSeconds: int(in / time.Second),
		Waited: clock(waited), StartIn: clock(in)}
}

func recentRow(j jobdata.Job, now time.Time, red func(string) string) RecentRow {
	r := RecentRow{ID: j.ID, State: j.State, Workflow: j.Workflow, FinishedAt: j.FinishedAt, RunSeconds: -1, Run: "–"}
	r.Ended = when(j.FinishedAt, now)
	queued := j.FinishedAt.Sub(j.CreatedAt)
	if !j.StartedAt.IsZero() {
		queued = j.StartedAt.Sub(j.CreatedAt)
		ran := nonNegative(j.FinishedAt.Sub(j.StartedAt))
		r.RunSeconds, r.Run = int(ran/time.Second), clock(ran)
	}
	queued = nonNegative(queued)
	r.QueueSeconds, r.Queue = int(queued/time.Second), clock(queued)
	switch j.State {
	case jobdata.Succeeded:
		r.Pill, r.PillClass = "PNG", "ok"
		var res struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(j.Result, &res)
		r.Detail, r.DetailTitle, r.DetailMono = shortPath(res.Path), res.Path, true
	case jobdata.Failed:
		r.Pill, r.PillClass = "failed", "bad"
		if e := j.Error; e != nil {
			r.Pill = e.Code
			r.Detail = red(e.Message)
			if p := plan(j); e.Code == "OUT_OF_MEMORY" && p.Width > 0 && p.Height > 0 {
				r.Detail = fmt.Sprintf("%d × %d", p.Width, p.Height)
			}
			r.DetailTitle = red(e.Message)
		}
	case jobdata.Cancelled:
		r.Pill, r.PillClass, r.Detail = "cancelled", "grey", "the agent cancelled the call"
	case jobdata.Expired:
		r.Pill, r.PillClass, r.Detail = "expired", "grey", "waited past its deadline"
	case jobdata.Abandoned:
		r.Pill, r.PillClass = "abandoned", "grey"
		r.Detail = fmt.Sprintf("no waiter for %d s", int(jobdata.AbandonAfter/time.Second))
	default:
		r.Pill, r.PillClass = string(j.State), "grey"
	}
	return r
}

func worker(r Reads, s Settings, sum jobdata.Summary, red func(string) string, now time.Time) WorkerView {
	w := WorkerView{Slots: max(s.Slots, 1), Messages: r.Depth}
	check := jobdata.CheckResult{Dependency: jobdata.DependencyWorker, Detail: "no check result"}
	for _, c := range r.Ready.Checks {
		if c.Dependency == jobdata.DependencyWorker {
			check = c
		}
	}
	w.Up = check.Up
	w.Models = "none"
	switch row := sum.Worker; {
	case row == nil:
		w.State = "no worker row: horus worker did not start"
	default:
		w.ID = row.ID
		if row.Slots > 0 {
			w.Slots = row.Slots
		}
		if check.Up {
			w.State = "running since " + when(row.StartedAt, now)
		} else {
			w.State = "down: " + red(check.Detail)
		}
		if k := row.LoadedResourceKey; k != "" {
			w.Models = "resource key " + shortKey(k)
			if name := workflowOfKey(sum, k); name != "" {
				w.Models = name + " (resource key " + shortKey(k) + ")"
			}
		}
	}
	if sum.Worker == nil && !check.Up {
		w.State = "down: " + red(check.Detail)
	}
	word := "messages"
	if r.Depth == 1 {
		word = "message"
	}
	broker := s.Broker
	if broker == "" {
		broker = "Postgres"
	}
	w.Broker = fmt.Sprintf("%s · work queue %s · %d %s", broker, jobs.WorkQueue(Backend), r.Depth, word)
	if r.DepthErr != nil {
		w.Broker = fmt.Sprintf("%s · work queue %s · no answer", broker, jobs.WorkQueue(Backend))
	}
	w.JobStore = storeName(s.DatabaseURL)
	w.ComfyUI = hostOf(s.ComfyURL)
	switch {
	case r.ComfyVersion != "":
		v := r.ComfyVersion
		if v[0] >= '0' && v[0] <= '9' {
			v = "v" + v
		}
		w.ComfyUI += " · " + v
	case !comfyUp(r.Ready):
		w.ComfyUI += " · no answer"
	}
	keyWord := "jobs"
	if sum.SameKeyToday == 1 {
		keyWord = "job"
	}
	w.SameKey = fmt.Sprintf("%d %s (dedupe is off)", sum.SameKeyToday, keyWord)
	return w
}

func comfyUp(r jobdata.Readiness) bool {
	for _, c := range r.Checks {
		if c.Dependency == jobdata.DependencyComfyUI {
			return c.Up
		}
	}
	return false
}

// workflowOfKey gives the workflow of a job of the summary with the resource
// key k, or "".
func workflowOfKey(sum jobdata.Summary, k string) string {
	for _, list := range [][]jobdata.Job{sum.RunningJobs, sum.Recent, sum.QueuedJobs} {
		for _, j := range list {
			if j.ResourceKey == k && j.Workflow != "" {
				return j.Workflow
			}
		}
	}
	return ""
}

// storeName is the host and the database of a database URL: "horus-db /
// horus". It never holds the user or the password. The port shows only when
// it is not 5432.
func storeName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "not set"
	}
	host := u.Hostname()
	if p := u.Port(); p != "" && p != "5432" {
		host += ":" + p
	}
	return host + " / " + strings.TrimPrefix(u.Path, "/")
}

// hostOf is the host and port of a URL: "host.docker.internal:8188".
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}

// redactor gives a function that replaces the password of the database URL
// in a text. A URL without a password gives a function that changes nothing.
func redactor(databaseURL string) func(string) string {
	u, err := url.Parse(databaseURL)
	if err != nil || u.User == nil {
		return func(s string) string { return s }
	}
	pw, ok := u.User.Password()
	if !ok || pw == "" {
		return func(s string) string { return s }
	}
	return func(s string) string {
		s = strings.ReplaceAll(s, pw, "***")
		if esc := url.QueryEscape(pw); esc != pw {
			s = strings.ReplaceAll(s, esc, "***")
		}
		return s
	}
}

// shortID is the first 8 characters of a job id.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// shortKey is the first 4 characters of a resource key and "…".
func shortKey(k string) string {
	if len(k) > 4 {
		return k[:4] + "…"
	}
	return k
}

// shortPath is the last 3 parts of a path, after "…": "…\outputs\it-illustration\x.png".
func shortPath(p string) string {
	sep := "/"
	if strings.Contains(p, `\`) {
		sep = `\`
	}
	parts := strings.Split(p, sep)
	if len(parts) <= 4 {
		return p
	}
	return "…" + sep + strings.Join(parts[len(parts)-3:], sep)
}

func nonNegative(d time.Duration) time.Duration { return max(d, 0) }

// roundTo10s rounds d to the nearest 10 s: the estimated start is an estimate.
func roundTo10s(d time.Duration) time.Duration {
	return (d + 5*time.Second) / (10 * time.Second) * (10 * time.Second)
}

// clock is a duration as m:ss, or h:mm:ss from 1 hour.
func clock(d time.Duration) string {
	s := int(d / time.Second)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// words is a duration as "31 s", "2 min 31 s" or "1 h 5 min".
func words(d time.Duration) string {
	s := int(d / time.Second)
	switch {
	case s < 60:
		return fmt.Sprintf("%d s", s)
	case s < 3600:
		return fmt.Sprintf("%d min %d s", s/60, s%60)
	}
	return fmt.Sprintf("%d h %d min", s/3600, s%3600/60)
}

// when is a time as 15:04 on the day of now, else as Jan 2 15:04.
func when(t, now time.Time) string {
	if t.IsZero() {
		return "–"
	}
	t = t.In(now.Location())
	if y, m, d := t.Date(); y == now.Year() && m == now.Month() && d == now.Day() {
		return t.Format("15:04")
	}
	return t.Format("Jan 2 15:04")
}
