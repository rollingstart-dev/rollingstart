// Package instance loads the instance definition — the file an instance
// author commits to a target repository to describe it to Rolling Start.
//
// The package knows nothing about git, rendering, or what the commands mean.
// Callers resolve the repository root and interpret the result; Load takes an
// explicit path and reports what it found there.
package instance

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"os"
	slashpath "path"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Path is the instance definition's location relative to the target
// repository's root.
const Path = ".rollingstart/instance.toml"

// ErrNotFound reports that no instance definition exists at the given path.
// A missing .rollingstart directory is the same condition.
var ErrNotFound = errors.New("no instance definition")

// Command is one declared toolchain command.
type Command struct {
	Name string // canonical key: build, typecheck, test, or lint
	Cmd  string // the shell command, exactly as the author wrote it
}

// Operation is one declared lifecycle ritual — reset the database, re-run
// the seeder. In this milestone operations are declared and validated only;
// nothing executes one until the session loop (M3), which honours
// Destructive by prompting.
type Operation struct {
	Name        string // the author's key under [operations]
	Cmd         string // the shell command, exactly as the author wrote it
	Destructive bool   // discards state; running one always prompts first
}

// Corpus is the declared set of corpus pointers. Zero values mean "not
// declared": validation rejects empty strings and empty entries, and an
// explicitly empty list is normalized to nil at load because it declares
// nothing, exactly like an absent key. Validation is form-only — whether a
// pointer's target exists in a given checkout is the doctor's question, not
// the loader's.
type Corpus struct {
	Exemplary         []string // repository-relative paths worth imitating
	ExemplarPRs       []string // absolute http(s) URLs of exemplar pull requests
	DefinitionOfReady string   // repository-relative path to the readiness document
}

// Instance is a parsed and validated instance definition.
type Instance struct {
	commands   []Command
	operations []Operation
	corpus     Corpus
}

// Commands returns the declared commands in canonical order: build,
// typecheck, test, lint. Undeclared commands are absent; a definition
// declaring none returns nil, which is a valid state the caller
// is expected to surface rather than treat as healthy. The slice is the
// caller's own copy — an Instance stays as validated no matter what a
// caller does with what the accessors hand out.
func (i *Instance) Commands() []Command {
	return slices.Clone(i.commands)
}

// Operations returns the declared operations sorted by name — the canonical
// order, because a decoded TOML table has no declaration order to preserve.
// The slice is the caller's own copy.
func (i *Instance) Operations() []Operation {
	return slices.Clone(i.operations)
}

// Corpus returns the declared corpus pointers. Lists keep the file's
// declaration order and are the caller's own copies.
func (i *Instance) Corpus() Corpus {
	c := i.corpus
	c.Exemplary = slices.Clone(c.Exemplary)
	c.ExemplarPRs = slices.Clone(c.ExemplarPRs)
	return c
}

// ParseError is a definition that exists but cannot be used. Error() renders
// one line with the file position; Detail() carries a source excerpt when the
// underlying decoder produced one.
type ParseError struct {
	Path   string
	Line   int // 1-based; 0 when the failure has no position
	Column int // 1-based; 0 when the failure has no position
	// Key is the offending key path, dotted, when known. It is for display,
	// never for splitting: a quoted TOML key such as "a.b" is one segment
	// that carries its own dot.
	Key    string
	Msg    string
	detail string
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: %s", e.Path, e.Line, e.Column, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Msg)
}

// Detail returns a multi-line excerpt of the document around the error,
// suitable for verbatim display. Empty when no excerpt is available.
func (e *ParseError) Detail() string {
	return e.detail
}

// document mirrors schema v1, documented in docs/reference/instance-toml.md.
// Strict decoding rejects everything outside it, including unknown keys
// inside the map-valued [operations.<name>] sub-tables. String fields whose
// absence and emptiness are different mistakes — commands, an operation's
// command, definition-of-ready — are pointers so the two stay
// distinguishable after decoding.
type document struct {
	Commands struct {
		Build     *string `toml:"build"`
		Typecheck *string `toml:"typecheck"`
		Test      *string `toml:"test"`
		Lint      *string `toml:"lint"`
	} `toml:"commands"`
	Operations map[string]operationDoc `toml:"operations"`
	Corpus     struct {
		Exemplary         []string `toml:"exemplary"`
		ExemplarPRs       []string `toml:"exemplar-prs"`
		DefinitionOfReady *string  `toml:"definition-of-ready"`
	} `toml:"corpus"`
}

