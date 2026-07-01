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
- [x] A2 — resume + control de flujo (`when` → gate/if_then/switch, estado `skipped`) + evaluador de expresiones model-free
- [x] A3 — loops (`repeat_while` + `max_iterations`) + validación expuesta (`validate`) + `list` runs → **Track A completo**
- [x] B1 · B2 · B3 — multi-agente: abstracción `AgentTarget`, target Cursor (`.cursor/mcp.json`), detección + flag `--agent` → **Track B completo**

**Tracks A/B/C completos.**

---

## Track O — Orquestación a paridad (Track 12: dos pilares)

Extiende esta base para volver la orquestación un PILAR paritario con la memoria
(decisión de trayecto 2026-06-30, ver memoria `roadmap/track-12-pilares`). No recorta
nada; sube el nivel del pilar de orquestación y lo fusiona con la memoria.

- [x] **O1 — `musubi_sdd`: flujo SDD guiado.** Genera el workflow canónico de un cambio
  (proposal→spec→design→tasks→implement→verify→archive) sobre el motor DAG, sin YAML.
  Surface por fase su directiva + plantilla; al cerrar una fase persiste su **contrato de
  resultado** (summary/artifacts/risks/next_recommended) en memoria bajo
  `sdd/<change>/<phase>` (upsert por id determinista). Las fases siguientes recuperan por
  referencia barata. Es la fusión memoria↔orquestación.
  Archivos: `internal/memory/sdd.go`, `internal/mcp/methods_sdd.go` (+ tests).
- [x] **O2 — Medición de delegación** (token governor × pizarra): `musubi_work action=savings`
  estima —model-free, con parámetros configurables (`avoided_context_tokens_per_unit`,
  `delegation_overhead_tokens`)— los tokens ahorrados por delegar vs. inline. El ahorro es
  lineal en unidades done (el contexto intermedio evitado − overhead), así que rinde con
  volumen y no con tareas triviales. Archivos: `internal/memory/delegation.go`,
  `internal/mcp/methods.go` (case savings), config `MultiAgentConfig` (+ tests).
- [x] **O3 — Biblioteca de roles SDD** + revisión adversarial. Los 7 roles de fase son
  first-class en `musubi_sdd` (campo `role` por fase activa, `internal/memory/sdd.go`), más
  2 skills cognitivas transversales: `sdd-flow` (orquestador del flujo) y `adversarial-review`
  (patrón judgment-day: escépticos por lente + veredicto por mayoría + fix-loop, cableado a
  `musubi_work`/`musubi_judge`). Archivos: `internal/memory/sdd.go`, `cmd/musubi/cognitive.go` (+ tests).
- [x] **O4 — Orquestación en el dashboard**: el snapshot (`buildExportSnapshot`) incluye
  `orchestration` (runs de workflow con progreso por fases; los flujos SDD marcados; pizarra
  multi-agente activa con conteos). El HTML del dashboard lo renderiza como segundo pilar,
  read-only y 0 tokens. Archivos: `cmd/musubi/export.go`, `cmd/musubi/assets/dashboard.html` (+ test).

## Track S — Servidor (bisagra)

El transporte HTTP ya existía (Track 4). El hallazgo clave: como el daemon remoto sirve
**todas** las tools, memoria y orquestación compartidas se logran apuntando el cliente al
cerebro central — **configuración, no motor nuevo**. Runbook: `docs/Server_Brain_Onboarding.md`.

- [x] **S1 — daemon HTTP multi-máquina**: `musubi serve` + `ListenAndServeHTTP` con bind
  loopback/remoto, bearer token (fail-closed), TLS, `/healthz` `/readyz` `/metrics`, shutdown
  graceful y defensa anti DNS-rebinding. Ya implementado en `internal/mcp/http.go`.
- [x] **S2 + S3 — memoria y orquestación compartidas remotas**: entrada `.mcp.json` remota
  (transporte HTTP) que apunta el cliente al cerebro central; el daemon sirve el catálogo
  completo, así memoria, `workflow_runs` y pizarra quedan compartidos entre máquinas. Helper
  `bootstrap.RemoteEntry` (token por `${ENV}`, el secreto no toca el archivo) +
  `MergeRemoteMCPServer`. Archivos: `internal/bootstrap/mcp.go` (+ tests).
- [x] **S4 — malla VPN + onboarding**: runbook completo de cutover (serve + WireGuard/
  Tailscale + config de clientes + verificación) en `docs/Server_Brain_Onboarding.md`.

**Nota de cutover**: la ejecución real (URL/token/TLS concretos, `musubi serve` en el fierro)
se hace cuando el servidor esté activo; el código y la config ya están listos.
