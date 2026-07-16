# mdbench — MVP Requirements

Status: Approved
Date: 2026-07-15

## 1. Product definition

mdbench is a local-first CLI and terminal UI for evaluating Markdown-based AI instruction artifacts, beginning with code-writing skill files such as `SKILL.md`.

It combines:

1. deterministic analysis of the Markdown and produced artifacts;
2. generated behavioral test cases;
3. isolated executions through an AI coding harness;
4. independent LLM judging of captured evidence;
5. detailed 0–10 scorecards;
6. persisted, reusable test suites and run results; and
7. fair comparison between two versions or two competing skill files.

The MVP is intended to answer:

> Given this code-writing skill and this frozen set of tests, how reliably does the skill cause an agent to produce correct, concise, safe, dependency-disciplined work?

## 2. Problem statement

Markdown instruction files are usually reviewed by reading them or trying a few ad hoc prompts. That makes iteration difficult because:

- test prompts change between versions;
- model and harness configuration is not recorded consistently;
- objective checks and subjective judgments are mixed together;
- the skill may be judged by the same session that executed it;
- results are difficult to compare over time; and
- failures do not show whether the problem was the skill, model, harness, test, or judge.

mdbench will make the evaluation procedure explicit, repeatable, inspectable, and comparable.

## 3. MVP boundary

### 3.1 Included in MVP

- One Markdown skill per evaluation run.
- File-path selection with completion and a multiline paste option.
- Static deterministic checks against the skill file.
- LLM-assisted generation of a user-reviewable behavioral test suite.
- Reuse of a frozen test suite across multiple skill versions and competing skills.
- One official execution harness adapter, initially Codex.
- User-selectable executor model and independent judge model supported by that adapter/provider configuration.
- Clean, isolated execution session per test trial.
- Deterministic assertions against execution results and workspace changes.
- Independent LLM scoring with a frozen rubric, per-dimension rating guidance, required evidence references, confidence, and concise explanations.
- Multiple dimension scores from 0.0 to 10.0 plus an overall score.
- Optional 1–3 trials per test case; default one trial for cost control.
- Automatically saved local results and reports.
- A focused terminal UI inspired by terminal.shop's centered, keyboard-first, step-based interaction model.
- Non-interactive CLI commands for evaluation, suite reuse, and two-run comparison.
- Comparison of exactly two comparable runs using the same frozen suite revision.
- Horizontal score bars and delta views suitable for a terminal.

### 3.2 Deferred beyond MVP

- Arbitrary N-way comparison and long time-series dashboards.
- A first-party Claude harness adapter.
- Remote/cloud execution, team accounts, shared dashboards, or hosted storage.
- Automatic Git integration or commits.
- Training, fine-tuning, or automatic rewriting of the skill.
- Fully autonomous test generation with no user review.
- Statistical claims based on large repeated samples.

## 4. Core concepts

### 4.1 Artifact

The Markdown skill being evaluated. Every run records an immutable snapshot, content hash, source path or paste origin, and optional version label.

### 4.2 Test suite

A versioned, immutable-after-freeze collection of behavioral cases and their complete scoring contract. Each suite contains:

- task prompts and fixture/setup references;
- origin artifact hash and intended suite applicability;
- timeouts and deterministic assertions;
- score dimensions and weights;
- the frozen judge rubric;
- per-dimension rating guidance and score anchors;
- hard-failure and score-cap rules; and
- required evidence types.

The same suite revision can be applied to later versions of one skill or to competing skills when the cases are relevant to every artifact being compared.

### 4.3 Harness

An adapter that launches an independent coding-agent session, applies the evaluated skill, provides a test task and isolated workspace, and returns structured evidence.

### 4.4 Trial

One isolated execution of one test case against one artifact and one executor configuration.

### 4.5 Judge

A fresh model session that did not execute the test. It receives the frozen rubric, per-dimension rating guidance, and a bounded evidence package, then returns structured scores, required evidence references, confidence, and concise reasons.

### 4.6 Run

