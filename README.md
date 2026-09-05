# aur-pipelines (AUR Pipeline Generator for Concourse)

[![CodeQL](https://github.com/n3tuk/aur-pipelines/actions/workflows/codeql.yaml/badge.svg)](https://github.com/n3tuk/aur-pipelines/actions/workflows/codeql.yaml)
[![Code Coverage](https://codecov.io/gh/n3tuk/aur-pipelines/branch/main/graph/badge.svg?token=GSu1DCSng1)](https://codecov.io/gh/n3tuk/aur-pipelines)
[![Draft Release](https://github.com/n3tuk/aur-pipelines/actions/workflows/draft-release.yaml/badge.svg?branch=main)](https://github.com/n3tuk/aur-pipelines/actions/workflows/draft-release.yaml)

`aur-pipelines` is a Go CLI tool for the purpose of building out Concourse CI pipelines for selected Arch Linux AUR
packages. It is designed to automate the process of creating and maintaining pipelines for AUR packages, including
dependency handling within the AUR, allowing users to easily set up continuous integration workflows for their packages.

## How it works

When `aur-pipelines` is run with the path to a YAML file listing a set of AUR package names, it will:

1. Query the [AUR RPC interface](https://wiki.archlinux.org/title/Aurweb_RPC_interface) to verify that each package
   exists, and map the package name to its package base (i.e. where the name provided is part of a multi-package, or
   _split_, build), using the package base name for pipeline generation.
2. Resolve the dependencies of each package — its runtime (`depends`), build-time (`makedepends`), and test-time
   (`checkdepends`) dependencies, with optional dependencies (`optdepends`) deliberately excluded — and, for any
   dependency that is itself an AUR package (including one satisfied through another AUR package's `provides`), add it
   to the set and repeat recursively until every dependency has been resolved. Dependencies that are not in the AUR are
   assumed to be provided by the official repositories and are skipped. Circular dependencies are detected and reported
   rather than looping indefinitely.
3. Generate one Concourse CI pipeline per package base, each written to its own file (`<package-base>.yaml`). Every
   pipeline contains three jobs:
   - **build-upload** — initialise the build image, run `pacman-key --init` and `pacman-key --populate archlinux` to
     initialise the keyring with the Arch Linux master keys, update the environment with `pacman -Syu --noconfirm`,
     clone the AUR package repository and build it with `makepkg`, then upload the built package files to the R2 bucket.
   - **sign** — download the built package files from the bucket, import the signing key, sign each package with
     `gpg --detach-sign --armor` to produce detached, armoured signatures, then upload the signature files back to the
     bucket. This job is standardised across every pipeline.
   - **repository** — download the built packages, their signatures, and the current repository database from the
     bucket, run `repo-add` to add the packages to the database, and upload the updated database back to the bucket.
     This is the serial job (see the warning below).
4. Generate a separate [daily repository-cleanup pipeline](#repository-cleanup).
5. On completion of the repository job, send a notification to each configured webhook whose `type` and `when` match
   (for example ntfy, Discord, or Slack). The notification task expands a small set of shell variables (such as
   `${PACKAGE_NAME}` and `${PIPELINE_URL}`) into the configured headers and message body at run time — see
   [Webhook notifications](#webhook-notifications) — to report either a successful upload or the failure of a job.

### Secrets and Concourse credential management

`aur-pipelines` never embeds secrets in the pipelines it generates. Instead, generated pipelines reference secrets using
Concourse's [credential-manager](https://concourse-ci.org/creds.html) syntax (for example `((gpg.signing-key))`), which
Concourse resolves against its configured credential manager (OpenBao) at run time. This keeps secret values out of the
configuration file, the generated pipeline files, and the pipeline definitions stored by Concourse, and lets the
credential manager own access control and auditing.

The following secrets must be populated in OpenBao (under the path Concourse is configured to look up for the team and
pipeline) before the generated pipelines can run:

| Reference                    | Purpose                                                             |
| ---------------------------- | ------------------------------------------------------------------- |
| `((gpg.signing-key))`        | ASCII-armoured GPG private key used to sign packages.               |
| `((gpg.signing-passphrase))` | Passphrase for the GPG signing key.                                 |
| `((r2.account-id))`          | Cloudflare R2 account ID, used to build the S3-compatible endpoint. |
| `((r2.access-key-id))`       | Cloudflare R2 access key ID.                                        |
| `((r2.secret-access-key))`   | Cloudflare R2 secret access key.                                    |
| `((webhook.url))`            | Notification webhook URL (including any embedded token).            |

### Dependency ordering between pipelines

Because each package base has its own pipeline, a package that depends on another AUR package must not be built until
that dependency is actually installable from the repository. To guarantee this, the dependent package's build-upload job
is triggered by a change to the shared repository database, which is only updated once the dependency's serial
repository job has completed. Triggering on the database — rather than on the raw package upload — ensures the
dependency is present in the repository before the dependent package attempts to build against it.

> [!WARNING]
>
> The updating of the repository database must be completed sequentially, or one job may override the changes made by
> another. The repository job is therefore configured as a serial job so that Concourse updates the database safely.

## Repository Cleanup

An Arch Linux package repository can only reference a single version of a package at any one time. When a new version of
a package is uploaded, the older version should be removed from the bucket to save space.

A separate pipeline (`cleanup.yaml`) is generated and triggered on a daily basis. It downloads the current repository
database, reads the exact package filename that the database references for each package, and removes from the bucket
any package file (and its signature) that the database no longer references. Because the database is the source of truth
for the current version — and is only written by the serial repository job — anything it does not reference is a
superseded version that is safe to delete. Reading the database before any removal ensures the repository is never left
referencing a package that has been deleted.

## Bucket Repository Handling

In a normal Arch Package Repository, the `.db` file is a symlink to the `.db.tar.gz` file, and the `.files` file is a
symlink to the `.files.tar.gz` file (e.g. `private.db` becomes a symlink to `private.db.tar.gz`). However, buckets do
not support symlinks, so the pipeline will create the `.db` and `.files` files as copies of the `.db.tar.gz` and
`.files.tar.gz` files, respectively, to ensure that the repository is usable by `pacman`.

## Configuration

`aur-pipelines` takes a YAML file which contains a list of AUR package names, and generates a Concourse CI pipeline YAML
file based on those packages and their dependencies. The input YAML file should have the following structure:

```yaml
---
container:
  build:
    image: archlinux
    tag: base-devel
  sign:
    image: archlinux
    tag: base-devel
  upload:
    image: archlinux
    tag: base-devel

bucket:
  name: your-bucket-name
  repository: private

webhook:
  - name: ntfy-success
    type: build
    when: on_success
    url: https://ntfy.sh/your-webhook-url
    headers:
      - name: Title
        value: ${PACKAGE_NAME} v${PACKAGE_VERSION} Built
      - name: Icon
        value: https://assets.n3t.uk/concourse.png
      - name: Click
        value: ${PIPELINE_URL}
      - name: Actions
        value: view, View Pipeline, ${PIPELINE_URL}
      - name: Priority
        value: low
      - name: Tags
        value: aur, concourse, ${PACKAGE_NAME}, ${PACKAGE_VERSION}
    template: >-
      Package ${PACKAGE_NAME} v${PACKAGE_VERSION} has been successfully built.
  - name: ntfy-failure
    type: build
    when: on_failure
    url: https://ntfy.sh/your-webhook-url
    headers:
      - name: Title
        value: ${PACKAGE_NAME} v${PACKAGE_VERSION} Build Failure
      - name: Icon
        value: https://assets.n3t.uk/concourse.png
      - name: Click
        value: ${PIPELINE_URL}
      - name: Actions
        value: view, View Pipeline, ${PIPELINE_URL}
      - name: Priority
        value: high
      - name: Tags
        value: aur, concourse, ${PACKAGE_NAME}, ${PACKAGE_VERSION}
    template: >-
      Package ${PACKAGE_NAME} v${PACKAGE_VERSION} failed to build or upload. Please check the pipeline logs at
      ${PIPELINE_URL} for additional information on why this job has failed.
  - name: ntfy-cleanup
    type: cleanup
    when: on_failure
    url: https://ntfy.sh/your-webhook-url
    headers:
      - name: Title
        value: Repository Cleanup Notification
      - name: Icon
        value: https://assets.n3t.uk/concourse.png
      - name: Click
        value: ${PIPELINE_URL}
      - name: Actions
        value: view, View Pipeline, ${PIPELINE_URL}
      - name: Priority
        value: high
      - name: Tags
        value: aur, concourse, cleanup
    template: >-
      The daily cleanup of the ${REPOSITORY} repository has failed to complete. Please check the pipeline logs at
      ${PIPELINE_URL} for additional information on why this job has failed.

packages:
  - name: kalc-bin
  - name: ntfysh-bin
  - name: keybase-bin
  - name: cloudflared-bin
  - name: snyk
  - name: codeql
```

The configuration is validated against a [JSON Schema](schemas/aur-pipelines.json) when it is loaded, so structural
mistakes (missing required fields, unknown keys, invalid `type`/`when` values, empty values) are reported with a clear
error before any pipelines are generated. Editors that understand the `yaml-language-server` schema directive can use
the same schema for completion and inline validation.

### Webhook notifications

Each `webhook` entry is selected for a job by two keys:

- `type` — `build` for the per-package build pipelines, or `cleanup` for the daily repository-cleanup pipeline.
- `when` — `on_success` or `on_failure`, selecting which job outcome the webhook fires on.

This means a separate webhook entry is configured for each combination you want to be notified about (for example a
success and a failure webhook for builds), which removes the need for any conditional logic inside the templates: each
entry's message already applies to exactly one outcome.

The `template` body and the header `value` fields are pass-through strings in which shell-style variables are expanded
by the notification task at run time. `aur-pipelines` does not otherwise interpret them; they are emitted into the
generated pipeline as written. The following variables are available:

| Variable             | Available in      | Description                                                  |
| -------------------- | ----------------- | ------------------------------------------------------------ |
| `${PACKAGE_NAME}`    | `build`           | The package base being built.                                |
| `${PACKAGE_VERSION}` | `build`           | The version of the built package, derived from its filename. |
| `${REPOSITORY}`      | `cleanup`         | The repository (database) name.                              |
| `${PIPELINE_STATUS}` | `build` `cleanup` | The job outcome: `succeeded` or `failed`.                    |
| `${PIPELINE_URL}`    | `build` `cleanup` | The URL of the Concourse build that sent the notification.   |

## Usage

Generate the pipelines for the packages listed in a configuration file:

```console
$ aur-pipelines generate config.yaml
Generated 3 pipeline(s) in pipelines:
  - cleanup.yaml
  - kalc-bin.yaml
  - ntfysh-bin.yaml
```

By default the generated pipelines are written to the `pipelines/` directory, one YAML file per pipeline (one per
package base, plus `cleanup.yaml`). Each file is applied to Concourse independently, for example with
`fly set-pipeline`.

| Flag             | Default     | Description                                                       |
| ---------------- | ----------- | ----------------------------------------------------------------- |
| `--output`, `-o` | `pipelines` | Directory to write the generated pipeline files to.               |
| `--dry-run`      | `false`     | Print the generated pipelines to standard output without writing. |

Use `--dry-run` to preview the output without writing any files:

```console
$ aur-pipelines generate config.yaml --dry-run
```

Print the build version information with `aur-pipelines version`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on setting up the development environment, running tests,
linting, and submitting pull requests.

## Authors

- Jonathan Wright (<jon@than.io>)

## Licence

[MIT](LICENSE)
