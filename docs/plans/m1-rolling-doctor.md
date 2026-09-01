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

### 1.1 — Instance configuration [COMPLETE]

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

- [x] Table-driven tests cover: minimal valid, all four commands, zero commands,
      unknown key, TOML syntax error, missing file, empty command string
- [x] Error messages for the failure cases include position information
- [x] `gofmt`, `go vet`, `go test ./...` clean

---

### 1.2 — Command runner [COMPLETE]

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

- [x] Tests exercise all four outcomes using synthetic commands (`true`,
      `false`, a nonexistent binary, a sleep against a short timeout)
- [x] Output-bounding behavior tested: oversized output keeps the tail
- [x] Tests pass on both Linux and macOS in CI
- [x] `gofmt`, `go vet`, `go test ./...` clean

---

### 1.3 — Harness probes [COMPLETE]

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

- [x] Probes tested against temp git repos in every state they classify
      (no repo, dirty tree, each autocrlf value, each config failure)
- [x] Watcher probe test proves both the event-received and the timeout path
- [x] A probe run on this repository itself comes back all green
- [x] `gofmt`, `go vet`, `go test ./...` clean

---

### 1.4 — `rolling doctor` command and report [COMPLETE]

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

- [x] End-to-end tests (separate `e2e` package) run the built binary against
      fixture repos covering: harness-red, harness-green/instance-red,
      all-green, no-config, zero-commands
- [x] Exit codes verified for each fixture state
- [x] Docs describe the output actually produced
- [x] `gofmt`, `go vet`, `go test ./...` clean

---

### 1.5 — Validation against Rallly [COMPLETE]

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

- [x] Both transcripts (broken-env and healthy-env) attached to the PR
- [x] Every red line in the broken-env transcript names its actual cause
- [x] Follow-up issues filed for anything not absorbed
- [x] `gofmt`, `go vet`, `go test ./...` clean

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

- [x] The roadmap exit criterion, verbatim: fresh Rallly clone, no pnpm, no
      stack → harness-green, instance-red, failures named accurately; install
      pnpm, stack up → second section green
- [x] Every failure line in the broken-environment run names its actual cause —
      nothing generic, nothing misattributed
- [x] The watcher probe's failure path is exercised in tests, honoring the
      silent-WSL2-watcher risk this milestone exists to mitigate
- [x] CI green on Linux and macOS: `go build`, `go vet`, `go test ./...`,
      `go mod tidy -diff`
- [x] No LLM call anywhere in the milestone

## Retrospective

