// Tests of the port of horus/errors.py.
package herr

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// allowedNextStep is the one change of rule 3 of the spec: a next_step that
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

// TestCodesMatchPython compares the Go table with the table of horus/errors.py
// (golden file made by tools/make_server_golden.py).
func TestCodesMatchPython(t *testing.T) {
	rows := readGolden(t)
	if len(rows) != 21 {
		t.Fatalf("golden file has %d codes, want 21", len(rows))
	}
	if len(Codes) != len(rows) {
		t.Fatalf("Go table has %d codes, Python has %d", len(Codes), len(rows))
	}
	for i, py := range rows {
		goRow := Codes[i]
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
	if len(table) != 21 {
		t.Fatalf("%d codes, want 21", len(table))
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
