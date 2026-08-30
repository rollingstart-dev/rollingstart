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

// watcherName labels the watcher probe's findings, shared by the setup in
// watcher and the wait loop in awaitEvent.
const watcherName = "file watcher"

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
//
// The synthetic file exists for the length of the wait, so this probe must
// not run alongside CleanTree, which would report that file as the
// learner's — see the package doc.
func Watcher(ctx context.Context, dir string) Result {
	return watcher(ctx, dir, dir, watcherTimeout)
}

// watcher is the seam behind Watcher. The watched directory and the
// directory the synthetic file is written into are separate parameters so a
// test can force the no-event path deterministically — write where the
// watcher is not looking — instead of needing a real WSL2 boundary. In
// production they are the same directory.
func watcher(ctx context.Context, watchDir, writeDir string, timeout time.Duration) (res Result) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return Result{Name: watcherName, Status: Red, Message: fmt.Sprintf("cannot create a file watcher: %v%s", err, limitHint(err))}
	}
	// Close also stops fsnotify's reader goroutine and waits for it.
	defer w.Close()

	if err := w.Add(watchDir); err != nil {
		return Result{Name: watcherName, Status: Red, Message: fmt.Sprintf("cannot watch %s: %v%s", watchDir, err, limitHint(err))}
	}

	// The watch is registered before the write, so the event cannot be
	// missed by arriving early.
	f, err := os.CreateTemp(writeDir, probeFilePrefix+"*")
	if err != nil {
		// Nothing was ever there to observe: a read-only checkout, not a
		// silent watcher, and the finding must not blame WSL2 for it.
		return Result{Name: watcherName, Status: Red, Message: fmt.Sprintf("cannot write a synthetic file in the checkout: %v", err)}
	}
	path := f.Name()
	defer func() {
		// Deferred rather than sequenced so a red probe cleans up too. A
		// file left behind would dirty the tree and trip the clean-tree
		// probe on the next run, so a failed removal is a finding in its
		// own right and outranks whatever the watcher reported.
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			res = Result{Name: watcherName, Status: Red, Message: fmt.Sprintf("synthetic file %s was left behind: %v — remove it by hand", path, err)}
		}
	}()
	if err := f.Close(); err != nil {
		return Result{Name: watcherName, Status: Red, Message: fmt.Sprintf("cannot write a synthetic file in the checkout: %v", err)}
	}

	return awaitEvent(ctx, w.Events, w.Errors, filepath.Base(path), timeout)
}

// awaitEvent is the wait loop, over channels rather than a Watcher so the
// paths fsnotify produces only under load or on shutdown — an overflowing
// queue, a closed channel — can be driven from a test deterministically.
// base is the synthetic file's name; only its event counts.
func awaitEvent(ctx context.Context, events <-chan fsnotify.Event, errs <-chan error, base string, timeout time.Duration) Result {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	overflowed := false
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return Result{Name: watcherName, Status: Red, Message: "file watcher closed before any event arrived"}
			}
			// Matched on the base name: fsnotify reports paths under the
			// spelling given to Add, which need not match CreateTemp's
			// once symlinks are involved (macOS's /var → /private/var),
			// and the random suffix makes the base unique to this run.
			// Any op counts — a Remove or Chmod for this file proves
			// delivery exactly as a Create does. Other events are skipped,
			// not mistaken for success: the checkout root is a busy place.
			// Nor do they count as delivery evidence the way an overflow
			// does below. The probe's claim is that the write it made was
			// noticed; an overflow is the kernel's own attestation that
			// events were dropped, a stronger excuse for a missing one
			// than a neighbour's activity.
			if filepath.Base(ev.Name) == base {
				return Result{Name: watcherName, Status: Green, Message: "file events are delivered"}
			}
		case err, ok := <-errs:
			if !ok {
				return Result{Name: watcherName, Status: Red, Message: "file watcher closed before any event arrived"}
			}
			// An overflow is the kernel dropping events because too many
			// arrived for the watched directory: the checkout root was
			// busy during the probe. That is evidence of delivery, not of
			// its absence, and inotify keeps reading afterwards — so keep
			// waiting, since the synthetic file's own event usually still
			// comes through, and let the timer arm know it may have been
			// among the dropped.
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				overflowed = true
				continue
			}
			return Result{Name: watcherName, Status: Red, Message: fmt.Sprintf("file watcher reported an error: %v", err)}
		case <-timer.C:
			if overflowed {
				// Nothing arrived for the synthetic file, but thousands of
				// events arrived for the directory around it. The boundary
				// delivers; the flood swallowed one event, and the WSL2
				// diagnosis would be a lie.
				return Result{Name: watcherName, Status: Green, Message: "file events are delivered (the event queue overflowed during the probe — the checkout root was busy)"}
			}
			return Result{Name: watcherName, Status: Red, Message: fmt.Sprintf(
				"wrote %s in the checkout but no file event arrived within %s — the likely cause is a filesystem "+
					"that does not deliver change notifications: a repository on a Windows drive (/mnt/c) watched "+
					"from WSL2, a VirtualBox shared folder, or a network mount; clone it onto a local filesystem instead",
				base, timeout)}
		case <-ctx.Done():
			// The caller's own cancellation, reported as a finding because
			// the signature has no other channel for it — the same seam
			// the git probes leave to the doctor command. Distinct words,
			// so an interrupted run never reads as a silent watcher.
			return Result{Name: watcherName, Status: Red, Message: fmt.Sprintf("file watcher probe interrupted: %v", ctx.Err())}
		}
	}
}

// limitHint appends the kernel limit behind an errno whose literal text
// would misdirect. inotify reports an exhausted per-user watch limit as
// ENOSPC — "no space left on device" — which sends a learner to df instead
// of sysctl. EMFILE is the descriptor limit on both platforms, and the one
// kqueue reaches first: fsnotify opens a descriptor per entry in the
// watched directory, so a large checkout root against a default ulimit is
// the realistic trigger on macOS, where no sysctl helps; on Linux the
// inotify instance limit reports the same errno. Empty for every other
// error.
func limitHint(err error) string {
	switch {
	case errors.Is(err, syscall.ENOSPC):
		return " (inotify's per-user watch limit is exhausted, not the disk — raise fs.inotify.max_user_watches)"
	case errors.Is(err, syscall.EMFILE):
		return " (the open-file limit is reached — raise ulimit -n; on Linux, fs.inotify.max_user_instances can also report this)"
	default:
		return ""
	}
}
