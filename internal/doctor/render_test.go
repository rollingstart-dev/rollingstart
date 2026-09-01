package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rollingstart-dev/rollingstart/internal/instance"
	"github.com/rollingstart-dev/rollingstart/internal/probe"
	"github.com/rollingstart-dev/rollingstart/internal/runner"
)

// allGreen is the harness section of a healthy checkout, in the documented
// order, with the probes' real findings.
var allGreen = []probe.Result{
	{Name: "git repository", Status: probe.Green, Message: "inside a git work tree"},
	{Name: "working tree", Status: probe.Green, Message: "working tree is clean"},
	{Name: "line endings", Status: probe.Green, Message: "core.autocrlf is unset"},
	{Name: "instance definition", Status: probe.Green, Message: "instance definition loaded (3 commands declared)"},
	{Name: "file watcher", Status: probe.Green, Message: "file events are delivered"},
}

const harnessGreen = `Harness preconditions
  ok    git repository       inside a git work tree
  ok    working tree         working tree is clean
  ok    line endings         core.autocrlf is unset
  ok    instance definition  instance definition loaded (3 commands declared)
  ok    file watcher         file events are delivered
`

const oneGreen = `Harness preconditions
  ok    git repository  inside a git work tree
`

func row(name, cmd string, res runner.Result) CommandRow {
	return CommandRow{Command: instance.Command{Name: name, Cmd: cmd}, Result: res}
}

// lines is n lines of output, each different, none ending in a space.
func lines(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString("line ")
		b.WriteString(strings.Repeat("x", i%3+1))
		b.WriteString("\n")
	}
	return b.String()
}

// tail is the last n lines of s, indented the way a rendered tail is.
func tail(s string, n int) string {
	ls := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	return indent(strings.Join(ls[len(ls)-n:], "\n") + "\n")
}

func indent(s string) string {
	var b strings.Builder
	for _, l := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		b.WriteString("    ")
		b.WriteString(l)
		b.WriteString("\n")
	}
	return b.String()
}

