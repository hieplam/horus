//go:build goal

// The readiness report of horus in the test stack, as the rows
// stack-starts-together and ready-when-four-up read it. One report has 4
// forms: the text of horus ready (also the error output of a horus mcp that
// does not start), the JSON of /readyz, the JSON of a 503 of /mcp, and the
// details of a NOT_READY envelope. A row compares the whole report with the
// state of the stack: a dependency that is down is named, and a dependency
// that is up is not named. A report that names each dependency, or always the
// same one, fails.
package goal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// dependencies are the 4 dependencies of horus, in the order of the report.
var dependencies = []string{"horus-db", "comfyui", "horus-worker", "horus-api"}

// readyzURL is /readyz of horus api in the test stack.
const readyzURL = "http://127.0.0.1:" + testAPIPort + "/readyz"

// readinessCheck is the result of one dependency in a report as JSON.
type readinessCheck struct {
	Dependency string `json:"dependency"`
	Up         bool   `json:"up"`
	Detail     string `json:"detail"`
}

// readinessReport is the readiness report as JSON: the body of /readyz and
// of a 503 of /mcp. The details of NOT_READY hold its checks and its down.
type readinessReport struct {
	Ready  bool             `json:"ready"`
	Up     *int             `json:"up"`
	Checks []readinessCheck `json:"checks"`
	Down   []string         `json:"down"`
}

// wrongList compares the dependencies that a report names as down with want,
// the dependencies that are down. It gives "" when they are the same list. If
// not, it names each dependency that is up and is in the list, and each one
// that is down and is not in the list.
func wrongList(got, want []string) string {
	if slices.Equal(got, want) || (len(got) == 0 && len(want) == 0) {
		return ""
	}
	var why []string
	for _, d := range got {
		switch {
		case !slices.Contains(dependencies, d):
			why = append(why, fmt.Sprintf("%q is not a dependency", d))
		case !slices.Contains(want, d):
			why = append(why, d+" is up and is in the list")
		}
	}
	for _, d := range want {
		if !slices.Contains(got, d) {
			why = append(why, d+" is down and is not in the list")
		}
	}
	if len(why) == 0 {
		why = append(why, "the order or the count of the names is not that of the report")
	}
	return fmt.Sprintf("names %v as down, want exactly %v: %s", got, want, strings.Join(why, "; "))
}

// readyLine is one line of the text of the report: "ok   <dependency>: ..."
// or "FAIL <dependency>: ...".
var readyLine = regexp.MustCompile(`^(ok  |FAIL) ([a-z-]+)(: .*)?$`)

// readyCount is the first line of the text of the report.
var readyCount = regexp.MustCompile(`^horus ready: (\d+) of (\d+)$`)

// wrongText checks the text of the report (the output of horus ready, or the
// error output of horus mcp) against want, the dependencies that are down. It
// gives the dependencies of the FAIL lines, and "" when the text has the
// count line "horus ready: <up> of 4", exactly one line for each of the 4
// dependencies in the order of the report, a FAIL line for each dependency
// of want, and an ok line for each other one.
func wrongText(text string, want []string) (down []string, why string) {
	count := ""
	var seen []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if m := readyCount.FindStringSubmatch(line); m != nil {
			if count != "" {
				return nil, "the text has 2 count lines"
			}
			count = m[1] + " of " + m[2]
			continue
		}
		if m := readyLine.FindStringSubmatch(line); m != nil {
			seen = append(seen, m[2])
			if m[1] == "FAIL" {
				down = append(down, m[2])
			}
		}
	}
	if !slices.Equal(seen, dependencies) {
		return down, fmt.Sprintf("the text has ok and FAIL lines for %v, want one line for each of %v", seen, dependencies)
	}
	if why := wrongList(down, want); why != "" {
		return down, "the FAIL lines: the text " + why
	}
	if wantCount := fmt.Sprintf("%d of 4", 4-len(want)); count != wantCount {
		return down, fmt.Sprintf("the count line says %q, want %q", count, wantCount)
	}
	return down, ""
}

// wrongReport checks a report as JSON against want, the dependencies that
// are down: the list down, the field ready, the count up, and one check for
// each of the 4 dependencies whose field up says the same as the list.
func wrongReport(r readinessReport, want []string) string {
	if why := wrongList(r.Down, want); why != "" {
		return "the list down " + why
	}
	if r.Ready != (len(want) == 0) {
		return fmt.Sprintf("ready is %v with %d dependencies down", r.Ready, len(want))
	}
	if r.Up == nil || *r.Up != 4-len(want) {
		return fmt.Sprintf("up is not %d", 4-len(want))
	}
	return wrongChecks(r.Checks, want)
}

