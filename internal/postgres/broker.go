package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hieplam/horus/internal/jobdata"
)

// The channels of the broker.
const (
	channelMessages = "horus_messages" // the payload is the name of a work queue
	channelEvents   = "horus_events"   // the payload is the JSON of a jobdata.Event
)

// The defaults of BrokerOptions: the values of the spec.
const (
	DefaultVisibility = 60 * time.Second
	DefaultPoll       = 5 * time.Second
)

// BrokerOptions are the times of the Postgres broker. A zero field gets its
// default. The program uses the defaults. A test can set a short visibility
// time, so that a check runs longer than it.
type BrokerOptions struct {
	// Visibility is the time for which a delivery hides its message from every
	// consumer. Default 60 s.
	Visibility time.Duration
	// Extend is how often a consumer hides each message that it holds for
	// Visibility again, while the delivery runs. It must be shorter than
	// Visibility. Default Visibility / 3: 20 s.
	Extend time.Duration
	// Poll is how often a consumer reads its queue when no NOTIFY comes, and
	// the wait before a new connection when a LISTEN connection breaks.
	// Default 5 s.
	Poll time.Duration
}

func (o BrokerOptions) withDefaults() BrokerOptions {
	if o.Visibility <= 0 {
		o.Visibility = DefaultVisibility
	}
	if o.Extend <= 0 {
		o.Extend = o.Visibility / 3
	}
	if o.Poll <= 0 {
		o.Poll = DefaultPoll
	}
	return o
}

// claimSQL takes at most $2 visible messages of the queue $1, oldest first,
// and hides them for $3 milliseconds. FOR UPDATE SKIP LOCKED makes two claims
// at the same time take different messages.
const claimSQL = `UPDATE horus.messages SET visible_at = now() + $3::bigint * interval '1 millisecond', deliveries = deliveries + 1
WHERE id IN (
    SELECT id FROM horus.messages WHERE queue = $1 AND visible_at <= now()
    ORDER BY id LIMIT $2 FOR UPDATE SKIP LOCKED)
RETURNING id, body::text, deliveries`

// Broker is the Postgres broker: the table horus.messages and the channels
// horus_messages and horus_events. It implements jobs.Broker. It never reads
// a job table.
type Broker struct {
	db    *DB
	opt   BrokerOptions
	claim string // the SQL of a claim: claimSQL. A test of this package changes it.
}

// NewBroker gives the Postgres broker of the database.
func NewBroker(db *DB, o BrokerOptions) *Broker {
	return &Broker{db: db, opt: o.withDefaults(), claim: claimSQL}
}

// Publish adds the message at the end of the queue, and wakes the consumers
// of the queue (NOTIFY horus_messages), in one statement.
func (b *Broker) Publish(ctx context.Context, queue string, m jobdata.Message) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = b.db.pool.Exec(ctx, `WITH m AS (INSERT INTO horus.messages (queue, body) VALUES ($1, $2::jsonb) RETURNING id)
SELECT pg_notify('`+channelMessages+`', $1) FROM m`, queue, string(body))
	if err != nil {
		return fmt.Errorf("postgres: publish to %s: %w", queue, err)
	}
	return nil
}

// Notify sends the event to each process that listens now (NOTIFY
// horus_events). It keeps nothing.
func (b *Broker) Notify(ctx context.Context, e jobdata.Event) error {
	p, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := b.db.pool.Exec(ctx, "SELECT pg_notify($1, $2)", channelEvents, string(p)); err != nil {
		return fmt.Errorf("postgres: notify: %w", err)
	}
	return nil
}

// listenBuffer is how many events a listener can hold before it reads them.
// A listener that is full loses the event: an event is only a signal.
const listenBuffer = 256

// Listen gives the events that Notify sends after Listen returns. It holds
// one connection that LISTENs to horus_events. The channel closes when ctx
// ends.
func (b *Broker) Listen(ctx context.Context) (<-chan jobdata.Event, error) {
	out := make(chan jobdata.Event, listenBuffer)
	done, err := b.listen(ctx, channelEvents, func(payload string) {
		var e jobdata.Event
		if payload == "" || json.Unmarshal([]byte(payload), &e) != nil {
			return
		}
		select {
		case out <- e:
		default:
		}
	})
	if err != nil {
		return nil, err
	}
	go func() {
		<-done
		close(out)
	}()
	return out, nil
}

