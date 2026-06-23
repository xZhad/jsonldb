package jsonldb

import "io"

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

// docBytes returns the JSON bytes for d honoring projection: the verbatim raw
// line when unprojected, else the marshaled narrowed doc (sorted keys).
func (r *Result) docBytes(d Doc) ([]byte, error) {
	if r.project == nil {
		return d.Raw(), nil
	}
	return narrowDoc(d, r.project).MarshalJSON()
}

// WriteJSONL writes one JSON object per line. Streams; bounded memory.
func (r *Result) WriteJSONL(w io.Writer) error {
	for _, i := range r.idx {
		d, ok := r.col.mustDoc(i)
		if !ok {
			continue
		}
		b, err := r.docBytes(d)
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}

// WriteJSON writes the result as a single JSON array. Streams; bounded memory.
func (r *Result) WriteJSON(w io.Writer) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	first := true
	for _, i := range r.idx {
		d, ok := r.col.mustDoc(i)
		if !ok {
			continue
		}
		if !first {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		first = false
		b, err := r.docBytes(d)
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}
