// Package graph converts a ComfyUI graph from the UI format to the API format,
// and finds the model files of a graph that the server does not list. It is
// pure: no input and no output.
//
// It also holds the data types that internal/generate (the service) and
// internal/comfy (the adapter) both need: the API graph, the node
// information of the server, and the result of a job (job.go).
package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/herr"
)

// simpleTypes are the input types that a widget holds. Any other type is a
// connection (MODEL, CLIP and so on).
var simpleTypes = map[string]bool{"INT": true, "FLOAT": true, "STRING": true, "BOOLEAN": true, "COMBO": true}

// noteTypes are node classes that only hold text for people.
var noteTypes = map[string]bool{"Note": true, "MarkdownNote": true}

// modelExt are the file name endings of model files.
var modelExt = []string{".safetensors", ".sft", ".gguf", ".ckpt", ".pt", ".pth", ".bin", ".onnx"}

// IsModelFile reports if the name s ends with one of modelExt.
func IsModelFile(s string) bool {
	low := strings.ToLower(s)
	for _, e := range modelExt {
		if strings.HasSuffix(low, e) {
			return true
		}
	}
	return false
}

// ObjectInfo is /object_info: for each node class, its definition as JSON.
type ObjectInfo map[string]json.RawMessage

// spec is one input of a node class: its name and its JSON definition.
type spec struct {
	name string
	raw  json.RawMessage
}

// specs returns {**required, **optional} of a node class, in that order. A
// name in both keeps its first place and the optional value.
func (o ObjectInfo) specs(class string) ([]spec, error) {
	var node struct {
		Input struct {
			Required *catalog.Object `json:"required"`
			Optional *catalog.Object `json:"optional"`
		} `json:"input"`
	}
	if err := json.Unmarshal(o[class], &node); err != nil {
		return nil, fmt.Errorf("object_info of %s: %w", class, err)
	}
	var out []spec
	at := map[string]int{}
	for _, part := range []*catalog.Object{node.Input.Required, node.Input.Optional} {
		if part == nil {
			continue
		}
		for _, k := range part.Keys() {
			if i, ok := at[k]; ok {
				out[i].raw = part.Raw(k)
				continue
			}
			at[k] = len(out)
			out = append(out, spec{k, part.Raw(k)})
		}
	}
	return out, nil
}

// API is a graph in API format: node id -> node, in the order of the UI graph.
type API struct {
	ids   []string
	nodes map[string]*Node
}

// Node is one node of the API graph.
type Node struct {
	ClassType string          `json:"class_type"`
	Inputs    *catalog.Object `json:"inputs"`
}

// Len is the number of nodes.
func (a *API) Len() int { return len(a.ids) }

// IDs returns the node ids in order.
func (a *API) IDs() []string { return append([]string(nil), a.ids...) }

// Node returns one node, or nil.
func (a *API) Node(id string) *Node { return a.nodes[id] }

func (a *API) add(id string, n *Node) {
	if a.nodes == nil {
		a.nodes = map[string]*Node{}
	}
	if _, ok := a.nodes[id]; !ok {
		a.ids = append(a.ids, id)
	}
	a.nodes[id] = n
}

