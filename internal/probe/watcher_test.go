package probe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// entries lists a directory's names so a test can assert the probe left the
// tree exactly as it found it.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(des))
	for _, de := range des {
		names = append(names, de.Name())
	}
	return names
}

func TestWatcherEventReceived(t *testing.T) {
	dir := t.TempDir()
	// A file already in the checkout, to prove the probe removes only its
	// own.
	write(t, dir, "README.md", "learner work\n")

	res := Watcher(context.Background(), dir)
	if res.Status != Green {
		t.Fatalf("Status = %v, want %v (message: %q)", res.Status, Green, res.Message)
	}
	if !strings.Contains(res.Message, "delivered") {
		t.Errorf("Message %q does not say events are delivered", res.Message)
	}
	if res.Name == "" {
		t.Error("Name is empty")
	}
	if got, want := entries(t, dir), []string{"README.md"}; !slices.Equal(got, want) {
		t.Errorf("tree after probe = %v, want %v", got, want)
	}
}

func TestWatcherRed(t *testing.T) {
	type fixture struct {
		ctx      context.Context
		watchDir string
		writeDir string
	}
	tests := []struct {
		name      string
		setup     func(t *testing.T) fixture
		timeout   time.Duration
		wantIn    []string
		wantNotIn []string
	}{
		{
			// The seam: the synthetic file lands where the watcher is not
			// looking, so the event can never arrive — the silent-WSL2
			// failure, forced without a WSL2 boundary.
			name: "no event arrives",
			setup: func(t *testing.T) fixture {
				return fixture{context.Background(), t.TempDir(), t.TempDir()}
			},
			timeout: 200 * time.Millisecond,
			wantIn:  []string{"no file event arrived", "200ms", "WSL2", probeFilePrefix},
		},
		{
			name: "watched directory missing",
			setup: func(t *testing.T) fixture {
				return fixture{context.Background(), filepath.Join(t.TempDir(), "gone"), t.TempDir()}
			},
			timeout:   time.Minute,
			wantIn:    []string{"cannot watch"},
			wantNotIn: []string{"WSL2"},
		},
		{
			// A read-only checkout: the write fails, so nothing was ever
			// there to observe, and the finding must say that rather than
			// blame the watcher.
			name: "checkout not writable",
			setup: func(t *testing.T) fixture {
				if os.Geteuid() == 0 {
					t.Skip("permission bits do not bind root")
				}
				writeDir := t.TempDir()
				if err := os.Chmod(writeDir, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(writeDir, 0o700) })
				return fixture{context.Background(), t.TempDir(), writeDir}
			},
			timeout:   time.Minute,
			wantIn:    []string{"cannot write", "permission denied"},
			wantNotIn: []string{"WSL2"},
		},
		{
			// Ctrl-C mid-probe. Separate directories keep every other
			// select arm quiet, so cancellation is the only thing that can
			// return — deterministic, not a race the test usually wins.
			name: "cancelled",
			setup: func(t *testing.T) fixture {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return fixture{ctx, t.TempDir(), t.TempDir()}
			},
			timeout:   time.Minute,
			wantIn:    []string{"interrupted", context.Canceled.Error()},
			wantNotIn: []string{"WSL2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.setup(t)
			res := watcher(f.ctx, f.watchDir, f.writeDir, tt.timeout)
			if res.Status != Red {
				t.Errorf("Status = %v, want %v (message: %q)", res.Status, Red, res.Message)
			}
			for _, want := range tt.wantIn {
				if !strings.Contains(res.Message, want) {
					t.Errorf("Message %q does not contain %q", res.Message, want)
				}
			}
			for _, not := range tt.wantNotIn {
				if strings.Contains(res.Message, not) {
					t.Errorf("Message %q contains %q — a misattributed cause", res.Message, not)
				}
			}
			// Cleanup runs on every path: a red probe leaves the tree as
			// it found it, so it cannot trip the clean-tree probe.
			if got := entries(t, f.writeDir); len(got) != 0 {
				t.Errorf("write directory after probe = %v, want empty", got)
			}
		})
	}
}

// TestWatcherRemovalFailureOutranksResult covers the one branch that
// rewrites a finding after the fact: the wait ran, but the synthetic file
// could not be removed, and a tree left dirty outranks whatever the watcher
// saw — green or red alike; the override is the same code either way, and
// the timeout path is the one whose timing is deterministic. Forced for
// real rather than through a hook: the write directory loses write
// permission once the file exists, so the deferred removal meets a genuine
// EACCES. The file exists for the whole timeout window, so the poller that
// flips the bit cannot miss it.
func TestWatcherRemovalFailureOutranksResult(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not bind root")
	}
	writeDir := t.TempDir()
	done := make(chan struct{})
	t.Cleanup(func() {
		close(done)
		_ = os.Chmod(writeDir, 0o700)
	})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			if des, err := os.ReadDir(writeDir); err == nil && len(des) > 0 {
				_ = os.Chmod(writeDir, 0o500)
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	res := watcher(context.Background(), t.TempDir(), writeDir, time.Second)
	if res.Status != Red {
		t.Errorf("Status = %v, want %v (message: %q)", res.Status, Red, res.Message)
	}
	for _, want := range []string{"left behind", probeFilePrefix, "permission denied"} {
		if !strings.Contains(res.Message, want) {
			t.Errorf("Message %q does not contain %q", res.Message, want)
		}
	}
	if strings.Contains(res.Message, "WSL2") {
		t.Errorf("Message %q blames the watcher for a removal failure", res.Message)
	}
	// The message must be true: the file really is still there.
	if got := entries(t, writeDir); len(got) != 1 || !strings.HasPrefix(got[0], probeFilePrefix) {
		t.Errorf("write directory after probe = %v, want exactly the synthetic file", got)
	}
}

