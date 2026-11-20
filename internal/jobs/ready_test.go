package jobs_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/brokertest"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
)

// fakeHealth is the database part of the readiness checks.
type fakeHealth struct {
	schema error
	held   bool
	lock   error
}

func (f *fakeHealth) CheckSchema(context.Context) error { return f.schema }
func (f *fakeHealth) WorkerLockHeld(context.Context) (bool, error) {
	return f.held, f.lock
}

var readyNow = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func readyEnv(t *testing.T, h *fakeHealth, heartbeat time.Time) *jobs.Service {
	t.Helper()
	st := brokertest.NewStore()
	st.Now = func() time.Time { return heartbeat }
	if !heartbeat.IsZero() {
		if err := st.Heartbeat(context.Background(), jobdata.Worker{ID: "w1", Slots: 1}); err != nil {
			t.Fatal(err)
		}
	}
	svc := jobs.New(st, brokertest.NewBroker(), jobs.Config{Now: func() time.Time { return readyNow }})
	if h != nil {
		svc.WithHealth(h)
	}
	return svc
}

// TestChecks: each cause of a failed check of horus-db or horus-worker gives
// a result that is not up, with the cause in its detail. Only a fresh
// heartbeat and a held lock make the worker up.
func TestChecks(t *testing.T) {
	fresh := readyNow.Add(-5 * time.Second)
	for _, c := range []struct {
		name      string
		health    *fakeHealth
		heartbeat time.Time
		dbUp      bool
		workerUp  bool
		detail    string // a part of the detail of the result that is down
	}{
		{"all up", &fakeHealth{held: true}, fresh, true, true, ""},
		{"no wiring", nil, fresh, false, false, "no database check"},
		{"database down", &fakeHealth{schema: errors.New("dial tcp: connection refused"), held: true}, fresh, false, false, "connection refused"},
		{"no worker row", &fakeHealth{held: true}, time.Time{}, true, false, "no worker row"},
		{"old heartbeat", &fakeHealth{held: true}, readyNow.Add(-16 * time.Second), true, false, "16s old"},
		{"no lock", &fakeHealth{held: false}, fresh, true, false, "no worker holds the worker lock"},
		{"lock read fails", &fakeHealth{lock: errors.New("read the worker lock: boom")}, fresh, true, false, "boom"},
	} {
		got := readyEnv(t, c.health, c.heartbeat).Checks(context.Background())
		if len(got) != 2 || got[0].Dependency != jobdata.DependencyDB || got[1].Dependency != jobdata.DependencyWorker {
			t.Fatalf("%s: the results %+v, want horus-db then horus-worker", c.name, got)
		}
		if got[0].Up != c.dbUp || got[1].Up != c.workerUp {
			t.Errorf("%s: horus-db up %v, horus-worker up %v, want %v and %v (%+v)", c.name, got[0].Up, got[1].Up, c.dbUp, c.workerUp, got)
		}
		if c.detail != "" && !strings.Contains(got[0].Detail+got[1].Detail, c.detail) {
			t.Errorf("%s: the details %q and %q do not name %q", c.name, got[0].Detail, got[1].Detail, c.detail)
		}
	}
}

// TestWorkerCheckLimit: a heartbeat exactly WorkerFresh old is up, one
// second older is down.
func TestWorkerCheckLimit(t *testing.T) {
	w := &jobdata.Worker{ID: "w1", HeartbeatAt: readyNow.Add(-jobs.WorkerFresh)}
	if c := jobs.WorkerCheck(w, true, readyNow); !c.Up {
		t.Errorf("a heartbeat %s old: %+v, want up", jobs.WorkerFresh, c)
	}
	if c := jobs.WorkerCheck(w, true, readyNow.Add(time.Second)); c.Up {
		t.Errorf("a heartbeat %s old: %+v, want down", jobs.WorkerFresh+time.Second, c)
	}
}
