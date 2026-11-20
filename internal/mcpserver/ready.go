// The readiness of horus in the stack (spec docs/specs/2026-11-19-job-queue.md,
// "Readiness"; ADR 0029). horus serves only when its 4 dependencies are up:
// horus-db, ComfyUI, horus-worker and horus-api. This handler joins the
// checks of the services internal/jobs (horus-db, horus-worker) and
// internal/generate (ComfyUI), and the check of horus-api, into one report
// (jobdata.ReadinessOf). horus ready, /readyz of horus api and horus mcp in
// the stack use the same report.
package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/settings"
)

// ReadyFunc gives the readiness report of horus in the stack. With only, it
// runs only the checks of those dependencies, and the report gives the others
// as "no check result".
type ReadyFunc func(ctx context.Context, only ...string) jobdata.Readiness

// QueueChecks is what the readiness needs from the job queue service
// (internal/jobs): the results of horus-db and horus-worker.
type QueueChecks interface {
	Checks(ctx context.Context) []jobdata.CheckResult
}

// CheckTimeout is the most time of one check: ComfyUI must answer in 2 s.
const CheckTimeout = 2 * time.Second

// LivePath is the path of horus api that answers 200 while the process
// serves. The check of horus-api asks it.
const LivePath = "/livez"

// Stack holds the parts of the 4 checks.
type Stack struct {
	// Queue gives the checks of horus-db and horus-worker.
	Queue QueueChecks
	// Images gives the check of ComfyUI: GET /system_stats.
	Images Images
	// APIURL is the address of horus api, for example http://127.0.0.1:8190.
	// The check of horus-api asks APIURL + LivePath. Empty: this process is
	// horus api, and it serves, so horus-api is up.
	APIURL string
}

// Ready runs the checks of the dependencies in only (all 4 when only is
// empty) at the same time, each one in at most CheckTimeout, and joins the
// results with jobdata.ReadinessOf.
func (s Stack) Ready(ctx context.Context, only ...string) jobdata.Readiness {
	want := func(deps ...string) bool {
		if len(only) == 0 {
			return true
		}
		for _, d := range deps {
			if slices.Contains(only, d) {
				return true
			}
		}
		return false
	}
	var (
		mu      sync.Mutex
		results []jobdata.CheckResult
		wg      sync.WaitGroup
	)
	run := func(check func(ctx context.Context) []jobdata.CheckResult) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, CheckTimeout)
			defer cancel()
			rs := check(cctx)
			mu.Lock()
			results = append(results, rs...)
			mu.Unlock()
		}()
	}
	if want(jobdata.DependencyDB, jobdata.DependencyWorker) {
		run(s.Queue.Checks)
	}
	if want(jobdata.DependencyComfyUI) {
		run(s.comfyCheck)
	}
	if want(jobdata.DependencyAPI) {
		run(s.apiCheck)
	}
	wg.Wait()
	if len(only) > 0 {
		results = slices.DeleteFunc(results, func(r jobdata.CheckResult) bool { return !slices.Contains(only, r.Dependency) })
	}
	return jobdata.ReadinessOf(results)
}

// comfyCheck: ComfyUI answers GET /system_stats with 200 in CheckTimeout.
func (s Stack) comfyCheck(ctx context.Context) []jobdata.CheckResult {
	url := settings.ComfyURL() + "/system_stats"
	up, err := s.Images.Running(ctx)
	c := jobdata.CheckResult{Dependency: jobdata.DependencyComfyUI, Up: up && err == nil}
	switch {
	case c.Up:
		c.Detail = "answers at " + url
	case err != nil:
		c.Detail = fmt.Sprintf("no answer from %s in %s: %v", url, CheckTimeout, err)
	default:
		c.Detail = fmt.Sprintf("no answer from %s in %s: the owner starts ComfyUI on the host", url, CheckTimeout)
	}
	return []jobdata.CheckResult{c}
}

// apiCheck: horus api answers APIURL + LivePath with 200 in CheckTimeout.
func (s Stack) apiCheck(ctx context.Context) []jobdata.CheckResult {
	c := jobdata.CheckResult{Dependency: jobdata.DependencyAPI}
	if s.APIURL == "" {
		c.Up, c.Detail = true, "this process is horus api, and it serves"
		return []jobdata.CheckResult{c}
	}
	url := strings.TrimRight(s.APIURL, "/") + LivePath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.Detail = err.Error()
		return []jobdata.CheckResult{c}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.Detail = fmt.Sprintf("horus api does not answer at %s in %s", url, CheckTimeout)
		return []jobdata.CheckResult{c}
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.Detail = fmt.Sprintf("horus api answers %s at %s", resp.Status, url)
		return []jobdata.CheckResult{c}
	}
	c.Up, c.Detail = true, "answers at "+url
	return []jobdata.CheckResult{c}
}

// notReady is the error of a call while horus is not ready: NOT_READY, with
// the dependencies that are down in details.down and each result in
// details.checks.
func notReady(r jobdata.Readiness) *herr.Error {
	var why []string
	for _, c := range r.Checks {
		if !c.Up {
			why = append(why, c.Dependency+" ("+c.Detail+")")
		}
	}
	return herr.New("NOT_READY", "horus is not ready, these dependencies are down: "+strings.Join(why, "; "),
		map[string]any{"down": r.Down, "checks": r.Checks})
}
