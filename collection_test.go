package jsonldb

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFixture(t *testing.T, lines string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "data.jsonl")
	if err := os.WriteFile(p, []byte(lines), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOpenScanAndRead(t *testing.T) {
	p := writeFixture(t, `{"id":"a"}
{"id":"b"}

{"id":"c"}
`)
	c, err := Open(p)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()
	if c.Count() != 3 {
		t.Fatalf("Count = %d, want 3 (blank line skipped)", c.Count())
	}
	first, _ := c.First()
	if first.GetString("id") != "a" || first.Line() != 1 {
		t.Errorf("First = %q line %d", first.GetString("id"), first.Line())
	}
	last, _ := c.Last()
	if last.GetString("id") != "c" {
		t.Errorf("Last = %q", last.GetString("id"))
	}
	// Line numbers count physical lines incl. blanks: c is on line 4.
	if last.Line() != 4 {
		t.Errorf("Last line = %d, want 4", last.Line())
	}
}

func TestOpenCreatesMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "new.jsonl")
	c, err := Open(p)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()
	if c.Count() != 0 {
		t.Errorf("new file Count = %d, want 0", c.Count())
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestEachStops(t *testing.T) {
	p := writeFixture(t, "{\"id\":\"a\"}\n{\"id\":\"b\"}\n{\"id\":\"c\"}\n")
	c, _ := Open(p)
	defer c.Close()
	seen := 0
	c.Each(func(d Doc) bool {
		seen++
		return d.GetString("id") != "b" // stop after b
	})
	if seen != 2 {
		t.Errorf("Each saw %d, want 2 (stopped at b)", seen)
	}
}

func TestSkipMalformedLines(t *testing.T) {
	c := openFixture(t, `{"a":1}
this is not json
{"a":2}
{bad
{"a":3}
`)
	if got := c.Where(predTrue()).Count(); got != 3 {
		t.Errorf("parseable count = %d, want 3", got)
	}
	if c.Skipped() != 2 {
		t.Errorf("skipped = %d, want 2", c.Skipped())
	}
}
