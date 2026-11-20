//go:build goal

// The parts of the oracles of MCP over HTTP (task #204): a minimal MCP client
// over Streamable HTTP to /mcp of horus api in the test stack, with the
// standard library only. The bearer token is HORUS_API_TOKEN of the test
// stack: the suite makes it at run time, and never prints it.
package goal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// httpURL is /mcp of the test stack on 127.0.0.1 of the host.
const httpURL = "http://127.0.0.1:" + testAPIPort + "/mcp"

// errNoHTTP is the error of openHTTP when no session starts: the status code
// and the body of the answer, or the error of a request with no answer
// (status 0).
type errNoHTTP struct {
	status int
	body   string
	err    error
}

func (e *errNoHTTP) Error() string {
	if e.status == 0 {
		return fmt.Sprintf("no HTTP answer: %v", e.err)
	}
	return fmt.Sprintf("HTTP %d: %s", e.status, strings.TrimSpace(e.body))
}

// httpSession is one MCP session over Streamable HTTP.
type httpSession struct {
	token, id string
	mu        sync.Mutex
	next      int
}

// mcpRequest makes a POST of one JSON-RPC message to /mcp, with the headers of
// Streamable HTTP, the bearer token and the session id. edit changes the
// request before it goes.
func mcpRequest(token, session string, msg any, edit func(r *http.Request)) (*http.Request, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, httpURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	if edit != nil {
		edit(req)
	}
	return req, nil
}

// initMessage is the JSON-RPC request initialize.
func initMessage() map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-06-18",
		"capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "goal-suite-http", "version": "0"}}}
}

// rpcAnswer reads the answer of the request id from a response of /mcp: one
// JSON object, or a stream of Server-Sent Events whose data lines are
// JSON-RPC messages (progress notifications, then the answer).
func rpcAnswer(resp *http.Response, id int) (json.RawMessage, error) {
	type message struct {
		ID     *int            `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	pick := func(m message) (json.RawMessage, bool, error) {
		if m.ID == nil || *m.ID != id {
			return nil, false, nil
		}
		if len(m.Error) > 0 {
			return nil, true, fmt.Errorf("JSON-RPC error %s", m.Error)
		}
		return m.Result, true, nil
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		var m message
		if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
			return nil, err
		}
		if r, ok, err := pick(m); ok {
			return r, err
		}
		return nil, errors.New("the JSON answer has another id")
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data:")
		if !ok {
			continue
		}
		var m message
		if json.Unmarshal([]byte(strings.TrimSpace(data)), &m) != nil {
			continue
		}
		if r, ok, err := pick(m); ok {
			return r, err
		}
	}
	return nil, fmt.Errorf("the stream ended with no answer to request %d: %v", id, sc.Err())
}

// openHTTP starts a session: initialize, then notifications/initialized. It
// gives *errNoHTTP when /mcp does not answer 200, or gives no answer.
func openHTTP(token string, within time.Duration) (*httpSession, error) {
	client := &http.Client{Timeout: within}
	req, err := mcpRequest(token, "", initMessage(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &errNoHTTP{err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, &errNoHTTP{status: resp.StatusCode, body: string(b)}
	}
	if _, err := rpcAnswer(resp, 1); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	s := &httpSession{token: token, id: resp.Header.Get("Mcp-Session-Id"), next: 1}
	if s.id == "" {
		return nil, errors.New("initialize: no Mcp-Session-Id header")
	}
	req, err = mcpRequest(token, s.id, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}, nil)
	if err != nil {
		return nil, err
	}
	resp2, err := client.Do(req)
	if err != nil {
		return nil, &errNoHTTP{err: err}
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("notifications/initialized: HTTP %d, want 202", resp2.StatusCode)
	}
	return s, nil
}

// call calls a tool in the session and gives its envelope.
func (s *httpSession) call(tool string, args map[string]any, within time.Duration) (envelope, error) {
	s.mu.Lock()
	s.next++
	id := s.next
	s.mu.Unlock()
	req, err := mcpRequest(s.token, s.id, map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}}, nil)
	if err != nil {
		return envelope{}, err
	}
	resp, err := (&http.Client{Timeout: within}).Do(req)
	if err != nil {
		return envelope{}, fmt.Errorf("%s: no HTTP answer: %w", tool, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return envelope{}, fmt.Errorf("%s: HTTP %d: %s", tool, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	raw, err := rpcAnswer(resp, id)
	if err != nil {
		return envelope{}, fmt.Errorf("%s: %w", tool, err)
	}
	var res struct {
		StructuredContent envelope `json:"structuredContent"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return envelope{}, fmt.Errorf("%s: the result is not a tool result: %s", tool, raw)
	}
	return res.StructuredContent, nil
}

// close ends the session (DELETE). A stopped horus api gives no answer.
func (s *httpSession) close() {
	req, err := http.NewRequest(http.MethodDelete, httpURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Mcp-Session-Id", s.id)
	if resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req); err == nil {
		resp.Body.Close()
	}
}

// readinessReport is the body of a 503 of /mcp: the readiness report.
type readinessReport struct {
	Ready bool     `json:"ready"`
	Down  []string `json:"down"`
}

// downOf gives the dependencies that a 503 of /mcp names in its report.
func downOf(no *errNoHTTP) []string {
	var r readinessReport
	if no.status != http.StatusServiceUnavailable || json.Unmarshal([]byte(no.body), &r) != nil || r.Ready {
		return nil
	}
	return r.Down
}
