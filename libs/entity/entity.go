// Package entity holds the domain types shared across services.
//
// It imports the standard library only. No database driver, no transport, no
// generated protobuf. Those live in libs/proto and are mapped across an explicit
// boundary (see plan.md §3.3) so the wire format and the schema can evolve
// independently.
package entity

import "fmt"

// ID identifies a domain object. Generation lives in libs/id so this package
// stays free of dependencies.
type ID string

func (i ID) String() string { return string(i) }

// Severity ranks a finding. Ordered so comparisons work.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

var severityNames = map[Severity]string{
	SeverityInfo:     "info",
	SeverityLow:      "low",
	SeverityMedium:   "medium",
	SeverityHigh:     "high",
	SeverityCritical: "critical",
}

func (s Severity) String() string {
	if n, ok := severityNames[s]; ok {
		return n
	}
	return "unknown"
}

// ParseSeverity converts the stored representation back into a Severity.
func ParseSeverity(s string) (Severity, error) {
	for sev, name := range severityNames {
		if name == s {
			return sev, nil
		}
	}
	return 0, fmt.Errorf("unknown severity %q", s)
}
