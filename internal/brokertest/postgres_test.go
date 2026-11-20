package brokertest

import (
	"context"
	"testing"
	"time"

	"github.com/hieplam/horus/internal/jobs"
	"github.com/hieplam/horus/internal/postgres"
	"github.com/hieplam/horus/internal/postgres/pgtest"
)

// testVisibility is the visibility time of the Postgres broker in its test.
// It is shorter than Hold, so that the check long_delivery runs longer than it
// and proves the extension. The program uses postgres.DefaultVisibility.
const testVisibility = time.Second

// TestPostgresBroker runs the contract suite on the Postgres broker of
// internal/postgres. Each check gets its own database. The goal row
// broker-contract reads its subtests. Without HORUS_TEST_DATABASE_URL it
// skips, and the goal row fails.
func TestPostgresBroker(t *testing.T) {
	pgtest.Need(t)
	if testVisibility >= Hold {
		t.Fatalf("the visibility time %s must be shorter than Hold %s", testVisibility, Hold)
	}
	Run(t, func(t testing.TB) jobs.Broker {
		return postgres.NewBroker(pgtest.DB(t), postgres.BrokerOptions{Visibility: testVisibility})
	})
}

// TestPostgresStore runs the store contract on the job store of
// internal/postgres. Each check gets its own database. The end of the session
// of the worker lock is pg_terminate_backend in that database.
func TestPostgresStore(t *testing.T) {
	pgtest.Need(t)
	RunStore(t, func(t testing.TB, now func() time.Time) (jobs.Store, func(string) []string, func()) {
		url := pgtest.URL(t)
		s := postgres.NewStore(pgtest.Open(t, url))
		s.Now = now
		endLock := func() {
			if n := pgtest.EndWorkerLock(t, url); n != 1 {
				t.Fatalf("the end of the session of the worker lock: %d sessions ended, want 1", n)
			}
		}
		return s, func(id string) []string {
			evs, err := s.Events(context.Background(), id)
			if err != nil {
				t.Fatalf("Events %s: %v", id, err)
			}
			var types []string
			for _, e := range evs {
				types = append(types, e.Type)
			}
			return types
		}, endLock
	})
}
