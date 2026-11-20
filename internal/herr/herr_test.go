// Tests of the error table.
package herr

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// allowedNextStep is the one difference from the golden table: a next_step that
// named a .ps1 script names the horus subcommand.
var allowedNextStep = strings.NewReplacer("scripts/start-comfyui.ps1", "horus start-comfyui")

type goldenRow struct {
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
	ActionBy  string `json:"action_by"`
	NextStep  string `json:"next_step"`
}

func readGolden(t *testing.T) []goldenRow {
	t.Helper()
	raw, err := os.ReadFile("testdata/errors.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []goldenRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

// newCodes are the codes that horus added after the Python version, each
// with the code that comes before it in the table. The golden file does not
// change (spec docs/specs/2026-11-15-release-install.md, "Setup through MCP";
// spec docs/specs/2026-11-19-job-queue.md, "MCP contract").
var newCodes = map[string]string{"SETUP_CONFIRMATION_REQUIRED": "LICENSE_CONFIRMATION_REQUIRED", "NOT_READY": "CANCELLED"}

// oldCodes is the table without the new codes, in table order.
func oldCodes() []Spec {
	var out []Spec
	for _, s := range Codes {
		if _, ok := newCodes[s.Code]; !ok {
			out = append(out, s)
		}
	}
	return out
}

// TestCodesMatchPython compares the Go table with the golden table of the first
// version of horus, written in Python. The table is the golden table plus
// exactly the new codes, each at its place.
func TestCodesMatchPython(t *testing.T) {
	rows := readGolden(t)
	if len(rows) != 21 {
		t.Fatalf("golden file has %d codes, want 21", len(rows))
	}
	if len(Codes) != len(rows)+len(newCodes) {
		t.Fatalf("Go table has %d codes, want the %d of Python plus %d new", len(Codes), len(rows), len(newCodes))
	}
	for i, s := range Codes {
		if before, ok := newCodes[s.Code]; ok && (i == 0 || Codes[i-1].Code != before) {
			t.Errorf("%s is not after %s", s.Code, before)
		}
	}
	old := oldCodes()
	for i, py := range rows {
		goRow := old[i]
		if goRow.Code != py.Code {
			t.Errorf("row %d: code %s, Python %s", i, goRow.Code, py.Code)
			continue
		}
		if goRow.Retryable != py.Retryable {
			t.Errorf("%s: retryable %v, Python %v", py.Code, goRow.Retryable, py.Retryable)
		}
		if goRow.ActionBy != py.ActionBy {
			t.Errorf("%s: action_by %s, Python %s", py.Code, goRow.ActionBy, py.ActionBy)
		}
		if want := allowedNextStep.Replace(py.NextStep); goRow.NextStep != want {
			t.Errorf("%s: next_step\n got %q\nwant %q", py.Code, goRow.NextStep, want)
		}
		if strings.Contains(goRow.NextStep, ".ps1") {
			t.Errorf("%s: next_step names a .ps1 script: %q", py.Code, goRow.NextStep)
		}
	}
}

func TestInfoDefaultsAndOverrides(t *testing.T) {
	info := New("OUT_OF_MEMORY", "boom", nil).Info()
	if !info.Retryable || info.ActionBy != "caller" || info.NextStep == "" || info.Details == nil {
		t.Fatalf("defaults wrong: %+v", info)
	}
	no := false
	e := New("GENERATION_FAILED", "bad graph", map[string]any{"x": 1})
	e.Retryable, e.NextStep = &no, "Fix the graph."
	info = e.Info()
	if info.Retryable || info.NextStep != "Fix the graph." || info.ActionBy != "retry" {
		t.Fatalf("overrides wrong: %+v", info)
	}
	raw, _ := json.Marshal(info)
	want := `{"code":"GENERATION_FAILED","message":"bad graph","retryable":false,"action_by":"retry","next_step":"Fix the graph.","details":{"x":1}}`
	if string(raw) != want {
		t.Fatalf("key order or content:\n got %s\nwant %s", raw, want)
	}
	if e.Error() != "GENERATION_FAILED: bad graph" {
		t.Fatalf("Error() = %q", e.Error())
	}
}

func TestUnknownCodePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New with an unknown code did not panic")
		}
	}()
	New("NOPE", "x", nil)
}

func TestTableJSON(t *testing.T) {
	var table map[string]struct {
		Retryable *bool  `json:"retryable"`
		ActionBy  string `json:"action_by"`
		NextStep  string `json:"next_step"`
	}
	if err := json.Unmarshal(TableJSON(), &table); err != nil {
		t.Fatal(err)
	}
	if len(table) != 23 {
		t.Fatalf("%d codes, want 23", len(table))
	}
	if s := table["NOT_READY"]; s.Retryable == nil || !*s.Retryable || s.ActionBy != "human" ||
		!strings.Contains(s.NextStep, "details.down") {
		t.Errorf("NOT_READY: %+v", s)
	}
	if s := table["SETUP_CONFIRMATION_REQUIRED"]; s.Retryable == nil || *s.Retryable || s.ActionBy != "human" ||
		!strings.Contains(s.NextStep, "accept_setup=true") {
		t.Errorf("SETUP_CONFIRMATION_REQUIRED: %+v", s)
	}
	for _, py := range readGolden(t) {
		row, ok := table[py.Code]
		if !ok || row.Retryable == nil || *row.Retryable != py.Retryable || row.ActionBy != py.ActionBy {
			t.Errorf("%s: %+v", py.Code, row)
		}
	}
	// Table order, so a reader of the JSON sees the codes as in docs/mcp.md.
	text := string(TableJSON())
	if strings.Index(text, `"INVALID_ARGUMENT"`) > strings.Index(text, `"INTERNAL"`) {
		t.Fatal("codes are not in table order")
	}
}
