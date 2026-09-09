# ADR 0001: Demonstration bars are counts of passed tasks, by kind

- **Status.** Proposed
- **Date.** 2026-09-09

## Context

The roadmap splits the trajectory in two (§ 2.2): the author defines the
destination set, the prerequisite edges, and *per node, what has to be
demonstrated before it counts as satisfied*; the harness decides, per
learner, which node to serve, which task, and *whether the evidence so far
satisfies the node or another task is needed*. Both sentences are about
the same moment — the one where a node flips to satisfied — and the
format has to say which side owns it and in what terms.

Three constraints bind. The deterministic loop (M3) must settle
satisfaction with no model call and no API key: "the whole product minus
judgment, and it runs with no API key" is that milestone's claim. The
judge (M4) arrives later and its verdicts are the only thing that moves
the skill graph — the coach's observations never do — so whatever the bar
counts must be something a verdict produces. And the format is being
fixed now, hand-authored, before either consumer exists (M2's plan:
"bars are designed for deterministic evaluation now precisely so M3
doesn't reopen the format"), so the shape has to be one an author can
write in a line and a loop can evaluate in one.

## Decision

A demonstration bar is a map from a kind of task to a minimum count of
passed tasks of that kind at the node. The kinds are the task ladder the
landscape survey took from repo-learner-suite — `use`, `modify`, `debug`,
`create`, `compare` — and one more, `tasks`, that counts passed tasks at
the node regardless of kind. Each entry is an independent condition; the
bar is met when every entry holds. A task counts once, toward the single
node it is keyed to, and only with a passing verdict.

```yaml
bar:
  modify: 1
  debug: 1
```

The bar is the author's and it is required on every node; there is no
default, because a default would be the harness deciding what competence
means. The ladder's identifiers are defined once in code and shared by
the skill loader (which validates bar keys) and the task loader (which
validates a task's kind, 2.3), so the two cannot drift.

Satisfaction is a count. Judgment lives in the verdict: when the judge
(M4) adjudicates a task, it decides whether *that task* passed — style,
idiom, house convention, on the diff alone — and the bar reads the
result. The harness's latitude over "how much evidence satisfies a node"
is exercised there, one task at a time, and nowhere else.

## Consequences

**Made easy.** M3 evaluates a node by counting rows in the profile's
evidence, with no model call. The check is the same on every machine and
every run, and a learner can see exactly why a node is or is not
satisfied. An author writes a bar in one to three lines, and a reviewer
reads it as a sentence: "one modify and one debug, here".

**Made checkable.** A bar names kinds, and the task pool is declared
per node (2.3), so a bar the pool cannot meet — a node demanding a `debug`
task with no debug task keyed to it — is a load-time error, not a learner
stuck forever. The task loader should make that check.

**Made hard.** The bar cannot express quality, breadth, or recency: it
cannot ask for "a passing task that touched the schema" or "two tasks a
month apart". Anything of that shape has to be expressed as a task kind
or a task's own verifier, or it cannot be expressed. Authors who want a
finer instrument will find the format blunt.

**Foreclosed.** The judge never decides satisfaction directly; a
"judge says this node is done" path would put a model call on the
deterministic loop's critical path and make M3's claim false. Bars never
reference other nodes' evidence; that is what edges are for. And there is
no decay: a met bar stays met unless the profile's mutation rules (2.4)
say a verdict can change, which is a question about verdicts, not bars —
consistent with the roadmap's rejection of any decay model (§ 6).

**Cost.** The ladder becomes load-bearing earlier than planned: 2.3 must
use exactly these five kinds, and adding a sixth is a format change to
both loaders. And `tasks` alongside the five kinds gives two ways to ask
for one task of any kind (`{tasks: 1}` versus five keys), which is a
small redundancy accepted for the sake of the common case reading
plainly.

## Alternatives considered

**A single count** (`bar: 2`). Simplest possible form, and it is the
`tasks` key alone. It lost because "demonstrated" is about the kind of
work — a node satisfied by two `use` tasks has not shown that the learner
can change the code — and the ladder already exists to say so.

**A rubric the judge evaluates.** The author writes prose — "can explain
the poll lifecycle and safely change it" — and the judge decides from the
evidence whether it is met. This is the richest form and the most honest
about what competence is. It lost on the constraints: M3 could not
evaluate it, a learner could not predict it, and it would put the model
on the path that the coach-never-grades seam keeps clean. The prose
belongs in the node's body, for humans.

**Weighted or scored evidence.** Verdicts carry a score, bars a
threshold. It lost because the deterministic loop produces pass/fail, and
a score would come from the judge alone — the rubric alternative with a
number on it.

**No bar — satisfied on the first passing task.** The zero-authoring
option, and what a `{tasks: 1}` default would amount to. It lost because
the design gives the author the bar on purpose: what has to be
demonstrated is part of defining the destination, not a routing detail.

**Bars on edges rather than nodes.** "To move from A to B, demonstrate
X." It lost because a node's satisfaction would then depend on which
edge the learner arrived by, and two learners reaching the same node by
different paths — the whole point of routing — would be held to
different standards.
