# Evidence commands by ecosystem

Use these to gather **real** evidence for the audit. Prefer tooling already in the repo; only
install extra tools with the user's consent. If nothing is available, fall back to reading import
statements and lower the confidence of affected findings.

## Universal (no install)
- Module/package sizes: list source files per directory and their line counts.
- Orphans (works anywhere): for each module, search the repo for importers of it; **zero importers
  (excluding itself and tests) = orphan candidate**. Cross-check for a test file.
- Cycles: most compilers/build tools reject import cycles — a clean build is partial evidence of
  no package-level cycle.

## Go
- Graph: `go list -deps ./...` · per-package imports: `go list -f '{{.ImportPath}} {{.Imports}}' ./...`
- Cycles: `go build ./...` fails on import cycles. Dead code/orphans: `go vet ./...`,
  `staticcheck` (U1000 unused), or grep for importers.

## JavaScript / TypeScript
- Cycles + orphans: `npx madge --circular src` · `npx madge --orphans src` · graph image:
  `npx madge --image graph.svg src`
- Rich rules: `npx depcruise --validate src` (dependency-cruiser). Dead exports: `npx ts-prune`.

## Python
- Graph/cycles: `pydeps <pkg> --show-cycles` · layered rules: `import-linter` (lint-imports).
- Cyclic imports: `pylint --disable=all --enable=cyclic-import <pkg>`. Dead code: `vulture <pkg>`.

## Rust
- Modules/graph: `cargo modules structure` / `cargo modules dependencies`. Deps tree: `cargo tree`.
- Unused deps: `cargo +nightly udeps`. Dead code: `cargo build` warns on unused items.

## Java / Kotlin
- Dependencies & cycles: `jdeps -verify -recursive <jars/classes>` · architecture rules: ArchUnit
  tests. Unused/dead: IDE inspections or `jdeps` summary.

## Reading the graph
- Direction: edges should point from outer (entrypoints, transport) toward inner (domain, core),
  never the reverse. An inner module importing an outer one is an **inversion (HIGH)**.
- Hub: a single composition/wiring module importing many is expected — note it, don't flag it.
- Orphan: a node with no inbound edges (and no test) is dead code — flag it MEDIUM.
