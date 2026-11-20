// Tests of the readiness of the stack through the wiring of cmd/horus, with
// no Docker: horus worker and horus api in this process on a test database,
// and the fake ComfyUI of comfytest. The goal rows stack-starts-together and
// ready-when-four-up run the same parts in containers.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
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
	if !strings.Contains(alog.String(), "horus api: serves /mcp, the Horus UI, /readyz and /livez on http://"+addr) || !strings.Contains(alog.String(), "horus api: stopped") {
		t.Errorf("the log of horus api:\n%s", alog.String())
	}
}

// TestAPIServesMCP: horus api serves /mcp with the token of HORUS_API_TOKEN.
// initialize with the token gets 200 and a session, without it 401. The log
// says that /mcp needs the token, and never has the token. Without
// HORUS_API_TOKEN, the log says why /mcp refuses each request.
func TestAPIServesMCP(t *testing.T) {
	pgtest.Need(t)
	url := pgtest.URL(t)
	imageEnv(t)
	for _, token := range []string{rand.Text(), ""} {
		addr := freeAddr(t)
		t.Setenv("HORUS_DATABASE_URL", url)
		t.Setenv("HORUS_API_PORT", strings.Split(addr, ":")[1])
		t.Setenv("HORUS_API_TOKEN", token)
		settings.Reset()
		wctx, stopWorker := context.WithCancel(context.Background())
		workerDone := make(chan error, 1)
		go func() { workerDone <- runWorker(wctx, io.Discard) }()
		actx, stopAPI := context.WithCancel(context.Background())
		var alog lockedBuffer
		apiDone := make(chan error, 1)
		go func() { apiDone <- runAPI(actx, addr, &alog) }()

		initialize := func(bearer string) (int, string) {
			body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`
			req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/mcp", strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			if bearer != "" {
				req.Header.Set("Authorization", "Bearer "+bearer)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return 0, err.Error()
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			return resp.StatusCode, resp.Header.Get("Mcp-Session-Id") + " " + string(b)
		}
		if token != "" {
			eventually(t, "/mcp answers initialize with 200", 15*time.Second, func() bool {
				code, _ := initialize(token)
				return code == http.StatusOK
			})
			if code, body := initialize(token); code != http.StatusOK || !strings.Contains(body, `"name":"horus"`) {
				t.Errorf("initialize with the token: %d %s", code, body)
			}
			if code, _ := initialize(""); code != http.StatusUnauthorized {
				t.Errorf("initialize without the token: %d, want 401", code)
			}
		} else {
			eventually(t, "horus api serves", 15*time.Second, func() bool {
				code, _ := initialize("")
				return code != 0
			})
			if code, body := initialize(rand.Text()); code != http.StatusUnauthorized || !strings.Contains(body, "HORUS_API_TOKEN is not set") {
				t.Errorf("initialize without HORUS_API_TOKEN on the server: %d %s", code, body)
			}
		}
		stopAPI()
		<-apiDone
		stopWorker()
		<-workerDone
		log := alog.String()
		wantLine := "horus api: /mcp needs the bearer token of HORUS_API_TOKEN"
		if token == "" {
			wantLine = "horus api: HORUS_API_TOKEN is not set, so /mcp refuses each request with 401"
		}
		if !strings.Contains(log, wantLine) {
			t.Errorf("the log of horus api has no line %q:\n%s", wantLine, log)
		}
		if token != "" && strings.Contains(log, token) {
			t.Error("the log of horus api has the token")
		}
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
