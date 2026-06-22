package jsonldb

import (
	"encoding/json"
	"sort"
)

// Result is a chainable view over a filtered set of Docs, backed by index positions.
type Result struct {
	col *Collection
	idx []int
}

func (r *Result) Docs() []Doc {
	out := make([]Doc, 0, len(r.idx))
	for _, i := range r.idx {
		if d, ok := r.col.mustDoc(i); ok {
			out = append(out, d)
		}
	}
	return out
}

func (r *Result) Count() int { return len(r.idx) }

func (r *Result) First() (Doc, bool) {
	if len(r.idx) == 0 {
		return Doc{}, false
	}
	return r.col.mustDoc(r.idx[0])
}

func (r *Result) Last() (Doc, bool) {
	if len(r.idx) == 0 {
		return Doc{}, false
	}
	return r.col.mustDoc(r.idx[len(r.idx)-1])
}

func (r *Result) GroupByFunc(fn func(Doc) string) map[string]*Result {
	out := map[string]*Result{}
	for _, i := range r.idx {
		d, ok := r.col.mustDoc(i)
		if !ok {
			continue
		}
		k := fn(d)
		g := out[k]
		if g == nil {
			g = &Result{col: r.col}
			out[k] = g
		}
		g.idx = append(g.idx, i)
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
	for _, i := range r.idx {
		if d, ok := r.col.mustDoc(i); ok {
			if v, ok := d.Get(field); ok {
				out[valueKey(v)]++
			}
		}
	}
	return out
}

func (r *Result) Distinct(field string) []any {
	var out []any
	seen := map[string]bool{}
	for _, i := range r.idx {
		if d, ok := r.col.mustDoc(i); ok {
			if v, ok := d.Get(field); ok {
				k := valueKey(v)
				if !seen[k] {
					seen[k] = true
					out = append(out, v)
				}
			}
		}
	}
	return out
}

func (r *Result) floats(field string) []float64 {
	var out []float64
	for _, i := range r.idx {
		if d, ok := r.col.mustDoc(i); ok {
			if f, ok := d.GetFloat(field); ok {
				out = append(out, f)
			}
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
	idx := make([]int, len(r.idx))
	copy(idx, r.idx)
	sort.SliceStable(idx, func(a, b int) bool {
		da, oka := r.col.mustDoc(idx[a])
		db, okb := r.col.mustDoc(idx[b])
		var vi, vj any
		var pi, pj bool
		if oka {
			vi, pi = da.Get(field)
		}
		if okb {
			vj, pj = db.Get(field)
		}
		if !pi || !pj {
			if pi != pj {
				return pi
			}
			return false
		}
		cc, ok := compareValues(vi, vj)
		if !ok {
			return false
		}
		if desc {
			return cc > 0
		}
		return cc < 0
	})
	return &Result{col: r.col, idx: idx}
}

func (r *Result) Limit(n int) *Result {
	if n < 0 {
		n = 0
	}
	if n > len(r.idx) {
		n = len(r.idx)
	}
	return &Result{col: r.col, idx: r.idx[:n]}
}

func (r *Result) Offset(n int) *Result {
	if n < 0 {
		n = 0
	}
	if n > len(r.idx) {
		n = len(r.idx)
	}
	return &Result{col: r.col, idx: r.idx[n:]}
}

// Page returns 1-based page num of the given size.
func (r *Result) Page(num, size int) *Result {
	if num < 1 || size < 1 {
		return &Result{col: r.col}
	}
	return r.Offset((num - 1) * size).Limit(size)
}
