package jsonldb

import (
	"os"
	"strings"
	"testing"
)

func TestAppendAndAtomicRewrite(t *testing.T) {
	c := openFixture(t, `{"id":"a"}
`)
	if err := c.Append(NewDoc(map[string]any{"id": "b"})); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := c.AppendAll([]Doc{
		NewDoc(map[string]any{"id": "c"}),
		NewDoc(map[string]any{"id": "d"}),
	}); err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	if c.Count() != 4 {
		t.Fatalf("Count = %d, want 4", c.Count())
	}
	// in-memory reflects new docs with line numbers assigned
	last, _ := c.Last()
	if last.GetString("id") != "d" {
		t.Errorf("Last = %q", last.GetString("id"))
	}
	// on-disk: one object per line, original line preserved verbatim
	raw, _ := os.ReadFile(c.Path())
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("disk lines = %d, want 4", len(lines))
	}
	if lines[0] != `{"id":"a"}` {
		t.Errorf("line0 not preserved verbatim: %q", lines[0])
	}
}
