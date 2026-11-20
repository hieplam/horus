package webui

import (
	"testing"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
)

// TestRunningRow: the row of a running job shows a step and a bar only for a
// progress with a total above 0, the step of the sampler node that the worker
// wrote. A progress without a total is the text of the run: the row shows
// that text, an empty bar, no step in the summary, and a hover text that
// says what the bar counts.
func TestRunningRow(t *testing.T) {
	job := func(p *jobdata.Progress) jobdata.Job {
		return jobdata.Job{ID: "0193a7c2-0000-7000-8000-000000000000", Workflow: "it-illustration",
			CreatedAt: at.Add(-151 * time.Second), StartedAt: at.Add(-151 * time.Second), Progress: p}
	}
	for _, c := range []struct {
		name        string
		progress    *jobdata.Progress
		step, steps float64
		percent     string
		title, line string
	}{
		{"a step", &jobdata.Progress{Value: 9, Total: 15, Message: "rendering, step 9 of 15, 151 s"}, 9, 15, "60%",
			"ComfyUI reports step 9 of 15", "step 9 of 15 · running 2 min 31 s · waited 0 s"},
		{"the first step", &jobdata.Progress{Value: 1, Total: 15, Message: "rendering, step 1 of 15, 20 s"}, 1, 15, "7%",
			"ComfyUI reports step 1 of 15", "step 1 of 15 · running 2 min 31 s · waited 0 s"},
		{"the text of the run, no step", &jobdata.Progress{Value: 143, Total: 0, Message: "rendering, 143 s"}, 0, 0, "0%",
			noStepTitle, "rendering, 143 s · running 2 min 31 s · waited 0 s"},
		{"no progress yet", nil, 0, 0, "0%", noStepTitle, "starting · running 2 min 31 s · waited 0 s"},
	} {
		r := runningRow(job(c.progress), 30*time.Minute, at)
		if r.Step != c.step || r.Steps != c.steps || r.Percent != c.percent || r.BarTitle != c.title || r.Line != c.line {
			t.Errorf("%s:\n got  step %g of %g, bar %s, hover text %q, line %q\n want step %g of %g, bar %s, hover text %q, line %q",
				c.name, r.Step, r.Steps, r.Percent, r.BarTitle, r.Line, c.step, c.steps, c.percent, c.title, c.line)
		}
	}
}
