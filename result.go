package jsonldb

import (
	"encoding/json"
	"sort"
)

// Result is a chainable view over a filtered set of Docs.
type Result struct {
	docs []Doc
}

func (r *Result) Docs() []Doc { return r.docs }
func (r *Result) Count() int  { return len(r.docs) }

func (r *Result) First() (Doc, bool) {
	if len(r.docs) == 0 {
		return Doc{}, false
	}
	return r.docs[0], true
}

func (r *Result) Last() (Doc, bool) {
	if len(r.docs) == 0 {
		return Doc{}, false
	}
	return r.docs[len(r.docs)-1], true
}

func (r *Result) GroupByFunc(fn func(Doc) string) map[string]*Result {
	out := map[string]*Result{}
	for _, d := range r.docs {
		k := fn(d)
		g := out[k]
		if g == nil {
			g = &Result{}
			out[k] = g
		}
		g.docs = append(g.docs, d)
	}
	return out
}

func (r *Result) GroupBy(field string) map[string]*Result {
	return r.GroupByFunc(func(d Doc) string {
		if v, ok := d.Get(field); ok {
			return valueKey(v)
		}
		return ""
	})
}

func (r *Result) CountBy(field string) map[string]int {
	out := map[string]int{}
	for _, d := range r.docs {
		if v, ok := d.Get(field); ok {
			out[valueKey(v)]++
		}
	}
	return out
}

func (r *Result) Distinct(field string) []any {
	var out []any
	seen := map[string]bool{}
	for _, d := range r.docs {
		if v, ok := d.Get(field); ok {
			k := valueKey(v)
			if !seen[k] {
				seen[k] = true
				out = append(out, v)
			}
		}
	}
	return out
}

func (r *Result) floats(field string) []float64 {
	var out []float64
	for _, d := range r.docs {
		if f, ok := d.GetFloat(field); ok {
			out = append(out, f)
		}
	}
	return out
}

func (r *Result) Sum(field string) (float64, bool) {
	fs := r.floats(field)
	if len(fs) == 0 {
		return 0, false
	}
	s := 0.0
	for _, f := range fs {
		s += f
	}
	return s, true
}

func (r *Result) Avg(field string) (float64, bool) {
	fs := r.floats(field)
	if len(fs) == 0 {
		return 0, false
	}
	s := 0.0
	for _, f := range fs {
		s += f
	}
	return s / float64(len(fs)), true
}

func (r *Result) Min(field string) (float64, bool) {
	fs := r.floats(field)
	if len(fs) == 0 {
		return 0, false
	}
	m := fs[0]
	for _, f := range fs[1:] {
		if f < m {
			m = f
		}
	}
	return m, true
}

func (r *Result) Max(field string) (float64, bool) {
	fs := r.floats(field)
	if len(fs) == 0 {
		return 0, false
	}
	m := fs[0]
	for _, f := range fs[1:] {
		if f > m {
			m = f
		}
	}
	return m, true
}

// valueKey renders a value as a stable string group key.
func valueKey(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// SortBy returns a new Result sorted by field using the coercion ladder.
// Docs missing the field sort last.
func (r *Result) SortBy(field string, desc bool) *Result {
	cp := make([]Doc, len(r.docs))
	copy(cp, r.docs)
	sort.SliceStable(cp, func(i, j int) bool {
		vi, oki := cp[i].Get(field)
		vj, okj := cp[j].Get(field)
		if !oki || !okj {
			if oki != okj {
				return oki // present sorts before missing
			}
			return false
		}
		c, ok := compareValues(vi, vj)
		if !ok {
			return false
		}
		if desc {
			return c > 0
		}
		return c < 0
	})
	return &Result{docs: cp}
}

func (r *Result) Limit(n int) *Result {
	if n < 0 {
		n = 0
	}
	if n > len(r.docs) {
		n = len(r.docs)
	}
	return &Result{docs: r.docs[:n]}
}

func (r *Result) Offset(n int) *Result {
	if n < 0 {
		n = 0
	}
	if n > len(r.docs) {
		n = len(r.docs)
	}
	return &Result{docs: r.docs[n:]}
}

// Page returns 1-based page num of the given size.
func (r *Result) Page(num, size int) *Result {
	if num < 1 || size < 1 {
		return &Result{}
	}
	return r.Offset((num - 1) * size).Limit(size)
}
