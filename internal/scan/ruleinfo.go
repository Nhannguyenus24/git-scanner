package scan

import (
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode"
)

// RuleInfo describes one rule for people rather than for the engine: what the
// credential looks like and what the rule anchors on. It backs the provider
// detail view, so it is derived from the compiled pattern and cannot drift from
// what the engine actually matches.
type RuleInfo struct {
	ID       string
	Severity string
	// Anchor is what makes a match trustworthy: "prefix" (the value names its
	// provider), "setting" (the setting name before it does), "url" (a
	// connection URL with the password inside), "block" (a PEM block) or
	// "shape" (only the value's layout).
	Anchor string
	// Shape is the secret span of the pattern in regex notation, e.g.
	// ghp_[0-9A-Za-z]{36}. Example is the same span drawn with literals kept
	// and every variable character shown as "•" — a picture of the layout that
	// carries no real value.
	Shape    string
	Example  string
	Literals []string
	Keywords []string
}

// exampleCap bounds how many "•" one repeat contributes, so a 140-character
// token does not dominate the view; the shape keeps the exact count.
const exampleCap = 48

// Info derives the rule's human-facing description from its pattern.
func (r *Rule) Info() RuleInfo {
	info := RuleInfo{
		ID: r.ID, Severity: r.Severity, Anchor: "shape",
		Literals: r.Literals, Keywords: r.Keywords,
	}
	re, err := syntax.Parse(r.re.String(), syntax.Perl)
	if err != nil {
		return info
	}
	secret, before := findSecret(re)
	if secret == nil {
		return info
	}
	info.Shape = shape(secret)
	info.Example = example(secret)

	switch {
	case strings.Contains(info.Example, "-----BEGIN"):
		info.Anchor = "block"
	case strings.Contains(info.Example, "://"):
		info.Anchor = "url"
	case hasWord(before):
		info.Anchor = "setting"
	case fixedPrefix(info.Example) >= 2:
		info.Anchor = "prefix"
	}
	return info
}

// findSecret returns the "secret" capture and the nodes matched before it at
// the top level, which is the context a setting-anchored rule requires.
func findSecret(re *syntax.Regexp) (*syntax.Regexp, []*syntax.Regexp) {
	if re.Op == syntax.OpCapture && re.Name == "secret" {
		return re.Sub[0], nil
	}
	if re.Op == syntax.OpConcat {
		for i, sub := range re.Sub {
			if s, inner := findSecret(sub); s != nil {
				return s, append(re.Sub[:i:i], inner...)
			}
		}
	}
	for _, sub := range re.Sub {
		if s, _ := findSecret(sub); s != nil {
			return s, nil
		}
	}
	return nil, nil
}

// hasWord reports whether the context holds a literal of two or more letters —
// a setting name. A one-character boundary class around a token is not one.
func hasWord(nodes []*syntax.Regexp) bool {
	for _, n := range nodes {
		if n.Op == syntax.OpLiteral && len(n.Rune) >= 2 {
			return true
		}
		if hasWord(n.Sub) {
			return true
		}
	}
	return false
}

// fixedPrefix counts the characters an example starts with before its first
// variable one. Reading it off the drawing rather than the parse tree matters:
// the parser factors AKIA|ASIA into A(KIA|SIA), which no longer starts with a
// literal of its own.
func fixedPrefix(example string) int {
	n := 0
	for _, c := range example {
		if c == '•' {
			break
		}
		n++
	}
	return n
}

