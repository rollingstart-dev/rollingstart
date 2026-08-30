package probe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolateGitConfig keeps the host's global and system git config out of the
// probes' view, so a developer's own core.autocrlf cannot sway a test. The
// probes themselves must see the real config chain — a global autocrlf=true
// is exactly what AutoCRLF exists to catch — so isolation is test-only.
func isolateGitConfig(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
}

// initRepo creates a temp git repository and returns its root.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	gitIn(t, dir, "config", "user.email", "probe-test@example.invalid")
	gitIn(t, dir, "config", "user.name", "Probe Test")
	return dir
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGitRepo(t *testing.T) {
	isolateGitConfig(t)
	tests := []struct {
		name       string
		dir        func(t *testing.T) string
		wantStatus Status
		wantIn     string
	}{
		{
			name:       "inside a work tree",
			dir:        initRepo,
			wantStatus: Green,
			wantIn:     "work tree",
		},
		{
			name:       "no repository",
			dir:        func(t *testing.T) string { return t.TempDir() },
			wantStatus: Red,
			wantIn:     "not inside a git repository",
		},
		{
			name:       "inside the git directory",
			dir:        func(t *testing.T) string { return filepath.Join(initRepo(t), ".git") },
			wantStatus: Red,
			wantIn:     "not a work tree",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := GitRepo(context.Background(), tt.dir(t))
			if res.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v (message: %q)", res.Status, tt.wantStatus, res.Message)
			}
			if !strings.Contains(res.Message, tt.wantIn) {
				t.Errorf("Message %q does not contain %q", res.Message, tt.wantIn)
			}
			if res.Name == "" {
				t.Error("Name is empty")
			}
		})
	}
}

func TestCleanTree(t *testing.T) {
	isolateGitConfig(t)
	commitAll := func(t *testing.T, dir string) {
		gitIn(t, dir, "add", ".")
		gitIn(t, dir, "commit", "-q", "-m", "x")
	}
	tests := []struct {
		name       string
		dir        func(t *testing.T) string
		wantStatus Status
		wantIn     string
	}{
		{
			name:       "fresh repository",
			dir:        initRepo,
			wantStatus: Green,
			wantIn:     "is clean",
		},
		{
			name: "committed and clean",
			dir: func(t *testing.T) string {
				dir := initRepo(t)
				write(t, dir, "a.txt", "a")
				commitAll(t, dir)
				return dir
			},
			wantStatus: Green,
			wantIn:     "is clean",
		},
		{
			name: "untracked file",
			dir: func(t *testing.T) string {
				dir := initRepo(t)
				write(t, dir, "junk.txt", "x")
				return dir
			},
			wantStatus: Red,
			wantIn:     "not clean",
		},
		{
			name: "staged file",
			dir: func(t *testing.T) string {
				dir := initRepo(t)
				write(t, dir, "a.txt", "a")
				gitIn(t, dir, "add", ".")
				return dir
			},
			wantStatus: Red,
			wantIn:     "not clean",
		},
		{
			name: "modified tracked file",
			dir: func(t *testing.T) string {
				dir := initRepo(t)
				write(t, dir, "a.txt", "a")
				commitAll(t, dir)
				write(t, dir, "a.txt", "b")
				return dir
			},
			wantStatus: Red,
			wantIn:     "not clean",
		},
		{
			// The learner's own config must not blind the probe: with
			// status.showUntrackedFiles=no, plain porcelain output is
			// empty for a tree full of untracked files.
			name: "untracked file hidden by config",
			dir: func(t *testing.T) string {
				dir := initRepo(t)
				gitIn(t, dir, "config", "status.showUntrackedFiles", "no")
				write(t, dir, "junk.txt", "x")
				return dir
			},
			wantStatus: Red,
			wantIn:     "not clean",
		},
		{
			name:       "no repository",
			dir:        func(t *testing.T) string { return t.TempDir() },
			wantStatus: Red,
			wantIn:     "cannot check",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := CleanTree(context.Background(), tt.dir(t))
			if res.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v (message: %q)", res.Status, tt.wantStatus, res.Message)
			}
			if !strings.Contains(res.Message, tt.wantIn) {
				t.Errorf("Message %q does not contain %q", res.Message, tt.wantIn)
			}
		})
	}
}

