// Package brokertest holds the contract suite of the interface jobs.Broker,
// the in-memory broker that passes it, and the in-memory job store of the
// tests.
//
// A broker passes Run before it ships (ADR 0026). The test of the in-memory
// broker is TestMemoryBroker. The test of the Postgres broker is
// TestPostgresBroker, under ./internal/brokertest/..., so that
// `go test ./internal/brokertest/...` runs one suite on both brokers.
//
// Only a test file imports this package.
package brokertest

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// The times of the checks.
var (
	// Timeout is the longest wait for a delivery or an event that must come.
	Timeout = 10 * time.Second
	// Quiet is the wait in which a delivery that must not come does not come.
	Quiet = 300 * time.Millisecond
	// Hold is how long the check long_delivery holds a delivery without an
	// ack. A broker that hides a delivery for a time (a visibility time) gets
	// a visibility time shorter than Hold in its test. So the check proves
	// that the broker extends the visibility time while a delivery runs.
	Hold = 3 * time.Second
)

// check is one check of the contract.
type check struct {
	name string
	run  func(t testing.TB, b jobs.Broker)
}

// checks are the checks of the contract, in the order of Run.
var checks = []check{
	{"order", checkOrder},
	{"at_most_prefetch", checkPrefetch},
	{"ack_deletes", checkAckDeletes},
	{"again_after_consumer_stops", checkAgainAfterStop},
	{"two_consumers_never_share", checkTwoConsumers},
	{"nack_requeue", checkNackRequeue},
	{"nack_drop", checkNackDrop},
	{"queues_are_separate", checkSeparateQueues},
	{"notify_each_listener", checkNotifyEachListener},
	{"listen_from_its_start", checkListenFromStart},
	{"long_delivery", checkLongDelivery},
	{"depth", checkDepth},
}

// Names are the names of the checks: each one is a subtest of Run.
func Names() []string {
	var out []string
	for _, c := range checks {
		out = append(out, c.name)
	}
	return out
}

// Run runs each check of the contract as a subtest of t, on a new broker from
// newBroker for each check.
func Run(t *testing.T, newBroker func(t testing.TB) jobs.Broker) {
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) { c.run(t, newBroker(t)) })
	}
}

// Failures runs the check named name on the broker b, and gives its failures
// instead of failing t. The stub check of a broker uses it: a broker that
// breaks a rule must fail the check of that rule.
func Failures(t *testing.T, name string, b jobs.Broker) []string {
	t.Helper()
	for _, c := range checks {
		if c.name == name {
			return record(t, func(r testing.TB) { c.run(r, b) })
		}
	}
	t.Fatalf("no check %s", name)
	return nil
}

// record runs f with a testing.TB that records each failure and does not
// fail t, and gives the failures.
func record(t *testing.T, f func(r testing.TB)) []string {
	r := &recorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		f(r)
	}()
	<-done
	return r.failures()
}

// recorder is a testing.TB that records a failure and does not fail the test.
// Fatal ends the goroutine of the check, as testing.T does.
type recorder struct {
	testing.TB
	mu     sync.Mutex
	failed []string
}

func (r *recorder) fail(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed = append(r.failed, s)
}

func (r *recorder) failures() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.failed...)
}

func (r *recorder) Helper()                         {}
func (r *recorder) Error(a ...any)                  { r.fail(fmt.Sprint(a...)) }
func (r *recorder) Errorf(f string, a ...any)       { r.fail(fmt.Sprintf(f, a...)) }
func (r *recorder) Fail()                           { r.fail("Fail") }
func (r *recorder) FailNow()                        { r.fail("FailNow"); runtime.Goexit() }
func (r *recorder) Fatal(a ...any)                  { r.fail(fmt.Sprint(a...)); runtime.Goexit() }
func (r *recorder) Fatalf(f string, a ...any)       { r.fail(fmt.Sprintf(f, a...)); runtime.Goexit() }
func (r *recorder) Failed() bool                    { return len(r.failures()) > 0 }
func (r *recorder) Logf(format string, args ...any) {}

// msg is the message number i of a test.
func msg(i int) jobdata.Message {
	return jobdata.Message{V: jobdata.MessageVersion, JobID: fmt.Sprintf("job-%d", i), Tool: "generate_image", Backend: "comfyui"}
}

func publish(t testing.TB, b jobs.Broker, queue string, ids ...int) {
	t.Helper()
	for _, i := range ids {
		if err := b.Publish(context.Background(), queue, msg(i)); err != nil {
			t.Fatalf("Publish %s: %v", msg(i).JobID, err)
		}
	}
}

// consume starts a consumer. stop ends it; the test also ends it at its end.
func consume(t testing.TB, b jobs.Broker, queue string, prefetch int) (deliveries <-chan jobdata.Delivery, stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch, err := b.Consume(ctx, queue, prefetch)
	if err != nil {
		t.Fatalf("Consume %s: %v", queue, err)
	}
	return ch, cancel
}