// wrongChecks checks the list checks of a report against want.
func wrongChecks(checks []readinessCheck, want []string) string {
	var seen, down []string
	for _, c := range checks {
		seen = append(seen, c.Dependency)
		if !c.Up {
			down = append(down, c.Dependency)
		}
	}
	if !slices.Equal(seen, dependencies) {
		return fmt.Sprintf("checks has the dependencies %v, want %v", seen, dependencies)
	}
	if why := wrongList(down, want); why != "" {
		return "the checks with up false: the report " + why
	}
	return ""
}

// wrongNotReady checks the envelope of a call while want is down: NOT_READY
// with action_by human, the list details.down, and details.checks.
func wrongNotReady(e envelope, want []string) string {
	if e.OK || e.Error == nil || e.Error.Code != "NOT_READY" {
		return fmt.Sprintf("%+v, want NOT_READY", e)
	}
	if e.Error.ActionBy != "human" {
		return fmt.Sprintf("NOT_READY with action_by %q, want human", e.Error.ActionBy)
	}
	if _, ok := e.Error.Details["down"].([]any); !ok {
		return fmt.Sprintf("NOT_READY with no list details.down: %v", e.Error.Details)
	}
	if why := wrongList(downNames(e), want); why != "" {
		return "details.down " + why
	}
	raw, err := json.Marshal(e.Error.Details["checks"])
	if err != nil {
		return "details.checks: " + err.Error()
	}
	var checks []readinessCheck
	if err := json.Unmarshal(raw, &checks); err != nil {
		return fmt.Sprintf("details.checks is not a list of checks: %s", raw)
	}
	if why := wrongChecks(checks, want); why != "" {
		return "details." + why
	}
	return ""
}

// horusReady runs docker exec horus_test-api horus ready, and checks it
// against want, the dependencies that are down: exit code 0 and 4 ok lines
// when want is empty, else exit code 1 and the FAIL lines of want. It gives
// the exit code, the output, and "" or what is wrong.
func horusReady(t *testing.T, want []string) (int, string, string) {
	t.Helper()
	code, out := run(t, root, "docker", "exec", apiName, "horus", "ready")
	wantCode := 0
	if len(want) > 0 {
		wantCode = 1
	}
	if code != wantCode {
		return code, out, fmt.Sprintf("docker exec %s horus ready exits %d while %s, want exit code %d:\n%s",
			apiName, code, downText(want), wantCode, out)
	}
	if _, why := wrongText(out, want); why != "" {
		return code, out, fmt.Sprintf("docker exec %s horus ready while %s: %s:\n%s", apiName, downText(want), why, out)
	}
	return code, out, ""
}

// downText says which dependencies are down, for a message.
func downText(want []string) string {
	if len(want) == 0 {
		return "each of the 4 dependencies is up"
	}
	return "the dependencies that are down are " + strings.Join(want, " and ")
}

// getReadyz asks GET /readyz of horus api. It gives the status code, the
// report and the body. With no HTTP answer, the status code is 0.
func getReadyz() (status int, r readinessReport, body string, err error) {
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Get(readyzURL)
	if err != nil {
		return 0, r, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(b, &r); err != nil {
		return resp.StatusCode, r, string(b), fmt.Errorf("HTTP %d with a body that is not the report: %s", resp.StatusCode, b)
	}
	return resp.StatusCode, r, string(b), nil
}

// readyz asks GET /readyz, and checks it against want: 200 with ready true
// when want is empty, else 503 with the report of want. It gives the list
// down of the report, and "" or what is wrong.
func readyz(want []string) ([]string, string) {
	status, r, body, err := getReadyz()
	if err != nil {
		return nil, "GET /readyz: " + err.Error()
	}
	wantStatus := http.StatusOK
	if len(want) > 0 {
		wantStatus = http.StatusServiceUnavailable
	}
	if status != wantStatus {
		return r.Down, fmt.Sprintf("GET /readyz: HTTP %d while %s, want %d: %s", status, downText(want), wantStatus, body)
	}
	if why := wrongReport(r, want); why != "" {
		return r.Down, fmt.Sprintf("GET /readyz while %s: %s: %s", downText(want), why, body)
	}
	return r.Down, ""
}

// toolNames gives the names of the tools of the server (tools/list).
func toolNames(s *session) ([]string, error) {
	raw, err := s.request("tools/list", map[string]any{}, 15*time.Second)
	if err != nil {
		return nil, err
	}
	var res struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	return names, nil
}

// healthOf gives the health of a container of the test stack in docker
// compose ps.
func healthOf(t *testing.T, env []string, name string) string {
	t.Helper()
	for _, r := range ps(t, env) {
		if r.Name == name {
			return r.Health
		}
	}
	return "no container " + name
}
