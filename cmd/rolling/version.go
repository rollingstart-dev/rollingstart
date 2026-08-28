package main

import (
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/rollingstart-dev/rollingstart/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and build information",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return writeVersion(cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// writeVersion renders build metadata. Split out from the command so tests can
// capture it without going through cobra's execution path.
func writeVersion(w io.Writer) error {
	_, err := fmt.Fprintf(w, "rolling %s\ncommit  %s\ngo      %s\nplatform %s/%s\n",
		version.Version, version.Commit,
		runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return err
}
