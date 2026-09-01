package instance

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// write puts content at name under a fresh temp dir and returns the full path.
func write(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValid(t *testing.T) {
	tests := []struct {
		name string
		toml string
		want []Command
	}{
		{
			name: "minimal single command",
			toml: "[commands]\ntest = \"pnpm test\"\n",
			want: []Command{{Name: "test", Cmd: "pnpm test"}},
		},
		{
			name: "all four in canonical order regardless of file order",
			toml: "[commands]\nlint = \"pnpm lint\"\ntest = \"pnpm test\"\nbuild = \"pnpm build\"\ntypecheck = \"pnpm typecheck\"\n",
			want: []Command{
				{Name: "build", Cmd: "pnpm build"},
				{Name: "typecheck", Cmd: "pnpm typecheck"},
				{Name: "test", Cmd: "pnpm test"},
				{Name: "lint", Cmd: "pnpm lint"},
			},
		},
		{
			name: "empty file declares nothing",
			toml: "",
			want: nil,
		},
		{
			name: "empty commands table declares nothing",
			toml: "[commands]\n",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst, err := Load(write(t, "instance.toml", tt.toml))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := inst.Commands()
			if len(got) != len(tt.want) {
				t.Fatalf("Commands() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Commands()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		name string
		toml string
		// wantInMsg must all appear in err.Error(); position info is part of
		// the contract, so failures with a location assert on "line".
		wantInMsg []string
	}{
		{
			name:      "unknown top-level table",
			toml:      "[comands]\ntest = \"pnpm test\"\n",
			wantInMsg: []string{"comands", "1:"},
		},
		{
			name:      "unknown key inside commands",
			toml:      "[commands]\nbild = \"pnpm build\"\n",
			wantInMsg: []string{"bild", "2:"},
		},
		{
			name:      "toml syntax error",
			toml:      "[commands\ntest = \"pnpm test\"\n",
			wantInMsg: []string{"1:"},
		},
		{
			name:      "empty command string",
			toml:      "[commands]\nbuild = \"\"\n",
			wantInMsg: []string{"commands.build", "empty"},
		},
		{
			name:      "whitespace-only command string",
			toml:      "[commands]\nlint = \"   \"\n",
			wantInMsg: []string{"commands.lint", "empty"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := write(t, "instance.toml", tt.toml)
			_, err := Load(path)
			if err == nil {
				t.Fatal("Load succeeded, want error")
			}
			if errors.Is(err, ErrNotFound) {
				t.Fatalf("Load: %v matches ErrNotFound; this file exists", err)
			}
			for _, want := range tt.wantInMsg {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("error %q does not name the file %q", err, path)
			}
		})
	}
}

func TestLoadUnknownKeyIsParseErrorWithDetail(t *testing.T) {
	path := write(t, "instance.toml", "[comands]\ntest = \"pnpm test\"\n")
	_, err := Load(path)
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("Load: %v is not a *ParseError", err)
	}
	if pe.Line != 1 {
		t.Errorf("Line = %d, want 1", pe.Line)
	}
	if pe.Key != "comands" {
		t.Errorf("Key = %q, want %q", pe.Key, "comands")
	}
	if pe.Detail() == "" {
		t.Error("Detail() is empty, want a source excerpt for verbatim display")
	}
}

func TestLoadMissing(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := Load(filepath.Join(t.TempDir(), "instance.toml"))
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Load: %v, want ErrNotFound", err)
		}
	})
	t.Run("missing directory", func(t *testing.T) {
		_, err := Load(filepath.Join(t.TempDir(), ".rollingstart", "instance.toml"))
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Load: %v, want ErrNotFound", err)
		}
	})
}

// TestLoadThisRepositoryDefinition loads the repository's own definition.
// It is the reference instance other authors will copy, and
// docs/workflow.md claims it mirrors CI; the probe self-test loads it too,
// but only in CI. A typo in it must fail go test everywhere rather than
// survive until someone runs doctor.
func TestLoadThisRepositoryDefinition(t *testing.T) {
	inst, err := Load(filepath.Join("..", "..", Path))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range inst.Commands() {
		names = append(names, c.Name)
	}
	if want := []string{"build", "test", "lint"}; !slices.Equal(names, want) {
		t.Errorf("commands = %v, want %v", names, want)
	}
}

// TestLoadRallyExample: the example definition is the first one an instance
// author sees, and it must parse — a typo there teaches the wrong lesson.
func TestLoadRallyExample(t *testing.T) {
	inst, err := Load(filepath.Join("..", "..", "examples", "rallly", ".rollingstart", "instance.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Command{
		{Name: "build", Cmd: "pnpm build"},
		{Name: "typecheck", Cmd: "pnpm type-check"},
		{Name: "test", Cmd: "pnpm test:unit"},
		{Name: "lint", Cmd: "pnpm check"},
	}
	if got := inst.Commands(); !slices.Equal(got, want) {
		t.Errorf("Commands() = %v, want %v", got, want)
	}
}
