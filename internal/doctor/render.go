package doctor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rollingstart-dev/rollingstart/internal/instance"
	"github.com/rollingstart-dev/rollingstart/internal/probe"
	"github.com/rollingstart-dev/rollingstart/internal/runner"
)

const (
	// tailLines is how much of a command's output follows its row when the
	// command is not healthy: the end is where toolchains put the verdict.
	tailLines = 20
	// commandCol is the widest the shell-string column gets before a
	// command is cut with an ellipsis. The full string is in the instance
	// definition; the column is for recognising it, not reading it.
	commandCol = 40
	// harnessStatusCol, statusCol, and nameCol are fixed — the widest
	// status word in each section and the widest canonical command name —
	// so rows line up the same way in every instance, whatever it
	// declares.
	harnessStatusCol = len("FAIL")
	statusCol        = len("not installed")
	nameCol          = len("typecheck")
)

// Render writes the report in the plain-text format of
// docs/reference/rolling-doctor.md. verbose shows every command's output,
// all of it, rather than the tail of the unhealthy ones'. An invalid report
// — a status or outcome never set, a section in an impossible state — is a
// caller bug: Render returns an error and writes nothing.
func Render(w io.Writer, r Report, verbose bool) error {
	if err := validate(r); err != nil {
		return err
	}
	var b bytes.Buffer
	renderHarness(&b, r.Harness)
	if r.Instance.State != StateNotRun {
		if len(r.Harness) > 0 {
			b.WriteString("\n")
		}
		renderInstance(&b, r.Instance, verbose)
	}
	if _, err := w.Write(b.Bytes()); err != nil {
		return fmt.Errorf("writing doctor report: %w", err)
	}
	return nil
}

// validate refuses what the renderer would otherwise have to invent a row
// for: a zero probe status or runner outcome — neither is ever produced by
// the packages that own them — or an instance section whose state and
// contents disagree.
func validate(r Report) error {
	for _, res := range r.Harness {
		if res.Status != probe.Green && res.Status != probe.Red {
			return fmt.Errorf("doctor: probe %q has status %v", res.Name, res.Status)
		}
	}
	switch r.Instance.State {
	case StateNotRun, StateNothingDeclared:
	case StateSkipped:
		if r.Instance.Reason == "" {
			return errors.New("doctor: instance section skipped without a reason")
		}
	case StateRan:
		if len(r.Instance.Commands) == 0 {
			return errors.New("doctor: instance section ran with no commands — use NothingDeclared")
		}
		for _, row := range r.Instance.Commands {
			if _, ok := outcomeWord(row.Result.Outcome); !ok {
				return fmt.Errorf("doctor: command %q has outcome %v", row.Command.Name, row.Result.Outcome)
			}
		}
	default:
		return fmt.Errorf("doctor: instance section in unknown state %d", r.Instance.State)
	}
	return nil
}

func renderHarness(b *bytes.Buffer, rows []probe.Result) {
	if len(rows) == 0 {
		return
	}
	b.WriteString("Harness preconditions\n")
	width := 0
	for _, res := range rows {
		width = max(width, utf8.RuneCountInString(res.Name))
	}
	// Two-space indent, the status column, two spaces, the name column,
	// two spaces: where a finding starts, and where a finding that spans
	// lines continues.
	col := 2 + harnessStatusCol + 2 + width + 2
	for _, res := range rows {
		status := "ok"
		if res.Status != probe.Green {
			status = "FAIL"
		}
		fmt.Fprintf(b, "  %-*s  %-*s  %s\n", harnessStatusCol, status, width, res.Name, continued(res.Message, col))
	}
}

