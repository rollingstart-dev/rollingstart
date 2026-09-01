---
name: plan-milestone
description: Expand a roadmap milestone into a plan document and set up GitHub tracking — project board, issues, milestone assignment. Use when starting a new milestone.
---

Expand a roadmap milestone into a working plan and set up its GitHub tracking.

## Inputs

Work out which milestone automatically:

1. Read [`CLAUDE.md`](../../../CLAUDE.md) § Status for the first unchecked milestone
2. Read [`docs/roadmap.md`](../../../docs/roadmap.md) § 3 for that milestone's scope and exit criterion
3. Confirm: "Next up is **M{N}: {Title}** — {scope}. Exit criterion is {…}. Plan this one?"
4. Proceed only after the user confirms

The user may name a different milestone instead.

## Process

### 1. Read context

- [`docs/workflow.md`](../../../docs/workflow.md) — how work happens here
- [`docs/plans/TEMPLATE.md`](../../../docs/plans/TEMPLATE.md) — the required format
- [`docs/roadmap.md`](../../../docs/roadmap.md) §§ 1–2 — decisions and architecture the plan must not contradict
- [`docs/decisions/`](../../../docs/decisions/) — anything decided since the roadmap
- The most recent plan in [`docs/plans/`](../../../docs/plans/), for style and depth
- The relevant risks in [`docs/roadmap.md`](../../../docs/roadmap.md) § 4 — a plan that ignores a named risk is incomplete

### 2. Draft the plan

Write `docs/plans/m{N}-{slug}.md` from the template:

- **Context** — what the previous milestone delivered, what this one delivers, the end state
- **Key decisions** — with rationale. Anything meeting the ADR threshold in [`docs/decisions/README.md`](../../../docs/decisions/README.md) gets an ADR instead of a table row
- **Sub-scopes** under a `## Sub-scopes` H2, each an H3 `### {N}.{M} — Title [PENDING]` carrying a one-sentence goal, branch name, dependencies, acceptance criteria, and a verification checklist, separated by `---`
- **Explicitly deferred** — with reasons, so it isn't re-litigated mid-build
- **Verification** — end-to-end criteria for the milestone as a whole

Acceptance criteria are outcomes, not file lists. The implementor picks the
files.

Slice sub-scopes at natural dependency boundaries, each independently
reviewable: it compiles, its tests pass, and it delivers something coherent. If
{N}.2 needs {N}.1 merged first, say so.

The plan file is the *drafting* artifact. Its content gets split across the
project board README and one issue per sub-scope, and `/milestone-endgame`
later rewrites this file into a retrospective — so everything an implementor
needs must reach the project and issues, not just this file.

### 3. Present for review

Do **not** commit yet. Present the plan and iterate until the user is satisfied.

### 4. Create the project board

One board per milestone, under the `rollingstart-dev` org:

```sh
gh project create --title "Rolling Start: M{N} — {Title}" --owner rollingstart-dev
```

Note the project number.

### 5. Link the board to the repo

Without this it doesn't appear on the repo's Projects tab:

```sh
gh project link {project-num} --owner rollingstart-dev --repo rollingstart-dev/rollingstart
```

### 6. Set the board's description and README

Together these carry everything in the plan except the sub-scopes, so the full
plan is reflected in project + issues.

- **Description** (~255 chars): one sentence distilled from Context
- **README**: all plan content *except* `## Sub-scopes`, opening with a link back to the plan file

```sh
gh project edit {project-num} --owner rollingstart-dev \
  --description "..." \
  --readme "$(cat /tmp/m{N}-readme.md)"
```

### 7. Create the sub-scope issues

The `M{N}: {Title}` milestone already exists — every roadmap milestone was
created up front. The script resolves it and errors if it's missing.

```sh
.claude/skills/plan-milestone/scripts/create-issues.sh \
  docs/plans/m{N}-{slug}.md \
  {N} \
  {project-num}
```

The script splits `## Sub-scopes` on `### {N}.{M} —` headers, creates one issue
per sub-scope on the milestone, adds each to the board, and prefixes each body
with a link back to the plan section. On the way it reflows hard-wrapped
paragraphs into single lines: GitHub renders each newline in an issue body as a
hard break, so the plan file's wrapped prose would otherwise render jagged.

**Full-fidelity rule**: sub-scope content is copied in full — reflowed for
issue rendering, never summarized or trimmed. An agent picking the issue up in
a future session must have everything it needs without reading anything else.

Milestones carry no due dates. This is a side project; invented dates rot in
public.

### 8. Commit the plan

Get explicit approval, then commit.

## Output

- `docs/plans/m{N}-{slug}.md`
- A linked project board with description and README
- One issue per sub-scope, all on the `M{N}: …` milestone and the board

## Next step

The issues are verbatim plan copies — they have **not** been refined into
ready-to-build requirements. Point the user at **`/refine-issue {first issue}`**,
not `/implement-issue`. The order in [`docs/workflow.md`](../../../docs/workflow.md)
is `/plan-milestone` → `/refine-issue` → `/implement-issue` → `/milestone-endgame`.