// listen opens a connection outside the pool and LISTENs to the channel.
// Then a goroutine gives the payload of each notification to on, until ctx
// ends. When the connection breaks, the goroutine opens a new one after
// Poll, and calls on("") because a notification can be lost in between. done
// closes when the goroutine ends and its connection is closed.
func (b *Broker) listen(ctx context.Context, channel string, on func(payload string)) (done <-chan struct{}, err error) {
	open := func() (*listenConn, error) {
		c, err := b.db.dial(ctx)
		if err != nil {
			return nil, err
		}
		if _, err := c.Exec(ctx, "LISTEN "+channel); err != nil {
			_ = c.Close(context.Background())
			return nil, err
		}
		return &listenConn{c}, nil
	}
	conn, err := open()
	if err != nil {
		return nil, fmt.Errorf("postgres: listen to %s: %w", channel, err)
	}
	d := make(chan struct{})
	go func() {
		defer close(d)
		for {
			n, err := conn.WaitForNotification(ctx)
			if err == nil {
				on(n.Payload)
				continue
			}
			conn.close()
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(b.opt.Poll):
				}
				if conn, err = open(); err == nil {
					break
				}
			}
			on("")
		}
	}()
	return d, nil
}

// listenConn is a connection that LISTENs to one channel.
type listenConn struct{ *pgx.Conn }

func (c *listenConn) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Close(ctx)
}

// errStopped is the error of an ack or a nack after the consumer of the
// delivery stopped: the message is back in the queue.
var errStopped = errors.New("postgres: the consumer of this delivery stopped, and the message is back in the queue")

// consumer is one call of Consume.
type consumer struct {
	b        *Broker
	queue    string
	prefetch int
	wake     chan struct{} // a NOTIFY of the queue, or a free place

	// mu guards held and stopped. An extension holds it during its SQL, so
	// that an ack, a nack or the stop comes before or after it, never during.
	mu      sync.Mutex
	held    map[int64]int32 // the id of each message without an ack, and its deliveries at the claim
	stopped bool
}

// Consume gives the messages of the queue, oldest first, at most prefetch
// without an ack. A delivery hides its message for the visibility time, and
// the consumer hides it again every Extend while it holds it. The channel
// closes when ctx ends. Then each message that the consumer holds is visible
// again at once, and the other consumers wake.
func (b *Broker) Consume(ctx context.Context, queue string, prefetch int) (<-chan jobdata.Delivery, error) {
	if prefetch < 1 {
		return nil, errors.New("postgres: prefetch must be 1 or more")
	}
	c := &consumer{b: b, queue: queue, prefetch: prefetch, wake: make(chan struct{}, 1), held: map[int64]int32{}}
	listening, err := b.listen(ctx, channelMessages, func(payload string) {
		if payload == queue || payload == "" {
			c.signal()
		}
	})
	if err != nil {
		return nil, err
	}
	out := make(chan jobdata.Delivery)
	go c.run(ctx, out, listening)
	return out, nil
}

// signal wakes the loop of the consumer. It never waits.
func (c *consumer) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// run is the loop of the consumer: it claims messages while it has a free
// place, gives them to out, and extends the visibility time of the messages
// that it holds. When ctx ends, it makes them visible, waits for its LISTEN
// connection to close, and closes out.
func (c *consumer) run(ctx context.Context, out chan<- jobdata.Delivery, listening <-chan struct{}) {
	defer close(out)
	defer func() { <-listening }()
	defer c.release()
	extend := time.NewTicker(c.b.opt.Extend)
	defer extend.Stop()
	poll := time.NewTicker(c.b.opt.Poll)
	defer poll.Stop()
	for {
		if free := c.free(); free > 0 {
			ds, err := c.claimN(ctx, free)
			if err == nil && len(ds) > 0 {
				for _, d := range ds {
					if !c.send(ctx, out, d, extend.C) {
						return
					}
				}
				continue
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
		case <-poll.C:
		case <-extend.C:
			c.extend(ctx)
		}
	}
}

// send gives the delivery to out, and extends while out waits. false: ctx
// ended.
func (c *consumer) send(ctx context.Context, out chan<- jobdata.Delivery, d jobdata.Delivery, extend <-chan time.Time) bool {
	for {
		select {
		case out <- d:
			return true
		case <-extend:
			c.extend(ctx)
		case <-ctx.Done():
			return false
		}
	}
}

// free gives the count of messages that the consumer can still take.
func (c *consumer) free() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.prefetch - len(c.held)
}

