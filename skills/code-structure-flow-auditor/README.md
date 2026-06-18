# code-structure-flow-auditor

A language-agnostic skill that audits **how a codebase is structured** and **how data and control
flow through it**, then emits a prioritized, evidence-backed report. Built for agents (Claude Code
& compatible), not for line-by-line bug hunting.

## What it checks

- **Structure** — module organization & cohesion, coupling (dependency graph), layering, public
  surface/encapsulation, and **dead code / orphan modules**.
- **Flow** — dependency direction (one-way DAG), cycles & inversions, input→output paths, and
  context/error propagation.

Each finding carries a **severity** (HIGH / MEDIUM / LOW via an explicit rubric), **evidence**
(`path:symbol` or real tool output), and **one concrete, minimal action**. It refuses to recommend
rewrites and avoids flagging normal patterns (wiring hubs, large-but-cohesive modules).

## Why it's different

- **Evidence over guesses.** It builds the dependency graph with real tooling per ecosystem
  (`go list`, `madge`, `pydeps`, `cargo modules`, `jdeps`) — see [references/evidence-commands.md](references/evidence-commands.md).
- **Reproducible.** A severity rubric and fixed decision gates make two runs agree.
- **Consistent output.** Every report follows [assets/report-template.md](assets/report-template.md).

## Install

Copy the `code-structure-flow-auditor/` folder into your agent's skills directory (e.g.
`.claude/skills/`). The agent picks it up via the `name`/`description` triggers.

## Use

Ask your agent to *"audit the structure and flow of this codebase"* (also triggers on
*auditar estructura / auditar flujo*). It returns a Summary, Structure findings, Flow findings, and
a Top-3 action list.

## Files

| File | Role |
|------|------|
| `SKILL.md` | The runtime contract (LLM-facing). |
| `assets/report-template.md` | Output shape for consistent reports. |
| `references/evidence-commands.md` | Per-ecosystem commands to gather real evidence. |

## Publishing to the Musubi catalog

[`catalog-entry.json`](catalog-entry.json) is the ready-to-publish catalog entry for the
distributable Musubi catalog (repo `codeabraham16/musubi-skills`, `index.json`). To publish:

1. Append the object in `catalog-entry.json` to the `entries` array of that repo's `index.json`.
2. Validate: `musubi catalog validate index.json` (must print `Catalogo valido`).
3. Commit & push in `musubi-skills`.

Validated against the live catalog (38 entries) with `musubi catalog validate`. The entry is
language-agnostic (lists all programming stacks) and `rules_url` points at this skill's `SKILL.md`.

---

Author: Musubi · License: Apache-2.0.
