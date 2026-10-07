# Go and golangci-lint conventions

The repository runs `golangci-lint` with `default: all` (most linters enabled) and a strict configuration in
`.golangci.yaml`. Every change must pass `task lint` with zero issues. The rules below are the ones that most often
catch new code in this project — write to them from the start rather than fixing afterwards.

## Formatting and imports

- Run `golangci-lint fmt --config .golangci.yaml` (or `task go:fmt`) before finishing; it applies gofumpt/goimports/gci.
- Import ordering (gci) is: standard, default (third-party), `prefix(github.com/n3tuk)`, local module, alias, blank —
  each in its own group. The formatter fixes this; do not hand-order imports.
- Max line length is 120 (`lll`). Struct tags cannot be wrapped — when a tag line exceeds 120, annotate the field with
  `//nolint:lll // struct tags cannot be wrapped`.

## Declaration ordering (decorder) — very common

- Within a single file, all `type` declarations must be in one `type ( ... )` block, all `const` in one `const ( ... )`
  block, and the file order must be **type → const → var → func**.
- Do not scatter multiple `type`/`const`/`var` blocks in one file; group them.
- A `//go:embed` directive must sit directly above its `var` — put that `var` inside the single `var ( ... )` block.

## nolint discipline (nolintlint)

- Every `//nolint` must be **specific** (name the linter, e.g. `//nolint:gosec`) and **explained** (add `// reason`).
- Recurring legitimate nolints in this project:
  - `//nolint:gosec` on `((...))` credential-reference string constants/maps (G101 false positive; they are not
    secrets).
  - `//nolint:misspell` on lines containing literal US-spelled tool flags (e.g. `gpg --armor`).
  - `//nolint:paralleltest` on tests that use `t.Chdir` (which is incompatible with `t.Parallel`).
  - `//nolint:gochecknoglobals` on a test's `-update` golden flag variable.

## Other frequently-hit linters

- `goconst`: extract a string literal that appears 3+ times into a constant (including in `_test.go` files — the
  `<pkg>_test` package shares constants across its files).
- `wsl_v5`: requires blank lines around blocks; notably a blank line before a `for`/`append`/`return` that does not
  directly follow a related statement, and before `defer` after several statements.
- `cyclop`: max cyclomatic complexity 15 per function. Split large table-driven tests or functions into helpers.
- `wrapcheck`: wrap errors returned from other packages with `fmt.Errorf("...: %w", err)`; do not return them bare.
- `errchkjson`: for a statically-safe `json.Marshal` (e.g. of a concrete struct) it flags the checked error as
  unnecessary — annotate with `//nolint:errchkjson // <type> marshalling is statically safe` while keeping the check.
- `nonamedreturns`: avoid named returns; use explicit return values.
- `noinlineerr`: do not use `if err := f(); err != nil` — assign first (`err := f()`), then check on the next line.
- `exhaustruct` and `exhaustruct_v5` are both disabled (cobra/large structs make them impractical); do not re-enable.
- `usetesting`/`modernize`: prefer `t.Chdir`/`t.Context` over `os.Chdir`/`context.Background` in tests; prefer
  `json:",omitzero"` over `,omitempty` on struct-typed fields.

## Struct tags and JSON-Schema (invopop)

- Config structs carry four tag families: `json`, `mapstructure`, `yaml`, and `jsonschema`.
- invopop treats a field **without** `,omitempty`/`,omitzero` on its `json` tag as **required** in the schema. To make a
  field optional, add `,omitzero` (struct-typed fields) or `,omitempty` (scalars). To make it required, add
  `jsonschema:"required"` and omit the omit tag.
- Constraints live in the `jsonschema` tag: `required`, `minLength=1`, `format=uri`, `minItems=1`, `enum=a,enum=b`,
  `pattern=^[a-zA-Z0-9._-]+$`. Descriptions come from the Go doc comment on the field.

## Testing

- Tests are hermetic: no network. Use `httptest` servers and in-memory fakes (see `internal/aur`, `internal/resolver`,
  `internal/generator` for the fake `Source`/`Fetcher`/config patterns).
- Golden-file tests live under each package's `testdata/`. Regenerate with `go test ./internal/generator/... -update`.
  Compare generated-vs-committed **semantically** (parse then compare), not byte-for-byte, because Prettier reformats
  committed YAML/JSON during `task lint` (see `schema.md`).
- `testdata/` directories are excluded from Prettier/yamllint, so intentionally-invalid fixtures are safe to commit.
