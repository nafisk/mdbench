<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./docs/assets/readme/hero-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="./docs/assets/readme/hero-light.svg">
    <img alt="mdbench turns Markdown instructions into reviewable test suites" src="./docs/assets/readme/hero-light.svg">
  </picture>
</p>

<p align="center"><strong>Turn Markdown instructions into reviewable test suites.</strong></p>
<p align="center"><sub>Local-first · terminal-native · early preview</sub></p>

mdbench is a focused TUI for inspecting a Markdown skill, drafting weighted tests, saving exact suite revisions, and reviewing how an evaluation will run.

## Quick start

Requires Go 1.26 or newer.

```bash
git clone https://github.com/nafisk/mdbench.git
cd mdbench
go run ./cmd/mdbench
```

## What works today

- Open a Markdown file or skill folder, or paste instructions directly.
- Inspect frontmatter, references, size limits, placeholders, and secret-like values before saving anything.
- Draft test suites against built-in Go, Node.js, Python, and empty workspaces.
- Review test cases, assertions, weighted scoring criteria, judge guidance, and hard-failure rules.
- Save immutable suite revisions, reuse them, and build an execution plan with separate executor and judge sessions.

## How it works

```text
Markdown skill
      ↓
Inspect input
      ↓
Draft test cases
      ↓
Review weighted scoring criteria
      ↓
Save test suite
      ↓
Review execution plan
```

The current generator is deterministic and intended for development. LLM-generated suites, trial execution, judging, scorecards, and run comparison are still in progress.

## Development

```bash
go test ./...
go vet ./...
go run ./cmd/mdbench
```

The full product requirements and design live in [`specs/mdbench`](./specs/mdbench).

## Contributing

mdbench is still early. Bug reports and focused ideas are welcome in [GitHub Issues](https://github.com/nafisk/mdbench/issues). A contribution guide will arrive before the first public release.

## License

An open-source license has not been selected yet. Until one is added, the repository is available for review but is not ready for redistribution.
