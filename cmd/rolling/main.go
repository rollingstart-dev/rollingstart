// Command rolling is the Rolling Start CLI: a teaching harness that turns a
// repository into an adaptive tutor.
package main

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/rollingstart-dev/rollingstart/internal/version"
)

// errSilentExit signals "exit non-zero without printing me on top of whatever
// the command already wrote to stdout." Commands like `rolling doctor` render
// their own summary and use this to avoid a duplicate "Error: ..." line.
var errSilentExit = errors.New("silent exit")

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
}

func main() {
	err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithVersion(version.Version),
		fang.WithErrorHandler(errorHandler),
	)
	if err != nil {
		os.Exit(1)
	}
}

// errorHandler wraps fang's styled error rendering with the errSilentExit
// shortcut: commands that print their own summary return errSilentExit so we
// exit non-zero without stacking a styled error block on top of their output.
func errorHandler(w io.Writer, styles fang.Styles, err error) {
	if errors.Is(err, errSilentExit) {
		return
	}
	fang.DefaultErrorHandler(w, styles, err)
}