The immutable result of evaluating one artifact snapshot against one test-suite revision with one recorded configuration.

## 5. Primary user flows

### 5.1 TUI home

The home screen contains four primary choices:

1. New evaluation
2. Compare runs
3. Saved runs
4. Settings

The interface is flow-based. Each screen asks for one coherent decision and provides Back, Continue, and contextual help rather than placing every control on one page.

### 5.2 New evaluation flow

```text
Home
  → Choose input method
  → Review input
  → Choose or generate test suite
  → Configure harness, models, cases, trials, and limits
  → Review and freeze test suite
  → Review execution plan
  → Run tests
  → View scorecard
  → Inspect test details
```

#### Step 1 — Choose input method

- Open a keyboard-operated file browser at the current user's home directory.
- Show hidden entries, permissions, and sizes in the file browser.
- Use `Right` to open a folder, `Enter` to open a folder or select a Markdown file, and `Left` to return to the parent folder.
- Keep filename filtering active while browsing. Typing filters the current directory by case-insensitive substring, `Up` and `Down` move through matches, and `Enter` immediately opens the highlighted folder or selects the highlighted Markdown file. `Esc` clears a query before leaving the browser, and changing directories clears the query.
- Resolve `SKILL.md` without case sensitivity when a directory is provided through a non-interactive interface.
- Paste a complete skill file or instructions-only Markdown into a multiline editor.
- Use `Enter` for new lines. Use `Command+Enter` to review when the terminal reports the Command modifier, with `Ctrl+S` as a reliable fallback.
- WHEN a selection is on the first or last row, THE SYSTEM SHALL let `Up` or `Down` wrap to the opposite end.
- WHEN contextual controls are shown, THE SYSTEM SHALL render each key and action as a distinct group.
- WHEN the paste editor opens, THE SYSTEM SHALL use the available content-pane height.

#### Step 2 — Review input

- Show detected name, frontmatter status, size, hash, and initial deterministic findings.
- Allow an optional version such as `ponytail-v2`.
- Require explicit confirmation of the chosen snapshot.
- Allow pasted text to continue with safe defaults without requiring file or version controls.
- WHEN the snapshot is saved successfully, THE SYSTEM SHALL continue directly to test-suite selection without a separate success screen.

#### Step 3 — Choose test suite

- Generate a new suite from the artifact and the MVP's code-writing evaluation contract.
- Reuse a previously frozen suite.
- Show compatibility information before reuse.

#### Step 4 — Configure execution

- Harness.
- Executor model.
- Judge model.
- Number of generated cases when creating a suite.
- Trials per case, from one to three.
- Per-trial timeout.
- Network policy.
- Maximum concurrency.

#### Step 5 — Review and freeze tests

- Show every generated case, assertion, judge criterion, score dimension, rating guide, hard-failure rule, and weight.
- Allow editing, disabling, reordering, rubric refinement, and weight adjustment.
- Freeze the suite into an immutable revision before execution.
- WHEN a suite is frozen or a matching frozen suite is selected, THE SYSTEM SHALL continue directly to evaluation configuration.
- WHEN a frozen suite comes from a different input, THE SYSTEM SHALL require a relevance confirmation before evaluation configuration.

#### Step 6 — Review execution plan

- Show artifact, suite, models, number of trials, estimated model-call count, timeout policy, and isolation policy.
- Require confirmation before model calls begin.

#### Step 7 — Run

- Show case-by-case queued/running/passed/failed/judging status.
- Show elapsed time and completed/total trials.
- Allow safe cancellation.

#### Step 8 — Results

- Overall score and evidence-confidence indicator.
- Dimension score bars.
- Deterministic check summary.
- Case table with success, score, duration, and failure reason.
- Drill-down into assertions, judge reasoning, transcript excerpts, and workspace changes.
- Saved run identifier and report location.

### 5.3 Comparison flow

```text
Home
  → Compare runs
  → Select baseline run
  → Select candidate run
  → Validate comparability
  → View overall and per-dimension deltas
  → Inspect changed test outcomes
```

The same comparison flow supports:

- version comparison: Ponytail v1 versus Ponytail v2; and
- competitor comparison: Caveman versus Ponytail.

The comparison is considered fair only when the suite revision, harness, executor model, judge model, trial count, and material execution policies match.

### 5.4 Non-interactive CLI flow

The CLI supports at minimum:

```text
mdbench                         Open the TUI
mdbench eval <artifact>         Run an evaluation
mdbench eval --stdin            Read the artifact from standard input
mdbench compare <run-a> <run-b> Compare two saved runs
mdbench runs                    List saved runs
mdbench show <run-id>           Render a saved report
mdbench config                  Inspect effective non-secret settings
```

Exact flag names are finalized during design, but non-interactive evaluation must support selecting a frozen suite, harness/model settings, trial count, timeouts, output format, and a no-prompt mode.

## 6. Score model

### 6.1 MVP dimensions

Every MVP suite begins with the following code-writing dimensions. The user reviews and freezes their definitions, rating guidance, applicability, and weights as part of the suite:

| Dimension | Meaning |
|---|---|
| Task success | Whether the requested behavior works and required checks pass |
| Correctness | Technical and semantic correctness beyond binary completion |
| Instruction adherence | Whether the agent follows the task and skill constraints |
| Code quality | Maintainability, clarity, scope, and consistency with the repository |
| Concision | Whether the solution avoids unnecessary code, explanation, or churn |
| Dependency discipline | Whether dependencies are avoided, reused, or added only with justification |
| Safety | Whether the run preserves permissions, secrets, data, and destructive-action boundaries |
| Verification quality | Whether the agent runs appropriate checks and reports remaining risk honestly |

Each score is a number from 0.0 to 10.0 with one decimal place, evidence references, and a short rationale.

### 6.2 Overall score

The overall score is a weighted aggregate of applicable dimensions using the weights frozen in the suite. Missing or non-applicable dimensions are identified explicitly and are not silently treated as zero.

### 6.3 Deterministic versus judged evidence

The report keeps these separate:

- deterministic checks produce pass/fail or numeric measurements;
- the judge interprets evidence using the suite's frozen rubric and per-dimension rating guidance; and
- the final scorecard shows which evidence affected each dimension.

The judge must not override a failed hard assertion by opinion. The frozen suite may define score caps caused by specific hard failures, such as tests not passing or a forbidden destructive action.

### 6.4 Repeated trials

When a case has multiple trials, mdbench reports the mean, minimum, maximum, and spread. It must not present a single-trial score as a statistical reliability estimate.

## 7. Deterministic checks for the MVP

### 7.1 Static artifact checks

- File exists or pasted input is non-empty.
- Input is readable UTF-8 Markdown within configured size limits.
- YAML frontmatter parses when present. A common single-line skill description containing literal colon punctuation is read as text without changing the saved source.
- Recognized skill frontmatter fields have valid names and value types when present.
- Markdown code fences are balanced.
- Internal heading anchors resolve.
- Referenced relative files exist within the allowed artifact root.
- Duplicate headings/identifiers and obvious placeholder markers are reported.
- Character, word, line, heading, rule, and code-block counts are recorded.
- Secret-like values and private-key patterns are flagged without persisting the matched secret.

Static checks distinguish errors from warnings and informational metrics.

### 7.2 Behavioral trial assertions

A generated or edited test case may use:

- process exit status;
- build, unit-test, lint, or format command result;
- required or forbidden file existence;
- required content or structured output;
- allowed file/directory scope;
- workspace diff size;
- package-manifest changes;
- dependency additions/removals;
- forbidden command/action observations;
- timeout and completion status; and
- user-defined shell-free structured checks supported by mdbench's built-in assertion catalog.

Arbitrary assertion scripts are outside the default trusted path for MVP. Any user-supplied executable hook requires an explicit unsafe opt-in and is deferred unless approved during design.

## 8. User stories and acceptance criteria

### US-01 — Select an artifact

As a skill author, I want to choose or paste a Markdown skill so that I can evaluate the exact content I am working on.

