package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/rollingstart-dev/rollingstart/internal/doctor"
	"github.com/rollingstart-dev/rollingstart/internal/instance"
	"github.com/rollingstart-dev/rollingstart/internal/probe"
	"github.com/rollingstart-dev/rollingstart/internal/runner"
)

var doctorTimeout time.Duration

var doctorCmd = &cobra.Command{
	Use:   "doctor [dir]",
	Short: "Check readiness: harness preconditions and instance command health",
	Long: `Check whether Rolling Start can operate here, and whether the codebase's
own toolchain considers the working copy healthy. Two sections, two
meanings: a FAIL under harness preconditions blocks and makes the exit
nonzero; red under instance command health informs — a broken-looking
instance is frequently a learner who has not done the local-dev lesson
yet — and exits zero.

Doctor resolves the repository root from dir (default: the current
directory), runs everything against it, reports, and fixes nothing.

Reference: https://github.com/rollingstart-dev/rollingstart/blob/main/docs/reference/rolling-doctor.md`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return usageError{err}
		}
		return nil
	},
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().DurationVar(&doctorTimeout, "timeout", 5*time.Minute,
		"per-command timeout for instance commands")
	rootCmd.AddCommand(doctorCmd)
}

// probes are the harness preconditions in the documented order — the
// watcher last, and never concurrently with CleanTree: it writes into the
// checkout for the length of its wait (internal/probe's package doc).
var probes = []func(context.Context, string) probe.Result{
	probe.GitRepo,
	probe.CleanTree,
	probe.AutoCRLF,
	probe.InstanceConfig,
	probe.Watcher,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return fmt.Errorf("reading --verbose: %w", err)
	}
	start := "."
	if len(args) == 1 {
		start = args[0]
		// A directory that does not exist is the caller's typo, not five
		// findings about a broken environment (usage, exit 2).
		fi, err := os.Stat(start)
		switch {
		case err != nil:
			return usageError{fmt.Errorf("no such directory: %s", start)}
		case !fi.IsDir():
			return usageError{fmt.Errorf("not a directory: %s", start)}
		}
	}
	dir := resolveRoot(ctx, start)

	// One load serves everything outside the probes: the corpus notes
	// ahead of the report and the instance section after it. The
	// instance-definition probe loads again on its own — its contract is
	// a directory — and the read is one local file.
	inst, loadErr := instance.Load(filepath.Join(dir, instance.Path))

	// Notes precede the report: git's override note first, then one line
	// per corpus pointer absent from this checkout, then a blank line. A
	// definition that did not load has no corpus to check — the
	// instance-definition row and the skipped section carry that story.
	var notes []string
	if note := gitOverrideNote(); note != "" {
		notes = append(notes, note)
	}
	if loadErr == nil {
		notes = append(notes, corpusNotes(dir, inst.Corpus())...)
	}
	if len(notes) > 0 {
		for _, note := range notes {
			fmt.Fprintln(out, note)
		}
		fmt.Fprintln(out)
	}

	var report doctor.Report
	interrupted := false
	for _, p := range probes {
		res := p(ctx, dir)
		if ctx.Err() != nil {
			// The probe's result is its cancellation wording, not a
			// finding about the environment; an interrupted run must
			// never read as a broken one (the spec's decision).
			interrupted = true
			break
		}
		report.Harness = append(report.Harness, res)
	}

	if !interrupted {
		var err error
		report.Instance, interrupted, err = instanceSection(ctx, dir, inst, loadErr, doctorTimeout)
		if err != nil {
			return err
		}
	}

	if err := doctor.Render(out, report, verbose); err != nil {
		return err
	}
	if interrupted {
		fmt.Fprintln(out, "interrupted")
		return errInterrupted
	}
	if report.Blocking() {
		return errSilentExit
	}
	return nil
}

