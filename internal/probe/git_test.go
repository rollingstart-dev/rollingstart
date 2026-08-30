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
	tests := []struct {
		name       string
		value      string // "" leaves core.autocrlf unset
		wantStatus Status
		wantIn     string
	}{
		{name: "unset", value: "", wantStatus: Green, wantIn: "unset"},
		{name: "false", value: "false", wantStatus: Green, wantIn: "false"},
		{name: "input", value: "input", wantStatus: Green, wantIn: "input"},
		{name: "true", value: "true", wantStatus: Red, wantIn: "CRLF"},
		{name: "unexpected value", value: "maybe", wantStatus: Red, wantIn: "unexpected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := initRepo(t)
			if tt.value != "" {
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
