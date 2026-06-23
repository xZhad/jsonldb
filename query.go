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

// rawFilterable reports whether s appears byte-for-byte inside JSON-encoded
// text (i.e. JSON would not escape any of its characters). Only such strings
// are safe to use as a raw-bytes pre-filter needle.
func rawFilterable(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == '"' || c == '\\' {
			return false
		}
	}
	return true
}

// valueToken returns the literal substring that MUST appear in the raw JSON
// if a string/number value is present. Returns "" when no safe token exists.
//
// Safety principle: a token is only valid if it is a GUARANTEED byte-substring
// of the canonical JSON encoding of the value. Unescaped strings satisfy this
// (guarded by rawFilterable). Numbers do NOT: the stored form is not
// canonicalized — 1500, 1.5e3, 15e2, 1500.0 all compare equal as floats but
// share no common literal substring. Returning "" for numbers disables the
// pre-filter, ensuring Eq never silently drops a true numeric match.
func valueToken(v any) string {
	switch x := v.(type) {
	case string:
		if !rawFilterable(x) {
			return "" // string needs JSON escaping; disable pre-filter
		}
		return x // the string content appears within the quoted value
	case json.Number:
		_ = x
		return "" // numbers are not byte-canonical; never pre-filter
	case bool, nil:
		return "" // "true"/"false"/"null" are too common to pre-filter safely
	}
	return ""
}

func Eq(k string, v any) Predicate {
	tok := valueToken(v) // "" ⇒ cannot pre-filter
	return predR(
		func(d Doc) bool { x, ok := d.getField(k); return ok && equalValues(x, v) },
		func(raw []byte) bool {
			if tok == "" {
				return false
			}
			return !bytes.Contains(raw, []byte(tok))
		},
	)
}
func Ne(k string, v any) Predicate {
	return pred(func(d Doc) bool { x, ok := d.getField(k); return !ok || !equalValues(x, v) })
}

func ordered(k string, v any, want func(c int) bool) Predicate {
	return pred(func(d Doc) bool {
		x, ok := d.getField(k)
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
		func(d Doc) bool { return strings.Contains(strings.ToLower(d.getStringField(k)), want) },
		func(raw []byte) bool {
			if want == "" {
				return false
			}
			if !rawFilterable(substr) {
				return false // needle needs JSON escaping; cannot safely pre-filter
			}
			return !rawContainsFold(raw, substr)
		},
	)
}

func HasKey(k string) Predicate {
	keyTok := `"` + k + `"`
	hasReject := rawFilterable(k) && !strings.Contains(k, ".")
	return predR(
		func(d Doc) bool { _, ok := d.getField(k); return ok },
		func(raw []byte) bool {
			if !hasReject {
				return false
			}
			return !bytes.Contains(raw, []byte(keyTok))
		},
	)
}

func Prefix(k, p string) Predicate {
	return predR(
		func(d Doc) bool { return strings.HasPrefix(d.getStringField(k), p) },
		func(raw []byte) bool {
			if p == "" {
				return false
			}
			if !rawFilterable(p) {
				return false // prefix needs JSON escaping; cannot safely pre-filter
			}
			return !bytes.Contains(raw, []byte(p))
		},
	)
}
func Suffix(k, s string) Predicate {
	return predR(
		func(d Doc) bool { return strings.HasSuffix(d.getStringField(k), s) },
		func(raw []byte) bool {
			if s == "" {
				return false
			}
			if !rawFilterable(s) {
				return false // suffix needs JSON escaping; cannot safely pre-filter
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
	return pred(func(d Doc) bool { return re.MatchString(d.getStringField(k)) })
}

func In(k string, vs ...any) Predicate {
	return pred(func(d Doc) bool {
		x, ok := d.getField(k)
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
