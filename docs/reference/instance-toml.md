# `instance.toml`

The instance definition file — what an instance author writes to tell Rolling
Start about a codebase. It lives at `.rollingstart/instance.toml`, at the
repository root, alongside everything else in the storage layout described in
[`docs/roadmap.md`](../roadmap.md) § 2.5.

This documents schema **v0**, which carries commands only. Operations, corpus
pointers, and the rest of the instance definition arrive with the formats
milestone (M2), and this document grows with them.

## `[commands]`

The scrutineering bay: the commands Rolling Start runs to find out whether the
codebase's own toolchain considers the working copy healthy.

```toml
# .rollingstart/instance.toml
[commands]
build     = "pnpm build"
typecheck = "pnpm typecheck"
test      = "pnpm test"
lint      = "pnpm lint"
```

A complete, commented example for a real target is in
[`examples/rallly/`](../../examples/rallly/).

Four keys are recognized: `build`, `typecheck`, `test`, and `lint`. Each is
optional. Each value is a shell command, written by a human and reviewed like
any other code in the repository.

Commands run via `sh -c`, from the repository root, with the environment
`rolling` was launched from — if `pnpm` is on your `PATH`, it is on the
harness's `PATH` too. Rolling Start never installs, wraps, or substitutes a
toolchain; it runs exactly what the instance declares, where the learner's
environment already runs.

A `[commands]` table with no entries — or a file with no `[commands]` table at
all — is valid. `rolling doctor` reports that state as "nothing declared"
rather than as a healthy instance.

An empty or whitespace-only command string is an error: it declares an
intention and then says nothing, which is always a mistake.

## Strict parsing

Unknown keys are errors, not warnings. A misspelled table name would otherwise
be ignored silently, and doctor would report an instance as undeclared when the
author believes it is configured — the worst kind of wrong, because nothing
looks broken.

Parse errors carry the file position and the offending key, and `rolling`
displays them verbatim. A typo should cost seconds, not a debugging session.

One consequence, deliberate: you cannot pre-declare sections from schema
versions the harness cannot read yet. The schema and the code that reads it
grow together.
