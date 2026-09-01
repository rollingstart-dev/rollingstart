# M2: Formats, hand-authored

> Expanded plan for the `M2: Formats, hand-authored` milestone. The roadmap
> entry is [`docs/roadmap.md`](../roadmap.md) § M2.
>
> This file is the drafting artifact. Its sub-scopes are copied verbatim into
> GitHub issues; its remaining content becomes the project board's README. After
> the milestone ships, `/milestone-endgame` appends a retrospective.

## Context

M1 delivered `rolling doctor` and the three capabilities under it: strict
config loading with positioned errors, a command runner with honest failure
classification, and the environment probes — proven against a real Rallly
clone both broken and healthy, with a reproducible recipe. Schema v0 carries
`[commands]` only, deliberately: specifying schema nothing reads is guessing.

M2 is where the rest of the instance definition stops being prose in the
roadmap and becomes files a loader rejects when they are wrong. Operations and
corpus pointers join `instance.toml`; the skill graph, the task pool, and the
learner profile get their formats, their reference pages, and their strict
loaders. Then the formats meet reality: a complete Rallly instance —
destination set, prerequisite edges, demonstration bars, operations mapped to
Rallly's own `db:*` scripts, and tasks with verifiers and reference solutions
— written **by hand**, because generating into an unvalidated format bakes the
format's mistakes into the generator.

This is the pace-notes milestone: the course gets written down by hand, and
checked, before anyone drives it at speed. No LLM is involved anywhere — that
stays true through M3.

**End state.** A fresh Rallly clone carrying the full `.rollingstart/`
definition validates under `rolling doctor`: graph well-formed, destination
reachable, tasks keyed to real nodes, verifiers referencing real commands.
Every task is proven solvable — its verifier passes the reference solution and
fails the base — by scripted check, the same both-ways proof M6 later
automates. The profile schema exists with written mutation rules and two
seeded example profiles, which M3's exit criterion consumes. And there is an
honest, recorded answer to the exit criterion's real question: is authoring a
trajectory a job a busy staff engineer would actually do?

## Key decisions

| Decision | Choice | Why |
|---|---|---|
| Frontmatter dialect | YAML, strict, via a position-aware library (`goccy/go-yaml` or equivalent) | The roadmap fixed Markdown + frontmatter and `[[links]]`; YAML is what the surrounding ecosystem — Obsidian, GitHub's preview, every static-site tool — actually renders and edits, and authoring UX is this milestone's exit criterion. Strict decoding with line-carrying errors preserves M1's rule: a typo costs seconds, not a debugging session. |
| Where format validation surfaces | `rolling doctor`'s harness-preconditions section | A committed-but-broken definition blocks every learner — the same class as an unparseable `instance.toml`, which is already a blocking red. This also gives authors their feedback loop for free: write markdown, run doctor, fix what it names. Absent `skills/` or `tasks/` stays valid — a v0 instance is not broken, it is smaller. |
| Schema growth is additive | Every valid v0 file is a valid v1 file | Strict parsing forbids pre-declaring future sections, so the only compatible direction of growth is new optional sections the old loader never saw. The repo's own `.rollingstart/instance.toml` stays v0 as the proof. |
| Doctor declares, never runs, the new content | Operations and verifiers are validated at load and executed by nothing in this milestone | Running operations (with prompt/auto and destructive handling) is M3's; interpreting verifiers is M3's. M2 proves them the way M1.5 proved doctor: a scripted, reproducible recipe with transcripts. |
| Operations are `db:*` rituals, not stack lifecycle | The Rallly example maps `db:generate` / `db:migrate` / `db:seed` / `db:reset` (destructive), not `docker:up` | The roadmap scopes M2's operations to Rallly's `db:*` scripts, and the design's examples are all database rituals. Whether a stack-up script may ever be an operation brushes the never-orchestrates seam — deferred below until an operation consumer exists. |
| Profile state format | TOML, strict, like `instance.toml` | The profile is machine-written and machine-re-read; it needs lossless round-trips and strict parsing more than it needs prose ergonomics. Markdown + frontmatter stays the format for things humans author. Still diffable, still greppable, per the design's plain-text requirement. |
| Node identity | The file's slug; edges are `[[slug]]` links | One file per node is roadmap-decided; slugs make edges greppable and renames reviewable as file renames. A dangling `[[link]]` is a load error, not a warning. |

