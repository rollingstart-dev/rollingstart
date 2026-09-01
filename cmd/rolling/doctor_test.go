package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scrubGitEnvT removes inherited GIT_* state for one test, the way e2e's
// TestMain does for its whole run: a commit hook exports GIT_DIR and
// GIT_INDEX_FILE, and resolveRoot must answer about the directory it was
// given, not about whatever repository the environment points at. t.Setenv
// registers the restore; Unsetenv makes the variable actually absent —
// git treats an empty GIT_DIR as set.
func scrubGitEnvT(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if key, _, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(key, "GIT_") {
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
}

func runGitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDoctorArgs(t *testing.T) {
	err := doctorCmd.Args(doctorCmd, []string{"a", "b"})
	if err == nil {
		t.Error("two arguments accepted; doctor takes at most one directory")
	}
	if !errors.Is(err, errUsage) {
		t.Errorf("a second argument is not a usage error: %v", err)
	}
	if err := doctorCmd.Args(doctorCmd, []string{"a"}); err != nil {
		t.Errorf("one directory rejected: %v", err)
	}
}

// TestResolveRootOutsideARepository: with no repository, the starting
// directory itself is the root — the git probe is the row that says so.
func TestResolveRootOutsideARepository(t *testing.T) {
	scrubGitEnvT(t)
	dir := t.TempDir()
	if got := resolveRoot(context.Background(), dir); got != dir {
		t.Errorf("resolveRoot = %q, want %q", got, dir)
	}
}

// TestResolveRootThroughSymlinkedCwd pins the pre-push finding on this
// branch: Getwd honours $PWD and answers in the shell's logical namespace,
// git answers in the kernel's physical one, and Rel across the two built a
// path to a directory that does not exist — the default shape of macOS's
// $TMPDIR. Whatever resolveRoot returns must name the root git named.
func TestResolveRootThroughSymlinkedCwd(t *testing.T) {
	scrubGitEnvT(t)
	base := t.TempDir()
	repo := filepath.Join(base, "phys", "a", "b", "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGitT(t, repo, "init", "-q")
	if err := os.Symlink(filepath.Join(base, "phys", "a", "b"), filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(base, "link", "repo")) // sets $PWD to the logical path
	got := resolveRoot(context.Background(), ".")
	a, errA := os.Stat(got)
	b, errB := os.Stat(repo)
	if errA != nil || errB != nil || !os.SameFile(a, b) {
		t.Errorf("resolveRoot = %q, which does not name the root %q (stat: %v, %v)", got, repo, errA, errB)
	}
}
