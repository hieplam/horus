package webui

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/brokertest"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/webui/uifixture"
)

func report(down ...string) jobdata.Readiness {
	var rs []jobdata.CheckResult
	for _, d := range jobdata.Dependencies {
		rs = append(rs, jobdata.CheckResult{Dependency: d, Up: !slices.Contains(down, d), Detail: "detail of " + d})
	}
	return jobdata.ReadinessOf(rs)
}

// The parts of the reference docs/specs/2026-11-19-job-queue-ui.html, in the
// order of the page.
var parts = []string{"ready-strip", "tile-waiting", "tile-running", "tile-succeeded", "tile-failed", "tile-workflows",
	"panel-running", "panel-queue", "panel-recent", "panel-worker", "panel-workflows"}

// password is the password of the database URL of the tests: a new random
// value at each run, so that no password is in the code. No page and no JSON
// may show it.
var password = func() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "t" + hex.EncodeToString(b[:])
}()

// at is the time of the fixtures of the tests: 10:44:00 on a day.
var at = time.Date(2026, 11, 19, 10, 44, 0, 0, time.Local)

func send(t *testing.T, h http.Handler, method, host, path string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(method, "http://"+host+path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	resp := rec.Result()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

// loaded gives the job queue service on the in-memory store and broker with
// the fixture f, and its worker row, at the clock of f.
func loaded(t *testing.T, f uifixture.Fixture) (*jobs.Service, *brokertest.Broker) {
	t.Helper()
	store, broker := brokertest.NewStore(), brokertest.NewBroker()
	store.Now = func() time.Time { return f.Now }
	ctx := context.Background()
	if err := f.Load(ctx, store, broker); err != nil {
		t.Fatal(err)
	}
	if err := store.Heartbeat(ctx, f.Worker); err != nil {
		t.Fatal(err)
	}
	return jobs.New(store, broker, jobs.Config{Now: func() time.Time { return f.Now }}), broker
}

func settingsOf() Settings {
	return Settings{DatabaseURL: "postgres://horus:" + password + "@horus-db:5432/horus?sslmode=disable",
		ComfyURL: "http://host.docker.internal:8188", Slots: 1, RunLimit: 1800 * time.Second, Broker: "Postgres"}
}

// installed is a setup service in which the named workflows are installed.
type installed []string

func (in installed) List(context.Context, string) ([]provision.Row, error) {
	var rows []provision.Row
	for _, n := range in {
		rows = append(rows, provision.Row{Name: n, Files: 1, Present: 1})
	}
	return rows, nil
}

// repoWorkflows makes the page list the workflows of the repo.
func repoWorkflows(t *testing.T) {
	t.Helper()
	dir, _ := filepath.Abs(filepath.Join("..", "..", "workflows"))
	t.Setenv("HORUS_WORKFLOWS_DIR", dir)
	settings.Reset()
	t.Cleanup(settings.Reset)
}

func handlerFor(t *testing.T, f uifixture.Fixture, ready jobdata.Readiness) http.Handler {
	t.Helper()
	repoWorkflows(t)
	q, _ := loaded(t, f)
	return Handler(Config{Ready: func(context.Context) jobdata.Readiness { return ready }, Queue: q,
		Setup:        installed{"it-illustration"},
		ComfyVersion: func(context.Context) (string, error) { return "0.44.0", nil },
		Ports:        []int{8190}, Settings: settingsOf(), Now: func() time.Time { return f.Now }})
}

// TestReadyz: /readyz answers 200 with the report when the 4 dependencies are
// up, and 503 with the same report when one is down. It runs the checks at
// each request, and it answers to any Host: the health check of the
// container asks it.
func TestReadyz(t *testing.T) {
	var down []string
	calls := 0
	h := Handler(Config{Ready: func(context.Context) jobdata.Readiness { calls++; return report(down...) }})
	for _, c := range []struct {
		down []string
		code int
	}{{nil, 200}, {[]string{jobdata.DependencyComfyUI}, 503}, {[]string{jobdata.DependencyDB, jobdata.DependencyWorker}, 503}, {nil, 200}} {
		down = c.down
		resp, body := send(t, h, "GET", "horus-api:8190", "/readyz")
		if resp.StatusCode != c.code || resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("down %v: status %d %q, want %d application/json", c.down, resp.StatusCode, resp.Header.Get("Content-Type"), c.code)
		}
		var got jobdata.Readiness
		if err := json.Unmarshal([]byte(body), &got); err != nil {
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
	h := Handler(Config{Ready: func(context.Context) jobdata.Readiness { t.Fatal("/livez ran the checks"); return jobdata.Readiness{} }})
	if resp, _ := send(t, h, "GET", "127.0.0.1:8190", "/livez"); resp.StatusCode != 200 {
		t.Errorf("/livez: %d", resp.StatusCode)
	}
	for _, p := range []string{"/nope", "/"} {
		if resp, _ := send(t, h, "GET", "127.0.0.1:8190", p); resp.StatusCode != 404 {
			t.Errorf("%s without a job queue: %d, want 404", p, resp.StatusCode)
		}
	}
}

// partOf gives the HTML of the element with the data-part name, up to the
// next part.
func partOf(html, name string) string {
	i := strings.Index(html, `data-part="`+name+`"`)
	if i < 0 {
		return ""
	}
	rest := html[i:]
	if j := strings.Index(rest[1:], `data-part="`); j >= 0 {
		return rest[:j+1]
	}
	return rest
}

// text gives the text of an HTML fragment: the tags go, and each run of
// space becomes one space.
func text(html string) string {
	html = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, " ")
	html = strings.NewReplacer("&#43;", "+", "&#39;", "'", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&#34;", `"`).Replace(html)
	return strings.Join(strings.Fields(html), " ")
}

// TestPageOfTheReference: the page of the example data of the reference has
// the 11 parts in their order, and each part shows the texts of the
// reference.
func TestPageOfTheReference(t *testing.T) {
	h := handlerFor(t, uifixture.Reference(at), report())
	resp, html := send(t, h, "GET", "127.0.0.1:8190", "/")
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("GET /: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	last := -1
	for _, p := range parts {
		i := strings.Index(html, `data-part="`+p+`"`)
		if i < 0 {
			t.Errorf("the page has no part %s", p)
			continue
		}
		if i < last {
			t.Errorf("the part %s is not in the order of the reference", p)
		}
		last = i
	}
	if n := strings.Count(html, "data-part="); n != len(parts) {
		t.Errorf("the page has %d parts, want %d", n, len(parts))
	}
	want := map[string][]string{
		"ready-strip":     {"Ready: 4 of 4 horus-db up horus-worker up horus-api up ComfyUI up"},
		"tile-waiting":    {"3 waiting in the queue"},
		"tile-running":    {"1 / 1 running / slots"},
		"tile-succeeded":  {"14 succeeded today"},
		"tile-failed":     {"1 failed today"},
		"tile-workflows":  {"2 workflows, 1 installed"},
		"panel-running":   {"Running slot 1 of 1 it-illustration job ", " · Claude Code · folder horus A flat vector illustration", "step 9 of 15 · running 2 min 31 s · waited 0 s run limit in 27 min"},
		"panel-queue":     {"1 it-illustration Codex · folder blog An isometric diagram of a Kubernetes cluster with three nodes 2:10 1:30", "2 it-illustration Claude Code · folder docs A line drawing of a database with a clock, muted colours 1:05 5:30", "3 it-illustration Claude Code · folder horus A poster of a falcon eye made of circuit lines 0:12 9:30"},
		"panel-recent":    {"Recent last 10 Ended Workflow Result Queue Run 10:42 it-illustration PNG …\\outputs\\it-illustration\\20261119T", "10:31 it-illustration OUT_OF_MEMORY 1536 × 1536 0:40 1:55", "10:20 it-illustration abandoned no waiter for 60 s 6:30 –"},
		"panel-worker":    {"State running since 08:04", "Slots 1 ( HORUS_WORKER_CONCURRENCY )", "Models in memory it-illustration (resource key 9f2c…)", "Broker Postgres · work queue jobs.comfyui · 3 messages", "Job store horus-db / horus", "ComfyUI host.docker.internal:8188 · v0.44.0", "Same key today 2 jobs (dedupe is off)"},
		"panel-workflows": {"it-illustration ComfyUI yes 15", "vi-speech speech worker no 0"},
	}
	for p, lines := range want {
		got := text(partOf(html, p))
		for _, w := range lines {
			if !strings.Contains(got, w) {
				t.Errorf("the part %s:\n got  %q\n want it to contain %q", p, got, w)
			}
		}
	}
	if !strings.Contains(partOf(html, "panel-running"), `style="width:60%"`) {
		t.Errorf("the bar of step 9 of 15 is not 60%% wide:\n%s", partOf(html, "panel-running"))
	}
	if strings.Count(partOf(html, "panel-recent"), "<tr data-job=") != 10 {
		t.Errorf("the part panel-recent has not 10 rows:\n%s", text(partOf(html, "panel-recent")))
	}
	if strings.Contains(html, password) {
		t.Error("the page shows the password of HORUS_DATABASE_URL")
	}
}

// TestSummaryOfBothFixtures: the JSON summary gives the counts of each
// fixture. A page with the numbers of one fixture fails the other.
func TestSummaryOfBothFixtures(t *testing.T) {
	for _, c := range []struct {
		f                                         uifixture.Fixture
		waiting, running, slots, succ, fail, same int
		messages, queue, recent, today, installed int
	}{
		{uifixture.Reference(at), 3, 1, 1, 14, 1, 2, 3, 3, 10, 15, 1},
		{uifixture.Other(at), 5, 2, 2, 3, 2, 3, 5, 5, 9, 4, 1},
	} {
		h := handlerFor(t, c.f, report())
		resp, body := send(t, h, "GET", "localhost:8190", "/api/summary")
		if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("%s: GET /api/summary: %d %s", c.f.Name, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		var v View
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Fatalf("%s: %v\n%s", c.f.Name, err, body)
		}
		got := []int{v.Waiting, v.Running, v.Slots, v.SucceededToday, v.FailedToday, v.SameKeyToday, v.Worker.Messages,
			len(v.Queue), len(v.Recent), v.Workflows[0].Today, v.Installed}
		want := []int{c.waiting, c.running, c.slots, c.succ, c.fail, c.same, c.messages, c.queue, c.recent, c.today, c.installed}
		if !slices.Equal(got, want) {
			t.Errorf("%s: waiting, running, slots, succeeded, failed, same key, messages, queue, recent, today, installed:\n got  %v\n want %v",
				c.f.Name, got, want)
		}
		if strings.Contains(body, password) {
			t.Errorf("%s: the summary shows the password of HORUS_DATABASE_URL", c.f.Name)
		}
	}
}

// TestPageOfTheOtherFixture: the readiness strip names the dependency that
// is down, and the parts show the other numbers.
func TestPageOfTheOtherFixture(t *testing.T) {
	ready := report(jobdata.DependencyWorker)
	ready.Checks[2].Detail = "the last heartbeat of the worker is 1m0s old, the database postgres://horus:" + password + "@horus-db"
	h := handlerFor(t, uifixture.Other(at), ready)
	_, html := send(t, h, "GET", "127.0.0.1:8190", "/")
	for p, w := range map[string]string{
		"ready-strip":     "Ready: 3 of 4 horus-db up horus-worker down horus-api up ComfyUI up",
		"tile-waiting":    "5 waiting in the queue",
		"tile-running":    "2 / 2 running / slots",
		"tile-succeeded":  "3 succeeded today",
		"tile-failed":     "2 failed today",
		"tile-workflows":  "2 workflows, 1 installed",
		"panel-queue":     "5 it-illustration Claude Code · folder docs A photo-style render of a fibre cable 1:40 10:20",
		"panel-running":   "Running slots 1 to 2 of 2",
		"panel-worker":    "State down: the last heartbeat of the worker is 1m0s old, the database postgres://horus:***@horus-db",
		"panel-recent":    "INTERNAL the speech worker stopped",
		"panel-workflows": "vi-speech speech worker no 1",
	} {
		if got := text(partOf(html, p)); !strings.Contains(got, w) {
			t.Errorf("the part %s:\n got  %q\n want it to contain %q", p, got, w)
		}
	}
	for _, w := range []string{"cancelled the agent cancelled the call", "expired waited past its deadline", "BACKEND_UNAVAILABLE ComfyUI does not answer", "Nov 18 22:00"} {
		if !strings.Contains(text(partOf(html, "panel-recent")), w) {
			t.Errorf("the part panel-recent has no %q:\n%s", w, text(partOf(html, "panel-recent")))
		}
	}
	if strings.Contains(html, password) {
		t.Error("the page shows the password of HORUS_DATABASE_URL")
	}
}

// TestHostGuard: the page, the summary and the events refuse a wrong Host
// with 403. 127.0.0.1 and localhost with a port of Ports pass. /readyz and
// /livez answer to any Host.
func TestHostGuard(t *testing.T) {
	h := handlerFor(t, uifixture.Reference(at), report())
	for _, path := range []string{"/", "/api/summary", "/api/events"} {
		for _, host := range []string{"evil.example:8190", "127.0.0.1:9999", "localhost", "127.0.0.1", "[::1]:8190", "horus-api:8190"} {
			if resp, _ := send(t, h, "GET", host, path); resp.StatusCode != 403 {
				t.Errorf("%s with Host %s: %d, want 403", path, host, resp.StatusCode)
			}
		}
	}
	for _, host := range []string{"127.0.0.1:8190", "localhost:8190"} {
		if resp, _ := send(t, h, "GET", host, "/"); resp.StatusCode != 200 {
			t.Errorf("/ with Host %s: %d, want 200", host, resp.StatusCode)
		}
	}
	for _, path := range []string{"/readyz", "/livez"} {
		if resp, _ := send(t, h, "GET", "horus-api:8190", path); resp.StatusCode != 200 {
			t.Errorf("%s with Host horus-api:8190: %d, want 200", path, resp.StatusCode)
		}
	}
}

// TestReadOnly: no route takes a write.
func TestReadOnly(t *testing.T) {
	h := handlerFor(t, uifixture.Reference(at), report())
	for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, path := range []string{"/", "/api/summary", "/api/events", "/readyz"} {
			if resp, _ := send(t, h, m, "127.0.0.1:8190", path); resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: %d, want 405", m, path, resp.StatusCode)
			}
		}
	}
}

// TestEvents: the stream gives one event "job" for each event of the broker.
func TestEvents(t *testing.T) {
	f := uifixture.Reference(at)
	q, broker := loaded(t, f)
	h := Handler(Config{Ready: func(context.Context) jobdata.Readiness { return report() }, Queue: q, Ports: []int{8190}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = "127.0.0.1:8190"
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("GET /api/events: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	lines := bufio.NewScanner(resp.Body)
	if !lines.Scan() || lines.Text() != "retry: 5000" {
		t.Fatalf("the first line of the stream: %q", lines.Text())
	}
	e := jobdata.Event{V: 1, JobID: f.Jobs[0].ID, Type: jobdata.EventProgress, At: at.UTC()}
	go func() {
		// The stream listens before it sends its first line.
		_ = broker.Notify(context.Background(), e)
	}()
	var got []string
	for lines.Scan() && len(got) < 2 {
		if l := lines.Text(); l != "" {
			got = append(got, l)
		}
	}
	data, _ := json.Marshal(e)
	if want := []string{"event: job", "data: " + string(data)}; !slices.Equal(got, want) {
		t.Errorf("the stream: %q, want %q", got, want)
	}
}

// TestBuildWithoutData: a job store that does not answer gives 0 in each
// count, the cause in the errors, and no password.
func TestBuildWithoutData(t *testing.T) {
	r := Reads{Ready: report(jobdata.DependencyDB, jobdata.DependencyWorker),
		SummaryErr: errorText("connect to postgres://horus:" + password + "@horus-db: refused"), DepthErr: errorText("down")}
	v := Build(r, settingsOf(), at)
	if v.Waiting != 0 || v.Running != 0 || len(v.Queue) != 0 || len(v.Errors) != 2 {
		t.Errorf("the view without a job store: %+v", v)
	}
	if strings.Contains(strings.Join(v.Errors, " "), password) || !strings.Contains(v.Errors[0], "***") {
		t.Errorf("the errors: %q", v.Errors)
	}
	if v.Worker.Broker != "Postgres · work queue jobs.comfyui · no answer" {
		t.Errorf("the broker: %q", v.Worker.Broker)
	}
}

type errorText string

func (e errorText) Error() string { return string(e) }

// TestFormats: the texts of the times.
func TestFormats(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{clock(0), "0:00"}, {clock(130 * time.Second), "2:10"}, {clock(3725 * time.Second), "1:02:05"},
		{words(31 * time.Second), "31 s"}, {words(151 * time.Second), "2 min 31 s"}, {words(3900 * time.Second), "1 h 5 min"},
		{clock(roundTo10s(89 * time.Second)), "1:30"}, {clock(roundTo10s(84 * time.Second)), "1:20"},
		{shortPath(`C:\a\outputs\it-illustration\x.png`), `…\outputs\it-illustration\x.png`},
		{shortPath("/shared/out/x.png"), "/shared/out/x.png"},
		{storeName("postgres://horus:" + password + "@horus-db:5432/horus?sslmode=disable"), "horus-db / horus"},
		{storeName("postgres://u:" + password + "@127.0.0.1:55433/horus_x_test"), "127.0.0.1:55433 / horus_x_test"},
		{when(at.Add(-time.Hour), at), "09:44"}, {when(at.Add(-24*time.Hour), at), "Nov 18 10:44"},
		{slotText(0, 1), "0 of 1 slots"}, {slotText(1, 3), "slot 1 of 3"}, {slotText(3, 3), "slots 1 to 3 of 3"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
	if got := Ports(8190, "0.0.0.0:8190"); !slices.Equal(got, []int{8190}) {
		t.Errorf("Ports: %v", got)
	}
	if got := Ports(18190, "127.0.0.1:18191"); !slices.Equal(got, []int{18190, 18191}) {
		t.Errorf("Ports: %v", got)
	}
}
