package jsonldb

import (
	"os"
	"testing"
)

type tSession struct {
	ID       string `json:"id"`
	Topic    string `json:"topic"`
	Duration int    `json:"duration"`
	Done     bool   `json:"done"`
}

func TestTypedRoundTrip(t *testing.T) {
	c := openFixture(t, "")
	db := Typed[tSession](c)
	if err := db.Append(tSession{ID: "a", Topic: "ml", Duration: 1500, Done: true}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := db.AppendAll([]tSession{
		{ID: "b", Topic: "go", Duration: 900},
		{ID: "c", Topic: "ml", Duration: 1200, Done: true},
	}); err != nil {
		t.Fatalf("AppendAll: %v", err)
	}
	all, err := db.All()
	if err != nil || len(all) != 3 {
		t.Fatalf("All len=%d err=%v", len(all), err)
	}
	// Query decodes survivors
	done, _ := db.Query(`done=true`)
	if len(done) != 2 {
		t.Errorf("Query done=true len=%d, want 2", len(done))
	}
	// Find
	b, ok, _ := db.Find(Eq("id", "b"))
	if !ok || b.Topic != "go" {
		t.Errorf("Find b: %+v ok=%v", b, ok)
	}
	// typed sort + First
	first, ok, _ := db.Where(Eq("topic", "ml")).SortBy("duration", true).First()
	if !ok || first.ID != "a" {
		t.Errorf("typed sorted First = %+v", first)
	}
	// Update typed
	n, _ := db.Update(Eq("id", "b"), func(s tSession) tSession { s.Done = true; return s })
	if n != 1 {
		t.Errorf("Update n=%d", n)
	}
	// aggregation via Raw()
	if sum, ok := db.Where(Eq("topic", "ml")).Raw().Sum("duration"); !ok || sum != 2700 {
		t.Errorf("Raw Sum = %v", sum)
	}
}

// tBadDecode is used to create a type mismatch: the fixture stores "duration"
// as a string but tBadSession declares it as int, so json.Unmarshal errors.
type tBadSession struct {
	ID       string `json:"id"`
	Duration int    `json:"duration"` // conflicts with stored `"duration":"notanumber"`
}

// TestTypedUpdateAbortOnDecodeError verifies that if a matched doc cannot be
// decoded into T, typed Update:
//   (a) returns a non-nil error,
//   (b) returns count 0,
//   (c) leaves the file on disk UNCHANGED.
func TestTypedUpdateAbortOnDecodeError(t *testing.T) {
	// Build a fixture with two docs.  The second doc has duration as a string,
	// which will cause json.Unmarshal into tBadSession.Duration (int) to fail.
	fixture := "{\"id\":\"ok\",\"duration\":42}\n{\"id\":\"bad\",\"duration\":\"notanumber\"}\n"
	c := openFixture(t, fixture)
	db := Typed[tBadSession](c)

	// Capture raw bytes BEFORE the call.
	before, err := os.ReadFile(c.Path())
	if err != nil {
		t.Fatalf("ReadFile before: %v", err)
	}

	// Predicate matches BOTH docs; the second will fail to decode.
	n, updateErr := db.Update(func(d Doc) bool { return true }, func(s tBadSession) tBadSession {
		s.ID += "_updated"
		return s
	})

	// (a) error must be non-nil
	if updateErr == nil {
		t.Errorf("Update: expected decode error, got nil")
	}
	// (b) count must be 0
	if n != 0 {
		t.Errorf("Update: count = %d, want 0", n)
	}
	// (c) file must be unchanged
	after, err := os.ReadFile(c.Path())
	if err != nil {
		t.Fatalf("ReadFile after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("file was modified despite decode error\nbefore: %q\nafter:  %q", before, after)
	}
}
