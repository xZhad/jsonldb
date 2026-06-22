package jsonldb

import "testing"

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
