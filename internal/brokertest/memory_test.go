package brokertest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// TestMemoryBroker runs the contract suite on the in-memory broker. The goal
// row broker-contract reads its subtests.
func TestMemoryBroker(t *testing.T) {
	Run(t, func(testing.TB) jobs.Broker { return NewBroker() })
}

// ignorePrefetch is a broker that ignores the prefetch.
type ignorePrefetch struct{ *Broker }

func (b ignorePrefetch) Consume(ctx context.Context, queue string, prefetch int) (<-chan jobdata.Delivery, error) {
	return b.Broker.Consume(ctx, queue, 1000)
}

// neverAgain is a broker that never delivers a message again: it acks each
// message when it delivers it (at-most-once).
type neverAgain struct{ *Broker }

func (b neverAgain) Consume(ctx context.Context, queue string, prefetch int) (<-chan jobdata.Delivery, error) {
	in, err := b.Broker.Consume(ctx, queue, prefetch)
	if err != nil {
		return nil, err
	}
	out := make(chan jobdata.Delivery)
	go func() {
		defer close(out)
		for d := range in {
			_ = d.Ack(ctx)
			noop := func(context.Context) error { return nil }
			select {
			case out <- jobdata.Delivery{Message: d.Message, Ack: noop, Nack: func(context.Context, bool) error { return nil }}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// keepOnAck is a broker whose ack does not delete the message: it gives the
// message back.
type keepOnAck struct{ *Broker }

func (b keepOnAck) Consume(ctx context.Context, queue string, prefetch int) (<-chan jobdata.Delivery, error) {
	in, err := b.Broker.Consume(ctx, queue, prefetch)
	if err != nil {
		return nil, err
	}
	out := make(chan jobdata.Delivery)
	go func() {
		defer close(out)
		for d := range in {
			d.Ack = func(ctx context.Context) error { return d.Nack(ctx, true) }
			select {
			case out <- d:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// TestChecksFailOnBadBrokers is the stub check of the suite: each bad broker
// fails the check of the rule that it breaks.
func TestChecksFailOnBadBrokers(t *testing.T) {
	old := Timeout
	Timeout = time.Second
	defer func() { Timeout = old }()
	for _, c := range []struct {
		check  string
		broker jobs.Broker
		want   string
	}{
		{"at_most_prefetch", ignorePrefetch{NewBroker()}, "prefetch 2 and 2 deliveries without an ack: a delivery of job-3 came"},
		{"again_after_consumer_stops", neverAgain{NewBroker()}, "no delivery in 1s, want job-1"},
		{"ack_deletes", keepOnAck{NewBroker()}, "want {V:1 JobID:job-2"},
	} {
		got := Failures(t, c.check, c.broker)
		if len(got) == 0 {
			t.Errorf("%s passes on a bad broker", c.check)
			continue
		}
		if !strings.Contains(got[0], c.want) {
			t.Errorf("%s: failure %q, want it to contain %q", c.check, got[0], c.want)
		}
	}
}

// TestNames checks the names of the checks that the goal row reads.
func TestNames(t *testing.T) {
	want := "order at_most_prefetch ack_deletes again_after_consumer_stops two_consumers_never_share " +
		"nack_requeue nack_drop queues_are_separate notify_each_listener listen_from_its_start long_delivery"
	if got := strings.Join(Names(), " "); got != want {
		t.Errorf("names %q, want %q", got, want)
	}
}

// An ack after the consumer stopped does nothing: the message is back in the
// queue.
func TestMemoryAckAfterStop(t *testing.T) {
	b := NewBroker()
	publish(t, b, "q", 1)
	ch, stop := consume(t, b, "q", 1)
	d := next(t, ch, 1)
	stop()
	for range ch {
	}
	if err := d.Ack(context.Background()); err == nil {
		t.Error("an ack after the consumer stopped: no error")
	}
	ch2, _ := consume(t, b, "q", 1)
	ack(t, next(t, ch2, 1))
}

// TestMemoryStore runs the store contract on the in-memory job store.
func TestMemoryStore(t *testing.T) {
	RunStore(t, func(_ testing.TB, now func() time.Time) (jobs.Store, func(string) []string) {
		s := NewStore()
		s.Now = now
		return s, s.Events
	})
}

// moveFromAny is a job store whose Move ignores the states from.
type moveFromAny struct{ *Store }

func (s moveFromAny) Move(ctx context.Context, id string, _ []jobdata.State, to jobdata.State, c jobdata.Change) (bool, error) {
	j, err := s.Get(ctx, id)
	if err != nil {
		return false, nil
	}
	return s.Store.Move(ctx, id, []jobdata.State{j.State}, to, c)
}

// noSweep is a job store whose Sweep changes nothing.
type noSweep struct{ *Store }

func (noSweep) Sweep(context.Context) (int, int, error) { return 0, 0, nil }

// noLock is a job store whose worker lock lets each caller in.
type noLock struct{ *Store }

func (noLock) WorkerLock(context.Context) (func(), error) { return func() {}, nil }

// TestStoreChecksFailOnBadStores is the stub check of the store contract:
// each bad store fails the check of the rule that it breaks.
func TestStoreChecksFailOnBadStores(t *testing.T) {
	for _, c := range []struct {
		check string
		wrap  func(*Store) jobs.Store
		want  string
	}{
		{"move_from_expected_state", func(s *Store) jobs.Store { return moveFromAny{s} }, "with the job in queued: true, want false"},
		{"sweep", func(s *Store) jobs.Store { return noSweep{s} }, "Sweep: 0 expired, 0 abandoned, <nil>; want 2 and 1"},
		{"worker_lock", func(s *Store) jobs.Store { return noLock{s} }, "error <nil>, want jobdata.ErrWorkerLocked"},
	} {
		got := storeFailures(t, c.check, func(_ testing.TB, now func() time.Time) (jobs.Store, func(string) []string) {
			s := NewStore()
			s.Now = now
			return c.wrap(s), s.Events
		})
		if len(got) == 0 {
			t.Errorf("%s passes on a bad store", c.check)
			continue
		}
		if !strings.Contains(got[0], c.want) {
			t.Errorf("%s: failure %q, want it to contain %q", c.check, got[0], c.want)
		}
	}
}
