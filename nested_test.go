package jsonldb

import "testing"

func TestNestedQuery(t *testing.T) {
	c := openFixture(t, `{"type":"message","message":{"role":"user","content":"hi"},"notes":[{"text":"a"}]}
{"type":"message","message":{"role":"assistant","content":"yo"}}
{"type":"custom"}
`)
	if n := c.Where(Eq("message.role", "user")).Count(); n != 1 {
		t.Errorf("Eq message.role=user count=%d, want 1", n)
	}
	r, err := c.Query("message.role=assistant")
	if err != nil || r.Count() != 1 {
		t.Errorf("DSL message.role=assistant count=%d err=%v", r.Count(), err)
	}
	if n := c.Where(Contains("message.content", "Y")).Count(); n != 1 {
		t.Errorf("Contains message.content~=Y count=%d, want 1", n)
	}
	if n := c.Where(Eq("notes.0.text", "a")).Count(); n != 1 {
		t.Errorf("Eq notes.0.text=a count=%d, want 1", n)
	}
	if n := c.Where(HasKey("message.role")).Count(); n != 2 {
		t.Errorf("HasKey message.role count=%d, want 2", n)
	}
	if n := c.Where(Eq("message.role", "nope")).Count(); n != 0 {
		t.Errorf("missing nested value count=%d, want 0", n)
	}
	// top-level still works
	if n := c.Where(Eq("type", "custom")).Count(); n != 1 {
		t.Errorf("top-level Eq broken: %d", n)
	}
}

func TestNestedSort(t *testing.T) {
	c := openFixture(t, `{"id":"a","m":{"n":3}}
{"id":"b","m":{"n":1}}
{"id":"c","m":{"n":2}}
`)
	docs := c.Where(predTrue()).SortBy("m.n", false).Docs()
	if docs[0].GetString("id") != "b" || docs[2].GetString("id") != "a" {
		t.Errorf("nested sort wrong: %s,%s,%s", docs[0].GetString("id"), docs[1].GetString("id"), docs[2].GetString("id"))
	}
}

func TestNestedRawRejectNoFalseNegatives(t *testing.T) {
	raw := `{"message":{"role":"user","content":"hi"}}`
	d, _ := parseDoc([]byte(raw), 1)
	for _, p := range []Predicate{
		Eq("message.role", "user"),
		Contains("message.content", "h"),
		HasKey("message.role"),
		Prefix("message.role", "us"),
	} {
		if p.Match(d) && p.rawReject([]byte(raw)) {
			t.Errorf("FALSE NEGATIVE: matched but rawReject dropped %s", raw)
		}
	}
}
