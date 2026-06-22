package jsonldb

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// Predicate decides whether a Doc matches. It also carries an optional
// raw-bytes pre-filter (rawReject) used to skip lines without parsing.
type Predicate struct {
	match  func(Doc) bool
	reject func(raw []byte) bool // nil ⇒ cannot pre-reject
}

// Match reports whether d satisfies the predicate.
func (p Predicate) Match(d Doc) bool { return p.match(d) }

// rawReject reports whether the raw line definitely cannot match (never a
// false positive). nil reject ⇒ false (unknown ⇒ must parse).
func (p Predicate) rawReject(raw []byte) bool {
	if p.reject == nil {
		return false
	}
	return p.reject(raw)
}

func pred(match func(Doc) bool) Predicate { return Predicate{match: match} }

func predR(match func(Doc) bool, reject func([]byte) bool) Predicate {
	return Predicate{match: match, reject: reject}
}

// rawContainsFold reports whether raw contains needle, case-insensitively.
func rawContainsFold(raw []byte, needle string) bool {
	return bytes.Contains(bytes.ToLower(raw), bytes.ToLower([]byte(needle)))
}

// valueToken returns the literal substring that MUST appear in the raw JSON
// if a string/number value is present. Returns "" when no safe token exists.
func valueToken(v any) string {
	switch x := v.(type) {
	case string:
		return x // the string content appears within the quoted value
	case json.Number:
		return x.String()
	case bool, nil:
		return "" // "true"/"false"/"null" are too common to pre-filter safely
	}
	return ""
}

func Eq(k string, v any) Predicate {
	tok := valueToken(v) // "" ⇒ cannot pre-filter
	return predR(
		func(d Doc) bool { x, ok := d.Get(k); return ok && equalValues(x, v) },
		func(raw []byte) bool {
			if tok == "" {
				return false
			}
			return !bytes.Contains(raw, []byte(tok))
		},
	)
}
func Ne(k string, v any) Predicate {
	return pred(func(d Doc) bool { x, ok := d.Get(k); return !ok || !equalValues(x, v) })
}

func ordered(k string, v any, want func(c int) bool) Predicate {
	return pred(func(d Doc) bool {
		x, ok := d.Get(k)
		if !ok {
			return false
		}
		c, ok := compareValues(x, v)
		return ok && want(c)
	})
}

func Gt(k string, v any) Predicate  { return ordered(k, v, func(c int) bool { return c > 0 }) }
func Gte(k string, v any) Predicate { return ordered(k, v, func(c int) bool { return c >= 0 }) }
func Lt(k string, v any) Predicate  { return ordered(k, v, func(c int) bool { return c < 0 }) }
func Lte(k string, v any) Predicate { return ordered(k, v, func(c int) bool { return c <= 0 }) }

func Contains(k, substr string) Predicate {
	want := strings.ToLower(substr)
	return predR(
		func(d Doc) bool { return strings.Contains(strings.ToLower(d.GetString(k)), want) },
		func(raw []byte) bool {
			if want == "" {
				return false
			}
			return !rawContainsFold(raw, substr)
		},
	)
}

func HasKey(k string) Predicate {
	keyTok := `"` + k + `"`
	return predR(
		func(d Doc) bool { return d.Has(k) },
		func(raw []byte) bool { return !bytes.Contains(raw, []byte(keyTok)) },
	)
}

func Prefix(k, p string) Predicate {
	return predR(
		func(d Doc) bool { return strings.HasPrefix(d.GetString(k), p) },
		func(raw []byte) bool {
			if p == "" {
				return false
			}
			return !bytes.Contains(raw, []byte(p))
		},
	)
}
func Suffix(k, s string) Predicate {
	return predR(
		func(d Doc) bool { return strings.HasSuffix(d.GetString(k), s) },
		func(raw []byte) bool {
			if s == "" {
				return false
			}
			return !bytes.Contains(raw, []byte(s))
		},
	)
}

func Regex(k, pattern string) Predicate {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return pred(func(Doc) bool { return false })
	}
	return pred(func(d Doc) bool { return re.MatchString(d.GetString(k)) })
}

func In(k string, vs ...any) Predicate {
	return pred(func(d Doc) bool {
		x, ok := d.Get(k)
		if !ok {
			return false
		}
		for _, v := range vs {
			if equalValues(x, v) {
				return true
			}
		}
		return false
	})
}

func Between(k string, lo, hi any) Predicate { return And(Gte(k, lo), Lte(k, hi)) }

func And(ps ...Predicate) Predicate {
	return predR(
		func(d Doc) bool {
			for _, p := range ps {
				if !p.Match(d) {
					return false
				}
			}
			return true
		},
		func(raw []byte) bool { // reject if ANY child rejects
			for _, p := range ps {
				if p.rawReject(raw) {
					return true
				}
			}
			return false
		},
	)
}
func Or(ps ...Predicate) Predicate {
	return predR(
		func(d Doc) bool {
			for _, p := range ps {
				if p.Match(d) {
					return true
				}
			}
			return false
		},
		func(raw []byte) bool { // reject only if ALL children can reject AND do
			if len(ps) == 0 {
				return false
			}
			for _, p := range ps {
				if p.reject == nil || !p.reject(raw) {
					return false
				}
			}
			return true
		},
	)
}
func Not(p Predicate) Predicate { return pred(func(d Doc) bool { return !p.Match(d) }) }