func TestAutoCRLF(t *testing.T) {
	isolateGitConfig(t)
	// The spelled-out cases cover git's whole boolean vocabulary, not just
	// the canonical spellings: git accepts 0/no/off as false and 1/yes/on
	// as true, and --get returns whichever raw string the learner wrote.
	tests := []struct {
		name       string
		set        bool
		value      string
		wantStatus Status
		wantIn     string
	}{
		{name: "unset", wantStatus: Green, wantIn: "unset"},
		{name: "false", set: true, value: "false", wantStatus: Green, wantIn: "false"},
		{name: "input", set: true, value: "input", wantStatus: Green, wantIn: "input"},
		{name: "true", set: true, value: "true", wantStatus: Red, wantIn: "CRLF"},
		{name: "zero is false", set: true, value: "0", wantStatus: Green, wantIn: "false"},
		{name: "off is false", set: true, value: "off", wantStatus: Green, wantIn: "false"},
		{name: "one is true", set: true, value: "1", wantStatus: Red, wantIn: "CRLF"},
		{name: "yes is true", set: true, value: "yes", wantStatus: Red, wantIn: "CRLF"},
		{name: "explicit empty is false", set: true, value: "", wantStatus: Green, wantIn: "false"},
		{name: "unexpected value", set: true, value: "maybe", wantStatus: Red, wantIn: "unexpected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := initRepo(t)
			if tt.set {
				gitIn(t, dir, "config", "core.autocrlf", tt.value)
			}
			res := AutoCRLF(context.Background(), dir)
			if res.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v (message: %q)", res.Status, tt.wantStatus, res.Message)
			}
			if !strings.Contains(res.Message, tt.wantIn) {
				t.Errorf("Message %q does not contain %q", res.Message, tt.wantIn)
			}
		})
	}
}

func TestAutoCRLFUnreadableConfigIsNotUnset(t *testing.T) {
	// A config read that fails must not masquerade as the key being unset —
	// unset is green, a broken config is a finding of its own.
	isolateGitConfig(t)
	dir := initRepo(t)
	write(t, dir, filepath.Join(".git", "config"), "[[[garbage")
	res := AutoCRLF(context.Background(), dir)
	if res.Status != Red {
		t.Errorf("Status = %v, want Red (message: %q)", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "cannot read") {
		t.Errorf("Message %q does not contain %q", res.Message, "cannot read")
	}
	if strings.Contains(res.Message, "unset") {
		t.Errorf("Message %q claims the key is unset", res.Message)
	}
}

func TestAutoCRLFValuelessKeyMeansTrue(t *testing.T) {
	// A bare "autocrlf" line with no value means true to git — the config
	// CLI cannot write that form, so it is appended to the file directly.
	isolateGitConfig(t)
	dir := initRepo(t)
	f, err := os.OpenFile(filepath.Join(dir, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("[core]\n\tautocrlf\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	res := AutoCRLF(context.Background(), dir)
	if res.Status != Red {
		t.Errorf("Status = %v, want Red (message: %q)", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "CRLF") {
		t.Errorf("Message %q does not carry the CRLF explanation", res.Message)
	}
}

func TestGitRepoDubiousOwnershipIsNotMisreported(t *testing.T) {
	// Exit 128 is git's generic fatal, not a synonym for "no repository".
	// A repo owned by another uid — routine in devcontainers and mounted
	// volumes — must surface git's own words, which include the
	// safe.directory remedy, rather than telling a learner standing in the
	// right directory to go somewhere else. GIT_TEST_ASSUME_DIFFERENT_OWNER
	// is how git's own test suite simulates the foreign owner.
	isolateGitConfig(t)
	dir := initRepo(t)
	t.Setenv("GIT_TEST_ASSUME_DIFFERENT_OWNER", "1")
	res := GitRepo(context.Background(), dir)
	if res.Status != Red {
		t.Errorf("Status = %v, want Red (message: %q)", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "dubious ownership") || !strings.Contains(res.Message, "safe.directory") {
		t.Errorf("Message %q does not surface git's dubious-ownership words and remedy", res.Message)
	}
	if strings.Contains(res.Message, "not inside a git repository") {
		t.Errorf("Message %q misdirects the learner to change directory", res.Message)
	}
}

func TestGitProbesWithoutGit(t *testing.T) {
	// An absent git binary is a red finding, not a crash — and every git
	// probe must classify it on its own, so probe order can never matter.
	dir := initRepo(t) // needs git on PATH, so before it is cleared
	t.Setenv("PATH", "")
	probes := []struct {
		name string
		run  func(context.Context, string) Result
	}{
		{name: "GitRepo", run: GitRepo},
		{name: "CleanTree", run: CleanTree},
		{name: "AutoCRLF", run: AutoCRLF},
	}
	for _, p := range probes {
		t.Run(p.name, func(t *testing.T) {
			res := p.run(context.Background(), dir)
			if res.Status != Red {
				t.Errorf("Status = %v, want Red (message: %q)", res.Status, res.Message)
			}
			if !strings.Contains(res.Message, "not installed or not on PATH") {
				t.Errorf("Message %q does not name the missing git binary", res.Message)
			}
		})
	}
}
