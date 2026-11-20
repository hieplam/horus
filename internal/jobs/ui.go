package jobs

import (
	"context"

	"github.com/hieplam/horus/internal/jobdata"
)

// The reads of the Horus UI (internal/webui, ADR 0028). The page also reads
// Summary.

// Depth gives the count of the messages in the work queue of the backend that
// wait for a delivery. A message that the worker holds does not count.
func (s *Service) Depth(ctx context.Context, backend string) (int, error) {
	return s.broker.Depth(ctx, WorkQueue(backend))
}

// Listen gives the events of the jobs from now on: a signal that a job
// changed (ADR 0031). A listener reads the job store for the facts. The
// channel closes when ctx ends.
func (s *Service) Listen(ctx context.Context) (<-chan jobdata.Event, error) {
	return s.broker.Listen(ctx)
}
