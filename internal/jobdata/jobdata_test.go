package jobdata

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

func TestEnded(t *testing.T) {
	var ended []State
	for _, s := range States {
		if s.Ended() {
			ended = append(ended, s)
		}
	}
	want := []State{Succeeded, Failed, Cancelled, Expired, Abandoned}
	if !slices.Equal(ended, want) {
		t.Errorf("end states %v, want %v", ended, want)
	}
}

func TestSweepState(t *testing.T) {
	queued := Job{State: Queued, Deadline: t0.Add(time.Hour), WaiterSeenAt: t0}
	for _, c := range []struct {
		name string
		job  Job
		now  time.Time
		want State
		ok   bool
	}{
		{"a waiter and no deadline yet", queued, t0.Add(AbandonAfter), "", false},
		{"no waiter for more than 60 s", queued, t0.Add(AbandonAfter + time.Second), Abandoned, true},
		{"after the deadline", Job{State: Queued, Deadline: t0, WaiterSeenAt: t0}, t0.Add(time.Second), Expired, true},
		{"the deadline wins over no waiter", Job{State: Queued, Deadline: t0, WaiterSeenAt: t0.Add(-time.Hour)}, t0.Add(time.Second), Expired, true},
		{"a running job is not swept", Job{State: Running, Deadline: t0, WaiterSeenAt: t0}, t0.Add(time.Hour), "", false},
	} {
		got, ok := c.job.SweepState(c.now)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: %q %v, want %q %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestApply(t *testing.T) {
	j := Job{ID: "a", State: Queued}
	j = Apply(j, Running, Change{At: t0, WorkerID: "w1"})
	if j.State != Running || !j.StartedAt.Equal(t0) || j.Attempt != 1 || j.WorkerID != "w1" || !j.FinishedAt.IsZero() {
		t.Fatalf("queued to running: %+v", j)
	}
	j = Apply(j, Running, Change{At: t0.Add(time.Minute), BackendJobID: "p1", Progress: &Progress{Value: 3, Total: 9, Message: "step 3 of 9"}})
	if !j.StartedAt.Equal(t0) || j.Attempt != 1 || j.BackendJobID != "p1" || j.Progress.Message != "step 3 of 9" || j.WorkerID != "w1" {
		t.Fatalf("an update in running changes started_at or attempt: %+v", j)
	}
	j = Apply(j, Running, Change{At: t0.Add(2 * time.Minute), CancelRequested: true})
	if !j.CancelRequested.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("cancel_requested_at: %+v", j)
	}
	j = Apply(j, Succeeded, Change{At: t0.Add(3 * time.Minute), Result: json.RawMessage(`{"path":"x.png"}`)})
	if j.State != Succeeded || !j.FinishedAt.Equal(t0.Add(3*time.Minute)) || string(j.Result) != `{"path":"x.png"}` {
		t.Fatalf("running to succeeded: %+v", j)
	}
	f := Apply(Job{State: Running}, Failed, Change{At: t0, Error: &JobError{Code: "WORKER_LOST", Message: "m"}})
	if f.Error == nil || f.Error.Code != "WORKER_LOST" || !f.FinishedAt.Equal(t0) {
		t.Fatalf("running to failed: %+v", f)
	}
}

func TestEventData(t *testing.T) {
	if d := (Change{At: t0}).EventData(); d != nil {
		t.Errorf("a change with no data: %s", d)
	}
	d := Change{Progress: &Progress{Value: 1, Total: 2, Message: "m"}}.EventData()
	if string(d) != `{"progress":{"value":1,"total":2,"message":"m"}}` {
		t.Errorf("progress: %s", d)
	}
	d = Change{Error: &JobError{Code: "WORKER_LOST", Message: "m"}}.EventData()
	if string(d) != `{"error":{"code":"WORKER_LOST","message":"m"}}` {
		t.Errorf("error: %s", d)
	}
}

func TestMessageJSON(t *testing.T) {
	b, err := json.Marshal(Message{V: MessageVersion, JobID: "j", Tool: "generate_image", Backend: "comfyui"})
	if err != nil || string(b) != `{"v":1,"job_id":"j","tool":"generate_image","backend":"comfyui"}` {
		t.Errorf("message: %s %v", b, err)
	}
	b, err = json.Marshal(Event{V: MessageVersion, JobID: "j", Type: EventType(Running), At: t0, Seq: 7})
	if err != nil || string(b) != `{"v":1,"job_id":"j","type":"job.running","at":"2026-11-20T10:00:00Z","seq":7}` {
		t.Errorf("event: %s %v", b, err)
	}
}
