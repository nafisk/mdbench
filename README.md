<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./docs/assets/readme/hero-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="./docs/assets/readme/hero-light.svg">
    <img alt="mdbench turns Markdown instructions into reviewable test suites" src="./docs/assets/readme/hero-light.svg">
  </picture>
</p>

<p align="center">
  <a href="./go.mod"><img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/nafisk/mdbench?style=flat-square&amp;logo=go&amp;logoColor=white&amp;label=go&amp;labelColor=191919&amp;color=3A3F42"></a>
  <a href="https://pkg.go.dev/charm.land/bubbletea/v2@v2.0.6"><img alt="Bubble Tea v2.0.6" src="https://img.shields.io/badge/Bubble_Tea-v2.0.6-3A3F42?style=flat-square&amp;labelColor=191919"></a>
  <a href="https://github.com/nafisk/mdbench/issues"><img alt="Issues welcome" src="https://img.shields.io/badge/issues-welcome-BC4C00?style=flat-square&amp;logo=github&amp;logoColor=white&amp;labelColor=191919"></a>
</p>

<p align="center">
  <a href="#quick-start">Quick start</a> ·
  <a href="#what-works-today">Current scope</a> ·
  <a href="./specs/mdbench/design.md">Design</a>
</p>

mdbench is a terminal workbench for turning Markdown instructions into versioned test suites with explicit scoring criteria.

## Quick start

Requires Go 1.26 or newer.

```bash
git clone https://github.com/nafisk/mdbench.git
cd mdbench
go run ./cmd/mdbench
```

## What works today

- [x] Inspect a Markdown file, skill folder, or pasted instructions before saving the input snapshot.
- [x] Draft and edit test cases for Go, Node.js, Python, or an empty workspace.
- [x] Review assertions, weighted scoring criteria, judge guidance, and hard-failure rules.
- [x] Save immutable suite revisions, reuse them, and configure an execution plan.
- [ ] Generate suites with a model, run isolated trials, judge the results, and compare scorecards.

> [!NOTE]
> The current build stops after execution-plan confirmation. Nothing runs a model or trial yet.

## Development

```bash
go test ./...
go vet ./...
go run ./cmd/mdbench
```

Project docs: [requirements](./specs/mdbench/requirements.md) · [design](./specs/mdbench/design.md) · [tasks](./specs/mdbench/tasks.md) · [verification](./specs/mdbench/verification.md)

## Contributing

For now, use [GitHub Issues](https://github.com/nafisk/mdbench/issues) for bug reports and focused product ideas.

## License

An open-source license has not been selected yet. Until one is added, the repository is available for review but is not ready for redistribution.
