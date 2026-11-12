package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// object is a JSON object that keeps its key order, so editing settings.json does not reshuffle the user's file.
type object []member

type member struct {
	key   string
	value json.RawMessage
}

func parseObject(data []byte) (object, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("expected a JSON object")
	}
	var o object
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		o.set(key, v)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return o, nil
}

func (o object) get(key string) (json.RawMessage, bool) {
	for _, m := range o {
		if m.key == key {
			return m.value, true
		}
	}
	return nil, false
}

func (o *object) set(key string, value json.RawMessage) {
	for i, m := range *o {
		if m.key == key {
			(*o)[i].value = value
			return
		}
	}
	*o = append(*o, member{key: key, value: value})
}

func (o *object) delete(key string) {
	for i, m := range *o {
		if m.key == key {
			*o = append((*o)[:i], (*o)[i+1:]...)
			return
		}
	}
}

// object returns the nested object at key, or an empty one when absent or null.
func (o object) object(key string) (object, error) {
	v, ok := o.get(key)
	if !ok || string(bytes.TrimSpace(v)) == "null" {
		return object{}, nil
	}
	nested, err := parseObject(v)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", key, err)
	}
	return nested, nil
}

func (o object) array(key string) ([]json.RawMessage, error) {
	v, ok := o.get(key)
	if !ok || string(bytes.TrimSpace(v)) == "null" {
		return nil, nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(v, &arr); err != nil {
		return nil, fmt.Errorf("%q: expected an array: %w", key, err)
	}
	return arr, nil
}

func (o *object) setArray(key string, arr []json.RawMessage) error {
	if arr == nil {
		arr = []json.RawMessage{}
	}
	data, err := marshal(arr)
	if err != nil {
		return err
	}
	o.set(key, data)
	return nil
}

func (o object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := marshal(m.key)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(m.value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshal encodes v without escaping <, > and &, which json.Marshal would rewrite in the user's hook commands.
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}