// next gives the next delivery, and checks that it holds the message i.
func next(t testing.TB, ch <-chan jobdata.Delivery, i int) jobdata.Delivery {
	t.Helper()
	select {
	case d, ok := <-ch:
		if !ok {
			t.Fatalf("the deliveries closed, want %s", msg(i).JobID)
		}
		if d.Message != msg(i) {
			t.Fatalf("delivery %+v, want %+v", d.Message, msg(i))
		}
		return d
	case <-time.After(Timeout):
		t.Fatalf("no delivery in %s, want %s", Timeout, msg(i).JobID)
	}
	return jobdata.Delivery{}
}

// none checks that no delivery comes in Quiet.
func none(t testing.TB, ch <-chan jobdata.Delivery, why string) {
	t.Helper()
	select {
	case d, ok := <-ch:
		if ok {
			t.Fatalf("%s: a delivery of %s came", why, d.Message.JobID)
		}
	case <-time.After(Quiet):
	}
}

func ack(t testing.TB, d jobdata.Delivery) {
	t.Helper()
	if err := d.Ack(context.Background()); err != nil {
		t.Fatalf("Ack %s: %v", d.Message.JobID, err)
	}
}

// checkOrder: a consumer gets the messages in the order of Publish, each one
// as it was published.
func checkOrder(t testing.TB, b jobs.Broker) {
	publish(t, b, "jobs.order", 1, 2, 3, 4, 5)
	ch, _ := consume(t, b, "jobs.order", 3)
	for i := 1; i <= 5; i++ {
		ack(t, next(t, ch, i))
	}
}

// checkPrefetch: a consumer holds at most prefetch messages without an ack.
// An ack frees a place for the next message.
func checkPrefetch(t testing.TB, b jobs.Broker) {
	publish(t, b, "jobs.prefetch", 1, 2, 3, 4)
	ch, _ := consume(t, b, "jobs.prefetch", 2)
	d1 := next(t, ch, 1)
	d2 := next(t, ch, 2)
	none(t, ch, "prefetch 2 and 2 deliveries without an ack")
	ack(t, d1)
	d3 := next(t, ch, 3)
	none(t, ch, "prefetch 2 and 2 deliveries without an ack, after one ack")
	ack(t, d2)
	ack(t, d3)
	ack(t, next(t, ch, 4))
}

// checkAckDeletes: a message with an ack never comes again. The consumer acks
// message 1, holds message 2 and stops. The next consumer gets message 2
// first: a message 1 that was not deleted comes before it.
func checkAckDeletes(t testing.TB, b jobs.Broker) {
	publish(t, b, "jobs.ack", 1, 2)
	ch, stop := consume(t, b, "jobs.ack", 1)
	ack(t, next(t, ch, 1))
	next(t, ch, 2)
	stop()
	ch2, _ := consume(t, b, "jobs.ack", 1)
	ack(t, next(t, ch2, 2))
	none(t, ch2, "message 1 had an ack")
}

// checkAgainAfterStop: a message without an ack comes again to another
// consumer after its consumer stops (at-least-once).
func checkAgainAfterStop(t testing.TB, b jobs.Broker) {
	publish(t, b, "jobs.again", 1)
	ch, stop := consume(t, b, "jobs.again", 1)
	next(t, ch, 1)
	stop()
	ch2, _ := consume(t, b, "jobs.again", 1)
	ack(t, next(t, ch2, 1))
}

// checkTwoConsumers: two consumers of one queue never get the same message.
// Each message comes one time.
func checkTwoConsumers(t testing.TB, b jobs.Broker) {
	const n = 10
	ids := make([]int, n)
	for i := range ids {
		ids[i] = i + 1
	}
	publish(t, b, "jobs.two", ids...)
	got := make(chan string, 4*n)
	for range 2 {
		ch, _ := consume(t, b, "jobs.two", 2)
		go func() {
			for d := range ch {
				_ = d.Ack(context.Background())
				got <- d.Message.JobID
			}
		}()
	}
	count := map[string]int{}
	deadline := time.After(Timeout)
	for len(count) < n {
		select {
		case id := <-got:
			count[id]++
		case <-deadline:
			t.Fatalf("%d of %d messages came in %s", len(count), n, Timeout)
		}
	}
	quiet := time.After(Quiet)
wait:
	for {
		select {
		case id := <-got:
			count[id]++
		case <-quiet:
			break wait
		}
	}
	for _, i := range ids {
		if c := count[msg(i).JobID]; c != 1 {
			t.Errorf("%s came %d times, want 1", msg(i).JobID, c)
		}
	}
}

// checkNackRequeue: a nack with requeue gives the message again, at its place.
func checkNackRequeue(t testing.TB, b jobs.Broker) {
	publish(t, b, "jobs.nack", 1, 2)
	ch, _ := consume(t, b, "jobs.nack", 1)
	d := next(t, ch, 1)
	if err := d.Nack(context.Background(), true); err != nil {
		t.Fatalf("Nack with requeue: %v", err)
	}
	ack(t, next(t, ch, 1))
	ack(t, next(t, ch, 2))
}

