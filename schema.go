package jsonldb

import (
	"encoding/json"
	"sort"
)

type FieldInfo struct {
	Key      string
	Types    []string
	Presence float64
	Sample   any
}

type Schema []FieldInfo

func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case json.Number, float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

// Schema infers per-key types, presence, and a sample value across all docs.
func (c *Collection) Schema() Schema {
	total := float64(len(c.index))
	type acc struct {
		count  int
		types  map[string]bool
		sample any
	}
	accs := map[string]*acc{}
	for i := range c.index {
		d, ok := c.mustDoc(i)
		if !ok {
			continue
		}
		for k, v := range d.m {
			a := accs[k]
			if a == nil {
				a = &acc{types: map[string]bool{}, sample: v}
				accs[k] = a
			}
			a.count++
			a.types[jsonType(v)] = true
		}
	}
	out := make(Schema, 0, len(accs))
	for k, a := range accs {
		types := make([]string, 0, len(a.types))
		for ty := range a.types {
			types = append(types, ty)
		}
		sort.Strings(types)
		presence := 0.0
		if total > 0 {
			presence = float64(a.count) / total
		}
		out = append(out, FieldInfo{Key: k, Types: types, Presence: presence, Sample: a.sample})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Presence != out[j].Presence {
			return out[i].Presence > out[j].Presence
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// Keys returns the sorted union of all keys.
func (c *Collection) Keys() []string {
	set := map[string]bool{}
	for i := range c.index {
		if d, ok := c.mustDoc(i); ok {
			for k := range d.m {
				set[k] = true
			}
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Sample returns up to n docs from the head.
func (c *Collection) Sample(n int) []Doc {
	if n < 0 {
		n = 0
	}
	if n > len(c.index) {
		n = len(c.index)
	}
	out := make([]Doc, 0, n)
	for i := 0; i < n; i++ {
		if d, ok := c.mustDoc(i); ok {
			out = append(out, d)
		}
	}
	return out
}
