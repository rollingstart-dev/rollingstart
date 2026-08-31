# `rolling doctor`

The readiness check — the formation lap before anything depends on the car.
Doctor answers two questions that look alike and are not
([`docs/roadmap.md`](../roadmap.md) § 2.6): can the harness operate here at
all, and does the codebase's own toolchain consider the working copy healthy.
It reports; it never fixes.

## Usage

```
rolling doctor [dir] [--timeout <duration>] [--verbose]
```

`dir` defaults to the current directory. Doctor resolves the repository root
from there (`git rev-parse --show-toplevel`) and runs everything against the
root: the instance definition lives there, and it is the directory the coach
will watch. When there is no repository, the root is `dir` itself and the
first row of the report says so.

`--timeout` bounds each declared command; the default is `5m`. A command that
exceeds it is shut down along with everything it spawned — SIGTERM first, then
SIGKILL to the whole process group.

`--verbose` prints every command's captured output, not only the failing
ones', and prints all of it rather than the last twenty lines.

The root `--no-tty` flag has no effect on doctor: its output is already plain
text.

## The report

Two sections, plain text, no color — readable in any terminal and in a CI log.
Harness rows use `ok` and `FAIL`; instance rows use the words that name what
actually happened. The vocabularies differ on purpose: `FAIL` blocks,
`failing` informs.

### Harness preconditions

Five probes, in this order, each run only after the previous one returned —
the file watcher writes a synthetic file into the checkout and must not
overlap the working-tree probe.

```
Harness preconditions
  ok    git repository       inside a git work tree
  ok    working tree         working tree is clean
  ok    line endings         core.autocrlf is unset
  ok    instance definition  instance definition loaded (3 commands declared)
  ok    file watcher         file events are delivered
```

Each row is the probe's finding, verbatim. A failed precondition reads `FAIL`
and its finding says what was observed and, where the harness knows, what to
do about it:

```
Harness preconditions
  ok    git repository       inside a git work tree
  FAIL  working tree         working tree is not clean: 2 path(s) modified, staged, or untracked — commit, stash, or remove them (see git status)
  ok    line endings         core.autocrlf is unset
  FAIL  instance definition  .rollingstart/instance.toml:2:1: unknown key "biuld"
  ok    file watcher         file events are delivered
```

Any `FAIL` in this section is blocking: nothing can proceed and no lesson can
be served. Doctor still runs the second section when it can, because a learner
with a dirty tree also wants to know whether `pnpm` is installed.

### Instance command health

One row per declared command, in canonical order — build, typecheck, test,
lint — each run through the runner from the repository root, sequentially,
with the environment `rolling` was launched from. Undeclared commands are not
listed.

```
Instance command health
  healthy        build      go build ./...        1.2s
  failing        test       go test ./...         exit 1 after 4.3s
    --- FAIL: TestLoad (0.00s)
        instance_test.go:31: Load: open x: no such file or directory
    FAIL
    FAIL	github.com/rollingstart-dev/rollingstart/internal/instance	0.012s
  not installed  lint       pnpm lint             exit 127 after 0.0s
    sh: line 1: pnpm: command not found
```

The four outcomes are the runner's, named accurately because the milestone's
exit criterion depends on it: **healthy** (exited zero), **failing** (ran and
exited nonzero, or was killed by a signal the harness did not send), **not
installed** (exit 127 or 126 — the binary is missing or not executable, the
"pnpm is not installed" case), **timed out** (exceeded `--timeout`). Anything
but `healthy` prints the last twenty lines of the command's combined output
beneath the row, indented; the end of the output is where toolchains put the
verdict.

```
  timed out      test       pnpm test             5m0s, process group terminated
```

Red here is informational. A codebase instance's first lesson is frequently
"get this running for local dev", and the harness cannot tell a broken
environment from a learner who has not done that lesson yet — so it does not
pretend to.

Two states this section can be in that are neither healthy nor red:

```
Instance command health
  skipped: .rollingstart/instance.toml:2:1: unknown key "biuld"
```

When the definition did not load, the section reports itself skipped with the
loader's reason — the same one the `instance definition` row shows — rather
than pretending the instance is healthy or broken.

```
Instance command health
  nothing declared: .rollingstart/instance.toml has no [commands]
```

A definition that declares no commands is valid, and this is what it looks
like: an explicit state, not an empty section that reads as green.

## Exit status

| Code | Meaning |
|---|---|
| `0` | Every harness precondition holds. Instance rows may say anything. |
| `1` | At least one harness precondition failed. |
| `2` | Usage error — an unknown flag, more than one argument. |
| `130` | Interrupted before the report finished. |

Instance-section red alone exits zero: it is a learnable state, not breakage,
and the local-dev-setup lesson's completion condition is this section going
green. Scripts and CI can still gate on harness readiness.

## Interruption

Ctrl-C stops doctor between steps. The command that was running, and
everything it spawned, is shut down the way a timeout shuts it down. The
report ends with a single line —

```
interrupted
```

— and nothing that did not run is rendered, so an interrupted run is never
mistaken for a broken environment.

## What doctor never does

It never changes git configuration, installs a toolchain, starts a service, or
writes into the repository — the file watcher's synthetic file is removed on
every path, and its distinctive name (`.rollingstart-probe-*`) means residue
from a crash names itself in `git status` rather than masquerading as your
work. If a declared command fails because a service is down, that is reported,
not fixed: Rolling Start runs where your dev environment already runs.

## Decisions recorded here

Three questions the probe layer deliberately left to rendering, settled by
this document:

- **Probe names are prose.** `git repository`, `working tree`, `line endings`,
  `instance definition`, `file watcher` — one register in the column. The
  config key `core.autocrlf` still appears in the finding, where it is the
  thing to fix.
- **A cancelled context is an interruption, not a finding.** Doctor checks its
  context between steps and stops; it does not render the probes' or the
  runner's cancellation wording as a red row.
- **Parse errors render as one positioned line.** The loader's source excerpts
  (`ParseError.Detail()`) are not shown. `file:line:col: message` is precise
  enough to fix a typo in seconds; if instance authors ask for the excerpt,
  that is a rendering change against the `ParseError` type and nothing else.
