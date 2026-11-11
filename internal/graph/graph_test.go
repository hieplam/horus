// Tests of the graph conversion.
package graph

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hieplam/horus/internal/herr"
)

func readFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fixtureInfo(t *testing.T) ObjectInfo {
	t.Helper()
	var info ObjectInfo
	if err := json.Unmarshal(readFile(t, "..", "..", "tests", "fixtures", "object_info.json"), &info); err != nil {
		t.Fatal(err)
	}
	return info
}

func jsonValue(t *testing.T, data []byte) any {
	t.Helper()
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func codeOf(err error) string {
	var he *herr.Error
	if errors.As(err, &he) {
		return he.Code
	}
	return ""
}

// TestUIToAPIGolden compares UIToAPI of workflows/it-illustration with the
// output of the first version of horus, written in Python (a golden file). The JSON
// values must be equal, and the node and input order must be the same too.
func TestUIToAPIGolden(t *testing.T) {
	graph := readFile(t, "..", "..", "workflows", "it-illustration", "workflow.json")
	golden := readFile(t, "..", "..", "tests", "fixtures", "ui_to_api.golden.json")
	api, err := UIToAPI(graph, fixtureInfo(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(api)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(jsonValue(t, got), jsonValue(t, golden)) {
		t.Fatalf("ui_to_api differs from the golden file\ngot:  %s", got)
	}
	var want bytes.Buffer
	if err := json.Compact(&want, golden); err != nil {
		t.Fatal(err)
	}
	if string(got) != want.String() {
		t.Fatalf("same values, but the key order differs\ngot:  %s\nwant: %s", got, want.String())
	}
	if api.Len() != 10 {
		t.Fatalf("got %d nodes, want 10", api.Len())
	}
}

const smallInfo = `{
 "Loader": {"input": {"required": {"ckpt": [["a.safetensors", "b.safetensors"]], "mode": ["COMBO", {"options": ["x", "y"]}]}}},
 "Sampler": {"input": {"required": {"model": ["MODEL"], "seed": ["INT", {"control_after_generate": true}], "steps": ["INT", {}]},
                       "optional": {"denoise": ["FLOAT", {}]}}},
 "Encode": {"input": {"required": {"text": ["STRING", {}], "clip": ["CLIP"]}}}
}`

// TestUIToAPIRules uses a small graph. The expected text is the output of the
// Python ui_to_api for the same input.
func TestUIToAPIRules(t *testing.T) {
	var info ObjectInfo
	if err := json.Unmarshal([]byte(smallInfo), &info); err != nil {
		t.Fatal(err)
	}
	graph := `{"nodes": [
	  {"id": 1, "type": "Loader", "widgets_values": ["a.safetensors", "y"]},
	  {"id": 2, "type": "Note", "widgets_values": ["for people"]},
	  {"id": 3, "type": "Sampler", "inputs": [{"name": "model", "link": 10}, {"name": "steps", "link": 11}],
	   "widgets_values": [42, "randomize", 20, 0.5]},
	  {"id": 4, "type": "Encode", "widgets_values": []}
	 ], "links": [[10, 1, 0, 3, 0, "MODEL"], [11, 1, 1, 3, 1, "INT"]]}`
	api, err := UIToAPI([]byte(graph), info)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(api)
	// The Note is left out. "randomize" after the seed is skipped. A linked
	// widget input keeps its link. A node without values gets no widget input.
	want := `{"1":{"class_type":"Loader","inputs":{"ckpt":"a.safetensors","mode":"y"}},` +
		`"3":{"class_type":"Sampler","inputs":{"model":["1",0],"steps":["1",1],"seed":42,"denoise":0.5}},` +
		`"4":{"class_type":"Encode","inputs":{}}}`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestUIToAPIMissingNodePack(t *testing.T) {
	_, err := UIToAPI([]byte(`{"nodes": [{"id": 1, "type": "Zeta"}, {"id": 2, "type": "Alpha"}, {"id": 3, "type": "Note"}], "links": []}`), ObjectInfo{})
	var he *herr.Error
	if !errors.As(err, &he) || he.Code != "NODE_PACK_MISSING" || !reflect.DeepEqual(he.Details["missing"], []string{"Alpha", "Zeta"}) {
		t.Fatalf("got %v", err)
	}
}

func TestMissingModelFiles(t *testing.T) {
	info := fixtureInfo(t)
	graph := readFile(t, "..", "..", "workflows", "it-illustration", "workflow.json")
	api, err := UIToAPI(graph, info)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := MissingModelFiles(api, info)
	if err != nil || len(missing) != 0 {
		t.Fatalf("shipped graph: missing %v, %v", missing, err)
	}
	for _, id := range api.IDs() {
		if api.Node(id).ClassType == "UNETLoader" {
			_ = api.Node(id).Inputs.Set("unet_name", "absent-model.safetensors")
		}
	}
	missing, err = MissingModelFiles(api, info)
	if err != nil || !reflect.DeepEqual(missing, []string{"absent-model.safetensors"}) {
		t.Fatalf("got %v, %v", missing, err)
	}
}

func TestComboOptions(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want []any
		ok   bool
	}{
		{`[["a", "b"], {}]`, []any{"a", "b"}, true},
		{`["COMBO", {"options": ["x"]}]`, []any{"x"}, true},
		{`["STRING", {}]`, nil, false},
		{`[]`, nil, false},
		{``, nil, false},
	} {
		got, ok := comboOptions(json.RawMessage(c.raw))
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v %v", c.raw, got, ok)
		}
	}
}

// TestPyDumps checks the json.dumps form: ", " and ": ", and \uXXXX for every
// character outside ASCII, with a surrogate pair above U+FFFF.
func TestPyDumps(t *testing.T) {
	bs := `\`
	text := string(rune(0xe9)) + " " + string(rune(0x2603)) + " " + string(rune(0x1f600))
	in, _ := json.Marshal(map[string]any{"a": []any{1, 2.5, true, nil, text + "\n"}})
	got := pyDumps(in)
	want := `{"a": [1, 2.5, true, null, "` + strings.Join([]string{bs + "u00e9", bs + "u2603", bs + "ud83d" + bs + "ude00"}, " ") + bs + `n"]}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}
