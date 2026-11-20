// The test of the step of the sampler node in the job row.
package worker_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/jobdata"
)

// TestProgressHoldsTheStepOfTheSampler: while a job runs, the worker writes
// the step that ComfyUI reports to the progress of the job row: value 3 of
// total 15, then 9 of 15, with the text of the run. Without a step the total
// is 0, so the Horus UI shows no bar that it cannot fill.
func TestProgressHoldsTheStepOfTheSampler(t *testing.T) {
	e := setup(t, options{})
	e.fake.setMode(modeHold)
	j, _ := e.submit(t, 1)
	e.start(t, e.config("worker-1", 1, nil))
	progress := func() jobdata.Progress {
		if p := e.get(t, j.ID).Progress; p != nil {
			return *p
		}
		return jobdata.Progress{}
	}
	waitFor(t, "the progress of the run without a step", func() bool {
		p := progress()
		return p.Total == 0 && strings.HasPrefix(p.Message, "rendering, ") && !strings.Contains(p.Message, "step")
	})
	for _, step := range []float64{3, 9} {
		e.fake.setStep(step, 15)
		want := fmt.Sprintf("rendering, step %g of 15, ", step)
		waitFor(t, "the job row holds "+want, func() bool {
			p := progress()
			return p.Value == step && p.Total == 15 && strings.HasPrefix(p.Message, want)
		})
	}
	e.fake.setStep(0, 0)
	waitFor(t, "the job row holds no step after ComfyUI reports none", func() bool {
		p := progress()
		return p.Total == 0 && !strings.Contains(p.Message, "step")
	})
	e.fake.release()
	waitFor(t, "the job succeeded", func() bool { return e.state(t, j.ID) == jobdata.Succeeded })
}
