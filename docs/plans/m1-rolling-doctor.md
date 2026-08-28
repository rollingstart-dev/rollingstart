# M1: `rolling doctor`

> Expanded plan for the `M1: rolling doctor` milestone. The roadmap entry is
> [`docs/roadmap.md`](../roadmap.md) § M1.
>
> This file is the drafting artifact. Its sub-scopes are copied verbatim into
> GitHub issues; its remaining content becomes the project board's README. After
> the milestone ships, `/milestone-endgame` appends a retrospective.

## Context

M0 delivered the skeleton: a Go module, the `cmd/rolling` + `internal/` layout,
Cobra with Fang, Apache-2.0 with NOTICE, and CI running build, vet, and test on
Linux and macOS. `rolling version` runs. Nothing else does.

M1 delivers the first command that does real work: `rolling doctor`, the
readiness check. It loads the instance configuration, runs the probes that
decide whether the harness can operate at all, runs the instance's declared
commands, and reports against the two-section model in roadmap § 2.6 —
**harness preconditions**, where red is blocking, kept strictly separate from
**instance command health**, where red is informational, because a broken-looking
instance is frequently just a learner who hasn't done lesson one yet.

This is the formation lap: no racing yet, but the car, the track, and the
telemetry all get proven before anything depends on them. No LLM is involved
anywhere in this milestone.

**End state.** Pointed at a fresh Rallly clone on a machine with no pnpm and no
stack running — the exact state a real learner starts in — `rolling doctor`
reports harness-green with instance-red, and the instance failures are named
accurately ("pnpm is not installed") rather than reported as breakage. After
installing pnpm and bringing the stack up, the second section flips to green.
Along the way we gain the three internal capabilities everything later builds
on: config loading, a command runner with honest failure classification, and
environment probes including the synthetic-watcher check that guards against
the silent WSL2 failure mode.

## Key decisions

| Decision | Choice | Why |
|---|---|---|
| TOML library | `pelletier/go-toml/v2`, strict decoding | Position-aware errors let doctor point at the exact offending line, and rejecting unknown keys catches the worst config failure mode: a misspelled key silently ignored. |
| Git interrogation | Shell out to the `git` CLI, no Go git library | A working `git` binary is itself a harness precondition, so depending on it costs nothing — and the CLI is the compatibility ground truth for whatever exotic repo state a learner's machine holds. |
| Command execution | `sh -c` with the inherited environment, from the repo root | Declared commands are human-authored shell strings; macOS, Linux, and WSL2 are all POSIX. Inheriting the caller's environment means doctor sees the same PATH the learner's terminal does — essential, since "is pnpm on the PATH" is precisely the question. |
| Failure taxonomy | Runner distinguishes *not startable*, *ran and failed* (exit code), and *timed out*, and keeps output | The exit criterion requires failures named accurately. Exit 127 / spawn errors mean "not installed"; a nonzero exit with test output means "tests failing". These are different diagnoses and the runner must not collapse them. |
| Exit code | Nonzero only when the blocking section is red | Instance-red is a learnable state, not breakage — the local-dev-setup lesson's completion condition (roadmap § 2.6) *is* the second section going green. Scripts and CI can still gate on harness readiness. |
| `instance.toml` schema scope | v0 carries `[commands]` only (build, typecheck, test, lint; each optional) | Operations, corpus pointers, skills, and tasks have no consumer until M2/M3. Specifying schema nothing reads is guessing; M2 extends the schema when it validates the formats. |

None of these met the ADR threshold in
[`docs/decisions/README.md`](../decisions/README.md) — each lives comfortably in
one package and is fully re-derivable from this table.

## Sub-scopes

### 1.1 — Instance configuration [PENDING]

**Goal.** Load and validate `.rollingstart/instance.toml` into a typed config,
with errors precise enough for doctor to display verbatim.

**Branch.** `m1.1/instance-config`

**Depends on.** Nothing.

**Acceptance criteria.**

- A minimal `instance.toml` declaring any subset of the four canonical commands
  (`build`, `typecheck`, `test`, `lint`) loads into a typed config; the schema
  and file location (`.rollingstart/instance.toml` at the git root, per roadmap
  § 2.5) are documented before implementation, per the docs-driven workflow