// TestAwaitEvent drives the wait loop with the channel states fsnotify
// produces only under load or on shutdown.
func TestAwaitEvent(t *testing.T) {
	const base = ".rollingstart-probe-test"
	ours := fsnotify.Event{Name: filepath.Join("/checkout", base), Op: fsnotify.Create}
	other := fsnotify.Event{Name: "/checkout/README.md", Op: fsnotify.Write}
	tests := []struct {
		name       string
		drive      func(events chan<- fsnotify.Event, errs chan<- error)
		wantStatus Status
		wantIn     []string
		wantNotIn  []string
	}{
		{
			name:       "our event",
			drive:      func(events chan<- fsnotify.Event, _ chan<- error) { events <- ours },
			wantStatus: Green,
			wantIn:     []string{"delivered"},
			wantNotIn:  []string{"overflowed"},
		},
		{
			// The checkout root is busy: events for the learner's files
			// are skipped, not mistaken for success.
			name:       "unrelated event then ours",
			drive:      func(events chan<- fsnotify.Event, _ chan<- error) { events <- other; events <- ours },
			wantStatus: Green,
			wantIn:     []string{"delivered"},
		},
		{
			name:       "unrelated events only",
			drive:      func(events chan<- fsnotify.Event, _ chan<- error) { events <- other },
			wantStatus: Red,
			wantIn:     []string{"no file event arrived", "WSL2", "network mount"},
		},
		{
			// An overflow is the kernel dropping events because too many
			// arrived — delivery evidence, not its absence. Sequenced: our
			// event is sent only once the loop has taken the overflow, so
			// the case exercises the overflow arm every run rather than
			// whenever select happens to pick it first.
			name: "overflow then ours",
			drive: func(events chan<- fsnotify.Event, errs chan<- error) {
				errs <- fsnotify.ErrEventOverflow
				go func() {
					for len(errs) > 0 {
						time.Sleep(time.Millisecond)
					}
					events <- ours
				}()
			},
			wantStatus: Green,
			wantIn:     []string{"delivered"},
		},
		{
			// The flood swallowed our event. Still green — thousands of
			// events for this directory arrived — and never the WSL2
			// diagnosis.
			name:       "overflow then silence",
			drive:      func(_ chan<- fsnotify.Event, errs chan<- error) { errs <- fsnotify.ErrEventOverflow },
			wantStatus: Green,
			wantIn:     []string{"delivered", "overflowed"},
			wantNotIn:  []string{"WSL2"},
		},
		{
			name:       "other error",
			drive:      func(_ chan<- fsnotify.Event, errs chan<- error) { errs <- errors.New("boom") },
			wantStatus: Red,
			wantIn:     []string{"reported an error", "boom"},
			wantNotIn:  []string{"WSL2"},
		},
		{
			name:       "events channel closed",
			drive:      func(events chan<- fsnotify.Event, _ chan<- error) { close(events) },
			wantStatus: Red,
			wantIn:     []string{"closed"},
			wantNotIn:  []string{"WSL2"},
		},
		{
			name:       "errors channel closed",
			drive:      func(_ chan<- fsnotify.Event, errs chan<- error) { close(errs) },
			wantStatus: Red,
			wantIn:     []string{"closed"},
			wantNotIn:  []string{"WSL2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Buffered and pre-filled, so the loop sees every state the
			// case describes without a goroutine to race against.
			events := make(chan fsnotify.Event, 4)
			errs := make(chan error, 4)
			tt.drive(events, errs)
			res := awaitEvent(context.Background(), events, errs, base, 50*time.Millisecond)
			if res.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v (message: %q)", res.Status, tt.wantStatus, res.Message)
			}
			for _, want := range tt.wantIn {
				if !strings.Contains(res.Message, want) {
					t.Errorf("Message %q does not contain %q", res.Message, want)
				}
			}
			for _, not := range tt.wantNotIn {
				if strings.Contains(res.Message, not) {
					t.Errorf("Message %q contains %q — a misattributed cause", res.Message, not)
				}
			}
		})
	}
}

// TestLimitHint pins the two errno translations: the kernel limits that
// surface under misleading names, and nothing else.
func TestLimitHint(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		wantIn []string // empty: no hint at all
	}{
		{"watch limit", syscall.ENOSPC, []string{"max_user_watches"}},
		// ulimit first: kqueue reaches EMFILE by opening a descriptor per
		// directory entry, where no sysctl helps.
		{"descriptor limit", syscall.EMFILE, []string{"ulimit -n", "max_user_instances"}},
		{"wrapped by fsnotify", fmt.Errorf("couldn't initialize inotify: %w", syscall.EMFILE), []string{"ulimit -n"}},
		{"anything else", errors.New("boom"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := limitHint(tt.err)
			if len(tt.wantIn) == 0 && got != "" {
				t.Errorf("limitHint(%v) = %q, want empty", tt.err, got)
			}
			for _, want := range tt.wantIn {
				if !strings.Contains(got, want) {
					t.Errorf("limitHint(%v) = %q, want it to name %s", tt.err, got, want)
				}
			}
		})
	}
}
