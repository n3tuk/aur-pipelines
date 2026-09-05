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
5. On completion of the repository job, send a notification to each configured webhook URL (for example ntfy, Discord,
   or Slack), passing the configured headers and message template through to Concourse to interpolate at run time, to
   report either a successful upload or the failure of a job in the pipeline.

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
  - name: ntfy
    url: https://ntfy.sh/your-webhook-url
    headers:
      - name: Title
        value: "{{ .PackageName }} v{{ .PackageVersion }} Build Notification"
      - name: Icon
        value: https://assets.n3t.uk/concourse.png
      - name: Click
        value: "{{ .PipelineURL }}"
      - name: Actions
        value: view, View Pipeline, {{ .PipelineURL }}
      - name: Priority
        value: low
      - name: Tags
        value: aur, concourse, {{ .PackageName }}, {{ .PackageVersion }}
    template: >-
      {{ if eq .PipelineStatus "succeeded" }} Package {{ .PackageName }} v{{ .PackageVersion }} has been successfully
      built and uploaded to the repository. {{ else }} Package {{ .PackageName }} v{{ .PackageVersion }} failed to build
      or upload. Please check the pipeline logs for more information at {{ .PipelineURL }}. {{ end }}

packages:
  - name: kalc-bin
  - name: ntfysh-bin
  - name: keybase-bin
  - name: cloudflared-bin
  - name: snyk
  - name: codeql
```

The configuration is validated against a [JSON Schema](schemas/aur-pipelines.json) when it is loaded, so structural
mistakes (missing required fields, unknown keys, empty values) are reported with a clear error before any pipelines are
generated. Editors that understand the `yaml-language-server` schema directive can use the same schema for completion
and inline validation.

The `webhook` header values and the `template` are pass-through strings: `aur-pipelines` does not interpret them. Any
templating within them (for example `{{ .PackageName }}`) is emitted verbatim into the generated pipeline for Concourse
to interpolate at run time.

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
