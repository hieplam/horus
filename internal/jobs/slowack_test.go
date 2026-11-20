package jobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/brokertest"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// slowAck is a broker whose Ack waits before the real ack. The wait is the
// time that a real broker needs for an ack on the network.
type slowAck struct {
	*brokertest.Broker
	wait time.Duration
}

func (s slowAck) Consume(ctx context.Context, queue string, prefetch int) (<-chan jobdata.Delivery, error) {
	in, err := s.Broker.Consume(ctx, queue, prefetch)
	if err != nil {
		return nil, err
	}
	out := make(chan jobdata.Delivery)
	go func() {
		defer close(out)
		for d := range in {
			ack := d.Ack
			d.Ack = func(ctx context.Context) error {
				time.Sleep(s.wait)
				return ack(ctx)
			}
			select {
			case out <- d:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// TestWaitSlowAck: Wait returns when the job is succeeded, and the test ends
// before the ack of the consumer. The end of the test stops the consumer,
// and the late ack gets an error. That error is not a failure of the test.
func TestWaitSlowAck(t *testing.T) {
	e := newEnvWith(t, jobs.Config{DefaultRun: 4 * time.Minute}, func(b *brokertest.Broker) jobs.Broker {
		return slowAck{Broker: b, wait: 50 * time.Millisecond}
	})
	testWait(t, e)
}

// TestFailsTest: an error of a delivery fails the test while the consumer
// runs. Only after the end of the test started, it does not.
func TestFailsTest(t *testing.T) {
	boom := errors.New("ack failed")
	for _, c := range []struct {
		name   string
		ending bool
		o      jobs.Outcome
		err    error
		want   bool
	}{
		{"error while running", false, jobs.Ran, boom, true},
		{"error after the end started", true, jobs.Ran, boom, false},
		{"stopped while running", false, jobs.Stopped, boom, false},
		{"no error", false, jobs.Ran, nil, false},
	} {
		if got := failsTest(c.ending, c.o, c.err); got != c.want {
			t.Errorf("%s: failsTest = %v, want %v", c.name, got, c.want)
		}
	}
}
