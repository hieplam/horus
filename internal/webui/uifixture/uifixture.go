// Package uifixture holds the two fixtures of the Horus UI (goal row
// ui-matches-mockup of docs/specs/2026-11-19-job-queue.md):
//
//   - Reference: the example data of the reference
//     docs/specs/2026-11-19-job-queue-ui.html. 3 jobs wait, 1 of 1 slot runs,
//     14 succeeded and 1 failed today, 1 was abandoned, 2 jobs of today have
//     the same key, and the worker is up.
//   - Other: other numbers in each part. 5 jobs wait, 2 of 2 slots run, 3
//     succeeded and 2 failed today (1 of them of the workflow vi-speech), 1
//     was cancelled and 1 expired, 3 jobs of today have the same key, and the
//     heartbeat of the worker is 60 s old, so horus-worker is down.
//
// The times of a fixture are relative to the time now that it gets, and the
// ended jobs of "today" stay after 00:00 of that day. Load writes a fixture
// into a job store and a broker. The test keeps the worker up.
//
// Only a test file imports this package.
package uifixture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// Fixture is the data of one fixture.
type Fixture struct {
	Name string
	Now  time.Time
	// Jobs are the rows of horus.jobs.
	Jobs []jobdata.Job
	// Worker is the row of horus.workers.
	Worker jobdata.Worker
	// WorkerUp: the test holds the worker lock and writes the heartbeat
	// every few seconds. Else the heartbeat is StaleBeat old.
	WorkerUp bool
	// Messages are the messages of the work queue jobs.comfyui: one for each
	// queued job, oldest first.
	Messages []jobdata.Message
}

// StaleBeat is the age of the heartbeat of a worker that is down.
const StaleBeat = time.Minute

// The values of the reference that a test reads. A job row of the stack
// holds the container path of its PNG: SharedDir is the container folder of
// the shared folder of the fixtures.
const (
	SharedDir   = "/shared/outputs"
	ResourceKey = "9f2c41d8e7b3a05c6f1e2d9a8b7c4e3f0a1b2c3d4e5f60718293a4b5c6d7e8f9"
	Workflow    = "it-illustration"
)

// Load inserts the jobs and publishes the messages. The worker row is the
// job of the test (Heartbeat), because its time is the clock of the store.
func (f Fixture) Load(ctx context.Context, s jobs.Store, b jobs.Broker) error {
	for _, j := range f.Jobs {
		if err := s.Insert(ctx, j); err != nil {
			return fmt.Errorf("fixture %s: %w", f.Name, err)
		}
	}
	for _, m := range f.Messages {
		if err := b.Publish(ctx, jobs.WorkQueue(m.Backend), m); err != nil {
			return fmt.Errorf("fixture %s: %w", f.Name, err)
		}
	}
	return nil
}

// builder makes the jobs of a fixture.
type builder struct {
	now      time.Time
	midnight time.Time
	n        int
	jobs     []jobdata.Job
	worker   string
}

func newBuilder(now time.Time) *builder {
	return &builder{now: now, midnight: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())}
}

// id gives a new UUIDv7 with the time t.
func (b *builder) id(t time.Time) string {
	b.n++
	seed := bytes.Repeat([]byte{byte(b.n)}, 10)
	id, err := jobs.NewID(t, bytes.NewReader(seed))
	if err != nil {
		panic(err)
	}
	return id
}

// ago gives now - d, but never before 00:00 + 1 s of the day of now, so
// that an ended job of a fixture is a job of today.
func (b *builder) ago(d time.Duration) time.Time {
	t := b.now.Add(-d)
	if floor := b.midnight.Add(time.Second); t.Before(floor) {
		return floor
	}
	return t
}

// job makes a job of the workflow with the subject, made at created.
func (b *builder) job(workflow, subject, caller string, created time.Time, w, h int64) jobdata.Job {
	id := b.id(created)
	payload, _ := json.Marshal(generate.Plan{Workflow: workflow, Subject: subject, Seed: uint64(b.n), Width: w, Height: h,
		Path: fmt.Sprintf(SharedDir+"/%s/%s-%d.png", workflow, created.UTC().Format("20060102T150405Z"), b.n)})
	tool := "generate_image"
	backend := "comfyui"
	if workflow == "vi-speech" {
		tool, backend = "generate_speech", "speech"
	}
	return jobdata.Job{ID: id, Tool: tool, Workflow: workflow, Backend: backend, WorkQueue: jobs.WorkQueue(backend),
		ResourceKey: ResourceKey, Payload: payload, State: jobdata.Queued, CreatedAt: created,
		Deadline: created.Add(jobs.DefaultQueueTimeout), WaiterSeenAt: b.now.Add(-5 * time.Second), Caller: caller}
}

