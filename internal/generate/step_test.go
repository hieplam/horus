// The test of the step of the sampler node in the reports of a run.
package generate

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

// steps records the calls of Options.Step.
type steps struct {
	mu   sync.Mutex
	list []string
}

func (s *steps) step(_ context.Context, value, total float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, fmt.Sprintf("%g of %g", value, total))
}

func (s *steps) all() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.list...)
}

// TestReportsNameTheStepOfTheSampler: while a job runs, the service asks the
// adapter for the step of the KSampler node of the graph. Each new step goes
// to Options.Step one time, and the report names it. When the adapter has no
// step any more, Options.Step gets "0 of 0" and the report is the text
// without a step. Before the first step, the reports have no step.
func TestReportsNameTheStepOfTheSampler(t *testing.T) {
	e := setup(t)
	e.fake.SetMode(Slow)
	var r reports
	var s steps
	o := e.opts(&r)
	o.Step = s.step
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := e.svc.Generate(ctx, Request{Workflow: "it-illustration", Subject: subject}, o)
		done <- err
	}()
	wait := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				cancel()
				t.Fatalf("%s: not in 5 s. reports %v, steps %v", what, r.all(), s.all())
			}
			time.Sleep(time.Millisecond)
		}
	}
	wait("the first report of the run", func() bool { return hasReport(r.all(), "rendering, ") })
	if got := s.all(); len(got) != 0 {
		t.Errorf("Options.Step before the first step: %v, want no call", got)
	}
	e.fake.SetStep("job-1", 3, 15)
	wait("the report of step 3", func() bool { return hasReport(r.all(), "rendering, step 3 of 15, ") })
	e.fake.SetStep("job-1", 9, 15)
	wait("the report of step 9", func() bool { return hasReport(r.all(), "rendering, step 9 of 15, ") })
	before := len(r.all())
	e.fake.SetStep("job-1", 0, 0)
	wait("a report without a step", func() bool {
		all := r.all()
		return len(all) > before && hasReport(all[before:], "rendering, ")
	})
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("the cancelled call: %v", err)
	}
	if got, want := s.all(), []string{"3 of 15", "9 of 15", "0 of 0"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Options.Step got %v, want %v", got, want)
	}
	sampler := ""
	for id, n := range e.fake.Job("job-1").Graph {
		if n.(map[string]any)["class_type"] == "KSampler" {
			sampler = id
		}
	}
	if got := e.fake.Asked(); sampler == "" || !reflect.DeepEqual(got, []string{sampler}) {
		t.Errorf("the service asked the step of the nodes %v, want only the KSampler node %q", got, sampler)
	}
}
