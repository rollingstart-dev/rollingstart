package e2e

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// greenDef declares two commands that succeed instantly.
const greenDef = "[commands]\nbuild = \"true\"\ntest = \"true\"\n"

// mustContain asserts each want appears in s.
func mustContain(t *testing.T, s string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
}

func TestDoctorAllGreen(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, greenDef)
	commitAll(t, repo)
	res := rolling(t, runOptions{dir: repo}, "doctor")
	if res.code != 0 {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", res.code, res.stdout, res.stderr)
	}
	mustContain(t, res.stdout,
		"Harness preconditions",
		"  ok    git repository       inside a git work tree",
		"  ok    working tree         working tree is clean",
		"  ok    line endings         core.autocrlf is unset",
		"  ok    instance definition  instance definition loaded (2 commands declared)",
		"  ok    file watcher         file events are delivered",
		"Instance command health",
		"  healthy        build      true",
		"  healthy        test       true",
	)
	if strings.Contains(res.stdout, "FAIL") {
		t.Errorf("FAIL in an all-green report:\n%s", res.stdout)
	}
	if res.stderr != "" {
		t.Errorf("stderr = %q, want empty", res.stderr)
	}
}

// TestDoctorDirtyTree: harness-red blocks the exit, and the second section
// still runs — a learner with a dirty tree also wants to know whether their
// toolchain is installed.
func TestDoctorDirtyTree(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, greenDef)
	commitAll(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := rolling(t, runOptions{dir: repo}, "doctor")
	if res.code != 1 {
		t.Fatalf("exit %d, want 1\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout,
		"  FAIL  working tree         working tree is not clean",
		"Instance command health",
		"  healthy        build      true",
	)
}

// TestDoctorInstanceRed: instance failures are informational — named
// accurately, with the output tail, and the exit stays zero.
func TestDoctorInstanceRed(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, "[commands]\nbuild = \"true\"\ntypecheck = \"rolling-e2e-no-such-tool\"\ntest = \"echo running tests; echo boom >&2; exit 1\"\n")
	commitAll(t, repo)
	res := rolling(t, runOptions{dir: repo}, "doctor")
	if res.code != 0 {
		t.Fatalf("exit %d, want 0 — instance red is informational\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout,
		"  healthy        build",
		"  not installed  typecheck  rolling-e2e-no-such-tool",
		"exit 127 after",
		"  failing        test",
		"exit 1 after",
		"    running tests",
		"    boom",
	)
}

// TestDoctorNoConfig: a missing definition is harness-red, the second
// section reports itself skipped, and every path is relative to the root —
// never the fixture's absolute temp path.
func TestDoctorNoConfig(t *testing.T) {
	repo := newRepo(t)
	commitAll(t, repo)
	res := rolling(t, runOptions{dir: repo}, "doctor")
	if res.code != 1 {
		t.Fatalf("exit %d, want 1\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout,
		"  FAIL  instance definition  no instance definition at .rollingstart/instance.toml",
		"skipped: no instance definition at .rollingstart/instance.toml",
	)
	if strings.Contains(res.stdout, repo) {
		t.Errorf("absolute fixture path leaked into the report:\n%s", res.stdout)
	}
}

// TestDoctorTypoConfig: positioned loader errors, all of them, relative.
func TestDoctorTypoConfig(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, "[commands]\nbiuld = \"x\"\ntset = \"y\"\n")
	commitAll(t, repo)
	res := rolling(t, runOptions{dir: repo}, "doctor")
	if res.code != 1 {
		t.Fatalf("exit %d, want 1\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout,
		`.rollingstart/instance.toml:2:1: unknown key "commands.biuld"`,
		`.rollingstart/instance.toml:3:1: unknown key "commands.tset"`,
		"skipped: ",
	)
	if strings.Contains(res.stdout, repo) {
		t.Errorf("absolute fixture path leaked into the report:\n%s", res.stdout)
	}
}

func TestDoctorZeroCommands(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, "")
	commitAll(t, repo)
	res := rolling(t, runOptions{dir: repo}, "doctor")
	if res.code != 0 {
		t.Fatalf("exit %d, want 0\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout, "  nothing declared: .rollingstart/instance.toml declares no commands")
}

func TestDoctorTimedOut(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, "[commands]\nbuild = \"sleep 30\"\n")
	commitAll(t, repo)
	res := rolling(t, runOptions{dir: repo}, "doctor", "--timeout", "200ms")
	if res.code != 0 {
		t.Fatalf("exit %d, want 0\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout, "  timed out      build      sleep 30", "process group terminated")
}

// TestDoctorVerbose: healthy output is hidden by default and shown with
// --verbose.
func TestDoctorVerbose(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, "[commands]\nbuild = \"echo built-ok\"\n")
	commitAll(t, repo)
	quiet := rolling(t, runOptions{dir: repo}, "doctor")
	// The command string itself carries "built-ok" in the row — an echo's
	// output is always part of its own command — so both assertions are on
	// the indented tail form, which only output lines have.
	if strings.Contains(quiet.stdout, "\n    built-ok") {
		t.Errorf("healthy output shown without --verbose:\n%s", quiet.stdout)
	}
	loud := rolling(t, runOptions{dir: repo}, "doctor", "--verbose")
	mustContain(t, loud.stdout, "\n    built-ok")
}

// TestDoctorFromSubdirectory: the root is resolved from wherever doctor
// runs, so the definition at the root is found from anywhere inside.
func TestDoctorFromSubdirectory(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, greenDef)
	sub := filepath.Join(repo, "internal", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "keep.go"), []byte("package deep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo)
	res := rolling(t, runOptions{dir: sub}, "doctor")
	if res.code != 0 {
		t.Fatalf("exit %d, want 0\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout, "instance definition loaded (2 commands declared)")
}

// TestDoctorDirArgument: pointing doctor at a directory works from
// anywhere — the shape 1.5 uses against a Rallly clone.
func TestDoctorDirArgument(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, greenDef)
	commitAll(t, repo)
	res := rolling(t, runOptions{}, "doctor", repo)
	if res.code != 0 {
		t.Fatalf("exit %d, want 0\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout, "instance definition loaded (2 commands declared)")
}

// uniqueMarker makes pgrep -f target this run alone: the process table is
// a shared namespace, and two concurrent suites must not see each other's
// sleeps.
func uniqueMarker(t *testing.T) string {
	return fmt.Sprintf("rolling-e2e-%d-%s", os.Getpid(), strings.ReplaceAll(t.Name(), "/", "-"))
}

// interruptDoctor starts doctor in repo, waits until marker shows in the
// process table — so the interrupt lands mid-command, not mid-probe. The
// fixture command must be a compound ("sleep … && echo marker"): sh execs
// a bare simple command, which strips a trailing comment and the marker
// with it —
// sends SIGINT, and returns the combined output and exit code. Before
// returning it insists the command's whole process group is gone: the
// runner's SIGTERM, reached through the cancelled context, must have taken
// it down.
func interruptDoctor(t *testing.T, repo, marker string) (string, int) {
	t.Helper()
	cmd := exec.Command(binary, "doctor")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		return exec.Command("pgrep", "-f", marker).Run() == nil
	}, "the declared command never started")
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	err := cmd.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("Wait: %v (output:\n%s)", err, out.String())
	}
	waitFor(t, 5*time.Second, func() bool {
		return exec.Command("pgrep", "-f", marker).Run() != nil
	}, "the command's process group survived the interrupt")
	return out.String(), exit.ExitCode()
}

// TestDoctorInterrupt: Ctrl-C during the first command exits 130, ends the
// report with a single interrupted line, and renders no instance section —
// nothing in it finished.
func TestDoctorInterrupt(t *testing.T) {
	marker := uniqueMarker(t)
	repo := newRepo(t)
	writeDefinition(t, repo, "[commands]\nbuild = \"sleep 31.7 && echo "+marker+"\"\n")
	commitAll(t, repo)
	out, code := interruptDoctor(t, repo, marker)
	if code != 130 {
		t.Errorf("exit %d, want 130\noutput:\n%s", code, out)
	}
	if !strings.HasSuffix(out, "interrupted\n") {
		t.Errorf("report does not end with the interrupted line:\n%s", out)
	}
	mustContain(t, out, "Harness preconditions")
	if strings.Contains(out, "Instance command health") {
		t.Errorf("a section that did not finish was rendered:\n%s", out)
	}
}

// TestDoctorInterruptAfterHealthyCommand: rows that finished still render —
// they ran — and the command the interrupt cut short does not.
func TestDoctorInterruptAfterHealthyCommand(t *testing.T) {
	marker := uniqueMarker(t)
	repo := newRepo(t)
	writeDefinition(t, repo, "[commands]\nbuild = \"true\"\ntest = \"sleep 31.7 && echo "+marker+"\"\n")
	commitAll(t, repo)
	out, code := interruptDoctor(t, repo, marker)
	if code != 130 {
		t.Errorf("exit %d, want 130\noutput:\n%s", code, out)
	}
	if !strings.HasSuffix(out, "interrupted\n") {
		t.Errorf("report does not end with the interrupted line:\n%s", out)
	}
	mustContain(t, out, "Instance command health", "  healthy        build      true")
	if strings.Contains(out, "sleep 31.7") {
		t.Errorf("the command the interrupt cut short was rendered:\n%s", out)
	}
}

// TestDoctorSymlinkedCwd pins the pre-push finding on this branch from the
// binary's side: a working directory reached through a symlink — the
// default shape of macOS's $TMPDIR — must not turn a healthy repository
// into five FAILs. The shell's logical $PWD rides in explicitly, since
// that is what a real shell exports.
func TestDoctorSymlinkedCwd(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "phys", "a", "b", "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "init", "-q")
	gitIn(t, repo, "config", "user.email", "e2e@example.invalid")
	gitIn(t, repo, "config", "user.name", "E2E")
	writeDefinition(t, repo, greenDef)
	commitAll(t, repo)
	if err := os.Symlink(filepath.Join(base, "phys", "a", "b"), filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	logical := filepath.Join(base, "link", "repo")
	res := rolling(t, runOptions{dir: logical, env: []string{"PWD=" + logical}}, "doctor")
	if res.code != 0 {
		t.Fatalf("exit %d, want 0\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout, "instance definition loaded (2 commands declared)")
	if strings.Contains(res.stdout, "FAIL") {
		t.Errorf("FAIL on a healthy repository behind a symlinked cwd:\n%s", res.stdout)
	}
}

// TestDoctorRelativeFromSubdirectory pins the doc's ../ example: from a
// subdirectory, findings name paths the learner can type from where they
// stand.
func TestDoctorRelativeFromSubdirectory(t *testing.T) {
	repo := newRepo(t)
	sub := filepath.Join(repo, "docs")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "readme.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo)
	res := rolling(t, runOptions{dir: sub}, "doctor")
	if res.code != 1 {
		t.Fatalf("exit %d, want 1\n%s", res.code, res.stdout)
	}
	mustContain(t, res.stdout, "no instance definition at ../.rollingstart/instance.toml")
	if strings.Contains(res.stdout, repo) {
		t.Errorf("absolute fixture path leaked into the report:\n%s", res.stdout)
	}
}

// TestDoctorUsageErrors: the caller's mistakes exit 2, distinct from a
// failed precondition's 1 — the distinction the doc's exit table exists
// to make.
func TestDoctorUsageErrors(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		wantIn string
	}{
		{"unknown flag", []string{"doctor", "--nosuchflag"}, "nosuchflag"},
		{"two directories", []string{"doctor", "a", "b"}, "at most 1"},
		{"bad flag value", []string{"doctor", "--timeout", "notaduration"}, "notaduration"},
		// fang's error rendering capitalises the first word, so the
		// assertion starts past it.
		{"missing directory", []string{"doctor", "/rolling-e2e-no-such-dir"}, "such directory: /rolling-e2e-no-such-dir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := rolling(t, runOptions{}, tt.args...)
			if res.code != 2 {
				t.Errorf("exit %d, want 2\nstdout:\n%s\nstderr:\n%s", res.code, res.stdout, res.stderr)
			}
			if !strings.Contains(res.stderr, tt.wantIn) {
				t.Errorf("stderr does not name the mistake (%q):\n%s", tt.wantIn, res.stderr)
			}
		})
	}
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, within time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(msg)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
