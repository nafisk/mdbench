# mdbench MVP implementation tasks

Status: Approved
Date: 2026-07-15

## Working agreement

Implementation is organized into five build stages. Each stage ends with one focused verification pass. Independently usable features receive focused Git commits, while user verification remains limited to the end of each stage.

Tests stay narrow for the MVP. We will cover score math, immutable hashes, storage recovery, security boundaries, and one complete product path. Broad snapshot matrices, exhaustive provider failures, and large compatibility suites are deferred until the product idea is proven.

## [x] Stage 1: Application foundation and artifact intake

Requirements: US-01, US-02, US-09, US-12; sections 7.1, 10, and 11.

Expected outcome:

- A buildable Go CLI opens the TUI when run without a subcommand.
- Config paths and precedence work without storing credentials.
- A Markdown file, skill folder, complete pasted skill, or instructions-only paste can be loaded, reviewed, checked, safely bundled, hashed, and persisted.
- The local JSON store uses schema versions, atomic writes, redaction, and startup reconciliation.

Likely files:

- `go.mod`, `go.sum`
- `cmd/mdbench/`
- `internal/app/`, `internal/model/`, `internal/store/`
- `internal/analyze/`
- initial `internal/tui/` shell and artifact screens

Dependencies: none.

Stage verification:

- Run focused tests for artifact hashing, secret blocking, and atomic storage.
- Manually open the TUI and complete file and paste intake once.
- Commit as `build foundation and artifact intake`.

## [x] Stage 2: Test suites and guided planning flow

Requirements: US-03, US-04, US-05; sections 4.2, 5.2 steps 3-6, and 6.

Expected outcome:

- Built-in fixture snapshots are available through a small fixture catalog.
- Codex can generate a schema-valid draft suite through a fake harness during normal development.
- The user can review cases, assertions, rubric anchors, weights, applicability, and hard-failure rules.
- Freezing creates an immutable suite revision and content hash.
- The TUI reaches a complete execution plan through generate, edit-revision, and reuse paths.

Likely files:

- `internal/suite/`, `internal/harness/`
- embedded fixture assets
- suite, fixture, configuration, review, and plan screens under `internal/tui/`
- suite schemas under an internal asset package

Dependencies: Stage 1.

Feature commits:

- `add built-in fixtures and suite contract`
- `add deterministic suite generation harness`
- `add guided suite review flow`
- `add immutable suite revisions and reuse`
- `add evaluation execution plan`

Stage verification:

- Run focused tests for suite validation, canonical hashing, and revision immutability.
- Manually walk the generate and reuse paths to the execution plan.

## [ ] Stage 3: Secure container runtime

Requirements: US-05 and US-06; sections 7.2, 11, and 12.

Expected outcome:

- Docker and Podman runtime implementations can start, exec, stop, and clean up bounded containers.
- The pinned image uses a read-only root, size-limited tmpfs work areas, dropped capabilities, process limits, and controlled mounts.
- Generated Codex permission profiles deny credential and host reads and disable trial-command network access by default.
- Preflight validates the runtime, image, Codex version, authentication, permission profile, and cached canary result.
- Cancellation removes the full container process tree.

Likely files:

- `internal/sandbox/`
- container image definition and pinned image metadata
- permission-profile templates
- preflight and cancellation code in `internal/app/`

Dependencies: Stage 1.

Feature commits:

- `add bounded container runtime`
- `add pinned evaluation image`
- `add Codex permission profiles`
- `add runtime preflight and canary cache`

Stage verification:

- Run one fake-runtime lifecycle test and one real local boundary smoke test when Docker or Podman is available.
- Confirm denied credential access, denied network access, bounded workspace writes, and full cancellation.
- Run the focused checks after all four feature commits, then complete one manual boundary checkpoint.

## [ ] Stage 4: Evaluation, assertions, scoring, and persistence

Requirements: US-06, US-07, US-08, and US-09; sections 6, 7.2, 7.3, 10, 11, and 12.

Expected outcome:

- Each case and trial runs in an isolated Codex session.
- JSONL events, final output, filesystem changes, usage, and duration are captured through the shared redaction boundary.
- Deterministic assertions run in the no-network assertion container or through the no-follow file resolver.
- A fresh judge returns schema-valid, evidence-linked scores.
- Hard caps, aggregation, coverage, confidence, partial status, and checkpoint recovery follow the approved formulas.
- Every run produces versioned JSON and a Markdown report.

Likely files:

- Codex implementation under `internal/harness/`
- `internal/assert/`, `internal/report/`
- runner, judge, score, and checkpoint use cases under `internal/app/`
- result models and JSON schemas

Dependencies: Stages 1-3.

Stage verification:

- Run one fake-Codex end-to-end evaluation covering success and partial cancellation.
- Run focused tests for assertion path safety and score aggregation.
- Keep the paid live Codex smoke test opt-in.
- Commit as `build evaluation and scoring pipeline`.

## [ ] Stage 5: Results UI, comparison, CLI, and MVP handoff

Requirements: US-08, US-10, US-11, and US-12; sections 5.3, 5.4, 9, 12, and 17.

Expected outcome:

- The TUI shows progress, score bars, deterministic findings, case details, bounded evidence, saved runs, and settings.
- Two complete runs can be checked for comparability and rendered with baseline, candidate, and signed deltas.
- `eval`, `compare`, `runs`, `show`, and `config` work without prompts and return documented exit codes.
- The interface keeps the centered terminal.shop-inspired frame, contextual footer, responsive layouts, and terminal cleanup behavior.
- The Ponytail v1 to v2 success path works from artifact intake through saved comparison.

Likely files:

- remaining screens and components under `internal/tui/`
- `internal/report/`
- subcommands under `cmd/mdbench/`
- user-facing README and example commands

Dependencies: Stages 1-4.

Stage verification:

- Run one fake-harness end-to-end path: evaluate v1, evaluate v2 with the frozen suite, reopen both, and compare.
- Manually smoke-test the TUI at 80 columns, 50 columns, and below the minimum width.
- Run CLI help and one JSON-output command.
- Commit as `finish mvp workflows and cli`.

## Deferred test work

- Full golden coverage for every terminal dimension and color profile.
- Large model/provider compatibility matrices.
- Load, soak, and high-concurrency testing.
- Exhaustive malformed JSONL and rate-limit permutations.
- Windows runtime coverage.
- Statistical testing beyond the MVP's one to three trials.
