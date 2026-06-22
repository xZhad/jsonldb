package jsonldb

import (
	"os"
	"path/filepath"
	"testing"
)

func fixtureFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "d.jsonl")
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIndexOffsetsAndMaterialize(t *testing.T) {
	c, err := Open(fixtureFile(t, "{\"id\":\"a\"}\n\n{\"id\":\"b\"}\n{\"id\":\"c\"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if len(c.index) != 3 {
		t.Fatalf("index len = %d, want 3 (blank skipped)", len(c.index))
	}
	// readRaw matches the original line bytes
	raw, err := c.readRaw(0)
	if err != nil || string(raw) != `{"id":"a"}` {
		t.Errorf("readRaw(0) = %q err=%v", raw, err)
	}
	// materialize parses + carries physical line (c is on physical line 4)
	d, err := c.materialize(2)
	if err != nil || d.GetString("id") != "c" || d.Line() != 4 {
		t.Errorf("materialize(2) id=%q line=%d err=%v", d.GetString("id"), d.Line(), err)
	}
	// cache hit returns same content
	if _, ok := c.cache[2]; !ok {
		t.Error("materialize did not cache")
	}
}