- A config declaring zero commands is valid and loads; the caller can see that
  it is empty
- Unknown keys and malformed TOML fail with errors carrying position (line) and
  the offending key, suitable for verbatim display
- A missing file is distinguishable from an unparseable file — doctor will
  render these as different findings
- Empty or whitespace-only command strings are rejected at load time
- The package knows nothing about doctor, rendering, or what the commands mean —
  it loads config, full stop

**Verification.**

- [ ] Table-driven tests cover: minimal valid, all four commands, zero commands,
      unknown key, TOML syntax error, missing file, empty command string
- [ ] Error messages for the failure cases include position information
- [ ] `gofmt`, `go vet`, `go test ./...` clean

---

### 1.2 — Command runner [PENDING]

**Goal.** Run one human-authored shell command and report what happened with
enough fidelity to name failures accurately.

**Branch.** `m1.2/command-runner`

**Depends on.** Nothing.

**Acceptance criteria.**

- Runs a declared shell string via `sh -c`, inheriting the caller's environment,
  with the working directory set by the caller
- The result distinguishes four outcomes: succeeded; ran and exited nonzero
  (code preserved); not startable (missing interpreter or, via exit 127,
  missing binary — the "pnpm is not installed" case); timed out under a
  caller-supplied timeout, with the process group cleaned up
- Combined stdout/stderr is captured with a size bound that keeps the tail —
  the end of the output is where toolchains put the verdict
- Duration is recorded per run
- The runner is language-agnostic: nothing in it knows or cares what command it
  is running, per the engine seam rule

**Verification.**

- [ ] Tests exercise all four outcomes using synthetic commands (`true`,
      `false`, a nonexistent binary, a sleep against a short timeout)
- [ ] Output-bounding behavior tested: oversized output keeps the tail
- [ ] Tests pass on both Linux and macOS in CI
- [ ] `gofmt`, `go vet`, `go test ./...` clean

---

### 1.3 — Harness probes [PENDING]

**Goal.** Implement the blocking-section probes: git repository present,
working tree clean, `core.autocrlf` sane, `instance.toml` parses, and a
synthetic file event the watcher must observe.

**Branch.** `m1.3/harness-probes`

**Depends on.** 1.1 (the config-parse probe wraps the loader).

**Acceptance criteria.**

- Each probe returns a structured result — name, status, human-readable
  message — with no rendering in the probe layer
- Git probes (repo present, tree clean, autocrlf) shell out to the `git` CLI;
  an absent or broken git binary is itself a red probe result, not a crash
- `core.autocrlf` is sane when unset, `false`, or `input`; `true` is red with a
  message explaining why it corrupts a POSIX working copy
- The config probe maps the loader's distinct failures (missing, unparseable,
  invalid) to distinct findings, displaying the loader's positioned errors
