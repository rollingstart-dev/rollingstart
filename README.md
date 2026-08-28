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
- [Contributing](CONTRIBUTING.md)

## License

[Apache-2.0](LICENSE).
