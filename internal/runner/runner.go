// Package runner executes one instance-declared command and reports what
// happened with enough fidelity to name failures accurately.
//
// Commands are human-authored shell strings (see
// docs/reference/instance-toml.md), run via sh -c with the environment the
// harness itself was launched from — never constructed, so the command sees
// exactly the PATH the learner's terminal has. The runner is
// language-agnostic: nothing in it knows what command it is running.
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

// waitDelay backstops the output pipes: if the process group somehow
// survives the kill, Wait is forcibly released this long after cancellation
// instead of hanging on an inherited pipe.
const waitDelay = 2 * time.Second

// Outcome classifies what happened to a command.
type Outcome int

const (
	// Succeeded: ran and exited zero.
	Succeeded Outcome = iota
	// Failed: ran and exited nonzero. The toolchain has a verdict, and it
	// is in Result.Output.
	Failed
	// NotStartable: the command could not run at all — the binary is not
	// installed (exit 127), not executable (126), or the shell itself could
	// not start. The "pnpm is not installed" case.
	NotStartable
	// TimedOut: killed after Spec.Timeout elapsed.
	TimedOut
)

// String returns the outcome's name for logs and test failures.
func (o Outcome) String() string {
	switch o {
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
	// Timeout kills the command's whole process group when exceeded.
	// Zero means no timeout.
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
	// behind the classification), and -1 when the process was killed or
	// never started.
	ExitCode int
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
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	cmd.WaitDelay = waitDelay

	start := time.Now()
	err := cmd.Run()
	res := Result{
		ExitCode:  0,
		Output:    out.bytes(),
		Truncated: out.truncated,
		Duration:  time.Since(start),
	}

	switch {
	case err == nil:
		res.Outcome = Succeeded
		return res, nil

	case ctx.Err() != nil:
		// The caller's context ended — their intent, not a fact about the
		// command, so it propagates as an error.
		return Result{}, ctx.Err()

	case runCtx.Err() != nil:
		res.Outcome = TimedOut
		res.ExitCode = -1
		return res, nil
	}

	var exit *exec.ExitError
	if errors.As(err, &exit) {
		res.ExitCode = exit.ExitCode()
		// 127 is sh for "command not found", 126 for "found but not
		// executable" — the difference between a broken change and a tool
		// that was never installed.
		if res.ExitCode == 127 || res.ExitCode == 126 {
			res.Outcome = NotStartable
		} else {
			res.Outcome = Failed
		}
		return res, nil
	}

	// sh itself never started. Report it like any other not-startable
	// command, with the start error as the output so the caller has
	// something concrete to display.
	res.Outcome = NotStartable
	res.ExitCode = -1
	res.Output = []byte(err.Error())
	res.Truncated = false
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
