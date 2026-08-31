---
name: refine-issue
description: Refine a GitHub issue — review requirements, write the documentation that specifies it, and slice it into review-sized sub-issues before implementation begins.
---

Refine a GitHub issue: review requirements, write the documentation that
specifies the work, and break it into review-sized pieces. This is the planning
step for an issue, and it happens before any code is written.

## Inputs

An issue number or URL.

## Process

### 1. Read the issue

```sh
gh issue view {number} --comments
```

`--comments` is load-bearing. Plain `gh issue view` prints the body alone, so a
scope note or a decision that landed as a comment is invisible without it.

Read every document, spec, and ADR linked from the body.

### 2. Read context

- [`CLAUDE.md`](../../../CLAUDE.md) — conventions and the architectural seams
- [`docs/workflow.md`](../../../docs/workflow.md) — the docs-driven sequence
- [`REVIEW.md`](../../../REVIEW.md) — what reviewers will check
- [`docs/roadmap.md`](../../../docs/roadmap.md) §§ 1–2 and any relevant risk in § 4
- The milestone's plan in [`docs/plans/`](../../../docs/plans/)

### 3. Decide what documentation this needs

**Does it change user-visible behaviour?** Write or update the documentation
describing that behaviour first — README, command help text, or the instance
authoring guide. That documentation is the spec, and the failing test in
`/implement-issue` encodes it.

Draft it during refinement. It lands through a PR — its own, `Refs #N`
against the issue, so the spec is reviewed before any code is written against
it — never as a direct commit to `main`. Sub-issues can cite its path on
`main`; the links resolve when the PR merges.

**Is it architecture or infrastructure?** Check whether it meets the ADR
threshold in [`docs/decisions/README.md`](../../../docs/decisions/README.md) —
affects more than the file you're editing, would have to be re-derived if
forgotten, and a reasonable person could have chosen otherwise. If so, draft the
ADR now; it lands the same way, through a PR. If not, the reasoning belongs in
the commit body and nowhere else.

**Does it cross an architectural seam?** The engine parsing a target language,
the harness touching environment lifecycle, generated content becoming shell,
coach observations reaching a verdict — any of these means the plan is wrong.
Stop and raise it rather than designing around it.

### 4. Slice into review-sized sub-issues

Default to slicing anything non-trivial. Each sub-issue becomes its own branch
and its own PR, because a PR spanning config parsing, the runner, and the CLI
cannot be reviewed properly.

- Cut vertical slices that are independently reviewable: compiles, tests pass,
  delivers something coherent
- Name the dependencies between them and order accordingly
- Branch names under the milestone scope prefix — `m1.2/command-runner`
- Draft a title, goal, branch, dependencies, and acceptance criteria for each,
  and **present the list to the user before creating anything**

Then, for each sub-issue:

- **Assign the parent's milestone at creation** (`gh issue create … --milestone "M{N}: …"`).
  Milestones are *not* inherited — an unassigned sub-issue is invisible to the
  milestone's progress bar and to milestone-filtered queries.
- **Link it natively to the parent**, which populates the parent's Sub-issues
  panel and the board's progress column. Native linking also carries board
  membership, so no separate `project item-add` is needed:

  ```sh
  SUB_ID=$(gh api repos/{owner}/{repo}/issues/<sub-number> -q '.id')
  gh api repos/{owner}/{repo}/issues/<parent-number>/sub_issues -X POST -F sub_issue_id=$SUB_ID
  ```

Fall back to a single-issue refinement only when one focused PR would genuinely
suffice.

### 5. Convert the parent into a tracking issue

- **Do not write a markdown checklist of sub-issues.** Once they're linked
  natively, GitHub renders the panel itself; a hand-written checklist is
  redundant and drifts.
- Keep a high-level scope summary and links to the plan, ADRs, and specs
- Move detailed acceptance criteria down into the sub-issues; keep the parent's
  end-to-end verification list, which the final sub-issue closes
- Fix any drift between the original body and what refinement established

If no sub-issues were created, add a refinement comment instead: what you found,
what documentation changed, open questions, and sharpened acceptance criteria.

**Fold-ins go in issue bodies.** If refinement folds another issue's work into
this one, record it as a checklist item in the receiving issue's *body* — never
only as a comment. Implementation reads bodies.

### 6. Present findings

Report what you found and proposed. Wait for approval before creating
sub-issues or opening the documentation PR. Never commit the documentation
directly to `main`.

## Output

- Parent converted to a tracking issue, or a refinement comment if the scope is small
- Sub-issues created, each scoped to one reviewable PR, on the milestone and natively linked
- Draft documentation or an ADR where the work needed one, opened as a PR
  against the issue
- Open questions flagged
