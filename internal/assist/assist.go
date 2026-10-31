// Package assist turns a question in plain words into the dashboard's findings
// filter, with one Claude call.
//
// The model is a parser here, not a source of truth: its answer only ever
// becomes filter fields, every field is checked against the values the
// dashboard accepts before it leaves this package, and nothing it says is
// shown as a fact about the data. What is sent is the question and the list of
// filter values — never a finding, a preview or anything read from a
// repository. A question that looks like it carries a credential is refused
// before any call, because the command palette it comes from is a box people
// paste into.
package assist

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// MaxQuestion bounds what is sent: a filter request is a sentence, not a
// document.
const MaxQuestion = 300

// maxQ bounds the free-text search the model may set.
const maxQ = 80

var (
	// ErrEmpty is an empty question.
	ErrEmpty = errors.New("assist: empty question")
	// ErrTooLong is a question over MaxQuestion characters.
	ErrTooLong = errors.New("assist: question too long")
	// ErrCredentialLike is a question carrying a token shaped like a secret.
	ErrCredentialLike = errors.New("assist: question looks like it contains a credential")
	// ErrRefused is a request the model declined.
	ErrRefused = errors.New("assist: the model declined the request")
)

// Filter is the findings filter the dashboard understands. Empty means "any".
type Filter struct {
	Severity string `json:"severity"`
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Exposure string `json:"exposure"`
	Sort     string `json:"sort"`
	Q        string `json:"q"`
	// Explanation is the model's one-line reading of the question, shown so
	// the person can see how they were understood and correct it.
	Explanation string `json:"explanation"`
}

// The values each field may take; anything else is dropped, not passed on.
var (
	Severities = []string{"critical", "high", "medium", "low", "info"}
	Statuses   = []string{"open", "rotated", "false_positive", "ignored"}
	Exposures  = []string{"head", "history", "live"}
	Sorts      = []string{"recent", "oldest", "severity"}
)

// Translator turns a question into a filter.
type Translator interface {
	Translate(ctx context.Context, question string, providers []string) (Filter, error)
}

// secretShape is a run long enough to be a key that mixes letters and digits —
// the shape of nearly every API key, and of almost no word in a question.
var secretShape = regexp.MustCompile(`[A-Za-z0-9_\-+/=]{20,}`)

// CheckQuestion validates a question before it may be sent anywhere.
func CheckQuestion(q string) (string, error) {
	q = strings.TrimSpace(q)
	switch {
	case q == "":
		return "", ErrEmpty
	case utf8.RuneCountInString(q) > MaxQuestion:
		return "", ErrTooLong
	}
	for _, tok := range secretShape.FindAllString(q, -1) {
		if strings.ContainsAny(tok, "0123456789") && strings.IndexFunc(tok, isLetter) >= 0 {
			return "", ErrCredentialLike
		}
	}
	// A URL with credentials in it is also a credential.
	if strings.Contains(q, "://") && strings.Contains(q, "@") {
		return "", ErrCredentialLike
	}
	return q, nil
}

func isLetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }

// Sanitize keeps only the values the dashboard accepts. A provider must be one
// the server knows (or "unmapped"); the search text is trimmed and bounded and
// must not itself look like a credential.
func Sanitize(f Filter, providers []string) Filter {
	one := func(v string, allowed []string) string {
		v = strings.ToLower(strings.TrimSpace(v))
		if slices.Contains(allowed, v) {
			return v
		}
		return ""
	}
	out := Filter{
		Severity: one(f.Severity, Severities),
		Status:   one(f.Status, Statuses),
		Exposure: one(f.Exposure, Exposures),
		Sort:     one(f.Sort, Sorts),
		Provider: one(f.Provider, append(slices.Clone(providers), "unmapped")),
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		if r := []rune(q); len(r) > maxQ {
			q = string(r[:maxQ])
		}
		if _, err := CheckQuestion(q); err == nil {
			out.Q = q
		}
	}
	if e := strings.TrimSpace(f.Explanation); e != "" {
		if r := []rune(e); len(r) > 200 {
			e = string(r[:200])
		}
		out.Explanation = e
	}
	return out
}