// goldens are the rendering cases. Their expected outputs are the reference
// page's example blocks, byte for byte — TestDocExamplesAreRendered checks
// that the page never says something the renderer does not do.
var goldens = []struct {
	name    string
	report  Report
	verbose bool
	want    string
}{
	{
		// The reference doc's leading example: every column rule at once —
		// status words, a name column sized by the longest name, a command
		// column sized by the longest shell string, the tail under anything
		// not healthy and nothing under what is.
		name: "all green harness, three outcomes",
		report: Report{
			Harness: allGreen,
			Instance: Ran([]CommandRow{
				row("build", "go build ./...", runner.Result{Outcome: runner.Succeeded, Duration: 1200 * time.Millisecond}),
				row("test", "go test ./...", runner.Result{Outcome: runner.Failed, ExitCode: 1, Duration: 4300 * time.Millisecond,
					Output: []byte("--- FAIL: TestLoad (0.00s)\n    instance_test.go:31: Load: open x: no such file or directory\nFAIL\nFAIL\tgithub.com/rollingstart-dev/rollingstart/internal/instance\t0.012s\n")}),
				row("lint", "pnpm lint", runner.Result{Outcome: runner.NotStartable, ExitCode: 127, Duration: 20 * time.Millisecond,
					Output: []byte("sh: line 1: pnpm: command not found\n")}),
			}),
		},
		want: harnessGreen + `
Instance command health
  healthy        build      go build ./...  1.2s
  failing        test       go test ./...   exit 1 after 4.3s
    --- FAIL: TestLoad (0.00s)
        instance_test.go:31: Load: open x: no such file or directory
    FAIL
    FAIL	github.com/rollingstart-dev/rollingstart/internal/instance	0.012s
  not installed  lint       pnpm lint       exit 127 after 0.0s
    sh: line 1: pnpm: command not found
`,
	},
	{
		name: "mixed harness, instance skipped",
		report: Report{
			Harness: []probe.Result{
				{Name: "git repository", Status: probe.Green, Message: "inside a git work tree"},
				{Name: "working tree", Status: probe.Red, Message: "working tree is not clean: 2 path(s) modified, staged, or untracked — commit, stash, or remove them (see git status)"},
				{Name: "line endings", Status: probe.Green, Message: "core.autocrlf is unset"},
				{Name: "instance definition", Status: probe.Red, Message: `.rollingstart/instance.toml:2:1: unknown key "biuld"`},
				{Name: "file watcher", Status: probe.Green, Message: "file events are delivered"},
			},
			Instance: Skipped(`.rollingstart/instance.toml:2:1: unknown key "biuld"`),
		},
		want: `Harness preconditions
  ok    git repository       inside a git work tree
  FAIL  working tree         working tree is not clean: 2 path(s) modified, staged, or untracked — commit, stash, or remove them (see git status)
  ok    line endings         core.autocrlf is unset
  FAIL  instance definition  .rollingstart/instance.toml:2:1: unknown key "biuld"
  ok    file watcher         file events are delivered

Instance command health
  skipped: .rollingstart/instance.toml:2:1: unknown key "biuld"
`,
	},
	{
		// The loader joins unknown keys with newlines; a finding that spans
		// lines continues under its own column, in both sections.
		name: "findings that span lines",
		report: Report{
			Harness: []probe.Result{
				{Name: "git repository", Status: probe.Green, Message: "inside a git work tree"},
				{Name: "instance definition", Status: probe.Red, Message: ".rollingstart/instance.toml:2:1: unknown key \"biuld\"\n.rollingstart/instance.toml:3:1: unknown key \"tset\""},
			},
			Instance: Skipped(".rollingstart/instance.toml:2:1: unknown key \"biuld\"\n.rollingstart/instance.toml:3:1: unknown key \"tset\""),
		},
		want: `Harness preconditions
  ok    git repository       inside a git work tree
  FAIL  instance definition  .rollingstart/instance.toml:2:1: unknown key "biuld"
                             .rollingstart/instance.toml:3:1: unknown key "tset"

Instance command health
  skipped: .rollingstart/instance.toml:2:1: unknown key "biuld"
           .rollingstart/instance.toml:3:1: unknown key "tset"
`,
	},
	{
		name: "nothing declared",
		report: Report{
			Harness:  allGreen[:1],
			Instance: NothingDeclared(),
		},
		want: oneGreen + `
Instance command health
  nothing declared: .rollingstart/instance.toml declares no commands
`,
	},
	{
		name: "timed out, killed by a signal, could not start, not executable",
		report: Report{
			Harness: allGreen[:1],
			Instance: Ran([]CommandRow{
				row("build", "make", runner.Result{Outcome: runner.Failed, ExitCode: -1, Signal: 9, Duration: 4300 * time.Millisecond, Output: []byte("working\n")}),
				row("typecheck", "./tc", runner.Result{Outcome: runner.NotStartable, ExitCode: 126, Duration: 10 * time.Millisecond, Output: []byte("sh: line 1: ./tc: Permission denied\n")}),
				row("test", "pnpm test", runner.Result{Outcome: runner.TimedOut, ExitCode: -1, Duration: 5 * time.Minute}),
				row("lint", "make lint", runner.Result{Outcome: runner.NotStartable, ExitCode: -1, Duration: 0, Output: []byte("fork/exec /bin/sh: no such file or directory")}),
			}),
		},
		want: oneGreen + `
Instance command health
  failing        build      make       killed by signal 9 (killed) after 4.3s
    working
  not installed  typecheck  ./tc       exit 126 after 0.0s
    sh: line 1: ./tc: Permission denied
  timed out      test       pnpm test  5m0s, process group terminated
  not installed  lint       make lint  could not start
    fork/exec /bin/sh: no such file or directory
`,
	},
	{
		// Twenty lines is the tail; the note says how many were cut and how
		// to see them. Healthy output stays hidden.
		name: "tail cut at twenty lines",
		report: Report{
			Harness: allGreen[:1],
			Instance: Ran([]CommandRow{
				row("build", "go build ./...", runner.Result{Outcome: runner.Succeeded, Duration: time.Second, Output: []byte("noise\n")}),
				row("test", "go test ./...", runner.Result{Outcome: runner.Failed, ExitCode: 1, Duration: time.Second, Output: []byte(lines(23))}),
			}),
		},
		want: oneGreen + `
Instance command health
  healthy        build      go build ./...  1.0s
  failing        test       go test ./...   exit 1 after 1.0s
    … (3 more lines captured; --verbose shows all)
` + tail(lines(23), 20),
	},
	{
		name: "exactly twenty lines is not cut",
		report: Report{
			Harness:  allGreen[:1],
			Instance: Ran([]CommandRow{row("test", "go test ./...", runner.Result{Outcome: runner.Failed, ExitCode: 1, Duration: time.Second, Output: []byte(lines(20))})}),
		},
		want: oneGreen + `
Instance command health
  failing        test       go test ./...  exit 1 after 1.0s
` + tail(lines(20), 20),
	},
	{
		name: "twenty-one lines cuts one line",
		report: Report{
			Harness:  allGreen[:1],
			Instance: Ran([]CommandRow{row("test", "go test ./...", runner.Result{Outcome: runner.Failed, ExitCode: 1, Duration: time.Second, Output: []byte(lines(21))})}),
		},
		want: oneGreen + `
Instance command health
  failing        test       go test ./...  exit 1 after 1.0s
    … (1 more line captured; --verbose shows all)
` + tail(lines(21), 20),
	},
	{
		// The runner's own bound dropped the head of the capture. That is
		// said in both modes — --verbose cannot bring it back — and the cut
		// note counts captured lines, promising nothing more.
		name: "bounded capture without verbose",
		report: Report{
			Harness:  allGreen[:1],
			Instance: Ran([]CommandRow{row("test", "go test ./...", runner.Result{Outcome: runner.Failed, ExitCode: 1, Duration: time.Second, Truncated: true, Output: []byte(lines(25))})}),
		},
		want: oneGreen + `
Instance command health
  failing        test       go test ./...  exit 1 after 1.0s
    … (earlier output not captured)
    … (5 more lines captured; --verbose shows all)
` + tail(lines(25), 20),
	},
	{
		// Verbose: every command's output, all of it.
		name: "verbose shows everything",
		report: Report{
			Harness: allGreen[:1],
			Instance: Ran([]CommandRow{
				row("build", "go build ./...", runner.Result{Outcome: runner.Succeeded, Duration: time.Second, Output: []byte("noise\n")}),
				row("test", "go test ./...", runner.Result{Outcome: runner.Failed, ExitCode: 1, Duration: time.Second, Truncated: true, Output: []byte(lines(22))}),
			}),
		},
		verbose: true,
		want: oneGreen + `
Instance command health
  healthy        build      go build ./...  1.0s
    noise
  failing        test       go test ./...   exit 1 after 1.0s
    … (earlier output not captured)
` + indent(lines(22)),
	},
	{
		name: "output without a trailing newline",
		report: Report{
			Harness:  allGreen[:1],
			Instance: Ran([]CommandRow{row("test", "go test ./...", runner.Result{Outcome: runner.Failed, ExitCode: 2, Duration: 59960 * time.Millisecond, Output: []byte("a\nb")})}),
		},
		want: oneGreen + `
Instance command health
  failing        test       go test ./...  exit 2 after 1m0s
    a
    b
`,
	},
	{
		// A shell string longer than the column is cut, not wrapped; the
		// full string is in the instance definition. Line breaks and tabs —
		// legal in a TOML multi-line string — become spaces first, so the
		// table survives them.
		name: "long and multi-line commands",
		report: Report{
			Harness: allGreen[:1],
			Instance: Ran([]CommandRow{
				row("build", "go build ./... &&\n  echo\tdone\n", runner.Result{Outcome: runner.Succeeded, Duration: time.Second}),
				row("lint", `go vet ./... && { files=$(gofmt -l .) || exit $?; [ -z "$files" ] || exit 1; }`, runner.Result{Outcome: runner.Succeeded, Duration: 61400 * time.Millisecond}),
			}),
		},
		want: oneGreen + `
Instance command health
  healthy        build      go build ./... && echo done               1.0s
  healthy        lint       go vet ./... && { files=$(gofmt -l .) |…  1m1s
`,
	},
	{
		// An interrupted run renders what ran and nothing else — no
		// instance heading when the section never started.
		name: "harness only",
		report: Report{
			Harness: allGreen[:2],
		},
		want: `Harness preconditions
  ok    git repository  inside a git work tree
  ok    working tree    working tree is clean
`,
	},
}

