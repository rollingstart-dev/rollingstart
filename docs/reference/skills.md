# `skills/`

The skill graph — the author's trajectory, written down. It lives at
`.rollingstart/skills/`, one Markdown file per node, alongside everything
else in the storage layout described in [`docs/roadmap.md`](../roadmap.md)
§ 2.5. The author defines where a learner ends up: which nodes constitute
competence in this codebase (the destination set), what must come before
what (prerequisite edges), and what has to be demonstrated before a node
counts as satisfied (its demonstration bar). The harness owns the route
through that graph, per learner, and never edits it (§ 2.2).

This documents the format as it arrives with the formats milestone (M2).
Nothing routes over the graph yet — that is the session loop (M3). What
exists now is the format, a loader that rejects a broken graph with errors
an author can act on, and `rolling doctor` reporting the result. The bars
are shaped now so that M3 can evaluate them without reopening the format:
[ADR 0001](../decisions/0001-demonstration-bars.md) records that decision.

## A node file

```markdown
---
title: Poll data model
requires:
  - "[[local-dev-setup]]"
  - "[[prisma-schema]]"
bar:
  modify: 1
  debug: 1
---

Rallly's poll, its options, its participants and their votes — the tables
everything else joins to, and the shape a contributor has to hold in their
head before any change to voting is safe.
```

Two parts. The **frontmatter** — YAML between a `---` line that opens the
file and the next line that is exactly `---` — is the part the harness
reads, and it is strict: every field below is checked, and anything else
is an error. The **body** is Markdown prose for people: what the node is,
why it matters here, what a learner will be able to do once it is
satisfied. The harness never reads the body. That is not a limitation of
this milestone, it is the seam: nothing in the loader knows what any node
teaches, and keeping the loader off the body makes that true by
construction rather than by discipline. Write the body for the human who
reviews the graph in a pull request, and for the learner the title is
shown to.

Line numbers in errors count from the top of the file, the opening `---`
included, so a position names the line an editor shows.

### The slug

A node's identity is its file name without the `.md`: `poll-data-model.md`
is the node `poll-data-model`, and that slug is what a `[[link]]`, the
destination set, a task (2.3), and a learner's evidence (2.4) all use to
name it. Renaming a node is renaming a file, which a pull request shows as
exactly that.

Slugs are lowercase kebab-case: one or more runs of `a`–`z` and `0`–`9`
joined by single hyphens, nothing else. The rule is portability, not
taste. Case-insensitive filesystems — macOS by default — resolve
`[[Poll-Model]]` and `[[poll-model]]` to the same file, and Linux does
not, so a graph that loads on the author's laptop would fail on a
learner's machine. That is the silent kind of wrong, so the loader rejects
the file name rather than the link.

The directory is flat, and every entry in it that does not begin with a
dot is a node file: a `README.md` is a node whose slug fails the rule, a
`drafts/` directory is an error, and a `poll-model.markdown` is an error
too — say what a thing is, or keep it out of `skills/`. Entries beginning
with a dot (`.obsidian/`, `.DS_Store`) are ignored, so opening the
directory as an Obsidian vault does not break the graph.

### Frontmatter fields

- **`title`** (required) — the node's name as a learner sees it. A
  nonempty string; leading or trailing whitespace is rejected, as
  everywhere in the instance definition.
