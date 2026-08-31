---
name: implement-issue
description: Implement a refined GitHub issue following the docs-driven sequence — docs, failing test, implement, verify, refine docs, open PR.
---

Implement a refined issue, following the docs-driven sequence in
[`docs/workflow.md`](../../../docs/workflow.md).

## Inputs

An issue number or URL, and optionally a branch name. If no branch is given,
suggest one from the issue title in kebab-case, prefixed with the milestone
scope (`m1.2/command-runner`).

**If no issue number was given**, suggest one rather than asking cold. The
likely answer is the next open sub-issue under whichever tracking issue is In
Progress on the active milestone board:

```sh
gh project list --owner rollingstart-dev
gh project item-list <project-number> --owner rollingstart-dev --limit 100 --format json \
  --jq '.items[] | select(.content.type == "Issue") | select(.status == "In Progress" or .status == "Todo") | "\(.status)\t#\(.content.number)\t\(.content.title)"'
```

Read the candidate's body to confirm it has no undeclared dependency on a
sibling, then propose it.

## Process

### 1. Read the issue

```sh
gh issue view {number} --comments
gh api repos/{owner}/{repo}/issues/{number}/sub_issues -q '.[] | "#\(.number) [\(.state)] \(.title)"'
```

`--comments` is load-bearing — refinement decisions and scope notes often land
as comments. Also read the **parent tracking issue's** comments; fold-ins are
supposed to live in bodies, but this is the backstop for the ones that don't.

**If the issue has linked sub-issues it is a tracking issue.** Do not implement
it in one branch. Take the first open sub-issue that has its dependencies
satisfied, and confirm the choice with the user before branching.

### 2. Read context

- [`CLAUDE.md`](../../../CLAUDE.md) — conventions and the architectural seams
- [`REVIEW.md`](../../../REVIEW.md) — know what review will check before starting
- The milestone plan in [`docs/plans/`](../../../docs/plans/)
- Relevant ADRs in [`docs/decisions/`](../../../docs/decisions/)

### 3. Verify board membership — a precondition, not a formality

Committing to implement an issue is committing to its milestone, and the board
must show it. This check is mechanical and runs regardless of how the decision
to implement was reached.

```sh
gh issue view {number} --json projectItems,milestone
```

- **`projectItems` non-empty** — already on a board, nothing to do.
- **Empty** — route by the issue's `M{N}: …` milestone. List boards including
  closed ones (`gh project list --owner rollingstart-dev --closed`) and match the
  milestone number to the board titled `… M{N} …`.
  - Board open → `gh project item-add <num> --owner rollingstart-dev --url <issue-url>`
  - Board closed because the milestone shipped → this is tail follow-up work. The
    milestone alone is sufficient. **Proceed and say so; don't stop to ask.**
- **Stop and ask only when the milestone is genuinely ambiguous** — no milestone
  and no way to infer one, or a closed board that signals future-milestone work
  pulled forward.

### 4. Branch — and decide the PR shape first

```sh
git checkout main && git pull origin main && git checkout -b {branch}
```

**If one PR would be too large to review** — the diff crosses layers, or runs
past a few hundred reviewable lines — plan a **base-chained stack** instead:

- Slice by dependency layer, each independently reviewable
- Build **serially**: open the first slice's PR and drive its review to
  settlement before writing the next, so upper layers are written against
  reviewed contracts
- Branch each slice off the one beneath it and open with `--base <branch-below>`.
  GitHub retargets to `main` automatically as bases merge — the only retargeting
  a stack needs
- Hand the whole stack over when every slice has settled; merge bottom-up

**Never use GitHub's native stacked PRs.** Do not run `gh stack init/add/submit`.
Native stacks cannot be admin-merged and render poorly for reviewers on mobile.
Base chaining keeps `gh pr merge` available and is how this repo stacks.

### 5. Docs-driven implementation

**For user-visible behaviour:**

