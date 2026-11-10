// JSON text in the form of Python's json.dumps with its defaults. Part of the
// port of horus/comfy.py, which puts json.dumps(reply) in backend_message.
package comfy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf16"
)

// pyDumpsValue marshals v and writes it as json.dumps(v) does: ", " and ": "
// between items, and every character outside ASCII as \uXXXX. The key order
// of v is kept.
func pyDumpsValue(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return pyDumps(raw)
}

func pyDumps(raw []byte) string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var b bytes.Buffer
	type frame struct {
		obj   bool
		count int
	}
	var stack []frame
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if d, ok := tok.(json.Delim); ok && (d == '}' || d == ']') {
			stack = stack[:len(stack)-1]
			b.WriteRune(rune(d))
			continue
		}
		if n := len(stack); n > 0 {
			top := &stack[n-1]
			if top.obj {
				if top.count%2 == 0 && top.count > 0 {
					b.WriteString(", ")
				} else if top.count%2 == 1 {
					b.WriteString(": ")
				}
			} else if top.count > 0 {
				b.WriteString(", ")
			}
			top.count++
		}
		switch t := tok.(type) {
		case json.Delim:
			b.WriteRune(rune(t))
			stack = append(stack, frame{obj: t == '{'})
		case string:
			writeASCII(&b, t)
		case json.Number:
			b.WriteString(t.String())
		case bool:
			fmt.Fprint(&b, t)
		case nil:
			b.WriteString("null")
		}
	}
	return b.String()
}

func writeASCII(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r < 0x20 || (r >= 0x7f && r <= 0xffff):
			fmt.Fprintf(b, `\u%04x`, r)
		case r > 0xffff:
			r1, r2 := utf16.EncodeRune(r)
			fmt.Fprintf(b, `\u%04x\u%04x`, r1, r2)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
}
