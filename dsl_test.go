package jsonldb

import "testing"

func TestDSLParse(t *testing.T) {
	d := docFrom(t, `{"topic":"jsonldb spec","status":"active","duration":1500,"completed":true,"notes":[]}`)
	cases := []struct {
		dsl  string
		want bool
	}{
		{`completed=true`, true},
		{`completed=false`, false},
		{`duration>=1500`, true},
		{`duration>1500`, false},
		{`topic~=JSONL`, true},
		{`topic^=json`, true},
		{`topic$=spec`, true},
		{`topic=~^json.*spec$`, true},
		{`notes`, true},               // has-key
		{`missing`, false},            // has-key absent
		{`!completed`, false},         // NOT true
		{`completed=true topic~=spec`, true},                 // AND
		{`status=active |= status=paused`, true},             // OR
		{`status=paused |= status=done`, false},
		{`completed=true (status=paused |= status=active)`, true}, // grouping
		{`!(status=done |= status=archived)`, true},          // grouped NOT
		{`duration=1500 |= completed=false topic~=zzz`, true},// space binds tighter than |=
	}
	for _, tc := range cases {
		p, err := parseDSL(tc.dsl)
		if err != nil {
			t.Errorf("parse %q: %v", tc.dsl, err)
			continue
		}
		if got := p(d); got != tc.want {
			t.Errorf("%q = %v, want %v", tc.dsl, got, tc.want)
		}
	}
}

func TestDSLErrors(t *testing.T) {
	for _, bad := range []string{`()`, `(a=1`, `a=1)`, `a=`} {
		if _, err := parseDSL(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}
