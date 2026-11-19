package jobdata

import (
	"slices"
	"testing"
)

func up(dep string) CheckResult { return CheckResult{Dependency: dep, Up: true} }

func TestReadinessAllUp(t *testing.T) {
	r := ReadinessOf([]CheckResult{up(DependencyAPI), up(DependencyDB), up(DependencyWorker), up(DependencyComfyUI)})
	if !r.Ready || r.Up != 4 || len(r.Down) != 0 {
		t.Fatalf("4 up: %+v", r)
	}
	var order []string
	for _, c := range r.Checks {
		order = append(order, c.Dependency)
	}
	if !slices.Equal(order, Dependencies) {
		t.Errorf("order %v, want %v", order, Dependencies)
	}
}

// Each of the 4 dependencies, when it is down, makes horus not ready and is
// named in Down.
func TestReadinessEachDown(t *testing.T) {
	for _, down := range Dependencies {
		var results []CheckResult
		for _, dep := range Dependencies {
			c := up(dep)
			if dep == down {
				c = CheckResult{Dependency: dep, Up: false, Detail: "no answer in 2 s"}
			}
			results = append(results, c)
		}
		r := ReadinessOf(results)
		if r.Ready || r.Up != 3 || !slices.Equal(r.Down, []string{down}) {
			t.Errorf("%s down: %+v", down, r)
		}
	}
}

func TestReadinessMissingAndDuplicate(t *testing.T) {
	r := ReadinessOf([]CheckResult{up(DependencyDB), up(DependencyWorker), up(DependencyAPI), up("other")})
	if r.Ready || !slices.Equal(r.Down, []string{DependencyComfyUI}) || r.Checks[1].Detail != "no check result" {
		t.Errorf("a missing result: %+v", r)
	}
	if len(r.Checks) != 4 {
		t.Errorf("an unknown name is in the report: %+v", r.Checks)
	}
	r = ReadinessOf([]CheckResult{up(DependencyDB), {Dependency: DependencyDB, Detail: "schema 1, want 2"},
		up(DependencyComfyUI), up(DependencyWorker), up(DependencyAPI)})
	if r.Ready || r.Checks[0].Detail != "schema 1, want 2" {
		t.Errorf("one down result of two: %+v", r)
	}
	if r := ReadinessOf(nil); r.Ready || len(r.Down) != 4 {
		t.Errorf("no result: %+v", r)
	}
}

func TestReadinessText(t *testing.T) {
	r := ReadinessOf([]CheckResult{up(DependencyDB), {Dependency: DependencyComfyUI, Detail: "no answer in 2 s"},
		up(DependencyWorker), up(DependencyAPI)})
	want := "horus ready: 3 of 4\nok   horus-db\nFAIL comfyui: no answer in 2 s\nok   horus-worker\nok   horus-api\n"
	if got := r.Text(); got != want {
		t.Errorf("text:\n%s\nwant:\n%s", got, want)
	}
}