// MarshalJSON writes the nodes in order.
func (a *API) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, id := range a.ids {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(id)
		v, err := json.Marshal(a.nodes[id])
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// UnmarshalJSON reads a graph in API format and keeps the node order.
func (a *API) UnmarshalJSON(data []byte) error {
	var o catalog.Object
	if err := json.Unmarshal(data, &o); err != nil {
		return err
	}
	*a = API{}
	for _, k := range o.Keys() {
		var n Node
		if err := json.Unmarshal(o.Raw(k), &n); err != nil {
			return err
		}
		if n.Inputs == nil {
			n.Inputs = catalog.NewObject()
		}
		a.add(k, &n)
	}
	return nil
}

// uiGraph is the part of a UI-format graph that the conversion reads.
type uiGraph struct {
	Nodes []struct {
		ID     json.RawMessage `json:"id"`
		Type   string          `json:"type"`
		Inputs []struct {
			Name string          `json:"name"`
			Link json.RawMessage `json:"link"`
		} `json:"inputs"`
		WidgetsValues []json.RawMessage `json:"widgets_values"`
	} `json:"nodes"`
	Links [][]json.RawMessage `json:"links"`
}

// PyStr is str(v) of Python for a JSON scalar: a string as is, a number as
// written, null as None, true and false as True and False.
func PyStr(raw json.RawMessage) string {
	t := strings.TrimSpace(string(raw))
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	switch t {
	case "null", "":
		return "None"
	case "true":
		return "True"
	case "false":
		return "False"
	}
	return t
}

// UIToAPI converts a ComfyUI UI-format graph to the API format.
//
// For each node, the widget values (widgets_values) are matched to the input
// names in the order that the server lists them in /object_info. Linked inputs
// become [source_node_id, source_slot].
func UIToAPI(workflow []byte, info ObjectInfo) (*API, error) {
	var g uiGraph
	dec := json.NewDecoder(bytes.NewReader(workflow))
	dec.UseNumber()
	if err := dec.Decode(&g); err != nil {
		return nil, fmt.Errorf("workflow.json: %w", err)
	}
	links := map[string][]json.RawMessage{}
	for _, l := range g.Links {
		if len(l) > 0 {
			links[strings.TrimSpace(string(l[0]))] = l
		}
	}
	missingSet := map[string]bool{}
	for _, n := range g.Nodes {
		if _, ok := info[n.Type]; !ok && !noteTypes[n.Type] {
			missingSet[n.Type] = true
		}
	}
	if len(missingSet) > 0 {
		missing := make([]string, 0, len(missingSet))
		for k := range missingSet {
			missing = append(missing, k)
		}
		sort.Strings(missing)
		return nil, herr.New("NODE_PACK_MISSING", "the backend lacks node classes of this workflow", map[string]any{"missing": missing})
	}
	api := &API{}
	for _, n := range g.Nodes {
		if noteTypes[n.Type] {
			continue
		}
		inputs := catalog.NewObject()
		linked := map[string]bool{}
		for _, slot := range n.Inputs {
			if t := strings.TrimSpace(string(slot.Link)); t == "" || t == "null" {
				continue
			}
			link, ok := links[strings.TrimSpace(string(slot.Link))]
			if !ok || len(link) < 3 {
				return nil, fmt.Errorf("node %s: link %s is not in the link table", PyStr(n.ID), slot.Link)
			}
			if err := inputs.Set(slot.Name, []any{PyStr(link[1]), link[2]}); err != nil {
				return nil, err
			}
			linked[slot.Name] = true
		}
		values := append([]json.RawMessage(nil), n.WidgetsValues...)
		specs, err := info.specs(n.Type)
		if err != nil {
			return nil, err
		}
		for _, sp := range specs {
			var parts []json.RawMessage
			if err := json.Unmarshal(sp.raw, &parts); err != nil || len(parts) == 0 {
				return nil, fmt.Errorf("object_info of %s: input %s is not a list", n.Type, sp.name)
			}
			var kind string
			isList := strings.HasPrefix(strings.TrimSpace(string(parts[0])), "[")
			if !isList {
				_ = json.Unmarshal(parts[0], &kind)
				if !simpleTypes[kind] {
					continue // a connection type such as MODEL or CLIP, never a widget
				}
			}
			var opts struct {
				ControlAfterGenerate json.RawMessage `json:"control_after_generate"`
			}
			if len(parts) > 1 {
				_ = json.Unmarshal(parts[1], &opts)
			}
			if len(values) == 0 {
				break
			}
			value := values[0]
			values = values[1:]
			if Truthy(opts.ControlAfterGenerate) && len(values) > 0 {
				values = values[1:] // the UI stores "fixed" or "randomize" after the seed
			}
			if !linked[sp.name] {
				if err := inputs.Set(sp.name, value); err != nil {
					return nil, err
				}
			}
		}
		api.add(PyStr(n.ID), &Node{ClassType: n.Type, Inputs: inputs})
	}
	return api, nil
}

// MissingModelFiles returns the model file names in the graph that the server
// does not list in its options.
func MissingModelFiles(api *API, info ObjectInfo) ([]string, error) {
	missing := []string{}
	for _, id := range api.ids {
		node := api.nodes[id]
		specs, err := info.specs(node.ClassType)
		if err != nil {
			return nil, err
		}
		byName := map[string]json.RawMessage{}
		for _, sp := range specs {
			byName[sp.name] = sp.raw
		}
		for _, name := range node.Inputs.Keys() {
			var value string
			if json.Unmarshal(node.Inputs.Raw(name), &value) != nil || !IsModelFile(value) {
				continue
			}
			options, ok := comboOptions(byName[name])
			if !ok {
				continue
			}
			found := false
			for _, o := range options {
				if s, isStr := o.(string); isStr && s == value {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, value)
			}
		}
	}
	return missing, nil
}

// comboOptions returns the allowed values of a COMBO input: [[a, b], {...}]
// (old form) or ["COMBO", {"options": [a, b]}].
func comboOptions(raw json.RawMessage) ([]any, bool) {
	var parts []json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &parts) != nil || len(parts) == 0 {
		return nil, false
	}
	var list []any
	if json.Unmarshal(parts[0], &list) == nil && strings.HasPrefix(strings.TrimSpace(string(parts[0])), "[") {
		return list, true
	}
	if len(parts) > 1 {
		var opts struct {
			Options json.RawMessage `json:"options"`
		}
		if json.Unmarshal(parts[1], &opts) == nil && strings.HasPrefix(strings.TrimSpace(string(opts.Options)), "[") {
			if json.Unmarshal(opts.Options, &list) == nil {
				return list, true
			}
		}
	}
	return nil, false
}
