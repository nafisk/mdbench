# mdbench verification log

## Stage 1: Application foundation and artifact intake

Status: Complete

Completed: 2026-07-15

Evidence:

- `go test ./...` passed the focused artifact hashing, secret blocking, atomic storage, and interrupted-write reconciliation tests.
- `go vet ./...` passed.
- `go build -o /private/tmp/mdbench ./cmd/mdbench` completed, and `mdbench --help` described the default TUI entry point and path flags.
- The file flow loaded, inspected, hashed, and saved a Markdown skill through the TUI. Front matter was excluded from the heading count.
- The paste flow loaded, inspected, hashed, and saved a Markdown artifact through the TUI.
- Both manual flows restored the terminal screen and cursor after exit.
- A repository-wide naming audit confirms that product, module, path, and file names use `mdbench`.
- Focused tests cover complete-skill paste compatibility, instructions-only wrapping, and skill-folder detection.
- A manual TUI pass confirmed the permissions-and-size file browser, folder-based Ponytail loading, multiline paste entry, and consistent review wording.
- A manual TUI pass confirmed the browser starts at `/Users/nafiskhan`, `Right` and `Left` navigate folders, the free-form path field is gone, and `Command+Enter` reviews pasted text while plain `Enter` creates new lines.
- A focused TUI test covers the home-directory start and distinguishes `Command+Enter` from plain `Enter`.
- Focused file-browser tests cover live substring filtering, result navigation while typing, immediate directory entry, query reset, two-step clear/cancel behavior, non-fuzzy matching, and Markdown-only selection.
- A manual TUI pass confirmed the always-active filter, arrow-key result navigation, immediate `Enter` action, and automatic query reset after entering a directory.

Known gaps:

- Secure trial execution and scoring are intentionally deferred to later approved stages.
- Broad terminal snapshots and compatibility matrices remain deferred under the MVP testing agreement.

## Stage 2: Test suites and guided planning flow

Status: Complete

Completed: 2026-07-15

Evidence:

- `go test ./...` passed fixture snapshot, suite validation, canonical hash, immutable revision, atomic suite storage, generation, review, reuse, run configuration, and execution-plan checks.
- `go vet ./...` passed.
- `go build -buildvcs=false -o /private/tmp/mdbench-stage2 ./cmd/mdbench` completed.
- A manual TUI pass generated six tests, inspected case assertions and scoring anchors, froze revision 1, configured evaluation defaults, and confirmed a six-trial, twelve-model-call execution plan.
- A second manual TUI pass loaded the frozen revision, showed its origin and applicability, required relevance confirmation, and reached an identical execution plan without changing the suite.
- Focused TUI checks cover generation, freezing, saved-suite listing, edit-revision entry, reuse confirmation, model configuration, and plan construction.
- Feature commits: `c91b01c`, `3eff67c`, `3f19a22`, and `66ef697`; the execution-plan commit completes this stage.

Known gaps:

- The development generator is deterministic and makes no model calls. The live Codex generator waits for the approved secure runtime boundary.
- Plan confirmation stops before execution. Containers, trials, and cancellation arrive in Stage 3.
