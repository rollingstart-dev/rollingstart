package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// binary is the rolling executable TestMain built for this run.
var binary string

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

// runMain scrubs the environment, builds once, runs the tests, and removes
// the build — split from TestMain so the deferred removal runs before
// os.Exit. A test that panics crashes the process from its own goroutine,
// and the removal does not run; the build is then left under
// $TMPDIR/rollingstart-e2e-*.
func runMain(m *testing.M) int {
	scrubGitEnv()
	dir, err := os.MkdirTemp("", "rollingstart-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		return 1
	}
	defer os.RemoveAll(dir)
	binary = filepath.Join(dir, "rolling")
	// The import path rather than a relative one: it is the same build a
	// learner runs, and it does not care where the test's cwd is.
	build := exec.Command("go", "build", "-o", binary, "github.com/rollingstart-dev/rollingstart/cmd/rolling")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "e2e: building rolling:", err)
		return 1
	}
	return m.Run()
}

// scrubGitEnv removes the host's git from view for the whole run — every
// fixture and every binary the tests start is a child of this process and
// inherits the result. GIT_CONFIG_GLOBAL and GIT_CONFIG_SYSTEM point at
// /dev/null so a developer's own core.autocrlf cannot sway a run, and every
// other GIT_* variable is unset: a commit hook exports GIT_DIR and
// GIT_INDEX_FILE, a rebase an absolute GIT_INDEX_FILE, and with either
// inherited a fixture's git init, add -A, and commit land on the
// repository the tests are running in. The pre-push review of #16
// demonstrated exactly that on this repository — a commit staging the
// deletion of every tracked file, and the suite reporting ok.
func scrubGitEnv() {
	for _, kv := range os.Environ() {
		if key, _, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(key, "GIT_") {
			os.Unsetenv(key)
		}
	}
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
}

// runOptions is where and how the binary runs. The zero value runs it in
// the test's working directory with the test's environment and no stdin.
type runOptions struct {
	dir   string
	env   []string // KEY=value entries appended to the test's environment
	stdin string
}

// result is what the binary did, kept apart: assertions about a report on
// stdout, an error on stderr, and an exit status are different assertions.
type result struct {
	stdout string
	stderr string
	code   int
}

// rolling runs the built binary with args. A nonzero exit is a result, not
// an error, because exit codes are part of what is under test. The test
// fails only when the binary could not be started at all, or died of a
// signal — the harness's own kill when the test's context ended, or an
// external one — which has no exit status a test could honestly assert on.
func rolling(t *testing.T, opts runOptions, args ...string) result {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, args...)
	cmd.Dir = opts.dir
	cmd.Env = append(os.Environ(), opts.env...)
	if opts.stdin != "" {
		cmd.Stdin = strings.NewReader(opts.stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// If the test's context ends while the binary holds its pipes open,
	// give up on them rather than hang the suite.
	cmd.WaitDelay = 5 * time.Second
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running %s %v: %v", binary, args, err)
	}
	res := result{stdout: stdout.String(), stderr: stderr.String()}
	if exit != nil {
		if exit.ExitCode() < 0 {
			t.Fatalf("%s %v was killed by a signal: %v\nstderr:\n%s", binary, args, exit, stderr.String())
		}
		res.code = exit.ExitCode()
	}
	return res
}

// newRepo creates a temporary git repository with an identity configured
// and returns its root. Isolation from the host is process-wide, from
// scrubGitEnv, so a test that never calls newRepo is isolated too, and
// tests may run in parallel. With the host config out of view
// init.defaultBranch is unset, so the branch is git's own default, master.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	gitIn(t, dir, "config", "user.email", "e2e@example.invalid")
	gitIn(t, dir, "config", "user.name", "E2E")
	return dir
}

// writeDefinition writes the instance definition where the binary looks
// for it. The path is spelled out rather than imported: it is the binary's
// contract, and this package tests contracts.
func writeDefinition(t *testing.T, dir, toml string) {
	t.Helper()
	path := filepath.Join(dir, ".rollingstart", "instance.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitAll stages and commits everything, leaving the tree clean. An
// empty commit is allowed so a repository with no files at all can be
// clean too.
func commitAll(t *testing.T, dir string) {
	t.Helper()
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "fixture")
}

// gitIn runs git in dir for its effect, failing the test if git does.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	gitOut(t, dir, args...)
}

// gitOut runs git in dir and returns its trimmed combined output, failing
// the test if git does.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
