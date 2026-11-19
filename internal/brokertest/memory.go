package brokertest

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/hieplam/horus/internal/jobdata"
)

// Broker is the in-memory broker of the tests. It passes the contract suite
// Run. Each queue keeps its messages in the order of Publish. A message that
// a consumer holds without an ack goes back to its place in the queue when
// the consumer stops.
type Broker struct {
	mu        sync.Mutex
	seq       int64
	ready     map[string][]*entry // the messages that no consumer holds, by queue, oldest first
	changed   chan struct{}       // closed and made again at each change
	listeners map[chan jobdata.Event]struct{}
}

type entry struct {
	id int64
	m  jobdata.Message
}

// consumer is one call of Consume: the messages that it holds without an ack.
type consumer struct {
	held    map[int64]*entry
	stopped bool
}

// NewBroker makes an empty in-memory broker.
func NewBroker() *Broker {
	return &Broker{ready: map[string][]*entry{}, changed: make(chan struct{}), listeners: map[chan jobdata.Event]struct{}{}}
}

// wake tells each waiting consumer that the queues changed. Hold mu.
func (b *Broker) wake() {
	close(b.changed)
	b.changed = make(chan struct{})
}

// put puts the entry back at its place in the queue. Hold mu.
func (b *Broker) put(queue string, e *entry) {
	q := b.ready[queue]
	i := sort.Search(len(q), func(i int) bool { return q[i].id > e.id })
	q = append(q, nil)
	copy(q[i+1:], q[i:])
	q[i] = e
	b.ready[queue] = q
}

// Publish adds the message at the end of the queue.
func (b *Broker) Publish(ctx context.Context, queue string, m jobdata.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	b.ready[queue] = append(b.ready[queue], &entry{id: b.seq, m: m})
	b.wake()
	return nil
}

// Consume gives the messages of the queue, oldest first, at most prefetch
// without an ack. The channel closes when ctx ends. Then each message that
// the consumer holds goes back to the queue.
func (b *Broker) Consume(ctx context.Context, queue string, prefetch int) (<-chan jobdata.Delivery, error) {
	if prefetch < 1 {
		return nil, errors.New("brokertest: prefetch must be 1 or more")
	}
	out := make(chan jobdata.Delivery)
	c := &consumer{held: map[int64]*entry{}}
	go func() {
		defer close(out)
		for {
			b.mu.Lock()
			var e *entry
			if len(c.held) < prefetch && len(b.ready[queue]) > 0 {
				e = b.ready[queue][0]
				b.ready[queue] = b.ready[queue][1:]
				c.held[e.id] = e
			}
			changed := b.changed
			b.mu.Unlock()
			if e == nil {
				select {
				case <-ctx.Done():
					b.stop(queue, c)
					return
				case <-changed:
					continue
				}
			}
			select {
			case out <- b.delivery(queue, c, e):
			case <-ctx.Done():
				b.stop(queue, c)
				return
			}
		}
	}()
	return out, nil
}

// stop puts each message that the consumer holds back in the queue.
func (b *Broker) stop(queue string, c *consumer) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c.stopped = true
	for id, e := range c.held {
		b.put(queue, e)
		delete(c.held, id)
	}
	b.wake()
}

// errStopped is the error of an ack or a nack after its consumer stopped.
var errStopped = errors.New("brokertest: the consumer of this delivery stopped, and the message is back in the queue")

func (b *Broker) delivery(queue string, c *consumer, e *entry) jobdata.Delivery {
	settle := func(requeue bool) error {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := c.held[e.id]; !ok || c.stopped {
			return errStopped
		}
		delete(c.held, e.id)
		if requeue {
			b.put(queue, e)
		}
		b.wake()
		return nil
	}
	return jobdata.Delivery{
		Message: e.m,
		Ack:     func(context.Context) error { return settle(false) },
		Nack:    func(_ context.Context, requeue bool) error { return settle(requeue) },
	}
}

// listenBuffer is how many events a listener can hold before it reads them.
// Notify drops an event for a listener that is full: it never waits.
const listenBuffer = 256

// Notify sends the event to each listener now. It keeps nothing.
func (b *Broker) Notify(ctx context.Context, e jobdata.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for l := range b.listeners {
		select {
		case l <- e:
		default:
		}
	}
	return nil
}

// Listen gives the events that Notify sends after Listen returns. The channel
// closes when ctx ends.
func (b *Broker) Listen(ctx context.Context) (<-chan jobdata.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l := make(chan jobdata.Event, listenBuffer)
	b.mu.Lock()
	b.listeners[l] = struct{}{}
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.listeners, l)
		close(l)
		b.mu.Unlock()
	}()
	return l, nil
}
