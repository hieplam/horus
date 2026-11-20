// MCP over HTTP at /mcp of horus api (spec docs/specs/2026-11-19-job-queue.md,
// "Two ways in"; ADR 0030 and ADR 0040). The same 6 tools, the same envelope
// and the same readiness gate as horus mcp over stdio, through the Streamable
// HTTP handler of the Go MCP SDK. In front of it, in this order:
//
//  1. a Host header that is not 127.0.0.1:<port> or localhost:<port>: 403;
//  2. an Origin header that is not the origin of horus api itself: 403;
//  3. no bearer token, or not the token of HORUS_API_TOKEN: 401 (a compare in
//     constant time). Without HORUS_API_TOKEN, each request gets 401;
//  4. a request that starts a new session (no Mcp-Session-Id header) while
//     horus is not ready: 503, with the readiness report as JSON.
//
// A call in an open session runs the readiness gate of each call, as over
// stdio: NOT_READY when a dependency is down. The token never reaches a tool:
// the handler removes the Authorization header after the check.
//
// A call that waits for a job holds its POST open for minutes, with a progress
// message every 5 s. When the client goes away (it closes the connection of
// the POST) or ends its session (DELETE), the call ends, and a queued or
// running job is cancelled, as when a stdio session ends.
package mcpserver

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hieplam/horus/internal/httpguard"
)

// HTTPConfig holds the parts of /mcp.
type HTTPConfig struct {
	// Token is HORUS_API_TOKEN. Empty: /mcp refuses each request with 401.
	Token string
	// Ports are the ports that the Host header and the Origin header can
	// name: HORUS_API_PORT and the port of the listener.
	Ports []int
}

// sessionHeader is the header of the session id of Streamable HTTP. A
// request without it starts a new session.
const sessionHeader = "Mcp-Session-Id"

// linkHeader carries the id of the HTTP request of a call to its tool
// handler. Only NewHTTP sets it: it replaces each value of the client.
const linkHeader = "Horus-Call-Link"

// linkKeep is how long a link stays after its POST ended, for a call that
// starts after the client went away.
const linkKeep = time.Minute

// NewHTTP gives the handler of /mcp. o holds the readiness gate and the job
// queue of the stack: each new session and each call run o.Ready. The stack
// starts no speech worker, so generate_speech refuses (ADR 0039).
func NewHTTP(images Images, setup Setup, o Options, c HTTPConfig) http.Handler {
	s := NewWith(images, setup, nil, o)
	links := newCallLinks()
	s.AddReceivingMiddleware(links.middleware)
	sdk := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if code, why := refuse(r, c); code != 0 {
			if code == http.StatusUnauthorized {
				w.Header().Set("WWW-Authenticate", `Bearer realm="horus"`)
			}
			http.Error(w, why, code)
			return
		}
		r.Header.Del("Authorization")
		if r.Header.Get(sessionHeader) == "" && o.Ready != nil {
			if report := o.Ready(r.Context()); !report.Ready {
				body, _ := json.MarshalIndent(report, "", " ")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write(append(body, '\n'))
				return
			}
		}
		links.serve(sdk, w, r)
	})
}

// refuse gives the status code and the text of the first check that fails,
// or 0 when the request passes the checks of the Host header, the Origin
// header and the bearer token.
func refuse(r *http.Request, c HTTPConfig) (int, string) {
	switch {
	case !httpguard.HostAllowed(r.Host, c.Ports):
		return http.StatusForbidden, "horus api: /mcp answers only to the Host 127.0.0.1:<port> or localhost:<port>"
	case !httpguard.OriginAllowed(r.Header.Get("Origin"), c.Ports):
		return http.StatusForbidden, "horus api: /mcp refuses a request from a web page of another origin"
	case c.Token == "":
		return http.StatusUnauthorized, "horus api: HORUS_API_TOKEN is not set in the .env of the stack, so /mcp refuses each request. " +
			"Set it, run docker compose up -d, and give the agent the same variable"
	case !httpguard.BearerMatches(r.Header.Get("Authorization"), c.Token):
		return http.StatusUnauthorized, "horus api: /mcp needs the header Authorization: Bearer <HORUS_API_TOKEN>"
	}
	return 0, ""
}

// callLinks joins each tool call to the HTTP request that carries it, and to
// its session. The SDK runs a tool with a context of the session. A POST that
// the client closes does not end the call, and the end of a session (DELETE)
// waits until each call of the session ends. So a link ends the call when its
// POST ends first, and a DELETE ends each call of its session first.
type callLinks struct {
	mu       sync.Mutex
	m        map[string]context.Context             // the context of each POST, by link id
	sessions map[string]map[*int]context.CancelFunc // the cancel of each running call, by session id
}

func newCallLinks() *callLinks {
	return &callLinks{m: map[string]context.Context{}, sessions: map[string]map[*int]context.CancelFunc{}}
}

// serve gives a POST a new link id, keeps the context of the request under
// that id while the SDK serves it, and some time after. Before a DELETE, it
// cancels each running call of the session.
func (l *callLinks) serve(next http.Handler, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		l.endSession(r.Header.Get(sessionHeader))
		next.ServeHTTP(w, r)
		return
	case http.MethodPost:
	default:
		next.ServeHTTP(w, r)
		return
	}
	id := rand.Text()
	r.Header.Set(linkHeader, id)
	l.mu.Lock()
	l.m[id] = r.Context()
	l.mu.Unlock()
	defer time.AfterFunc(linkKeep, func() {
		l.mu.Lock()
		delete(l.m, id)
		l.mu.Unlock()
	})
	next.ServeHTTP(w, r)
}

// endSession cancels each running call of the session.
func (l *callLinks) endSession(session string) {
	l.mu.Lock()
	calls := l.sessions[session]
	delete(l.sessions, session)
	l.mu.Unlock()
	for _, cancel := range calls {
		cancel()
	}
}

// middleware cancels a tool call when the HTTP request that carries it ends
// before the call, or when its session ends.
func (l *callLinks) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != "tools/call" {
			return next(ctx, method, req)
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		if ex := req.GetExtra(); ex != nil && ex.Header != nil {
			l.mu.Lock()
			hctx := l.m[ex.Header.Get(linkHeader)]
			l.mu.Unlock()
			if hctx != nil {
				stop := context.AfterFunc(hctx, cancel)
				defer stop()
			}
		}
		if ss := req.GetSession(); ss != nil && ss.ID() != "" {
			key, sid := new(int), ss.ID()
			l.mu.Lock()
			if l.sessions[sid] == nil {
				l.sessions[sid] = map[*int]context.CancelFunc{}
			}
			l.sessions[sid][key] = cancel
			l.mu.Unlock()
			defer func() {
				l.mu.Lock()
				delete(l.sessions[sid], key)
				if len(l.sessions[sid]) == 0 {
					delete(l.sessions, sid)
				}
				l.mu.Unlock()
			}()
		}
		return next(ctx, method, req)
	}
}
