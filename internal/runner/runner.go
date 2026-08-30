//go:build unix

// Package runner executes one instance-declared command and reports what
// happened with enough fidelity to name failures accurately.
//
// Commands are human-authored shell strings (see
// docs/reference/instance-toml.md), run via sh -c with the environment the
// harness itself was launched from — never constructed, so the command sees
// exactly the PATH the learner's terminal has. The runner is
// language-agnostic: nothing in it knows what command it is running.
//
// The package is unix-only, per the build tag: killing a timed-out command's
// whole process group is part of the contract, and native Windows is out of
// scope — WSL2 is the Windows story (docs/design.md).
package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// DefaultOutputLimit bounds captured output when Spec.OutputLimit is zero.
const DefaultOutputLimit = 64 * 1024

// termGrace is how long a cancelled command gets to shut down after SIGTERM
// before its whole process group is SIGKILLed. A hard kill first would leave
// half-written caches and build output in the learner's working copy. Kept
// below waitDelay so the escalation always lands before Wait gives up on the
// pipes — a command that ignores SIGTERM must not outlive Run.
const termGrace = 1 * time.Second

// waitDelay backstops the output pipes: if the process group somehow
// survives both signals, Wait is forcibly released this long after
// cancellation instead of hanging on an inherited pipe.
const waitDelay = 2 * time.Second

// Outcome classifies what happened to a command.
type Outcome int

const (
	// outcomeInvalid is the deliberate zero value, never assigned by Run: a
	// zero Result — what a caller holds alongside a non-nil error — must not
	// read as success.
	outcomeInvalid Outcome = iota
	// Succeeded: ran and exited zero.
	Succeeded
	// Failed: ran and did not succeed — a nonzero exit whose verdict is in
	// Result.Output, or a kill by a signal the harness did not send, named
	// in Result.Signal.
	Failed
	// NotStartable: the command could not run at all — the binary is not
	// installed (exit 127), not executable (126), or the shell itself could
	// not start. The "pnpm is not installed" case.
	NotStartable
	// TimedOut: Spec.Timeout elapsed and the command was shut down — SIGTERM
	// first, SIGKILL to the whole process group after a grace period.
	TimedOut
)

// String returns the outcome's name for logs and test failures.
func (o Outcome) String() string {
	switch o {
	case outcomeInvalid:
		return "invalid"
	case Succeeded:
		return "succeeded"
	case Failed:
		return "failed"
	case NotStartable:
		return "not startable"
	case TimedOut:
		return "timed out"
	default:
		return fmt.Sprintf("Outcome(%d)", int(o))
	}
}

// Spec describes one command to run.
type Spec struct {
	// Command is the shell string, passed to sh -c verbatim.
	Command string
	// Dir is the working directory; empty means the caller's.
	Dir string
	// Timeout shuts the command down when exceeded: SIGTERM, then SIGKILL
	// to the whole process group after a grace period. Zero means no
	// timeout.
	Timeout time.Duration
	// OutputLimit caps how many bytes of combined output are kept.
	// Zero means DefaultOutputLimit.
	OutputLimit int
}

// Result is what happened. It is a report, not an error: every command-level
// failure mode is a valid observation for the caller to render.
type Result struct {
	Outcome Outcome
	// ExitCode is the raw code the process exited with: 0 for Succeeded,
	// the honest nonzero code for Failed and NotStartable (127 is a shell
	// convention, not a guarantee, so it is preserved rather than hidden
	// behind the classification), and -1 when the process was killed — see
	// Signal — or never started.
	ExitCode int
	// Signal is set when a signal ended the process: the harness's own
	// SIGTERM or SIGKILL for TimedOut, an external killer — an OOM kill, a
	// stray Ctrl-C — for Failed with ExitCode -1. Zero when the process
	// exited on its own.
	Signal syscall.Signal
	// Output is combined stdout and stderr, interleaved, bounded to the
	// tail — the end is where toolchains put the verdict. For a command
	// that never started it carries the start error instead.
	Output    []byte
	Truncated bool
	Duration  time.Duration
}

