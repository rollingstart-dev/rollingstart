package probe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GitRepo probes whether dir is inside a git work tree.
func GitRepo(ctx context.Context, dir string) Result {
	const name = "git repository"
	out, err := runGit(ctx, dir, "rev-parse", "--is-inside-work-tree")
	var exit *gitError
	switch {
	case err == nil && out == "true":
		return Result{Name: name, Status: Green, Message: "inside a git work tree"}
	case err == nil:
		// rev-parse answers "false" from inside the .git directory.
		return Result{Name: name, Status: Red, Message: "inside a git directory, not a work tree"}
	case errors.Is(err, exec.ErrNotFound):
		return gitMissing(name)
	// Exit 128 is git's generic fatal, so the stderr text — not the code —
	// decides whether this is "no repository here" or a different failure
	// whose remedy is in git's own words: dubious ownership, routine in
	// devcontainers and mounted volumes, names the safe.directory fix
	// itself, and telling that learner to change directory would misdirect.
	case errors.As(err, &exit) && strings.Contains(exit.stderr, "not a git repository"):
		return Result{Name: name, Status: Red, Message: "not inside a git repository — run from the target repository"}
	case errors.As(err, &exit) && exit.stderr != "":
		return Result{Name: name, Status: Red, Message: exit.stderr}
	default:
		return Result{Name: name, Status: Red, Message: fmt.Sprintf("cannot interrogate git: %v", err)}
	}
}

// CleanTree probes whether the working tree carries no uncommitted state.
// Untracked files count as dirty: the loop evaluates work as a diff of the
// tree, and anything already lying around untracked would be misattributed
// to the learner's change.
func CleanTree(ctx context.Context, dir string) Result {
	const name = "working tree"
	// --untracked-files=normal states the intent explicitly: the learner's
	// status.showUntrackedFiles=no would otherwise hide untracked files
	// from porcelain output and turn this probe green on a dirty tree.
	// --no-optional-locks keeps a read-only diagnostic from rewriting
	// .git/index or contending with an editor's background status.
	out, err := runGit(ctx, dir, "--no-optional-locks", "status", "--porcelain", "--untracked-files=normal")
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return gitMissing(name)
	case err != nil:
		return Result{Name: name, Status: Red, Message: fmt.Sprintf("cannot check the working tree: %v", err)}
	case out == "":
		return Result{Name: name, Status: Green, Message: "working tree is clean"}
	}
	n := len(strings.Split(out, "\n"))
	return Result{Name: name, Status: Red, Message: fmt.Sprintf(
		"working tree is not clean: %d path(s) modified, staged, or untracked — commit, stash, or remove them (see git status)", n)}
}

// AutoCRLF probes core.autocrlf, wherever in the config chain it is set —
// a global value affects this working copy just as much as a local one.
// Boolean classification is delegated back to git (--type=bool) so every
// spelling git accepts — 0, no, off, a valueless key — reads exactly the
// way git will read it, instead of replicating the boolean table here.
func AutoCRLF(ctx context.Context, dir string) Result {
	const name = "core.autocrlf"
	out, err := runGit(ctx, dir, "config", "--get", "core.autocrlf")
	var exit *gitError
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return gitMissing(name)
	case errors.As(err, &exit) && exit.code == 1 && exit.stderr == "":
		// git config --get exits 1 with no complaint when the key is not
		// set anywhere in the chain.
		return Result{Name: name, Status: Green, Message: "core.autocrlf is unset"}
	case err != nil:
		return Result{Name: name, Status: Red, Message: fmt.Sprintf("cannot read git config: %v", err)}
	}
	if strings.EqualFold(out, "input") {
		return Result{Name: name, Status: Green, Message: "core.autocrlf is input"}
	}
	canonical, err := runGit(ctx, dir, "config", "--get", "--type=bool", "core.autocrlf")
	switch {
	case err != nil:
		// Neither input nor anything git itself recognises as a boolean.
		return Result{Name: name, Status: Red, Message: fmt.Sprintf("core.autocrlf has unexpected value %q", out)}
	case canonical == "true":
		return Result{Name: name, Status: Red, Message: "core.autocrlf is true: git would rewrite LF to CRLF on checkout, " +
			"corrupting files in a POSIX working copy — set it to false or input"}
	case canonical == "false":
		return Result{Name: name, Status: Green, Message: "core.autocrlf is false"}
	default:
		return Result{Name: name, Status: Red, Message: fmt.Sprintf("core.autocrlf has unexpected value %q", out)}
	}
}

// gitMissing is the finding every git probe reports when the binary cannot
// be found: a harness precondition in its own right, classified by each
// probe independently so their order never matters.
func gitMissing(name string) Result {
	return Result{Name: name, Status: Red, Message: "git is not installed or not on PATH"}
}

// gitError is a git invocation that ran and exited nonzero.
type gitError struct {
	code   int
	stderr string
}

func (e *gitError) Error() string {
	if e.stderr == "" {
		return fmt.Sprintf("git exited %d", e.code)
	}
	return fmt.Sprintf("git exited %d: %s", e.code, e.stderr)
}

// runGit executes git with constructed arguments in dir and returns trimmed
// stdout. Exit failures come back as *gitError so probes can classify on the
// code and git's own words; start failures (git absent, broken) come back
// unwrapped for errors.Is. The trim clips porcelain's leading column from
// its first line — fine while callers only count lines, a trap if one ever
// parses the columns.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return strings.TrimSpace(stdout.String()), nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return "", &gitError{code: exit.ExitCode(), stderr: strings.TrimSpace(stderr.String())}
	}
	return "", err
}