// operationDoc is one [operations.<name>] table as decoded, before
// validation.
type operationDoc struct {
	Command     *string `toml:"command"`
	Destructive bool    `toml:"destructive"`
}

// Load reads and validates the instance definition at path.
func Load(path string) (*Instance, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w at %s", ErrNotFound, path)
	}
	if err != nil {
		return nil, fmt.Errorf("reading instance definition: %w", err)
	}

	dec := toml.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc document
	if err := dec.Decode(&doc); err != nil {
		return nil, parseError(path, err)
	}

	inst := &Instance{}
	for _, c := range []struct {
		name string
		cmd  *string
	}{
		{"build", doc.Commands.Build},
		{"typecheck", doc.Commands.Typecheck},
		{"test", doc.Commands.Test},
		{"lint", doc.Commands.Lint},
	} {
		if c.cmd == nil {
			continue
		}
		if strings.TrimSpace(*c.cmd) == "" {
			return nil, &ParseError{
				Path: path,
				Key:  "commands." + c.name,
				Msg:  emptyValueMsg("commands."+c.name, *c.cmd, "declare a command or remove the key"),
			}
		}
		inst.commands = append(inst.commands, Command{Name: c.name, Cmd: *c.cmd})
	}

	// Sorted names make the first error deterministic as well as fixing the
	// canonical accessor order.
	for _, name := range slices.Sorted(maps.Keys(doc.Operations)) {
		if strings.TrimSpace(name) == "" {
			// A whitespace-only name is echoed back quoted: %q turns the
			// invisible character the author is staring past into a
			// visible one.
			msg := "operations declares an operation with an empty name: name the ritual or remove it"
			if name != "" {
				msg = fmt.Sprintf("operations declares an operation whose name %q is only whitespace: name the ritual or remove it", name)
			}
			return nil, &ParseError{
				Path: path,
				Key:  "operations",
				Msg:  msg,
			}
		}
		// A padded name looks identical to its trimmed form everywhere it
		// is displayed and matches it nowhere it is compared — a verifier
		// selecting reset-db could never reach " reset-db".
		if name != strings.TrimSpace(name) {
			return nil, &ParseError{
				Path: path,
				Key:  "operations",
				Msg:  fmt.Sprintf("operations declares an operation whose name %q is padded with whitespace: remove the padding", name),
			}
		}
		op := doc.Operations[name]
		if op.Command == nil {
			return nil, &ParseError{
				Path: path,
				Key:  "operations." + name,
				Msg:  fmt.Sprintf("operations.%s has no command: declare one or remove the operation", name),
			}
		}
		if strings.TrimSpace(*op.Command) == "" {
			return nil, &ParseError{
				Path: path,
				Key:  "operations." + name + ".command",
				Msg:  emptyValueMsg("operations."+name+".command", *op.Command, "declare a command or remove the operation"),
			}
		}
		inst.operations = append(inst.operations, Operation{Name: name, Cmd: *op.Command, Destructive: op.Destructive})
	}

	for n, p := range doc.Corpus.Exemplary {
		// An empty entry has no value to quote, so its 1-based position
		// says which one; the other rejections identify themselves by
		// echoing the offending value.
		if strings.TrimSpace(p) == "" {
			return nil, &ParseError{
				Path: path,
				Key:  "corpus.exemplary",
				Msg:  emptyValueMsg(fmt.Sprintf("corpus.exemplary entry %d", n+1), p, "point at a path or remove it"),
			}
		}
		if p != strings.TrimSpace(p) {
			return nil, &ParseError{
				Path: path,
				Key:  "corpus.exemplary",
				Msg:  fmt.Sprintf("corpus.exemplary entry %q is padded with whitespace: remove the padding", p),
			}
		}
		if reason := corpusPathReason(p); reason != "" {
			return nil, &ParseError{
				Path: path,
				Key:  "corpus.exemplary",
				Msg:  fmt.Sprintf("corpus.exemplary entry %q %s", p, reason),
			}
		}
	}
	for n, u := range doc.Corpus.ExemplarPRs {
		if strings.TrimSpace(u) == "" {
			return nil, &ParseError{
				Path: path,
				Key:  "corpus.exemplar-prs",
				Msg:  emptyValueMsg(fmt.Sprintf("corpus.exemplar-prs entry %d", n+1), u, "point at a pull request or remove it"),
			}
		}
		if u != strings.TrimSpace(u) {
			return nil, &ParseError{
				Path: path,
				Key:  "corpus.exemplar-prs",
				Msg:  fmt.Sprintf("corpus.exemplar-prs entry %q is padded with whitespace: remove the padding", u),
			}
		}
		// url.Parse alone is not the check: it accepts a bare repository
		// path as a relative URL. The spec pins absolute, http(s), and a
		// nonempty host — and nothing ever fetches one.
		parsed, err := url.Parse(u)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, &ParseError{
				Path: path,
				Key:  "corpus.exemplar-prs",
				Msg:  fmt.Sprintf("corpus.exemplar-prs entry %q is not an absolute http(s) URL: write the pull request's full address", u),
			}
		}
	}
	if dor := doc.Corpus.DefinitionOfReady; dor != nil {
		if strings.TrimSpace(*dor) == "" {
			return nil, &ParseError{
				Path: path,
				Key:  "corpus.definition-of-ready",
				Msg:  emptyValueMsg("corpus.definition-of-ready", *dor, "point at a document or remove the key"),
			}
		}
		if *dor != strings.TrimSpace(*dor) {
			return nil, &ParseError{
				Path: path,
				Key:  "corpus.definition-of-ready",
				Msg:  fmt.Sprintf("corpus.definition-of-ready %q is padded with whitespace: remove the padding", *dor),
			}
		}
		if reason := corpusPathReason(*dor); reason != "" {
			return nil, &ParseError{
				Path: path,
				Key:  "corpus.definition-of-ready",
				Msg:  fmt.Sprintf("corpus.definition-of-ready %q %s", *dor, reason),
			}
		}
		inst.corpus.DefinitionOfReady = *dor
	}
	// An explicitly empty list declares nothing, the same as an absent key;
	// normalizing to nil gives consumers one "nothing declared" state
	// instead of two.
	if len(doc.Corpus.Exemplary) > 0 {
		inst.corpus.Exemplary = doc.Corpus.Exemplary
	}
	if len(doc.Corpus.ExemplarPRs) > 0 {
		inst.corpus.ExemplarPRs = doc.Corpus.ExemplarPRs
	}

	return inst, nil
}

