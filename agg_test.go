package jsonldb

import "testing"

// predTrue returns a predicate that matches all documents.
func predTrue() Predicate {
	return pred(func(Doc) bool { return true })
}

func TestAggregation(t *testing.T) {
	c := openFixture(t, `{"topic":"ml","dur":1500}
{"topic":"ml","dur":900}
{"topic":"go","dur":1500}
`)
	all := c.Where(pred(func(Doc) bool { return true }))

	groups := all.GroupBy("topic")
	if groups["ml"].Count() != 2 || groups["go"].Count() != 1 {
		t.Errorf("GroupBy wrong: ml=%d go=%d", groups["ml"].Count(), groups["go"].Count())
	}
	if sum, ok := groups["ml"].Sum("dur"); !ok || sum != 2400 {
		t.Errorf("ml Sum = %v, %v", sum, ok)
	}
	if cb := all.CountBy("topic"); cb["ml"] != 2 {
		t.Errorf("CountBy ml = %d", cb["ml"])
	}
	if d := all.Distinct("topic"); len(d) != 2 {
		t.Errorf("Distinct = %v, want 2", d)
	}
	if avg, ok := all.Avg("dur"); !ok || avg != 1300 {
		t.Errorf("Avg = %v", avg)
	}
	if mn, ok := all.Min("dur"); !ok || mn != 900 {
		t.Errorf("Min = %v", mn)
	}
	if mx, ok := all.Max("dur"); !ok || mx != 1500 {
		t.Errorf("Max = %v", mx)
	}
	// GroupByFunc: bucket by dur>=1500
	gf := all.GroupByFunc(func(d Doc) string {
		if v, _ := d.GetInt("dur"); v >= 1500 {
			return "long"
		}
		return "short"
	})
	if gf["long"].Count() != 2 || gf["short"].Count() != 1 {
		t.Errorf("GroupByFunc wrong")
	}
}

func TestAggregationIndexBacked(t *testing.T) {
	c := openFixture(t, `{"topic":"ml","dur":1500}
{"topic":"ml","dur":900}
{"topic":"go","dur":1500}
`)
	all := c.Where(predTrue())
	if sum, ok := all.Sum("dur"); !ok || sum != 3900 {
		t.Errorf("Sum = %v ok=%v, want 3900", sum, ok)
	}
	grp := all.GroupBy("topic")
	if grp["ml"].Count() != 2 || grp["go"].Count() != 1 {
		t.Errorf("GroupBy counts wrong: ml=%d go=%d", grp["ml"].Count(), grp["go"].Count())
	}
}