func (b *builder) add(j jobdata.Job) jobdata.Job {
	b.jobs = append(b.jobs, j)
	return j
}

// queued adds a queued job made d ago.
func (b *builder) queued(subject, caller string, d time.Duration) {
	b.add(b.job(Workflow, subject, caller, b.now.Add(-d), 1024, 1024))
}

// running adds a running job that started ran ago, after it waited.
func (b *builder) running(subject, caller string, ran, waited time.Duration, p *jobdata.Progress) {
	started := b.now.Add(-ran)
	j := b.job(Workflow, subject, caller, started.Add(-waited), 1024, 1024)
	j.State, j.StartedAt, j.Attempt, j.WorkerID, j.BackendJobID, j.Progress = jobdata.Running, started, 1, b.worker, "prompt-"+j.ID[:8], p
	b.add(j)
}

// ended adds a job that ended in the state s at finished, after it waited
// and ran. ran 0: the job never started.
func (b *builder) ended(workflow string, s jobdata.State, finished time.Time, waited, ran time.Duration, w, h int64) *jobdata.Job {
	created := finished.Add(-ran - waited)
	j := b.job(workflow, "an ended job of the fixture", "Claude Code · folder horus", created, w, h)
	j.State, j.FinishedAt = s, finished
	j.WaiterSeenAt = finished
	if ran > 0 {
		j.StartedAt, j.Attempt, j.WorkerID = created.Add(waited), 1, b.worker
	}
	switch s {
	case jobdata.Succeeded:
		var p generate.Plan
		_ = json.Unmarshal(j.Payload, &p)
		p.Path = fmt.Sprintf(SharedDir+"/%s/%s-%d.png", workflow, finished.UTC().Format("20060102T150405Z"), b.n)
		j.Result, _ = json.Marshal(map[string]any{"workflow": workflow, "path": p.Path, "width": w, "height": h, "seed": p.Seed})
	case jobdata.Abandoned:
		j.WaiterSeenAt = finished.Add(-jobdata.AbandonAfter - time.Second)
	case jobdata.Cancelled:
		j.CancelRequested = finished
	}
	b.jobs = append(b.jobs, j)
	return &b.jobs[len(b.jobs)-1]
}

// messages gives one message for each queued job, oldest first.
func (b *builder) messages() []jobdata.Message {
	var out []jobdata.Message
	for _, j := range b.jobs {
		if j.State == jobdata.Queued {
			out = append(out, jobdata.Message{V: jobdata.MessageVersion, JobID: j.ID, Tool: j.Tool, Backend: j.Backend})
		}
	}
	return out
}

