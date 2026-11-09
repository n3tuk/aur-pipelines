---
inclusion: fileMatch
fileMatchPattern: "internal/config/**|tools/schema/**|schemas/**"
---

# Configuration schema management

The configuration JSON Schema is **reflected from the Go config structs**, not hand-written. `tools/schema/main.go` uses
`github.com/invopop/jsonschema` to reflect `internal/config.Config` and writes the result to two places from a single
run:

- `schemas/aur-pipelines.json` — the canonical, published artefact (referenced by editors via the `$id`).
- `internal/config/schema/aur-pipelines.json` — an embedded copy validated against at runtime.

A test asserts the two files are byte-identical, so they can never drift.

## How to change the schema

You never edit the JSON files by hand. Instead:

1. Change the struct fields and their `jsonschema` tags in `internal/config/config.go` (see `go.md` for tag semantics —
   `required`, `enum`, `pattern`, `minLength`, `minItems`, `format`; optionality via `,omitzero`/`,omitempty`).
2. Field **descriptions come from the Go doc comment** on the field (invopop `AddGoComments`), so write a clear comment.
3. Run `task go:schema` to regenerate both files.
4. Update fixtures and validation tests (see the `adding-config-field` skill for the full sequence).

## Runtime validation

`internal/config` embeds the schema and validates the **raw parsed document** (`viper.AllSettings()`), not the decoded
struct. This is deliberate: struct decoding silently drops unknown keys, so validating the struct would make
`additionalProperties: false` useless. Validating the raw document reports unknown keys, missing required fields, bad
enum/pattern values, and empty values against what the user actually wrote.

Defaults (e.g. container images) are applied **after** validation, in `internal/config` (`applyDefaults`), so the loaded
`Config` is always fully populated while validation still runs against the user's literal input.

## Generator reflection detail (AddGoComments)

`generate()` in `tools/schema/main.go` calls `reflector.AddGoComments(moduleRoot, "./internal/config")`. The base must
be the **module root** (`github.com/n3tuk/aur-pipelines`) and the path a **relative** subdir; invopop joins them to form
the package path used as comment-map keys. An absolute path or the full package path as the base produces mangled keys
and no descriptions. Because it reads source relative to the working directory, the schema golden test uses `t.Chdir` to
the repository root (and is therefore `//nolint:paralleltest`).

## Golden test: compare semantically, not byte-for-byte

The schema and generator golden tests must compare the generated output against the committed files **semantically**
(unmarshal both, then compare structures) — never as raw bytes. Prettier reformats the committed JSON/YAML during
`task lint` (for example compacting short arrays onto one line), which `json.MarshalIndent` never produces, so a
byte-for-byte comparison will spuriously fail after any lint run.
