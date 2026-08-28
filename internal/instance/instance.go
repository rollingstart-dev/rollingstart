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
	"os"
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

// Instance is a parsed and validated instance definition.
type Instance struct {
	commands []Command
}

// Commands returns the declared commands in canonical order: build,
// typecheck, test, lint. Undeclared commands are absent; a definition
// declaring none returns an empty slice, which is a valid state the caller
// is expected to surface rather than treat as healthy.
func (i *Instance) Commands() []Command {
	return i.commands
}

// ParseError is a definition that exists but cannot be used. Error() renders
// one line with the file position; Detail() carries a source excerpt when the
// underlying decoder produced one.
type ParseError struct {
	Path   string
	Line   int    // 1-based; 0 when the failure has no position
	Column int    // 1-based; 0 when the failure has no position
	Key    string // offending key path (dotted), when known
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

// document mirrors schema v0, documented in docs/reference/instance-toml.md.
// Strict decoding rejects everything outside it. Command fields are pointers
// so that a key declared with an empty value — always a mistake — is
// distinguishable from a key not declared at all.
type document struct {
	Commands struct {
		Build     *string `toml:"build"`
		Typecheck *string `toml:"typecheck"`
		Test      *string `toml:"test"`
		Lint      *string `toml:"lint"`
	} `toml:"commands"`
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
				Msg:  fmt.Sprintf("commands.%s is empty: declare a command or remove the key", c.name),
			}
		}
		inst.commands = append(inst.commands, Command{Name: c.name, Cmd: *c.cmd})
	}
	return inst, nil
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