func renderInstance(b *bytes.Buffer, s InstanceSection, verbose bool) {
	b.WriteString("Instance command health\n")
	switch s.State {
	case StateSkipped:
		const lead = "  skipped: "
		fmt.Fprintf(b, "%s%s\n", lead, continued(s.Reason, len(lead)))
	case StateNothingDeclared:
		fmt.Fprintf(b, "  nothing declared: %s declares no commands\n", instance.Path)
	case StateRan:
		width := 0
		for _, row := range s.Commands {
			width = max(width, utf8.RuneCountInString(cut(row.Command.Cmd)))
		}
		for _, row := range s.Commands {
			word, _ := outcomeWord(row.Result.Outcome)
			fmt.Fprintf(b, "  %-*s  %-*s  %-*s  %s\n",
				statusCol, word, nameCol, row.Command.Name, width, cut(row.Command.Cmd), summary(row.Result))
			if verbose || row.Result.Outcome != runner.Succeeded {
				renderTail(b, row.Result, verbose)
			}
		}
	}
}

// outcomeWord names what happened to a command — the runner's four outcomes
// in the words the milestone's exit criterion uses, because "pnpm is not
// installed" and "tests are failing" are different diagnoses and the report
// must not collapse them.
func outcomeWord(o runner.Outcome) (string, bool) {
	switch o {
	case runner.Succeeded:
		return "healthy", true
	case runner.Failed:
		return "failing", true
	case runner.NotStartable:
		return "not installed", true
	case runner.TimedOut:
		return "timed out", true
	}
	return "", false
}

// summary is the right-hand column: how long, and how it ended.
func summary(res runner.Result) string {
	d := duration(res.Duration)
	switch res.Outcome {
	case runner.Succeeded:
		return d
	case runner.Failed:
		if res.Signal != 0 {
			return fmt.Sprintf("killed by signal %d (%s) after %s", int(res.Signal), res.Signal, d)
		}
		return fmt.Sprintf("exit %d after %s", res.ExitCode, d)
	case runner.NotStartable:
		if res.ExitCode < 0 {
			// sh itself never started; the runner put the start error in
			// the output, which the tail shows.
			return "could not start"
		}
		return fmt.Sprintf("exit %d after %s", res.ExitCode, d)
	case runner.TimedOut:
		return d + ", process group terminated"
	}
	return "" // unreachable after validate
}

// duration reads as a stopwatch: tenths of a second under a minute, whole
// seconds above it. Rounded to a tenth first, so 59.96s is a minute and
// never prints as 60.0s.
func duration(d time.Duration) string {
	d = d.Round(100 * time.Millisecond)
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}

// renderTail writes a command's captured output under its row, indented.
// Two notes can precede it, and they say different things: the runner's
// own 64 KB bound dropped the head of the capture — a cut Truncated can
// see but the line count cannot, and one --verbose cannot undo, so it is
// said in both modes — and, without verbose, that only the last tailLines
// of what was captured follow, which --verbose does undo.
func renderTail(b *bytes.Buffer, res runner.Result, verbose bool) {
	out := strings.TrimSuffix(string(res.Output), "\n")
	if out == "" {
		return
	}
	ls := strings.Split(out, "\n")
	if res.Truncated {
		b.WriteString("    … (earlier output not captured)\n")
	}
	if !verbose && len(ls) > tailLines {
		n := len(ls) - tailLines
		fmt.Fprintf(b, "    … (%d more %s captured; --verbose shows all)\n", n, plural(n, "line"))
		ls = ls[len(ls)-tailLines:]
	}
	for _, l := range ls {
		b.WriteString("    ")
		b.WriteString(l)
		b.WriteString("\n")
	}
}

// cut fits a shell string to the command column: runs of whitespace — line
// breaks and the indentation after them, legal in a TOML multi-line string
// and fatal to a table — collapse to one space, and anything past the
// column is cut, ellipsis included.
func cut(cmd string) string {
	cmd = strings.Join(strings.Fields(cmd), " ")
	if utf8.RuneCountInString(cmd) <= commandCol {
		return cmd
	}
	runes := []rune(cmd)
	return string(runes[:commandCol-1]) + "…"
}

func plural(n int, noun string) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}

// continued indents every line after the first by col, so a multi-line
// finding stays inside its column.
func continued(s string, col int) string {
	return strings.ReplaceAll(s, "\n", "\n"+strings.Repeat(" ", col))
}
