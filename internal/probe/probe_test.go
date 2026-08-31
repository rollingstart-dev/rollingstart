package probe

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestZeroResultIsNotGreen(t *testing.T) {
	var res Result
	if res.Status == Green {
		t.Error("zero Result reads as Green — an unset finding would render as passing")
	}
}

// TestProbesGreenOnThisRepository is the harness-probes scope's end-to-end
// check: every precondition holds for this repository, which is meant to
// become an instance of its own tool. Nothing is isolated — the probes must
// see the real git config chain and the real filesystem — so it runs only in
// CI, where the checkout is pristine; a developer's tree is legitimately
// dirty mid-change.
func TestProbesGreenOnThisRepository(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("CI only: a developer's working tree is legitimately dirty mid-change")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	probes := []func(context.Context, string) Result{GitRepo, CleanTree, AutoCRLF, InstanceConfig, Watcher}
	for _, probe := range probes {
		res := probe(context.Background(), root)
		t.Logf("%s: %v — %s", res.Name, res.Status, res.Message)
		if res.Status != Green {
			t.Errorf("%s: %v — %s", res.Name, res.Status, res.Message)
		}
	}
}

// TestProbeNamesShareOneRegister pins the rendering decision from
// docs/reference/rolling-doctor.md: names are prose, in the documented
// order, so they sit in one register in doctor's column. The config key
// core.autocrlf stays in the finding, where it is the thing to fix.
func TestProbeNamesShareOneRegister(t *testing.T) {
	isolateGitConfig(t)
	dir := initRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".rollingstart"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, ".rollingstart/instance.toml", "")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	want := []string{"git repository", "working tree", "line endings", "instance definition", "file watcher"}
	probes := []func(context.Context, string) Result{GitRepo, CleanTree, AutoCRLF, InstanceConfig, Watcher}
	for i, probe := range probes {
		res := probe(context.Background(), dir)
		if res.Name != want[i] {
			t.Errorf("probe %d Name = %q, want %q", i, res.Name, want[i])
		}
	}
}
