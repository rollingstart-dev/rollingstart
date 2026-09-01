// Command rolling is the Rolling Start CLI: a teaching harness that turns a
// repository into an adaptive tutor.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/rollingstart-dev/rollingstart/internal/version"
)

// errSilentExit signals "exit non-zero without printing me on top of whatever
// the command already wrote to stdout." Commands like `rolling doctor` render
// their own summary and use this to avoid a duplicate "Error: ..." line.
var errSilentExit = errors.New("silent exit")

// errInterrupted signals that the user stopped a command before it
// finished: the command has already said "interrupted", nothing is printed
// on top, and the process exits 130 — the shell convention for SIGINT.
var errInterrupted = errors.New("interrupted")

// errUsage marks an error as the caller's, not the environment's: a typo'd
// flag or argument exits 2, so a script gating on doctor can tell "you
// invoked me wrong" apart from "a harness precondition failed" (exit 1).
var errUsage = errors.New("usage error")

// usageError wraps an error so errors.Is(err, errUsage) holds while the
// original message prints unchanged.
type usageError struct{ error }

func (usageError) Is(target error) bool { return target == errUsage }

var rootCmd = &cobra.Command{
	Use:   "rolling",
	Short: "Rolling Start — turn a repository into an adaptive tutor",
	Long: `Rolling Start turns a repository into an adaptive tutor.

An instance author defines the destination — what competence in this codebase
looks like. Rolling Start finds each learner's route there: one task at a time,
evaluated on the diff when the learner says they're done.

You are already a skilled engineer. This gets you up to speed in *this* codebase.

See https://rollingstart.dev for documentation.`,
	Version:       version.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().Bool("verbose", false, "verbose output")
	rootCmd.PersistentFlags().Bool("no-tty", false, "force plain output, no TUI")
	// Flag-parse failures anywhere in the tree, and an unknown subcommand,
	// are usage errors. Replicating cobra's own unknown-command wording
	// keeps the message familiar while the exit code becomes honest.
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError{err}
	})
	// The root runs so that an unknown subcommand reaches it as an
	// argument: a bare non-runnable root short-circuits to help before
	// argument validation, and "rolling frobnicate" would print help and
	// exit 0 — a typo reading as success.
	rootCmd.Args = cobra.ArbitraryArgs
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return usageError{fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())}
		}
		return cmd.Help()
	}
}

func main() {
	// SIGINT and SIGTERM cancel the command's context rather than kill
	// the process: a running instance command lives in its own process
	// group, so a bare Ctrl-C would orphan it — cancellation lets the
	// runner shut the whole group down first.
	err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithVersion(version.Version),
		fang.WithErrorHandler(errorHandler),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	)
	switch {
	case err == nil:
	case errors.Is(err, errInterrupted):
		os.Exit(130)
	case errors.Is(err, errUsage):
		os.Exit(2)
	default:
		os.Exit(1)
	}
}

// errorHandler wraps fang's styled error rendering with the errSilentExit
// shortcut: commands that print their own summary return errSilentExit so we
// exit non-zero without stacking a styled error block on top of their output.
func errorHandler(w io.Writer, styles fang.Styles, err error) {
	if errors.Is(err, errSilentExit) || errors.Is(err, errInterrupted) {
		return
	}
	fang.DefaultErrorHandler(w, styles, err)
}
