package graph

import "testing"

func TestQueueStates(t *testing.T) {
	cases := []struct {
		name                     string
		q                        Queue
		running, pending, absent bool
	}{
		{"running", Queue{Running: true}, true, false, false},
		{"pending", Queue{Position: 2}, false, true, false},
		{"absent", Queue{}, false, false, true},
	}
	for _, c := range cases {
		if c.q.Running != c.running || c.q.Pending() != c.pending || c.q.Absent() != c.absent {
			t.Errorf("%s: %+v", c.name, c.q)
		}
	}
}
