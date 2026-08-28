# Rolling Start — Product Summary

*The shape of the thing, and the decisions behind it.*

## The idea

An open source **teaching harness**: a deterministic runtime wrapped around an LLM
coding agent that turns a repository into an adaptive tutor.

The instance author defines the destination — what competence in this codebase
looks like — as a scaffold the harness must honor. The harness finds each
learner's route through it: one task at a time, evaluated when the learner says
they're done, with a persistent model of what they know deciding what comes next.
Two people reach the same destination by different paths. Not a fixed syllabus,
not waterfall, and not open-ended self-direction either.

The agent generates and judges. The harness owns state, verification, and
repeatability — the parts an LLM does unreliably alone.

## Who it's for

In-house development teams at large companies, frequently not tech companies. The
motivating case: an industrial, public-sector, or education org modernizing legacy
applications, needing to onboard existing staff onto a new codebase they had no
hand in building.

That audience shapes everything downstream. Their new codebases are polyglot —
TypeScript in front, a service layer behind it, SQL, Terraform, container configs.
Their laptops are locked down and their install paths run through an internal
artifact repository. Their legal teams read licenses. Their source frequently
cannot leave the building. And they have already adopted an agent CLI, which is
why they'd consider this at all.

## Shape

**Loop:** pick the next skill on this learner's route → serve a task → learner
edits in their own editor → learner says done → evaluate the diff → update the
learner profile → recompute the route → repeat.

**Two layers:** a generic **engine** (what contributors hack on) and **instances**
— a Rolling Start for a specific codebase. Instances are forkable and shareable;
that's where the network effect lives.

**Human role:** one author per instance, not a tutor per learner. The author owns
the destination — which skills constitute competence here, what has to be
demonstrated, and in what rough order — and supplies what an LLM can't infer:
which code is exemplary and which is legacy, a few PRs that show how work is done
here, what "ready" means, and the local lifecycle rituals. Per-learner human
involvement is on-call triage, not tutoring.

**Posture:** Rolling Start runs wherever the learner's dev environment already
runs — bare metal, a container, a devcontainer, WSL2. It never orchestrates that
environment, never assumes a stack, and requires no accommodation to install.

## What makes it work

**Durable learner profile.** Skills attempted, evidence for each, open
misconceptions, the route taken so far. Plain text, instance-scoped, kept in the
working copy so it survives container rebuilds. Everything else is regenerable;
this isn't.

**Authored destination, adaptive route.** The author owns where the learner ends
up: the set of skills that constitute competence in this codebase. The harness
owns how they get there — which node comes next among those they're ready for,
which task from the pool, how much evidence satisfies a node.

The route frequently runs through territory the author never mapped. A learner who
knows TypeScript cold may never have met a jsonb column; another may be fluent in
Postgres and unable to read a union type. Neither gap is about this repo, and both
are blocking. The agent creates those nodes itself, on evidence, as prerequisites
hanging off the path — grounded in the repo's own code wherever the concept
appears there. What it never does is decide what competence here means.

Tasks are drawn from a pool and thrown away after use; adaptation lives in
routing, not in regenerating a curriculum every time.

**The tightrope.** Too rigid and we have rebuilt the fixed syllabus we're
replacing. Too loose and learners wander, plateau, and never arrive. The
trajectory belongs to the author; the route belongs to the learner. Getting this
balance wrong is the most likely way this fails as a product rather than as
software.

**Self-generating content from git history.** A bugfix commit is a ready-made
exercise: revert it, hand over the issue, verify against the test that shipped
with the fix. A merged PR is a ready-made review exercise. Content comes from the
repo, not from hand-authoring.

**Self-verifying task generation.** Generation emits task, verifier, and reference
solution; the harness checks the verifier passes the reference and fails the
starting state before the learner sees anything. A subtly impossible exercise
destroys trust faster than no exercise. The same check doubles as an environment
probe — a verifier that fails on *both* base and reference means something is
broken that isn't the learner.

