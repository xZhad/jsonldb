package jsonldb

import "testing"

func openExportFixture(t *testing.T) *Collection {
	t.Helper()
	c, err := Open(fixtureFile(t, `{"id":"a","topic":"ml","dur":1500,"done":true}
{"id":"b","topic":"go","dur":900,"done":false}
`))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestProjectNarrowsDocs(t *testing.T) {
	c := openExportFixture(t)
	docs := c.Where(predTrue()).Project("topic", "id").Docs()
	if len(docs) != 2 {
		t.Fatalf("got %d docs, want 2", len(docs))
	}
	// narrowed: only id + topic present, dur/done dropped
	if _, ok := docs[0].Get("dur"); ok {
		t.Errorf("dur should be projected out")
	}
	if docs[0].GetString("id") != "a" || docs[0].GetString("topic") != "ml" {
		t.Errorf("projected doc wrong: %v", docs[0])
	}
	// projected key absent from a doc → simply absent (no synthesized null)
	missing := c.Where(predTrue()).Project("nope").Docs()
	if _, ok := missing[0].Get("nope"); ok {
		t.Errorf("absent projected key should not appear")
	}
}
