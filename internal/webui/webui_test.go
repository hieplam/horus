package webui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/hieplam/horus/internal/jobdata"
)

func report(down ...string) jobdata.Readiness {
	var rs []jobdata.CheckResult
	for _, d := range jobdata.Dependencies {
		rs = append(rs, jobdata.CheckResult{Dependency: d, Up: !slices.Contains(down, d), Detail: "detail of " + d})
	}
	return jobdata.ReadinessOf(rs)
}

func get(t *testing.T, h http.Handler, path string) (*http.Response, []byte) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

// TestReadyz: /readyz answers 200 with the report when the 4 dependencies are
// up, and 503 with the same report when one is down. It runs the checks at
// each request.
func TestReadyz(t *testing.T) {
	var down []string
	calls := 0
	h := Handler(func(context.Context) jobdata.Readiness { calls++; return report(down...) })
	for _, c := range []struct {
		down []string
		code int
	}{{nil, 200}, {[]string{jobdata.DependencyComfyUI}, 503}, {[]string{jobdata.DependencyDB, jobdata.DependencyWorker}, 503}, {nil, 200}} {
		down = c.down
		resp, body := get(t, h, "/readyz")
		if resp.StatusCode != c.code || resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("down %v: status %d %q, want %d application/json", c.down, resp.StatusCode, resp.Header.Get("Content-Type"), c.code)
		}
		var got jobdata.Readiness
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("down %v: the body is not a report: %v\n%s", c.down, err, body)
		}
		if got.Ready != (len(c.down) == 0) || !slices.Equal(got.Down, append([]string{}, c.down...)) || len(got.Checks) != 4 {
			t.Errorf("down %v: the report %+v", c.down, got)
		}
	}
	if calls != 4 {
		t.Errorf("the checks ran %d times for 4 requests", calls)
	}
}

// TestLivez: /livez answers 200 and runs no check. Another path is 404.
func TestLivez(t *testing.T) {
	h := Handler(func(context.Context) jobdata.Readiness { t.Fatal("/livez ran the checks"); return jobdata.Readiness{} })
	if resp, _ := get(t, h, "/livez"); resp.StatusCode != 200 {
		t.Errorf("/livez: %d", resp.StatusCode)
	}
	if resp, _ := get(t, h, "/nope"); resp.StatusCode != 404 {
		t.Errorf("/nope: %d", resp.StatusCode)
	}
}
