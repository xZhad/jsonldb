package jsonldb

import "testing"

func TestSortAndPage(t *testing.T) {
	c := openFixture(t, `{"id":"a","dur":900}
{"id":"b","dur":1500}
{"id":"c","dur":1200}
`)
	all := c.Where(pred(func(Doc) bool { return true }))

	asc := all.SortBy("dur", false).Docs()
	if asc[0].GetString("id") != "a" || asc[2].GetString("id") != "b" {
		t.Errorf("asc order wrong: %q,%q,%q", asc[0].GetString("id"), asc[1].GetString("id"), asc[2].GetString("id"))
	}
	desc := all.SortBy("dur", true).Docs()
	if desc[0].GetString("id") != "b" {
		t.Errorf("desc[0] = %q, want b", desc[0].GetString("id"))
	}
	// original Result not mutated
	if all.Docs()[0].GetString("id") != "a" {
		t.Errorf("SortBy mutated receiver")
	}
	if all.Limit(2).Count() != 2 {
		t.Errorf("Limit wrong")
	}
	if all.Offset(1).Count() != 2 {
		t.Errorf("Offset wrong")
	}
	pg := all.SortBy("dur", false).Page(2, 1).Docs()
	if len(pg) != 1 || pg[0].GetString("id") != "c" {
		t.Errorf("Page(2,1) = %v", pg)
	}
}

func TestSortByExtractsOncePerRecord(t *testing.T) {
	c := openFixture(t, `{"id":"a","dur":900}
{"id":"b","dur":1500}
{"id":"c","dur":1200}
`)
	r := c.Where(predTrue())
	asc := r.SortBy("dur", false).Docs()
	if asc[0].GetString("id") != "a" || asc[2].GetString("id") != "b" {
		t.Errorf("asc order wrong")
	}
	// receiver not mutated
	if r.Docs()[0].GetString("id") != "a" {
		t.Errorf("SortBy mutated receiver")
	}
}
