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

- Test-suite generation and evaluation are intentionally deferred to later approved stages.
- Broad terminal snapshots and compatibility matrices remain deferred under the MVP testing agreement.
