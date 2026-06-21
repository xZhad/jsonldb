package jsonldb

import "testing"

func docFrom(t *testing.T, raw string) Doc {
	t.Helper()
	d, err := parseDoc([]byte(raw), 1)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestBuilderPredicates(t *testing.T) {
	d := docFrom(t, `{"topic":"jsonldb spec","duration":1500,"completed":true}`)
	cases := []struct {
		name string
		p    Predicate
		want bool
	}{
		{"eq-num", Eq("duration", 1500), true},
		{"eq-bool", Eq("completed", true), true},
		{"ne", Ne("duration", 900), true},
		{"gt", Gt("duration", 1000), true},
		{"gte-eq", Gte("duration", 1500), true},
		{"lt-false", Lt("duration", 1500), false},
		{"contains-ci", Contains("topic", "JSONL"), true},
		{"haskey", HasKey("topic"), true},
		{"haskey-missing", HasKey("nope"), false},
		{"prefix", Prefix("topic", "json"), true},
		{"suffix", Suffix("topic", "spec"), true},
		{"regex", Regex("topic", "^json.*spec$"), true},
		{"in", In("duration", 900, 1500), true},
		{"between", Between("duration", 1000, 2000), true},
		{"and", And(Eq("completed", true), Gt("duration", 1000)), true},
		{"or", Or(Eq("duration", 1), Eq("duration", 1500)), true},
		{"not", Not(Eq("completed", false)), true},
	}
	for _, tc := range cases {
		if got := tc.p(d); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}
}
