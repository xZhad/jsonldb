package jsonldb

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDocAccessors(t *testing.T) {
	raw := []byte(`{"id":"s_1","duration":1500,"completed":true,"started":"2026-06-08T18:30:00-04:00","notes":[{"text":"hi"}]}`)
	d, err := parseDoc(raw, 3)
	if err != nil {
		t.Fatalf("parseDoc: %v", err)
	}
	if d.Line() != 3 {
		t.Errorf("Line = %d, want 3", d.Line())
	}
	if string(d.Raw()) != string(raw) {
		t.Errorf("Raw not preserved")
	}
	if d.GetString("id") != "s_1" {
		t.Errorf("GetString id = %q", d.GetString("id"))
	}
	if n, ok := d.GetInt("duration"); !ok || n != 1500 {
		t.Errorf("GetInt duration = %d, %v", n, ok)
	}
	if !d.GetBool("completed") {
		t.Errorf("GetBool completed = false")
	}
	if _, ok := d.GetTime("started"); !ok {
		t.Errorf("GetTime started failed")
	}
	if v, ok := d.Path("notes.0.text"); !ok || v != "hi" {
		t.Errorf("Path = %v, %v", v, ok)
	}
	if _, ok := d.Get("missing"); ok {
		t.Errorf("Get missing should be !ok")
	}
}

func TestDocMarshalStableOrder(t *testing.T) {
	d := NewDoc(map[string]any{"b": 2, "a": 1, "c": 3})
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `{"a":1,"b":2,"c":3}` {
		t.Errorf("MarshalJSON = %s, want sorted keys", b)
	}
}

func TestUseNumberDecode(t *testing.T) {
	d, _ := parseDoc([]byte(`{"n":42}`), 1)
	v, _ := d.Get("n")
	if _, ok := v.(json.Number); !ok {
		t.Errorf("number decoded as %T, want json.Number", v)
	}
}

func TestCompareAndEqualCoercion(t *testing.T) {
	// json.Number vs builder int
	if !equalValues(json.Number("1500"), 1500) {
		t.Errorf("1500 == 1500 failed across types")
	}
	c, ok := compareValues(json.Number("10"), json.Number("9"))
	if !ok || c != 1 {
		t.Errorf("compare 10 vs 9 = %d, %v", c, ok)
	}
	// RFC3339 time ordering
	c, ok = compareValues("2026-06-01T00:00:00Z", "2026-06-02T00:00:00Z")
	if !ok || c != -1 {
		t.Errorf("time compare = %d, %v", c, ok)
	}
	// forced string vs bool should not be equal
	if equalValues("true", true) {
		t.Errorf("string \"true\" must not equal bool true")
	}
	_ = time.Now
}
