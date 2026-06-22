package jsonldb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRawRejectNoFalseNegatives(t *testing.T) {
	// For a battery of predicates over a fixture, every doc that Match()es
	// must NOT be rawReject'd. (rawReject may keep non-matches; it must never
	// drop a true match.)
	docs := []string{
		`{"topic":"jsonldb spec","dur":1500,"done":true,"tags":["x"]}`,
		`{"topic":"ml","dur":900,"done":false}`,
		`{"note":"see jsonldb"}`,
	}
	preds := []Predicate{
		Eq("done", true),
		Contains("topic", "JSONL"),
		HasKey("tags"),
		Prefix("topic", "json"),
		Suffix("topic", "spec"),
		And(Eq("done", true), Contains("topic", "spec")),
		Or(Eq("topic", "ml"), HasKey("note")),
	}
	for _, raw := range docs {
		d, _ := parseDoc([]byte(raw), 1)
		for pi, p := range preds {
			if p.Match(d) && p.rawReject([]byte(raw)) {
				t.Errorf("pred %d FALSE-NEGATIVE on %s", pi, raw)
			}
		}
	}
}

func TestRawRejectActuallyRejects(t *testing.T) {
	// A selective Eq should reject a line that cannot contain the value.
	p := Eq("id", "zzz")
	if !p.rawReject([]byte(`{"id":"aaa"}`)) {
		t.Error("Eq should reject a line missing the value token")
	}
	if p.rawReject([]byte(`{"id":"zzz"}`)) {
		t.Error("Eq must not reject a line containing the value")
	}
}

// TestNumericRepresentationParity guards against the false-negative that was
// present when valueToken returned the literal text of a json.Number. Numbers
// are not byte-canonical: 1500, 1.5e3, 15e2, 1500.0 all compare equal as
// float values but share no common literal substring. This test verifies:
//  1. rawReject never drops a doc that Match() accepts (no false negatives).
//  2. A lazy Query over a fixture containing multiple representations of the
//     same number returns the same results as eager mode (parity).
func TestNumericRepresentationParity(t *testing.T) {
	// Part 1: rawReject must never drop a doc that Match() accepts.
	numericDocs := []string{
		`{"dur":1500}`,
		`{"dur":1.5e3}`,
		`{"dur":15e2}`,
		`{"dur":1500.0}`,
		`{"n":1e2}`,
		`{"n":100}`,
		`{"n":100.0}`,
	}
	preds := []struct {
		name string
		p    Predicate
	}{
		{"Eq(dur,1500)", Eq("dur", 1500)},
		{"Eq(n,100)", Eq("n", 100)},
	}
	for _, raw := range numericDocs {
		d, err := parseDoc([]byte(raw), 1)
		if err != nil {
			t.Fatalf("parseDoc(%s): %v", raw, err)
		}
		for _, tc := range preds {
			if tc.p.Match(d) && tc.p.rawReject([]byte(raw)) {
				t.Errorf("FALSE NEGATIVE: pred %s matched doc %s but rawReject dropped it", tc.name, raw)
			}
		}
	}

	// Part 2: lazy and eager query parity over alternative numeric encodings.
	fixture := `{"dur":1.5e3,"label":"a"}
{"dur":15e2,"label":"b"}
{"dur":1500,"label":"c"}
{"dur":1500.0,"label":"d"}
{"dur":999,"label":"e"}
`
	dir := t.TempDir()
	p := filepath.Join(dir, "nums.jsonl")
	if err := os.WriteFile(p, []byte(fixture), 0644); err != nil {
		t.Fatal(err)
	}
	eager, err := Open(p, WithEagerThreshold(1<<30))
	if err != nil {
		t.Fatal(err)
	}
	defer eager.Close()
	lazy, err := Open(p, WithEagerThreshold(0))
	if err != nil {
		t.Fatal(err)
	}
	defer lazy.Close()

	// Eq("dur", 1500) must return 4 docs (a, b, c, d) in both modes.
	eq1500 := Eq("dur", 1500)
	eagerResult := eager.Where(eq1500).Docs()
	lazyResult := lazy.Where(eq1500).Docs()
	if len(eagerResult) != 4 {
		t.Errorf("eager Eq(dur,1500): want 4, got %d", len(eagerResult))
	}
	if len(lazyResult) != 4 {
		t.Errorf("lazy Eq(dur,1500): want 4, got %d", len(lazyResult))
	}
	if len(eagerResult) != len(lazyResult) {
		t.Errorf("parity failure: eager=%d lazy=%d", len(eagerResult), len(lazyResult))
	}

	// DSL Query parity: "dur=1500" must also return all 4.
	eagerDSL, err := eager.Query("dur=1500")
	if err != nil {
		t.Fatal(err)
	}
	lazyDSL, err := lazy.Query("dur=1500")
	if err != nil {
		t.Fatal(err)
	}
	if eagerDSL.Count() != lazyDSL.Count() {
		t.Errorf("DSL parity failure: eager=%d lazy=%d", eagerDSL.Count(), lazyDSL.Count())
	}
	if lazyDSL.Count() != 4 {
		t.Errorf("DSL lazy Query(dur=1500): want 4, got %d", lazyDSL.Count())
	}
}

func TestRawRejectEscapedStrings(t *testing.T) {
	// value contains a double-quote → raw stores O\"Brien
	raw := `{"name":"O\"Brien","note":"a\\b"}`
	d, _ := parseDoc([]byte(raw), 1)
	for _, p := range []Predicate{
		Eq("name", `O"Brien`),
		Eq("note", `a\b`),
		Contains("name", `"Brien`),
		Prefix("name", `O"`),
	} {
		if p.Match(d) && p.rawReject([]byte(raw)) {
			t.Errorf("FALSE NEGATIVE: pred matched but rawReject dropped it (raw=%s)", raw)
		}
	}
}
