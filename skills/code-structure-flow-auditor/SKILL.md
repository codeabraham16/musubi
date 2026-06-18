---
name: code-structure-flow-auditor
description: "Trigger: audit structure, audit flow, architecture review, dependency audit, coupling, cycles, dead code, auditar estructura, auditar flujo. Audits structure and data/control flow across any stack; emits prioritized, evidence-backed findings."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "2.0"
---

## Activation Contract

Use when asked to audit **how a codebase is structured** (module/package organization, cohesion,
coupling, layering, public surface, dead code) or its **flow** (dependency direction, cycles,
entry→exit paths, context/error propagation). Language-agnostic. Do NOT use for line-by-line bug
hunting (that is code review) or for style/lint nits.

## Hard Rules

- Verify every claim against the code before reporting it — never infer structure from names alone.
- Gather evidence with real tooling (see `references/evidence-commands.md`), not guesses. If you
  cannot run a tool, say so and lower the finding's confidence.
- Dependencies must flow one direction. Report any cycle or inward→outward inversion (a core/domain
  module importing an outer IO/transport module) as **HIGH**.
- Always run the **dead-code / orphan** check: modules or symbols imported by nothing and untested.
- Keep **structure** (static shape) and **flow** (dynamic path) as separate report sections.
- Every finding needs: severity (per rubric), evidence (`path:symbol` or command output), one action.
- Do NOT reflexively flag normal patterns: a composition/wiring hub with high fan-out, a large but
  cohesive module, or an entrypoint that does IO are not defects alone. Flag only with a concrete cost.
- Propose the smallest high-impact change; never recommend a rewrite.

## Severity Rubric

| Severity | Criterion |
|----------|-----------|
| HIGH | Breaks the dependency DAG (cycle/inversion), hidden global mutable state, or swallowed errors that lose failures |
| MEDIUM | Cohesion/coupling smell with real maintenance cost (god-file, grab-bag module, leaky boundary) or dead code |
| LOW | Cosmetic/organizational (mixed concerns in one file, naming drift) with small blast radius |

## Decision Gates

| Dimension | Look at | Alarm |
|-----------|---------|-------|
| Organization | one module = one responsibility | `utils`/`common`/`helpers` growing without a theme |
| Coupling | who imports whom (dep graph) | bidirectional deps or cycles |
| Layering | thin entrypoints, logic in the core | domain layer reaching into IO/transport |
| Dead code | reachability | module/symbol imported by nothing, untested |
| Flow | input → transform → output | state mutated at a distance, shared globals |
| Boundaries | encapsulation / public surface | internals exported widely, no anti-corruption layer |
| Context/errors | explicit propagation | errors swallowed, context/cancellation not threaded |

## Execution Steps

1. Map structure: list modules/packages, size, and declared responsibility.
2. Build the dependency graph with the right tool for the stack (`references/evidence-commands.md`);
   detect cycles, inversions, and orphans.
3. Trace 1–3 key input→output flows; note every layer crossed and where state/errors propagate.
4. Score each Decision Gate dimension; attach `path:symbol` or command-output evidence per finding.
5. Emit the report using `assets/report-template.md`, sorted by severity.

## Output Contract

Produce, in this order (shape: `assets/report-template.md`):
- **Summary** — 2–3 lines on structure and flow health + finding counts by severity.
- **Structure** — findings as `severity · evidence · action`.
- **Flow** — findings as `severity · evidence · action`.
- **Top 3 actions** — highest impact / lowest risk, ordered.

## References

- `references/evidence-commands.md` — build the dep graph and find orphans/cycles per ecosystem.
- `assets/report-template.md` — output template for consistent reports.