// emptyValueMsg words an empty value apart from a whitespace-only one,
// echoing the latter %q-quoted: "empty" would gaslight an author staring
// at characters that do not show.
func emptyValueMsg(what, value, remedy string) string {
	if value == "" {
		return fmt.Sprintf("%s is empty: %s", what, remedy)
	}
	return fmt.Sprintf("%s %q is only whitespace: %s", what, value, remedy)
}

// corpusPathReason says why p cannot be a repository-relative pointer, or ""
// when it can. Corpus paths are slash-separated and repository-relative by
// definition, so this is the pure path package, not filepath: OS semantics
// never enter into it. Escape detection cleans first, so an interior ..
// that climbs out of the root is caught wherever it sits.
func corpusPathReason(p string) string {
	if slashpath.IsAbs(p) {
		return "is absolute: corpus paths are repository-relative"
	}
	if c := slashpath.Clean(p); c == ".." || strings.HasPrefix(c, "../") {
		return "escapes the repository: corpus paths stay inside the root"
	}
	return ""
}

// parseError maps the decoder's failures onto ParseError. Unknown keys arrive
// as a StrictMissingError holding one DecodeError per key; each becomes its
// own ParseError so nothing is collapsed, joined so callers report them all.
func parseError(path string, err error) error {
	var strict *toml.StrictMissingError
	if errors.As(err, &strict) {
		errs := make([]error, 0, len(strict.Errors))
		for i := range strict.Errors {
			de := &strict.Errors[i]
			row, col := de.Position()
			key := strings.Join(de.Key(), ".")
			errs = append(errs, &ParseError{
				Path:   path,
				Line:   row,
				Column: col,
				Key:    key,
				Msg:    fmt.Sprintf("unknown key %q", key),
				detail: de.String(),
			})
		}
		return errors.Join(errs...)
	}

	var de *toml.DecodeError
	if errors.As(err, &de) {
		row, col := de.Position()
		return &ParseError{
			Path:   path,
			Line:   row,
			Column: col,
			Key:    strings.Join(de.Key(), "."),
			Msg:    de.Error(),
			detail: de.String(),
		}
	}

	return fmt.Errorf("parsing %s: %w", path, err)
}
