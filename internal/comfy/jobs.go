// The mapping of rejected graphs and failed jobs to horus errors, and the
// time that a job took.
package comfy

import (
	"encoding/json"
	"strings"

	"github.com/hieplam/horus/internal/catalog"
	"github.com/hieplam/horus/internal/graph"
	"github.com/hieplam/horus/internal/herr"
)

func ptr[T any](v T) *T { return &v }

// rejectedGraphError maps a /prompt rejection to a horus error. A model file
// that the server does not list means the workflow is not installed (or the
// backend has not seen the new file).
func rejectedGraphError(reply *catalog.Object) *herr.Error {
	text := graph.PyDumps(reply)
	missing := []string{}
	var nodeErrors catalog.Object
	if json.Unmarshal(reply.Raw("node_errors"), &nodeErrors) == nil {
		for _, k := range nodeErrors.Keys() {
			var node struct {
				Errors []struct {
					Type      string                     `json:"type"`
					ExtraInfo map[string]json.RawMessage `json:"extra_info"`
				} `json:"errors"`
			}
			if json.Unmarshal(nodeErrors.Raw(k), &node) != nil {
				continue
			}
			for _, e := range node.Errors {
				value := ""
				if raw, ok := e.ExtraInfo["received_value"]; ok {
					value = graph.PyStr(raw)
				}
				if e.Type == "value_not_in_list" && graph.IsModelFile(value) {
					missing = append(missing, value)
				}
			}
		}
	}
	if len(missing) > 0 {
		return herr.New("WORKFLOW_NOT_INSTALLED", "the backend does not list model files of this workflow", map[string]any{"missing": missing})
	}
	e := herr.New("GENERATION_FAILED", "ComfyUI rejected the graph", map[string]any{"backend_message": headRunes(text, 1500)})
	e.Retryable = ptr(false)
	e.NextStep = "This is a defect of the workflow, not of your call. Report details.backend_message to the user."
	return e
}

// messages reads the status messages [[name, data], ...] into a map. A later
// message with the same name wins.
func messages(raw []json.RawMessage) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for _, m := range raw {
		var pair []json.RawMessage
		if json.Unmarshal(m, &pair) != nil || len(pair) != 2 {
			continue
		}
		var name string
		if json.Unmarshal(pair[0], &name) != nil {
			continue
		}
		out[name] = pair[1]
	}
	return out
}

// jobError maps a failed job in /history to a horus error.
func jobError(promptID string, status jobStatus) *herr.Error {
	msgs := messages(status.Messages)
	if _, ok := msgs["execution_interrupted"]; ok {
		return herr.New("CANCELLED", "the job was interrupted on the backend", map[string]any{"job_id": promptID})
	}
	var info map[string]json.RawMessage
	_ = json.Unmarshal(msgs["execution_error"], &info)
	str := func(key string) string {
		raw, ok := info[key]
		if !ok {
			return ""
		}
		return graph.PyStr(raw)
	}
	etype, emsg := str("exception_type"), str("exception_message")
	var nodeType any
	if raw, ok := info["node_type"]; ok {
		_ = json.Unmarshal(raw, &nodeType)
	}
	facts := map[string]any{"job_id": promptID, "node_type": nodeType, "exception_type": etype,
		"backend_message": tailRunes(emsg, 1500)}
	low := strings.ToLower(etype + " " + emsg)
	if strings.Contains(strings.ReplaceAll(low, " ", ""), "outofmemory") || strings.Contains(low, "out of memory") ||
		strings.Contains(low, "allocation on device") {
		return herr.New("OUT_OF_MEMORY", "the GPU ran out of memory", facts)
	}
	if strings.Contains(low, "cuda driver version is insufficient") {
		e := herr.New("GENERATION_FAILED", "the NVIDIA driver is too old for a kernel of this workflow", facts)
		e.Retryable = ptr(false)
		e.NextStep = "Ask the user to update the NVIDIA driver (docs/why-flux3-klein.md in the horus repo)."
		return e
	}
	return herr.New("GENERATION_FAILED", "the backend failed while it ran the job", facts)
}

// executionSeconds is the time from execution_start to execution_success,
// rounded to 0.1 s, or nil when the entry lacks one of them.
func executionSeconds(e *historyEntry) *float64 {
	stamps := map[string]float64{}
	for name, raw := range messages(e.Status.Messages) {
		var data map[string]json.RawMessage
		if json.Unmarshal(raw, &data) != nil {
			continue
		}
		var ts float64
		if json.Unmarshal(data["timestamp"], &ts) == nil {
			stamps[name] = ts
		}
	}
	start, done := stamps["execution_start"], stamps["execution_success"]
	if start == 0 || done == 0 {
		return nil
	}
	return ptr(graph.Round1((done - start) / 1000))
}
