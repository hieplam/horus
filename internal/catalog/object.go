// Object: a JSON object that keeps the order of its keys, as a Python dict
// does. Part of the port of horus/catalog.py.
package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Object is a JSON object that keeps the order of its keys.
type Object struct {
	keys []string
	vals map[string]json.RawMessage
}

// NewObject makes an empty Object.
func NewObject() *Object { return &Object{vals: map[string]json.RawMessage{}} }

// Has reports if the key is present.
func (o *Object) Has(key string) bool {
	_, ok := o.vals[key]
	return ok
}

// Keys returns the keys in order.
func (o *Object) Keys() []string { return append([]string(nil), o.keys...) }

// Raw returns the JSON value of a key, or nil.
func (o *Object) Raw(key string) json.RawMessage { return o.vals[key] }

// Decode reads the value of a key into v. A missing key is an error.
func (o *Object) Decode(key string, v any) error {
	raw, ok := o.vals[key]
	if !ok {
		return fmt.Errorf("missing key %q", key)
	}
	return json.Unmarshal(raw, v)
}

// Object returns the value of a key as an Object. A missing key or a null
// value gives nil and no error.
func (o *Object) Object(key string) (*Object, error) {
	raw, ok := o.vals[key]
	if !ok || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	var sub Object
	if err := json.Unmarshal(raw, &sub); err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return &sub, nil
}

// Set puts a value at its old place, or at the end for a new key.
func (o *Object) Set(key string, v any) error {
	raw, err := marshal(v)
	if err != nil {
		return err
	}
	if o.vals == nil {
		o.vals = map[string]json.RawMessage{}
	}
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = raw
	return nil
}

// Copy returns a shallow copy.
func (o *Object) Copy() *Object {
	c := NewObject()
	for _, k := range o.keys {
		c.keys = append(c.keys, k)
		c.vals[k] = o.vals[k]
	}
	return c
}

// UnmarshalJSON reads an object and keeps the key order. A repeated key keeps
// its first place and its last value, as json.loads does.
func (o *Object) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("not a JSON object")
	}
	o.keys, o.vals = nil, map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, _ := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		if _, seen := o.vals[key]; !seen {
			o.keys = append(o.keys, key)
		}
		o.vals[key] = raw
	}
	_, err = dec.Token()
	return err
}

// MarshalJSON writes the keys in order.
func (o *Object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(o.vals[k])
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshal is json.Marshal without HTML escapes, as json.dumps writes "<" and "&".
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}
