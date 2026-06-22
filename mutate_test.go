package jsonldb

import "testing"

func TestMutations(t *testing.T) {
	c := openFixture(t, `{"id":"a","done":false}
{"id":"b","done":false}
{"id":"c","done":true}
`)
	// Update: mark a done
	n, err := c.Update(Eq("id", "a"), func(d Doc) Doc {
		m := map[string]any{}
		for _, k := range []string{"id", "done"} {
			if v, ok := d.Get(k); ok {
				m[k] = v
			}
		}
		m["done"] = true
		return NewDoc(m)
	})
	if err != nil || n != 1 {
		t.Fatalf("Update n=%d err=%v", n, err)
	}
	if got, _ := c.Find(Eq("id", "a")); !got.GetBool("done") {
		t.Errorf("a not updated")
	}
	// Replace
	n, _ = c.Replace(Eq("id", "b"), NewDoc(map[string]any{"id": "b", "done": true, "x": 1}))
	if n != 1 {
		t.Errorf("Replace n=%d", n)
	}
	// DeleteWhere
	n, _ = c.DeleteWhere(Eq("done", true))
	if n != 3 {
		t.Errorf("DeleteWhere n=%d, want 3", n)
	}
	if c.Count() != 0 {
		t.Errorf("count after delete = %d, want 0", c.Count())
	}
}

func TestDeleteAt(t *testing.T) {
	c := openFixture(t, `{"id":"a"}
{"id":"b"}
{"id":"c"}
`)
	b, _ := c.Find(Eq("id", "b"))
	if err := c.DeleteAt(b.Line()); err != nil {
		t.Fatalf("DeleteAt: %v", err)
	}
	if c.Count() != 2 {
		t.Errorf("count = %d, want 2", c.Count())
	}
	if _, ok := c.Find(Eq("id", "b")); ok {
		t.Errorf("b still present")
	}
}
