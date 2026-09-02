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
		// wantKey, when set, asserts the error is a *ParseError carrying
		// this Key — the loader's own value checks promise the type and
		// the offending key, and a bare fmt.Errorf with the right wording
		// would break the consumer the probe layer plans to become.
		// Decoder-produced failures leave it empty: their keys are the
		// library's to shape.
		wantKey string
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
			wantKey:   "commands.build",
		},
		{
			name:      "whitespace-only command string",
			toml:      "[commands]\nlint = \"   \"\n",
			wantInMsg: []string{"commands.lint", "empty"},
			wantKey:   "commands.lint",
		},
		{
			// The missing key and the empty value are different mistakes,
			// worded apart — the spec's rule, asserted by the two cases
			// below wanting different substrings.
			name:      "operation without a command",
			toml:      "[operations]\nreset-db = { destructive = true }\n",
			wantInMsg: []string{"operations.reset-db", "no command"},
			wantKey:   "operations.reset-db",
		},
		{
			name:      "operation with an empty command",
			toml:      "[operations.reset-db]\ncommand = \"\"\n",
			wantInMsg: []string{"operations.reset-db.command", "empty"},
			wantKey:   "operations.reset-db.command",
		},
		{
			name:      "operation with a whitespace-only command",
			toml:      "[operations.seed-db]\ncommand = \"   \"\n",
			wantInMsg: []string{"operations.seed-db.command", "empty"},
			wantKey:   "operations.seed-db.command",
		},
		{
			name:      "operation with an empty name",
			toml:      "[operations]\n\"\" = { command = \"pnpm db:seed\" }\n",
			wantInMsg: []string{"operations", "empty name"},
			wantKey:   "operations",
		},
		{
			// The quoted echo matters here: the author is staring at a key
			// that looks non-empty on screen.
			name:      "operation with a whitespace-only name",
			toml:      "[operations.\"  \"]\ncommand = \"pnpm db:seed\"\n",
			wantInMsg: []string{"operations", `"  "`, "whitespace"},
			wantKey:   "operations",
		},
		{
			// A padded name looks identical to reset-db on screen and can
			// never be selected by it.
			name:      "operation with a padded name",
			toml:      "[operations.\" reset-db\"]\ncommand = \"pnpm db:reset\"\n",
			wantInMsg: []string{"operations", `" reset-db"`, "padded"},
			wantKey:   "operations",
		},
		{
			// Strictness reaches inside the named sub-tables: a typo in an
			// operation's keys fails with the decoder's position, like every
			// other unknown key.
			name:      "unknown key inside an operation",
			toml:      "[operations.reset-db]\ncommand = \"pnpm db:reset\"\ndestrutive = true\n",
			wantInMsg: []string{"destrutive", "3:"},
		},
		{
			name:      "unknown key inside corpus",
			toml:      "[corpus]\nexemplray = [\"apps/web\"]\n",
			wantInMsg: []string{"exemplray", "2:"},
		},
		{
			// An empty entry has no value to echo, so the error counts:
			// entry 2, 1-based, the way an author reads the list.
			name:      "empty exemplary entry",
			toml:      "[corpus]\nexemplary = [\"apps/web\", \"\"]\n",
			wantInMsg: []string{"corpus.exemplary", "entry 2", "empty"},
			wantKey:   "corpus.exemplary",
		},
		{
			name:      "absolute exemplary path",
			toml:      "[corpus]\nexemplary = [\"/srv/app\"]\n",
			wantInMsg: []string{"corpus.exemplary", "/srv/app", "repository-relative"},
			wantKey:   "corpus.exemplary",
		},
		{
			name:      "exemplary path escaping the repository",
			toml:      "[corpus]\nexemplary = [\"../secrets\"]\n",
			wantInMsg: []string{"corpus.exemplary", "../secrets", "escapes"},
			wantKey:   "corpus.exemplary",
		},
		{
			// Escape detection resolves the path first: a prefix that dips
			// back out through an interior .. is still an escape.
			name:      "exemplary path escaping through an interior dot-dot",
			toml:      "[corpus]\nexemplary = [\"apps/../../other\"]\n",
			wantInMsg: []string{"corpus.exemplary", "apps/../../other", "escapes"},
			wantKey:   "corpus.exemplary",
		},
		{
			name:      "padded exemplary entry",
			toml:      "[corpus]\nexemplary = [\" apps/web\"]\n",
			wantInMsg: []string{"corpus.exemplary", `" apps/web"`, "padded"},
			wantKey:   "corpus.exemplary",
		},
		{
			name:      "empty exemplar-prs entry",
			toml:      "[corpus]\nexemplar-prs = [\"\"]\n",
			wantInMsg: []string{"corpus.exemplar-prs", "entry 1", "empty"},
			wantKey:   "corpus.exemplar-prs",
		},
		{
			// net/url happily parses a bare repository path as a relative
			// URL — the spec pins absolute, http(s), nonempty host.
			name:      "exemplar-prs entry that is a bare path",
			toml:      "[corpus]\nexemplar-prs = [\"lukevella/rallly/pull/1502\"]\n",
			wantInMsg: []string{"corpus.exemplar-prs", "lukevella/rallly/pull/1502", "http(s) URL"},
			wantKey:   "corpus.exemplar-prs",
		},
		{
			name:      "exemplar-prs entry with the wrong scheme",
			toml:      "[corpus]\nexemplar-prs = [\"ftp://github.com/x/pull/1\"]\n",
			wantInMsg: []string{"corpus.exemplar-prs", "ftp://github.com/x/pull/1", "http(s) URL"},
			wantKey:   "corpus.exemplar-prs",
		},
		{
			name:      "exemplar-prs entry without a host",
			toml:      "[corpus]\nexemplar-prs = [\"https:///pull/1\"]\n",
			wantInMsg: []string{"corpus.exemplar-prs", "http(s) URL"},
			wantKey:   "corpus.exemplar-prs",
		},
		{
			// Trailing padding, deliberately: url.Parse would accept this
			// one, so only the padding rule stands between it and a doctor
			// note nobody can explain.
			name:      "padded exemplar-prs entry",
			toml:      "[corpus]\nexemplar-prs = [\"https://github.com/x/pull/1 \"]\n",
			wantInMsg: []string{"corpus.exemplar-prs", "padded"},
			wantKey:   "corpus.exemplar-prs",
		},
		{
			name:      "empty definition-of-ready",
			toml:      "[corpus]\ndefinition-of-ready = \"\"\n",
			wantInMsg: []string{"corpus.definition-of-ready", "empty"},
			wantKey:   "corpus.definition-of-ready",
		},
		{
			name:      "absolute definition-of-ready",
			toml:      "[corpus]\ndefinition-of-ready = \"/etc/ready.md\"\n",
			wantInMsg: []string{"corpus.definition-of-ready", "repository-relative"},
			wantKey:   "corpus.definition-of-ready",
		},
		{
			name:      "definition-of-ready escaping the repository",
			toml:      "[corpus]\ndefinition-of-ready = \"../ready.md\"\n",
			wantInMsg: []string{"corpus.definition-of-ready", "escapes"},
			wantKey:   "corpus.definition-of-ready",
		},
		{
			name:      "padded definition-of-ready",
			toml:      "[corpus]\ndefinition-of-ready = \" ready.md\"\n",
			wantInMsg: []string{"corpus.definition-of-ready", `" ready.md"`, "padded"},
			wantKey:   "corpus.definition-of-ready",
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
			if tt.wantKey != "" {
				var pe *ParseError
				if !errors.As(err, &pe) {
					t.Fatalf("error %v is not a *ParseError", err)
				}
				if pe.Key != tt.wantKey {
					t.Errorf("Key = %q, want %q", pe.Key, tt.wantKey)
				}
			}
		})
	}
}

