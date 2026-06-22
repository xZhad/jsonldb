package jsonldb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLargeFileLazyPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big.jsonl")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	const N = 50000
	for i := 0; i < N; i++ {
		topic := "ml"
		if i%3 == 0 {
			topic = "go"
		}
		fmt.Fprintf(f, "{\"id\":%d,\"topic\":%q,\"dur\":%d}\n", i, topic, (i%5+1)*300)
	}
	f.Close()

	c, err := Open(p, WithEagerThreshold(1<<20), WithCacheSize(256)) // ~file>1MB ⇒ lazy
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !c.lazy {
		t.Fatalf("expected lazy mode for the large fixture")
	}
	if c.Count() != N {
		t.Fatalf("Count = %d, want %d", c.Count(), N)
	}
	// selective query — raw pre-filter should keep memory + work low
	r, _ := c.Query("id=42")
	if r.Count() != 1 {
		t.Errorf("id=42 count = %d, want 1", r.Count())
	}
	// broad query stays index-backed (retains metas, not content)
	gq, _ := c.Query("topic=go")
	if gq.Count() == 0 || gq.Count() >= N {
		t.Errorf("topic=go count = %d looks wrong", gq.Count())
	}
	// cache bounded after a big scan
	if len(c.cache) > 256 {
		t.Errorf("cache exceeded cap: %d", len(c.cache))
	}
	// paging the unfiltered view materializes only a page
	page := c.Where(predTrue()).SortBy("id", false).Page(2, 20).Docs()
	if len(page) != 20 {
		t.Errorf("page len = %d, want 20", len(page))
	}
	// aggregation
	if sum, ok := c.Where(Eq("topic", "ml")).Sum("dur"); !ok || sum <= 0 {
		t.Errorf("Sum dur = %v ok=%v", sum, ok)
	}
	// typed round-trip still works
	type Rec struct {
		ID    int    `json:"id"`
		Topic string `json:"topic"`
	}
	recs, err := Typed[Rec](c).Query("id=7")
	if err != nil || len(recs) != 1 || recs[0].Topic == "" {
		t.Errorf("typed query failed: %+v err=%v", recs, err)
	}
}
