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
Never GitHub's native stacked PRs. Direct commits to `main` are the
exception, not a lane: for a change where a PR would be needless ceremony —
a plan status flip at closeout, a one-line guidance fix — and only on the
maintainer's say-so, asked each time. Anything that deserves a reviewer's
eyes goes through a PR attached to its issue, and a new spec, reference
page, or ADR always does: writing one is work product, not housekeeping.

**Every push that changes behavior gets a local code review first.** Spawn a
code-review sub-agent on the Opus model (`claude-opus-5`) over the outgoing
diff — reviewing against [`REVIEW.md`](REVIEW.md), the milestone plan in
[`docs/plans/`](docs/plans/), and the relevant ADRs — and fix the real
findings before pushing. The gate is the push, not finishing the work: a
review-round fix is a push and gets its own review, however small the fix
felt; and a push the maintainer defers ("don't push yet") runs its review
when the push happens, not when the code was written. The reviewer's brief
confines any state-changing experiment — a poisoned git environment, hooks,
test runs under unusual env — to a scratch copy of the repository, never
the live checkout, and the tree is verified (status, log, local config)
when a review returns: on #19 a reviewer proving a real finding committed a
52-file deletion onto the live branch with a GIT_DIR experiment. Exempt
only pushes
that change no behavior at all — comment wording, test renames, doc prose —
because a rule that fires on a one-word comment fix is a rule that gets
skipped when it matters.

**Code review lives on three surfaces — triage all of them.** Inline per-file
comments (`gh api repos/rollingstart-dev/rollingstart/pulls/<N>/comments`),
review summary bodies (`gh pr view <N> --json reviews`), and the PR-level
conversation (`gh api repos/rollingstart-dev/rollingstart/issues/<N>/comments`).
The review bot posts whole-PR findings as conversation comments, so an empty
inline list plus green checks is not review-clean — a substantive finding may
be sitting in the conversation tab. Check all three before declaring a review
triaged or replying "no comments".

**Answer review findings where they were raised.** Reply to each inline
comment in its own thread
(`gh api repos/rollingstart-dev/rollingstart/pulls/<N>/comments -X POST -f body='…' -F in_reply_to=<id>`)
with the disposition and the fixing commit; reserve PR-level comments for
whole-PR feedback. A PR-level digest leaves every inline thread visibly
unanswered, and whoever merges must then reconstruct which threads are
addressed instead of reading each thread's reply.

**Write commit bodies you'd want a new hire to learn from.** This repo is meant
to become an instance of its own tool, so its history is teaching material.

**Stop on surprises.** An unexpected incompatibility or an uncovered design
question means stop and ask, not guess and continue.

## Conventions

- Go, standard layout, `cmd/` + `internal/`. Cobra with Fang.
- Test file beside each source file. A separate `e2e` package for end-to-end.
- `gofmt`, `go vet`, `go test ./...` all clean before any push — each
  checked by its own exit status, never through a pipe that masks it. A
  `go test | grep` let a red branch reach the remote on #18.
- Errors that a command already rendered use `errSilentExit` so Fang doesn't
  stack a styled block on top of the command's own output.
- Docs carry a light motorsport theme in prose. Never in identifiers —
  `rolling doctor`, not `rolling pitstop`.

## Workflow

[`docs/workflow.md`](docs/workflow.md) is the reference. The cycle is
`/plan-milestone` → `/refine-issue` → `/implement-issue` → `/milestone-endgame`.
