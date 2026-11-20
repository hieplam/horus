// Tests of /mcp (http.go): the 4 checks in front of the Streamable HTTP
// handler and their order, the same tools and gate over HTTP, and a client
// that goes away. The server runs on the job queue of queue_test.go. Each
// test makes its token at run time.
package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// httpEnv is /mcp on 127.0.0.1 of a free port, in front of a server of the
// stack.
type httpEnv struct {
	*stackEnv
	url   string // http://127.0.0.1:<port>/mcp
	host  string // 127.0.0.1:<port>
	token string
}

// newHTTPEnv starts /mcp with the token token ("" = HORUS_API_TOKEN is not
// set).
func newHTTPEnv(t *testing.T, token string) *httpEnv {
	t.Helper()
	e := newStackEnv(t, jobs.Config{})
	srv := httptest.NewUnstartedServer(nil)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	srv.Config.Handler = NewHTTP(e.images, noFilesSetup{}, e.opts, HTTPConfig{Token: token, Ports: []int{port}})
	srv.Start()
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	return &httpEnv{stackEnv: e, url: srv.URL + "/mcp", host: srv.Listener.Addr().String(), token: token}
}

// initBody is the JSON-RPC request initialize.
var initBody = map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
	"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "http-test", "version": "0"}}}

// post sends one JSON-RPC message to /mcp. edit changes the request before
// it goes. The caller closes the body.
func (h *httpEnv) post(t *testing.T, ctx context.Context, session string, msg any, edit func(r *http.Request)) *http.Response {
	t.Helper()
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+h.token)
	if session != "" {
		req.Header.Set(sessionHeader, session)
	}
	if edit != nil {
		edit(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// status sends initialize with edit, and gives the status code and the body.
func (h *httpEnv) status(t *testing.T, edit func(r *http.Request)) (int, string) {
	t.Helper()
	resp := h.post(t, context.Background(), "", initBody, edit)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// TestHTTPGuard sends the 5 requests of the goal row http-guard: each one
// has exactly one fault, and the last has none. Then it checks the order of
// the checks: Host, Origin, token, readiness.
func TestHTTPGuard(t *testing.T) {
	h := newHTTPEnv(t, rand.Text())
	cases := []struct {
		name string
		edit func(r *http.Request)
		want int
	}{
		{"no token", func(r *http.Request) { r.Header.Del("Authorization") }, http.StatusUnauthorized},
		{"a wrong token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+rand.Text()) }, http.StatusUnauthorized},
		{"a wrong Host", func(r *http.Request) { r.Host = "evil.example:" + strings.Split(h.host, ":")[1] }, http.StatusForbidden},
		{"a foreign Origin", func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }, http.StatusForbidden},
		{"the right request", nil, http.StatusOK},
	}
	refused := 0
	for _, c := range cases {
		code, body := h.status(t, c.edit)
		if code != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, code, body, c.want)
			continue
		}
		if code != http.StatusOK {
			refused++
		}
	}
	if refused != 4 {
		t.Errorf("bad requests refused: %d of 4", refused)
	}
	resp := h.post(t, context.Background(), "", initBody, func(r *http.Request) { r.Header.Del("Authorization") })
	resp.Body.Close()
	if !strings.HasPrefix(resp.Header.Get("WWW-Authenticate"), "Bearer") {
		t.Errorf("401 without WWW-Authenticate Bearer: %q", resp.Header.Get("WWW-Authenticate"))
	}

	// The order: a wrong Host comes before the token, the Origin before the
	// token, and the token before the readiness.
	h.down.Store(jobdata.DependencyComfyUI)
	for _, c := range []struct {
		name string
		edit func(r *http.Request)
		want int
	}{
		{"a wrong Host and no token", func(r *http.Request) { r.Host = "evil.example"; r.Header.Del("Authorization") }, http.StatusForbidden},
		{"a foreign Origin and no token", func(r *http.Request) {
			r.Header.Set("Origin", "http://evil.example")
			r.Header.Del("Authorization")
		}, http.StatusForbidden},
		{"no token while horus is not ready", func(r *http.Request) { r.Header.Del("Authorization") }, http.StatusUnauthorized},
	} {
		if code, body := h.status(t, c.edit); code != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, code, body, c.want)
		}
	}
}

// TestHTTPNoToken: without HORUS_API_TOKEN, /mcp refuses each request with
// 401, also a request with a token, and says why.
func TestHTTPNoToken(t *testing.T) {
	h := newHTTPEnv(t, "")
	h.token = rand.Text()
	code, body := h.status(t, nil)
	if code != http.StatusUnauthorized || !strings.Contains(body, "HORUS_API_TOKEN is not set") {
		t.Errorf("a request without HORUS_API_TOKEN on the server: %d %s, want 401 that says HORUS_API_TOKEN is not set", code, body)
	}
	h.token = ""
	if code, body := h.status(t, nil); code != http.StatusUnauthorized {
		t.Errorf("an empty bearer token without HORUS_API_TOKEN on the server: %d %s, want 401", code, body)
	}
}