Two decisions are expected to meet the ADR threshold and are drafted during
refinement rather than settled in this table: the **verifier interpretation
model** (sub-scope 2.3 — it shapes M3's loop and M6's generator, would need
re-deriving, and has real alternatives) and, if its resolution is non-obvious,
the **AGPL disposition for Rallly-derived task content** (sub-scope 2.5).

## Sub-scopes

### 2.1 — Instance schema v1: operations and corpus pointers [PENDING]

**Goal.** Extend `instance.toml` with declared operations and corpus pointers,
keeping every v0 file valid.

**Branch.** `m2.1/instance-schema-v1`

**Depends on.** Nothing.

**Acceptance criteria.**

- `[operations]` declares named lifecycle rituals: each carries a
  human-authored shell command and a destructive marking; names and commands
  are validated at load (nonempty, distinct), with positioned errors. The
  schema records the destructive flag; nothing in this milestone runs an
  operation — prompt-by-default arrives with the M3 runner
- Corpus pointers declare what roadmap § 2.1 names: exemplary code paths,
  exemplar PRs, and a definition of ready — validated as well-formed, with a
  pointer whose target does not exist in the working copy reported by doctor
- A v0 file (commands only) loads unchanged; strictness is preserved — unknown
  keys still fail with position and key
- [`docs/reference/instance-toml.md`](../reference/instance-toml.md) grows
  from v0 to v1 before implementation; the docs are the spec
- [`examples/rallly/`](../../examples/rallly/) gains 3–5 operations mapped to
  Rallly's own `db:*` scripts, destructive ones marked (`db:reset`),
  commented in the same register as the existing example
- The repository's own `.rollingstart/instance.toml` stays v0 — nothing forces
  adoption, and its continuing to load is the additive-growth proof

**Verification.**

- [ ] Table-driven loader tests: each new section, strictness on unknown keys,
      v0 compatibility, empty/duplicate names
- [ ] An e2e doctor fixture with a dangling corpus pointer reports it by name
- [ ] `gofmt`, `go vet`, `go test ./...` clean

---

### 2.2 — Skill graph format and loader [PENDING]

**Goal.** Define and load the author's graph — one Markdown file per node,
frontmatter, `[[links]]` — carrying the destination set, prerequisite edges,
and demonstration bars, all mechanically validated.

**Branch.** `m2.2/skill-graph`

**Depends on.** 2.1 (loader shape; the destination declaration may touch
`instance.toml`).

**Acceptance criteria.**

- A reference page (`docs/reference/skills.md`) is written first: node file
  anatomy, frontmatter fields, edge direction and semantics, how the
  destination set is declared, what a demonstration bar is
- One file per node under `.rollingstart/skills/`; node identity is the file
  slug; prerequisite edges are `[[slug]]` links with a defined, documented
  direction
- The destination set is declared in exactly one reviewable place; every
  destination node must exist and be reachable, mechanically checked
- Demonstration bars are per-node and expressible so that M3's deterministic
  loop can evaluate satisfaction without a model call
- Optional nodes are mechanically distinguishable from the required spine
- The loader rejects, with positioned errors suitable for verbatim display:
  malformed or unknown frontmatter, dangling `[[links]]`, duplicate slugs,
  prerequisite cycles, and a destination node that is missing or unreachable
- An instance with no `skills/` directory remains valid; doctor reports the
  state distinctly ("no graph declared"), not as red
- When a graph is present, doctor's harness section validates it — red on
  invalid, because committed-and-broken blocks every learner
- The engine seam holds: nothing in the loader knows what any node teaches

**Verification.**

- [ ] Table-driven tests cover every rejection class and a valid graph with
      optional nodes
- [ ] e2e doctor fixtures: valid graph green, each broken-graph class red with
      the error displayed
- [ ] `gofmt`, `go vet`, `go test ./...` clean

---

### 2.3 — Task format and verifier schema [PENDING]

**Goal.** Define and load the task pool: tasks keyed by skill node, typed on
the ladder, each carrying a structured verifier and a reference solution.

**Branch.** `m2.3/task-format`

**Depends on.** 2.1 (verifiers select declared commands and operations by
key), 2.2 (tasks key to nodes).

**Acceptance criteria.**

- A reference page (`docs/reference/tasks.md`) is written first: task anatomy,
  the type ladder (Use → Modify → Debug → Create → Compare, per
  [`docs/landscape.md`](../landscape.md)), the verifier schema, the
  reference-solution form, and base-commit recording
