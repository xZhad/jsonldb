package jsonldb

import (
	"encoding/csv"
	"encoding/json"
	"io"
)

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

// columns returns the output columns: the projection (in order) if set, else
// the collection's Schema keys (presence-sorted).
func (r *Result) columns() []string {
	if r.project != nil {
		return r.project
	}
	sch := r.col.Schema()
	cols := make([]string, len(sch))
	for i, f := range sch {
		cols[i] = f.Key
	}
	return cols
}

// cellValue renders a doc's value for a flat-table cell: scalar text,
// JSON-encoded non-scalar, or "" for a missing key or null.
func cellValue(d Doc, key string) string {
	v, ok := d.Get(key)
	if !ok || v == nil {
		return ""
	}
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
	}
	b, _ := json.Marshal(v) // array / object
	return string(b)
}

// WriteCSV writes a header row + one row per doc. Streams; bounded memory.
func (r *Result) WriteCSV(w io.Writer) error {
	cols := r.columns()
	if len(cols) == 0 {
		return nil
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(cols); err != nil {
		return err
	}
	for _, i := range r.idx {
		d, ok := r.col.mustDoc(i)
		if !ok {
			continue
		}
		row := make([]string, len(cols))
		for j, c := range cols {
			row[j] = cellValue(d, c)
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
