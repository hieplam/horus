// The test of the Horus UI through the wiring of cmd/horus: horus api in
// this process on a test database with each fixture of
// internal/webui/uifixture. The goal row ui-matches-mockup
// (goals/2026-11-19-job-queue) runs it and reads its lines.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/cli"
	"github.com/hieplam/horus/internal/postgres"
	"github.com/hieplam/horus/internal/postgres/pgtest"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/webui"
	"github.com/hieplam/horus/internal/webui/uifixture"
)

// uiParts are the 11 parts of the reference docs/specs/2026-11-19-job-queue-ui.html.
var uiParts = []string{"ready-strip", "tile-waiting", "tile-running", "tile-succeeded", "tile-failed", "tile-workflows",
	"panel-running", "panel-queue", "panel-recent", "panel-worker", "panel-workflows"}

// TestHorusUI: for each fixture, horus api serves the Horus UI. The page has
// the 11 parts of the reference, and each count of the page and of the JSON
// summary equals the count that SQL reads from the test database. The page
// refuses a wrong Host, takes no write, and never shows the password of
// HORUS_DATABASE_URL. Each fixture prints one line:
//
//	ui-matches-mockup: fixture reference: 11 of 11 parts, 16 of 16 counts equal
//
// With HORUS_UI_SHOTS set to a folder, the test also takes a screenshot at
// 1920 x 1080 of the page of each fixture and of the reference, with a
// headless Edge or Chrome (HORUS_BROWSER, or the browser of this machine).
func TestHorusUI(t *testing.T) {
	pgtest.Need(t)
	for _, make := range []func(time.Time) uifixture.Fixture{uifixture.Reference, uifixture.Other} {
		f := make(time.Now())
		t.Run(f.Name, func(t *testing.T) { horusUI(t, f) })
	}
}

