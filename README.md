# aur-pipelines (AUR Pipeline Generator for Concourse)

[![CodeQL](https://github.com/n3tuk/aur-pipelines/actions/workflows/codeql.yaml/badge.svg)](https://github.com/n3tuk/aur-pipelines/actions/workflows/codeql.yaml)
[![Code Coverage](https://codecov.io/gh/n3tuk/aur-pipelines/branch/main/graph/badge.svg?token=GSu1DCSng1)](https://codecov.io/gh/n3tuk/aur-pipelines)
[![Draft Release](https://github.com/n3tuk/aur-pipelines/actions/workflows/draft-release.yaml/badge.svg?branch=main)](https://github.com/n3tuk/aur-pipelines/actions/workflows/draft-release.yaml)

`aur-pipelines` is a Go CLI tool for the purpose of building out Concourse CI pipelines for selected Arch Linux AUR
packages. It is designed to automate the process of creating and maintaining pipelines for AUR packages, including
dependency handling within the AUR, allowing users to easily set up continuous integration workflows for their packages.

## How it works

When `aur-pipelines` is run with the path to a YAML file with a list of AUR package names, it will:

1. Verify that that package exists, and check to see if the package name matches the package base (i.e. the name
   provided is part of a multi-package build), and uses the package base name for the pipeline generation if it does;
2. Reviews all the dependencies of the package, and if any one of them is also part of the AUR, adds them to the list,
   running this recursively until all dependencies have been resolved (or the dependencies become circular);
3. Generates a Concourse CI pipeline YAML file that includes jobs to:
   - Initialise the build image and run `pacman-key --init` to initialise the keyring and populate it with the Arch
     Linux master keys, update all packages with `pacman -Syu --noconfirm` to ensure the build environment is up to
     date, and then build clone the AUR package repository and built the package using the `makepkg` command, providing
     all built package files as artifacts for the next job.
   - Initialise the sign image, authenticate with OpenBao using AppRole credentials, fetch the signing key from OpenBao
     and run `gpg --import` to import the signing key, and then sign the provided packages using
     `gpg --detach-sign --armor` to create detached signature files for each package, providing all signed package files
     as artifacts for the next job.
   - Initialise the upload image, authenticate with Cloudflare R2 using the provided credentials, upload the signed
     packages to the specified R2 bucket, and, once successfully uploaded, download the repository files from the R2
     bucket, run `repo-add` to update the repository database with the newly uploaded packages, and then upload the
     updated repository files back to the R2 bucket.
4. Send a notification to a specified webhook URL (e.g. ntfy, Discord, Slack, etc.), using a template with the results
   of the pipeline run interpolated into the message, to notify the user of either a successful package upload, or the
   failure of any of the jobs in the pipeline. The notification will include details of the success or failure of the
   pipeline run, the version of the package that was built, and any other relevant information, such as what failed in
   the event of a failure.

> [!WARNING]
>
> The uploading of the signed packages to the R2 bucket and the updating of the repository database must be completed
> sequentially, or one job may override the changes made by another job. Concourse therefore runs this job as a serial
> job to ensure that the repository is updated safely.

## Repository Cleanup

An Arch Linux Package Repository is only capable of supporting a single version of a package at any one time. If a new
version of a package is uploaded to the repository, the old version of the package must be removed from the repository
to save space.

A separate pipeline is created which is triggered on a daily basis to check the repository for any packages that have
more than one version, and if so, check that the newer version exists in the package database first, and, if so, removes
the older versions of the package from the bucket. This ensures that the repository is kept clean and only contains the
latest versions of packages, while also ensuring that the repository is not left in a broken state by removing a package
version that is still referenced in the package database.

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
        value: https://assets.n3t.uk/images/concourse-256x256.png
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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on setting up the development environment, running tests,
linting, and submitting pull requests.

## Authors

- Jonathan Wright (<jon@than.io>)

## Licence

[MIT](LICENSE)