- WHEN the user enters a partial file path and requests completion, THE SYSTEM SHALL display matching files and directories without leaving the TUI.
- WHEN the user chooses a readable Markdown file, THE SYSTEM SHALL snapshot its content and compute a stable content hash before evaluation.
- WHEN the user pastes Markdown, THE SYSTEM SHALL preserve the pasted content as an immutable run input without requiring a source file.
- WHEN the input is empty, unreadable, non-UTF-8, or exceeds the configured limit, THE SYSTEM SHALL block execution and explain the correction required.

### US-02 — Inspect deterministic quality signals

As a skill author, I want objective structural feedback before spending model calls so that basic defects are inexpensive to fix.

- WHEN an artifact is confirmed, THE SYSTEM SHALL run all applicable static deterministic checks before test generation or execution.
- WHEN a deterministic check fails, THE SYSTEM SHALL identify the check, severity, location when available, and remediation guidance.
- WHEN a secret-like value is detected, THE SYSTEM SHALL redact the matched value from saved output and logs.

### US-03 — Generate behavioral tests

As a skill author, I want a model to propose relevant tests from the skill's stated purpose so that evaluation covers more than generic linting.

- WHEN the user requests a new suite, THE SYSTEM SHALL generate the requested number of cases using the MVP code-writing evaluation contract and artifact snapshot.
- WHEN a case is generated, THE SYSTEM SHALL require a title, task prompt, fixture/setup description, timeout, deterministic assertions, judge criteria, applicable dimensions, and required evidence.
- WHEN a suite is generated, THE SYSTEM SHALL include a reviewable rubric with per-dimension rating guidance, score anchors, hard-failure rules, and weights.
- WHEN a suite is generated for later comparison, THE SYSTEM SHALL describe cases in terms of observable behavior and SHALL NOT make the originating skill's name or wording part of the expected result unless the user explicitly approves it.
- WHEN generated output does not match the suite schema, THE SYSTEM SHALL reject or repair it before presenting it as a test case.
- WHEN generation completes, THE SYSTEM SHALL require user review before the suite can be frozen or executed.

### US-04 — Review and freeze a suite

As a skill author, I want to edit and freeze tests so that later versions and competing skills are measured against the same standard.

- WHEN reviewing a draft suite, THE SYSTEM SHALL allow cases to be edited, disabled, reordered, and reweighted.
- WHEN the user freezes a suite, THE SYSTEM SHALL assign an immutable suite revision and content hash covering its cases, assertions, rubric, rating guidance, hard-failure rules, and weights.
- WHEN a frozen suite is modified, THE SYSTEM SHALL create a new revision rather than mutate the previous revision.
- WHEN a frozen suite is reused, THE SYSTEM SHALL preserve the original prompts, assertions, rubric, rating guidance, hard-failure rules, and weights exactly.
- WHEN a frozen suite is applied to an artifact different from its origin artifact, THE SYSTEM SHALL show the suite's origin and intended applicability and require the user to confirm that the cases are relevant to the new artifact.

### US-05 — Configure models and harness

As an evaluator, I want to choose the execution and judging configuration so that I can test the artifact under the conditions I care about.

- WHEN starting a run, THE SYSTEM SHALL record the harness, executor model, judge model, suite revision, trial count, timeout, concurrency, and network policy.
- WHEN a configured model or harness is unavailable, THE SYSTEM SHALL fail before launching trials and identify the unavailable setting.
- WHEN executor and judge use the same model, THE SYSTEM SHALL still create independent sessions and label that configuration in the report.
- WHEN a secret is required by a provider, THE SYSTEM SHALL read it from an environment/provider credential mechanism and SHALL NOT store the secret in mdbench configuration or run artifacts.

### US-06 — Execute isolated trials

As a skill author, I want each test to run in a clean session so that results are not contaminated by previous tests.

