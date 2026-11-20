// The test of the step of a running job, through the wiring of cmd/horus:
// horus worker and horus api in this process on a test database, the real
// ComfyUI adapter, and the fake ComfyUI of comfytest. The fake reports a step
// as ComfyUI does: only on the WebSocket of the client id of the job. The goal
// row ui-matches-mockup (goals/2026-11-19-job-queue) runs the test and reads
// its line.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/comfy/comfytest"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
	"github.com/hieplam/horus/internal/postgres"
	"github.com/hieplam/horus/internal/postgres/pgtest"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/webui"
)

// The parts of the panel "Running" that show the step: the hover text and
// the width of the bar, and the line below the bar.
var (
	runningBar  = regexp.MustCompile(`<div class="bar-outer" title="([^"]*)"><div class="bar-inner" style="width:(\d+)%">`)
	runningLine = regexp.MustCompile(`<div class="meta"><span>([^<]*)</span>`)
)

// runningRead is one read of the panel "Running" and of /api/summary.
type runningRead struct {
	step, steps float64 // of /api/summary
	bar         int     // the width of the bar on the page, in percent
	hover, line string  // the hover text of the bar, and the line below it
}

// TestRunningStep: while a job runs, the fake ComfyUI reports step 3 of 15
// and then step 9 of 15 of the sampler node. The worker writes each step to
// the job row. /api/summary gives the step and the total, and the panel
// "Running" of the page shows "step N of 15" and a bar of that part. The
// second read is larger than the first one, so a bar with a constant width
// fails. The progress text of the job names the step too. When the WebSocket
// ends, the page shows no step and an empty bar, and the job runs on. The
// test prints one line:
//
//	ui-running-step: read 1: step 3 of 15, bar 20%; read 2: step 9 of 15, bar 60%; without the WebSocket: bar 0%
func TestRunningStep(t *testing.T) {
	pgtest.Need(t)
	ctx := context.Background()
	dbURL := pgtest.URL(t)
	f := imageEnv(t)
	f.SetMode(comfytest.Slow)
	addr := freeAddr(t)
	t.Setenv("HORUS_DATABASE_URL", dbURL)
	t.Setenv("HORUS_API_PORT", strings.Split(addr, ":")[1])
	t.Setenv("HORUS_SHARED_DIRS", `C:\horus\outputs=/shared/outputs`)
	t.Setenv("HORUS_HOST_OS", "windows")
	settings.Reset()

	var wlog, alog lockedBuffer
	rctx, stop := context.WithCancel(ctx)
	workerDone, apiDone := make(chan error, 1), make(chan error, 1)
	go func() { workerDone <- runWorker(rctx, &wlog) }()
	t.Cleanup(func() {
		stop()
		for name, done := range map[string]chan error{"horus worker": workerDone, "horus api": apiDone} {
			select {
			case <-done:
			case <-time.After(15 * time.Second):
				t.Errorf("%s did not stop in 15 s:\n%s\n%s", name, wlog.String(), alog.String())
			}
		}
	})
	db, err := postgres.Connect(ctx, dbURL)
	if err != nil {
		apiDone <- nil
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	eventually(t, "the worker applies the schema", 15*time.Second, func() bool { return db.CheckSchema(ctx) == nil })
	apiErr := make(chan error, 1)
	go func() {
		err := runAPI(rctx, addr, &alog)
		apiErr <- err
		apiDone <- err
	}()
	base := "http://" + addr
	eventually(t, "horus api answers", 15*time.Second, func() bool {
		select {
		case err := <-apiErr:
			t.Fatalf("horus api stopped: %v: %s", err, alog.String())
		default:
		}
		resp, err := http.Get(base + "/livez")
		if err == nil {
			resp.Body.Close()
		}
		return err == nil && resp.StatusCode == 200
	})

	queue := jobs.New(postgres.NewStore(db), postgres.NewBroker(db, postgres.BrokerOptions{}), jobs.Config{})
	p, err := newImages("").Check(generate.Request{Workflow: "it-illustration", Subject: testSubject + " The picture contains no text.", Seed: int64(7)})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	j, err := queue.Submit(ctx, jobdata.Job{Tool: "generate_image", Workflow: p.Workflow, Backend: "comfyui", Payload: raw})
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the job runs on the fake ComfyUI", 15*time.Second, func() bool { return len(f.Jobs()) == 1 })
	backendID := f.Jobs()[0]

	// read gives the panel "Running" and the summary after the summary shows
	// the step value of total.
	read := func(value, total float64) runningRead {
		t.Helper()
		var r runningRead
		eventually(t, fmt.Sprintf("/api/summary gives step %g of %g for the running job", value, total), 15*time.Second, func() bool {
			var v webui.View
			_ = json.Unmarshal([]byte(fetch(t, "GET", base+"/api/summary", "", 200)), &v)
			if len(v.RunningJobs) != 1 || v.RunningJobs[0].ID != j.ID {
				return false
			}
			r.step, r.steps = v.RunningJobs[0].Step, v.RunningJobs[0].Steps
			return r.steps == total && (total == 0 || r.step == value)
		})
		panel := part(fetch(t, "GET", base+"/", "", 200), "panel-running")
		bar, line := runningBar.FindStringSubmatch(panel), runningLine.FindAllStringSubmatch(panel, -1)
		if bar == nil || len(line) == 0 {
			t.Fatalf("the panel Running has no bar or no line:\n%s", panel)
		}
		r.hover, r.line = bar[1], line[len(line)-1][1]
		r.bar, _ = strconv.Atoi(bar[2])
		return r
	}
	report := func(value, total int) runningRead {
		t.Helper()
		if !f.Step(backendID, value, total) {
			t.Fatalf("the fake ComfyUI has no WebSocket of the client id of the job %s: horus does not listen to the steps, "+
				"and ComfyUI gives a step only there", backendID)
		}
		return read(float64(value), float64(total))
	}

	first := report(3, 15)
	second := report(9, 15)
	for i, r := range []runningRead{first, second} {
		want := fmt.Sprintf("step %g of %g", r.step, r.steps)
		if r.step <= 0 || r.step >= r.steps {
			t.Errorf("read %d: step %g of %g, want a step above 0 and below the total", i+1, r.step, r.steps)
		}
		if got := int(r.step / r.steps * 100); r.bar != got {
			t.Errorf("read %d: the bar is %d%% wide, want %d%% for %s", i+1, r.bar, got, want)
		}
		if r.hover != "ComfyUI reports "+want {
			t.Errorf("read %d: the hover text of the bar is %q, want %q", i+1, r.hover, "ComfyUI reports "+want)
		}
		if !strings.HasPrefix(r.line, want+" · running ") {
			t.Errorf("read %d: the line below the bar is %q, want it to start with %q", i+1, r.line, want+" · running ")
		}
	}
	if second.step <= first.step || second.bar <= first.bar {
		t.Errorf("the second read (step %g, bar %d%%) is not larger than the first read (step %g, bar %d%%)",
			second.step, second.bar, first.step, first.bar)
	}
	// The progress text of a waiting call is the message of the job row.
	eventually(t, "the progress text names step 9 of 15", 15*time.Second, func() bool {
		st, err := queue.Progress(ctx, j.ID)
		return err == nil && strings.HasPrefix(st.Message, "rendering, step 9 of 15, ")
	})

	// The WebSocket ends while the job runs: no step, an empty bar, and the
	// job runs on.
	f.DropSockets()
	lost := read(0, 0)
	if lost.bar != 0 || strings.Contains(lost.line, "step ") || !strings.Contains(lost.hover, "ComfyUI reports no step of this job now") {
		t.Errorf("without the WebSocket: bar %d%%, hover text %q, line %q; want an empty bar and no step", lost.bar, lost.hover, lost.line)
	}
	if !strings.HasPrefix(lost.line, "rendering, ") {
		t.Errorf("without the WebSocket: the line below the bar is %q, want the text of the run (rendering, N s)", lost.line)
	}
	if st, err := queue.Progress(ctx, j.ID); err != nil || st.State != jobdata.Running {
		t.Errorf("without the WebSocket: the job is %+v, %v; want running", st, err)
	}
	t.Logf("ui-running-step: read 1: step %g of %g, bar %d%%; read 2: step %g of %g, bar %d%%; without the WebSocket: bar %d%%",
		first.step, first.steps, first.bar, second.step, second.steps, second.bar, lost.bar)

	if _, err := queue.Cancel(ctx, j.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the job is cancelled", 15*time.Second, func() bool {
		st, err := queue.Progress(ctx, j.ID)
		return err == nil && st.State == jobdata.Cancelled
	})
}
