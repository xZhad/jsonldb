package jsonldb

import (
	"os"
	"path/filepath"
	"testing"
)

func twoModes(t *testing.T, body string) (eager, lazy *Collection) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "data.jsonl")
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	e, err := Open(p, WithEagerThreshold(1<<30))
	if err != nil {
		t.Fatal(err)
	}
	l, err := Open(p, WithEagerThreshold(0), WithCacheSize(2)) // tiny cache forces eviction
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close(); l.Close() })
	return e, l
}

func TestLazyEagerParity(t *testing.T) {
	body := `{"id":"a","topic":"ml","dur":1500,"done":true}
{"id":"b","topic":"go","dur":900,"done":false}
{"id":"c","topic":"ml","dur":1200,"done":true}
{"id":"d","topic":"ml","dur":300,"done":false}
`
	e, l := twoModes(t, body)
	if !e.lazy && l.lazy {
		// good: e eager, l lazy
	} else {
		t.Fatalf("mode setup wrong: e.lazy=%v l.lazy=%v", e.lazy, l.lazy)
	}
	// Count, Query, Sort, Sum, GroupBy must match across modes
	check := func(name string, ev, lv any) {
		if ev != lv {
			t.Errorf("%s mismatch: eager=%v lazy=%v", name, ev, lv)
		}
	}
	check("count", e.Count(), l.Count())

	eq, _ := e.Query("done=true topic=ml")
	lq, _ := l.Query("done=true topic=ml")
	check("query count", eq.Count(), lq.Count())

	es := e.Where(predTrue()).SortBy("dur", false).Docs()
	ls := l.Where(predTrue()).SortBy("dur", false).Docs()
	check("sort len", len(es), len(ls))
	for i := range es {
		check("sort["+es[i].GetString("id")+"]", es[i].GetString("id"), ls[i].GetString("id"))
	}

	esum, _ := e.Where(Eq("topic", "ml")).Sum("dur")
	lsum, _ := l.Where(Eq("topic", "ml")).Sum("dur")
	check("sum", esum, lsum)

	eg := e.Where(predTrue()).GroupBy("topic")
	lg := l.Where(predTrue()).GroupBy("topic")
	check("group ml", eg["ml"].Count(), lg["ml"].Count())
}
