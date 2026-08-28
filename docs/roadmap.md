# Rolling Start — Roadmap

Companion to `docs/design.md`. The design doc says what we're building; this
says what we decided, why, and in what order we build it.

Milestones here are the roadmap. Each one gets an expanded plan in
`docs/plans/` when `/plan-milestone` runs on it, and a GitHub milestone and
project board that track the work.

---

## 1. Decision record

Recorded so we don't re-litigate in three months. "Reversibility" is what it would
cost to change our minds after contributors arrive.

| Decision | Choice | Why | Reversibility |
|---|---|---|---|
| Engine language | Go | Single static binary matters more than anything else for locked-down enterprise laptops. Subprocess management is the engine's core job. A Java or C# developer can read and contribute to Go on day one; Rust they can't. | Expensive |
| License | Apache-2.0 | Explicit patent grant to users *and* from contributors; §5 removes the need for a CLA; the default for infra tooling enterprises adopt. MIT would also have been fine — the real decision was avoiding AGPL, which blocks pilots at conservative legal departments. | Expensive |
| Interface | CLI, long-running session | The learner uses their own editor. Watching the filesystem is *more* editor-agnostic than an extension, not less. | Cheap |
| VS Code | Deferred, as a thin client | The CLI must exist regardless. A later extension shells out over the same IPC — a display surface, not a second implementation. Explicitly not a fork. | N/A |
| Environment | Agnostic; Rolling Start never owns lifecycle | Devcontainers are common, not universal. Rolling Start runs where the dev environment already runs — bare metal, container, WSL2. No compose knowledge, no health checks, no port management. | Cheap |
| Task isolation | One working copy, temporal state | A real app stack (postgres + redis + localstack + …) can only host one checkout. Worktrees would mean duplicate stacks and port collisions. Also pedagogically better: the learner sees their change in the running app. | Moderate |
| Model access | Hybrid: Explorer + Judge | Split by what the call needs, not by fallback. Exploration is latency-tolerant, wants agentic traversal, and the agent's helpful bias is aligned. Judgment needs clean-room context, structured output, cheap small calls, and our prompt on top rather than someone else's. | Moderate |
| Agent adapters | One in v1 (Claude Code) | The *interface* makes "your tool of choice" credible; N implementations make it expensive. Document the seam, let contributors send Codex and opencode. | Cheap |
| Generation timing | Author-time, pooled | Generation is expensive and infrequent; judgment is cheap and constant. Pooling means most learners never invoke an agent CLI at all, which fixes per-seat cost. Adaptivity moves into selection. | Moderate |
| Verifiers | Structured data, not shell | Generated code nobody reviews must not execute arbitrarily. A verifier is "run the declared test command against these paths, expect these cases" — interpreted by the harness. Residual risk equals running the repo's own test suite, which the learner already accepted. | Expensive |
| Operations | Shell, human-authored | Different provenance from verifiers: committed to the repo and code-reviewed like anything else there. Destructive ones default to prompting. | Cheap |
| Profile storage | Repo-relative, self-protecting | The working copy is the only location that survives container rebuilds without a named-volume accommodation. `.rollingstart/profile/.gitignore` containing `*` means privacy doesn't depend on the repo's root gitignore, and is scoped so the committed instance definition survives. Env var override for cross-machine users. | Cheap |
| Profile scope | Per instance | A tutor, not a university. Also correct on the merits — "knows async TS" in one repo doesn't mean knowing it in another repo's idiom. Export/import is an add-on if portability ever matters. | Cheap |
| Formats | Markdown + frontmatter | Diffable, greppable, model-friendly, and a skill-graph change becomes reviewable in a PR. One file per skill node, `[[links]]` for edges. | Moderate |
| Platforms | macOS + Linux native; Windows via WSL2 | Enterprise Node development happens in WSL2 or a devcontainer because native Windows Node tooling is miserable. "Run Rolling Start where you run your dev environment" *is* the Windows story. | Cheap |
| Instance #1 | Rallly | Application, not framework. 48MB — the fastest iteration loop of the candidates. Structurally near-identical to uceap3: pnpm workspaces, turbo, `vitest.workspace.ts`, `apps/` + `packages/`, Prisma. Ships real `db:reset` / `db:seed` / `db:migrate` scripts, so the operations concept is exercised from day one. | Cheap |
| Instance #2 | uceap3 | The actual target. Unusually rich commit history, which is a hazard as much as an asset — see Risks. | N/A |
| Instance #1.5 | A Go repo | Deliberate second ecosystem, added early. Generalizing an adapter interface from one toolchain produces the wrong interface. Nearly free since the engine is Go. | Cheap |

---

## 2. Architecture

### 2.1 The seam