func TestRenderGolden(t *testing.T) {
	for _, tt := range goldens {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Render(&buf, tt.report, tt.verbose); err != nil {
				t.Fatalf("Render: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("Render output differs\n--- got ---\n%s--- want ---\n%s--- first differing line ---\n%s", got, tt.want, firstDiff(got, tt.want))
			}
		})
	}
}

func firstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return "got:  " + g[i] + "\nwant: " + w[i]
		}
	}
	return "(lengths differ)"
}

// TestDocExamplesAreRendered keeps docs/reference/rolling-doctor.md honest:
// every report-shaped fenced block on the page must appear, byte for byte,
// in what Render produces for the golden reports. The page is the spec, and
// a spec that shows output the renderer does not produce is the drift this
// package exists to prevent.
func TestDocExamplesAreRendered(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "rolling-doctor.md"))
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	for _, g := range goldens {
		if err := Render(&rendered, g.report, g.verbose); err != nil {
			t.Fatal(err)
		}
		rendered.WriteString("\n")
	}
	blocks := strings.Split(string(doc), "```")
	checked := 0
	for i := 1; i < len(blocks); i += 2 { // odd indices are inside fences
		block := strings.TrimPrefix(blocks[i], "\n")
		first, _, _ := strings.Cut(block, "\n")
		// Usage, the interrupted line, and the git-override note belong to
		// the command, not the renderer; everything else on the page is
		// report output.
		if strings.HasPrefix(first, "rolling doctor") || strings.HasPrefix(first, "note:") || first == "interrupted" {
			continue
		}
		checked++
		if !strings.Contains(rendered.String(), block) {
			t.Errorf("the reference page shows output the renderer does not produce:\n%s", block)
		}
	}
	// A floor, not an exact count: additions are welcome, but a drop
	// below what existed when this was written means a block was silently
	// exempted from the guard.
	if checked < 8 {
		t.Fatalf("only %d report-shaped example blocks checked; 8 existed when this guard was written — was one silently exempted?", checked)
	}
}

