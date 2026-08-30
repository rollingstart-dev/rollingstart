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

// TestLimitHint pins the two errno translations: the kernel limits that
// inotify reports under misleading names, and nothing else.
func TestLimitHint(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"watch limit", syscall.ENOSPC, "max_user_watches"},
		{"instance limit", syscall.EMFILE, "max_user_instances"},
		{"wrapped by fsnotify", fmt.Errorf("couldn't initialize inotify: %w", syscall.EMFILE), "max_user_instances"},
		{"anything else", errors.New("boom"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := limitHint(tt.err)
			if tt.want == "" && got != "" {
				t.Errorf("limitHint(%v) = %q, want empty", tt.err, got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("limitHint(%v) = %q, want it to name %s", tt.err, got, tt.want)
			}
		})
	}
}