func horusUI(t *testing.T, f uifixture.Fixture) {
	ctx := context.Background()
	dbURL := pgtest.URL(t)
	db := pgtest.Open(t, dbURL)
	store := postgres.NewStore(db)
	if err := f.Load(ctx, store, postgres.NewBroker(db, postgres.BrokerOptions{})); err != nil {
		t.Fatal(err)
	}
	if f.WorkerUp {
		// The test is the worker: it holds the worker lock and writes the
		// heartbeat every 2 s.
		lock, err := store.WorkerLock(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(lock.Release)
		if err := store.Heartbeat(ctx, f.Worker); err != nil {
			t.Fatal(err)
		}
		bctx, stop := context.WithCancel(ctx)
		t.Cleanup(stop)
		go func() {
			tick := time.NewTicker(2 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-bctx.Done():
					return
				case <-tick.C:
					_ = store.Heartbeat(bctx, f.Worker)
				}
			}
		}()
	} else {
		stale := postgres.NewStore(db)
		stale.Now = func() time.Time { return time.Now().Add(-uifixture.StaleBeat) }
		if err := stale.Heartbeat(ctx, f.Worker); err != nil {
			t.Fatal(err)
		}
	}

	imageEnv(t) // the fake ComfyUI, the workflows of the repo
	addr := freeAddr(t)
	port := strings.Split(addr, ":")[1]
	t.Setenv("HORUS_DATABASE_URL", dbURL)
	t.Setenv("HORUS_API_PORT", port)
	settings.Reset()
	actx, stopAPI := context.WithCancel(ctx)
	var alog lockedBuffer
	apiDone := make(chan error, 1)
	go func() { apiDone <- runAPI(actx, addr, &alog) }()
	t.Cleanup(func() {
		stopAPI()
		select {
		case <-apiDone:
		case <-time.After(15 * time.Second):
			t.Errorf("horus api did not stop in 15 s:\n%s", alog.String())
		}
	})
	base := "http://" + addr
	eventually(t, "horus api answers", 15*time.Second, func() bool {
		resp, err := http.Get(base + "/livez")
		if err == nil {
			resp.Body.Close()
		}
		return err == nil && resp.StatusCode == 200
	})
	ready := 4
	if !f.WorkerUp {
		ready = 3
	}
	var v webui.View
	eventually(t, fmt.Sprintf("the readiness %d of 4", ready), 15*time.Second, func() bool {
		v = webui.View{}
		_ = json.Unmarshal([]byte(fetch(t, "GET", base+"/api/summary", "", 200)), &v)
		return v.Ready.Up == ready
	})
	html := fetch(t, "GET", base+"/", "", 200)
	body := fetch(t, "GET", base+"/api/summary", "", 200)
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("/api/summary: %v\n%s", err, body)
	}

	found := 0
	for _, p := range uiParts {
		if strings.Contains(html, `data-part="`+p+`"`) {
			found++
		} else {
			t.Errorf("the page has no part %s", p)
		}
	}

	sql := sqlOf(t, dbURL)
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tile := func(part string) string {
		m := regexp.MustCompile(`data-part="` + part + `"[^>]*><span class="n">([^<]*)</span>`).FindStringSubmatch(html)
		if m == nil {
			return "no tile"
		}
		return m[1]
	}
	slots := sql.ints(`SELECT slots FROM horus.workers ORDER BY heartbeat_at DESC NULLS LAST, id LIMIT 1`)
	running := sql.ints(`SELECT count(*) FROM horus.jobs WHERE state = 'running'`)
	succeeded := sql.ints(`SELECT count(*) FROM horus.jobs WHERE state = 'succeeded' AND finished_at >= $1`, midnight)
	failed := sql.ints(`SELECT count(*) FROM horus.jobs WHERE state = 'failed' AND finished_at >= $1`, midnight)
	queued := sql.ints(`SELECT count(*) FROM horus.jobs WHERE state = 'queued'`)
	same := sql.ints(`SELECT coalesce(sum(n), 0)::int FROM (SELECT count(*) AS n FROM horus.jobs
WHERE idempotency_key IS NOT NULL AND created_at >= $1 GROUP BY idempotency_key HAVING count(*) > 1) AS s`, midnight)
	messages := sql.ints(`SELECT count(*) FROM horus.messages WHERE queue = 'jobs.comfyui' AND visible_at <= now()`)
	today := map[string]int{}
	for _, row := range sql.rows(`SELECT workflow, count(*)::text FROM horus.jobs
WHERE state IN ('succeeded', 'failed') AND finished_at >= $1 GROUP BY workflow`, midnight) {
		n, _ := strconv.Atoi(row[1])
		today[row[0]] = n
	}
	flows, err := catalog.All()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := newSetup(cli.SetupPlace{}, seams{}).List(ctx, settings.WorkflowsDir())
	if err != nil {
		t.Fatal(err)
	}
	installed := 0
	for _, r := range rows {
		if r.State() == "installed" {
			installed++
		}
	}
	var viewToday []string
	var sqlToday []string
	for _, w := range v.Workflows {
		viewToday = append(viewToday, fmt.Sprintf("%s=%d", w.Name, w.Today))
		sqlToday = append(sqlToday, fmt.Sprintf("%s=%d", w.Name, today[w.Name]))
	}
	ids := func(list []string) string { return strings.Join(list, ",") }
	var queueIDs, runningIDs, recentIDs []string
	for _, r := range v.Queue {
		queueIDs = append(queueIDs, r.ID)
	}
	for _, r := range v.RunningJobs {
		runningIDs = append(runningIDs, r.ID)
	}
	for _, r := range v.Recent {
		recentIDs = append(recentIDs, r.ID)
	}
	worker := text(part(html, "panel-worker"))
	strip := text(part(html, "ready-strip"))
	checks := []struct {
		name      string
		got, want string
	}{
		{"tile-waiting on the page", tile("tile-waiting"), strconv.Itoa(queued)},
		{"waiting in the summary", strconv.Itoa(v.Waiting), strconv.Itoa(queued)},
		{"tile-running on the page", tile("tile-running"), fmt.Sprintf("%d / %d", running, slots)},
		{"running and slots in the summary", fmt.Sprintf("%d / %d", v.Running, v.Slots), fmt.Sprintf("%d / %d", running, slots)},
		{"tile-succeeded on the page", tile("tile-succeeded"), strconv.Itoa(succeeded)},
		{"succeeded today in the summary", strconv.Itoa(v.SucceededToday), strconv.Itoa(succeeded)},
		{"tile-failed on the page", tile("tile-failed"), strconv.Itoa(failed)},
		{"failed today in the summary", strconv.Itoa(v.FailedToday), strconv.Itoa(failed)},
		{"tile-workflows on the page", tile("tile-workflows"), strconv.Itoa(len(flows))},
		{"installed in the summary", strconv.Itoa(v.Installed), strconv.Itoa(installed)},
		{"the ready strip (the fixture, not SQL)", regexp.MustCompile(`Ready: \d+ of \d+`).FindString(strip), fmt.Sprintf("Ready: %d of 4", ready)},
		{"the running jobs", ids(runningIDs), ids(sql.strings(`SELECT id::text FROM horus.jobs WHERE state = 'running' ORDER BY started_at, id`))},
		{"the queue", ids(queueIDs), ids(sql.strings(`SELECT id::text FROM horus.jobs WHERE state = 'queued' ORDER BY created_at, id`))},
		{"the recent jobs", ids(recentIDs), ids(sql.strings(`SELECT id::text FROM horus.jobs
WHERE state IN ('succeeded', 'failed', 'cancelled', 'expired', 'abandoned') ORDER BY finished_at DESC, id DESC LIMIT 10`))},
		{"the messages in panel-worker", regexp.MustCompile(`· (\d+) messages?`).FindString(worker), fmt.Sprintf("· %d message%s", messages, plural(messages))},
		{"the same key in panel-worker", regexp.MustCompile(`Same key today (\d+) jobs?`).FindString(worker), fmt.Sprintf("Same key today %d job%s", same, plural(same))},
		{"the jobs of today in panel-workflows", ids(viewToday), ids(sqlToday)},
	}
	equal := 0
	for _, c := range checks {
		if c.got == c.want {
			equal++
		} else {
			t.Errorf("%s: horus api gives %q, the oracle %q", c.name, c.got, c.want)
		}
	}
	t.Logf("ui-matches-mockup: fixture %s: %d of %d parts, %d of %d counts equal", f.Name, found, len(uiParts), equal, len(checks))

	// The guard: a wrong Host gets 403, a write gets 405, and the password
	// of the database URL is in no answer.
	for _, path := range []string{"/", "/api/summary", "/api/events"} {
		fetch(t, "GET", base+path, "evil.example:"+port, 403)
		fetch(t, "POST", base+path, "", 405)
	}
	u, _ := url.Parse(dbURL)
	if pw, ok := u.User.Password(); ok && pw != "" && (strings.Contains(html, pw) || strings.Contains(body, pw)) {
		t.Error("the page or the summary shows the password of HORUS_DATABASE_URL")
	}

	if dir := os.Getenv("HORUS_UI_SHOTS"); dir != "" {
		shoot(t, base+"/", filepath.Join(dir, "horus-ui-"+f.Name+".png"))
		ref, _ := filepath.Abs(filepath.Join("..", "..", "docs", "specs", "2026-11-19-job-queue-ui.html"))
		shoot(t, "file:///"+filepath.ToSlash(ref), filepath.Join(dir, "reference.png"))
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// fetch sends one request and checks its status. host "" keeps the Host of
// the URL.
func fetch(t *testing.T, method, u, host string, status int) string {
	t.Helper()
	req, err := http.NewRequest(method, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatalf("%s %s: %v", method, u, err)
	}
	defer resp.Body.Close()
	var b []byte
	if !strings.HasSuffix(u, "/api/events") || resp.StatusCode != 200 {
		b, _ = io.ReadAll(resp.Body)
	}
	if resp.StatusCode != status {
		t.Errorf("%s %s with Host %q: %d, want %d", method, u, req.Host, resp.StatusCode, status)
	}
	return string(b)
}

// part gives the HTML of the element with the data-part name, up to the next
// part.
func part(html, name string) string {
	i := strings.Index(html, `data-part="`+name+`"`)
	if i < 0 {
		return ""
	}
	rest := html[i:]
	if j := strings.Index(rest[1:], `data-part="`); j >= 0 {
		rest = rest[:j+1]
	}
	if k := strings.Index(rest, ">"); k >= 0 {
		rest = rest[k+1:] // the rest of the opening tag
	}
	return rest
}

// text gives the text of an HTML fragment, with one space between words.
func text(html string) string {
	html = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, " ")
	return strings.Join(strings.Fields(html), " ")
}

