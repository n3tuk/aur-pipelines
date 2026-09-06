---
inclusion: fileMatch
fileMatchPattern: "internal/generator/**|internal/pipeline/**"
---

# Concourse pipeline generation

`internal/pipeline` is a typed model of a Concourse pipeline that marshals to YAML. `internal/generator` builds those
structs from the config and the resolver result. Pipeline generation is **programmatic (typed structs)**, never
templated — keep it that way so output is type-safe and cannot drift.

## Topology (one pipeline per package base, plus cleanup)

Each package base gets its own pipeline (`<base>.yaml`) with three jobs, chained within the pipeline by `passed:`:

1. **build-upload** — `pacman-key --init`/`--populate`, `pacman -Syu`, clone + `makepkg`, then `put` the built package
   to R2. When the package has AUR dependencies, this job also `get`s the shared repository-db resource with
   `trigger: true` so it rebuilds once a dependency is installable.
2. **sign** — `get` artefacts (`passed: [build-upload]`), import the GPG key, `gpg --detach-sign --armor`, `put`
   signatures. Standardised across pipelines.
3. **repository** — `serial: true` (+ `serial_groups: [repository]`); `get` artefacts + signatures + repo DB,
   `repo-add`, write `.db`/`.files` as **copies not symlinks** (buckets lack symlink support), `put` the DB. Carries the
   notify hooks.

A separate `cleanup.yaml` runs daily (a `time` resource) and removes bucket objects the repository database no longer
references (DB is the source of truth; read before any delete).

Cross-pipeline dependency ordering uses the shared **repository-db** resource as the trigger — never the raw package
upload — because the DB only changes after the serial repository job completes, which avoids a race where a dependent
builds before its dependency is installable. Note `passed:` and `serial_groups` are within-pipeline only; cross-pipeline
ordering is resource-version-based.

## R2 access

R2 is accessed via `s3` resources (get/put) using the configured bucket and an S3-compatible endpoint built from
`((r2.account-id))`. Cleanup uses the AWS CLI directly against the endpoint.

## Secret conventions (credential-manager `((...))`)

Never embed secret values. The references emitted are:

- `((gpg.signing-key))`, `((gpg.signing-passphrase))` — GPG signing key + passphrase.
- `((r2.account-id))`, `((r2.access-key-id))`, `((r2.secret-access-key))` — Cloudflare R2.
- `((webhooks/<secret>.url))` — per-webhook endpoint, from the webhook entry's `secret` key.
- `((repositories/<secret>.username))` / `((repositories/<secret>.password))` — per-image registry pull credentials,
  from an image's optional `secret` key.

These reference strings trip `gosec` G101; annotate with `//nolint:gosec // credential-manager reference, not a secret`.

## Container images

All `container` stages (`build`, `sign`, `upload`, `cleanup`, `notify`) are optional with load-time defaults (see the
README "Container images" table). The generator builds every image via `g.image(cfg.Container.<stage>)`, which emits
`username`/`password` from `((repositories/<secret>.*))` when the image has a `secret`, and pulls anonymously otherwise.

## Notifications

Webhooks are selected by `type` (`build`|`cleanup`) and `when` (`on_success`|`on_failure`); the generator attaches
matching webhooks to the corresponding job hook. Distinct success/failure webhooks mean there is no conditional
templating.

- Template body and header values use **shell `${VAR}` expansion at run time** (NOT Concourse — Concourse only expands
  `((...))`). The notify task exports the variables and the shell expands them. Available: `${PACKAGE_NAME}`,
  `${PACKAGE_VERSION}` (build), `${REPOSITORY}` (cleanup), `${PIPELINE_STATUS}`, `${PIPELINE_URL}`.
- `${PIPELINE_URL}` is built from build metadata, which **Concourse does not expose to task environments** (that is an
  intentional anti-pattern per the Concourse maintainers, still true on the latest release). So the generator adds a
  `metadata` resource type (`swce/metadata-resource`) and a `meta` resource, fetches it (`get: meta`) in each
  notification-carrying job, and the notify task reads `meta/atc-external-url`, `meta/build-team-name`, etc. from files
  via a `meta()`/`build_url()` shell helper. The `metadata` resource and its `get` are only added when a webhook of the
  matching type is configured. Never revert to referencing `${BUILD_TEAM_NAME}`/`${ATC_EXTERNAL_URL}` directly — they
  are unset in tasks and abort the script under `set -u`.
- The notify script is **best-effort**: `set -u` only (no `-e`/`pipefail`), and each `curl` is guarded with `|| true` so
  one failing/unreachable endpoint neither aborts the others nor fails the hook. `--fail` is kept for log visibility.
- Multiple webhooks sharing a `type`+`when` each get their own `WEBHOOK_URL_<n>` (plain `WEBHOOK_URL` when only one).
- Header/body values are placed in **double quotes** (so `${VAR}` expands); escape `\`, `"`, and backtick.

## YAML block-style gotcha (readability of scripts)

`go.yaml.in/yaml/v3` only emits a readable `|-` block scalar when **no line has trailing whitespace**. A single trailing
space (common in user templates — for example a space after `{{ else }}`) forces the whole string to an escaped
single-line double-quoted scalar. This is why templates are passed via env-var **params** rather than interpolated into
the script body — it keeps the generated script free of user-controlled trailing whitespace so it renders as a clean
multi-line block. Keep new generated scripts free of trailing whitespace, and keep user content out of the script body.
