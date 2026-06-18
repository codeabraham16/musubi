# Roadmap: adopción de ideas de spec-kit en Musubi

Este documento es la **fuente de verdad** del esfuerzo de incorporar tres capacidades
inspiradas en [github/spec-kit](https://github.com/github/spec-kit), adaptadas al ADN de
Musubi. Es Fase 0 (groundwork): fija contratos y secuencia antes de codear.

## Principio rector

Musubi se vuelve **memoria + orquestación persistente, multi-agente** — sin dejar de ser
local-first y model-free.

## Contrato de ejecución (NO negociable)

> **Musubi orquesta y persiste; el agente ejecuta.**

El daemon **nunca** corre shell, red ni prompts por un step de workflow. Musubi define el
grafo, lo valida, persiste el estado del run en SQLite y le dice al agente *qué step(s)
están listos*; el agente (Claude u otro) ejecuta y reporta el resultado. Esto difiere de
spec-kit, que sí ejecuta los steps — y es deliberado: preserva los invariantes de Musubi
(local-first, model-free, sin efectos colaterales desde el daemon).

## Invariantes que todo track respeta

- Go puro, sin CGo; SQLite local-first.
- Model-free: validación y expresiones sin LLM.
- MCP coordina, no ejecuta.
- `schema_version` explícito en todo artefacto declarativo (workflows, templates), como spec-kit.
- CI con `-race` (la concurrencia del engine lo exige).
- Delivery por **chained PRs** + work-unit commits; cada fase = release menor.

## Tracks y secuencia

```
Fase 0 (este doc)
   └─► C1  templates SDD            (quick win, bajo riesgo)
        └─► A1 → A2 → A3  motor DAG (flagship, el grueso)
             └─► B1 → B2 → B3  multi-agente (breadth)
```

### Track C — Templates SDD (quick win)
- **C1**: bundle de templates de artefactos SDD (proposal/spec/design/tasks) versionados,
  escritos por `musubi setup` en `.musubi/templates/sdd/` (igual que el bundle cognitivo).
  Delgado: solo scaffold, sin duplicar el orquestador SDD ni engram.

### Track A — Motor DAG resumible (flagship)
- **A1 — MVP**: `.musubi/workflows/*.yaml` (steps + deps), tabla `workflow_runs` en SQLite,
  tool `musubi_workflow` (`start/next/complete/status`). Reusa el claim atómico de `musubi_work`
  para el fan-out.
- **A2 — Resume + control de flujo**: `resume()` cross-sesión, steps `gate`/`if_then`/`switch`,
  evaluador de expresiones model-free.
- **A3 — Loops + validación + observabilidad**: `do_while`/`while_loop`, validación estática
  pre-run, `list_runs`.

**Diferenciador:** el `RunState` de spec-kit es local al run; el de Musubi es memoria viva
resumible entre sesiones y compactaciones. Musubi lo hace mejor en su propio terreno.

### Track B — Amplitud multi-agente (breadth)
- **B1**: abstracción `AgentTarget` extraída del bootstrap (hoy solo Claude Code), sin cambiar
  comportamiento.
- **B2**: 2-3 agentes de alto valor (Cursor, Codex/Gemini) como `AgentTarget`.
- **B3**: autodetección de agentes presentes + catálogo/docs.

## Estado

- [x] Fase 0 — groundwork (este doc)
- [x] C1 — templates SDD
- [x] A1 — motor DAG MVP (`musubi_workflow` start/next/complete/status + `workflow_runs` en SQLite)
- [ ] A2 · A3 — resume/control de flujo · loops/validación
- [ ] B1 · B2 · B3 — multi-agente
