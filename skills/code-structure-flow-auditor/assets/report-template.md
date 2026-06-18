# Structure & Flow Audit — {project}

## Summary
{2–3 lines on structure and flow health.}
**Findings:** HIGH {n} · MEDIUM {n} · LOW {n}

## Structure
| Sev | Finding | Evidence | Action |
|-----|---------|----------|--------|
| HIGH/MED/LOW | {what} | `{path:symbol}` or `{command output}` | {smallest high-impact fix} |

## Flow
| Sev | Finding | Evidence | Action |
|-----|---------|----------|--------|
| HIGH/MED/LOW | {what} | `{path:symbol}` or `{trace}` | {smallest high-impact fix} |

> Note positives too: a clean DAG, one-directional deps, or threaded context/errors are worth stating.

## Top 3 actions
1. {highest impact / lowest risk}
2. {…}
3. {…}

## Method
- Dependency graph built with: `{tool/command}`
- Cycles/orphans checked with: `{tool/command}`
- Confidence caveats: {anything not verifiable by tooling}