- Tasks live under `.rollingstart/tasks/`, each keyed to a skill node; a key
  naming no node is a load error, and a task type off the ladder is too
- A verifier is structured data, never shell: it selects declared commands or
  operations **by key** and states expectations; it may also declare test
  files the task adds — the shape M6's generator will emit — interpreted by
  the harness, never executed as shell
- A `doctor`-type verifier exists: satisfied when doctor's instance section is
  green — the local-dev-setup node's completion condition, per roadmap § 2.6
- The reference solution is recorded alongside the task, and the format
  records the base commit it applies to — the pool-staleness risk (roadmap
  § 4) means the format must carry what lazy re-verification will need
- Loader validation surfaces in doctor like the graph's: positioned errors,
  red when a committed pool is broken, "no tasks declared" when absent
- The verifier interpretation model is checked against the ADR threshold
  during refinement and drafted as an ADR if it meets it

**Verification.**

- [ ] Table-driven tests: valid tasks of several ladder types, unknown node
      key, unknown command key, off-ladder type, malformed verifier
- [ ] e2e doctor fixtures for valid and broken pools
- [ ] `gofmt`, `go vet`, `go test ./...` clean

---

### 2.4 — Profile schema and mutation rules [PENDING]

**Goal.** Define the per-learner profile — layout, evidence format, the
overlay reservation, the self-protecting `.gitignore` — and write the
normative rules for who mutates what.

**Branch.** `m2.4/profile-schema`

**Depends on.** 2.2 (evidence cites nodes by slug).

**Acceptance criteria.**

- A reference page (`docs/reference/profile.md`) is written first: the layout
  from roadmap § 2.5 (`profile/.gitignore` containing `*`, `evidence/`,
  `overlay/`), field semantics, and the mutation rules
- The mutation rules are normative and explicit: only grader verdicts change
  node satisfaction; coach observations (M5) append evidence and never satisfy
  a bar; overlay node creation belongs to M4's judge and to nothing else;
  no engine code ever commits profile content
- The schema is machine-written but diffable and human-readable; it re-loads
  strictly with positioned errors
- An absent profile is the valid fresh-learner state; a corrupt one is a
  named, blocking doctor finding — the learner's own state being unreadable
  stops the session before it lies
- The self-protection is proven by test: a repo with a committed instance
  definition and a populated profile keeps the profile untracked, regardless
  of the repo's root `.gitignore`
- Two seeded example profiles with visibly different starting evidence exist
  as committed fixtures (located during refinement — never inside a real
  `profile/`), because M3's exit criterion consumes exactly that pair

**Verification.**

- [ ] Table-driven tests: fresh, populated, corrupt, and both seeds
- [ ] e2e doctor fixtures: absent profile green, corrupt profile red
- [ ] The gitignore self-protection test fails if the `.gitignore` is removed
- [ ] `gofmt`, `go vet`, `go test ./...` clean

---

### 2.5 — The Rallly trajectory, hand-authored [PENDING]

**Goal.** Author the real thing — Rallly's graph and tasks — as the formats'
first contact with a real repository and the measure of the authoring job.

**Branch.** `m2.5/rallly-trajectory`