// TestRenderZeroReport pins the lesson from #7 and #10 at the report level:
// a report nothing was written into renders as nothing and blocks — it can
// never read as a healthy checkout.
func TestRenderZeroReport(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, Report{}, false); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("zero Report rendered %q, want nothing", buf.String())
	}
	if !(Report{}).Blocking() {
		t.Error("zero Report is not Blocking — an empty run would read as ready")
	}
}

// TestRenderRejectsInvalid: a probe status or runner outcome that was never
// set, or an instance section in an impossible state, is a bug in the
// caller. Render refuses and writes nothing rather than inventing a row.
func TestRenderRejectsInvalid(t *testing.T) {
	tests := []struct {
		name   string
		report Report
	}{
		{"zero probe status", Report{Harness: []probe.Result{{Name: "x", Message: "y"}}}},
		{"zero runner outcome", Report{Harness: allGreen[:1], Instance: Ran([]CommandRow{row("build", "x", runner.Result{})})}},
		{"ran with no commands", Report{Harness: allGreen[:1], Instance: Ran(nil)}},
		{"skipped without a reason", Report{Harness: allGreen[:1], Instance: Skipped("")}},
		{"unknown instance state", Report{Harness: allGreen[:1], Instance: InstanceSection{State: InstanceState(99)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := Render(&buf, tt.report, false)
			if err == nil {
				t.Fatalf("Render accepted an invalid report; wrote %q", buf.String())
			}
			if buf.Len() != 0 {
				t.Errorf("Render wrote %q before failing; want nothing", buf.String())
			}
		})
	}
}
