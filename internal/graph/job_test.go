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

// TestRound1RoundsTheExactBinaryValueHalfToEven: 3.55 is 3.5499... in binary,
// so it gives 3.5. 0.25 is exact, and a half goes to the even digit.
func TestRound1RoundsTheExactBinaryValueHalfToEven(t *testing.T) {
	for in, want := range map[float64]float64{3.55: 3.5, 0.25: 0.2, 0.35: 0.3, 1.04: 1.0, 12.96: 13.0} {
		if got := Round1(in); got != want {
			t.Errorf("Round1(%v) = %v, want %v", in, got, want)
		}
	}
}
