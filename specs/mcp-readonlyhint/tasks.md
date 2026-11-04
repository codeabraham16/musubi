---
artifact: tasks
schema_version: "1.0"
change: mcp-readonlyhint
status: draft
---

# Tareas — Emitir `readOnlyHint` en el catálogo MCP

Checklist atómica, ordenada por dependencia. Cada tarea es cerrable de forma independiente.
Compilar/testear siempre con `GOWORK=off` (el `go.work` pinea un Go desfasado, como el CI).

## Implementación
- [ ] **T1** — `internal/mcp/methods.go`: definir `type ToolAnnotations struct { ReadOnlyHint bool \`json:"readOnlyHint,omitempty"\` }` y agregar `Annotations *ToolAnnotations \`json:"annotations,omitempty"\`` al struct `Tool`, con comentario del porqué (hint de UI del protocolo MCP, no autorización).
- [ ] **T2** — `internal/mcp/registry.go`, en el loop de `handleToolsList` tras `t := s.tools[i].Tool`: `if s.tools[i].readOnly || s.tools[i].ro == roLeeConEscrituraIncidental { t.Annotations = &ToolAnnotations{ReadOnlyHint: true} }`.
- [ ] **T3** — Regenerar los dos goldens: `GOWORK=off go test ./internal/mcp -run TestToolsListGolden -update`. Inspeccionar el `git diff` de `testdata/toolslist.golden.json` y `toolslist-completo.golden.json`: sólo deben aparecer bloques `"annotations": {"readOnlyHint": true}` en las tools de lectura.

## Verificación (invariante + sabotaje)
- [ ] **T4** — `internal/mcp/readonly_hint_test.go` (nuevo): barrido R5 (para cada `toolEntry`, `hintEmitido == (readOnly || ro==roLeeConEscrituraIncidental)`) + anclas R1/R2/R3 (`musubi_recall_facts`, `musubi_recall`+`musubi_memory_expand`+`musubi_recall_code`, `musubi_save_observation`). Correr: verde.
- [ ] **T5** — Sabotaje A: quitar `|| s.tools[i].ro == roLeeConEscrituraIncidental` de T2 → el test DEBE ponerse ROJO nombrando `musubi_recall` (y las otras incidentales). Revertir.
- [ ] **T6** — Sabotaje B: forzar la condición a `true` (hint en toda tool) → el test DEBE ponerse ROJO nombrando `musubi_save_observation`. Revertir.
- [ ] **T7** — Huella intacta: `GOWORK=off go test ./internal/mcp -run 'TestContratoPorToolFijado|TestLaHuellaCubreElEfecto' -count=1` verde, y `git status` confirma que `testdata/contrato-tools.golden.json` NO cambió.

## Entrega
- [x] **T8** — `CHANGELOG.md`: entrada `[Unreleased] Added` + renglón Destacado/Highlights (notas-release publica sólo esos). ⚠️ **NO se toca el archivo `VERSION`**: en este repo lo sube SÓLO el release-cut (`deploy/cmd/preparar-release`), guardado por `TestVersioninfoMatchesVERSION` + `release.yml` (tag==VERSION); un feature-PR que lo edite rompe el CI. El PR agrega sólo CHANGELOG `[Unreleased]`. «Menor» es la clasificación semver del cambio, no una edición de `VERSION`.
- [ ] **T9** — Puerta final: `GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./internal/mcp -count=1` en verde.
- [ ] **T10** — Fase verify: revisión adversarial (skill `adversarial-review`) con la evidencia de T3–T9 antes de dar por cerrado.

## Alcance descubierto (fuera de la propuesta original)
- [x] **T11** — Emitir `readOnlyHint` en las 3 lecturas del núcleo (`musubi_impact`, `musubi_memory_expand`, `musubi_recall`) agregó `"annotations":{"readOnlyHint":true}` a su JSON servido y empujó el peso del núcleo por encima del tope `techoDelNucleo = 8000` de `internal/mcp/agente_test.go` (hoy 8065). **Decisión del dueño (2026-11-03): subir el tope a 8192** (~+110 chars ≈ ~27 tokens/sesión, el precio de que `musubi_recall` no pida aprobación en el cuerpo). Registrado en el comentario de la constante; `TestElNucleoEsLoQueLasInstruccionesNombranYCabe` verde a 8065/8192.
