// Package doctor assembles and renders rolling doctor's two-section
// readiness report, documented in docs/reference/rolling-doctor.md. It
// renders what it is given: the package knows nothing about cobra, signals,
// or the filesystem, and decides nothing except how a finding is worded and
// whether the harness section blocks.
package doctor

import (
	"github.com/rollingstart-dev/rollingstart/internal/instance"
	"github.com/rollingstart-dev/rollingstart/internal/probe"
	"github.com/rollingstart-dev/rollingstart/internal/runner"
)

// Report is one doctor run. Harness holds the probes that ran, in the order
// they ran; Instance is the second section in whichever state it reached. A
// report nothing was written into renders as nothing and blocks — see
// Blocking.
type Report struct {
	Harness  []probe.Result
	Instance InstanceSection
}

// InstanceState is which of the second section's states applies.
type InstanceState int

const (
	// StateNotRun is the deliberate zero value: the section never started
	// — doctor was interrupted first — and is not rendered at all.
	StateNotRun InstanceState = iota
	// StateSkipped: the definition did not load; Reason says why, in the
	// loader's own positioned words.
	StateSkipped
	// StateNothingDeclared: the definition loaded and declares no commands
	// — a valid state, rendered explicitly so it never reads as green.
	StateNothingDeclared
	// StateRan: the declared commands ran; Commands has one row each.
	StateRan
)

// InstanceSection is the second section. Build one with Skipped,
// NothingDeclared, or Ran; the zero value is StateNotRun.
type InstanceSection struct {
	State InstanceState
	// Reason is the loader's error, verbatim, for StateSkipped. It may span
	// lines when several keys were unknown.
	Reason string
	// Commands is one row per declared command in canonical order, for
	// StateRan.
	Commands []CommandRow
}

// CommandRow pairs a declared command with what happened when it ran.
type CommandRow struct {
	Command instance.Command
	Result  runner.Result
}

// Skipped is the instance section when the definition did not load.
func Skipped(reason string) InstanceSection {
	return InstanceSection{State: StateSkipped, Reason: reason}
}

// NothingDeclared is the instance section for a definition that declares
// no commands.
func NothingDeclared() InstanceSection {
	return InstanceSection{State: StateNothingDeclared}
}

// Ran is the instance section after the declared commands ran. rows must
// hold at least one: a definition that declares nothing is NothingDeclared,
// and Render refuses an empty Ran rather than print a section that reads as
// green.
func Ran(rows []CommandRow) InstanceSection {
	return InstanceSection{State: StateRan, Commands: rows}
}

// Blocking reports whether the harness section stops the harness: any probe
// that is not green, or no probe at all — a run that verified nothing has
// not earned a zero exit. The instance section never counts; red there is
// informational (roadmap § 2.6). The exit decision lives here so the
// command has nothing to interpret.
func (r Report) Blocking() bool {
	if len(r.Harness) == 0 {
		return true
	}
	for _, res := range r.Harness {
		if res.Status != probe.Green {
			return true
		}
	}
	return false
}
