// The step of a job that runs. ComfyUI gives the step of a node only on its
// WebSocket /ws, and only to the client id of the job: no REST route has it
// (ADR 0044).
package comfy

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// socketDial is the time limit of the open of the WebSocket of one job.
const socketDial = 5 * time.Second

// socketReadLimit is the largest message that the adapter reads. ComfyUI
// also sends the outputs of a node and, with a setting of the owner, preview
// images on the same socket.
const socketReadLimit = 64 << 20

// step is the last "progress" message of one node: value of max.
type step struct {
	value, max float64
}

// watch holds the steps that ComfyUI reported on the WebSocket of one job.
type watch struct {
	stop context.CancelFunc

	mu    sync.Mutex
	steps map[string]step // by node id
	lost  bool            // the socket ended before the end of the job
}

// message is one JSON message of the WebSocket.
type message struct {
	Type string `json:"type"`
	Data struct {
		// A "progress" message: the node reports value of max.
		Value float64 `json:"value"`
		Max   float64 `json:"max"`
		Node  *string `json:"node"`
	} `json:"data"`
}

// listen opens the WebSocket of the client id, and reads it until the job
// ends, the socket fails or ctx ends. It gives nil when ComfyUI refuses the
// WebSocket: the job then runs without steps.
func (c *Client) listen(ctx context.Context, clientID string) *watch {
	dctx, cancel := context.WithTimeout(ctx, socketDial)
	conn, _, err := websocket.Dial(dctx, c.base()+"/ws?clientId="+url.QueryEscape(clientID), nil)
	cancel()
	if err != nil {
		return nil
	}
	conn.SetReadLimit(socketReadLimit)
	rctx, stop := context.WithCancel(ctx)
	w := &watch{stop: stop, steps: map[string]step{}}
	go w.read(rctx, conn)
	return w
}

// read keeps the last "progress" message of each node. The job ended when
// ComfyUI sends execution_success, execution_error or execution_interrupted:
// the steps stay. Each other end of the socket loses the steps, because a
// step that no longer changes is not the step of the job.
func (w *watch) read(ctx context.Context, conn *websocket.Conn) {
	defer func() { _ = conn.CloseNow() }()
	for {
		kind, data, err := conn.Read(ctx)
		if err != nil {
			w.mu.Lock()
			w.lost = true
			w.mu.Unlock()
			return
		}
		if kind != websocket.MessageText {
			continue
		}
		var m message
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Type {
		case "progress":
			if m.Data.Node != nil {
				w.mu.Lock()
				w.steps[*m.Data.Node] = step{m.Data.Value, m.Data.Max}
				w.mu.Unlock()
			}
		case "execution_success", "execution_error", "execution_interrupted":
			return
		}
	}
}

// watching keeps the watch of the job.
func (c *Client) watching(promptID string, w *watch) {
	if w == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.watches == nil {
		c.watches = map[string]*watch{}
	}
	c.watches[promptID] = w
}

// forget closes the WebSocket of the job and drops its steps.
func (c *Client) forget(promptID string) {
	c.mu.Lock()
	w := c.watches[promptID]
	delete(c.watches, promptID)
	c.mu.Unlock()
	if w != nil {
		w.stop()
	}
}

// Step gives the last step that ComfyUI reported for the node of the job:
// value of total, for a sampler node "step 9 of 15". ok is false when
// ComfyUI reported no step of the node yet, when it refused the WebSocket of
// the job, and when the WebSocket ended before the job.
func (c *Client) Step(promptID, nodeID string) (value, total float64, ok bool) {
	c.mu.Lock()
	w := c.watches[promptID]
	c.mu.Unlock()
	if w == nil {
		return 0, 0, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	s, ok := w.steps[nodeID]
	if !ok || w.lost || s.max <= 0 {
		return 0, 0, false
	}
	return s.value, s.max, true
}
