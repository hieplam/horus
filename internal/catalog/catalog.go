// Package catalog is the list of workflows that horus offers.
//
// A workflow is a folder workflows/<name>/ with:
//
//	manifest.json     the public contract: the kind, the summary and what it
//	                  is good for. An image manifest also has the output
//	                  sizes, how to write the subject and which graph inputs
//	                  horus sets (graph.inputs). A speech manifest has the
//	                  limits of the text, the voices, the tags and the formats.
//	workflow.json     the ComfyUI graph in UI format (an image workflow only).
//	models.json       the model files, with size and SHA-256.
//
// A workflow is style-neutral. horus sends the caller's subject to the model
// as written. The look, the palette and the design system belong to the
// caller, so a manifest with "theme" or "caption" (a house style) is refused.
//
// A folder without manifest.json (or with a name that starts with "_") is not
// offered by the MCP server.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hieplam/horus/internal/herr"
	"github.com/hieplam/horus/internal/settings"
)

// nameRE matches a workflow name. Like Python's re.match with "$", it accepts one
// newline at the end.
var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\n?$`)

// The kinds of a workflow. A manifest without "kind" is of the kind image.
const (
	KindImage  = "image"
	KindSpeech = "speech"
)

// The keys that a manifest must have, for each kind. A speech manifest has
// none of imageOnlyKeys.
var (
	speechKeys    = []string{"name", "title", "summary", "text", "voices", "default_voice", "tags", "formats"}
	imageKeys     = []string{"name", "title", "summary", "output", "subject", "graph"}
	imageOnlyKeys = []string{"graph", "subject", "output"}
)

// requiredInputs are the graph inputs that every manifest must name.
var requiredInputs = []string{"caption", "seed", "width", "height", "filename_prefix"}

// Workflow is one folder under workflows/.
type Workflow struct {
	Name     string
	Dir      string
	Manifest *Object
}

// Kind is what the workflow makes: KindImage or KindSpeech. A manifest without
// the field "kind" is of the kind image. All checks the value, so a workflow
// that All returns has one of the two kinds.
func (w *Workflow) Kind() string { return kindOf(w.Manifest) }

func kindOf(m *Object) string {
	var k string
	if m.Has("kind") && m.Decode("kind", &k) == nil && k != "" {
		return k
	}
	return KindImage
}

// GraphPath is the path of the UI-format graph.
func (w *Workflow) GraphPath() (string, error) {
	g, err := w.Manifest.Object("graph")
	if err != nil || g == nil {
		return "", fmt.Errorf("manifest.json of %s has no graph", w.Name)
	}
	var file string
	if err := g.Decode("file", &file); err != nil {
		return "", fmt.Errorf("manifest.json of %s: graph.file: %w", w.Name, err)
	}
	return filepath.Join(w.Dir, file), nil
}

// ModelFile is one entry of models.json "files". Raw keeps every field.
type ModelFile struct {
	Role     string `json:"role"`
	RepoPath string `json:"repo_path"`
	Folder   string `json:"folder"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
}

// Models is models.json.
type Models struct {
	Repo    string      `json:"repo"`
	License *string     `json:"license"`
	Files   []ModelFile `json:"files"`
}

// Models reads models.json. A missing file gives no files.
func (w *Workflow) Models() (Models, error) {
	var m Models
	data, err := os.ReadFile(filepath.Join(w.Dir, "models.json"))
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("models.json of %s: %w", w.Name, err)
	}
	return m, nil
}

// License is the license view of describe_workflow, with the keys in the
// order that describe_workflow shows.
type License struct {
	Name          string  `json:"name"`
	URL           *string `json:"url"`
	DownloadBytes int64   `json:"download_bytes"`
}

// License returns the license name, its page and the total download size.
func (w *Workflow) License() (License, error) {
	m, err := w.Models()
	if err != nil {
		return License{}, err
	}
	lic := License{Name: "not stated"}
	if m.License != nil {
		lic.Name = *m.License
	}
	if m.Repo != "" {
		u := "https://huggingface.co/" + m.Repo
		lic.URL = &u
	}
	for _, f := range m.Files {
		lic.DownloadBytes += f.Bytes
	}
	return lic, nil
}

