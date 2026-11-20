// Tests of the step of a job: the WebSocket of ComfyUI.
package comfy

import (
	"context"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/comfy/comfytest"
	"github.com/hieplam/horus/internal/graph"
)

// until waits until cond is true, for at most 10 s.
func until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not in 10 s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// slowJob submits the graph of it-illustration as a job that runs for ever.
func slowJob(ctx context.Context, t *testing.T, f *comfytest.Fake, c *Client) (id, sampler string) {
	t.Helper()
	info, err := c.ObjectInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	api, err := graph.UIToAPI(readFile(t, "..", "..", "workflows", "it-illustration", "workflow.json"), info)
	if err != nil {
		t.Fatal(err)
	}
	f.SetMode(comfytest.Slow)
	id, err = c.Submit(ctx, api)
	if err != nil {
		t.Fatal(err)
	}
	return id, f.SamplerNode(id)
}

// TestStepComesFromTheWebSocket: the fake gives a step as ComfyUI 0.44.0
// does, in a "progress" message on the WebSocket of the client id of the job.
// Step gives the last step of the node that the caller names, and no step of
// another node. A cancel closes the WebSocket and drops the steps.
func TestStepComesFromTheWebSocket(t *testing.T) {
	f := comfytest.New(t, fixturePath())
	c := New(f.URL())
	ctx := context.Background()
	id, sampler := slowJob(ctx, t, f, c)
	if f.Job(id).ClientID == "" || f.Sockets() != 1 {
		t.Fatalf("after Submit: client_id %q, %d open WebSockets; want a client id and 1 WebSocket", f.Job(id).ClientID, f.Sockets())
	}
	if v, total, ok := c.Step(id, sampler); ok {
		t.Fatalf("before the first step: step %g of %g, want no step", v, total)
	}
	for _, step := range []int{1, 2, 9} {
		if !f.Step(id, step, 15) {
			t.Fatal("the fake has no WebSocket of the job")
		}
		until(t, "the step of the sampler node", func() bool {
			v, total, ok := c.Step(id, sampler)
			return ok && v == float64(step) && total == 15
		})
	}
	// A message of another node, and a message that is not a step.
	f.Send(id, map[string]any{"type": "progress", "data": map[string]any{"value": 1, "max": 4, "prompt_id": id, "node": "another-node"}})
	f.Send(id, map[string]any{"type": "executing", "data": map[string]any{"node": "another-node", "prompt_id": id}})
	until(t, "the step of the other node", func() bool { _, _, ok := c.Step(id, "another-node"); return ok })
	if v, total, ok := c.Step(id, sampler); !ok || v != 9 || total != 15 {
		t.Errorf("after a step of another node: the sampler node has step %g of %g (%v), want 9 of 15", v, total, ok)
	}
	if _, _, ok := c.Step("no-such-job", sampler); ok {
		t.Error("a job that this client did not submit has a step")
	}

	c.Cancel(ctx, id)
	if _, _, ok := c.Step(id, sampler); ok {
		t.Error("after Cancel: the job still has a step")
	}
	until(t, "the WebSocket closes after Cancel", func() bool { return f.Sockets() == 0 })
}

// TestStepsStayAtTheEndOfTheJobAndGoWithTheSocket: after execution_success
// the adapter closes the WebSocket and keeps the last step until History
// reports the end. A WebSocket that ends before the job loses the steps: a
// step that no longer changes is not the step of the job.
func TestStepsStayAtTheEndOfTheJobAndGoWithTheSocket(t *testing.T) {
	f := comfytest.New(t, fixturePath())
	c := New(f.URL())
	ctx := context.Background()
	id, sampler := slowJob(ctx, t, f, c)
	f.Step(id, 15, 15)
	f.End(id)
	until(t, "the WebSocket closes at the end of the job", func() bool { return f.Sockets() == 0 })
	if v, total, ok := c.Step(id, sampler); !ok || v != 15 || total != 15 {
		t.Errorf("after the end of the job: step %g of %g (%v), want 15 of 15", v, total, ok)
	}

	lost, sampler := slowJob(ctx, t, f, c)
	f.Step(lost, 3, 15)
	until(t, "step 3 of 15", func() bool { v, _, ok := c.Step(lost, sampler); return ok && v == 3 })
	f.DropSockets()
	until(t, "no step after the WebSocket ended", func() bool { _, _, ok := c.Step(lost, sampler); return !ok })
	if e, err := c.History(ctx, lost); err != nil || e != nil {
		t.Errorf("the job after its WebSocket ended: %+v, %v; want a job that still runs", e, err)
	}
}

// TestSubmitWithoutAWebSocket: a ComfyUI that refuses the WebSocket still
// takes the job. The job has no step.
func TestSubmitWithoutAWebSocket(t *testing.T) {
	f := comfytest.New(t, fixturePath())
	f.RefuseSockets()
	c := New(f.URL())
	ctx := context.Background()
	id, sampler := slowJob(ctx, t, f, c)
	if id != "job-1" || f.Step(id, 3, 15) {
		t.Fatalf("Submit without a WebSocket: job %q, and the fake sent a step", id)
	}
	if _, _, ok := c.Step(id, sampler); ok {
		t.Error("a job without a WebSocket has a step")
	}
	if s, err := c.QueueState(ctx, id); err != nil || !s.Running {
		t.Errorf("the job without a WebSocket: %+v, %v; want running", s, err)
	}
}

// TestTheWebSocketEndsWithTheContextOfSubmit: when the context of Submit
// ends, the WebSocket closes.
func TestTheWebSocketEndsWithTheContextOfSubmit(t *testing.T) {
	f := comfytest.New(t, fixturePath())
	c := New(f.URL())
	ctx, cancel := context.WithCancel(context.Background())
	slowJob(ctx, t, f, c)
	if f.Sockets() != 1 {
		t.Fatalf("%d open WebSockets after Submit, want 1", f.Sockets())
	}
	cancel()
	until(t, "the WebSocket closes", func() bool { return f.Sockets() == 0 })
}
