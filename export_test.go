package jsonldb

import (
	"bytes"
	"testing"
)

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

func TestWriteJSONL(t *testing.T) {
	c := openExportFixture(t)
	var buf bytes.Buffer
	if err := c.Where(predTrue()).WriteJSONL(&buf); err != nil {
		t.Fatal(err)
	}
	// unprojected → raw lines verbatim, newline-terminated
	want := `{"id":"a","topic":"ml","dur":1500,"done":true}` + "\n" +
		`{"id":"b","topic":"go","dur":900,"done":false}` + "\n"
	if buf.String() != want {
		t.Errorf("WriteJSONL =\n%q\nwant\n%q", buf.String(), want)
	}
	// projected → marshaled narrowed docs (sorted keys)
	var pb bytes.Buffer
	if err := c.Where(predTrue()).Project("id", "topic").WriteJSONL(&pb); err != nil {
		t.Fatal(err)
	}
	pwant := `{"id":"a","topic":"ml"}` + "\n" + `{"id":"b","topic":"go"}` + "\n"
	if pb.String() != pwant {
		t.Errorf("projected WriteJSONL =\n%q\nwant\n%q", pb.String(), pwant)
	}
}

func TestWriteJSON(t *testing.T) {
	c := openExportFixture(t)
	var buf bytes.Buffer
	if err := c.Where(predTrue()).Project("id").WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != `[{"id":"a"},{"id":"b"}]` {
		t.Errorf("WriteJSON = %q", buf.String())
	}
	// empty result → []
	var eb bytes.Buffer
	if err := c.Where(Eq("id", "zzz")).WriteJSON(&eb); err != nil {
		t.Fatal(err)
	}
	if eb.String() != "[]" {
		t.Errorf("empty WriteJSON = %q, want []", eb.String())
	}
}
