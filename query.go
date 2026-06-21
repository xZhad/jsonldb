package jsonldb

import (
	"regexp"
	"strings"
)

// Predicate decides whether a Doc matches.
type Predicate func(Doc) bool

func Eq(k string, v any) Predicate {
	return func(d Doc) bool { x, ok := d.Get(k); return ok && equalValues(x, v) }
}

func Ne(k string, v any) Predicate {
	return func(d Doc) bool { x, ok := d.Get(k); return !ok || !equalValues(x, v) }
}

func ordered(k string, v any, want func(c int) bool) Predicate {
	return func(d Doc) bool {
		x, ok := d.Get(k)
		if !ok {
			return false
		}
		c, ok := compareValues(x, v)
		return ok && want(c)
	}
}

func Gt(k string, v any) Predicate  { return ordered(k, v, func(c int) bool { return c > 0 }) }
func Gte(k string, v any) Predicate { return ordered(k, v, func(c int) bool { return c >= 0 }) }
func Lt(k string, v any) Predicate  { return ordered(k, v, func(c int) bool { return c < 0 }) }
func Lte(k string, v any) Predicate { return ordered(k, v, func(c int) bool { return c <= 0 }) }

func Contains(k, substr string) Predicate {
	want := strings.ToLower(substr)
	return func(d Doc) bool { return strings.Contains(strings.ToLower(d.GetString(k)), want) }
}

func HasKey(k string) Predicate { return func(d Doc) bool { return d.Has(k) } }

func Prefix(k, p string) Predicate {
	return func(d Doc) bool { return strings.HasPrefix(d.GetString(k), p) }
}

func Suffix(k, s string) Predicate {
	return func(d Doc) bool { return strings.HasSuffix(d.GetString(k), s) }
}

func Regex(k, pattern string) Predicate {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return func(Doc) bool { return false }
	}
	return func(d Doc) bool { return re.MatchString(d.GetString(k)) }
}

func In(k string, vs ...any) Predicate {
	return func(d Doc) bool {
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
	}
}

func Between(k string, lo, hi any) Predicate {
	return And(Gte(k, lo), Lte(k, hi))
}

func And(ps ...Predicate) Predicate {
	return func(d Doc) bool {
		for _, p := range ps {
			if !p(d) {
				return false
			}
		}
		return true
	}
}

func Or(ps ...Predicate) Predicate {
	return func(d Doc) bool {
		for _, p := range ps {
			if p(d) {
				return true
			}
		}
		return false
	}
}

func Not(p Predicate) Predicate { return func(d Doc) bool { return !p(d) } }
