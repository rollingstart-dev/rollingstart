# Rolling Start — agent instructions

Read this before doing anything in this repository.

## What this is

A teaching harness: a deterministic Go runtime wrapped around an LLM coding
agent that turns a repository into an adaptive tutor. An instance author defines
the destination; the harness finds each learner's route there and evaluates real
work on a diff.

Read [`docs/design.md`](docs/design.md) for the principles and
[`docs/roadmap.md`](docs/roadmap.md) for the decision record and architecture
before proposing anything structural.

## Status

- [x] **M0 — Skeleton.** Go module, CLI surface, CI. `rolling version` runs.
- [ ] **M1 — `rolling doctor`.** Instance config, command runner, environment probes.
- [ ] M2 — Formats, hand-authored
- [ ] M3 — The loop, deterministic
- [ ] M4 — Judge
- [ ] M5 — Coach
- [ ] M6 — Explorer
- [ ] M7 — Real targets
- [ ] M8 — Release

## Key locations

| Path | What |
|---|---|
| `cmd/rolling/` | CLI surface. One file per command. |
| `internal/` | The engine. Never parses a target language. |
| `docs/design.md` | What this is, and why |
| `docs/roadmap.md` | Decision record, architecture, milestones, risks |
| `docs/landscape.md` | Competitors, and what we take from them |
| `docs/workflow.md` | How work happens — the skills point here |
| `docs/plans/` | Expanded plan per milestone, plus retrospectives |
| `docs/decisions/` | ADRs for decisions made after the roadmap |
| `.claude/skills/` | `/plan-milestone`, `/refine-issue`, `/implement-issue`, `/milestone-endgame` |

## Rules

**Never commit or push without explicit approval.** Not once, not for a
one-liner.

**The engine is language-agnostic.** It reads git, spawns commands the instance
declares, diffs a working copy, keeps state, and calls a model. If you find
yourself parsing TypeScript, stop — that belongs in an instance adapter, and
probably not at all.

**Rolling Start never orchestrates the environment.** No docker, no compose, no
health checks, no port management, no installing toolchains. It runs where the
learner's dev environment already runs. If a declared command fails because a
service is down, that is reported, not fixed.

**Generated verifiers are structured data, never shell.** Generation emits test
files and command *selections* the harness interprets. Instance-declared
operations are shell, but they are human-authored and code-reviewed — different
provenance, different trust.

**Destructive operations prompt.** Always. Dropping a learner's local database
uninvited is the same class of damage as stomping their uncommitted work.

**The coach never grades.** The grader runs in fresh context on the diff alone.
Coach observations feed the profile; only grader verdicts move the skill graph.
Keep that seam intact — it is the whole non-sycophancy argument.

**One issue → one branch → one PR.** For large scopes, base-chained stacks.
Never GitHub's native stacked PRs.

**Write commit bodies you'd want a new hire to learn from.** This repo is meant
to become an instance of its own tool, so its history is teaching material.

**Stop on surprises.** An unexpected incompatibility or an uncovered design
question means stop and ask, not guess and continue.

## Conventions

- Go, standard layout, `cmd/` + `internal/`. Cobra with Fang.
- Test file beside each source file. A separate `e2e` package for end-to-end.
- `gofmt`, `go vet`, `go test ./...` all clean before any push.
- Errors that a command already rendered use `errSilentExit` so Fang doesn't
  stack a styled block on top of the command's own output.
- Docs carry a light motorsport theme in prose. Never in identifiers —
  `rolling doctor`, not `rolling pitstop`.

## Workflow

[`docs/workflow.md`](docs/workflow.md) is the reference. The cycle is
`/plan-milestone` → `/refine-issue` → `/implement-issue` → `/milestone-endgame`.
