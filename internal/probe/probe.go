// Package probe implements the harness-precondition checks behind rolling
// doctor's blocking section — the facts about the environment that must hold
// before any lesson can be served (roadmap § 2.6).
//
// Probes observe and report; they never render, never fix, and never depend
// on one another having run. Git probes interrogate the repository by running
// the git CLI with constructed arguments — not through internal/runner, which
// executes instance-declared shell strings: different provenance, different
// trust, different execution model.
package probe

import "fmt"

// Status classifies one probe's finding.
type Status int

const (
	// statusInvalid is the deliberate zero value, never returned by a
	// probe: a zero Result must not read as a passing check.
	statusInvalid Status = iota
	// Green: the precondition holds.
	Green
	// Red: the precondition fails. Every red in the blocking section stops
	// the harness; the caller decides how to say so.
	Red
)

// String returns the status name for logs and test failures.
func (s Status) String() string {
	switch s {
	case statusInvalid:
		return "invalid"
	case Green:
		return "green"
	case Red:
		return "red"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

// Result is one probe's finding. It is a report, not an error: a red result
// is a valid observation for the caller to render. Rendering — color,
// layout, exit codes — is entirely the caller's job.
type Result struct {
	// Name identifies the probe, stable across runs, for labelling the
	// finding.
	Name string
	// Status is the finding.
	Status Status
	// Message is a human-readable account of what was observed, suitable
	// for verbatim display. It may span multiple lines when the underlying
	// tool reported several positioned errors.
	Message string
}
