package jobs

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
)

var t0 = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

// TestCanMove checks the whole table of moves: 7 x 7 states.
func TestCanMove(t *testing.T) {
	allowed := map[[2]jobdata.State]bool{
		{jobdata.Queued, jobdata.Running}:    true,
		{jobdata.Queued, jobdata.Failed}:     true,
		{jobdata.Queued, jobdata.Cancelled}:  true,
		{jobdata.Queued, jobdata.Expired}:    true,
		{jobdata.Queued, jobdata.Abandoned}:  true,
		{jobdata.Running, jobdata.Running}:   true,
		{jobdata.Running, jobdata.Succeeded}: true,
		{jobdata.Running, jobdata.Failed}:    true,
		{jobdata.Running, jobdata.Cancelled}: true,
	}
	for _, from := range jobdata.States {
		for _, to := range jobdata.States {
			if got, want := CanMove(from, to), allowed[[2]jobdata.State{from, to}]; got != want {
				t.Errorf("CanMove(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestWorkQueue(t *testing.T) {
	if q := WorkQueue("comfyui"); q != "jobs.comfyui" {
		t.Errorf("work queue %q", q)
	}
}

func TestNewID(t *testing.T) {
	id, err := NewID(t0, bytes.NewReader(bytes.Repeat([]byte{0xff}, 10)))
	if err != nil {
		t.Fatal(err)
	}
	// 2026-11-20T10:00:00Z is 1795168800000 ms = 0x01a1f8660d00 (date -u -d ... +%s).
	if id != "01a1f866-0d00-7fff-bfff-ffffffffffff" {
		t.Errorf("id %s", id)
	}
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	a, _ := NewID(t0, bytes.NewReader(make([]byte, 10)))
	b, _ := NewID(t0.Add(time.Millisecond), bytes.NewReader(make([]byte, 10)))
	if !re.MatchString(a) || !re.MatchString(b) || !(a < b) {
		t.Errorf("ids %s and %s: not UUIDv7 in time order", a, b)
	}
	if _, err := NewID(t0, bytes.NewReader(nil)); err == nil {
		t.Error("no random bytes: no error")
	}
}

func TestEstimateStart(t *testing.T) {
	run := 4 * time.Minute
	for _, c := range []struct {
		name    string
		pos     int
		slots   int
		running []time.Time
		want    time.Duration // after t0
	}{
		{"a free slot", 1, 1, nil, 0},
		{"one slot, a job that ran 1 min", 1, 1, []time.Time{t0.Add(-time.Minute)}, 3 * time.Minute},
		{"one slot, position 3", 3, 1, []time.Time{t0.Add(-time.Minute)}, 3*time.Minute + 2*run},
		{"a job past the mean run time ends now", 1, 1, []time.Time{t0.Add(-time.Hour)}, 0},
		{"two slots, one free", 1, 2, []time.Time{t0.Add(-time.Minute)}, 0},
		{"two slots, position 2", 2, 2, []time.Time{t0.Add(-time.Minute)}, 3 * time.Minute},
		{"two slots, position 3", 3, 2, []time.Time{t0.Add(-time.Minute)}, run},
		{"more running jobs than slots", 1, 1, []time.Time{t0.Add(-time.Minute), t0.Add(-2 * time.Minute)}, 2 * time.Minute},
	} {
		got := EstimateStart(t0, c.pos, c.slots, c.running, run)
		if got.Sub(t0) != c.want {
			t.Errorf("%s: start after %s, want %s", c.name, got.Sub(t0), c.want)
		}
	}
}

func TestCheckQueue(t *testing.T) {
	run := 4 * time.Minute
	deadline := t0.Add(2 * time.Hour)
	for _, c := range []struct {
		name   string
		queued int
		start  time.Time
		slots  int
		want   QueueCheck
	}{
		{"room and in time", 19, t0.Add(time.Hour), 1, QueueCheck{}},
		{"the start equals the deadline", 0, deadline, 1, QueueCheck{}},
		{"20 queued of 20", 20, t0.Add(time.Hour), 1, QueueCheck{Full: true, Reason: FullCount, RetryAfter: run}},
		{"20 queued, 2 slots", 20, t0.Add(time.Hour), 2, QueueCheck{Full: true, Reason: FullCount, RetryAfter: 2 * time.Minute}},
		{"late by 90.2 s", 3, deadline.Add(90*time.Second + 200*time.Millisecond), 1,
			QueueCheck{Full: true, Reason: FullDeadline, RetryAfter: 91 * time.Second}},
		{"late by 1 ns", 3, deadline.Add(1), 1, QueueCheck{Full: true, Reason: FullDeadline, RetryAfter: time.Second}},
	} {
		if got := CheckQueue(c.queued, 20, c.start, deadline, run, c.slots); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestQueueText(t *testing.T) {
	for _, c := range []struct {
		pos, total int
		in         time.Duration
		want       string
	}{
		{2, 3, 4 * time.Minute, "waiting in the horus queue, position 2 of 3, start in about 4 min"},
		{1, 1, 20 * time.Second, "waiting in the horus queue, position 1 of 1, start in less than 1 min"},
		{1, 1, 90 * time.Second, "waiting in the horus queue, position 1 of 1, start in about 2 min"},
	} {
		if got := QueueText(c.pos, c.total, c.in); got != c.want {
			t.Errorf("QueueText(%d, %d, %s) = %q, want %q", c.pos, c.total, c.in, got, c.want)
		}
	}
}