- The watcher probe (fsnotify) watches a path **under the repo root** — the
  filesystem boundary is the thing under test — writes a synthetic file, and
  goes green only if the event arrives within a timeout; no event is red with
  a message naming the likely cause (e.g. a Windows-drive repo watched from
  WSL2 receives no inotify events, per the roadmap's top risk)
- The watcher probe leaves the working tree exactly as it found it, so it
  cannot trip the clean-tree probe

**Verification.**

- [ ] Probes tested against temp git repos in every state they classify
      (no repo, dirty tree, each autocrlf value, each config failure)
- [ ] Watcher probe test proves both the event-received and the timeout path
- [ ] A probe run on this repository itself comes back all green
- [ ] `gofmt`, `go vet`, `go test ./...` clean

---

### 1.4 — `rolling doctor` command and report [PENDING]

**Goal.** Wire config, probes, and runner into the two-section report, and ship
it as the `rolling doctor` command.

**Branch.** `m1.4/doctor-command`

**Depends on.** 1.1, 1.2, 1.3.

**Acceptance criteria.**

- Doctor's user-facing behavior — sections, statuses, exit codes — is
  documented before implementation; the docs are the spec
- Section one, **harness preconditions**, runs the five probes; any red makes
  the overall exit nonzero, rendered via the `errSilentExit` convention so Fang
  doesn't stack a styled block on the command's own output
- Section two, **instance command health**, runs each declared command through
  the runner with a per-command timeout and reports per command, naming the
  outcome accurately: not installed, failing (with the output tail), timed out,
  or healthy
- Instance-section red alone exits zero — informational, per roadmap § 2.6
- When config didn't load, the instance section reports itself skipped with the
  reason, rather than pretending the instance is healthy or broken
- A config declaring zero commands renders as an explicit "nothing declared"
  state, not an empty-therefore-green section
- Plain terminal output, readable without color; no TUI — Bubble Tea arrives
  with the session in M3
- Doctor never mutates anything it finds: it reports, it does not fix

**Verification.**

- [ ] End-to-end tests (separate `e2e` package) run the built binary against
      fixture repos covering: harness-red, harness-green/instance-red,
      all-green, no-config, zero-commands
- [ ] Exit codes verified for each fixture state
- [ ] Docs describe the output actually produced
- [ ] `gofmt`, `go vet`, `go test ./...` clean

---

### 1.5 — Validation against Rallly [PENDING]

**Goal.** Prove the milestone exit criterion against a real target, and feed
what breaks back into the tools.

**Branch.** `m1.5/rallly-validation`

**Depends on.** 1.4.

**Acceptance criteria.**

- A minimal hand-written Rallly `instance.toml` (commands only) exists in this
  repo as a documented example — the first config an instance author will see,
  so it reads like one
- On a fresh Rallly clone with no pnpm and no stack running: doctor reports
  harness-green, instance-red, with every failure named accurately rather than
  reported as breakage; the transcript is captured on the PR
- After installing pnpm and bringing Rallly's dev stack up (its own
  `docker-compose.dev.yml` — Rolling Start never orchestrates it): the second
  section flips green; that transcript is captured too
- Gaps surfaced by real output — misclassified failures, unreadable rendering,
  schema friction — are fixed here if they meet all three in-flight scope
  rules in [`docs/workflow.md`](../workflow.md), and filed as issues otherwise

**Verification.**

- [ ] Both transcripts (broken-env and healthy-env) attached to the PR
- [ ] Every red line in the broken-env transcript names its actual cause
- [ ] Follow-up issues filed for anything not absorbed
- [ ] `gofmt`, `go vet`, `go test ./...` clean

## Explicitly deferred

- **Operations** — no consumer until M2 declares them and M3 runs them. Doctor
  never runs operations; v0 schema omits them entirely.
- **`skills/`, `tasks/`, `profile/` formats** — M2's whole job. Hand-author
  against a validated repo before generating into an unvalidated format.
- **Corpus pointers in `instance.toml`** — M2, alongside the formats they point
  at.
- **Per-toolchain semantic output parsing** (failed-test counts, per-package
  vitest results) — doctor's exit criterion needs failure *classification*
  (not installed / failing / timed out), which process-level evidence provides.
  Deeper parsing lands when a consumer — the M3 verdict loop — exists.
- **JSON / machine-readable output** — no consumer yet.
- **TUI rendering** — Bubble Tea arrives with the M3 session. Doctor stays
  plain text.
- **The container-resident toolchain shape** (uceap3's `app` service) — M7
  exercises it deliberately. M1 targets Rallly, whose toolchain runs on the
  host.
- **Fix-it suggestions or remediation** — doctor reports; the setup *lesson*
  (M2/M3) is where fixing becomes the learner's task. Rolling Start never
  orchestrates the environment.
- **Native Windows** — roadmap § 5. WSL2 is the Windows story, and the watcher
  probe is exactly the guard that makes that story honest.

## Verification

End-to-end criteria for the milestone as a whole. Sub-scope 1.5 confirms the
first two directly.

- [ ] The roadmap exit criterion, verbatim: fresh Rallly clone, no pnpm, no
      stack → harness-green, instance-red, failures named accurately; install
      pnpm, stack up → second section green
- [ ] Every failure line in the broken-environment run names its actual cause —
      nothing generic, nothing misattributed
- [ ] The watcher probe's failure path is exercised in tests, honoring the
      silent-WSL2-watcher risk this milestone exists to mitigate
- [ ] CI green on Linux and macOS: `go build`, `go vet`, `go test ./...`,
      `go mod tidy -diff`
- [ ] No LLM call anywhere in the milestone
