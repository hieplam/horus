package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/brokertest"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/webui/uifixture"
)

// The tests of the page in the stack: horus api runs in a Linux container at
// UTC, the job rows hold container paths, and the browser of the owner is on
// the host.

// endedPNG inserts a succeeded job of it-illustration that ended at finished
// with the result path.
func endedPNG(t *testing.T, store *brokertest.Store, n byte, finished time.Time, path string) string {
	t.Helper()
	created := finished.Add(-5 * time.Minute)
	id, err := jobs.NewID(created, strings.NewReader(strings.Repeat(string(rune(n)), 10)))
	if err != nil {
		t.Fatal(err)
	}
	result, _ := json.Marshal(map[string]any{"workflow": uifixture.Workflow, "path": path})
	j := jobdata.Job{ID: id, Tool: "generate_image", Workflow: uifixture.Workflow, Backend: Backend, WorkQueue: jobs.WorkQueue(Backend),
		Payload: json.RawMessage(`{}`), State: jobdata.Succeeded, CreatedAt: created, StartedAt: created.Add(time.Minute),
		FinishedAt: finished, WaiterSeenAt: finished, Deadline: created.Add(jobs.DefaultQueueTimeout), Attempt: 1, Result: result}
	if err := store.Insert(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	return id
}

// stackHandler gives the handler of the page on the in-memory store at the
// time now, with the settings s.
func stackHandler(t *testing.T, store *brokertest.Store, s Settings, now time.Time) http.Handler {
	t.Helper()
	repoWorkflows(t)
	store.Now = func() time.Time { return now }
	q := jobs.New(store, brokertest.NewBroker(), jobs.Config{Now: func() time.Time { return now }})
	return Handler(Config{Ready: func(context.Context) jobdata.Readiness { return report() }, Queue: q,
		Setup: installed{uifixture.Workflow}, Ports: []int{8190}, Settings: s, Now: func() time.Time { return now }})
}

// rowOf gives the HTML of the row of the job id in the page.
func rowOf(html, id string) string {
	i := strings.Index(html, `<tr data-job="`+id+`"`)
	if i < 0 {
		return ""
	}
	rest := html[i:]
	if j := strings.Index(rest, "</tr>"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// recentPaths gives the path of each recent job of GET /api/summary, by job
// id.
func recentPaths(t *testing.T, h http.Handler) map[string]string {
	t.Helper()
	_, body := send(t, h, "GET", "127.0.0.1:8190", "/api/summary")
	var v struct {
		Recent []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"recent"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("/api/summary: %v\n%s", err, body)
	}
	out := map[string]string{}
	for _, r := range v.Recent {
		out[r.ID] = r.Path
	}
	return out
}

// TestRecentShowsTheHostPath (issue #264): in the stack, the job row holds
// the container path of a PNG. The panel "recent" and the summary show the
// host path of the pair of HORUS_SHARED_DIRS: the last 3 parts in the cell,
// and the whole path in the hover text. A path in no shared folder stays as
// it is, and so does each path when horus has no shared folder.
func TestRecentShowsTheHostPath(t *testing.T) {
	now := time.Date(2026, 11, 20, 19, 40, 0, 0, time.UTC)
	const inside, outside = "/shared/pictures/a/cover.png", "/tmp/outputs/b/c/d/x.png"
	const host = `C:\Users\you\Pictures\horus\a\cover.png`
	pair := jobs.PathsOf([]settings.SharedDir{{Host: `C:\Users\you\Pictures\horus`, Container: "/shared/pictures"}}, "windows")
	for _, c := range []struct {
		name        string
		paths       jobs.Paths
		title, cell string
	}{
		{"the stack with a shared folder", pair, host, `…\horus\a\cover.png`},
		{"no shared folder", jobs.Paths{}, inside, "…/pictures/a/cover.png"},
	} {
		store := brokertest.NewStore()
		in := endedPNG(t, store, 1, now.Add(-3*time.Minute), inside)
		out := endedPNG(t, store, 2, now.Add(-9*time.Minute), outside)
		s := settingsOf()
		s.Paths = c.paths
		h := stackHandler(t, store, s, now)
		_, html := send(t, h, "GET", "127.0.0.1:8190", "/")
		for _, w := range []struct{ id, title, cell string }{{in, c.title, c.cell}, {out, outside, "…/c/d/x.png"}} {
			row := rowOf(html, w.id)
			if want := `<span class="mono" title="` + w.title + `">` + w.cell + `</span>`; !strings.Contains(row, want) {
				t.Errorf("%s: the row of the PNG:\n got  %s\n want it to contain %s", c.name, row, want)
			}
		}
		if got := recentPaths(t, h); got[in] != c.title || got[out] != outside {
			t.Errorf("%s: the paths of /api/summary: %q and %q, want %q and %q", c.name, got[in], got[out], c.title, outside)
		}
	}
}

// TestTimesOfTheBrowserZone (issue #265): horus api runs at UTC in the
// stack, and the browser of the owner is at UTC+7. With the time zone of
// the browser in the query, the page shows each clock time in that zone, and
// "today" starts at 00:00 of that zone: a PNG that ended at 23:30 of the day
// before is not a PNG of today. Without a time zone, or with one that horus
// does not know, the page shows the times of the process and names its zone
// next to each time.
func TestTimesOfTheBrowserZone(t *testing.T) {
	now := time.Date(2026, 11, 20, 19, 40, 0, 0, time.UTC)
	store := brokertest.NewStore()
	late := endedPNG(t, store, 1, time.Date(2026, 11, 20, 19, 37, 0, 0, time.UTC), "/shared/outputs/1.png")
	early := endedPNG(t, store, 2, time.Date(2026, 11, 20, 16, 30, 0, 0, time.UTC), "/shared/outputs/2.png")
	h := stackHandler(t, store, settingsOf(), now)
	if err := store.Heartbeat(context.Background(), jobdata.Worker{ID: "w", Slots: 1, StartedAt: time.Date(2026, 11, 20, 19, 25, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		query                    string
		late, early, since, tile string
		today                    int
	}{
		{"?tz=Asia/Ho_Chi_Minh", "02:37", "Nov 20 23:30", "running since 02:25", "1 succeeded today", 1},
		{"?tz=America/St_Johns", "16:07", "13:00", "running since 15:55", "2 succeeded today", 2},
		{"", "19:37 UTC", "16:30 UTC", "running since 19:25 UTC", "2 succeeded today", 2},
		{"?tz=Mars/Olympus", "19:37 UTC", "16:30 UTC", "running since 19:25 UTC", "2 succeeded today", 2},
	} {
		_, html := send(t, h, "GET", "127.0.0.1:8190", "/"+c.query)
		for id, want := range map[string]string{late: c.late, early: c.early} {
			if got := text(rowOf(html, id)); !strings.HasPrefix(got, want+" "+uifixture.Workflow) {
				t.Errorf("GET /%s: the row of the PNG is %q, want the end time %q", c.query, got, want)
			}
		}
		if got := text(partOf(html, "tile-succeeded")); !strings.Contains(got, c.tile) {
			t.Errorf("GET /%s: the tile: %q, want %q", c.query, got, c.tile)
		}
		if got := text(partOf(html, "panel-worker")); !strings.Contains(got, "State "+c.since+" Slots") {
			t.Errorf("GET /%s: the panel worker: %q, want %q", c.query, got, c.since)
		}
		if got, want := text(partOf(html, "panel-workflows")), fmt.Sprintf("%s ComfyUI yes %d", uifixture.Workflow, c.today); !strings.Contains(got, want) {
			t.Errorf("GET /%s: the panel workflows: %q, want %q", c.query, got, want)
		}
		_, body := send(t, h, "GET", "127.0.0.1:8190", "/api/summary"+c.query)
		var v struct {
			SucceededToday int `json:"succeeded_today"`
		}
		if err := json.Unmarshal([]byte(body), &v); err != nil || v.SucceededToday != c.today {
			t.Errorf("GET /api/summary%s: succeeded_today %d (%v), want %d", c.query, v.SucceededToday, err, c.today)
		}
	}
}