// claimN takes at most n messages, oldest first.
func (c *consumer) claimN(ctx context.Context, n int) ([]jobdata.Delivery, error) {
	rows, err := c.b.db.pool.Query(ctx, c.b.claim, c.queue, n, ms(c.b.opt.Visibility))
	if err != nil {
		return nil, err
	}
	type claimed struct {
		id         int64
		body       string
		deliveries int32
	}
	var got []claimed
	for rows.Next() {
		var m claimed
		if err := rows.Scan(&m.id, &m.body, &m.deliveries); err != nil {
			rows.Close()
			return nil, err
		}
		got = append(got, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// RETURNING keeps no order.
	sort.Slice(got, func(a, b int) bool { return got[a].id < got[b].id })
	c.mu.Lock()
	for _, m := range got {
		c.held[m.id] = m.deliveries
	}
	c.mu.Unlock()
	out := make([]jobdata.Delivery, 0, len(got))
	for _, m := range got {
		var msg jobdata.Message
		// Publish wrote the body from a Message. A body that does not decode
		// gives a zero Message: the worker finds no such job and acks it.
		_ = json.Unmarshal([]byte(m.body), &msg)
		out = append(out, c.delivery(m.id, m.deliveries, msg))
	}
	return out, nil
}

// delivery gives the delivery of the message id, whose claim set deliveries.
// The value of deliveries tells this delivery from a later one of the same
// message: an ack after the visibility time passed and another consumer got
// the message gives an error and deletes nothing.
func (c *consumer) delivery(id int64, deliveries int32, m jobdata.Message) jobdata.Delivery {
	take := func() error {
		c.mu.Lock()
		defer c.mu.Unlock()
		if _, ok := c.held[id]; !ok || c.stopped {
			return errStopped
		}
		delete(c.held, id)
		return nil
	}
	settle := func(ctx context.Context, requeue bool) error {
		if err := take(); err != nil {
			return err
		}
		defer c.signal()
		sql := "DELETE FROM horus.messages WHERE id = $1 AND deliveries = $2"
		if requeue {
			sql = `WITH m AS (UPDATE horus.messages SET visible_at = now() WHERE id = $1 AND deliveries = $2 RETURNING queue)
SELECT pg_notify('` + channelMessages + `', queue) FROM m`
		}
		tag, err := c.b.db.pool.Exec(ctx, sql, id, deliveries)
		if err != nil {
			return fmt.Errorf("postgres: settle the message %d: %w", id, err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("postgres: the message %d came again to another consumer: its visibility time passed", id)
		}
		return nil
	}
	return jobdata.Delivery{
		Message: m,
		Ack:     func(ctx context.Context) error { return settle(ctx, false) },
		Nack:    func(ctx context.Context, requeue bool) error { return settle(ctx, requeue) },
	}
}

// heldLists gives the ids and the deliveries of the held messages. Hold mu.
func (c *consumer) heldLists() (ids []int64, deliveries []int32) {
	for id, n := range c.held {
		ids = append(ids, id)
		deliveries = append(deliveries, n)
	}
	return ids, deliveries
}

// extend hides each held message for the visibility time again. A failed
// extension is tried again at the next tick: a message whose visibility time
// passes can come again (at-least-once).
func (c *consumer) extend(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || len(c.held) == 0 {
		return
	}
	ids, deliveries := c.heldLists()
	_, _ = c.b.db.pool.Exec(ctx, `UPDATE horus.messages AS m SET visible_at = now() + $1::bigint * interval '1 millisecond'
FROM unnest($2::bigint[], $3::integer[]) AS h(id, deliveries) WHERE m.id = h.id AND m.deliveries = h.deliveries`,
		ms(c.b.opt.Visibility), ids, deliveries)
}

// release stops the consumer: each held message is visible again at once, and
// the consumers of the queue wake. A release that fails leaves the messages
// hidden until their visibility time passes.
func (c *consumer) release() {
	c.mu.Lock()
	c.stopped = true
	ids, deliveries := c.heldLists()
	c.held = map[int64]int32{}
	c.mu.Unlock()
	if len(ids) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = c.b.db.pool.Exec(ctx, `WITH m AS (
    UPDATE horus.messages AS m SET visible_at = now()
    FROM unnest($1::bigint[], $2::integer[]) AS h(id, deliveries) WHERE m.id = h.id AND m.deliveries = h.deliveries
    RETURNING m.queue)
SELECT pg_notify('`+channelMessages+`', queue) FROM m`, ids, deliveries)
}
