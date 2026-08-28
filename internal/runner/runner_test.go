package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunOutcomes(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		wantOutcome  Outcome
		wantExitCode int
		wantInOutput string
	}{
		{
			name:         "succeeded",
			command:      "echo hello",
			wantOutcome:  Succeeded,
			wantExitCode: 0,
			wantInOutput: "hello",
		},
		{
			name:         "failed preserves exit code",
			command:      "echo broken >&2; exit 3",
			wantOutcome:  Failed,
			wantExitCode: 3,
			wantInOutput: "broken",
		},
		{
			name:         "missing binary is not startable",
			command:      "definitely-not-a-real-command-xyz",
			wantOutcome:  NotStartable,
			wantExitCode: 127,
			wantInOutput: "definitely-not-a-real-command-xyz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Run(context.Background(), Spec{Command: tt.command})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if res.Outcome != tt.wantOutcome {
				t.Errorf("Outcome = %v, want %v", res.Outcome, tt.wantOutcome)
			}
			if res.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", res.ExitCode, tt.wantExitCode)
			}
			if !strings.Contains(string(res.Output), tt.wantInOutput) {
				t.Errorf("Output %q does not contain %q", res.Output, tt.wantInOutput)
			}
		})
	}
}

func TestRunInterleavesStdoutAndStderr(t *testing.T) {
	res, err := Run(context.Background(), Spec{Command: "echo out; echo err >&2"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{"out", "err"} {
		if !strings.Contains(string(res.Output), want) {
			t.Errorf("Output %q does not contain %q", res.Output, want)
		}
	}
}

func TestRunShellNotStartable(t *testing.T) {
	// With an empty PATH, sh itself cannot be resolved: the not-startable
	// classification must cover the shell as well as the command.
	t.Setenv("PATH", "")
	res, err := Run(context.Background(), Spec{Command: "echo hi"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Outcome != NotStartable {
		t.Errorf("Outcome = %v, want NotStartable", res.Outcome)
	}
	if len(res.Output) == 0 {
		t.Error("Output is empty, want the start error for display")
	}
}

func TestRunDirIsHonoured(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker"), []byte("here"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), Spec{Command: "cat marker", Dir: dir})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Outcome != Succeeded || !strings.Contains(string(res.Output), "here") {
		t.Errorf("Outcome = %v, Output = %q; want Succeeded reading the marker", res.Outcome, res.Output)
	}
}

func TestRunTimeout(t *testing.T) {
	start := time.Now()
	res, err := Run(context.Background(), Spec{Command: "sleep 30", Timeout: 100 * time.Millisecond})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Outcome != TimedOut {
		t.Errorf("Outcome = %v, want TimedOut", res.Outcome)
	}
	if res.Duration < 100*time.Millisecond {
		t.Errorf("Duration = %v, want >= the 100ms timeout", res.Duration)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Run returned after %v, want promptly after the timeout", elapsed)
	}
}

func TestRunTimeoutKillsProcessGroup(t *testing.T) {
	// The backgrounded child inherits the output pipe. If only sh dies, the
	// child keeps the pipe open and Run blocks until WaitDelay — killing the
	// process group is what makes the return prompt.
	start := time.Now()
	res, err := Run(context.Background(), Spec{Command: "sleep 30 & sleep 30", Timeout: 100 * time.Millisecond})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Outcome != TimedOut {
		t.Errorf("Outcome = %v, want TimedOut", res.Outcome)
	}
	if elapsed > 1*time.Second {
		t.Errorf("Run returned after %v, want well under WaitDelay — the group was not killed", elapsed)
	}
}

func TestRunCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err := Run(ctx, Spec{Command: "sleep 30", Timeout: 10 * time.Second})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run: %v, want context.Canceled — caller cancellation is an error, not an outcome", err)
	}
}

func TestRunOutputKeepsTail(t *testing.T) {
	res, err := Run(context.Background(), Spec{
		Command:     "printf aaaaaaaaaa; printf bbbbbbbbbb",
		OutputLimit: 10,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Truncated {
		t.Error("Truncated = false, want true")
	}
	if got := string(res.Output); got != "bbbbbbbbbb" {
		t.Errorf("Output = %q, want the tail %q", got, "bbbbbbbbbb")
	}
}

func TestRunRecordsDuration(t *testing.T) {
	res, err := Run(context.Background(), Spec{Command: "sleep 0.1"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Duration < 50*time.Millisecond {
		t.Errorf("Duration = %v, want at least the command's runtime", res.Duration)
	}
}