- WHEN a trial starts, THE SYSTEM SHALL create a clean agent session and isolated disposable workspace from the case fixture.
- WHEN a trial starts, THE SYSTEM SHALL provide only the evaluated artifact, test task, allowed fixture, and explicitly configured capabilities.
- WHEN a trial completes, THE SYSTEM SHALL capture exit status, bounded transcript, workspace diff, command/test evidence, duration, and available usage metadata.
- WHEN a trial times out or is cancelled, THE SYSTEM SHALL terminate the child session, mark the trial incomplete, and retain safe diagnostic evidence.
- WHEN network access is disabled, THE SYSTEM SHALL prevent or clearly fail network-dependent behavior rather than silently enabling it.

### US-07 — Judge results independently

As an evaluator, I want a fresh judge to score evidence against a fixed rubric so that the execution session does not grade itself.

- WHEN trial evidence is ready, THE SYSTEM SHALL launch a judge session that is independent from the executor session.
- WHEN judging a trial, THE SYSTEM SHALL present the artifact as quoted evaluation subject matter and SHALL NOT allow artifact instructions to replace the judge rubric.
- WHEN the judge responds, THE SYSTEM SHALL require schema-valid dimension scores, required evidence references, confidence, and concise rationale that follows the frozen per-dimension rating guidance.
- WHEN a hard deterministic assertion fails, THE SYSTEM SHALL apply the frozen suite's declared cap or failure rule regardless of judge preference.
- WHEN judge output cannot be validated after the allowed retries, THE SYSTEM SHALL mark judging incomplete rather than invent a score.

### US-08 — Understand a result

As a skill author, I want a concise summary with drill-down evidence so that I know what to improve.

- WHEN a run completes, THE SYSTEM SHALL display overall and per-dimension scores on a 0.0–10.0 scale.
- WHEN displaying terminal charts, THE SYSTEM SHALL include numeric labels and SHALL NOT rely on color alone.
- WHEN the user selects a test case, THE SYSTEM SHALL show assertions, scores, evidence, reasons, runtime, and failure details.
- WHEN some trials or judgments are incomplete, THE SYSTEM SHALL label the run partial and SHALL NOT present it as fully comparable.

### US-09 — Save and reopen results

As a skill author, I want every run saved locally so that I can inspect or compare it later.

- WHEN a run is created, THE SYSTEM SHALL persist its input snapshot, configuration, suite revision, trial evidence, scores, and timestamps under a unique run identifier.
- WHEN a run is saved, THE SYSTEM SHALL produce both machine-readable structured results and a human-readable Markdown report.
- WHEN mdbench is interrupted, THE SYSTEM SHALL preserve completed trial results and mark unfinished work accurately.
- WHEN a saved run is reopened, THE SYSTEM SHALL render scores from persisted data without rerunning model calls.

### US-10 — Compare versions or competing skills

As a skill author, I want to compare two runs against the same tests so that I can see whether a revision or alternative is better.

- WHEN two runs share the same suite revision, THE SYSTEM SHALL compare overall, dimension, case, deterministic, duration, and available usage results.
- WHEN all material execution settings match, THE SYSTEM SHALL label the comparison fair.
- WHEN material settings differ, THE SYSTEM SHALL identify every confounding difference and SHALL NOT label the score delta as an artifact-only improvement.
- WHEN the comparison is rendered, THE SYSTEM SHALL show baseline value, candidate value, and signed delta for every applicable dimension.
- WHEN a test changes from pass to fail or fail to pass, THE SYSTEM SHALL make that regression or improvement directly inspectable.

### US-11 — Use mdbench without the TUI

As an automation user, I want one-shot commands so that I can run evaluations in scripts or CI.

- WHEN all required non-interactive arguments are supplied, THE SYSTEM SHALL complete without opening the TUI or prompting for input.
- WHEN required arguments are missing in no-prompt mode, THE SYSTEM SHALL exit non-zero with actionable usage information.
- WHEN structured output is requested, THE SYSTEM SHALL write schema-versioned JSON and keep human progress output separate.
- WHEN an evaluation fails or is partial, THE SYSTEM SHALL return a documented exit status suitable for automation.

### US-12 — Configure defaults safely