// instanceSection runs the declared commands sequentially, in canonical
// order, from the root. It runs whenever the definition loaded, even when
// another precondition failed — a learner with a dirty tree also wants to
// know whether their toolchain is installed. inst and loadErr are the one
// load runDoctor made; a load failure is the section's skipped reason.
// Operations never reach here: the section reports command health, and a
// definition declaring operations but no commands still reads "nothing
// declared".
func instanceSection(ctx context.Context, dir string, inst *instance.Instance, loadErr error, timeout time.Duration) (doctor.InstanceSection, bool, error) {
	if loadErr != nil {
		return doctor.Skipped(loadErr.Error()), false, nil
	}
	cmds := inst.Commands()
	if len(cmds) == 0 {
		return doctor.NothingDeclared(), false, nil
	}
	var rows []doctor.CommandRow
	for _, c := range cmds {
		res, err := runner.Run(ctx, runner.Spec{Command: c.Cmd, Dir: dir, Timeout: timeout})
		if err != nil {
			// The runner reserves its error for the caller's own
			// cancellation and for harness-internal failures. Only the
			// first is an interruption; anything else must surface as
			// itself, never wear the user-intent word. Rows that
			// finished still render — they ran.
			if ctx.Err() == nil {
				return doctor.InstanceSection{}, false, fmt.Errorf("running %s: %w", c.Name, err)
			}
			if len(rows) == 0 {
				return doctor.InstanceSection{}, true, nil
			}
			return doctor.Ran(rows), true, nil
		}
		rows = append(rows, doctor.CommandRow{Command: c, Result: res})
	}
	return doctor.Ran(rows), false, nil
}

// gitOverrideNote is the #21 decision: git's state-relocating variables
// are followed, never fought and never silently — setting them is
// deliberate, and git honouring them is git behaving as configured. When
// any of the three is set the report opens with one line saying so;
// benign git variables (GIT_EDITOR, GIT_PAGER) relocate nothing and
// produce nothing. Informational only: no red row, no changed exit code.
func gitOverrideNote() string {
	var set []string
	for _, name := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		if val, ok := os.LookupEnv(name); ok {
			set = append(set, name+"="+val)
		}
	}
	switch {
	case len(set) == 0:
		return ""
	case len(set) == 1:
		return fmt.Sprintf("note: %s is set — git operations, and this report, follow it", set[0])
	case len(set) == 2:
		return fmt.Sprintf("note: %s and %s are set — git operations, and this report, follow them", set[0], set[1])
	default:
		// Generic, not three slots: a variable added to the watch list
		// must appear, or the note lies by omission — the failure mode
		// this feature exists to prevent.
		list := strings.Join(set[:len(set)-1], ", ") + ", and " + set[len(set)-1]
		return fmt.Sprintf("note: %s are set — git operations, and this report, follow them", list)
	}
}

// corpusNotes is the git note's sibling (#37): a corpus pointer whose
// target is absent from this checkout is named, not fought. Every
// path-valued pointer is checked — exemplary entries in list order, then
// definition-of-ready. exemplar-prs are URLs, and doctor performs no
// network I/O, so they are never touched. Existence follows symlinks —
// Stat, not Lstat — so a link to nowhere is a missing target. The path is
// echoed as declared, an exception to the relative-paths rule: the thing to
// fix is the line in instance.toml, not a file at that path. Informational
// only — a stale exemplar stops no lesson — so no red row, no changed exit
// code. Absence earns the note: no such entry, or a path component that
// is a file where a directory was expected (ENOTDIR, which fs.ErrNotExist
// does not cover — a document that became a file under a pointer that
// still expects a directory). A target that exists but cannot be read is
// a different problem, and not this note's claim.
func corpusNotes(dir string, c instance.Corpus) []string {
	paths := slices.Clone(c.Exemplary)
	if c.DefinitionOfReady != "" {
		paths = append(paths, c.DefinitionOfReady)
	}
	var notes []string
	for _, p := range paths {
		_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p)))
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			notes = append(notes, fmt.Sprintf("note: corpus pointer %s does not exist in this checkout", p))
		}
	}
	return notes
}

// resolveRoot finds the repository root from start — the directory the
// instance definition lives in and the coach will watch — and returns it
// relative to the process's working directory, so every path the report
// shows is one the learner can type. When there is no repository, start
// itself is the root, and the git probe is the row that says so.
func resolveRoot(ctx context.Context, start string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", start, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return start
	}
	top := strings.TrimSpace(string(out))
	cwd, err := os.Getwd()
	if err != nil {
		return top
	}
	// Getwd honours $PWD and answers in the shell's logical namespace;
	// git answers in the kernel's physical one. Rel across the two counts
	// parent hops against the wrong tree — a cwd reached through a
	// symlink, the default shape of macOS's $TMPDIR, would get a path to
	// a directory that does not exist. Resolve first, and never hand on
	// an answer without checking it names the root git named.
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}
	rel, err := filepath.Rel(cwd, top)
	if err != nil {
		return top
	}
	a, errA := os.Stat(rel)
	b, errB := os.Stat(top)
	if errA != nil || errB != nil || !os.SameFile(a, b) {
		return top
	}
	return rel
}