- **`requires`** (optional) — the node's prerequisites, as a list of
  `[[slug]]` links. See [Edges](#edges) for what an edge means. Each entry
  is exactly `[[` + a slug + `]]`: no alias (`[[slug|Text]]`), no heading
  (`[[slug#Part]]`), no padding — the title comes from the node it names,
  and a link is a reference, not a caption. A slug that names no node, a
  node listed twice, and a node requiring itself are each errors. An
  empty list declares nothing, exactly like an absent key.
- **`optional`** (optional, default `false`) — marks a node off the
  required spine. See [Optional nodes](#optional-nodes) for the rules it
  has to satisfy.
- **`bar`** (required) — the demonstration bar: what has to be
  demonstrated before the node counts as satisfied. See [Demonstration
  bars](#demonstration-bars).

Unknown fields are errors with a position, like unknown keys in
`instance.toml`: a misspelled `requries` would otherwise declare a node
with no prerequisites, and nothing would look broken. YAML's own faults —
a duplicated key, a tab where a space was meant, an unclosed `{` — are
positioned the same way.

One wart of the dialect, worth knowing before it costs a minute: the
links are quoted above because an unquoted `[[local-dev-setup]]` is YAML
for a list containing a list. The loader reports that as a type error at
the position, and the fix is the quotes.

## Edges

An edge is declared on the node that depends. `requires` on
`poll-data-model` naming `prisma-schema` means *`prisma-schema` is a
prerequisite of `poll-data-model`*: the harness serves the prerequisite
first, and offers the dependent only once every node it requires is
satisfied. Read the link the way git reads a parent pointer — a node knows
what it builds on, never what builds on it. To find what depends on a
node, grep for its slug.

The graph is a directed acyclic graph. A cycle is an error naming every
node on it, because there is no order in which the nodes on a cycle could
ever be served. Nodes with no prerequisites are where a learner can start.

Edges carry no weights, and nodes carry no ordinal — no `order`, no
`weight`, no `after`. The prerequisite structure is the whole of what the
author says about sequence, and everything else is the harness's choice
per learner. That is deliberate: routing that collapses to a fixed order
is the roadmap's most likely silent failure (§ 4), and a format that
offers an ordering field would hand it the excuse.

## The destination set

The nodes that constitute competence here — the author's definition of
done — are declared in exactly one place, so that "what does competence
in this codebase mean" is one reviewable list rather than a flag scattered
across files. That place is `instance.toml`
([`instance-toml.md`](instance-toml.md)):

```toml
[skills]
destination = ["poll-data-model", "poll-api", "email-notifications"]
```

Plain slugs, not links: `[[…]]` is Markdown's idiom for an edge, and this
is TOML naming members of a set. Each entry must name an existing node,
and a node listed twice is an error.

The **spine** is the destination set together with every node reachable
from it through `requires`, transitively — the nodes every learner must
satisfy to arrive. Everything else is off the spine and must say so (see
below). Two states are errors rather than valid-but-empty:

- `skills/` holds nodes and no destination is declared. A graph with no
  destination is a syllabus with no end, and the harness has nowhere to
  route toward.
- A destination is declared and `skills/` does not exist, or is empty.
  Every entry is then a slug naming no node, reported as such.

An instance with no `skills/` directory *and* no `[skills]` table has no
graph, which is valid: a v0 instance is not broken, it is smaller. An
empty `skills/` directory is the same as an absent one.

Every destination must be **reachable**: a learner starting from nothing
can satisfy its prerequisites in some order. Given the rules on this page
that reduces to three checks the loader already makes — every link
resolves, no cycle exists, and no node on the spine is marked optional —
because an optional node is opened only on evidence, and a destination
that waits on one may never be offered. There is no separate reachability
error; those three are what unreachable means.

## Optional nodes

An optional node hangs off the spine: a detour the harness may open for a
learner who demonstrates a gap or an aptitude the spine does not cover —
a deeper look at jsonb semantics for the learner who keeps tripping over
it, a performance node for the one who is done early. What opens one is
routing's decision and arrives with the loop (M3) and the judge (M4).
What the format fixes now is the marking, and the structure it has to
respect:

- A node on the spine is never optional. Marking a destination, or any
  node a destination requires, `optional: true` is an error naming the
  destination that needs it.
- A node off the spine is always optional. A node that is neither a
  destination, nor required by one, nor marked optional is an error: the
  harness would never serve it, and the author almost certainly meant one
  of the three — add it to the destination set, require it from a node
  on the spine, or mark it optional.

Optional nodes may require spine nodes, other optional nodes, or nothing
at all. What they may not do is be required by the spine, which the first
rule covers.

## Demonstration bars

A bar says what a learner has to demonstrate at this node before it counts
as satisfied. It is per node, it is the author's, and it is evaluated by
counting — no model call — so that the deterministic loop (M3) can settle
satisfaction on its own. The decision and its alternatives are in
[ADR 0001](../decisions/0001-demonstration-bars.md).

```yaml
bar:
  modify: 1
  debug: 1
```

A bar is a map from a kind of task to a minimum count of passed tasks of
that kind, all of them at this node. The kinds are the task ladder from
[`docs/landscape.md`](../landscape.md) — **`use`**, **`modify`**,
**`debug`**, **`create`**, **`compare`** — plus **`tasks`**, which counts
passed tasks at the node regardless of kind. Every entry is a separate
condition, and the bar is met when all of them hold: the one above wants
at least one passed `modify` task and at least one passed `debug` task;
`{tasks: 2}` wants any two; `{tasks: 3, debug: 1}` wants three, one of
them a debug. A task counts once, toward the node it is keyed to (2.3),
and only with a passing verdict — how a verdict is recorded and whether it
can change is the profile's rule (2.4), and the bar reads what the profile
holds.

The bar is required, and it must ask for something: an empty `bar: {}`
declares that nothing need be demonstrated, which is never what an author
means. Each count is a whole number of 1 or more, written as a plain
integer — `1.5`, `"1"`, and `0` are rejected. A key outside the six is an
unknown field, with a position.

The local-dev-setup node the roadmap has promised since M1 (§ 2.6) is an
ordinary node under this rule: a bar of `{use: 1}`, met by the one task
keyed to it, whose verifier is doctor's instance section going green
(2.3).

## What the loader rejects

The whole graph loads or none of it does: a `skills/` directory with one
broken file is a broken graph, because a committed-and-broken definition
blocks every learner, and a partial graph would route around the hole in
silence. Faults are reported the way `instance.toml`'s are — every fault
found, one line each, in a fixed order, so a graph is fixed in one pass
rather than one fault per run. Two phases, because the second needs the
first:

1. **Files.** Every node file, in slug order. A file that does not decode
   — no frontmatter, an unknown field, a type mismatch, a YAML fault —
   reports the decoder's positioned message and nothing more about that
   file. A file that decodes reports every value fault its fields have,
   in field order. All files are reported together.
2. **Graph.** Once every file loads: dangling links, then cycles, then the
   destination set, then the spine rules — each class in slug order, and
   a fault in one class never repeats as another.

A file that does not decode has no fields to check and no edges to walk,
so a key typo and a dangling link are two runs, not three.

Decoder faults carry `file:line:col` and, except for the unknown-key
case worded above, the decoder's own message; value and graph faults name
the file and the field, the same shape as `instance.toml`'s value checks.
A cycle is reported once, on the first of its nodes in slug order. What
the report looks like:

```
.rollingstart/skills/poll-data-model.md:3:1: unknown key "requries"
.rollingstart/skills/poll-api.md: requires "[[poll-data-modle]]" names no node
.rollingstart/skills/prisma-schema.md: prerequisite cycle: prisma-schema → poll-data-model → prisma-schema
.rollingstart/instance.toml: skills.destination "emails" names no node
.rollingstart/skills/jsonb-columns.md: marked optional, but destination poll-api requires it (through poll-data-model)
.rollingstart/skills/turbo-cache.md: neither a destination, nor required by one, nor marked optional: add it to skills.destination, require it from a spine node, or mark it optional
```

Duplicate slugs need no rule of ours: the slug is the file name, and a
filesystem holds one file per name — which is the reason the slug rule
insists on a case the filesystem cannot fold.

## In `rolling doctor`

The graph is part of the instance definition, and a broken one blocks
every learner, so doctor validates it in the harness section
([`rolling-doctor.md`](rolling-doctor.md)) — the same section that reports
whether `instance.toml` loads, and red on the same terms. One row,
`skill graph`, after `instance definition`:

- **No graph** — no `skills/` and no destination: `ok`, reading
  `no graph declared`. Distinct from a loaded graph, never red.
- **A graph that loads** — `ok`, with its census:
  `skill graph loaded (9 nodes, 3 destinations, 1 optional)`. The
  optional count is omitted when it is zero.
- **A graph that does not** — `FAIL`, with every fault from the list
  above, verbatim, one per line.
- **A definition that did not load** — the destination set is declared
  in `instance.toml`, so with `skills/` present and the definition red
  the graph cannot be checked: `FAIL`, reading
  `not checked: the instance definition did not load`. With no `skills/`
  the row stays `no graph declared`; whether the unloaded definition
  names a destination is found on the next run, once it loads.

That is also the authoring loop: write a node, run doctor, fix what it
names. Nothing runs, routes, or serves a task in this milestone.

## Decisions recorded here

Format questions the roadmap left open, settled by this page:

- **The destination set lives in `instance.toml`.** A per-node
  `destination: true` would scatter the one list that says what
  competence means across as many files as it has members, and reviewing
  a change to it would be a grep. The roadmap's storage layout put
  declarations in `instance.toml` and the graph in `skills/`; the
  destination set is a declaration *about* the graph, and goes with the
  declarations.
- **Edges live in the frontmatter, and the body is never read.** A
  `[[link]]` in the body is prose — a *see also*, a pointer to a sibling
  a learner might want next — and treating every body link as an edge
  would make that impossible to write. The body is the author's, and
  keeping the loader out of it is what makes the "nothing in the loader
  knows what a node teaches" seam mechanical. Body links are therefore
  unchecked in this version; a dangling one is an editing lapse, not a
  load error.
- **Optional is declared, and checked against the spine, rather than
  derived.** The spine can be computed from the destination set alone,
  which would make an `optional` flag redundant — and would also make a
  node the author forgot to connect silently optional. Declaring the flag
  and rejecting disagreement turns both mistakes into named errors.
- **`[[slug]]` in Markdown, bare slugs in TOML.** Each file uses its own
  idiom for naming a node: the link syntax is what Markdown tooling
  renders and follows, and it means nothing to TOML.
- **No legacy marking, no per-node task list.** Which code is legacy is a
  corpus pointer and waits on the judge (M4);
  which tasks serve a node is declared from the task's side (2.3), so a
  node never has to be edited when its pool changes.
