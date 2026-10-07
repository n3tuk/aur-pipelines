# Contributing to aur-pipelines

Thank you for your interest in contributing. This document describes how to set up a development environment, run the
tests and linters, regenerate the configuration schema, and submit changes.

## Prerequisites

- [Go](https://go.dev/) (the version pinned in [`go.mod`](go.mod) or newer).
- [Task](https://taskfile.dev/) — the task runner used for all development workflows.
- [GoReleaser](https://goreleaser.com/) — used to build the binary.
- [golangci-lint](https://golangci-lint.run/) — used for Go linting and formatting.
- [pre-commit](https://pre-commit.com/) — installs the Git hooks used to catch issues before committing.
- [Prettier](https://prettier.io/), [markdownlint](https://github.com/igorshubovych/markdownlint-cli), and
  [yamllint](https://www.yamllint.com/) — used to lint documentation and configuration files.

The static-analysis step additionally uses [CodeQL](https://codeql.github.com/), [Snyk](https://snyk.io/),
[actionlint](https://github.com/rhysd/actionlint), and [zizmor](https://github.com/zizmorcore/zizmor); these are only
required if you run `task analyse`.

## Project layout

| Path                  | Purpose                                                             |
| --------------------- | ------------------------------------------------------------------- |
| `cmd/aur-pipelines/`  | The command-line entry point and Cobra command wiring.              |
| `internal/cli/`       | Dependency-light CLI support (for example build-version reporting). |
| `internal/config/`    | Configuration structs, loading, and JSON Schema validation.         |
| `internal/aur/`       | AUR RPC client and `.SRCINFO` fetching and parsing.                 |
| `internal/resolver/`  | Recursive AUR dependency resolution.                                |
| `internal/pipeline/`  | Typed Concourse pipeline model and YAML marshalling.                |
| `internal/generator/` | Pipeline generation and output writing.                             |
| `tools/schema/`       | Generator for the configuration JSON Schema.                        |
| `schemas/`            | The generated, published configuration JSON Schema.                 |

## Development workflow

The default Task target runs the full development loop — linting, formatting, testing, building, and static analysis:

```console
$ task
```

You can also run the individual stages:

| Task             | Description                                                                        |
| ---------------- | ---------------------------------------------------------------------------------- |
| `task lint`      | Lint Go, JSON, YAML, and Markdown files.                                           |
| `task test`      | Run the unit tests.                                                                |
| `task build`     | Build the application binary to `bin/aur-pipelines` using GoReleaser.              |
| `task analyse`   | Run static code and security analysis (CodeQL, Snyk, actionlint, zizmor, schemas). |
| `task go:schema` | Regenerate the configuration JSON Schema.                                          |
| `task clean`     | Remove temporary build and test artefacts.                                         |

Run `task --list` to see every available task.

## Testing

Run the unit tests with:

```console
$ task test
```

Tests avoid network access: the AUR RPC client and the pipeline generator are exercised through in-memory fakes and
`httptest` servers rather than the live AUR. Please keep new tests hermetic and deterministic.

Some tests use golden files (under each package's `testdata/` directory) to assert exact generated output. When a change
intentionally alters that output, regenerate the golden files:

```console
$ go test ./internal/generator/... -update
```

## Linting and formatting

All Go code must pass `golangci-lint` with the repository configuration, and be formatted by the configured formatters:

```console
$ task lint
```

Documentation and configuration files are formatted and linted with Prettier, markdownlint, and yamllint. British
English spelling is used throughout (the `misspell` linter is configured for the UK locale).

## Regenerating the configuration schema

The configuration JSON Schema in `schemas/aur-pipelines.json` is reflected from the configuration structs in
`internal/config/config.go`. Whenever those structs change, regenerate the schema and commit the result:

```console
$ task go:schema
```

A test fails if the committed schema is out of date, so remember to run this before opening a pull request.

## Submitting changes

1. Create a branch for your change (do not commit directly to `main`).
2. Make your change, keeping it focused and adding tests for new behaviour.
3. Run `task` to confirm linting, tests, and the build all pass.
4. Commit using clear, conventional commit messages (the `pre-commit` hooks will run automatically).
5. Open a pull request describing the change, what was tested, and any follow-up work.

## Licence

By contributing, you agree that your contributions will be licensed under the [MIT Licence](LICENSE).