The engine never parses the target language. It reads git, spawns commands the
instance declares, diffs a working copy, keeps state files, and calls a model.
Supporting a new ecosystem is a config file and an output parser, not a port.

An instance declares **commands** (build, typecheck, test, lint), **operations**
(named lifecycle rituals), a **skill graph** carrying the author's **trajectory**,
a **task pool**, and **corpus pointers** (exemplary code, exemplar PRs, definition
of ready).

### 2.2 Trajectory and route

The central mechanic, and the one most likely to be got wrong. The author owns
where the learner ends up; the harness owns how they get there.

**The author defines** a destination set — the nodes that constitute competence in
this codebase — plus prerequisite edges between nodes and, per node, what has to
be demonstrated before it counts as satisfied. Optional nodes may hang off the
required spine. This is committed, human-edited, and reviewable in a PR.

**The harness decides**, per learner, from the profile: which of the currently
reachable nodes to serve next, which task from that node's pool, whether the
evidence so far satisfies the node or another task is needed, and whether a
demonstrated gap or aptitude should open an optional node. Selection is
pathfinding across an author-drawn graph, not curriculum generation.

**The harness also creates nodes.** The route frequently needs territory the
author never mapped, and it is learner-specific: one person needs a detour through
jsonb semantics, another through TypeScript union types, and neither is about this
repo. The agent adds these on evidence, as prerequisites hanging off the path.
Wherever the concept appears in the repo, the remedial task grounds in that real
code rather than a synthetic exercise — the grounding requirement doesn't lapse
just because the node is remedial.

**Two layers, different lifetimes.** The **instance graph** is committed: the
author's destination set, their prerequisite structure, their demonstration bars.
The **learner overlay** lives in the profile and is not committed: nodes the agent
created for this person. Routing runs over graph plus overlay. This falls out of
the per-instance profile decision at no extra cost.

**The harness never** adds to, removes from, or reorders the *destination set*.
Deciding what competence here means is the author's job and only theirs. When the
graph itself looks wrong — an unreachable node, a prerequisite that isn't one, or
the same overlay node appearing across many learners — that escalates to the
author as a proposal with the trace attached. Promotion from overlay to instance
graph is a human act; creating an overlay node is not.

Two learners with different backgrounds should arrive at the same destination
having done substantially different work. If they don't, the routing isn't
adapting. If they arrive somewhere the author didn't specify, the scaffold isn't
holding.

### 2.3 Two model interfaces, deliberately asymmetric

**Judge** — one method: given an exact prompt and a schema, return conforming
JSON. **No filesystem access at all**, which makes fresh context true by
construction rather than by discipline. Implemented as a thin HTTP client against
the Messages API with a pluggable base URL, so Bedrock, Azure, and internal
gateways work. Defaults are detected from the agent CLI's existing environment, so
nobody configures an endpoint twice.

**Explorer** — given a repo and a goal, produce a structured artifact.
`exec.Command`, stdin, parse the envelope. One adapter is roughly 60 lines: binary
name, one-shot flags, output format, parser.

Judge runs at learn time, constantly, cheaply. Explorer runs at author time,
rarely, expensively.

### 2.4 Session model

`rolling start` opens a long-running session — Bubble Tea TUI in one pane, filesystem
watcher, task state. `rolling done` from another pane talks to it over local IPC
(localhost TCP with a token file; portable, and the same surface a VS Code client
would use later).

Three tiers of feedback:

- **Tier 0** — the instance's declared commands, run on debounce with a
  concurrency guard. No model call. Most of the value.
- **Tier 1** — small-model, fired on *events*, never on saves: same test failing
  three times, edits outside the task's expected scope, long silence, reinventing
  something that already exists in the repo. Offers; does not assert.
- **Tier 2** — learner-triggered full review. Fresh context, diff only.

### 2.5 Storage

```
<repo>/.rollingstart/
  instance.toml       # commands, operations, corpus pointers
  skills/             # the author's graph: one file per node, [[links]] for edges
  tasks/              # candidate pool, keyed by skill node
  profile/
    .gitignore        # contains "*" — self-protecting
    overlay/          # nodes the agent created for this learner
    evidence/         # what they've demonstrated, and where
```

The instance definition — `instance.toml`, `skills/`, `tasks/` — is committed and
reviewed like any other repo config. Everything under `profile/` belongs to one
learner and is never committed. The self-protecting `.gitignore` sits **inside
`profile/`**, not at the `.rollingstart/` root: at the root it would swallow the
instance definition too.

---

### 2.6 Readiness has two meanings

`rolling doctor` reports two things that look alike and are not.

**Harness preconditions** — a git repository, a clean working tree, an
`instance.toml` that parses, a watcher that fires on a synthetic file event, a
sane `core.autocrlf`. Red here is blocking: nothing can proceed and no lesson can
be served.

