package jsonldb

import "testing"

func TestSchemaKeysSample(t *testing.T) {
	c := openFixture(t, `{"id":"a","dur":1500,"tag":"x"}
{"id":"b","dur":900}
{"id":"c","dur":"oops"}
`)
	s := c.Schema()
	byKey := map[string]FieldInfo{}
	for _, f := range s {
		byKey[f.Key] = f
	}
	if byKey["id"].Presence != 1.0 {
		t.Errorf("id presence = %v, want 1.0", byKey["id"].Presence)
	}
	if byKey["tag"].Presence < 0.32 || byKey["tag"].Presence > 0.34 {
		t.Errorf("tag presence = %v, want ~0.333", byKey["tag"].Presence)
	}
	// dur is mixed number+string
	if len(byKey["dur"].Types) != 2 {
		t.Errorf("dur types = %v, want 2", byKey["dur"].Types)
	}
	// Schema sorted by presence desc — id/dur (1.0) before tag (0.33)
	if s[len(s)-1].Key != "tag" {
		t.Errorf("last field = %q, want tag", s[len(s)-1].Key)
	}
	if got := c.Keys(); len(got) != 3 {
		t.Errorf("Keys = %v, want 3", got)
	}
	if len(c.Sample(2)) != 2 {
		t.Errorf("Sample(2) wrong length")
	}
	if len(c.Sample(99)) != 3 {
		t.Errorf("Sample(99) should cap at 3")
	}
	if got := c.Sample(-1); len(got) != 0 {
		t.Errorf("Sample(-1) = %d docs, want 0 (no panic)", len(got))
	}
}

func TestSchemaApproxFlag(t *testing.T) {
	c := openFixture(t, `{"id":"a","topic":"ml"}
{"id":"b","topic":"go"}
`)
	for _, f := range c.Schema() {
		if f.Approx {
			t.Errorf("eager schema must be exact, got Approx on %s", f.Key)
		}
	}
	// force lazy
	cl, _ := Open(c.Path(), WithEagerThreshold(0))
	defer cl.Close()
	for _, f := range cl.Schema() {
		if !f.Approx {
			t.Errorf("lazy schema must be Approx, got exact on %s", f.Key)
		}
	}
}