// bearer is an http.RoundTripper that adds the bearer token.
type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// client connects the Go MCP client over Streamable HTTP with the token.
func (h *httpEnv) client(t *testing.T) (*mcp.ClientSession, error) {
	t.Helper()
	tr := &mcp.StreamableClientTransport{Endpoint: h.url, HTTPClient: &http.Client{Transport: bearer{h.token}}, MaxRetries: -1}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "http-test", Version: "0"}, nil).Connect(context.Background(), tr, nil)
	if err == nil {
		t.Cleanup(func() { _ = cs.Close() })
	}
	return cs, err
}

// TestHTTPSameToolsAndGate: over HTTP, a client sees the 6 tools and the
// envelope of stdio. While a dependency is down, a new session gets 503 with
// the readiness report, and a call in an open session gets NOT_READY.
func TestHTTPSameToolsAndGate(t *testing.T) {
	h := newHTTPEnv(t, rand.Text())
	cs, err := h.client(t)
	if err != nil {
		t.Fatal(err)
	}
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	if want := []string{"describe_workflow", "generate_image", "generate_speech", "install_workflow", "list_workflows", "uninstall_workflow"}; !slices.Equal(names, want) {
		t.Errorf("tools over HTTP: %v, want %v", names, want)
	}
	if env := envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{})); env["ok"] != true {
		t.Errorf("list_workflows over HTTP: %v", env)
	}

	h.down.Store(jobdata.DependencyComfyUI)
	env := envelopeOf(t, callTool(t, cs, "list_workflows", map[string]any{}))
	e, _ := env["error"].(map[string]any)
	details, _ := e["details"].(map[string]any)
	if e["code"] != "NOT_READY" || !slices.Contains(details["down"].([]any), any(jobdata.DependencyComfyUI)) {
		t.Errorf("a call in an open session while ComfyUI is down: %v, want NOT_READY with comfyui in details.down", env)
	}
	code, body := h.status(t, nil)
	var report jobdata.Readiness
	if code != http.StatusServiceUnavailable || json.Unmarshal([]byte(body), &report) != nil || !slices.Equal(report.Down, []string{jobdata.DependencyComfyUI}) {
		t.Errorf("a new session while ComfyUI is down: %d %s, want 503 with the report that names comfyui", code, body)
	}
	if _, err := h.client(t); err == nil {
		t.Error("the Go MCP client started a session while ComfyUI is down")
	}

	h.down.Store("")
	if _, err := h.client(t); err != nil {
		t.Errorf("a new session after ComfyUI is up again: %v", err)
	}
}

// openRaw starts a session with raw requests, and gives its id.
func (h *httpEnv) openRaw(t *testing.T) string {
	t.Helper()
	resp := h.post(t, context.Background(), "", initBody, nil)
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	id := resp.Header.Get(sessionHeader)
	if resp.StatusCode != http.StatusOK || id == "" {
		t.Fatalf("initialize: %d, session id %q", resp.StatusCode, id)
	}
	resp = h.post(t, context.Background(), id, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("notifications/initialized: %d", resp.StatusCode)
	}
	return id
}

// startCall posts generate_image with a progress token, and waits for the
// first progress message on the stream of the POST. Then ctx can end the
// POST.
func (h *httpEnv) startCall(t *testing.T, ctx context.Context, session string) {
	t.Helper()
	call := map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{
		"name": "generate_image", "arguments": map[string]any{"workflow": "it-illustration", "subject": testSubject},
		"_meta": map[string]any{"progressToken": "tok"}}}
	resp := h.post(t, ctx, session, call, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call: %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "data:") && strings.Contains(sc.Text(), "notifications/progress") {
			return
		}
	}
	t.Fatalf("the stream of the call ended before a progress message: %v", sc.Err())
}

// waitState waits until the one job of the job store is in state.
func (h *httpEnv) waitState(t *testing.T, state jobdata.State, why string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for h.onlyJob(t).State != state {
		if time.Now().After(deadline) {
			t.Fatalf("%s: the job is %s after 5 s, want %s", why, h.onlyJob(t).State, state)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestHTTPClientGoneCancelsJob: a client that closes the connection of its
// call while the job waits cancels the job, as the end of a stdio session
// does. No worker runs, so only the cancel moves the job.
func TestHTTPClientGoneCancelsJob(t *testing.T) {
	h := newHTTPEnv(t, rand.Text())
	session := h.openRaw(t)
	ctx, cancel := context.WithCancel(context.Background())
	h.startCall(t, ctx, session)
	if s := h.onlyJob(t).State; s != jobdata.Queued {
		t.Fatalf("the job is %s while its call waits, want queued", s)
	}
	cancel() // the client goes away: it closes the connection of the POST
	h.waitState(t, jobdata.Cancelled, "the client closed the connection of its call")
}

// TestHTTPSessionEndCancelsJob: a client that ends its session (DELETE)
// while a call waits cancels the job.
func TestHTTPSessionEndCancelsJob(t *testing.T) {
	h := newHTTPEnv(t, rand.Text())
	session := h.openRaw(t)
	h.startCall(t, context.Background(), session) // the POST stays open
	req, err := http.NewRequest(http.MethodDelete, h.url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set(sessionHeader, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE of the session: %d, want 204", resp.StatusCode)
	}
	h.waitState(t, jobdata.Cancelled, "the client ended its session")
}
