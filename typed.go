package jsonldb

import (
	"bytes"
	"encoding/json"
)

type TypedCollection[T any] struct{ c *Collection }

func Typed[T any](c *Collection) *TypedCollection[T] { return &TypedCollection[T]{c: c} }

func (t *TypedCollection[T]) Raw() *Collection { return t.c }

func decode[T any](d Doc) (T, error) {
	var v T
	dec := json.NewDecoder(bytes.NewReader(d.Raw()))
	dec.UseNumber()
	err := dec.Decode(&v)
	return v, err
}

func toDoc[T any](v T) (Doc, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return Doc{}, err
	}
	return parseDoc(b, 0)
}

func decodeAll[T any](ds []Doc) ([]T, error) {
	out := make([]T, 0, len(ds))
	for _, d := range ds {
		v, err := decode[T](d)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (t *TypedCollection[T]) All() ([]T, error) { return decodeAll[T](t.c.All()) }

func (t *TypedCollection[T]) Each(fn func(T) bool) error {
	var derr error
	t.c.Each(func(d Doc) bool {
		v, err := decode[T](d)
		if err != nil {
			derr = err
			return false
		}
		return fn(v)
	})
	return derr
}

func (t *TypedCollection[T]) First() (T, bool, error) {
	d, ok := t.c.First()
	if !ok {
		var z T
		return z, false, nil
	}
	v, err := decode[T](d)
	return v, true, err
}

func (t *TypedCollection[T]) Last() (T, bool, error) {
	d, ok := t.c.Last()
	if !ok {
		var z T
		return z, false, nil
	}
	v, err := decode[T](d)
	return v, true, err
}

func (t *TypedCollection[T]) Find(p Predicate) (T, bool, error) {
	d, ok := t.c.Find(p)
	if !ok {
		var z T
		return z, false, nil
	}
	v, err := decode[T](d)
	return v, true, err
}

func (t *TypedCollection[T]) Query(dsl string) ([]T, error) {
	r, err := t.c.Query(dsl)
	if err != nil {
		return nil, err
	}
	return decodeAll[T](r.Docs())
}

func (t *TypedCollection[T]) Where(p Predicate) *TypedResult[T] {
	return &TypedResult[T]{r: t.c.Where(p)}
}

func (t *TypedCollection[T]) Append(v T) error {
	d, err := toDoc[T](v)
	if err != nil {
		return err
	}
	return t.c.Append(d)
}

func (t *TypedCollection[T]) AppendAll(vs []T) error {
	ds := make([]Doc, 0, len(vs))
	for _, v := range vs {
		d, err := toDoc[T](v)
		if err != nil {
			return err
		}
		ds = append(ds, d)
	}
	return t.c.AppendAll(ds)
}

func (t *TypedCollection[T]) Update(p Predicate, mut func(T) T) (int, error) {
	var derr error
	n, err := t.c.Update(p, func(d Doc) Doc {
		v, e := decode[T](d)
		if e != nil {
			derr = e
			return d
		}
		nd, e := toDoc[T](mut(v))
		if e != nil {
			derr = e
			return d
		}
		return nd
	})
	if derr != nil {
		return 0, derr
	}
	return n, err
}

func (t *TypedCollection[T]) Replace(p Predicate, v T) (int, error) {
	d, err := toDoc[T](v)
	if err != nil {
		return 0, err
	}
	return t.c.Replace(p, d)
}

func (t *TypedCollection[T]) DeleteWhere(p Predicate) (int, error) { return t.c.DeleteWhere(p) }
func (t *TypedCollection[T]) DeleteAt(line int) error              { return t.c.DeleteAt(line) }

type TypedResult[T any] struct{ r *Result }

func (tr *TypedResult[T]) Raw() *Result { return tr.r }
func (tr *TypedResult[T]) Count() int   { return tr.r.Count() }

func (tr *TypedResult[T]) SortBy(field string, desc bool) *TypedResult[T] {
	return &TypedResult[T]{r: tr.r.SortBy(field, desc)}
}
func (tr *TypedResult[T]) Limit(n int) *TypedResult[T]  { return &TypedResult[T]{r: tr.r.Limit(n)} }
func (tr *TypedResult[T]) Offset(n int) *TypedResult[T] { return &TypedResult[T]{r: tr.r.Offset(n)} }
func (tr *TypedResult[T]) Page(num, size int) *TypedResult[T] {
	return &TypedResult[T]{r: tr.r.Page(num, size)}
}

func (tr *TypedResult[T]) Docs() ([]T, error) { return decodeAll[T](tr.r.Docs()) }

func (tr *TypedResult[T]) First() (T, bool, error) {
	d, ok := tr.r.First()
	if !ok {
		var z T
		return z, false, nil
	}
	v, err := decode[T](d)
	return v, true, err
}

func (tr *TypedResult[T]) Last() (T, bool, error) {
	d, ok := tr.r.Last()
	if !ok {
		var z T
		return z, false, nil
	}
	v, err := decode[T](d)
	return v, true, err
}