func shape(re *syntax.Regexp) string {
	switch re.Op {
	case syntax.OpLiteral:
		return syntaxEscape(string(re.Rune))
	case syntax.OpCharClass:
		return classString(re.Rune)
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return "."
	case syntax.OpCapture:
		return shape(re.Sub[0])
	case syntax.OpConcat:
		var b strings.Builder
		for _, sub := range re.Sub {
			b.WriteString(shape(sub))
		}
		return b.String()
	case syntax.OpAlternate:
		parts := make([]string, len(re.Sub))
		for i, sub := range re.Sub {
			parts[i] = shape(sub)
		}
		return "(" + strings.Join(parts, "|") + ")"
	case syntax.OpStar:
		return group(re.Sub[0]) + "*"
	case syntax.OpPlus:
		return group(re.Sub[0]) + "+"
	case syntax.OpQuest:
		return group(re.Sub[0]) + "?"
	case syntax.OpRepeat:
		q := "{" + strconv.Itoa(re.Min)
		switch {
		case re.Max == -1:
			q += ",}"
		case re.Max != re.Min:
			q += "," + strconv.Itoa(re.Max) + "}"
		default:
			q += "}"
		}
		return group(re.Sub[0]) + q
	}
	return ""
}

// group wraps a multi-character operand so a quantifier applies to all of it.
func group(re *syntax.Regexp) string {
	s := shape(re)
	if re.Op == syntax.OpLiteral && len(re.Rune) > 1 || re.Op == syntax.OpConcat {
		return "(" + s + ")"
	}
	return s
}

func classString(r []rune) string {
	if len(r) == 2 && r[0] == r[1] {
		return syntaxEscape(string(r[0]))
	}
	var b strings.Builder
	b.WriteByte('[')
	if len(r) > 0 && r[0] == 0 && r[len(r)-1] == unicode.MaxRune {
		// A negated class: print the runes it leaves out.
		b.WriteByte('^')
		for i := 1; i+1 < len(r); i += 2 {
			writeRange(&b, r[i]+1, r[i+1]-1)
		}
	} else {
		// Case folding adds non-ASCII look-alikes (ſ for s, K for K); they
		// are matched but would only confuse a reader.
		for i := 0; i+1 < len(r); i += 2 {
			if r[i] > unicode.MaxASCII {
				continue
			}
			writeRange(&b, r[i], min(r[i+1], unicode.MaxASCII))
		}
	}
	b.WriteByte(']')
	return b.String()
}

func writeRange(b *strings.Builder, lo, hi rune) {
	esc := func(c rune) string {
		switch c {
		case '\n':
			return `\n`
		case '\t':
			return `\t`
		case '\r':
			return `\r`
		case '\f':
			return `\f`
		case '\v':
			return `\v`
		case ' ':
			return ` `
		case '-', ']', '^', '\\':
			return `\` + string(c)
		}
		return string(c)
	}
	switch {
	case lo == hi:
		b.WriteString(esc(lo))
	case hi == lo+1:
		b.WriteString(esc(lo) + esc(hi))
	default:
		b.WriteString(esc(lo) + "-" + esc(hi))
	}
}

func syntaxEscape(s string) string {
	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(`\.+*?()|[]{}^$`, c) {
			b.WriteByte('\\')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// example draws the span: literals as written, a fixed character class as its
// first rune, and every variable position as "•". Optional parts are left out.
func example(re *syntax.Regexp) string {
	switch re.Op {
	case syntax.OpLiteral:
		return string(re.Rune)
	case syntax.OpCharClass:
		if len(re.Rune) == 2 && re.Rune[0] == re.Rune[1] {
			return string(re.Rune[0])
		}
		return "•"
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return "•"
	case syntax.OpCapture:
		return example(re.Sub[0])
	case syntax.OpConcat:
		var b strings.Builder
		for _, sub := range re.Sub {
			b.WriteString(example(sub))
		}
		return b.String()
	case syntax.OpAlternate:
		return example(re.Sub[0])
	case syntax.OpPlus:
		return repeatExample(re.Sub[0], 4)
	case syntax.OpRepeat:
		return repeatExample(re.Sub[0], re.Min)
	}
	// Star, Quest, anchors and boundaries contribute nothing.
	return ""
}

func repeatExample(sub *syntax.Regexp, n int) string {
	one := example(sub)
	if one != "•" {
		return strings.Repeat(one, min(n, 3))
	}
	if n > exampleCap {
		return strings.Repeat("•", exampleCap) + "…"
	}
	return strings.Repeat("•", n)
}