// checkNackDrop: a nack without requeue deletes the message.
func checkNackDrop(t testing.TB, b jobs.Broker) {
	publish(t, b, "jobs.drop", 1, 2)
	ch, _ := consume(t, b, "jobs.drop", 1)
	d := next(t, ch, 1)
	if err := d.Nack(context.Background(), false); err != nil {
		t.Fatalf("Nack without requeue: %v", err)
	}
	ack(t, next(t, ch, 2))
	none(t, ch, "message 1 had a nack without requeue")
}

// checkSeparateQueues: a consumer gets only the messages of its queue.
func checkSeparateQueues(t testing.TB, b jobs.Broker) {
	publish(t, b, "jobs.a", 1)
	publish(t, b, "jobs.b", 2)
	cha, _ := consume(t, b, "jobs.a", 2)
	ack(t, next(t, cha, 1))
	none(t, cha, "message 2 is in another queue")
	chb, _ := consume(t, b, "jobs.b", 2)
	ack(t, next(t, chb, 2))
}

// depthIs checks the depth of the queue.
func depthIs(t testing.TB, b jobs.Broker, queue string, want int, when string) {
	t.Helper()
	n, err := b.Depth(context.Background(), queue)
	if err != nil || n != want {
		t.Errorf("Depth of %s %s: %d, %v; want %d", queue, when, n, err, want)
	}
}

// checkDepth: the depth of a queue counts the messages that wait for a
// delivery. A message that a consumer holds does not count, and an ack does
// not change the depth. Each queue has its own depth.
func checkDepth(t testing.TB, b jobs.Broker) {
	depthIs(t, b, "jobs.depth", 0, "before a publish")
	publish(t, b, "jobs.depth", 1, 2, 3)
	publish(t, b, "jobs.other", 4)
	depthIs(t, b, "jobs.depth", 3, "after 3 publishes")
	depthIs(t, b, "jobs.other", 1, "after 1 publish")
	ch, _ := consume(t, b, "jobs.depth", 1)
	d1 := next(t, ch, 1)
	depthIs(t, b, "jobs.depth", 2, "while a consumer holds 1 message")
	ack(t, d1)
	d2 := next(t, ch, 2)
	depthIs(t, b, "jobs.depth", 1, "after the ack of 1 message, while the consumer holds the next one")
	ack(t, d2)
	ack(t, next(t, ch, 3))
	depthIs(t, b, "jobs.depth", 0, "after the ack of each message")
	depthIs(t, b, "jobs.other", 1, "after the acks in another queue")
}

func listen(t testing.TB, b jobs.Broker) <-chan jobdata.Event {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch, err := b.Listen(ctx)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	return ch
}

func event(i int) jobdata.Event {
	return jobdata.Event{V: jobdata.MessageVersion, JobID: msg(i).JobID, Type: jobdata.EventType(jobdata.Running),
		At: time.Date(2026, 11, 20, 10, 0, i, 123456000, time.UTC), Seq: int64(i)}
}

func notify(t testing.TB, b jobs.Broker, i int) {
	t.Helper()
	if err := b.Notify(context.Background(), event(i)); err != nil {
		t.Fatalf("Notify: %v", err)
	}
}

// nextEvent checks that the next event of the listener is event i.
func nextEvent(t testing.TB, ch <-chan jobdata.Event, i int) {
	t.Helper()
	want := event(i)
	select {
	case e, ok := <-ch:
		if !ok {
			t.Fatalf("the events closed, want %s", want.JobID)
		}
		if e.V != want.V || e.JobID != want.JobID || e.Type != want.Type || e.Seq != want.Seq || !e.At.Equal(want.At) {
			t.Fatalf("event %+v, want %+v", e, want)
		}
	case <-time.After(Timeout):
		t.Fatalf("no event in %s, want %s", Timeout, want.JobID)
	}
}

// checkNotifyEachListener: Notify reaches each listener, with each field.
func checkNotifyEachListener(t testing.TB, b jobs.Broker) {
	l1 := listen(t, b)
	l2 := listen(t, b)
	notify(t, b, 1)
	nextEvent(t, l1, 1)
	nextEvent(t, l2, 1)
}

// checkListenFromStart: a listener gets no event from before Listen.
func checkListenFromStart(t testing.TB, b jobs.Broker) {
	notify(t, b, 1)
	l := listen(t, b)
	notify(t, b, 2)
	nextEvent(t, l, 2)
}

// checkLongDelivery: a delivery that runs longer than the visibility time of
// the broker does not come again while its consumer holds it: not to another
// consumer, and not to its own consumer, which has a free place. Its ack at
// the end works.
func checkLongDelivery(t testing.TB, b jobs.Broker) {
	publish(t, b, "jobs.long", 1)
	ch, _ := consume(t, b, "jobs.long", 2)
	d := next(t, ch, 1)
	time.Sleep(Hold)
	ch2, _ := consume(t, b, "jobs.long", 1)
	why := fmt.Sprintf("a delivery held for %s without an ack", Hold)
	none(t, ch2, why)
	none(t, ch, why+", to its own consumer")
	ack(t, d)
	none(t, ch2, "the message had an ack")
}
