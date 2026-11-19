package jobs

import (
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"sort"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
)

// moves are the allowed moves of each state. A move from Running to Running
// writes a change (progress, backend_job_id, a cancel request) in the state.
// An end state has no move.
var moves = map[jobdata.State][]jobdata.State{
	// The worker starts the job, the API process cancels it, the sweep
	// expires or abandons it. The API process fails it when the broker
	// refuses its message: no worker can run a job without a message.
	jobdata.Queued: {jobdata.Running, jobdata.Failed, jobdata.Cancelled, jobdata.Expired, jobdata.Abandoned},
	// The worker updates the job, or ends it.
	jobdata.Running: {jobdata.Running, jobdata.Succeeded, jobdata.Failed, jobdata.Cancelled},
}

// CanMove tells if a job can move from the state from to the state to.
func CanMove(from, to jobdata.State) bool { return slices.Contains(moves[from], to) }

// WorkQueue is the name of the work queue of a backend, for example
// jobs.comfyui.
func WorkQueue(backend string) string { return "jobs." + backend }

// NewID makes a UUIDv7 (RFC 9562): 48 bits of the Unix time in milliseconds,
// then random bits from r.
func NewID(now time.Time, r io.Reader) (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(r, b[6:]); err != nil {
		return "", err
	}
	ms := uint64(now.UnixMilli())
	for i := 0; i < 6; i++ {
		b[i] = byte(ms >> (8 * (5 - i)))
	}
	b[6] = 0x70 | b[6]&0x0f // version 7
	b[8] = 0x80 | b[8]&0x3f // variant 10
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}

// EstimateStart gives the estimated start of the queued job at the place pos
// (1 is the next to start). running holds the start times of the running
// jobs. Each job runs for the time run. A slot that is free now takes the next
// job now. Each job before pos takes the slot that is free first.
func EstimateStart(now time.Time, pos, slots int, running []time.Time, run time.Duration) time.Time {
	if slots < len(running) {
		slots = len(running)
	}
	if slots < 1 {
		slots = 1
	}
	free := make([]time.Time, slots)
	for i := range free {
		free[i] = now
		if i < len(running) {
			if end := running[i].Add(run); end.After(now) {
				free[i] = end
			}
		}
	}
	first := func() int {
		k := 0
		for i := range free {
			if free[i].Before(free[k]) {
				k = i
			}
		}
		return k
	}
	for n := 1; n < pos; n++ {
		k := first()
		free[k] = free[k].Add(run)
	}
	return free[first()]
}

// The reasons of QUEUE_FULL.
const (
	FullCount    = "count"    // HORUS_QUEUE_MAX jobs wait
	FullDeadline = "deadline" // the estimated start is after the deadline
)

// QueueCheck is the result of the rule of QUEUE_FULL for a new call.
type QueueCheck struct {
	Full bool
	// Reason is FullCount or FullDeadline when Full.
	Reason string
	// RetryAfter is the time after which a call again can pass, in whole
	// seconds and at least 1 s, when Full.
	RetryAfter time.Duration
}

// CheckQueue applies the rule of QUEUE_FULL to a new call. queued is the
// count of queued jobs before the call, max is HORUS_QUEUE_MAX, start is the
// estimated start of the new job and deadline is its deadline. run and slots
// give the time until one job leaves the queue: run / slots.
//
//   - queued >= max: Full by FullCount. Retry after run / slots.
//   - start after deadline: Full by FullDeadline. Retry after the time by
//     which the start is late.
func CheckQueue(queued, max int, start, deadline time.Time, run time.Duration, slots int) QueueCheck {
	if slots < 1 {
		slots = 1
	}
	switch {
	case queued >= max:
		return QueueCheck{Full: true, Reason: FullCount, RetryAfter: wholeSeconds(run / time.Duration(slots))}
	case start.After(deadline):
		return QueueCheck{Full: true, Reason: FullDeadline, RetryAfter: wholeSeconds(start.Sub(deadline))}
	}
	return QueueCheck{}
}

// wholeSeconds rounds d up to whole seconds, at least 1 s.
func wholeSeconds(d time.Duration) time.Duration {
	s := (d + time.Second - 1) / time.Second
	if s < 1 {
		s = 1
	}
	return s * time.Second
}

// QueueText is the progress text of a queued job: "waiting in the horus
// queue, position 2 of 3, start in about 4 min".
func QueueText(pos, total int, startIn time.Duration) string {
	when := "start in less than 1 min"
	if m := int((startIn + 30*time.Second) / time.Minute); m >= 1 {
		when = fmt.Sprintf("start in about %d min", m)
	}
	return fmt.Sprintf("waiting in the horus queue, position %d of %d, %s", pos, total, when)
}

// runningStarts gives the start times of the running jobs of a summary,
// oldest first.
func runningStarts(s jobdata.Summary) []time.Time {
	var out []time.Time
	for _, j := range s.RunningJobs {
		out = append(out, j.StartedAt)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Before(out[b]) })
	return out
}

// meanRun gives the mean run time of a summary, or def when no job succeeded.
func meanRun(s jobdata.Summary, def time.Duration) time.Duration {
	if s.MeanRunSeconds <= 0 {
		return def
	}
	return time.Duration(s.MeanRunSeconds * float64(time.Second))
}