func checkManifest(name string, m *Object) error {
	kind := KindImage
	if m.Has("kind") {
		var raw any
		_ = m.Decode("kind", &raw)
		if k, ok := raw.(string); !ok || (k != KindImage && k != KindSpeech) {
			return fmt.Errorf("manifest.json of %s has kind %s: allowed are 'image' and 'speech'", name, pyRepr(raw))
		}
		kind = kindOf(m)
	}
	required := imageKeys
	if kind == KindSpeech {
		required = speechKeys
	}
	var missing []string
	for _, k := range required {
		if !m.Has(k) {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("manifest.json of %s lacks: %s", name, strings.Join(missing, ", "))
	}
	var styled []string
	for _, k := range []string{"theme", "caption"} {
		if m.Has(k) {
			styled = append(styled, k)
		}
	}
	if len(styled) > 0 {
		return fmt.Errorf("manifest.json of %s has %s: a workflow must not add a style; the caller owns it", name, strings.Join(styled, ", "))
	}
	var got any
	_ = m.Decode("name", &got)
	if s, ok := got.(string); !ok || s != name {
		return fmt.Errorf("manifest.json of %s has name %s", name, pyRepr(got))
	}
	if kind == KindSpeech {
		return checkSpeech(name, m)
	}
	graph, err := m.Object("graph")
	if err != nil {
		return fmt.Errorf("manifest.json of %s: %w", name, err)
	}
	var inputs *Object
	if graph != nil {
		if inputs, err = graph.Object("inputs"); err != nil {
			return fmt.Errorf("manifest.json of %s: graph.%w", name, err)
		}
	}
	var absent []string
	for _, k := range requiredInputs {
		if inputs == nil || !inputs.Has(k) {
			absent = append(absent, k)
		}
	}
	if len(absent) > 0 {
		return fmt.Errorf("manifest.json of %s: graph.inputs lacks %s", name, strings.Join(absent, ", "))
	}
	return nil
}

// checkSpeech checks the part of a speech manifest that is not in an image
// manifest. A speech workflow has no ComfyUI graph, no subject and no output
// size.
func checkSpeech(name string, m *Object) error {
	var extra []string
	for _, k := range imageOnlyKeys {
		if m.Has(k) {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		return fmt.Errorf("manifest.json of %s has %s: a speech workflow has no graph, no subject and no output", name, strings.Join(extra, ", "))
	}
	var voices []string
	if err := m.Decode("voices", &voices); err != nil || len(voices) == 0 {
		return fmt.Errorf("manifest.json of %s: voices must be a list of voice names, not empty", name)
	}
	var def string
	if err := m.Decode("default_voice", &def); err != nil {
		return fmt.Errorf("manifest.json of %s: default_voice must be a voice name", name)
	}
	known := false
	for _, v := range voices {
		known = known || v == def
	}
	if !known {
		return fmt.Errorf("manifest.json of %s: default_voice %s is not in voices", name, pyRepr(def))
	}
	for _, k := range []string{"tags", "formats"} {
		var list []string
		if err := m.Decode(k, &list); err != nil {
			return fmt.Errorf("manifest.json of %s: %s must be a list of strings", name, k)
		}
	}
	if _, err := m.Object("text"); err != nil {
		return fmt.Errorf("manifest.json of %s: %w", name, err)
	}
	return nil
}

// All returns the workflows that the server offers, sorted by folder name.
// A manifest that breaks a rule is an error.
func All() ([]*Workflow, error) {
	root := settings.WorkflowsDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil // no workflows folder: no workflows
	}
	var out []*Workflow
	for _, e := range entries { // os.ReadDir sorts by name
		if !isDir(filepath.Join(root, e.Name())) || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil {
			continue
		}
		var m Object
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("manifest.json of %s: %w", e.Name(), err)
		}
		if err := checkManifest(e.Name(), &m); err != nil {
			return nil, err
		}
		out = append(out, &Workflow{Name: e.Name(), Dir: dir, Manifest: &m})
	}
	return out, nil
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// Get returns one workflow, or the error INVALID_ARGUMENT or WORKFLOW_NOT_FOUND.
func Get(name string) (*Workflow, error) {
	if !nameRE.MatchString(name) {
		return nil, herr.New("INVALID_ARGUMENT", "invalid workflow name "+pyRepr(name), map[string]any{"field": "workflow"})
	}
	all, err := All()
	if err != nil {
		return nil, err
	}
	known := []string{}
	for _, w := range all {
		if w.Name == name {
			return w, nil
		}
		known = append(known, w.Name)
	}
	sort.Strings(known)
	return nil, herr.New("WORKFLOW_NOT_FOUND", "no workflow named "+pyRepr(name), map[string]any{"known": known})
}

// Output is the short output view of list_workflows.
type Output struct {
	Format        json.RawMessage `json:"format"`
	DefaultWidth  json.RawMessage `json:"default_width"`
	DefaultHeight json.RawMessage `json:"default_height"`
}

// Summary is the short view for list_workflows, with the keys in the order of
// catalog.summary.
type Summary struct {
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Title          json.RawMessage `json:"title"`
	Summary        json.RawMessage `json:"summary"`
	GoodFor        json.RawMessage `json:"good_for"`
	NotGoodFor     json.RawMessage `json:"not_good_for"`
	Output         any             `json:"output"`
	TypicalSeconds json.RawMessage `json:"typical_seconds"`
}

func orEmptyList(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return json.RawMessage("[]")
	}
	return raw
}

