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

func TestSortWithNullsAndMixed(t *testing.T) {
	c := openFixture(t, `{"id":"a","n":3}
{"id":"b","n":null}
{"id":"c","n":1}
{"id":"d"}
{"id":"e","n":2}
`)
	all := c.Where(predTrue())

	asc := all.SortBy("n", false).Docs()
	got := ""
	for _, d := range asc {
		got += d.GetString("id")
	}
	// 1,2,3 first (c,e,a) then null/absent last (b,d in stable order)
	if got != "ceabd" {
		t.Errorf("asc with nulls = %q, want ceabd", got)
	}

	desc := all.SortBy("n", true).Docs()
	got = ""
	for _, d := range desc {
		got += d.GetString("id")
	}
	// 3,2,1 (a,e,c) then null/absent still last (b,d)
	if got != "aecbd" {
		t.Errorf("desc with nulls = %q, want aecbd", got)
	}
}

func TestSortMixedTypesDeterministic(t *testing.T) {
	c := openFixture(t, `{"id":"a","v":"hello"}
{"id":"b","v":10}
{"id":"c","v":true}
{"id":"d","v":2}
`)
	one := c.Where(predTrue()).SortBy("v", false).Docs()
	two := c.Where(predTrue()).SortBy("v", false).Docs()
	s1, s2 := "", ""
	for i := range one {
		s1 += one[i].GetString("id")
		s2 += two[i].GetString("id")
	}
	if s1 != s2 {
		t.Errorf("mixed-type sort not deterministic: %q vs %q", s1, s2)
	}
	// numbers (rank 0) sort before strings (rank 2) before bools (rank 3)
	if s1 != "dbac" {
		t.Errorf("mixed-type order = %q, want dbac (2,10,hello,true)", s1)
	}
}
