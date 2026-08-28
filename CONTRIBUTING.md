# Contributing to Rolling Start

Thanks for looking. This is pre-alpha — the design is settled, the implementation
is early. [docs/plan.md](docs/plan.md) carries the decision record and the
milestone sequence, and reading it first will save you time.

## Licensing

Contributions are accepted under [Apache-2.0](LICENSE), the project's license.
There is no CLA and no copyright assignment: Apache-2.0 §5 already places
inbound contributions under the same license, and §3 carries a patent grant from
contributors. Nothing to sign, nothing to wait on.

## Building

```
go build ./cmd/rolling
go test ./...
go vet ./...
```

## Where to contribute

**The engine** is `cmd/rolling` and `internal/*` — orchestration, state, and
verification. It never parses a target language.

**Agent adapters** are the seam most likely to want your help. Rolling Start
shells out to whatever coding agent the learner already has. One adapter ships in
v1; the interface exists so you can add yours.

**Instances** are the Rolling Start for a specific codebase — commands,
operations, a skill graph, and corpus pointers. They live with the repo they
teach, not here.

## Commit messages

Write commit bodies you'd want a new hire to learn from. That isn't a stylistic
preference: Rolling Start generates exercises from git history, and this
repository is meant to be its own instance. A `wip` commit is a lesson we can't
teach.

## Code of conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md).
