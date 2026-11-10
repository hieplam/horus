// Readers of the workflow files. Ports the helper functions of
// scripts/workflow.ps1: Read-Manifest, Get-Description, Get-YamlFolders,
// Get-ModelDirs, Find-ModelFile, Get-Workflows, Get-OtherUsers, Get-AllStrings
// and Get-NodeTypes.
package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// modelExt matches a model file name. The -match of workflow.ps1 ignores case.
var modelExt = regexp.MustCompile(`(?i)\.(safetensors|sft|gguf|ckpt|pt|pth|bin|onnx)$`)

// object is a JSON object that keeps the order of its keys, as
// ConvertFrom-Json does. The order decides the order of the result items.
type object struct {
	keys []string
	vals []any
}

// get looks a key up without case, as workflow.ps1 reads a property.
func (o *object) get(key string) any {
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

func (o *object) has(key string) bool {
	for _, k := range o.keys {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

// parseJSON reads JSON into *object, []any, string, json.Number, bool or nil.
func parseJSON(data []byte) (any, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("invalid JSON: more data after the value")
	}
	return v, nil
}

func parseValue(dec *json.Decoder) (any, error) {
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
		o := &object{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := parseValue(dec)
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
			v, err := parseValue(dec)
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

func readJSON(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v, err := parseJSON(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	return v, nil
}

// truthy is the truth (as workflow.ps1 tests it) of a JSON value.
func truthy(v any) bool {
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
			return truthy(x[0])
		}
		return true
	}
	return true
}

// str is the [string] cast of workflow.ps1 of a JSON value.
func str(v any) string {
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

// toLong is the [long] cast of workflow.ps1 of a JSON value. A value that does not
// convert gives 0, which the manifest check refuses.
func toLong(v any) int64 {
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
	case *object:
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
	o, _ := graph.(*object)
	if o == nil {
		return nil
	}
	if nodes := o.get("nodes"); truthy(nodes) {
		list, ok := nodes.([]any)
		if !ok {
			list = []any{nodes}
		}
		for _, n := range list {
			if no, ok := n.(*object); ok {
				if t := str(no.get("type")); t != "" {
					types = append(types, t)
				}
			}
		}
		return uniqueStrings(types)
	}
	for _, v := range o.vals {
		if no, ok := v.(*object); ok {
			if t := str(no.get("class_type")); t != "" {
				types = append(types, t)
			}
		}
	}
	return uniqueStrings(types)
}

// modelFile is one file of models.json.
type modelFile struct {
	Role, Name, Folder string
	Bytes              int64
	Sha256, URL        string
}

type manifest struct {
	License string
	Files   []modelFile
}

// readManifest is Read-Manifest.
func readManifest(path string) (manifest, error) {
	v, err := readJSON(path)
	if err != nil {
		return manifest{}, err
	}
	m, _ := v.(*object)
	if m == nil {
		m = &object{}
	}
	var entries []any
	switch f := m.get("files").(type) {
	case []any:
		entries = f
	default:
		entries = []any{f}
	}
	out := manifest{License: str(m.get("license"))}
	for _, e := range entries {
		if !truthy(e) {
			continue
		}
		f, ok := e.(*object)
		if !ok {
			f = &object{}
		}
		pick := func(key string) string {
			if truthy(f.get(key)) {
				return str(f.get(key))
			}
			return str(m.get(key))
		}
		repo, rev := pick("repo"), pick("revision")
		u := str(f.get("url"))
		if !truthy(f.get("url")) {
			u = "https://huggingface.co/" + repo + "/resolve/" + rev + "/" + str(f.get("repo_path"))
		}
		var name string
		switch {
		case truthy(f.get("file")):
			name = str(f.get("file"))
		case truthy(f.get("repo_path")):
			name = leaf(str(f.get("repo_path")))
		default:
			name = leaf(urlPath(u))
		}
		if !truthy(f.get("folder")) {
			return manifest{}, fmt.Errorf("A file in %s has no 'folder'.", path)
		}
		sha := str(f.get("sha256"))
		if !hexSHA.MatchString(sha) {
			return manifest{}, fmt.Errorf("File %s in %s has no valid sha256.", name, path)
		}
		n := toLong(f.get("bytes"))
		if n <= 0 {
			return manifest{}, fmt.Errorf("File %s in %s has no valid bytes.", name, path)
		}
		out.Files = append(out.Files, modelFile{
			Role: str(f.get("role")), Name: name, Folder: str(f.get("folder")),
			Bytes: n, Sha256: strings.ToLower(sha), URL: u,
		})
	}
	return out, nil
}

var hexSHA = regexp.MustCompile(`^[A-Fa-f0-9]{64}$`)

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

var (
	yamlComment  = regexp.MustCompile(`\s+#.*$`)
	yamlBasePath = regexp.MustCompile(`(?i)^\s+base_path:\s*(.+)$`)
	yamlDefault  = regexp.MustCompile(`(?i)^\s+is_default:`)
	yamlBlock    = regexp.MustCompile(`^\s+([A-Za-z0-9_]+):\s*\|\s*$`)
	yamlKeyValue = regexp.MustCompile(`^\s+([A-Za-z0-9_]+):\s*(.+)$`)
	yamlTopLevel = regexp.MustCompile(`^\S`)
)

// yamlFolders is Get-YamlFolders, the small reader of extra_model_paths.yaml:
// folder name -> full paths, in file order.
func yamlFolders(path string) map[string][]string {
	out := map[string][]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	base, blockKey := "", ""
	for _, raw := range splitLines(data) {
		line := strings.TrimRightFunc(yamlComment.ReplaceAllString(raw, ""), isSpace)
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimLeftFunc(line, isSpace), "#") {
			continue
		}
		if yamlTopLevel.MatchString(line) {
			base, blockKey = "", ""
			continue
		}
		if m := yamlBasePath.FindStringSubmatch(line); m != nil {
			base, blockKey = trimQuotes(m[1]), ""
			continue
		}
		if yamlDefault.MatchString(line) {
			blockKey = ""
			continue
		}
		if m := yamlBlock.FindStringSubmatch(line); m != nil {
			blockKey = m[1]
			continue
		}
		var key, val string
		if m := yamlKeyValue.FindStringSubmatch(line); m != nil {
			blockKey, key, val = "", m[1], m[2]
		} else if blockKey != "" {
			key, val = blockKey, strings.TrimSpace(line)
		} else {
			continue
		}
		val = trimQuotes(val)
		if val == "" {
			continue
		}
		p := val
		if !isRooted(val) && base != "" {
			p = joinPath(base, val)
		}
		out[key] = append(out[key], fullPath(p))
	}
	return out
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == '\f' || r == '\v'
}

// trimQuotes is .Trim().Trim('"').Trim("'").
func trimQuotes(s string) string {
	return strings.Trim(strings.Trim(strings.TrimSpace(s), `"`), "'")
}

// isRooted is [IO.Path]::IsPathRooted.
func isRooted(p string) bool {
	if filepath.IsAbs(p) || filepath.VolumeName(p) != "" {
		return true
	}
	return strings.HasPrefix(p, "/") || (runtime.GOOS == "windows" && strings.HasPrefix(p, `\`))
}

// joinPath is Join-Path.
func joinPath(a, b string) string { return filepath.Join(a, b) }

// fullPath is Get-FullPath: [IO.Path]::GetFullPath, without a trailing backslash.
func fullPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		p = a
	}
	return strings.TrimRight(filepath.Clean(p), `\`)
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// workflowDir is one folder under the workflows folder.
type workflowDir struct {
	Name, Dir, ModelsJSON, Description string
}

// listWorkflows is Get-Workflows: every folder that does not start with "_",
// sorted by name without case.
func listWorkflows(root string) ([]workflowDir, error) {
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
	var out []workflowDir
	for _, n := range names {
		if strings.HasPrefix(n, "_") {
			continue
		}
		dir := filepath.Join(root, n)
		w := workflowDir{Name: n, Dir: dir, Description: description(filepath.Join(dir, "README.md"))}
		if mj := filepath.Join(dir, "models.json"); isFile(mj) {
			w.ModelsJSON = mj
		}
		out = append(out, w)
	}
	return out, nil
}
