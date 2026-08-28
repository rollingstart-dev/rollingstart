# Rolling Start

**Turn a repository into an adaptive tutor.**

A rolling start means the field is already at speed when the flag drops. You are
already a skilled engineer — Rolling Start doesn't teach you to program. It gets
you up to speed in *this* codebase.

> **Status: pre-alpha.** The design is settled and the skeleton builds. Nothing
> teaches anything yet. See [docs/plan.md](docs/plan.md) for the milestone this
> is on.

## What it is

An open source teaching harness: a deterministic runtime wrapped around an LLM
coding agent that turns a repository into a tutor for the people who have to work
in it.

An instance author defines the destination — what competence in this codebase
looks like. Rolling Start finds each learner's route there: one task at a time,
drawn from the repo's own git history, evaluated on the diff when the learner
says they're done, with a persistent model of what they know deciding what comes
next.

Two people reach the same destination by different paths.

## Who it's for

In-house development teams at large organizations, frequently not tech companies
— an industrial, public-sector, or education org modernizing legacy applications
and needing to onboard existing staff onto a codebase they had no hand in
building.

## Why this exists

There are good tools for understanding a codebase. Greptile indexes your repo and
answers questions about it. Unblocked connects a line of code to the Slack thread
that explains it. DeepWiki turns a repository into a browsable wiki. They work,
and if what you need is an answer, use them.

They share three assumptions Rolling Start rejects.

**That understanding is the goal.** You can read an excellent explanation of a
subsystem and remain unable to fix a bug in it. The feeling of understanding is
cheap to produce and unreliable as a signal. The only test that means anything is
whether you can do the work — so Rolling Start doesn't explain the codebase to
you. It puts you in it, gives you a real task, and checks what you actually
changed, starting with the compiler and the repo's own test suite.

**That the tool should be permanent.** Ask a comprehension tool a question, get an
answer, ask again next week. Nothing accumulates in *you*. That isn't an
engineering failure, it's a business model — per-seat-per-month requires that you
keep needing it. Rolling Start's success condition is that you stop using it.
It's an on-ramp, not a place to live. If getting someone productive takes more
than a few months, it failed.

**That every developer is the same developer.** Those tools answer identically no
matter who is asking. There's no model of you, so nothing gets skipped because
you already know it and nothing gets revisited because you keep getting it wrong.
Rolling Start keeps a durable record of what you've actually demonstrated and
routes around what you've already proven.

> They explain the code to you. Rolling Start makes you change it, then checks
> whether you got it right.

## How it works

- **You keep your editor.** Rolling Start watches the working copy and evaluates
  a diff. vim in tmux and VS Code are the same integration: none.
- **It runs where your dev environment runs.** Bare metal, a container, a
  devcontainer, WSL2. It never orchestrates your stack, assumes a stack, or needs
  an accommodation to install.
- **Deterministic first.** Compiler, tests, and linter run without an API key.
  LLM adjudication layers on top for idiom and house convention, and degrades to
  "skipped" without one.
- **The coach isn't the grader.** A coach runs alongside you and offers. A grader
  runs only when you say done, in fresh context, on the diff alone. The tutor who
  watched you struggle never judges the result.

## Documentation

- [Design](docs/design.md) — what it is and the principles behind it
- [Plan](docs/plan.md) — decision record, architecture, milestones, risks
- [Landscape](docs/landscape.md) — what else exists and why this is different
- [Contributing](CONTRIBUTING.md)

## License

[Apache-2.0](LICENSE).
