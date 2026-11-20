package jobdata

import (
	"fmt"
	"strings"
)

// The 4 dependencies of horus in the stack, in the order of the readiness
// report: the 3 containers and the ComfyUI of the host.
const (
	DependencyDB      = "horus-db"
	DependencyComfyUI = "comfyui"
	DependencyWorker  = "horus-worker"
	DependencyAPI     = "horus-api"
)

// Dependencies are the 4 dependencies, in the order of the report.
var Dependencies = []string{DependencyDB, DependencyComfyUI, DependencyWorker, DependencyAPI}

// CheckResult is the result of the check of one dependency.
type CheckResult struct {
	Dependency string `json:"dependency"`
	Up         bool   `json:"up"`
	// Detail says why the check failed, or what it found.
	Detail string `json:"detail"`
}

// Readiness is the readiness report of horus.
type Readiness struct {
	// Ready is true only when the 4 dependencies are up.
	Ready bool `json:"ready"`
	// Up is the count of the dependencies that are up.
	Up int `json:"up"`
	// Checks has one result for each of the 4 dependencies, in the order of
	// Dependencies.
	Checks []CheckResult `json:"checks"`
	// Down names the dependencies that are not up, in the same order.
	Down []string `json:"down"`
}

// ReadinessOf makes the readiness report from the results of the checks. A
// dependency is up only when it has a result and each of its results is up.
// A dependency with no result is down, with the detail "no check result". A
// result for a name that is not one of the 4 is not in the report.
func ReadinessOf(results []CheckResult) Readiness {
	r := Readiness{Checks: []CheckResult{}, Down: []string{}}
	for _, dep := range Dependencies {
		c := CheckResult{Dependency: dep, Up: false, Detail: "no check result"}
		found := false
		for _, res := range results {
			if res.Dependency != dep {
				continue
			}
			if !found || (c.Up && !res.Up) {
				c = res
			}
			found = true
		}
		r.Checks = append(r.Checks, c)
		if c.Up {
			r.Up++
		} else {
			r.Down = append(r.Down, dep)
		}
	}
	r.Ready = len(r.Down) == 0
	return r
}

// Text is the report as lines of text: the count, then one line for each
// dependency.
func (r Readiness) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "horus ready: %d of %d\n", r.Up, len(r.Checks))
	for _, c := range r.Checks {
		b.WriteString(c.Line() + "\n")
	}
	return b.String()
}

// Line is the result as one line of the report: "ok   <dependency>: <detail>"
// or "FAIL <dependency>: <detail>".
func (c CheckResult) Line() string {
	word := "ok  "
	if !c.Up {
		word = "FAIL"
	}
	line := word + " " + c.Dependency
	if c.Detail != "" {
		line += ": " + c.Detail
	}
	return line
}
