# Decisions

Architecture Decision Records. Numbered, immutable once accepted, superseded
rather than edited.

These cover decisions made *after* the roadmap was written. The foundational
choices — language, licence, model access, storage — are in
[`docs/roadmap.md`](../roadmap.md) § 1 with their rationale and a reversibility
column, and they stay there.

## When to write one

All three must hold:

1. It affects more than the file you're editing
2. You'd have to re-derive it if you forgot
3. A reasonable person could have chosen differently

Everything else belongs in a commit body. Most decisions are not ADRs.

## Why "decisions" and not "RFDs"

The artifact is the same, but nobody is being asked to discuss anything at the
moment it's written — this is a small project. The discussion venue is the pull
request that lands the record, which is public and threaded and works fine the
day a contributor turns up. Naming the file after a conversation that hasn't
happened yet would be a small lie.

Start from [`0000-template.md`](0000-template.md).
