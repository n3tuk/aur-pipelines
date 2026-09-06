# AGENTS.md

Guidance for AI agents and automated tooling working in this repository. `aur-pipelines` is a Go CLI that generates
Concourse CI pipelines for Arch Linux AUR packages.

## Where the detailed guidance lives

This file is a concise pointer; the authoritative, detailed guidance is maintained in dedicated locations:

- **Kiro steering** (`.kiro/steering/`): `project.md` (architecture, layout, dependency policy), `go.md` (Go and
  golangci-lint conventions), `tasks.md` (Task workflow), `schema.md` (JSON-Schema generation), and `pipelines.md`
  (Concourse generation model and secret conventions).
- **Kiro skills** (`.kiro/skills/`): `adding-config-field` — the end-to-end procedure for changing a config field.
- **GitHub Copilot** (`.github/copilot-instructions.md`): change and pull-request-review guidance.
- **Design rationale** ([docs/DECISIONS.md](docs/DECISIONS.md)): the _why_ behind the significant design decisions.
- **Contributors** ([CONTRIBUTING.md](CONTRIBUTING.md)): human-facing development setup and workflow.

## Essentials

- Run everything through [Task](https://taskfile.dev/). Before finishing any change: `golangci-lint fmt`, then
  `task lint` (zero issues), `task test`, and `task build`. If config structs changed, run `task go:schema`; if
  generator output changed, regenerate golden files with `go test ./internal/generator/... -update`.
- `golangci-lint` runs with `default: all`; write to the conventions in `.kiro/steering/go.md` (one type/const block per
  file, wrapped errors, no named returns, specific+explained `//nolint`, 120-column lines, UK spelling).
- Never hand-edit the generated JSON Schema files; change the config structs and run `task go:schema`.
- Never embed secrets; generated pipelines use Concourse `((...))` credential references.
- Tests are hermetic (no network) and use in-memory fakes; golden comparisons are semantic, not byte-for-byte.
- Keep documentation (README, CONTRIBUTING, and the steering files) in step with behaviour changes.

## Out of scope (as of this writing)

The GitHub Actions build/test/release workflow and native immutable-releases configuration are planned but not yet
implemented.
