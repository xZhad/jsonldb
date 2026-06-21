package jsonldb

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// token kinds
type tokKind int

const (
	tEOF tokKind = iota
	tLParen
	tRParen
	tBang
	tOr     // |=
	tClause // raw clause text "key op value" or "key"
)

type token struct {
	kind tokKind
	text string
}

func lexDSL(s string) ([]token, error) {
	var toks []token
	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '(':
			toks = append(toks, token{tLParen, "("})
			i++
		case c == ')':
			toks = append(toks, token{tRParen, ")"})
			i++
		case c == '!':
			toks = append(toks, token{tBang, "!"})
			i++
		case c == '|' && i+1 < n && s[i+1] == '=':
			toks = append(toks, token{tOr, "|="})
			i += 2
		default:
			// read a clause: until whitespace/paren/|= at top level, honoring quotes
			start := i
			for i < n {
				ch := s[i]
				if ch == '"' { // consume quoted segment incl. escapes
					i++
					for i < n && s[i] != '"' {
						if s[i] == '\\' {
							i++
						}
						i++
					}
					i++
					continue
				}
				if ch == ' ' || ch == '\t' || ch == '(' || ch == ')' {
					break
				}
				if ch == '|' && i+1 < n && s[i+1] == '=' {
					break
				}
				i++
			}
			text := strings.TrimSpace(s[start:i])
			if text != "" {
				toks = append(toks, token{tClause, text})
			}
		}
	}
	toks = append(toks, token{tEOF, ""})
	return toks, nil
}

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() token { return p.toks[p.pos] }
func (p *parser) next() token { t := p.toks[p.pos]; p.pos++; return t }

func parseDSL(input string) (Predicate, error) {
	if strings.TrimSpace(input) == "" {
		return func(Doc) bool { return true }, nil
	}
	toks, err := lexDSL(input)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	pred, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tEOF {
		return nil, fmt.Errorf("unexpected token %q", p.peek().text)
	}
	return pred, nil
}

func (p *parser) parseOr() (Predicate, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tOr {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = Or(left, right)
	}
	return left, nil
}

func (p *parser) parseAnd() (Predicate, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for {
		k := p.peek().kind
		if k == tBang || k == tLParen || k == tClause {
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = And(left, right)
			continue
		}
		break
	}
	return left, nil
}

func (p *parser) parseNot() (Predicate, error) {
	if p.peek().kind == tBang {
		p.next()
		inner, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return Not(inner), nil
	}
	return p.parseAtom()
}

func (p *parser) parseAtom() (Predicate, error) {
	switch p.peek().kind {
	case tLParen:
		p.next()
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tRParen {
			return nil, fmt.Errorf("expected ')'")
		}
		p.next()
		return inner, nil
	case tClause:
		return clauseToPredicate(p.next().text)
	default:
		return nil, fmt.Errorf("unexpected token %q", p.peek().text)
	}
}

// operator table, longest-match first
var dslOps = []struct {
	op string
	mk func(k string, raw string) (Predicate, error)
}{
	{"!=", func(k, raw string) (Predicate, error) { return Ne(k, parseValue(raw)), nil }},
	{">=", func(k, raw string) (Predicate, error) { return Gte(k, parseValue(raw)), nil }},
	{"<=", func(k, raw string) (Predicate, error) { return Lte(k, parseValue(raw)), nil }},
	{"~=", func(k, raw string) (Predicate, error) { return Contains(k, unquote(raw)), nil }},
	{"^=", func(k, raw string) (Predicate, error) { return Prefix(k, unquote(raw)), nil }},
	{"$=", func(k, raw string) (Predicate, error) { return Suffix(k, unquote(raw)), nil }},
	{"=~", func(k, raw string) (Predicate, error) { return Regex(k, unquote(raw)), nil }},
	{"=", func(k, raw string) (Predicate, error) { return Eq(k, parseValue(raw)), nil }},
	{">", func(k, raw string) (Predicate, error) { return Gt(k, parseValue(raw)), nil }},
	{"<", func(k, raw string) (Predicate, error) { return Lt(k, parseValue(raw)), nil }},
}

func clauseToPredicate(clause string) (Predicate, error) {
	for _, o := range dslOps {
		if idx := indexOpOutsideQuotes(clause, o.op); idx >= 0 {
			key := strings.TrimSpace(clause[:idx])
			raw := strings.TrimSpace(clause[idx+len(o.op):])
			if key == "" {
				return nil, fmt.Errorf("empty key in %q", clause)
			}
			if raw == "" {
				return nil, fmt.Errorf("empty value in %q", clause)
			}
			return o.mk(key, raw)
		}
	}
	// bare key = HasKey
	if strings.ContainsAny(clause, "=<>~^$!") {
		return nil, fmt.Errorf("malformed clause %q", clause)
	}
	return HasKey(clause), nil
}

func indexOpOutsideQuotes(s, op string) int {
	inQuote := false
	for i := 0; i+len(op) <= len(s); i++ {
		if s[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && s[i:i+len(op)] == op {
			return i
		}
	}
	return -1
}

func unquote(raw string) string {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		var s string
		if err := json.Unmarshal([]byte(raw), &s); err == nil {
			return s
		}
		return raw[1 : len(raw)-1]
	}
	return raw
}

// parseValue applies DSL value typing.
func parseValue(raw string) any {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return unquote(raw) // forced string
	}
	switch raw {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if isNumber(raw) {
		return json.Number(raw)
	}
	return raw // bare string (RFC3339 strings handled by coerce at compare time)
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	dot := false
	for i, r := range s {
		switch {
		case r == '-' && i == 0:
		case r == '.' && !dot:
			dot = true
		case unicode.IsDigit(r):
		default:
			return false
		}
	}
	return s != "-" && s != "."
}
