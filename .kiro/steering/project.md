# Project: aur-pipelines

`aur-pipelines` is a Go CLI that generates Concourse CI pipelines for selected Arch Linux AUR packages. Given a YAML
configuration listing AUR package names, it resolves each package (and its AUR dependencies) and writes one Concourse
pipeline per package base, plus a daily repository-cleanup pipeline.

## Module and entry point

- Module: `github.com/n3tuk/aur-pipelines`
- Entry point: `cmd/aur-pipelines/main.go` (Cobra command tree lives in the `cmd` package).
- Go: use the toolchain version pinned in `go.mod`.

## Package layout

| Path                  | Responsibility                                                              |
| --------------------- | --------------------------------------------------------------------------- |
| `cmd/aur-pipelines/`  | CLI entry point + Cobra commands (`generate`, `version`). May import cobra. |
| `internal/cli/`       | Dependency-light CLI support (build-version reporting). No cobra.           |
| `internal/config/`    | Config structs, YAML loading, defaults, JSON-Schema validation.             |
| `internal/aur/`       | AUR RPC v5 client + `.SRCINFO` fetch (go-git) and parser.                   |
| `internal/resolver/`  | Recursive AUR dependency resolution (RPCSource is the primary Source).      |
| `internal/pipeline/`  | Typed Concourse pipeline model + YAML marshalling.                          |
| `internal/generator/` | Pipeline generation (build/sign/repository + cleanup) and output writing.   |
| `tools/schema/`       | Generates `schemas/aur-pipelines.json` by reflecting the config structs.    |
| `schemas/`            | The published configuration JSON Schema.                                    |

## Dependency policy (enforced by golangci-lint depguard)

Imports are restricted per directory. When adding a dependency you must also add it to the relevant `depguard` allow
list in `.golangci.yaml`:

- `internal/**`: stdlib, the module itself, `Masterminds/sprig/v3`, `go-git/go-git/v5`, `go-git/go-billy/v5`,
  `spf13/viper`, `santhosh-tekuri/jsonschema/v6`, `go.yaml.in/yaml/v3`.
- `cmd/**`: stdlib, the module, `spf13/cobra`, `spf13/viper`.
- `tools/**`: stdlib, the module, `invopop/jsonschema`.

Two JSON-Schema libraries are used deliberately and must not be conflated: `invopop/jsonschema` **generates** the schema
(in `tools/`); `santhosh-tekuri/jsonschema/v6` **validates** against it (in `internal/config`).

## Key conventions

- British English spelling throughout (the `misspell` linter is set to the UK locale). Note that literal flag names such
  as `gpg --armor` are US-spelled by the upstream tool and must be preserved (annotate with `//nolint:misspell`).
- The tool never embeds secrets: generated pipelines reference them via Concourse credential-manager syntax `((...))`.
- Output is deterministic and idempotent: regenerating from the same input produces byte-identical files.

## Working style expectations

- Read `.kiro/steering/tasks.md` for the build/test/lint workflow, `go.md` for Go/linter conventions, and (when touching
  those areas) `schema.md` and `pipelines.md`.
- For the reasoning behind significant design choices (RPC-primary resolver, the `((...))` credential model, notify
  templating, pipeline topology, and so on), see [docs/DECISIONS.md](../../docs/DECISIONS.md) before changing them.
- After any change, run `task lint` and `task test`; both must pass with zero issues before presenting work.
- Keep documentation (README, CONTRIBUTING, and these steering files) in step with behaviour changes.
