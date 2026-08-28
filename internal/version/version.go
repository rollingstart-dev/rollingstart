// Package version carries build metadata stamped in at link time.
package version

// Version is the release version. Overridden via -ldflags at build time;
// "dev" when built from a working tree.
var Version = "dev"

// Commit is the git SHA the binary was built from, when known.
var Commit = "unknown"
