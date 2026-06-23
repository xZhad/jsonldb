package jsonldb

import (
	"encoding/json"
	"sort"
)

// Result is a chainable view over a filtered set of Docs, backed by index positions.
type Result struct {
	col     *Collection
	idx     []int
	project []string // nil = no projection; else ordered key subset for output
}

func (r *Result) Docs() []Doc {
	out := make([]Doc, 0, len(r.idx))
	for _, i := range r.idx {
		d, ok := r.col.mustDoc(i)
		if !ok {
			continue
		}
		if r.project != nil {
			d = narrowDoc(d, r.project)
		}
		out = append(out, d)
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
		if v, ok := d.getField(field); ok {
			return valueKey(v)
		}
		return ""
	})
}

func (r *Result) CountBy(field string) map[string]int {
	out := map[string]int{}
	for _, i := range r.idx {
		if d, ok := r.col.mustDoc(i); ok {
			if v, ok := d.getField(field); ok {
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
			if v, ok := d.getField(field); ok {
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

// reduce folds over the result index, computing count, sum, min, and max of a numeric field.
// Returns (count, sum, min, max).
func (r *Result) reduce(field string) (count int, sum, mn, mx float64) {
	first := true
	for _, i := range r.idx {
		d, ok := r.col.mustDoc(i)
		if !ok {
			continue
		}
		f, ok := d.getFloatField(field)
		if !ok {
			continue
		}
		count++
		sum += f
		if first || f < mn {
			mn = f
		}
		if first || f > mx {
			mx = f
		}
		first = false
	}
	return
}

func (r *Result) Sum(field string) (float64, bool) {
	n, s, _, _ := r.reduce(field)
	if n == 0 {
		return 0, false
	}
	return s, true
}

func (r *Result) Avg(field string) (float64, bool) {
	n, s, _, _ := r.reduce(field)
	if n == 0 {
		return 0, false
	}
	return s / float64(n), true
}

func (r *Result) Min(field string) (float64, bool) {
	n, _, mn, _ := r.reduce(field)
	if n == 0 {
		return 0, false
	}
	return mn, true
}

func (r *Result) Max(field string) (float64, bool) {
	n, _, _, mx := r.reduce(field)
	if n == 0 {
		return 0, false
	}
	return mx, true
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
	type kv struct {
		i       int
		v       any
		present bool
	}
	items := make([]kv, len(r.idx))
	for n, i := range r.idx {
		it := kv{i: i}
		if d, ok := r.col.mustDoc(i); ok {
			it.v, it.present = d.getField(field)
		}
		items[n] = it
	}
	sort.SliceStable(items, func(a, b int) bool {
		// Missing keys and JSON nulls always sort last, in both directions.
		na := !items[a].present || items[a].v == nil
		nb := !items[b].present || items[b].v == nil
		if na || nb {
			if na != nb {
				return nb // b is the null/absent one → a comes first
			}
			return false
		}
		cc := compareForSort(items[a].v, items[b].v)
		if desc {
			return cc > 0
		}
		return cc < 0
	})
	idx := make([]int, len(items))
	for n, it := range items {
		idx[n] = it.i
	}
	return &Result{col: r.col, idx: idx, project: r.project}
}

func (r *Result) Limit(n int) *Result {
	if n < 0 {
		n = 0
	}
	if n > len(r.idx) {
		n = len(r.idx)
	}
	return &Result{col: r.col, idx: r.idx[:n], project: r.project}
}

func (r *Result) Offset(n int) *Result {
	if n < 0 {
		n = 0
	}
	if n > len(r.idx) {
		n = len(r.idx)
	}
	return &Result{col: r.col, idx: r.idx[n:], project: r.project}
}

// Page returns 1-based page num of the given size.
func (r *Result) Page(num, size int) *Result {
	if num < 1 || size < 1 {
		return &Result{col: r.col, project: r.project}
	}
	return r.Offset((num - 1) * size).Limit(size)
}
