// Tests of the readiness of the stack through the wiring of cmd/horus, with
// no Docker: horus worker and horus api in this process on a test database,
// and the fake ComfyUI of comfytest. The goal rows stack-starts-together and
// ready-when-four-up run the same parts in containers.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/postgres/pgtest"
	"github.com/hieplam/horus/internal/settings"
)

// freeAddr gives an address of 127.0.0.1 with a free port.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

// TestReadinessOnPostgres: with horus worker and horus api up, and ComfyUI
// up, the report is 4 of 4, /readyz answers 200 and /livez 200. ComfyUI
// down names comfyui. The worker stopped names horus-worker at once: its
// worker lock is gone. horus api stopped names horus-api.
func TestReadinessOnPostgres(t *testing.T) {
	pgtest.Need(t)
	url := pgtest.URL(t)
	f := imageEnv(t)
	addr := freeAddr(t)
	t.Setenv("HORUS_DATABASE_URL", url)
	t.Setenv("HORUS_API_PORT", strings.Split(addr, ":")[1])
	settings.Reset()

	wctx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	var wlog lockedBuffer
	workerDone := make(chan error, 1)
	go func() { workerDone <- runWorker(wctx, &wlog) }()

	actx, stopAPI := context.WithCancel(context.Background())
	defer stopAPI()
	var alog lockedBuffer
	apiDone := make(chan error, 1)
	go func() { apiDone <- runAPI(actx, addr, &alog) }()

	stack, done := stackOf(context.Background(), localAPI())
	defer done()
	ready := func() jobdata.Readiness { return stack.Ready(context.Background()) }
	eventually(t, "4 of 4", 15*time.Second, func() bool { return ready().Ready })

	resp, err := http.Get("http://" + addr + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	var report jobdata.Readiness
	_ = json.NewDecoder(resp.Body).Decode(&report)
	resp.Body.Close()
	if resp.StatusCode != 200 || !report.Ready || report.Up != 4 {
		t.Errorf("/readyz: %d %+v", resp.StatusCode, report)
	}

	f.SetDown(true)
	if r := ready(); r.Ready || strings.Join(r.Down, ",") != jobdata.DependencyComfyUI {
		t.Errorf("ComfyUI down: %+v", r)
	}
	resp, err = http.Get("http://" + addr + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("/readyz with ComfyUI down: %d, want 503", resp.StatusCode)
	}
	f.SetDown(false)

	stopWorker()
	if err := <-workerDone; err != nil {
		t.Fatalf("the worker: %v", err)
	}
	if r := ready(); r.Ready || strings.Join(r.Down, ",") != jobdata.DependencyWorker ||
		!strings.Contains(r.Checks[2].Detail, "no worker holds the worker lock") {
		t.Errorf("the worker stopped: %+v", r)
	}

	stopAPI()
	if err := <-apiDone; err != nil {
		t.Fatalf("horus api: %v", err)
	}
	if r := ready(); r.Ready || !strings.Contains(strings.Join(r.Down, ","), jobdata.DependencyAPI) {
		t.Errorf("horus api stopped: %+v", r)
	}
	if !strings.Contains(alog.String(), "horus api: serves the Horus UI, /readyz and /livez on http://"+addr) || !strings.Contains(alog.String(), "horus api: stopped") {
		t.Errorf("the log of horus api:\n%s", alog.String())
	}
}

// TestReadyWithoutTheDatabase: horus ready on the host without
// HORUS_DATABASE_URL names horus-db and horus-worker, and says where to run
// it. horus api refuses to start.
func TestReadyWithoutTheDatabase(t *testing.T) {
	t.Setenv("HORUS_DATABASE_URL", "")
	settings.Reset()
	stack, done := stackOf(context.Background(), "http://127.0.0.1:1")
	defer done()
	r := stack.Ready(context.Background(), jobdata.DependencyDB, jobdata.DependencyWorker)
	if r.Checks[0].Up || !strings.Contains(r.Checks[0].Detail, "docker exec horus-api horus ready") {
		t.Errorf("no HORUS_DATABASE_URL: %+v", r.Checks)
	}
	var out bytes.Buffer
	if err := runAPI(context.Background(), "127.0.0.1:0", &out); err == nil || !strings.Contains(err.Error(), "HORUS_DATABASE_URL is not set") {
		t.Errorf("horus api without HORUS_DATABASE_URL: %v", err)
	}
}