func TestLoadOperations(t *testing.T) {
	tests := []struct {
		name string
		toml string
		want []Operation
	}{
		{
			// The inline form and the header form are the same document;
			// declaration order is reversed against the expected order
			// because names, sorted, are the canonical order — a TOML
			// table has no declaration order once decoded.
			name: "both forms, sorted by name",
			toml: "[operations]\nseed-db = { command = \"pnpm db:seed\" }\n\n[operations.reset-db]\ncommand = \"pnpm db:reset --force\"\ndestructive = true\n",
			want: []Operation{
				{Name: "reset-db", Cmd: "pnpm db:reset --force", Destructive: true},
				{Name: "seed-db", Cmd: "pnpm db:seed"},
			},
		},
		{
			name: "destructive declared false is the same as omitted",
			toml: "[operations]\nseed-db = { command = \"pnpm db:seed\", destructive = false }\n",
			want: []Operation{{Name: "seed-db", Cmd: "pnpm db:seed"}},
		},
		{
			name: "empty operations table declares none",
			toml: "[operations]\n",
			want: nil,
		},
		{
			name: "v0 file declares none",
			toml: "[commands]\ntest = \"pnpm test\"\n",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst, err := Load(write(t, "instance.toml", tt.toml))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := inst.Operations(); !slices.Equal(got, tt.want) {
				t.Errorf("Operations() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestOperationNamespaceIsSeparate: an operation may be called build without
// colliding with [commands].build — anything that later selects one says
// which kind it selects.
func TestOperationNamespaceIsSeparate(t *testing.T) {
	inst, err := Load(write(t, "instance.toml",
		"[commands]\nbuild = \"pnpm build\"\n\n[operations]\nbuild = { command = \"pnpm generate\" }\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []Command{{Name: "build", Cmd: "pnpm build"}}; !slices.Equal(inst.Commands(), want) {
		t.Errorf("Commands() = %v, want %v", inst.Commands(), want)
	}
	if want := []Operation{{Name: "build", Cmd: "pnpm generate"}}; !slices.Equal(inst.Operations(), want) {
		t.Errorf("Operations() = %v, want %v", inst.Operations(), want)
	}
}

func TestLoadCorpus(t *testing.T) {
	t.Run("all keys, lists in declaration order", func(t *testing.T) {
		inst, err := Load(write(t, "instance.toml",
			"[corpus]\nexemplary = [\"packages/database\", \"apps/web/src/features/poll\"]\nexemplar-prs = [\"https://github.com/lukevella/rallly/pull/1502\"]\ndefinition-of-ready = \".rollingstart/ready.md\"\n"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		got := inst.Corpus()
		if want := []string{"packages/database", "apps/web/src/features/poll"}; !slices.Equal(got.Exemplary, want) {
			t.Errorf("Exemplary = %v, want %v (declaration order)", got.Exemplary, want)
		}
		if want := []string{"https://github.com/lukevella/rallly/pull/1502"}; !slices.Equal(got.ExemplarPRs, want) {
			t.Errorf("ExemplarPRs = %v, want %v", got.ExemplarPRs, want)
		}
		if want := ".rollingstart/ready.md"; got.DefinitionOfReady != want {
			t.Errorf("DefinitionOfReady = %q, want %q", got.DefinitionOfReady, want)
		}
	})
	t.Run("v0 file declares nothing", func(t *testing.T) {
		inst, err := Load(write(t, "instance.toml", "[commands]\ntest = \"pnpm test\"\n"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := inst.Corpus(); got.Exemplary != nil || got.ExemplarPRs != nil || got.DefinitionOfReady != "" {
			t.Errorf("Corpus() = %+v, want the zero value", got)
		}
	})
	t.Run("empty corpus table declares nothing", func(t *testing.T) {
		inst, err := Load(write(t, "instance.toml", "[corpus]\n"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := inst.Corpus(); got.Exemplary != nil || got.ExemplarPRs != nil || got.DefinitionOfReady != "" {
			t.Errorf("Corpus() = %+v, want the zero value", got)
		}
	})
	t.Run("explicitly empty lists declare nothing", func(t *testing.T) {
		// Normalized to the zero value at load: [] and an absent key both
		// declare nothing, and consumers get one state to check, not two.
		inst, err := Load(write(t, "instance.toml", "[corpus]\nexemplary = []\nexemplar-prs = []\n"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got := inst.Corpus(); got.Exemplary != nil || got.ExemplarPRs != nil {
			t.Errorf("Corpus() = %+v, want nil lists", got)
		}
	})
}

// TestDocExamplesLoad keeps docs/reference/instance-toml.md honest: every
// toml-fenced block on the page must load. The page is the spec, and a spec
// whose own examples fail the loader is the drift this test exists to
// prevent — the loader-side analog of the doctor page's rendered-output test.
func TestDocExamplesLoad(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "instance-toml.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks := strings.Split(string(doc), "```")
	checked := 0
	for i := 1; i < len(blocks); i += 2 { // odd indices are inside fences
		// The language is the info string's first word, so a block that
		// grows fence attributes later still gets checked rather than
		// silently slipping under the floor.
		info, block, found := strings.Cut(blocks[i], "\n")
		if fields := strings.Fields(info); !found || len(fields) == 0 || fields[0] != "toml" {
			continue
		}
		checked++
		if _, err := Load(write(t, "instance.toml", block)); err != nil {
			t.Errorf("the reference page shows a document the loader rejects: %v\n%s", err, block)
		}
	}
	// A floor, not an exact count: additions are welcome, but a drop below
	// what existed when this was written means a block was silently
	// exempted from the guard.
	if checked < 3 {
		t.Fatalf("only %d toml example blocks checked; 3 existed when this guard was written — was one silently exempted?", checked)
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
