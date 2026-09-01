# `instance.toml`

The instance definition file — what an instance author writes to tell Rolling
Start about a codebase. It lives at `.rollingstart/instance.toml`, at the
repository root, alongside everything else in the storage layout described in
[`docs/roadmap.md`](../roadmap.md) § 2.5.

This documents schema **v1**: the commands of v0 (M1), plus the operations
and corpus pointers that arrive with the formats milestone (M2). Growth is
additive — every valid v0 file is a valid v1 file. The rest of the instance
definition — the skill graph in `skills/`, the task pool in `tasks/` — lives
in its own files with its own reference pages, though where the graph's
destination set is declared is that milestone's decision and may yet touch
this file.

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

## `[operations]`

The pit crew's checklist: named lifecycle rituals — reset the database to the
dev seed, re-run the seeder — that a codebase's own contributors know by
muscle memory and nobody ever writes down. Declaring them makes them
teachable: a precondition for a task, a step inside one, or the way out when
local state is wedged.

```toml
[operations]
reset-db = { command = "pnpm db:reset --force", destructive = true }
seed-db  = { command = "pnpm db:seed" }
```

Each operation is a table under its own name — the inline form above and the
`[operations.reset-db]` header form are the same TOML document; use whichever
reads better in context. Two keys are recognized per operation:

- **`command`** (required) — a shell command, exactly like the entries in
  `[commands]`: run via `sh -c` from the repository root with the inherited
  environment, written by a human, reviewed like code.
- **`destructive`** (optional, default `false`) — marks a ritual that
  discards state: dropping a database, wiping local data. The harness will
  always prompt before running one, uninvited data loss being the fastest way
  to lose a learner's trust.

Operation names are ordinary TOML keys; kebab-case reads best. Name the
ritual, not the script — `reset-db`, not `prisma-migrate-reset` — because the
name is what a learner is offered.

Write commands that run **without interaction**. The `--force` above disarms
the tool's own confirmation prompt on purpose: prompting is the harness's
job, and it prompts once, up front, for the whole operation. A tool prompt
inside a declared command is invisible and hangs the run.

Operations are rituals performed *inside* an environment that is already
running — resetting data, re-seeding, regenerating artifacts. Bringing
services up or down is not one: Rolling Start never orchestrates the
environment, and a learner's stack is theirs to start. Whether a stack-up
script may ever be declared is deliberately unsettled until M3, when
something first executes an operation.

An operation that omits `command`, or declares it empty or whitespace-only,
is an error naming the offending operation — and the missing key and the
empty value are different mistakes, worded apart. An empty or
whitespace-only operation *name* is an error for the same reason an empty
command is: it declares nothing. Duplicate names need no rule of ours —
TOML itself rejects a redefined key, with a position. Like the value checks
in `[commands]`, these run after decoding, so they name the file and the
offending key rather than a line and column; only the decoder's own
failures — unknown keys, type mismatches, broken syntax — carry
`file:line:col`.

Commands and operations live in separate namespaces: an operation may be
called `build` without colliding with `[commands].build`, because anything
that later selects one — a task's verifier, in the tasks format — says
which kind it is selecting.

In this milestone operations are declared and validated only: `rolling
doctor` counts them and nothing runs them — execution arrives with the
session loop (M3), which honors `destructive` by prompting.

## `[corpus]`

Pointers to the material the author holds up as ground truth: the code worth
imitating, the pull requests that show how work is done here, the definition
of ready. Later milestones read the corpus — judgment grounds its verdicts in
it (M4), generation mines it (M6). Declaring it is part of authoring an
instance, so the schema and its validation arrive with the formats milestone
even though nothing consumes the content yet.

```toml
[corpus]
exemplary           = ["apps/web/src/features/poll", "packages/database/prisma/schema.prisma"]
exemplar-prs        = ["https://github.com/lukevella/rallly/pull/1502"]
definition-of-ready = ".rollingstart/ready.md"
```

- **`exemplary`** — repository-relative paths, files or directories, naming
  the code a learner should imitate. Not everything; the good parts.
- **`exemplar-prs`** — full URLs of merged pull requests that show how a
  change becomes a contribution here. URLs rather than bare numbers because
  the engine assumes no forge: a GitHub number means nothing to a GitLab
  mirror.
- **`definition-of-ready`** — a repository-relative path to the document
  saying what "ready to merge" means in this codebase. If the repository has
  no such document, writing one at `.rollingstart/ready.md` is part of
  authoring the instance.

All keys are optional, and form is checked at load, naming the offending
key. An empty-string entry is an error — a pointer at nothing declares an
intention and then says nothing. A path must stay inside the repository:
absolute paths and paths that climb out through `..` are rejected. An
`exemplar-prs` entry must parse as an *absolute* URL with scheme `http` or
`https` and a nonempty host — stated that precisely because `net/url`
happily accepts a bare repository path as a relative URL. Nothing more is
checked: form only, no filesystem, no network.

One pointer the design names is deliberately absent from v1: which code is
*legacy* — the anti-exemplar half of "which code is exemplary and which is
legacy". It arrives when the judge (M4) exists to consume it; strict
parsing makes the addition purely additive.

Two kinds of wrong, told apart on purpose. A malformed file fails at load:
the decoder's own failures — an unknown key, a type mismatch, broken syntax
— carry `file:line:col`, and the value checks above name the file and the
offending key, the same shape as the empty-command rule in `[commands]`. A
well-formed pointer whose target is absent from *this* checkout is not a
load error: the file is fine, the working copy just doesn't match it.
`rolling doctor` reports that case as a note, never as a failure — see
[`rolling-doctor.md`](rolling-doctor.md). And nothing ever fetches a URL to
check one: validation performs no network I/O.

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