func orNull(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return json.RawMessage("null")
	}
	return raw
}

// SummaryOf returns the short view of a workflow.
func SummaryOf(w *Workflow) (Summary, error) {
	m := w.Manifest
	if w.Kind() == KindSpeech {
		return Summary{
			Name:           w.Name,
			Kind:           KindSpeech,
			Title:          m.Raw("title"),
			Summary:        m.Raw("summary"),
			GoodFor:        orEmptyList(m.Raw("good_for")),
			NotGoodFor:     orEmptyList(m.Raw("not_good_for")),
			TypicalSeconds: typicalSeconds(m),
		}, nil
	}
	out, err := m.Object("output")
	if err != nil || out == nil {
		return Summary{}, fmt.Errorf("manifest.json of %s: output is not an object", w.Name)
	}
	for _, k := range []string{"format", "default_width", "default_height"} {
		if !out.Has(k) {
			return Summary{}, fmt.Errorf("manifest.json of %s: output lacks %s", w.Name, k)
		}
	}
	return Summary{
		Name:           w.Name,
		Kind:           KindImage,
		Title:          m.Raw("title"),
		Summary:        m.Raw("summary"),
		GoodFor:        orEmptyList(m.Raw("good_for")),
		NotGoodFor:     orEmptyList(m.Raw("not_good_for")),
		Output:         Output{out.Raw("format"), out.Raw("default_width"), out.Raw("default_height")},
		TypicalSeconds: typicalSeconds(m),
	}, nil
}

func typicalSeconds(m *Object) json.RawMessage {
	var typical json.RawMessage
	if timing, err := m.Object("timing"); err == nil && timing != nil {
		typical = timing.Raw("measured_seconds")
	}
	return orNull(typical)
}

// Detail is the full view for describe_workflow, with the keys in the order of
// catalog.detail. It has no ComfyUI details (graph, node names).
type Detail struct {
	Summary
	Subject *Object `json:"subject"`
	// Speech is set for a workflow of the kind speech: the limits of the text,
	// the voices, the default voice, the tags and the formats.
	Speech  *SpeechView
	Timing  json.RawMessage `json:"timing"`
	License License         `json:"license"`
}

