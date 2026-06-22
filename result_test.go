package jsonldb

import "testing"

func openFixture(t *testing.T, lines string) *Collection {
	t.Helper()
	c, err := Open(writeFixture(t, lines))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestResultIsIndexBacked(t *testing.T) {
	c := openFixture(t, `{"id":"a"}
{"id":"b"}
{"id":"c"}
`)
	r := c.Where(Eq("id", "b"))
	if r.col != c || len(r.idx) != 1 {
		t.Fatalf("expected index-backed result with 1 idx, got idx=%v", r.idx)
	}
	d, ok := r.First()
	if !ok || d.GetString("id") != "b" {
		t.Errorf("First = %q", d.GetString("id"))
	}
}

func TestWhereQueryFind(t *testing.T) {
	c := openFixture(t, `{"id":"a","done":true,"dur":1500}
{"id":"b","done":false,"dur":900}
{"id":"c","done":true,"dur":1500}
`)
	if n := c.Where(Eq("done", true)).Count(); n != 2 {
		t.Errorf("Where count = %d, want 2", n)
	}
	r, err := c.Query(`done=true dur>=1500`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if r.Count() != 2 {
		t.Errorf("Query count = %d, want 2", r.Count())
	}
	first, ok := r.First()
	if !ok || first.GetString("id") != "a" {
		t.Errorf("First = %q", first.GetString("id"))
	}
	last, _ := r.Last()
	if last.GetString("id") != "c" {
		t.Errorf("Last = %q", last.GetString("id"))
	}
	found, ok := c.Find(Eq("id", "b"))
	if !ok {
		t.Errorf("Find b failed: not found")
	}
	if v, _ := found.GetInt("dur"); v != 900 {
		t.Errorf("Find b dur = %d, want 900", v)
	}
	if _, err := c.Query(`dur>=`); err == nil {
		t.Errorf("expected parse error")
	}
}
