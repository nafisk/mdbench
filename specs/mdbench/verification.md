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
- A follow-up TUI usability pass adds wrapped list navigation, grouped footer controls, a full-height paste editor, and a `Ctrl+S` fallback for terminals that do not report Command keys.
- Successful input saves and same-input suite freezes or reuse now continue directly; only cross-input suite reuse keeps its relevance confirmation.
- Feature commits: `c91b01c`, `3eff67c`, `3f19a22`, and `66ef697`; the execution-plan commit completes this stage.

Known gaps:

- The development generator is deterministic and makes no model calls. The live Codex generator waits for the approved secure runtime boundary.
- Plan confirmation stops before execution. Containers, trials, and cancellation arrive in Stage 3.

## Stage 3: Secure container runtime

Status: Complete

Completed: 2026-07-16

Evidence:

- Focused tests cover Docker and Podman argument construction, immutable image resolution, tmpfs ownership, bounded runtime output, stop-kill-remove cleanup, private permission-config writes, authentication detection, preflight checks, boundary failures, and successful canary reuse.
- `go test ./...`, `go vet ./...`, and a full binary build pass.
- The local image builds from pinned Node and Go digests, exact OS package versions, and the locked official `@openai/codex` 0.144.3 package.
- The image reports Codex 0.144.3, Node 22.23.1, Python 3.11.2, and Go 1.26.5.
- A real Colima/Docker boundary probe confirms a writable size-limited `/work`, a read-only root, readable control input, no host or credential path, and no network under `--network none`.
- The live preflight resolves an immutable local image digest, validates image labels, Codex version, and saved-auth metadata, then enforces the named Bubblewrap permission profile.
- The real nested canary confirms bounded workspace writes, readable control input, denied credential and host paths, denied command network, and successful cache reuse.
- The final Codex launcher adds `SYS_ADMIN`, `SYS_CHROOT`, `SETUID`, `SETGID`, `SYS_PTRACE`, and `NET_ADMIN`; `NET_RAW` is not granted. Removing `SYS_CHROOT` or `SYS_PTRACE` caused the live canary to fail.
- Strict assertion-style containers still use `--cap-drop ALL`, `no-new-privileges`, and `--network none`; their live boundary test passes after the Codex compatibility change.
- Feature commits: `1b6c6d2`, `765a9e0`, `ff419fe`, `ec2c3d4`, plus the final nested-sandbox commit.

Security boundary:

- Current Codex on Linux uses Bubblewrap. The user approved a Codex-launcher-only compatibility policy after the strict Colima canary proved that `--cap-drop ALL` and `no-new-privileges` block namespace creation. Assertions and other non-Codex containers keep the strict policy. The compatibility policy remains fail-closed: no model call can start without a matching successful canary.