**Lifecycle operations as first-class content.** Instances declare named
operations — reset the database to the dev seed, empty local before testing a
migration, re-run the seeder. Knowing these rituals is among the most expensive
tacit knowledge a new hire acquires, it is never documented, and no competing tool
teaches it. Operations serve three roles: precondition for a task, teachable step
inside a task, and recovery when the learner wedges their local state.

**Two-tier evaluation.** Deterministic first — compiler, tests, linter — and it
runs without an API key. LLM adjudication on top for style, idiom, and house
convention, degrading gracefully to "skipped." Contributors without a key can
still use and extend the thing.

**Coach and grader are different roles.** A **coach** runs continuously alongside
the learner, watching the filesystem, offering rather than asserting. A **grader**
runs only when the learner says done, in fresh context, seeing the diff and
nothing else. The coach's observations feed the profile; only the grader's
verdicts move the skill graph. The tutor who helped you is never the examiner who
judges you.

**Grounded, non-sycophantic judgment.** Style verdicts cite real code in the repo,
not the model's priors. Verdicts require evidence — a line and a rule. And
verdicts are labeled by provenance: a local convention is not a language norm, and
a junior can't tell them apart unless we say so.

**Escalation as a feature.** Repeated failure on a node, a disputed verdict, a
question the corpus can't ground, a task that fails its own verification twice —
each routes to a human with the trace attached. So does a detour many learners
turn out to need: when eight of twelve people required the same unmapped
prerequisite, that's a hole in the author's graph, surfaced as a proposal for a
human to promote. Instances get better as people learn from them.

## The arc

Learn the codebase → contribute (bug fixes, then features) → review contributions
→ refactor → architect. These are task types over one instance's skill graph, not
separate products. Existing tools pick one rung. Teaching *review* is the least
served and probably the most defensible.

## What it is not

**An on-ramp, not a residence.** The goal is self-sustaining proficiency, and
then the learner leaves. There is no spaced repetition, no decay model, no
scheduled re-drilling. A learner still being examined a year in is a symptom, not
a feature, and if getting someone productive takes more than about three months
we have failed. Keeping people abreast of a codebase as it evolves is a real
problem and a different product.

**Not a documentation tool.** It never sets out to explain the codebase. Where
explanation happens it is in service of a task the learner is about to attempt.

## Where it sits

- Rustlings, Exercism, CodeCrafters — the watch-and-verify loop is commodity. The
  differentiator is generation and adaptation against *your* repo, not the loop.
- CodeTour, JetBrains EduTools, CodeRoad — in-editor playback of hand-authored
  content. They explain; they never make you prove anything.
- Claude Code's Learning output style — the honest baseline. The harness has to
  earn its keep with persistence, adaptation, and verification you can trust.
- Greptile, Swimm, Unblocked, OnBoardAI — codebase comprehension and onboarding.
  Documentation and answers, no assessment, no review training, and nothing that
  teaches the local rituals that aren't in the code.

## Decided

- **Go**, distributed as a single static binary. Install friction is the
  underrated adoption barrier for this audience.
- **Apache-2.0.** Explicit patent grant in both directions; the default for infra
  tooling enterprises adopt.
- **CLI first**, as a long-running session. Editor-agnostic by construction; a
  thin VS Code client over the same IPC is possible later and is not a fork.
- **One working copy, no worktrees.** A task is a temporal state of the repo
  against a recorded base commit. Real app stacks can only host one checkout.
- **Hybrid model access.** An **Explorer** shells out to the learner's own agent
  CLI for author-time generation and repo mining; a **Judge** calls the API
  directly, with no filesystem access, for learn-time judgment.
- **Structured verifiers.** Generation emits test files and command selections
  interpreted by the harness — never shell. Operations are shell, but they are
  human-authored and code-reviewed.
- **Instance-scoped profiles**, stored in the working copy. A tutor, not a
  university.
- **macOS and Linux natively; Windows via WSL2 or a devcontainer** — which is
  where enterprise Node development actually happens.
