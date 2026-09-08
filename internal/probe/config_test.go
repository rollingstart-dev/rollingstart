package probe

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rollingstart-dev/rollingstart/internal/instance"
)

func TestInstanceConfig(t *testing.T) {
	tests := []struct {
		name       string
		missing    bool   // no definition file at all
		content    string // written to .rollingstart/instance.toml otherwise
		wantStatus Status
		wantIn     []string
	}{
		{
			name:       "no definition",
			missing:    true,
			wantStatus: Red,
			wantIn:     []string{"no instance definition"},
		},
		{
			name:       "minimal valid",
			content:    "[commands]\nbuild = \"pnpm build\"\n",
			wantStatus: Green,
			wantIn:     []string{"loaded", "1 command"},
		},
		{
			name:       "zero commands",
			content:    "",
			wantStatus: Green,
			wantIn:     []string{"loaded", "no commands declared"},
		},
		{
			// The loader's position must survive into the message verbatim,
			// so doctor can point at the offending line.
			name:       "syntax error",
			content:    "[commands\n",
			wantStatus: Red,
			wantIn:     []string{"instance.toml:1"},
		},
		{
			// Both unknown keys, not just the first — the loader joins them
			// and the probe must not collapse the join.
			name:       "unknown keys",
			content:    "[commands]\nbiuld = \"x\"\nfrobnicate = \"y\"\n",
			wantStatus: Red,
			wantIn:     []string{"biuld", "frobnicate"},
		},
		{
			name:       "empty command string",
			content:    "[commands]\ntest = \"  \"\n",
			wantStatus: Red,
			wantIn:     []string{"commands.test", "empty"},
		},
		// Operations are counted beside the commands, never run, and the
		// phrasing follows the reference page's composition rule: each
		// phrase keeps its singular and plural, one "declared" governs
		// both, and a definition with no operations keeps the v0 wording.
		{
			name:       "one command, one operation",
			content:    "[commands]\nbuild = \"x\"\n[operations]\nreset-db = { command = \"y\", destructive = true }\n",
			wantStatus: Green,
			wantIn:     []string{"loaded (1 command, 1 operation declared)"},
		},
		{
			name:       "commands and operations",
			content:    "[commands]\nbuild = \"x\"\ntest = \"x\"\nlint = \"x\"\n[operations]\nreset-db = { command = \"y\" }\nseed-db = { command = \"z\" }\n",
			wantStatus: Green,
			wantIn:     []string{"loaded (3 commands, 2 operations declared)"},
		},
		{
			name:       "operations without commands",
			content:    "[operations]\nreset-db = { command = \"y\" }\nseed-db = { command = \"z\" }\n",
			wantStatus: Green,
			wantIn:     []string{"loaded (no commands, 2 operations declared)"},
		},
		{
			name:       "commands without operations keep the v0 wording",
			content:    "[commands]\nbuild = \"x\"\ntest = \"x\"\n",
			wantStatus: Green,
			wantIn:     []string{"loaded (2 commands declared)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.missing {
				path := filepath.Join(dir, instance.Path)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			res := InstanceConfig(context.Background(), dir)
			if res.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v (message: %q)", res.Status, tt.wantStatus, res.Message)
			}
			for _, want := range tt.wantIn {
				if !strings.Contains(res.Message, want) {
					t.Errorf("Message %q does not contain %q", res.Message, want)
				}
			}
			if res.Name == "" {
				t.Error("Name is empty")
			}
		})
	}
}
