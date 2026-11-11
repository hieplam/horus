// JSON schemas of the tools, written as pydantic writes them for the result
// models and the arguments of horus/server.py. Part of the port of
// horus/server.py. The test compares them with testdata/tools.golden.json.
package mcpserver

import "strings"

type obj = map[string]any

// field is one property of an object schema.
type field struct {
	name     string
	base     obj
	desc     string
	nullable bool // anyOf [base, null]
	optional bool // not in "required"
	def      any
	hasDef   bool // write "default"
}

func f(name string, base obj) field { return field{name: name, base: base} }

// d sets the description.
func (x field) d(desc string) field { x.desc = desc; return x }

// null makes the value nullable: anyOf [base, {"type": "null"}].
func (x field) null() field { x.nullable = true; return x }

// dflt makes the field optional with a default value.
func (x field) dflt(v any) field { x.optional, x.def, x.hasDef = true, v, true; return x }

// factory makes the field optional without a "default" key, as pydantic
// writes a field with default_factory.
func (x field) factory() field { x.optional = true; return x }

// title is the pydantic title of a field: "out_path" becomes "Out Path".
func title(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, " ")
}

func (x field) schema() obj {
	s := obj{}
	if x.nullable {
		s["anyOf"] = []any{x.base, obj{"type": "null"}}
	} else {
		for k, v := range x.base {
			s[k] = v
		}
	}
	s["title"] = title(x.name)
	if x.desc != "" {
		s["description"] = x.desc
	}
	if x.hasDef {
		s["default"] = x.def
	}
	return s
}

// model is the object schema of a pydantic model or of the arguments of a tool.
func model(name string, fields ...field) obj {
	props := obj{}
	required := []any{}
	for _, x := range fields {
		props[x.name] = x.schema()
		if !x.optional {
			required = append(required, x.name)
		}
	}
	m := obj{"properties": props, "title": name, "type": "object"}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

var (
	tString = obj{"type": "string"}
	tInt    = obj{"type": "integer"}
	tNumber = obj{"type": "number"}
	tBool   = obj{"type": "boolean"}
	tDict   = obj{"additionalProperties": true, "type": "object"}
)

func tList(items obj) obj { return obj{"items": items, "type": "array"} }
func ref(name string) obj { return obj{"$ref": "#/$defs/" + name} }
func tConst(v string) obj { return obj{"const": v, "type": "string"} }

func tEnum(values ...string) obj {
	vs := make([]any, len(values))
	for i, v := range values {
		vs[i] = v
	}
	return obj{"enum": vs, "type": "string"}
}

var tInstallState = tEnum("installed", "partially_installed", "not_installed")

// errorInfoModel is ErrorInfo of server.py.
var errorInfoModel = model("ErrorInfo",
	f("code", tString).d("Stable error code, for example WORKFLOW_NOT_INSTALLED. See docs/mcp.md."),
	f("message", tString).d("What went wrong, for a person."),
	f("retryable", tBool).d("True: the same call can pass when it is sent again."),
	f("action_by", tEnum("caller", "retry", "human")).d(
		"caller: change the arguments. retry: send the same call again. human: the user must act first."),
	f("next_step", tString).d("The exact thing to do next."),
	f("details", tDict).d("Facts: file names, allowed values, backend message.").factory(),
)

// installFileModel is InstallFile of server.py.
var installFileModel = model("InstallFile",
	f("role", tString),
	f("file", tString),
	f("bytes", tInt),
	f("state", tString).d("present, missing, wrong_size, downloaded, deleted, would_delete, kept_shared, absent"),
	f("path", tString).null().dflt(nil),
	f("detail", tString).null().dflt(nil),
)

// envelope is the output schema Envelope[<result>] of server.py. defs holds
// the result model and the models that it refers to.
func envelope(result string, defs obj) obj {
	all := obj{"ErrorInfo": errorInfoModel}
	for k, v := range defs {
		all[k] = v
	}
	return obj{
		"$defs": all,
		"properties": obj{
			"error":  obj{"anyOf": []any{ref("ErrorInfo"), obj{"type": "null"}}, "default": nil},
			"ok":     obj{"title": "Ok", "type": "boolean"},
			"result": obj{"anyOf": []any{ref(result), obj{"type": "null"}}, "default": nil},
		},
		"required": []any{"ok"},
		"title":    "Envelope[" + result + "]",
		"type":     "object",
	}
}
