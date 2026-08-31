// Package e2e tests the rolling binary the way a learner meets it: built
// once per test run with the same go build they would use, then executed
// against fixture repositories with its stdout, stderr, and exit status
// read back separately. It imports nothing from internal/ — it tests the
// binary's contract, not the packages' — so a test here fails only when a
// learner would see the difference.
//
// TestMain builds the binary into a temporary directory and removes it after
// a normal run; a test that panics crashes the process first and leaves it
// under $TMPDIR/rollingstart-e2e-*. The build is not race-instrumented even
// under go test -race — the detector covers the harness, not the CLI — and
// the package is Unix-only, deliberately: so is the product (the runner's
// process-group contract), and the binary is built without an .exe suffix.
//
// Before any test runs, the host's git configuration and every inherited
// GIT_* variable are removed from the process, so a developer's own
// core.autocrlf cannot sway a run and a fixture — a temporary git
// repository with an identity configured — can never reach the repository
// the tests are running in. The binary inherits that environment because it
// is a child of the test process.
package e2e
