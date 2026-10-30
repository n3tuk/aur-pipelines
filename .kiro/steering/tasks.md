# Taskfile workflow

All development actions go through [Task](https://taskfile.dev/); the `Taskfile.yaml` at the root includes per-tool
Taskfiles from `.task/`. Run `task --list` to see everything. Prefer these tasks over ad-hoc `go`/tool invocations so
the same configuration and exclusions apply.

## The tasks you will use most

| Task                    | What it does                                                                      |
| ----------------------- | --------------------------------------------------------------------------------- |
| `task` / `task develop` | Full loop: lint → test → build → analyse (plus pre-commit install, healthcheck).  |
| `task lint`             | Go lint (`golangci-lint`), Prettier, markdownlint, yamllint.                      |
| `task test`             | Unit tests (`go test`).                                                           |
| `task build`            | Build `bin/aur-pipelines` via GoReleaser snapshot.                                |
| `task go:schema`        | Regenerate `schemas/aur-pipelines.json` (and the embedded copy). See `schema.md`. |
| `task analyse`          | Static/security analysis: check-jsonschema, actionlint, zizmor, snyk, CodeQL.     |
| `task clean`            | Remove temporary build/test artefacts.                                            |

## Expected workflow for a change

1. Make the change.
2. `golangci-lint fmt --config .golangci.yaml` (or `task go:fmt`) to format.
3. If config structs changed, `task go:schema` to regenerate the schema.
4. If generator output changed intentionally, `go test ./internal/generator/... -update` to refresh golden files.
5. `task lint` — must report zero issues.
6. `task test` — must pass.
7. `task build` — must succeed.

`task analyse` is heavier (some steps need network or extra tools) and is primarily for CI; run it when relevant but the
gating checks for local work are lint + test + build.

## GoReleaser build notes

- `task build` uses `goreleaser build --snapshot --single-target`. `.goreleaser.yaml` pins amd64 to `v3`, so the
  `go:build` task exports `GOAMD64=v3` to make the single-target match on amd64 hosts.
- The GoReleaser `main:` points at the package directory `./cmd/aur-pipelines` (not a single file), so multi-file main
  packages compile correctly.

## Lint scope gotcha: generated output directories

Prettier and yamllint discover files with `find`, which does **not** honour `.gitignore`. Any directory of generated
YAML/JSON in the working tree will otherwise be linted and fail. Two directories are explicitly excluded in
`.task/prettier.yaml`, `.task/yamllint.yaml`, `.yamllint.yaml`, and `.prettierignore`:

- `internal/**/testdata/` — golden files and intentionally-invalid fixtures.
- `pipelines/` — the default `generate` output directory.

If you add another location that holds generated or intentionally-non-conforming files, add it to those exclusions too
(and keep the `find` predicate lines under 120 characters — split into multiple `-not` clauses if needed).