**Depends on.** 2.2, 2.3 (2.1's operations already landed with the example).

**Acceptance criteria.**

- 8–10 skill nodes with prerequisite edges, a declared destination set,
  per-node demonstration bars, and at least one optional node off the
  required spine — in [`examples/rallly/`](../../examples/rallly/), commented
  the way the existing example is: choices, not syntax
- One node is local-dev setup, whose verifier is doctor's instance section
  going green — the promise roadmap § 2.6 has been making since M1
- 2–3 tasks on the ladder, one new concept per task, each with a verifier and
  a reference solution, grounded in Rallly's real code — never a synthetic
  exercise
- **The licensing question is resolved before any reference solution merges.**
  Rallly is AGPL-3.0 and this repository is Apache-2.0; a reference solution
  that modifies Rallly's code is plausibly a derivative work. The disposition
  — original-only additions, a differently-licensed subtree, held-back
  reference material, or counsel that it is fine — is the maintainer's call,
  recorded where future instance authors will find it, as an ADR if it meets
  the threshold. Roadmap § 4 parked this at "before shipping a public Rallly
  instance"; M2 is when it arrives
- Authoring effort is recorded **as it happens** — wall-clock per artifact and
  a running friction list — because the exit criterion is a claim about the
  job, and the retrospective needs data rather than memory
- Format friction found while authoring feeds back: absorbed where the
  in-flight rules in [`docs/workflow.md`](../workflow.md) allow, filed
  otherwise

**Verification.**

- [ ] The full definition validates via doctor in a Rallly clone
- [ ] Maintainer reviews the trajectory content itself — the "busy staff
      engineer" judgment belongs to a human, not to this checklist
- [ ] The effort record exists and is honest
- [ ] `gofmt`, `go vet`, `go test ./...` clean

---

### 2.6 — Validation against Rallly [PENDING]

**Goal.** Prove the milestone exit criterion against a fresh clone —
mechanically where possible, honestly where not.

**Branch.** `m2.6/rallly-validation`

**Depends on.** 2.4, 2.5.

**Acceptance criteria.**

- M1's container recipe extends to the full definition: fresh Rallly clone
  plus `examples/rallly/.rollingstart/` → doctor validates the graph, pool,
  and operations declarations; the transcript is captured on the PR
- Every task is proven both ways by script: the verifier's selected commands
  pass with the reference solution applied and fail on the base — the
  self-verification M6 automates, run by hand here, transcripts captured
- The seeded profiles load and validate in the same run
- Gaps surfaced by real contact — schema friction, misvalidation, unreadable
  errors — are fixed here under the in-flight rules or filed as issues
- The effort record from 2.5 is summarized against the exit criterion: would
  a busy staff engineer actually do this job? Answered candidly in the PR and
  carried into the retrospective — an aspirational answer here defeats the
  milestone's purpose

**Verification.**

- [ ] Both proof transcripts (definition validation, task both-ways checks)
      attached to the PR
- [ ] Follow-up issues filed for anything not absorbed
- [ ] `gofmt`, `go vet`, `go test ./...` clean

## Explicitly deferred

- **Running operations** — prompt/auto flow, destructive prompting, recovery
  — M3. The schema carries the destructive flag now so M3 has something to
  honor.
- **The verifier interpreter** — M3 executes verifiers inside the loop; M2
  proves them by scripted recipe, the same way M1.5 proved doctor.
- **Routing, sessions, `rolling start`/`rolling done`, TUI** — M3. Bars are
  designed for deterministic evaluation now precisely so M3 doesn't reopen the
  format.
- **Overlay node creation, duplicate matching, remedial-depth caps** — M4.
  The profile schema reserves `overlay/` and nothing writes to it.
- **Judge and Explorer** — M4/M6. Corpus pointers are declared and validated
  but nothing reads their content yet.
- **Task generation and automated self-verification** — M6. The both-ways
  check exists in M2 only as a scripted proof.
- **`rolling init` authoring assistance** — M6, and it is the standing
  mitigation for the authoring-is-too-much-work risk; M2's effort record is
  the evidence it will be designed against.
- **A Go-repo instance, including this repository's own graph** — M7 owns the
  second ecosystem. Authoring it now would double the work before the formats
  have survived contact once.
- **Stack lifecycle as an operation** (`docker:up`) — brushes the
  never-orchestrates seam; no consumer until M3 runs operations, so the
  question is decided then, with the seam rule on the table.
- **Machine-readable doctor output** — still no consumer.

## Verification

End-to-end criteria for the milestone as a whole. Sub-scope 2.6 confirms the
first three directly.

- [ ] The roadmap exit criterion: the formats survive contact with a real
      repo — the complete Rallly definition validates on a fresh clone, and
      every task proves solvable both ways — and the authoring-effort record
      answers the staff-engineer question candidly
- [ ] Everything the roadmap enumerates exists: 8–10 nodes, destination set,
      demonstration bars, ≥1 optional node, 3–5 `db:*` operations, 2–3
      laddered tasks with verifiers and reference solutions, the profile
      schema with mutation rules
- [ ] Every format has a reference page written before its loader, and every
      v0 artifact remains valid under v1
- [ ] Doctor rejects every malformed-definition class with a positioned error,
      pinned by e2e tests
- [ ] Two seeded profiles with different starting evidence exist and validate
      — M3's exit criterion has its inputs
- [ ] The AGPL disposition for Rallly-derived content is decided and recorded
- [ ] CI green on Linux and macOS; no LLM call anywhere in the milestone
