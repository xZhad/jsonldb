package jsonldb

import (
	"bytes"
	"encoding/csv"
	"strings"
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

func TestWriteCSV(t *testing.T) {
	c := openExportFixture(t)
	var buf bytes.Buffer
	if err := c.Where(predTrue()).Project("id", "topic", "dur").WriteCSV(&buf); err != nil {
		t.Fatal(err)
	}
	// header in projection order, then rows
	got := buf.String()
	want := "id,topic,dur\na,ml,1500\nb,go,900\n"
	if got != want {
		t.Errorf("WriteCSV =\n%q\nwant\n%q", got, want)
	}
	// round-trips through encoding/csv
	if _, err := csv.NewReader(bytes.NewReader(buf.Bytes())).ReadAll(); err != nil {
		t.Errorf("CSV not parseable: %v", err)
	}
}

func TestCSVNonScalarAndMissing(t *testing.T) {
	c, err := Open(fixtureFile(t, `{"id":"a","tags":["x","y"]}
{"id":"b"}
`))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var buf bytes.Buffer
	if err := c.Where(predTrue()).Project("id", "tags").WriteCSV(&buf); err != nil {
		t.Fatal(err)
	}
	// non-scalar tags → JSON-encoded cell (csv-quoted); missing tags → empty cell
	rows, _ := csv.NewReader(bytes.NewReader(buf.Bytes())).ReadAll()
	if rows[1][1] != `["x","y"]` {
		t.Errorf("row a tags cell = %q, want JSON array", rows[1][1])
	}
	if rows[2][1] != "" {
		t.Errorf("row b tags cell = %q, want empty", rows[2][1])
	}
}

func TestWriteMarkdown(t *testing.T) {
	c := openExportFixture(t)
	var buf bytes.Buffer
	if err := c.Where(predTrue()).Project("id", "topic").WriteMarkdown(&buf); err != nil {
		t.Fatal(err)
	}
	want := "| id | topic |\n| --- | --- |\n| a | ml |\n| b | go |\n"
	if buf.String() != want {
		t.Errorf("WriteMarkdown =\n%q\nwant\n%q", buf.String(), want)
	}
}

func TestMarkdownEscaping(t *testing.T) {
	c, err := Open(fixtureFile(t, `{"id":"a","note":"x|y\nz"}
`))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var buf bytes.Buffer
	if err := c.Where(predTrue()).Project("id", "note").WriteMarkdown(&buf); err != nil {
		t.Fatal(err)
	}
	// pipe escaped, newline collapsed to a space → table stays rectangular
	want := "| id | note |\n| --- | --- |\n| a | x\\|y z |\n"
	if buf.String() != want {
		t.Errorf("MD escaping =\n%q\nwant\n%q", buf.String(), want)
	}
}

func TestExportEmptyAndZeroColumns(t *testing.T) {
	c := openExportFixture(t)
	empty := c.Where(Eq("id", "zzz")) // no matches

	// JSONL empty → nothing
	var jl bytes.Buffer
	empty.WriteJSONL(&jl)
	if jl.String() != "" {
		t.Errorf("empty JSONL = %q, want \"\"", jl.String())
	}
	// CSV empty result (collection non-empty) → header only, no rows
	var cs bytes.Buffer
	empty.WriteCSV(&cs)
	rows, _ := csv.NewReader(bytes.NewReader(cs.Bytes())).ReadAll()
	if len(rows) != 1 {
		t.Errorf("empty-result CSV rows = %d, want 1 (header only)", len(rows))
	}
	// MD empty result → header + separator only (2 lines)
	var md bytes.Buffer
	empty.WriteMarkdown(&md)
	if n := strings.Count(md.String(), "\n"); n != 2 {
		t.Errorf("empty-result MD lines = %d, want 2", n)
	}

	// zero columns: an empty collection → no schema → CSV/MD write nothing
	ec, _ := Open(fixtureFile(t, ""))
	defer ec.Close()
	var z bytes.Buffer
	ec.Where(predTrue()).WriteCSV(&z)
	if z.String() != "" {
		t.Errorf("zero-column CSV = %q, want \"\"", z.String())
	}
	var zm bytes.Buffer
	ec.Where(predTrue()).WriteMarkdown(&zm)
	if zm.String() != "" {
		t.Errorf("zero-column MD = %q, want \"\"", zm.String())
	}
}

func TestExportLazyEagerParity(t *testing.T) {
	body := `{"id":"a","topic":"ml","dur":1500}
{"id":"b","topic":"go","dur":900}
{"id":"c","topic":"ml","dur":1200}
`
	p := fixtureFile(t, body)
	eager, err := Open(p, WithEagerThreshold(1<<30))
	if err != nil {
		t.Fatal(err)
	}
	defer eager.Close()
	lazy, err := Open(p, WithEagerThreshold(0), WithCacheSize(1)) // force lazy + eviction
	if err != nil {
		t.Fatal(err)
	}
	defer lazy.Close()

	for _, tc := range []struct {
		name  string
		write func(*Result, *bytes.Buffer) error
	}{
		{"jsonl", func(r *Result, b *bytes.Buffer) error { return r.WriteJSONL(b) }},
		{"json", func(r *Result, b *bytes.Buffer) error { return r.WriteJSON(b) }},
		{"csv", func(r *Result, b *bytes.Buffer) error { return r.WriteCSV(b) }},
		{"md", func(r *Result, b *bytes.Buffer) error { return r.WriteMarkdown(b) }},
	} {
		var eb, lb bytes.Buffer
		if err := tc.write(eager.Where(predTrue()).Project("id", "topic", "dur"), &eb); err != nil {
			t.Fatal(err)
		}
		if err := tc.write(lazy.Where(predTrue()).Project("id", "topic", "dur"), &lb); err != nil {
			t.Fatal(err)
		}
		if eb.String() != lb.String() {
			t.Errorf("%s: lazy != eager\neager=%q\nlazy =%q", tc.name, eb.String(), lb.String())
		}
	}
}