// sqlReader runs the SQL of the oracle on its own connection: the counts do
// not come from the code of horus.
type sqlReader struct {
	t    *testing.T
	conn *pgx.Conn
}

func sqlOf(t *testing.T, u string) sqlReader {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), u)
	if err != nil {
		t.Fatalf("connect to the test database: %v", err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return sqlReader{t, conn}
}

func (s sqlReader) ints(q string, args ...any) int {
	s.t.Helper()
	var n int
	if err := s.conn.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		s.t.Fatalf("%s: %v", q, err)
	}
	return n
}

func (s sqlReader) strings(q string, args ...any) []string {
	var out []string
	for _, r := range s.rows(q, args...) {
		out = append(out, r[0])
	}
	return out
}

func (s sqlReader) rows(q string, args ...any) [][]string {
	s.t.Helper()
	rows, err := s.conn.Query(context.Background(), q, args...)
	if err != nil {
		s.t.Fatalf("%s: %v", q, err)
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			s.t.Fatal(err)
		}
		var row []string
		for _, v := range vals {
			row = append(row, fmt.Sprint(v))
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatal(err)
	}
	return out
}

// browser gives the path of a Chromium browser: HORUS_BROWSER, else Edge or
// Chrome at their usual place.
func browser() string {
	if b := os.Getenv("HORUS_BROWSER"); b != "" {
		return b
	}
	// The usual places on Windows, then on macOS. A place of another
	// system does not exist.
	var places []string
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
		if dir := os.Getenv(env); dir != "" {
			places = append(places, filepath.Join(dir, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(dir, "Google", "Chrome", "Application", "chrome.exe"))
		}
	}
	places = append(places, "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge")
	for _, p := range places {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "microsoft-edge"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// shoot takes a screenshot of the page at 1920 x 1080 into file, with a
// headless browser and a new profile in a temporary folder.
func shoot(t *testing.T, page, file string) {
	t.Helper()
	b := browser()
	if b == "" {
		t.Fatal("HORUS_UI_SHOTS is set, but no Edge or Chrome is on this machine: set HORUS_BROWSER")
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(file)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, b, "--headless=new", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
		"--user-data-dir="+t.TempDir(), "--window-size=1920,1080", "--force-device-scale-factor=1",
		"--screenshot="+abs, page)
	out, err := cmd.CombinedOutput()
	if _, serr := os.Stat(abs); err != nil || serr != nil {
		t.Fatalf("the screenshot of %s: %v %v\n%s", page, err, serr, out)
	}
	t.Logf("the screenshot of %s: %s", page, abs)
}