// Reference is the example data of the reference page at the time now.
func Reference(now time.Time) Fixture {
	b := newBuilder(now)
	wstart := now.Add(-2*time.Hour - 40*time.Minute)
	b.worker = b.id(wstart)
	worker := jobdata.Worker{ID: b.worker, Host: "horus-worker", PID: 1, Version: "0.0.0-fixture", Slots: 1,
		StartedAt: wstart, LoadedResourceKey: ResourceKey}

	b.running("A flat vector illustration of a message queue between three laptops and one server, blue and orange palette",
		"Claude Code · folder horus", 151*time.Second, 0, &jobdata.Progress{Value: 9, Total: 15, Message: "step 9 of 15"})
	b.queued("An isometric diagram of a Kubernetes cluster with three nodes", "Codex · folder blog", 130*time.Second)
	b.queued("A line drawing of a database with a clock, muted colours", "Claude Code · folder docs", 65*time.Second)
	b.queued("A poster of a falcon eye made of circuit lines", "Claude Code · folder horus", 12*time.Second)

	// The 3 rows of the reference: a PNG, an OUT_OF_MEMORY and an abandoned job.
	b.ended(Workflow, jobdata.Succeeded, b.ago(2*time.Minute), 192*time.Second, 241*time.Second, 1024, 1024)
	oom := b.ended(Workflow, jobdata.Failed, b.ago(13*time.Minute), 40*time.Second, 115*time.Second, 1536, 1536)
	oom.Error = &jobdata.JobError{Code: "OUT_OF_MEMORY", Message: "ComfyUI ran out of GPU memory at 1536 x 1536"}
	b.ended(Workflow, jobdata.Abandoned, b.ago(24*time.Minute), 390*time.Second, 0, 1024, 1024)

	// 13 more PNG of today, newest first. The last 10 succeeded jobs (the PNG
	// above and the first 9 here) run 240 s on average, so the estimated
	// starts of the queue are 1:30, 5:30 and 9:30, as in the reference.
	runs := []time.Duration{240, 240, 240, 240, 240, 240, 240, 240, 239, 250, 230, 260, 220}
	for i, r := range runs {
		j := b.ended(Workflow, jobdata.Succeeded, b.ago(30*time.Minute+time.Duration(i)*8*time.Minute), 30*time.Second, r*time.Second, 1024, 1024)
		if i == 2 || i == 5 { // 2 jobs of today with the same key
			j.IdempotencyKey = "f1:0000000000000000000000000000000000000000000000000000000000000001"
		}
	}
	return Fixture{Name: "reference", Now: now, Jobs: b.jobs, Worker: worker, WorkerUp: true, Messages: b.messages()}
}

// Other is a fixture with other numbers in each part, at the time now.
func Other(now time.Time) Fixture {
	b := newBuilder(now)
	wstart := now.Add(-50 * time.Minute)
	b.worker = b.id(wstart)
	worker := jobdata.Worker{ID: b.worker, Host: "horus-worker", PID: 7, Version: "0.0.0-fixture", Slots: 2, StartedAt: wstart}

	b.running("A watercolour map of a data centre with two rows of racks", "Codex · folder infra", 40*time.Second, 75*time.Second,
		&jobdata.Progress{Value: 3, Total: 20, Message: "step 3 of 20"})
	b.running("A blueprint of a load balancer in front of four servers", "Claude Code · folder blog", 5*time.Second, 200*time.Second, nil)
	for i, s := range []string{"A pixel art router with blinking lights", "A chalk drawing of a hash table",
		"A paper cut of a cloud with three buckets", "A neon sign of a terminal prompt", "A photo-style render of a fibre cable"} {
		b.queued(s, "Claude Code · folder docs", time.Duration(300-50*i)*time.Second)
	}

	same := "k1:0000000000000000000000000000000000000000000000000000000000000002"
	for i := 0; i < 3; i++ {
		j := b.ended(Workflow, jobdata.Succeeded, b.ago(time.Duration(5+10*i)*time.Minute), time.Duration(20+i)*time.Second, time.Duration(180+i*7)*time.Second, 1024, 1024)
		j.IdempotencyKey = same
	}
	f1 := b.ended(Workflow, jobdata.Failed, b.ago(8*time.Minute), 10*time.Second, 30*time.Second, 1024, 1024)
	f1.Error = &jobdata.JobError{Code: "BACKEND_UNAVAILABLE", Message: "ComfyUI does not answer"}
	f2 := b.ended("vi-speech", jobdata.Failed, b.ago(18*time.Minute), 5*time.Second, 12*time.Second, 0, 0)
	f2.Error = &jobdata.JobError{Code: "INTERNAL", Message: "the speech worker stopped"}
	b.ended(Workflow, jobdata.Cancelled, b.ago(28*time.Minute), 45*time.Second, 0, 1024, 1024)
	b.ended(Workflow, jobdata.Expired, b.ago(38*time.Minute), 2*time.Hour, 0, 1024, 1024)
	// 2 PNG of yesterday: in "recent", but not in the counts of today. The
	// first one has the same key: a job of yesterday does not count.
	y := b.midnight.Add(-2 * time.Hour)
	b.ended(Workflow, jobdata.Succeeded, y, time.Minute, 4*time.Minute, 1024, 1024).IdempotencyKey = same
	b.ended(Workflow, jobdata.Succeeded, y.Add(-time.Hour), time.Minute, 5*time.Minute, 1024, 1024)
	return Fixture{Name: "other", Now: now, Jobs: b.jobs, Worker: worker, WorkerUp: false, Messages: b.messages()}
}
