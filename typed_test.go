package jsonldb

import "testing"

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