// SpeechView is what describe_workflow shows of a speech workflow, in the order
// of the manifest keys.
type SpeechView struct {
	Text         *Object         `json:"text"`
	Voices       json.RawMessage `json:"voices"`
	DefaultVoice json.RawMessage `json:"default_voice"`
	Tags         json.RawMessage `json:"tags"`
	Formats      json.RawMessage `json:"formats"`
}

// MarshalJSON writes the keys in the order of catalog.detail:
// the summary keys (output replaced by the full output), subject, timing, license.
func (d Detail) MarshalJSON() ([]byte, error) {
	o := NewObject()
	sum, err := marshal(d.Summary)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(sum, o); err != nil {
		return nil, err
	}
	type kv struct {
		k string
		v any
	}
	rows := []kv{{"output", d.Output}, {"subject", d.Subject}}
	if d.Speech != nil {
		rows = []kv{{"output", nil}, {"subject", nil}, {"text", d.Speech.Text}, {"voices", d.Speech.Voices},
			{"default_voice", d.Speech.DefaultVoice}, {"tags", d.Speech.Tags}, {"formats", d.Speech.Formats}}
	}
	rows = append(rows, kv{"timing", d.Timing}, kv{"license", d.License})
	for _, r := range rows {
		if err := o.Set(r.k, r.v); err != nil {
			return nil, err
		}
	}
	return o.MarshalJSON()
}

// DetailOf returns the full view of a workflow.
func DetailOf(w *Workflow) (Detail, error) {
	sum, err := SummaryOf(w)
	if err != nil {
		return Detail{}, err
	}
	m := w.Manifest
	if w.Kind() == KindSpeech {
		text, err := m.Object("text")
		if err != nil {
			return Detail{}, fmt.Errorf("manifest.json of %s: %w", w.Name, err)
		}
		lic, err := w.License()
		if err != nil {
			return Detail{}, err
		}
		return Detail{Summary: sum, Timing: orNull(m.Raw("timing")), License: lic, Speech: &SpeechView{
			Text: text, Voices: m.Raw("voices"), DefaultVoice: m.Raw("default_voice"), Tags: m.Raw("tags"), Formats: m.Raw("formats"),
		}}, nil
	}
	sum.Output = orNull(m.Raw("output"))
	subject, err := m.Object("subject")
	if err != nil || subject == nil {
		return Detail{}, fmt.Errorf("manifest.json of %s: subject is not an object", w.Name)
	}
	var rawExamples []json.RawMessage
	if subject.Has("examples") {
		if err := subject.Decode("examples", &rawExamples); err != nil {
			return Detail{}, fmt.Errorf("manifest.json of %s: subject.examples: %w", w.Name, err)
		}
	}
	examples := []*Object{}
	for _, raw := range rawExamples {
		var ex Object
		if err := json.Unmarshal(raw, &ex); err != nil {
			return Detail{}, fmt.Errorf("manifest.json of %s: subject.examples: %w", w.Name, err)
		}
		var image *string
		var rel any
		_ = ex.Decode("image", &rel)
		if s, ok := rel.(string); ok && s != "" {
			p := filepath.Join(w.Dir, s)
			if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
				image = &p
			}
		}
		if err := ex.Set("image", image); err != nil {
			return Detail{}, err
		}
		examples = append(examples, &ex)
	}
	subj := subject.Copy()
	if err := subj.Set("examples", examples); err != nil {
		return Detail{}, err
	}
	lic, err := w.License()
	if err != nil {
		return Detail{}, err
	}
	return Detail{Summary: sum, Subject: subj, Timing: orNull(m.Raw("timing")), License: lic}, nil
}

// pyRepr writes a value as Python's repr does, for the messages about a manifest:
// a string in single quotes (double quotes when it holds a single quote and no
// double quote), other values as JSON.
func pyRepr(v any) string {
	s, ok := v.(string)
	if !ok {
		if v == nil {
			return "None"
		}
		raw, _ := json.Marshal(v)
		return string(raw)
	}
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteByte(quote)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}