// Run executes spec and reports the outcome. The returned error is reserved
// for the caller's own cancellation via ctx and for harness-internal
// failures; everything that is a fact about the command is in Result.
func Run(ctx context.Context, spec Spec) (Result, error) {
	limit := spec.OutputLimit
	if limit <= 0 {
		limit = DefaultOutputLimit
	}
	out := &tailBuffer{limit: limit}

	// The duration clock starts with the timeout clock, before exec.Command
	// walks PATH, so a timed-out Result never reports Duration < Timeout.
	start := time.Now()

	runCtx := ctx
	if spec.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, spec.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(runCtx, "sh", "-c", spec.Command)
	cmd.Dir = spec.Dir
	cmd.Stdout = out
	cmd.Stderr = out
	// A new process group, so cancellation can kill the command's children
	// (sh → pnpm → node) and not just sh. Killing only sh leaves
	// grandchildren holding the output pipe, and Wait would block on it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// done stops the SIGKILL escalation once Wait has returned, so a kill
	// can never land on a recycled process group id.
	done := make(chan struct{})
	cmd.Cancel = func() error {
		// SIGTERM first: the command may be mid-write to caches or build
		// output in the learner's working copy, and a hard kill would
		// leave them corrupt. The group is SIGKILLed only if it outlives
		// the grace period.
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err == nil {
			go func() {
				select {
				case <-done:
				case <-time.After(termGrace):
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				}
			}()
		}
		return err
	}
	cmd.WaitDelay = waitDelay

	err := cmd.Run()
	close(done)
	res := Result{
		Output:    out.bytes(),
		Truncated: out.truncated,
		Duration:  time.Since(start),
	}

	// One decision, every arm in one place: each way cmd.Run can report
	// maps to exactly one Outcome — or, for the caller's own cancellation,
	// to an error.
	var exit *exec.ExitError
	switch {
	case err == nil:
		res.Outcome = Succeeded

	case errors.Is(err, exec.ErrWaitDelay) && runCtx.Err() == nil:
		// The process exited zero, but something it spawned — a daemon, a
		// watcher — held the output pipe past waitDelay and the copy was
		// abandoned. The command itself succeeded; output written after
		// the abandonment is silently absent, which Truncated cannot see.
		// With the deadline expired this same error would mean a graceful
		// exit on the harness's SIGTERM — a timeout, not a success —
		// hence the runCtx guard.
		res.Outcome = Succeeded

	case ctx.Err() != nil:
		// The caller's context ended — their intent, not a fact about the
		// command, so it propagates as an error.
		return Result{}, ctx.Err()

	case errors.As(err, &exit):
		// A real exit status outranks an expired deadline: a command that
		// finished just under the wire has a verdict, and it is not
		// "timed out". Only a signal death is attributed to the timeout.
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			// The harness's own kill, if the deadline had fired;
			// otherwise an external killer — an OOM kill, a stray
			// Ctrl-C — and the signal itself is the diagnosis.
			res.ExitCode = -1
			res.Signal = status.Signal()
			if runCtx.Err() != nil {
				res.Outcome = TimedOut
			} else {
				res.Outcome = Failed
			}
		} else if code := exit.ExitCode(); code == 127 || code == 126 {
			// 127 is sh for "command not found", 126 for "found but not
			// executable" — the difference between a broken change and a
			// tool that was never installed.
			res.ExitCode = code
			res.Outcome = NotStartable
		} else {
			res.ExitCode = code
			res.Outcome = Failed
		}

	case runCtx.Err() != nil:
		// The deadline fired and the process left no exit status of its
		// own: it shut down cleanly on the harness's SIGTERM, or was gone
		// before it could start.
		res.Outcome = TimedOut
		res.ExitCode = -1

	default:
		// sh itself never started. Report it like any other not-startable
		// command, with the start error as the output so the caller has
		// something concrete to display.
		res.Outcome = NotStartable
		res.ExitCode = -1
		res.Output = []byte(err.Error())
		res.Truncated = false
	}
	return res, nil
}

// tailBuffer keeps the last limit bytes written to it. os/exec guarantees at
// most one goroutine calls Write when Stdout and Stderr are the same writer,
// so no locking is needed.
type tailBuffer struct {
	buf       []byte
	limit     int
	truncated bool
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.limit {
		t.buf = t.buf[len(t.buf)-t.limit:]
		t.truncated = true
	}
	return len(p), nil
}

func (t *tailBuffer) bytes() []byte {
	return t.buf
}
