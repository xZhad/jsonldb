package jsonldb

// Project returns a new Result that narrows each doc to the given keys (in
// order) at output time — for Docs() and the Write* methods. It does not
// re-filter; do filtering/sorting/aggregation before projecting.
func (r *Result) Project(keys ...string) *Result {
	return &Result{col: r.col, idx: r.idx, project: append([]string(nil), keys...)}
}

// narrowDoc returns a Doc containing only the given keys that are present in d,
// preserving their values (and json.Number typing).
func narrowDoc(d Doc, keys []string) Doc {
	m := make(map[string]any, len(keys))
	for _, k := range keys {
		if v, ok := d.Get(k); ok {
			m[k] = v
		}
	}
	return NewDoc(m)
}