As a repeat user, I want local defaults for harnesses and models so that new runs require fewer steps.

- WHEN a setting is changed, THE SYSTEM SHALL show its source and effective value.
- WHEN configuration is persisted, THE SYSTEM SHALL exclude credentials and secret values.
- WHEN command-line flags conflict with saved settings, THE SYSTEM SHALL apply a documented precedence order and record the effective configuration in the run.

## 9. TUI requirements

- The TUI uses a centered maximum canvas and adapts to smaller terminals.
- The TUI uses a persistent header, optional flow progress, focused content region, and contextual footer.
- The TUI presents one decision group per step.
- The TUI supports arrow navigation, Enter to select/continue, Esc to go back/cancel, and visible direct shortcuts where appropriate.
- The TUI provides a compact help/menu view.
- The TUI uses horizontal bars, small tables, and signed deltas rather than dense dashboards or radar charts in the MVP.
- The TUI keeps the current action and footer reachable at supported minimum dimensions.
- The TUI supports light and dark terminal backgrounds and never relies solely on color.
- The TUI restores terminal state and cursor visibility after normal exit, error, or cancellation.

Acceptance criteria:

- WHEN the terminal is at least 80 columns wide, THE SYSTEM SHALL render the primary flow in a centered large layout with readable content and footer.
- WHEN the terminal is between 50 and 79 columns wide, THE SYSTEM SHALL render a compact centered layout without hiding the current action.
- WHILE the user is in the primary setup flow, THE SYSTEM SHALL use the standard setup canvas unless the user has explicitly approved a screen-specific size.
- WHEN a screen-specific size is explicitly approved, THE SYSTEM SHALL allow that screen to use a larger or smaller bounded layout without changing the default setup canvas.
- WHEN the terminal is below the supported minimum, THE SYSTEM SHALL show a resize message rather than clipping required controls.
- WHEN content exceeds the available height, THE SYSTEM SHALL make the content scrollable while keeping navigation discoverable.
- WHEN the user exits, THE SYSTEM SHALL restore the previous screen buffer and terminal cursor state.

## 10. Persistence and reproducibility requirements

Every run records:

- schema version;
- mdbench version;
- artifact snapshot, hash, and optional label;
- suite identifier, revision, and hash;
- frozen score dimensions, weights, rubric, rating guidance, hard-failure rules, and rubric hash;
- harness and model identifiers;
- judge model identifier;
- effective settings and capability policy;
- timestamps and durations;
- per-case/per-trial status;
- deterministic observations;
- judge outputs;
- aggregate calculations;
- usage/cost metadata when available; and
- redaction/incompleteness warnings.

Acceptance criteria:

- WHEN the same saved run is rendered more than once, THE SYSTEM SHALL produce the same numeric scores without invoking a model.
- WHEN aggregate scores are calculated, THE SYSTEM SHALL use a documented deterministic formula over persisted inputs.
- WHEN a schema changes, THE SYSTEM SHALL preserve the schema version and fail clearly or migrate explicitly rather than misreading old results.

## 11. Safety and trust requirements

- Test execution is untrusted code execution.
- The evaluated Markdown is untrusted instruction content.
- Model and provider outputs are untrusted structured data until validated.
- Credentials, personal files, and the user's main workspace are unavailable to trials by default.
- Network access is disabled by default for trials unless a test explicitly requires and the user approves it.
- Destructive host operations are never allowed as an implicit test capability.
- Logs and reports are redacted before persistence.
- The judge receives only bounded evidence needed for the rubric.

Acceptance criteria:

- WHEN a trial requests access outside its isolated workspace, THE SYSTEM SHALL deny the request unless an explicit approved capability permits it.
- WHEN artifact text attempts to instruct the generator or judge to ignore its rubric, THE SYSTEM SHALL continue treating the artifact as evaluation subject matter.
- WHEN a process exceeds its timeout, THE SYSTEM SHALL terminate it and any owned children.
- WHEN sensitive-looking content is captured, THE SYSTEM SHALL redact it before display or persistence while recording that redaction occurred.

