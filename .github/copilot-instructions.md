# Copilot instructions for aur-pipelines

`aur-pipelines` is a Go CLI that generates Concourse CI pipelines for Arch Linux AUR packages. It reads a YAML config
listing AUR package names, resolves each package and its AUR dependencies, and writes one Concourse pipeline per package
base plus a daily repository-cleanup pipeline.

Use these instructions when suggesting changes and when reviewing pull requests.

## Architecture

- Module `github.com/n3tuk/aur-pipelines`; entry point `cmd/aur-pipelines/main.go`.
- `cmd/aur-pipelines/` — Cobra commands. `internal/cli/` — dependency-light CLI helpers.
- `internal/config/` — config structs, loading, defaults, JSON-Schema validation.
- `internal/aur/` — AUR RPC v5 client + `.SRCINFO` fetch/parse.
- `internal/resolver/` — recursive AUR dependency resolution.
- `internal/pipeline/` — typed Concourse model + YAML marshalling.
- `internal/generator/` — pipeline generation and output writing.
- `tools/schema/` — reflects the config structs into `schemas/aur-pipelines.json`.

## Build, test, and lint (Task)

- All actions run through [Task](https://taskfile.dev/): `task lint`, `task test`, `task build`, `task go:schema`.
- Every change must pass `task lint` (golangci-lint + Prettier + markdownlint + yamllint) with zero issues, and
  `task test`.
- Format Go with `golangci-lint fmt --config .golangci.yaml` before committing.

## Dependency policy

Imports are restricted per directory by golangci-lint `depguard`. A new dependency must be added to the correct allow
list in `.golangci.yaml`:

- `internal/**`: stdlib, the module, sprig, go-git, go-billy, viper, `santhosh-tekuri/jsonschema/v6`,
  `go.yaml.in/yaml/v3`.
- `cmd/**`: stdlib, the module, cobra, viper.
- `tools/**`: stdlib, the module, `invopop/jsonschema`.

`invopop/jsonschema` generates the schema; `santhosh-tekuri/jsonschema/v6` validates against it — do not conflate them.

## Coding conventions (golangci-lint `default: all`)

- British English spelling (misspell UK locale). Preserve literal US-spelled tool flags such as `gpg --armor` with
  `//nolint:misspell`.
- One `type ( ... )` block and one `const ( ... )` block per file; file order type → const → var → func (`decorder`).
- Wrap cross-package errors with `fmt.Errorf("...: %w", err)` (`wrapcheck`). No named returns (`nonamedreturns`). No
  inline `if err := ...; err != nil` (`noinlineerr`) — assign then check. Max line length 120; max cyclomatic
  complexity 15.
- Every `//nolint` must name the linter and give a reason.
- Extract string literals used 3+ times into constants (`goconst`), including in tests.

## Schema and config

- Never hand-edit `schemas/aur-pipelines.json` or `internal/config/schema/aur-pipelines.json`. Change the structs in
  `internal/config/config.go` (`jsonschema` tags for constraints; doc comment for the description) and run
  `task go:schema`. Both files are generated together and a test enforces they stay identical.
- Optionality is driven by the `json` tag: `,omitzero`/`,omitempty` makes a field optional; a field without either is
  required in the schema. Constraints: `required`, `enum=...`, `pattern=...`, `minLength=1`, `minItems=1`, `format=uri`.
- Config validation runs against the raw parsed document (so unknown keys and bad values are caught); defaults are
  applied afterwards.

## Pipelines and secrets

- Pipeline generation is programmatic (typed structs), never templated.
- Secrets are never embedded; generated pipelines use Concourse credential references: `((gpg.signing-key))`,
  `((gpg.signing-passphrase))`, `((r2.account-id))`, `((r2.access-key-id))`, `((r2.secret-access-key))`,
  `((webhooks/<secret>.url))`, and `((repositories/<secret>.username|password))`. These trip gosec G101 — annotate with
  `//nolint:gosec // credential-manager reference, not a secret`.
- Notification templates use shell `${VAR}` expansion at run time (not Concourse templating). The notify script is
  best-effort: `set -u` only, each `curl` guarded with `|| true`.
- The repository job is `serial: true`; `.db`/`.files` are written as copies (buckets lack symlinks).

## Tests

- Tests are hermetic (no network): use `httptest` and in-memory fakes.
- Golden files live in each package's `testdata/`; regenerate with `go test ./internal/generator/... -update`. Golden
  comparisons are semantic (parse then compare) because Prettier reformats committed files during lint.

## Review checklist

When reviewing a PR, check that:

1. `task lint` and `task test` would pass (formatting, the linter rules above, no unexplained `//nolint`).
2. Config struct changes regenerated the schema (`task go:schema`) and both schema files match.
3. New/changed behaviour has tests; golden files were regenerated when output changed.
4. Secrets use `((...))` references and are never embedded literally.
5. Documentation (README, and `.kiro/steering/*.md` where relevant) was updated to match behaviour.
6. British spelling is used in prose and identifiers.
