---
name: adding-config-field
description:
  Step-by-step procedure for adding, changing, or removing a field in the aur-pipelines configuration (internal/config),
  keeping the struct, JSON Schema, validation, fixtures, generator wiring, golden tests, and README all in step. Use
  when a task involves changing what the config YAML accepts or how a config value flows into generated pipelines.
---

# Adding or changing a configuration field

Changing the config surface touches several layers that must stay consistent, or tests/lint will fail. Follow this
sequence. (See the `schema.md` and `go.md` steering files for the underlying rules.)

## 1. Change the struct

In `internal/config/config.go`, add/modify the field on the relevant struct with all four tag families: `json`,
`mapstructure`, `yaml`, and `jsonschema`.

- Optional field: add `,omitzero` (struct-typed) or `,omitempty` (scalar) to the `json` tag.
- Required field: add `jsonschema:"required"` and omit the omit tag.
- Constraints in the `jsonschema` tag: `minLength=1`, `format=uri`, `enum=a,enum=b`, `pattern=^[a-zA-Z0-9._-]+$`,
  `minItems=1`.
- Write a clear Go doc comment on the field — it becomes the schema `description`.
- Keep all types in the single `type ( ... )` block; keep any new consts in the single `const ( ... )` block.

## 2. Defaults (if the field is optional with a default)

Add the default handling in `internal/config/defaults.go` (`applyDefaults`), following the existing container-image
pattern. Defaults are applied after validation, so the loaded `Config` is always populated.

## 3. Regenerate the schema

Run `task go:schema`. This rewrites both `schemas/aur-pipelines.json` and `internal/config/schema/aur-pipelines.json`.
Never edit those files by hand. Verify the `required`/`enum`/`pattern` came out as intended.

## 4. Update fixtures

- `internal/config/testdata/valid.yaml` and `minimal.yaml` — add the new field where appropriate so they still load.
- Add an `invalid-<thing>.yaml` fixture demonstrating a rejected value (bad enum, bad pattern, missing required), and
  register it in `TestLoadRejectsInvalidConfigs` in `validate_test.go`.
- If you added a required field, add it to the other `invalid-*.yaml` fixtures so they only fail for their intended
  reason (not also for the new missing field).

## 5. Wire it through the generator (if it affects output)

Thread the field into `internal/generator` (see `pipelines.md` for conventions). For secrets, emit a `((...))` reference
and add a `//nolint:gosec` where needed. For images, go through `g.image(...)`.

## 6. Update tests

- Add/adjust config tests in `internal/config` (loading, defaulting, rejection).
- If generator output changed, update the generator test config, add a focused behavioural test, and regenerate golden
  files with `go test ./internal/generator/... -update`.
- Watch `cyclop` (≤15): extract assertions into helpers if a test grows too complex.
- Watch `goconst`: extract repeated literals (including in tests) into constants.

## 7. Update documentation

Update `README.md`: the config example, and the relevant reference (secrets table, container-image table, webhook
variables, etc.). Keep British spelling. If the change alters an established behaviour, also update the matching
`.kiro/steering/*.md`.

## 8. Verify

Run `task lint` (zero issues), `task test`, and `task build`. All must pass before presenting the change.
