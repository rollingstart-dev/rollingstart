package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	res := rolling(t, runOptions{}, "version")
	if res.code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", res.code, res.stderr)
	}
	for _, want := range []string{"rolling ", "commit ", "go ", "platform "} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, res.stdout)
		}
	}
	if res.stderr != "" {
		t.Errorf("stderr = %q, want empty", res.stderr)
	}
}

// TestUnknownCommand: a typo must be an error on stderr and a nonzero
// exit, never a silent success or help text on stdout.
func TestUnknownCommand(t *testing.T) {
	res := rolling(t, runOptions{}, "frobnicate")
	if res.code != 2 {
		t.Fatalf("exit %d for an unknown command, want 2 — the usage exit", res.code)
	}
	if !strings.Contains(res.stderr, "frobnicate") {
		t.Errorf("stderr does not name the unknown command:\n%s", res.stderr)
	}
	if res.stdout != "" {
		t.Errorf("stdout = %q, want empty — an error belongs on stderr", res.stdout)
	}
}

// TestFixtureHelpers is the harness checking itself: a fixture repository
// is a real one, with an identity, a definition where the binary will look
// for it, and — after commitAll — a clean tree, since the clean-tree probe
// is one of the things the fixtures exist to exercise.
func TestFixtureHelpers(t *testing.T) {
	repo := newRepo(t)
	writeDefinition(t, repo, "[commands]\nbuild = \"true\"\n")
	commitAll(t, repo)
	if got := gitOut(t, repo, "status", "--porcelain"); got != "" {
		t.Errorf("tree not clean after commitAll:\n%s", got)
	}
	if got := gitOut(t, repo, "config", "--get", "user.email"); got != "e2e@example.invalid" {
		t.Errorf("user.email = %q, want the fixture identity", got)
	}
	if got := gitOut(t, repo, "ls-files"); got != ".rollingstart/instance.toml" {
		t.Errorf("ls-files = %q, want the definition alone", got)
	}
}

// TestInheritedGitStateIsScrubbed pins the hazard the pre-push review of
// #16 demonstrated on this repository: with GIT_DIR or GIT_INDEX_FILE
// inherited — a commit hook exports both — a fixture's git init, add -A,
// and commit land on the repository the tests are running in. runMain
// scrubs the environment before any test; this one re-creates the
// inherited state, scrubs again, and checks that a fixture reaches only
// itself and the other repository is untouched.
func TestInheritedGitStateIsScrubbed(t *testing.T) {
	// Serial only: t.Setenv and t.Parallel are mutually exclusive, and the
	// scrub below mutates the process environment.
	elsewhere := t.TempDir()
	gitIn(t, elsewhere, "init", "-q")
	gitIn(t, elsewhere, "config", "user.email", "victim@example.invalid")
	gitIn(t, elsewhere, "config", "user.name", "Victim")
	if err := os.WriteFile(filepath.Join(elsewhere, "important.txt"), []byte("staged work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, elsewhere, "add", "-A")
	gitIn(t, elsewhere, "commit", "-q", "-m", "important")
	head := gitOut(t, elsewhere, "rev-parse", "HEAD")
	t.Setenv("GIT_DIR", filepath.Join(elsewhere, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(elsewhere, ".git", "index"))
	scrubGitEnv()

	repo := newRepo(t)
	writeDefinition(t, repo, "")
	commitAll(t, repo)
	if got := gitOut(t, repo, "rev-parse", "--git-dir"); got != ".git" {
		t.Errorf("fixture git dir = %q, want .git — the fixture reached another repository", got)
	}
	if got := gitOut(t, elsewhere, "rev-parse", "HEAD"); got != head {
		t.Errorf("the other repository's HEAD moved: %s -> %s — the fixture committed into it", head, got)
	}
	if got := gitOut(t, elsewhere, "status", "--porcelain"); got != "" {
		t.Errorf("the other repository's index or tree changed — a leak would stage deletions:\n%s", got)
	}
}