## 12. Performance and operability requirements

- Initial non-model screens should feel immediate on a normal local terminal.
- Progress must remain responsive while harness/model calls run.
- Model calls and trials support cancellation.
- Concurrency defaults conservatively to avoid rate-limit and resource surprises.
- The execution plan shows the number of planned generator, executor, and judge calls.
- Partial failure of one case does not erase other completed results.

Acceptance criteria:

- WHEN a long operation is running, THE SYSTEM SHALL update progress without blocking keyboard cancellation.
- WHEN one trial fails, THE SYSTEM SHALL continue or stop according to an explicit run policy and record the decision.
- WHEN provider rate limiting occurs, THE SYSTEM SHALL apply bounded retries and expose the resulting delay/failure.

## 13. Edge cases

- Markdown with no frontmatter.
- Markdown containing its own prompt-injection language.
- Very large or deeply nested Markdown.
- Broken relative references or references outside the allowed root.
- Duplicate or renamed skill versions with identical content hashes.
- Generated suite with invalid, impossible, or unsafe assertions.
- Test fixture incompatible with the selected harness/model.
- Executor returns prose but makes no workspace changes.
- Executor succeeds but required deterministic tests fail.
- Judge produces malformed output, missing scores, or unsupported evidence claims.
- One or more trials time out.
- User cancels during generation, execution, or judging.
- Provider authentication expires mid-run.
- Different model aliases resolve to changed upstream model versions.
- Comparison inputs use different suites, models, trial counts, or network policies.
- Terminal resized during file selection, execution, or chart viewing.
- Saved run is partially written or from a newer schema.

## 14. Non-goals

- Proving that an LLM judgment is objectively correct.
- Claiming benchmark-level statistical significance from one to three trials.
- Hiding model, harness, or judge configuration behind one opaque score.
- Automatically selecting a “best” skill without evidence review.
- Modifying the evaluated skill automatically in MVP.
- Executing tests directly in the user's source repository by default.
- Supporting arbitrary executable test hooks without explicit trust controls.
- Reproducing terminal.shop branding; only its clear, compact interaction principles are a reference.

## 15. Assumptions proposed for MVP

1. The MVP evaluates code-writing skills with one base evaluation contract; it does not include a selectable evaluation-profile layer.
2. Codex is the first official harness; the harness boundary must allow later Claude and custom adapters.
3. A run defaults to six generated cases and one trial per case.
4. Generator, executor, and judge may use configurable models, but executor and judge always use separate sessions.
5. Test suites are reviewed by a human before freezing.
6. Results are local files; there is no database server or account system.
7. Comparison is limited to two runs in MVP.
8. Fair artifact comparison requires identical material execution configuration.
9. Terminal charts use labeled horizontal bars and deltas, not a radar chart.
10. Provider credentials remain in existing environment/provider mechanisms.

## 16. Approved MVP decisions

1. Codex is the only MVP harness. Claude is deferred.
2. The harness interface is included; a generic command/JSON adapter is not.
3. Defaults are six generated cases and one trial per case.
4. Human review is mandatory before a generated suite can freeze or run.
5. Trial network access is disabled by default.
6. Bounded, redacted transcripts are saved by default.
7. A fair comparison requires the same judge model. Re-judging is deferred.
8. Plain Markdown is accepted and may be wrapped as a temporary Codex skill.
9. The eight MVP dimensions start with equal weights and may be marked not applicable during review.
10. V1 exports JSON data and Markdown reports. CSV is deferred.

## 17. MVP success criteria

The MVP validates the product idea when a user can:

1. select or paste Ponytail v1;
2. generate, review, and freeze a six-case code-writing suite;
3. execute it through Codex and receive a saved evidence-backed scorecard;
4. revise the skill into Ponytail v2;
5. rerun the exact frozen suite with the same configuration;
6. compare v1 and v2 in the TUI;
7. identify which dimensions and test cases improved or regressed; and
8. reproduce the same comparison view later without new model calls.

No implementation should begin until these requirements are approved or revised.
