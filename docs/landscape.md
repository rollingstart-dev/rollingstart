# Landscape

*Surveyed 2026-08-28. Specific products will go stale; the reasoning should
outlive them.*

## The short version

The busy category is codebase **comprehension** — tools that answer questions
about a repository. The products are good and there are a lot of them. Not one
assesses whether you can do the work.

From the opposite direction, technical assessment products test generic
algorithm problems disconnected from any real codebase.

Nobody occupies the middle: assessment against the repository you actually have
to work in.

## Commercial

| Tool | What it does | What it doesn't |
|---|---|---|
| **Greptile** | Indexes a repo into a file/function graph; PR review and a query API | Assess anything, or model the reader |
| **Unblocked** | Answers "why was this built this way" by linking code to git history, Slack, and Jira | Ask you to prove you understood the answer |
| **DeepWiki** | Turns a repo into a structured wiki with Mermaid architecture diagrams and Q&A | Know whether you read it |
| **Sourcegraph Cody** | Cross-repository comprehension at enterprise scale | Teach; it assists |
| **Driver.ai** | Enterprise codebase explanation | Verify |
| **CodeSignal / HackerRank / Karat** | Real assessment, with rubrics and scoring | Use *your* codebase; the problems are generic |

## Open source

**`PranitMohnot/repo-learner-suite`** — conceptually the closest work that
exists. Curricula, exercises, Socratic tutoring, adaptive quizzes, progress
tracking, adapters for Claude Code / Codex / Gemini, and a language-adapter
contract. Structurally it is five `SKILL.md` prompt files and four Python helper
scripts. No test-runner integration, no diff evaluation, no verification;
exercises are notebooks and the strongest check is whether the notebook
executed. It is the idea without the harness — which is precisely the part we
argue an LLM does unreliably alone.

**`kirilxd/claude-tutor`** — the most popular thing nearby. A general-purpose
topic tutor inside Claude Code with SM-2 spaced repetition and a web dashboard.
Not codebase-specific.

**`CarsonDavis/codebase-tutorial`** — agent pipeline generates tutorial data, a
web app renders it with a quiz. Explanation plus recall, not doing.

**`divar-ir/ai-doc-gen`** — multi-agent documentation generation. Adjacent to
our corpus mining, aimed at output rather than assessment.

A GitHub search for exercise generation from git history returned nothing.

## Prior art in the literature

*Evaluating Contextually Personalized Programming Exercises Created with
Generative AI* (arXiv 2407.11994) studies LLM-generated exercises graded by
LLM-generated tests — our self-verifying generation, examined empirically. Read
before M6.

## Ideas worth taking

- **The exercise ladder.** repo-learner-suite progresses `Use → Modify → Debug →
  Create → Compare`, one new concept per exercise. A cleaner node-type taxonomy
  than we had.
- **Corpus breadth.** Unblocked links code to Slack and Jira, not just git. Our
  corpus is commits and PRs; issue trackers and chat are a richer seam, and
  uceap3 already has Jira wired into its devcontainer.
- **Architecture diagrams.** DeepWiki's generated Mermaid maps are a good
  orientation aid, and cheap to produce from a corpus we already parse.

## Ideas deliberately not taken

**Spaced repetition.** claude-tutor schedules reviews with SM-2, and our profile
has no decay model. That is intentional. Rolling Start is an on-ramp, not a
residence: the goal is self-sustaining proficiency, and then you leave. If
onboarding takes longer than about three months we have failed, and a learner
still being drilled a year in is a symptom, not a feature. Keeping people
abreast of a codebase as it evolves is a different product.

## Why we are not joining an existing project

There is nothing to join. The nearest neighbour is a prompt suite with four
stars and no harness, aimed at Python notebooks. The gap it leaves —
deterministic verification, a durable learner model, and evaluation of real work
in a real working copy — is the entire thing we are building.