1. **Docs** — write or update the documentation describing the behaviour. That
   is the spec. Commit: `docs: describe {behaviour}`
2. **Failing test** — encode the expected behaviour. It must fail now. Commit:
   `test: add failing test for {behaviour}`
3. **Implement** — build until it passes, following existing patterns. Logical
   commits with real bodies.
4. **Verify** — `gofmt -l .`, `go vet ./...`, `go test ./...`, `go build ./...`
5. **Refine docs** — update with what implementation taught you

**For architecture and infrastructure:** the ADR (from `/refine-issue` or written
now) replaces step 1, and step 5 updates it with what was learned.

### 6. Pre-push review, then open the PR

Before any push that changes behavior, run the local review from
[`CLAUDE.md`](../../../CLAUDE.md) § Rules: spawn a code-review sub-agent on
the Opus model (`claude-opus-5`) over the outgoing diff — `git diff main`,
including anything uncommitted that will ship — reviewing against
[`REVIEW.md`](../../../REVIEW.md), the milestone plan, and the relevant ADRs,
and have it verify each finding before reporting. Fix the real ones, re-run
the gate, then push. A review-round fix is a push and gets its own review.

One sub-issue → one branch → one PR. Never bundle.

```sh
git push -u origin {branch}
gh pr create --title "{title}" --body "Closes #{number}

## Summary
{what and why}

## Test plan
{how to verify}"
```

The PR closes the **sub-issue**, not the parent. Do not add the PR to the board
separately — the issue row already shows linked PRs.

Report the PR URL, then watch CI in the background and read the result when it
finishes. Never poll in a foreground loop.

```sh
gh run watch <run-id> --exit-status
```

### 7. Close out the parent — only if this was the last sub-issue

A sub-issue's PR closes only that sub-issue. After the PR **merges** (not at
open — the parent must not close while a sibling PR is pending), check:

```sh
gh api repos/{owner}/{repo}/issues/{parent}/sub_issues -q '.[] | "#\(.number) [\(.state)]"'
```

If all are closed:

1. **Verify the parent's end-to-end criteria** actually hold across the shipped
   sub-issues — they are distinct from any single sub-issue's
2. **Coherence sweep** — confirm deferred follow-ups were filed and linked rather
   than silently dropped, and that the plan and ADRs describe *shipped* reality.
   Grep for stale forward-references ("settled in 1.3") and fix them in the same
   closeout
3. **Tick the parent's verification checkboxes**
4. **Flip the plan's sub-scope heading** `[PENDING]` → `[COMPLETE]` in
   [`docs/plans/`](../../../docs/plans/). This is a small docs change —
   directly on `main`, its own PR, or folded into the next sub-scope's first
   PR. Ask which, and note it in the closeout
5. **Close the parent** with a summary: sub-issue → PR table, verification
   confirmation, and tracked deferrals

This is usually a separate turn, since it waits on the merge. Get explicit
approval before pushing the plan change and before closing.

## Important

- **Never commit or push without explicit approval.**
- **Before any push to GitHub, run the local pre-push code review** — the rule
  and its calibration live in [`CLAUDE.md`](../../../CLAUDE.md) § Rules
  ("Every push that changes behavior gets a local code review first"). Read
  your own diff as a reviewer would first; it is cheaper than a CI round-trip
  and much cheaper than a review round.
- **Stop on surprises.** An unexpected incompatibility or an uncovered design
  question means stop and ask — do not continue in a possibly wrong direction.
- **In-flight scope**: apply the three tests in
  [`docs/workflow.md`](../../../docs/workflow.md) — Small, Clearly related,
  Documented — before absorbing anything the issue didn't anticipate. When in
  doubt, file the follow-up.

## Output

- A branch with the implementation, or a base-chained stack for large scopes
- An open PR linked to the issue, CI green
- If this was the last sub-issue: the parent closed out and the plan marked complete