**Instance command health** — whether the declared build, typecheck, test, and
lint commands actually run. Red here is *informational*, because a codebase
instance's first lesson is frequently "get this running for local dev." A failing
`pnpm install` may mean the environment is broken, or it may mean the learner
hasn't done lesson one yet. The harness cannot tell those apart and must not
pretend to.

That gives the setup lesson somewhere real to live: a skill node whose operations
are the project's own install and stack-up commands, and whose completion
condition is doctor's second section going green. The learner's first task is
making the tool's own readiness check pass.

Where `rolling` runs varies by target, and the harness does not care. Rallly's
`docker-compose.dev.yml` is infrastructure only — postgres, redis, garage,
mailpit — and its toolchain runs on the host. uceap3 has an `app` service with
the workspace bind-mounted, so its toolchain lives inside the container. Instance
#1 and instance #2 therefore exercise both shapes, which is the agnosticism claim
being tested rather than asserted.

---

## 3. Milestones

Each is independently useful. No model call appears until M4 — M1 through M3 are a
working deterministic tutor.

### M0 — Skeleton
Go module, `cmd/rolling` + `internal/*` mirroring homie's layout. Cobra + Fang, Bubble
Tea. Apache-2.0, NOTICE, CI.
**Exit:** `rolling version` runs on macOS and Linux.

### M1 — `rolling doctor`
Instance config loading. Run the declared commands, parse their output, and
report against the two-section model in §2.6 — blocking harness preconditions
kept separate from informational instance command health.

Probes: git repo present, working tree clean, `instance.toml` parses,
`core.autocrlf` sane, and a synthetic file event the watcher must observe.

**Exit:** point `rolling doctor` at a fresh Rallly clone on a machine with no
pnpm and no stack running — the exact state a real learner starts in — and get
harness-green with instance-red, the instance failures named accurately rather
than reported as breakage. Then install pnpm, bring the stack up, and watch the
second section flip to green.
*No LLM. Proves the adapter and output parsing before anything depends on them.*

### M2 — Formats, hand-authored
Write a Rallly instance **by hand**: 8–10 skill nodes with prerequisite edges, a
declared destination set, per-node demonstration bars, at least one optional node
off the required spine, 3–5 operations mapped to its `db:*` scripts, and 2–3 tasks
with verifiers and reference solutions. One of those nodes is local-dev setup,
whose verifier is doctor's instance section going green. Task types follow the
ladder borrowed from repo-learner-suite — Use → Modify → Debug → Create →
Compare — one new concept per task (see docs/landscape.md). Profile schema and its mutation rules.
**Exit:** the formats survive contact with a real repo, and authoring a trajectory
feels like a job a busy staff engineer would actually do.
*Hand-author before generating. Otherwise you generate into an unvalidated format.*

### M3 — The loop, deterministic
`rolling start` → session → route selection over the authored graph → serve a task →
learner edits → `rolling done` → run verifiers → deterministic verdict → profile update
→ recompute the route. Base-commit recording, dirty-tree refusal, task
abandonment. Operations with prompt/auto and destructive handling.
**Exit:** Brandt completes three hand-authored Rallly tasks end to end and the
profile reflects it — *and* two seeded profiles with different starting evidence
produce visibly different routes to the same destination set.
*The whole product minus judgment, and it runs with no API key.*

### M4 — Judge
Direct API with pluggable base URL. LLM adjudication over the deterministic
result. Structured verdicts carrying file, line, and rule. Provenance labels
distinguishing local convention from language norm. Degrades to "skipped" without
a key. Also where **overlay nodes** arrive: diagnosing "this failed because they
don't understand jsonb, not because they can't write TypeScript" is a judgment
call, so it can't exist in M3's deterministic loop. Node creation, duplicate
matching against the existing overlay, and the remedial-depth cap all land here.
**Exit:** a style verdict on a real Rallly diff that cites real Rallly code, and a
seeded profile with a planted gap earns a correctly-named overlay node whose task
grounds in Rallly's own use of the concept.

### M5 — Coach
Watcher with aggressive ignores. Tier 0 on debounce. Tier 1 event triggers.
Offers-not-asserts presentation in the TUI. Process observations feed the profile
and never the verdict.
**Exit:** the coach catches a wrong-file edit within a minute and says so once.

### M6 — Explorer
Claude Code adapter. Mine git history for candidate tasks. Self-verification —
passes the reference, fails the base — before anything enters the pool. `rolling init`
flow that walks an author through operations first, since those are the easiest to
write from memory.
**Exit:** 20 generated Rallly tasks, all self-verified, indistinguishable in
quality from the hand-authored ones.

### M7 — Real targets
An instance for uceap3. An instance for a Go repo, to break the TS/pnpm
assumptions baked in by M1–M6.
**Exit:** a colleague who has never seen uceap3 completes a real task in it.

