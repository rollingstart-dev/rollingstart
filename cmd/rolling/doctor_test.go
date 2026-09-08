package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rollingstart-dev/rollingstart/internal/instance"
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

// TestGitOverrideNote pins the #21 decision: the three state-relocating
// variables produce one note line, informational and grammatical; benign
// git variables produce nothing.
func TestGitOverrideNote(t *testing.T) {
	scrubGitEnvT(t)
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"none set", nil, ""},
		{"benign variables", map[string]string{"GIT_EDITOR": "vim", "GIT_PAGER": "less"}, ""},
		{"one", map[string]string{"GIT_DIR": "/x/.git"},
			"note: GIT_DIR=/x/.git is set — git operations, and this report, follow it"},
		{"two", map[string]string{"GIT_DIR": "/x/.git", "GIT_WORK_TREE": "/x"},
			"note: GIT_DIR=/x/.git and GIT_WORK_TREE=/x are set — git operations, and this report, follow them"},
		{"three", map[string]string{"GIT_DIR": "/x/.git", "GIT_WORK_TREE": "/x", "GIT_INDEX_FILE": "/x/.git/index"},
			"note: GIT_DIR=/x/.git, GIT_WORK_TREE=/x, and GIT_INDEX_FILE=/x/.git/index are set — git operations, and this report, follow them"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scrubGitEnvT(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := gitOverrideNote(); got != tt.want {
				t.Errorf("gitOverrideNote() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNoteExampleMatchesTheReferencePage: the page is the spec, and
// internal/doctor's drift guard deliberately skips note: blocks — they are
// the command's lines, not the renderer's. This is the replacement check,
// where the string lives.
func TestNoteExampleMatchesTheReferencePage(t *testing.T) {
	scrubGitEnvT(t)
	t.Setenv("GIT_DIR", "/mnt/elsewhere/.git")
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "rolling-doctor.md"))
	if err != nil {
		t.Fatal(err)
	}
	if note := gitOverrideNote(); !strings.Contains(string(doc), note) {
		t.Errorf("the reference page does not show the note the command prints:\n%s", note)
	}
}

// TestCorpusNotes: every path-valued pointer is checked against the root,
// exemplary entries in list order and definition-of-ready last; URLs are
// never touched; existence follows symlinks. The path is echoed as declared
// — the line to fix is in instance.toml, not a file at that path.
func TestCorpusNotes(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string) {
		t.Helper()
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("apps/web/present.go")
	mk("docs/ready.md")
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		corpus instance.Corpus
		want   []string
	}{
		{"nothing declared", instance.Corpus{}, nil},
		{"all present", instance.Corpus{Exemplary: []string{"apps/web", "apps/web/present.go"}, DefinitionOfReady: "docs/ready.md"}, nil},
		{"urls are never checked", instance.Corpus{ExemplarPRs: []string{"https://example.invalid/pull/1"}}, nil},
		{"missing exemplary, in list order", instance.Corpus{Exemplary: []string{"apps/web/src/features/poll", "apps/web", "packages/gone"}},
			[]string{
				"note: corpus pointer apps/web/src/features/poll does not exist in this checkout",
				"note: corpus pointer packages/gone does not exist in this checkout",
			}},
		{"definition-of-ready last", instance.Corpus{Exemplary: []string{"missing"}, DefinitionOfReady: ".rollingstart/ready.md"},
			[]string{
				"note: corpus pointer missing does not exist in this checkout",
				"note: corpus pointer .rollingstart/ready.md does not exist in this checkout",
			}},
		{"a dangling symlink is a missing target", instance.Corpus{Exemplary: []string{"dangling"}},
			[]string{"note: corpus pointer dangling does not exist in this checkout"}},
		{"a path through a file is absent too", instance.Corpus{Exemplary: []string{"docs/ready.md/inner"}},
			[]string{"note: corpus pointer docs/ready.md/inner does not exist in this checkout"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := corpusNotes(root, tt.corpus)
			if !slices.Equal(got, tt.want) {
				t.Errorf("corpusNotes() =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

// TestCorpusNoteExampleMatchesTheReferencePage: as for the git note, the
// renderer's drift guard skips note: blocks, so the check lives here.
func TestCorpusNoteExampleMatchesTheReferencePage(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "rolling-doctor.md"))
	if err != nil {
		t.Fatal(err)
	}
	notes := corpusNotes(t.TempDir(), instance.Corpus{Exemplary: []string{"apps/web/src/features/poll"}})
	if len(notes) != 1 {
		t.Fatalf("corpusNotes() = %q, want one note", notes)
	}
	if !strings.Contains(string(doc), notes[0]) {
		t.Errorf("the reference page does not show the note the command prints:\n%s", notes[0])
	}
}
