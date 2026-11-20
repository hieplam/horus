// Package webui is the handler of the HTTP routes of horus api that are not
// MCP (ADR 0028):
//
//	GET /readyz   the readiness report of the 4 dependencies as JSON: 200 when horus is ready, else 503
//	GET /livez    200 while the process serves: the check of horus-api asks it
//
// The Horus UI (the page, the JSON summary and the stream of events) comes
// here too. cmd/horus puts this handler and the handler of /mcp on one
// server. The spec: docs/specs/2026-11-19-job-queue.md, "Readiness".
package webui

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/hieplam/horus/internal/jobdata"
)

// ReadyFunc gives the readiness report of horus now.
type ReadyFunc func(ctx context.Context) jobdata.Readiness

// Handler serves the routes of this package. ready runs the 4 checks at each
// request of /readyz.
func Handler(ready ReadyFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		report := ready(r.Context())
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
	return mux
}