### M8 — Release
Public repo, adapter documentation, contributing guide, instance authoring guide.
**Exit:** a contributor lands a second agent adapter without our help.

---

## 4. Risks

**Silent watcher failure across the WSL2 boundary.** A repo on `C:` watched from
WSL2 receives no inotify events at all. The coach would sit there looking healthy
and never fire. *Mitigation:* M1's synthetic-event probe, and refuse to start the
coach if it fails.

**Generation assumes rich commit history.** Both our instance repos are
unrepresentative — uceap3's commit bodies explain domain reasoning and cite prior
decisions; most enterprise repos say `fix stuff` and `wip`. Building the generator
against them bakes in an assumption that breaks on the first real customer.
*Mitigation:* test the generator against a deliberately bad-history repo before
M8, and make the author-nominated exemplar path first-class rather than a
fallback.

**Routing collapses to a fixed order.** The most likely silent failure. If
selection always picks the lowest-numbered reachable node, every learner walks the
same path and we've shipped a syllabus with extra steps. It will still *work*,
which is why nobody will notice. *Mitigation:* M3's exit criterion tests it
directly, and route divergence between learners is a metric worth watching for the
life of the project, not a one-time check.

**Trajectory authoring is too much work.** The author's job now includes a
destination set, prerequisite edges, and demonstration bars — on top of exemplars,
operations, and a definition of ready. If bootstrapping an instance takes a week,
nobody does it and the network effect never starts. *Mitigation:* M6's `rolling init`
should let the Explorer propose a draft graph from the repo for the author to
edit, so authoring is review rather than blank-page work. The author still owns
every node; they just don't type them all.

**Unbounded remediation.** The agent can now create nodes, so a struggling
learner can accumulate an ever-deepening tree of prerequisites and never reach the
destination. Detours must be bounded — cap remedial depth on any one path, and
escalate to a human when the cap is hit. A learner three levels deep in
prerequisites has a problem no amount of routing solves.

**Overlay churn.** Agent-created nodes are cheap to make and easy to duplicate:
"jsonb columns," "postgres JSON operators," and "querying nested JSON" may be one
node created three times under different names. *Mitigation:* match new overlay
nodes against existing ones before creating, and treat near-duplicates as a signal
the node needs promoting into the instance graph.

**Commit-message convention lock-in.** Rallly uses gitmoji, uceap3 and Documenso
use conventional commits. Do not hardcode a `fix:` prefix anywhere.

**Pool staleness.** Generated tasks reference commits, files, and line ranges that
drift as the instance repo moves. A task that no longer applies is worse than no
task. *Mitigation:* re-run self-verification lazily at selection time; retire
tasks that fail.

**Migration-touching commits.** Reverting a commit that changed a schema isn't a
file change. These are among the best exercises available and they only work if
the instance declares the right operations as preconditions. Detect and skip when
it can't.

**Agent CLI context contamination.** Rallly, uceap3, Documenso, and Zod all ship
`CLAUDE.md` or `AGENTS.md`, and some ship `.mcp.json`. The Explorer inherits all of
it. Suppress explicitly; never assume a clean invocation.

**Destructive operations.** Dropping someone's local database uninvited is the
same category of damage as stomping uncommitted work — one incident and the tool
is never trusted again. Prompt by default, always.

**Published-instance licensing.** Rallly and Documenso are AGPL-3.0. Irrelevant
for private company repos, which are the actual market, but an instance that
embeds snippets from an AGPL repo raises a derivative-work question if we publish
it. Check before shipping a public Rallly instance.

**Scope creep into the editor.** The VS Code extension is a display surface. The
moment it starts owning the editing experience, we've become the thing the brief
says we're not.

---

## 5. Deferred, explicitly

Not now, and not by accident:

- Multi-agent adapters beyond Claude Code
- VS Code extension
- Cross-instance profile portability
- Cohort/manager views over learner progress
- Sandboxing beyond "structured verifiers plus the environment the learner already
  trusts"
- Native Windows support
- The language-tutor instance — it exercises the harness but dodges every risk
  that could kill the project: content provenance, grounding, skill-graph
  induction, and the author role

## 6. Rejected, with reasons

Distinct from §5: these are not postponed, they are out of scope by design.

**Spaced repetition and any decay model.** Rolling Start is an on-ramp, not a
residence. The goal is self-sustaining proficiency and then departure. A learner
still being drilled a year in is a symptom, not a feature, and if getting someone
productive takes longer than about three months the product failed. Tracking a
codebase as it evolves is a real problem and a different product.

**Explaining the codebase as an end in itself.** That category is crowded and
well funded — Greptile, Unblocked, DeepWiki, Sourcegraph. Explanation here exists
only in service of a task the learner is about to attempt. See
docs/landscape.md.