Written at closure (#25), 2026-09-01. M1 ran 2026-08-28 to 2026-09-01:
eleven PRs of milestone work merged plus the closure's own two, sixteen
issues closed by the end, no LLM call anywhere.

### Planned vs. delivered

All five sub-scopes shipped, none as a single PR. 1.3 split into the
contract-and-git-probes half (#8 → #10) and the watcher (#9 → #12); 1.4
split three ways — report model (#15 → #18), e2e harness (#16 → #19),
command (#17 → #20) — with the harness promoted to a slice of its own,
which 1.5 then reused unchanged. Three follow-ups the plan did not
anticipate: the gofmt gate (#13 → #14), after review noticed a documented
gate nothing ran; the GIT_DIR notice (#21 → #22), a product decision
pulled out of a review finding; and #23, the config-relocating sibling,
closed as not planned with the distinction recorded — state-relocating
variables made the report about a different repository and earned a note,
config variables change behaviour git itself exhibits and the probes
measure truthfully. The repository also became its own first instance
(`.rollingstart/instance.toml`, #12), which the plan implied but never
scheduled.

### Decisions made in flight

None met the ADR threshold; each lives in one package or one reference
page. The ones a future reader will want: an inotify queue overflow is
delivery evidence, not failure — green with the overflow named (#12); the
report's two vocabularies, ok/FAIL against healthy/failing/not
installed/timed out, because one blocks and the other informs (#18); exit
2 for the caller's own mistakes, made true rather than reworded when
review measured the documented table lying (#20); paths in findings are
relative once a repository is found, so the report names what a learner
can type (#20); overrides are followed, never fought and never silently
(#22, maintainer decision on #21); the Rallly example declares unit
tests, not the integration suite — stack-bound suites are operations,
which arrive with M2 (#24).

### What went wrong

Candour is the point of this section.

- **A red branch reached the remote** (#18). The pre-push gate piped
  `go test` through `grep`, the pipe's exit status masked a failing
  golden, and the push went out with the suite red. Caught minutes later
  by reading, not by process; repaired by follow-up commit, since pushed
  history is not amended here. The Conventions gate line now requires
  each tool's own exit status (#28).
- **A review experiment committed to the live branch** (#19). The
  pre-push reviewer proved the milestone's most important finding — an
  inherited GIT_DIR redirects the e2e fixtures at the host repository —
  by running that exact experiment against the live checkout: a 52-file
  deletion committed onto the branch under review, restored by hand. The
  finding shipped as the process-wide environment scrub; the incident
  shipped as the scratch-copy rule (#28). The same finding, followed to
  the product side, became #21/#22.
- **A spec was committed straight to main** during refinement of #4. The
  direct-to-main rule as first written listed examples that read as a
  lane; the maintainer's correction — the exception is for changes where
  a PR would be needless ceremony, and a new spec always deserves the
  ceremony — produced the rule's current wording and refine-issue's
  lands-through-a-PR requirement. The spec itself stayed; the process
  around it changed twice in one day, which is what public process
  iteration looks like.
- **The #4 closeout half-fired.** A broken scratch-path fallback closed
  the tracking issue with an empty comment and committed nothing; the
  retried commit then swallowed a staged-but-unrelated skill edit under
  the wrong message, already pushed. Repaired honestly — back-out and
  re-land — and the shell lessons went to the working agent's memory.
- **Two parents closed without their plan flips** (#1, #2), found only
  by #3's closeout sweep. Nothing scheduled the closeout: the trigger
  was someone remembering. implement-issue now opens with a
  closeout-debt sweep and last-sub-issue PRs carry the reminder in
  their bodies.

### Deferred, and where it went

Machine-readable doctor output, `ParseError.Detail()` excerpts, and the
e2e environment allowlist wait for consumers (the reference page and the
#19/#22 review threads record the reasoning). GIT_CEILING_DIRECTORIES
and GIT_COMMON_DIR were declined with reasons on #22's review and #23.
Operations — including Rallly's integration suite and db rituals — are
M2's, as planned.

### Patterns to carry forward

- **The pre-push Opus review earned its keep on every PR that had one**:
  a blocking lint idiom that reported success with the tool missing
  (#14), four renderer edge cases (#18), the fixture-isolation hole
  (#19), the symlinked-cwd break and the exit-table lie (#20). Nothing
  it flagged and we took later proved wrong; two things it suggested and
  the maintainer declined (#22) stayed declined without regret.
- **Docs as spec, mechanically enforced.** The reference page's example
  blocks are renderer output by test; the validation recipe in the
  Rallly README is the transcript-producing script, verbatim. Drift has
  nowhere to hide.
- **The e2e package tests the binary's contract** and imports nothing
  from internal/ — three review rounds of fixtures later, it is the
  closest thing the repo has to a user.
- **Validation as a reproducible, side-effect-free recipe** (maintainer
  requirement, #24): a throwaway directory, an official image, the
  target's own compose file. Three runs, three environment lessons, zero
  doctor gaps.

### Patterns to change

Both changes shipped before this retrospective (#28): gates check exit
codes, and reviewers get scratch copies. The remaining habit worth
naming has no rule yet because it may not need one: every incident above
that was not caught by review was caught by reading output rather than
trusting a green-looking script — the discipline the exit-code rule
mechanises for the one place it burned us.
