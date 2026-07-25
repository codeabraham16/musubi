# Contributing to Musubi

<strong>English</strong> · <a href="CONTRIBUTING.md">Español</a>

Thanks for your interest in improving Musubi! This guide summarizes how to propose changes.

## Before you start

- For **bugs** or **ideas**, open an [issue](https://github.com/codeabraham16/musubi/issues)
  first using the matching template. Discussing the approach before writing code saves work.
- For small, obvious changes (typos, docs), you can go straight to a PR.

## Development environment

You need **Go 1.26+**. Nothing else is required: the database is embedded SQLite
(pure Go, no CGo) and embeddings are optional.

```bash
git clone https://github.com/codeabraham16/musubi.git
cd musubi
go build ./cmd/musubi   # builds the binary
```

## Before opening a PR

Run locally the same things CI runs, and make sure everything passes:

```bash
go vet ./...
go build ./...
go test -race ./...
```

If you add new logic, **add tests**. The project aims to keep or raise coverage
(`go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`).

## Conventions

- **Language:** comments, commit messages and CLI messages in **Spanish**; code
  identifiers and MCP tool names in **English** (e.g. `musubi_save_observation`).
- **Commits:** short imperative title describing the *what*. If it closes an issue,
  reference it (`#NN`).
- **Go style:** `gofmt` (idiomatic), errors wrapped with `%w`, no `panic` in production
  code.
- **Local-first:** no change may require a mandatory external service. Network
  dependencies (embeddings, etc.) are always optional and have a fallback.

## Versioning and changelog

The project follows [Semantic Versioning](https://semver.org/). If your change is
user-visible, add an entry to the `[Unreleased]` section of
[CHANGELOG.md](CHANGELOG.md).

## Cutting a release

The version has **a single source of truth**: the [`VERSION`](VERSION) file at the root.
Everything derives from it — the tag (verified at release time), `versioninfo.json` (verified
by test) and `musubi version` (injected from the tag). There is nothing to sync by hand.

1. Bump **`VERSION`** to `X.Y.Z` and update **`cmd/musubi/versioninfo.json`** to match
   (the numeric fields of `FixedFileInfo` and the `FileVersion`/`ProductVersion` strings
   of `StringFileInfo`, which carry a fourth component: `X.Y.Z.0`). The
   `TestVersioninfoMatchesVERSION` test fails if they diverge. **Don't edit the `.syso`
   files by hand**: `release.yml` regenerates them from `versioninfo.json` with a pinned
   `goversioninfo`.
2. Move the contents of `[Unreleased]` into a dated `[X.Y.Z]` section in `CHANGELOG.md`
   and update the comparison links at the bottom of the file.
3. Commit, merge to `main` and create the tag: `git tag -a vX.Y.Z -m "..." && git push origin vX.Y.Z`.
   The [`release.yml`](.github/workflows/release.yml) workflow **aborts if the tag does not
   match `VERSION`**, regenerates the Windows resource and builds the cross-platform binaries
   (Windows/Linux/macOS, amd64+arm64) with SHA-256 checksums, and publishes the release.

## License

By contributing you agree that your contribution is released under the project's
[MIT](LICENSE) license.
