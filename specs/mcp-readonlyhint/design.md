---
artifact: design
schema_version: "1.0"
change: mcp-readonlyhint
status: draft
---

# Diseño técnico — Emitir `readOnlyHint` en el catálogo MCP

## Decisiones de arquitectura
| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | Derivar el hint de los ejes existentes: `readOnlyHint = readOnly \|\| ro == roLeeConEscrituraIncidental` | (a) derivar sólo de `readOnly`; (b) un cuarto eje/campo explícito por-tool | `readOnlyHint` responde "¿muta estado observable?", que el registro YA contesta con `readOnly` (efecto) + `ro` (sirve en sólo-lectura). (a) deja afuera a `musubi_recall` —la tool más llamada— que no es `readOnly` pero su escritura es incidental: no resuelve el bug. (b) duplica la fuente de verdad y abre la deriva (hint que contradice al eje). La derivada es exacta y gratis. |
| D2 | Setear la anotación en `handleToolsList` sobre la **copia local** `t`, no en el spec de `buildRegistry` | Setearla en el `toolEntry` guardado | Un solo emisor ⇒ catálogo coherente para central/relé/agente. La copia local deja el spec crudo intacto, así `huellaDelCatalogo` (que marshalea `entries[i].Tool`) no ve la anotación: `catalog_sha256` y `contrato-tools.golden.json` no se mueven (R6). La info del hint ya vive en el contrato `{ro}` que la huella pliega; duplicarla en el cable sería ruido. |
| D3 | DTO: `Annotations *ToolAnnotations` con `omitempty`; `ToolAnnotations{ ReadOnlyHint bool }`. Asignar el struct **sólo** cuando el hint es `true` | `Annotations ToolAnnotations` por valor | El puntero + `omitempty` hace que una tool sin hint no emita la clave `annotations` (R3) y serialice byte-idéntico (R7). Por valor emitiría `"annotations":{}` en toda tool sin hint: rompe R7 y ensucia el golden. El struct nombrado deja lugar a los otros hints MCP sin tocar el cable. |
| D4 | Verificar con un barrido del catálogo (invariante R5) + anclas nombradas, y dos sabotajes dirigidos | Sólo asserts puntuales sobre 2-3 tools | El barrido ata CADA tool a la fórmula, no sólo las elegidas: una tool nueva mal clasificada cae. Las anclas (recall_facts, recall, save_observation) dan sabotaje legible: A) quitar la rama incidental ⇒ recall pierde el hint ⇒ ROJO; B) derivación siempre-true ⇒ save_observation gana hint ⇒ ROJO. Ambos atacan el invariante declarado, no una validación lateral. |
| D5 | Regenerar `toolslist.golden.json` + `toolslist-completo.golden.json`; bump **menor** | — | Son los dos goldens que capturan la salida de `handleToolsList` (visible + completo). `contrato-tools.golden.json` NO cambia (D2). Aditivo a la superficie MCP ⇒ semver menor. |
| D6 | (alcance descubierto) Subir `techoDelNucleo` 8000→8192 en `agente_test.go` | dejar el tope en 8000 y recortar instrucciones | El hint sobre las 3 lecturas del núcleo (`musubi_impact`, `musubi_memory_expand`, `musubi_recall`) suma ~+110 chars al JSON servido del núcleo y cruza el tope de 8000 (hoy 8065). El tope es un presupuesto, no un derivado: subirlo es una decisión, y el dueño la tomó (2026-11-03). El precio —~27 tokens/sesión— compra que `musubi_recall` no pida aprobación en el cuerpo. Recortar instrucciones habría sacado una tool del núcleo por un costo que el dueño aceptó. |

## Enfoque de implementación
1. `internal/mcp/methods.go`: agregar `Annotations *ToolAnnotations json:"annotations,omitempty"` al struct `Tool`, y definir `ToolAnnotations` con `ReadOnlyHint bool json:"readOnlyHint,omitempty"`.
2. `internal/mcp/registry.go`, dentro del loop de `handleToolsList`, tras `t := s.tools[i].Tool`:
   ```go
   if s.tools[i].readOnly || s.tools[i].ro == roLeeConEscrituraIncidental {
       t.Annotations = &ToolAnnotations{ReadOnlyHint: true}
   }
   ```
3. Regenerar los dos goldens: `go test ./internal/mcp -run TestToolsListGolden -update` (cubre visible y completo).
4. `internal/mcp/readonly_hint_test.go` (nuevo): el barrido R5 + anclas + los dos sabotajes documentados como comentario de intención.
5. `CHANGELOG.md`: entrada `[Unreleased] Added` + renglón Destacado/Highlights; bump menor.

## Contratos / interfaces
- Salida de `tools/list` por tool de lectura:
  `{"name": "...", "description": "...", "inputSchema": {...}, "annotations": {"readOnlyHint": true}}`
- Salida por tool de escritura: sin la clave `annotations` (idéntica a hoy).
- `catalog_sha256` (handshake MCP `_meta[musubi/identity]`, `musubi version --json`): **sin cambio**.

## Trade-offs
- **Se gana:** un cliente MCP que respeta la anotación (el `pi` del cuerpo) deja de pedir aprobación en las lecturas; cero fuente de verdad nueva; cero movimiento de la huella.
- **Se cede:** no se exponen los otros hints del sobre (`destructiveHint`, etc.) — fuera de alcance, el sobre `ToolAnnotations` queda listo para sumarlos después.

## Plan de pruebas
- `readonly_hint_test.go`: (R1) `musubi_recall_facts` ⇒ hint true; (R2) `musubi_recall`/`musubi_memory_expand`/`musubi_recall_code` ⇒ hint true; (R3) `musubi_save_observation` ⇒ sin `annotations`; (R5) barrido: para cada tool, `hint == (readOnly || ro==roLeeConEscrituraIncidental)`.
- Sabotaje A (quitar `|| ro==...`) y B (forzar true) ⇒ el test DEBE ponerse ROJO en el ancla correcta.
- Goldens: `TestToolsListGolden` + `...Completo` verdes con los dos regenerados.
- Huella: `TestContratoPorToolFijado` + `TestLaHuellaCubreElEfecto` verdes **sin** tocar `contrato-tools.golden.json`.
- Cierre: `go test ./internal/mcp -count=1` y `go build ./...` + `go vet ./...` en verde (GOWORK=off, como el CI).
