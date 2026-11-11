// The arguments of a tool call: checked against the fields of the tool.
package mcpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// args holds the checked arguments. A missing optional argument holds its default.
type args map[string]any

func (a args) str(name string) string {
	s, _ := a[name].(string)
	return s
}

func (a args) strPtr(name string) *string {
	if s, ok := a[name].(string); ok {
		return &s
	}
	return nil
}

func (a args) boolean(name string) bool {
	b, _ := a[name].(bool)
	return b
}

func (a args) intPtr(name string) *int64 {
	if n, ok := a[name].(int64); ok {
		return &n
	}
	return nil
}

// parseArgs checks the raw arguments against the fields. Unknown arguments
// are ignored. The problems are returned as lines
// "<field>\n  <message>".
func parseArgs(raw json.RawMessage, fields []field) (args, []string) {
	in := map[string]json.RawMessage{}
	if t := bytes.TrimSpace(raw); len(t) > 0 && string(t) != "null" {
		if err := json.Unmarshal(t, &in); err != nil {
			return nil, []string{"arguments\n  Input should be an object"}
		}
	}
	out := args{}
	var problems []string
	for _, x := range fields {
		v, present := in[x.name]
		if !present {
			if !x.optional {
				problems = append(problems, x.name+"\n  Field required")
				continue
			}
			out[x.name] = x.def
			continue
		}
		val, msg := checkValue(v, x)
		if msg != "" {
			problems = append(problems, x.name+"\n  "+msg)
			continue
		}
		out[x.name] = val
	}
	return out, problems
}

func checkValue(raw json.RawMessage, x field) (any, string) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, "Input is not valid JSON"
	}
	if v == nil {
		if x.nullable {
			return nil, ""
		}
	}
	switch x.base["type"] {
	case "string":
		if s, ok := v.(string); ok {
			return s, ""
		}
		return nil, "Input should be a valid string"
	case "boolean":
		if b, ok := v.(bool); ok {
			return b, ""
		}
		return nil, "Input should be a valid boolean"
	case "integer":
		if n, ok := v.(json.Number); ok {
			if i, err := n.Int64(); err == nil {
				return i, ""
			}
			if b, ok := new(big.Int).SetString(n.String(), 10); ok {
				return b, "" // a whole number of any size is kept; the tool checks the range
			}
			if fl, err := n.Float64(); err == nil && fl == math.Trunc(fl) && math.Abs(fl) < 1<<53 {
				return int64(fl), ""
			}
			return nil, "Input should be a valid integer"
		}
		return nil, "Input should be a valid integer"
	}
	return v, ""
}

// argsError is the answer of the server to arguments that fail the
// check: isError, no structuredContent, and the problems as text.
func argsError(tool string, problems []string) *mcp.CallToolResult {
	n := len(problems)
	word := "error"
	if n > 1 {
		word = "errors"
	}
	text := fmt.Sprintf("Error executing tool %s: %d validation %s for %sArguments\n%s",
		tool, n, word, tool, strings.Join(problems, "\n"))
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: true}
}
