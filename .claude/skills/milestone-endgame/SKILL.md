---
name: milestone-endgame
description: Wrap up a shipped milestone — triage open issues, write the retrospective, update CLAUDE.md, archive the board and close the milestone.
---

Wrap up a shipped milestone: triage what's left, write the retrospective, update
the project documentation, and prepare for the next one.

## Inputs

A milestone number.

## Execution model

Run the endgame as tracked, reviewable work — not one untracked session. It is
the milestone's own closing act and it earns the same process as everything else.

1. Open a **closure tracking issue** and run it through `/refine-issue`, slicing
   along these natural lines:
   - **Open-issue triage** — every open issue on the milestone gets an explicit
     disposition: close, pull into this milestone, move to a later milestone, or
     backlog. All of them, before the retrospective is written.
   - **Process amendments** — any guardrail the retrospective questions produce.
     Changes to `CLAUDE.md` rules, `REVIEW.md`, or the skills deserve their own
     reviewable PR.
   - **Retrospective and closure** — the final sub-issue.
2. Land the triage's pulled-in fixes and the process amendments **first**, each
   via its own PR. The retrospective cites them, so they must be settled before
   it's written.
3. Run the Process steps below on the closure sub-issue's branch, so the
   retrospective and doc updates get reviewed like any other change.

## Process

### 1. Gather the data

```sh
gh pr list --state merged --search "milestone:\"M{N}: {Title}\""
gh issue list --state closed --milestone "M{N}: {Title}"
gh issue list --state open   --milestone "M{N}: {Title}"
```

Read the plan at `docs/plans/m{N}-*.md` and its sub-scope statuses.

### 2. Write the retrospective

Append a `## Retrospective` section to the plan document:

- **Planned vs. delivered** — which sub-scopes shipped, which changed shape, which didn't
- **Decisions made during implementation** that weren't in the plan. Anything
  meeting the ADR threshold gets an ADR, not just a retro paragraph
- **What went wrong** and how it was resolved, citing PR numbers
- **What was deferred**, with reasons and where it went
- **Patterns to carry forward** — what worked and should become standard
- **Patterns to change** — what didn't, and what to do instead

Be specific and unflattering. A retrospective that reads as a success report is
not doing its job, and this repository is public — a candid one is worth more to
a reader than a tidy one.

### 3. Update [`CLAUDE.md`](../../../CLAUDE.md)

- Tick the milestone in § Status
- Add conventions that emerged during the work
- Update § Key locations if new important paths appeared
- Add rules the retrospective argued for

### 4. Re-examine what's next

Read the next milestone's roadmap entry. What did this one teach that changes
it? Flag risks that are now visible and adjust
[`docs/roadmap.md`](../../../docs/roadmap.md) § 4 if a risk landed or a new one
appeared. Propose changes; don't make structural ones without approval.

### 5. Archive the board and close the milestone

```sh
gh project close {project-number} --owner rollingstart-dev
gh api repos/{owner}/{repo}/milestones/{milestone-number} -X PATCH -f state=closed
```

Before closing, confirm every issue is either closed or explicitly moved to
another milestone. An issue orphaned by a closing milestone is lost work.

### 6. Report

- What shipped, and what didn't
- Key learnings and any new ADRs
- Recommendations for the next milestone
- Anything still needing a decision

## Output

- Plan document with a `## Retrospective` section
- Updated `CLAUDE.md`, and any ADRs the milestone produced
- Board archived, milestone closed, no orphaned issues
- A recommendation for what to plan next
