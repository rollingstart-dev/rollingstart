# Workflow

How work happens here. The Claude Code skills in [`.claude/skills/`](../.claude/skills/)
encode this; this document is the reference they point at.

Rolling Start is built primarily by AI agents with a human maintainer. The
workflow is deliberately identical for both — same branches, same PRs, same
review. It also runs entirely in public, on purpose: the process is part of what
this project is demonstrating.

## The cycle

```
/plan-milestone  →  /refine-issue  →  /implement-issue  →  /milestone-endgame
```

Run in that order. `/refine-issue` is the per-issue planning step and comes
before any code is written; `/implement-issue` runs only on a refined issue.

## Where things live

**Durable artifacts live in the repo.** They survive the work that produced them.

| Path | What |
|---|---|
| [`docs/design.md`](design.md) | What Rolling Start is and the principles behind it |
| [`docs/roadmap.md`](roadmap.md) | Decision record, architecture, milestones, risks |
| [`docs/landscape.md`](landscape.md) | What else exists and why this is different |
| [`docs/plans/`](plans/) | One expanded plan per milestone, plus its retrospective |
| [`docs/decisions/`](decisions/) | ADRs — decisions made after the roadmap was written |

**Working state lives in GitHub.** It's disposable and it moves.

- **Milestones** (`M1: rolling doctor`) carry membership. No due dates — this is
  a side project and invented dates rot in public.
- **Project boards**, one per milestone, are the working surface: status columns
  and sub-issue progress.
- **Issues** carry the work. **PRs** carry the change and the discussion.

## Branches

Trunk-based. Feature branches off `main`, PRs back to `main`, no long-lived
branches. Descriptive kebab-case names, optionally prefixed with the milestone
scope (`m1.2/command-runner`). No issue numbers in branch names — link with
`Closes #N` in the PR body.

A small, isolated change with no issue behind it — the milestone plan when
`/plan-milestone` commits it, a plan status flip at closeout, a guidance edit
— may land directly on `main` instead of through a PR. The agent asks which
landing the maintainer wants; it never assumes.

## Issues

### Ready criteria

An issue is ready to implement when it has testable acceptance criteria, its
documentation exists (see below), and its dependencies are resolved or named.

### Board membership is a precondition

Committing to implement an issue is committing to the milestone it belongs to,
and the board has to reflect that. `/implement-issue` verifies this mechanically
before branching.

Native sub-issues inherit board membership through their parent. Standalone
follow-ups filed mid-milestone need explicit membership, routed by their
milestone — never by whichever board feels current.

### Fold-in decisions go in issue bodies, never only in comments

When triage folds one issue's work into another, record it as a checklist item
in the *receiving* issue's body.

Both `/refine-issue` and `/implement-issue` read comments, so this isn't about
what gets read. It's about what gets **found**. A fold-in sitting in comment 34
of 40 is technically read and practically missed; a closed issue's comments are
invisible to everyone in practice; and whoever implements a sub-issue reads
*that* sub-issue, so a note on the parent's thread still isn't where the work
happens. The comment reads catch violations of this rule. They don't replace
it.

### In-flight scope additions

Implementation surfaces gaps the issue didn't anticipate. Absorb one into the
current PR only if all three hold:

1. **Small** — roughly one commit, no new dependencies, no extra docs
2. **Clearly related** — the issue wouldn't feel done without it
3. **Documented** — noted on the issue and in the PR description

Fail any one and it becomes a follow-up issue. When in doubt, file the
follow-up.

## Documentation-driven development

Documentation comes before implementation. Which documentation depends on the
work.

**User-facing behavior** — write or update the docs describing the behavior
first. That's the spec. Then a failing test that encodes it. Then implement until
it passes. Then refine the docs with what you learned.

**Architecture and infrastructure** — write the ADR
([`docs/decisions/`](decisions/)) before implementing. Then implement. Then
update the ADR with what implementation taught you.

**Write an ADR only when all three hold:** it affects more than the file you're
editing, you'd have to re-derive it if you forgot, and a reasonable person could
have chosen differently. Everything else belongs in a commit body.

## Pull requests

One issue → one branch → one PR. Focused PRs are a hard requirement; a PR
spanning config, runner, and CLI at once cannot be reviewed properly.

If a single PR would be too large — the diff crosses layers, or runs past a few
hundred reviewable lines — build a **base-chained stack**: slice by dependency
layer, branch each slice off the one beneath it, open with
`--base <branch-below>`, and drive each slice's review to settlement before
writing the next. Upper layers get written against reviewed contracts.

**Never use GitHub's native stacked PRs.** Base chaining is how this repo
stacks.

PR bodies carry `Closes #N`, a summary of what and why, and a test plan.

Every push that changes behavior gets a local code review first — a
sub-agent on the Opus model over the outgoing diff, against
[`REVIEW.md`](../REVIEW.md), the plan, and the ADRs — and the review's real
findings are fixed before the push. The rule and its calibration are in
[`CLAUDE.md`](../CLAUDE.md) § Rules.

### CI

`go build ./...`, `go vet ./...`, `gofmt -l .` (any file it lists fails the
run, and the listing is the failure message), and `go test ./...` on Linux and
macOS, plus `go mod tidy -diff`. All must pass before merge. The repository's
own [`.rollingstart/instance.toml`](../.rollingstart/instance.toml) declares
the same build, test, and lint checks, so a run of the instance's commands
covers everything CI does except `go mod tidy -diff` and the macOS leg.

### Merging

Squash merge to `main`. Delete the branch after.

## Commit messages

Write commit bodies you'd want a new hire to learn from — explain why, not just
what. This isn't only style: Rolling Start generates exercises from git history,
and this repository is meant to become its own instance. A `wip` commit is a
lesson we can't teach.
