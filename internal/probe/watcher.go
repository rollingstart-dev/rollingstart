package probe

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watcherTimeout bounds the wait for the synthetic event. A working inotify
// or kqueue delivers within milliseconds; the failure this probe exists to
// catch delivers never. The margin is for loaded CI runners and slow virtual
// machines, where a false red would cost far more than a slow diagnosis —
// and the full wait is only ever paid on the failure path.
const watcherTimeout = 5 * time.Second

// probeFilePrefix names the synthetic file. Dot-named so editors' file trees
// ignore it, and distinctive so that if the process dies mid-probe the
// residue names itself in git status — and so in the clean-tree probe —
// instead of masquerading as learner work.
const probeFilePrefix = ".rollingstart-probe-"

// Watcher probes whether the filesystem under dir delivers change events.
// dir is the checkout root — the directory the learner edits and the coach
// will watch — so the filesystem boundary that matters is the one under
// test; watching .git/ would prove the wrong path. The probe writes a
// synthetic file there, waits for its event, and removes the file on every
// path, leaving the tree exactly as found.
//
// The failure it guards against is silent: a repository on a Windows drive
// watched from WSL2 receives no inotify events at all (roadmap § 4), and a
// coach watching it would sit there looking healthy and never fire.
func Watcher(ctx context.Context, dir string) Result {
	return watcher(ctx, dir, dir, watcherTimeout)
}

// watcher is the seam behind Watcher. The watched directory and the
// directory the synthetic file is written into are separate parameters so a
// test can force the no-event path deterministically — write where the
// watcher is not looking — instead of needing a real WSL2 boundary. In
// production they are the same directory.
func watcher(ctx context.Context, watchDir, writeDir string, timeout time.Duration) (res Result) {
	const name = "file watcher"

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return Result{Name: name, Status: Red, Message: fmt.Sprintf("cannot create a file watcher: %v%s", err, limitHint(err))}
	}
	// Close also stops fsnotify's reader goroutine and waits for it.
	defer w.Close()

	if err := w.Add(watchDir); err != nil {
		return Result{Name: name, Status: Red, Message: fmt.Sprintf("cannot watch %s: %v%s", watchDir, err, limitHint(err))}
	}

	// The watch is registered before the write, so the event cannot be
	// missed by arriving early.
	f, err := os.CreateTemp(writeDir, probeFilePrefix+"*")
	if err != nil {
		// Nothing was ever there to observe: a read-only checkout, not a
		// silent watcher, and the finding must not blame WSL2 for it.
		return Result{Name: name, Status: Red, Message: fmt.Sprintf("cannot write a synthetic file in the checkout: %v", err)}
	}
	path := f.Name()
	defer func() {
		// Deferred rather than sequenced so a red probe cleans up too. A
		// file left behind would dirty the tree and trip the clean-tree
		// probe on the next run, so a failed removal is a finding in its
		// own right and outranks whatever the watcher reported.
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			res = Result{Name: name, Status: Red, Message: fmt.Sprintf("synthetic file %s was left behind: %v — remove it by hand", path, err)}
		}
	}()
	if err := f.Close(); err != nil {
		return Result{Name: name, Status: Red, Message: fmt.Sprintf("cannot write a synthetic file in the checkout: %v", err)}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	base := filepath.Base(path)
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return Result{Name: name, Status: Red, Message: "file watcher closed before any event arrived"}
			}
			// Matched on the base name: fsnotify reports paths under the
			// spelling given to Add, which need not match CreateTemp's
			// once symlinks are involved (macOS's /var → /private/var),
			// and the random suffix makes the base unique to this run.
			// Any op counts — a Remove or Chmod for this file proves
			// delivery exactly as a Create does. Other events are skipped,
			// not mistaken for success: the checkout root is a busy place.
			if filepath.Base(ev.Name) == base {
				return Result{Name: name, Status: Green, Message: "file events are delivered"}
			}
		case err, ok := <-w.Errors:
			if !ok {
				return Result{Name: name, Status: Red, Message: "file watcher closed before any event arrived"}
			}
			return Result{Name: name, Status: Red, Message: fmt.Sprintf("file watcher reported an error: %v", err)}
		case <-timer.C:
			return Result{Name: name, Status: Red, Message: fmt.Sprintf(
				"wrote %s in the checkout but no file event arrived within %s — this filesystem does not deliver "+
					"change notifications; the likely cause is a repository on a Windows drive (/mnt/c) watched "+
					"from WSL2, which receives no inotify events: clone it inside the Linux filesystem instead",
				base, timeout)}
		case <-ctx.Done():
			// The caller's own cancellation, reported as a finding because
			// the signature has no other channel for it — the same seam
			// the git probes leave to the doctor command. Distinct words,
			// so an interrupted run never reads as a silent watcher.
			return Result{Name: name, Status: Red, Message: fmt.Sprintf("file watcher probe interrupted: %v", ctx.Err())}
		}
	}
}

// limitHint appends the kernel limit behind an errno whose literal text
// would misdirect. inotify reports an exhausted per-user watch limit as
// ENOSPC — "no space left on device" — which sends a learner to df instead
// of sysctl, and an exhausted instance limit as EMFILE, which reads as an
// ordinary descriptor leak. fsnotify's FAQ documents both under exactly
// these sysctl names. Empty for every other error.
func limitHint(err error) string {
	switch {
	case errors.Is(err, syscall.ENOSPC):
		return " (inotify's per-user watch limit is exhausted, not the disk — raise fs.inotify.max_user_watches)"
	case errors.Is(err, syscall.EMFILE):
		return " (the open-file limit is reached; on Linux this is usually fs.inotify.max_user_instances)"
	default:
		return ""
	}
}
