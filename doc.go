package jsonldb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Doc is the dynamic record: a value-bag plus metadata for lossless rewrites.
type Doc struct {
	m    map[string]any
	raw  []byte
	line int
}

// NewDoc builds a Doc from a map. raw is derived on marshal; line is 0.
func NewDoc(m map[string]any) Doc { return Doc{m: m} }

// parseDoc decodes one JSONL line, preserving raw bytes and 1-based line number.
func parseDoc(raw []byte, line int) (Doc, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	m := map[string]any{}
	if err := dec.Decode(&m); err != nil {
		return Doc{}, err
	}
	cp := make([]byte, len(raw))
	copy(cp, raw)
	return Doc{m: m, raw: cp, line: line}, nil
}

func (d Doc) Get(key string) (any, bool) { v, ok := d.m[key]; return v, ok }
func (d Doc) Has(key string) bool        { _, ok := d.m[key]; return ok }
func (d Doc) Line() int                  { return d.line }

func (d Doc) Raw() []byte {
	if d.raw != nil {
		return d.raw
	}
	b, _ := d.MarshalJSON()
	return b
}

func (d Doc) GetString(key string) string {
	s, _ := d.m[key].(string)
	return s
}

func (d Doc) GetInt(key string) (int64, bool) {
	switch n := d.m[key].(type) {
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case float64:
		return int64(n), true
	}
	return 0, false
}

func (d Doc) GetFloat(key string) (float64, bool) {
	switch n := d.m[key].(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	}
	return 0, false
}

func (d Doc) GetBool(key string) bool {
	b, _ := d.m[key].(bool)
	return b
}

func (d Doc) GetTime(key string) (time.Time, bool) {
	s := d.GetString(key)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}

// Path walks nested objects/arrays via dotted keys + numeric indices.
func (d Doc) Path(dotted string) (any, bool) {
	var cur any = d.m
	for _, p := range strings.Split(dotted, ".") {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[p]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(p)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// getField resolves a key: dotted keys ("message.role", "notes.0.text") go
// through Path; plain keys through Get. Used by queries, sort, and aggregation
// so nested fields work everywhere top-level keys do.
func (d Doc) getField(key string) (any, bool) {
	if strings.Contains(key, ".") {
		return d.Path(key)
	}
	return d.Get(key)
}

func (d Doc) getStringField(key string) string {
	if !strings.Contains(key, ".") {
		return d.GetString(key)
	}
	if v, ok := d.Path(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (d Doc) getFloatField(key string) (float64, bool) {
	if !strings.Contains(key, ".") {
		return d.GetFloat(key)
	}
	v, ok := d.Path(key)
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	}
	return 0, false
}

// MarshalJSON emits m in stable (sorted) key order for deterministic rewrites.
func (d Doc) MarshalJSON() ([]byte, error) {
	keys := make([]string, 0, len(d.m))
	for k := range d.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(d.m[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// coerce normalizes a value to a comparable form: float64, time.Time, bool, string, or nil.
func coerce(v any) any {
	switch x := v.(type) {
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case bool:
		return x
	case string:
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			return t
		}
		return x
	case nil:
		return nil
	}
	return v
}

// compareValues returns -1/0/1 and ok=true for ordered types; ok=false if incomparable.
func compareValues(a, b any) (int, bool) {
	ca, cb := coerce(a), coerce(b)
	switch av := ca.(type) {
	case float64:
		bv, ok := cb.(float64)
		if !ok {
			return 0, false
		}
		switch {
		case av < bv:
			return -1, true
		case av > bv:
			return 1, true
		default:
			return 0, true
		}
	case time.Time:
		bv, ok := cb.(time.Time)
		if !ok {
			return 0, false
		}
		switch {
		case av.Before(bv):
			return -1, true
		case av.After(bv):
			return 1, true
		default:
			return 0, true
		}
	case string:
		bv, ok := cb.(string)
		if !ok {
			return 0, false
		}
		return strings.Compare(av, bv), true
	}
	return 0, false
}

// sortRank groups coerced values into ordered type buckets so that a column
// mixing types still has a deterministic, sensible order (numbers, then dates,
// then strings, then bools, then anything else).
func sortRank(c any) int {
	switch c.(type) {
	case float64:
		return 0
	case time.Time:
		return 1
	case string:
		return 2
	case bool:
		return 3
	default:
		return 4
	}
}

// compareForSort returns a total order (-1/0/1) over arbitrary JSON values:
// first by type bucket, then within a bucket by value, falling back to the
// string form so the result is always deterministic (never an unstable 0).
func compareForSort(a, b any) int {
	ca, cb := coerce(a), coerce(b)
	if ra, rb := sortRank(ca), sortRank(cb); ra != rb {
		if ra < rb {
			return -1
		}
		return 1
	}
	if c, ok := compareValues(a, b); ok {
		return c
	}
	return strings.Compare(fmt.Sprintf("%v", ca), fmt.Sprintf("%v", cb))
}

// equalValues compares for equality across coerced types.
func equalValues(a, b any) bool {
	ca, cb := coerce(a), coerce(b)
	switch av := ca.(type) {
	case float64:
		bv, ok := cb.(float64)
		return ok && av == bv
	case time.Time:
		bv, ok := cb.(time.Time)
		return ok && av.Equal(bv)
	}
	return ca == cb
}
