// Package webui is the handler of the HTTP routes of horus api that are not
// MCP (ADR 0028):
//
//	GET /readyz       the readiness report of the 4 dependencies as JSON: 200 when horus is ready, else 503
//	GET /livez        200 while the process serves: the check of horus-api asks it
//	GET /             the Horus UI: a read-only page with the 11 parts of docs/specs/2026-11-19-job-queue-ui.html
//	GET /api/summary  the view of the page as JSON (View)
//	GET /api/events   a stream of Server-Sent Events: one event "job" for each event of the broker (Listen)
//
// The page, the summary and the events refuse a request whose Host header is
// not 127.0.0.1:<port> or localhost:<port> (403), so that a web page of
// another domain cannot read them through DNS rebinding. They need no token,
// and no route changes a job. The page is in the program (go:embed), with no
// build step. cmd/horus puts this handler and the handler of /mcp
// (mcpserver.NewHTTP) on one server. The spec: docs/specs/2026-11-19-job-queue.md, "Readiness" and "The
// Horus UI".
package webui

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/httpguard"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
)

// ReadyFunc gives the readiness report of horus now.
type ReadyFunc func(ctx context.Context) jobdata.Readiness

// Queue is what the Horus UI reads from the service of the job queue
// (internal/jobs).
type Queue interface {
	Summary(ctx context.Context) (jobdata.Summary, error)
	// Depth gives the count of the messages of the work queue of the backend
	// that wait for a delivery.
	Depth(ctx context.Context, backend string) (int, error)
	Listen(ctx context.Context) (<-chan jobdata.Event, error)
}

// Setup is what the Horus UI reads from the setup service
// (internal/provision): the install state of each workflow, as
// list_workflows shows it.
type Setup interface {
	List(ctx context.Context, workflowsDir string) ([]provision.Row, error)
}

// Config holds the parts of the routes. cmd/horus fills it.
type Config struct {
	// Ready runs the 4 checks of horus (mcpserver.Stack.Ready).
	Ready ReadyFunc
	// Queue and Setup give the data of the page. Without Queue, the page,
	// the summary and the events are not served.
	Queue Queue
	Setup Setup
	// ComfyVersion gives the version that ComfyUI reports in /system_stats.
	// nil: the page shows no version.
	ComfyVersion func(ctx context.Context) (string, error)
	// Ports are the ports that the Host header of a request of the page can
	// name: HORUS_API_PORT and the port of the listener.
	Ports []int
	// Settings are the settings that the page shows.
	Settings Settings
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

//go:embed page.html
var pageHTML string

var page = template.Must(template.New("page").Parse(pageHTML))

// KeepAlive is how often the stream of events sends a comment when no event
// comes, so that a proxy keeps the connection open.
var KeepAlive = 15 * time.Second

// Handler serves the routes of this package. ready runs the 4 checks at each
// request of /readyz.
func Handler(c Config) http.Handler {
	if c.Now == nil {
		c.Now = time.Now
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		report := c.Ready(r.Context())
		body, err := json.MarshalIndent(report, "", " ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if report.Ready {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_, _ = w.Write(append(body, '\n'))
	})
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("horus api serves\n"))
	})
	if c.Queue == nil {
		return mux
	}
	mux.HandleFunc("GET /{$}", c.guard(func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if err := page.Execute(&buf, c.view(r.Context())); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(buf.Bytes())
	}))
	mux.HandleFunc("GET /api/summary", c.guard(func(w http.ResponseWriter, r *http.Request) {
		body, err := json.MarshalIndent(c.view(r.Context()), "", " ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(append(body, '\n'))
	}))
	mux.HandleFunc("GET /api/events", c.guard(c.events))
	return mux
}

// guard refuses a request whose Host header is not 127.0.0.1:<port> or
// localhost:<port>, for a port of c.Ports, with 403 (httpguard.HostAllowed,
// the same check as /mcp). It sets the headers of
// a page that stays on this machine.
func (c Config) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
		if !httpguard.HostAllowed(r.Host, c.Ports) {
			http.Error(w, fmt.Sprintf("horus api: the Horus UI answers only to the Host 127.0.0.1:<port> or localhost:<port>, not %q", r.Host),
				http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// view runs the reads of one request and builds the view.
func (c Config) view(ctx context.Context) View {
	var r Reads
	r.Ready = c.Ready(ctx)
	r.Summary, r.SummaryErr = c.Queue.Summary(ctx)
	r.Depth, r.DepthErr = c.Queue.Depth(ctx, Backend)
	r.Flows, r.FlowsErr = c.flows(ctx)
	if c.ComfyVersion != nil && comfyUp(r.Ready) {
		if v, err := c.ComfyVersion(ctx); err == nil {
			r.ComfyVersion = v
		}
	}
	return Build(r, c.Settings, c.Now())
}

// flows gives the workflows that list_workflows shows, with their install
// state.
func (c Config) flows(ctx context.Context) ([]Flow, error) {
	all, err := catalog.All()
	if err != nil {
		return nil, err
	}
	state := map[string]provision.Row{}
	if c.Setup != nil {
		rows, err := c.Setup.List(context.WithoutCancel(ctx), settings.WorkflowsDir())
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			state[row.Name] = row
		}
	}
	out := []Flow{}
	for _, w := range all {
		f := Flow{Name: w.Name, Kind: w.Kind(), InstallState: provision.InstallState(0, 0)}
		if row, ok := state[w.Name]; ok {
			f.InstallState = row.State()
		}
		out = append(out, f)
	}
	return out, nil
}

// events is the stream of Server-Sent Events: one event "job" for each event
// of the broker, with the JSON of the jobdata.Event, until the client goes or
// horus api stops. When the broker cannot listen, the stream sends one event
// "down" and ends. The client connects again after 5 s.
func (c Config) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "horus api: this connection cannot stream", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	ch, err := c.Queue.Listen(r.Context())
	if err != nil {
		msg, _ := json.Marshal(map[string]string{"error": redactor(c.Settings.DatabaseURL)(err.Error())})
		fmt.Fprintf(w, "retry: 5000\nevent: down\ndata: %s\n\n", msg)
		flusher.Flush()
		return
	}
	fmt.Fprint(w, "retry: 5000\n\n")
	flusher.Flush()
	keep := time.NewTicker(KeepAlive)
	defer keep.Stop()
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(e)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: job\ndata: %s\n\n", data)
		case <-keep.C:
			fmt.Fprint(w, ": keep-alive\n\n")
		case <-r.Context().Done():
			return
		}
		flusher.Flush()
	}
}

// Ports gives the ports of a Host header: the port of HORUS_API_PORT and the
// port of the address listen, without a copy.
func Ports(apiPort int, listen string) []int {
	out := []int{apiPort}
	if _, ps, err := net.SplitHostPort(listen); err == nil {
		if p, err := strconv.Atoi(ps); err == nil && p > 0 && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}
