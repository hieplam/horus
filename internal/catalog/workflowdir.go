// The reader of the workflows folder for the setup of the model files (horus
// workflow and the MCP tools that install, check and remove). It keeps the
// rules of scripts/workflow.ps1: keys without case, the casts of the script,
// and the order of the keys of a JSON object. So it is a second reader of
// models.json, beside Models: Models is the reader of catalog.py for the MCP
// tools, this one is the reader of workflow.ps1. The two give the same files
// for a valid models.json, and they keep their own error texts.
package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Folder is one folder under the workflows folder.
type Folder struct {
	Name        string
	Dir         string
	ModelsJSON  string // the path of models.json, or "" when the folder has none
	Description string // the first paragraph after the "# name" heading of README.md
}

// Folders is Get-Workflows: every folder that does not start with "_",
// sorted by name without case.
func Folders(root string) ([]Folder, error) {
	if !isDir(root) {
		return nil, fmt.Errorf("No workflows folder: %s", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || (e.Type()&os.ModeSymlink != 0 && isDir(filepath.Join(root, e.Name()))) {
			names = append(names, e.Name())
		}
	}
	sort.SliceStable(names, func(i, j int) bool {
		a, b := strings.ToLower(names[i]), strings.ToLower(names[j])
		if a != b {
			return a < b
		}
		return names[i] < names[j]
	})
	var out []Folder
	for _, n := range names {
		if strings.HasPrefix(n, "_") {
			continue
		}
		dir := filepath.Join(root, n)
		w := Folder{Name: n, Dir: dir, Description: description(filepath.Join(dir, "README.md"))}
		if mj := filepath.Join(dir, "models.json"); isRegular(mj) {
			w.ModelsJSON = mj
		}
		out = append(out, w)
	}
	return out, nil
}

var workflowName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// FindModelList finds the models.json of one workflow under root, as
// workflow.ps1 does, and reads it. modelsJSON, when it is not "", is the file
// to read instead of <root>/<name>/models.json. graphPath is the path of the
// workflow.json of the workflow (the file can be absent).
func FindModelList(root, name, modelsJSON string) (list ModelList, graphPath string, err error) {
	if !isDir(root) {
		return ModelList{}, "", fmt.Errorf("No workflows folder: %s", root)
	}
	if name == "" {
		return ModelList{}, "", fmt.Errorf("Name a workflow. Run 'horus workflow list' to see them.")
	}
	if !workflowName.MatchString(name) {
		return ModelList{}, "", fmt.Errorf("Invalid workflow name: %s", name)
	}
	dir := filepath.Join(root, name)
	if modelsJSON == "" {
		if !isDir(dir) {
			ws, err := Folders(root)
			if err != nil {
				return ModelList{}, "", err
			}
			names := make([]string, len(ws))
			for i, w := range ws {
				names[i] = w.Name
			}
			return ModelList{}, "", fmt.Errorf("No workflow '%s'. Known: %s", name, strings.Join(names, ", "))
		}
		modelsJSON = filepath.Join(dir, "models.json")
	}
	if !isRegular(modelsJSON) {
		return ModelList{}, "", fmt.Errorf("Model list not found: %s", modelsJSON)
	}
	list, err = ReadModelList(modelsJSON)
	if err != nil {
		return ModelList{}, "", err
	}
	return list, filepath.Join(dir, "workflow.json"), nil
}

// ListedFile is one file of models.json, as workflow.ps1 reads it.
type ListedFile struct {
	Role, Name, Folder string
	Bytes              int64
	SHA256, URL        string
}

// ModelList is models.json, as workflow.ps1 reads it.
type ModelList struct {
	License string
	Files   []ListedFile
}

// ReadModelList is Read-Manifest of workflow.ps1.
func ReadModelList(path string) (ModelList, error) {
	v, err := readPSJSON(path)
	if err != nil {
		return ModelList{}, err
	}
	m, _ := v.(*psObject)
	if m == nil {
		m = &psObject{}
	}
	var entries []any
	switch f := m.get("files").(type) {
	case []any:
		entries = f
	default:
		entries = []any{f}
	}
	out := ModelList{License: psString(m.get("license"))}
	for _, e := range entries {
		if !psTruthy(e) {
			continue
		}
		f, ok := e.(*psObject)
		if !ok {
			f = &psObject{}
		}
		pick := func(key string) string {
			if psTruthy(f.get(key)) {
				return psString(f.get(key))
			}
			return psString(m.get(key))
		}
		repo, rev := pick("repo"), pick("revision")
		u := psString(f.get("url"))
		if !psTruthy(f.get("url")) {
			u = "https://huggingface.co/" + repo + "/resolve/" + rev + "/" + psString(f.get("repo_path"))
		}
		var name string
		switch {
		case psTruthy(f.get("file")):
			name = psString(f.get("file"))
		case psTruthy(f.get("repo_path")):
			name = leaf(psString(f.get("repo_path")))
		default:
			name = leaf(urlPath(u))
		}
		if !psTruthy(f.get("folder")) {
			return ModelList{}, fmt.Errorf("A file in %s has no 'folder'.", path)
		}
		sha := psString(f.get("sha256"))
		if !hexSHA.MatchString(sha) {
			return ModelList{}, fmt.Errorf("File %s in %s has no valid sha256.", name, path)
		}
		n := psLong(f.get("bytes"))
		if n <= 0 {
			return ModelList{}, fmt.Errorf("File %s in %s has no valid bytes.", name, path)
		}
		out.Files = append(out.Files, ListedFile{
			Role: psString(f.get("role")), Name: name, Folder: psString(f.get("folder")),
			Bytes: n, SHA256: strings.ToLower(sha), URL: u,
		})
	}
	return out, nil
}

// GraphRefs are the names in a graph file that the check of the model files
// reads.
type GraphRefs struct {
	// Models are the model file names that the graph loads, without a folder,
	// in the order of the file, each one time.
	Models []string
	// NodeTypes are the node classes of the graph (UI or API format), in order,
	// each one time.
	NodeTypes []string
}

// ReadGraphRefs reads a graph file (workflow.json) as workflow.ps1 does. It
// returns nil when no file is at path.
func ReadGraphRefs(path string) (*GraphRefs, error) {
	if !isRegular(path) {
		return nil, nil
	}
	g, err := readPSJSON(path)
	if err != nil {
		return nil, err
	}
	var models []string
	allStrings(g, func(s string) {
		if modelExt.MatchString(s) {
			models = append(models, leaf(strings.ReplaceAll(s, `\`, "/")))
		}
	})
	return &GraphRefs{Models: uniqueStrings(models), NodeTypes: nodeTypes(g)}, nil
}

// modelExt matches a model file name. The -match of workflow.ps1 ignores case.
var modelExt = regexp.MustCompile(`(?i)\.(safetensors|sft|gguf|ckpt|pt|pth|bin|onnx)$`)

var hexSHA = regexp.MustCompile(`^[A-Fa-f0-9]{64}$`)

// psObject is a JSON object that keeps the order of its keys, as
// ConvertFrom-Json does. The order decides the order of the result items.
type psObject struct {
	keys []string
	vals []any
}

// get looks a key up without case, as workflow.ps1 reads a property.
func (o *psObject) get(key string) any {
	for i, k := range o.keys {
		if k == key {
			return o.vals[i]
		}
	}
	for i, k := range o.keys {
		if strings.EqualFold(k, key) {
			return o.vals[i]
		}
	}
	return nil
}

// parsePSJSON reads JSON into *psObject, []any, string, json.Number, bool or nil.
func parsePSJSON(data []byte) (any, error) {
	data = bytes.TrimPrefix(data, bom)
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parsePSValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("invalid JSON: more data after the value")
	}
	return v, nil
}

func parsePSValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch d {
	case '{':
		o := &psObject{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := parsePSValue(dec)
			if err != nil {
				return nil, err
			}
			o.keys = append(o.keys, kt.(string))
			o.vals = append(o.vals, v)
		}
		_, err = dec.Token()
		return o, err
	case '[':
		arr := []any{}
		for dec.More() {
			v, err := parsePSValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		_, err = dec.Token()
		return arr, err
	}
	return nil, fmt.Errorf("invalid JSON")
}

func readPSJSON(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v, err := parsePSJSON(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	return v, nil
}

// psTruthy is the truth (as workflow.ps1 tests it) of a JSON value.
func psTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	case bool:
		return x
	case json.Number:
		f, err := x.Float64()
		return err != nil || f != 0
	case []any:
		switch len(x) {
		case 0:
			return false
		case 1:
			return psTruthy(x[0])
		}
		return true
	}
	return true
}

// psString is the [string] cast of workflow.ps1 of a JSON value.
func psString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "True"
		}
		return "False"
	}
	return fmt.Sprint(v)
}

// psLong is the [long] cast of workflow.ps1 of a JSON value. A value that does
// not convert gives 0, which the manifest check refuses.
func psLong(v any) int64 {
	var s string
	switch x := v.(type) {
	case json.Number:
		s = x.String()
	case string:
		s = strings.TrimSpace(x)
	case bool:
		if x {
			return 1
		}
		return 0
	default:
		return 0
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return int64(math.RoundToEven(f))
	}
	return 0
}

// allStrings calls fn for every string value in a JSON tree, in order.
func allStrings(v any, fn func(string)) {
	switch x := v.(type) {
	case string:
		fn(x)
	case []any:
		for _, e := range x {
			allStrings(e, fn)
		}
	case *psObject:
		for _, e := range x.vals {
			allStrings(e, fn)
		}
	}
}

// leaf is the last part of a path with / or \ separators.
func leaf(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// uniqueStrings keeps the first of equal strings (Select-Object -Unique, with case).
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// nodeTypes is Get-NodeTypes: the node classes of a UI graph or an API graph.
func nodeTypes(graph any) []string {
	var types []string
	o, _ := graph.(*psObject)
	if o == nil {
		return nil
	}
	if nodes := o.get("nodes"); psTruthy(nodes) {
		list, ok := nodes.([]any)
		if !ok {
			list = []any{nodes}
		}
		for _, n := range list {
			if no, ok := n.(*psObject); ok {
				if t := psString(no.get("type")); t != "" {
					types = append(types, t)
				}
			}
		}
		return uniqueStrings(types)
	}
	for _, v := range o.vals {
		if no, ok := v.(*psObject); ok {
			if t := psString(no.get("class_type")); t != "" {
				types = append(types, t)
			}
		}
	}
	return uniqueStrings(types)
}

// urlPath is the path part of an address, as [uri].AbsolutePath gives it.
func urlPath(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
		if j := strings.Index(u, "/"); j >= 0 {
			u = u[j:]
		} else {
			u = "/"
		}
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return u
}

// description is Get-Description: the first paragraph after the "# name" heading.
func description(readme string) string {
	data, err := os.ReadFile(readme)
	if err != nil {
		if fi, serr := os.Stat(readme); serr != nil || fi.IsDir() {
			return "(no README.md)"
		}
		return "(no description)"
	}
	var para []string
	after := false
	for _, l := range splitLines(data) {
		if !after {
			if headingLine.MatchString(l) {
				after = true
			}
			continue
		}
		if strings.TrimSpace(l) == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if skipLine.MatchString(l) {
			if len(para) > 0 {
				break
			}
			continue
		}
		para = append(para, strings.TrimSpace(l))
	}
	if len(para) == 0 {
		return "(no description)"
	}
	return strings.Join(para, " ")
}

var (
	headingLine = regexp.MustCompile(`^#\s`)
	skipLine    = regexp.MustCompile("^(#|\\||!\\[|```)")
)

// splitLines splits as [IO.File]::ReadAllLines does, without a BOM.
func splitLines(data []byte) []string {
	s := string(bytes.TrimPrefix(data, bom))
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

var bom = []byte{0xef, 0xbb, 0xbf}

func isRegular(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}
