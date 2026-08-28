# Review guidelines

What reviewers check, human or agent. Read this before opening a PR — knowing
the criteria up front is cheaper than learning them in review.

## Correctness first

- Does it do what the issue said? Check the acceptance criteria explicitly.
- Are the failure paths handled, or only the happy one? This tool runs other
  people's commands in environments it does not control; failure is the common
  case, not the edge.
- Are errors actionable? "command failed" tells a learner nothing. Name what
  ran, what it exited with, and what they can do.

## Scope

- One issue, one PR. A diff spanning config parsing, the runner, and the CLI at
  once should have been a stack.
- Did in-flight additions pass the three tests in
  [`docs/workflow.md`](docs/workflow.md)? If not, they belong in a follow-up.

## The architectural seams

These are the ones worth blocking a PR over, because they are expensive to
recover once crossed:

- **The engine does not parse target languages.** Ecosystem knowledge belongs in
  instance config and output parsers.
- **The harness does not orchestrate environments.** No docker, no service
  lifecycle, no toolchain installation.
- **Generated content is data, not shell.**
- **Coach observations never become grader verdicts.**

## Go specifics

- Errors wrapped with context (`fmt.Errorf("...: %w", err)`), not swallowed or
  logged-and-returned.
- `context.Context` threaded through anything that spawns a process or makes a
  network call, and actually honoured.
- No goroutine without a clear lifetime and a way to stop.
- Exported identifiers have doc comments. Unexported ones have comments where
  the *why* isn't obvious from the name.
- Table-driven tests where there is more than one case.

## Tests

- Does the test fail without the change? If not, it isn't testing the change.
- Are the assertions about behaviour, or about implementation detail that will
  break on the next refactor?
- Subprocess and filesystem behaviour needs real coverage — that is this tool's
  actual job, not an incidental detail.

## Documentation

- User-visible behaviour changed → the docs describing it changed in the same
  PR.
- A decision meeting the ADR threshold got an ADR.
- Comments explain why, not what. Match the density of the surrounding code.

## Commit history

- Bodies explain the reasoning, not just the change. This repository is meant to
  become an instance of its own tool.
- No `wip`, no `fix typo` chains that should have been squashed before pushing.
