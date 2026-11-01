---
artifact: tasks
schema_version: "1.0"
change: readiness-ve-la-frescura-del-grafo
status: draft
---

# Tareas — checklist atómica

Orden elegido: primero el cambio mecánico y verificable solo (la firma), después el que tiene la
lógica (el enriquecido), y los tests **al cierre de cada pieza**, no todos juntos al final.

## T1 — Ampliar la firma de `graphFreshness` y adaptar su llamador

- [ ] `internal/mcp/methods_codegraph.go`: `graphFreshness` devuelve
      `(stale, ghosts, missing, enGrafo, enDisco int)`. El cuerpo **no cambia**: `enGrafo` es
      `len(stored)` y `enDisco` es `len(enDisco)`, que ya se calculan y hoy se tiran.
- [ ] El `return 0, 0, 0` del fail-safe `arbolFueraDeAlcance()` pasa a `return 0, 0, 0, 0, 0`, y lo
      mismo el del error de `GraphFileFingerprintsCtx`. **La semántica no se toca** (R7).
- [ ] Actualizar el comentario de la función para nombrar los dos valores nuevos.
- [ ] `toolMap` (mismo archivo): ignora los dos nuevos con `_, _`. Su salida JSON **no cambia**.
- [ ] Los tres tests que ya la llaman (`codegraph_missing_test.go:66,102,125`) se adaptan a la
      firma nueva sin cambiar lo que afirman.
- [ ] **Cierre de T1:** `go build ./...` y `go test ./internal/mcp -run 'Codegraph|Map' -count=1`
      en verde. T1 es puramente mecánica: si algo de comportamiento cambió acá, está mal.

## T2 — El enriquecido en `toolReadiness`

- [ ] `internal/mcp/methods_readiness.go`: función nueva `enriquecerGrafo(rep *memory.ReadinessReport, scoped context.Context)`,
      llamada entre `lector.Readiness(...)` y el `jsonResult(rep)`.
- [ ] Ubicar la dimensión por `Key == "grafo"`. Si no está, no hacer nada (un backend que no la
      calcula no puede ganar un check).
- [ ] Rama **no aplica** (D4): si `s.arbolFueraDeAlcance()`, poner
      `evidence["frescura_no_medida"]` con el motivo y **salir sin agregar check ni tocar
      `Observed` ni `Score`**.
- [ ] Rama normal: llamar `graphFreshness`, escribir las cinco claves crudas en `evidence`
      (`stale`, `ghosts`, `missing`, `archivos_en_grafo`, `archivos_en_disco`) y agregar el check
      compuesto con el `Why` que nombra los tres ejes y el remedio.
- [ ] El umbral va en **una constante nombrada** con el comentario de dónde sale el 5 %
      (300 commits: mediana 0,29 %, máximo 4,66 %).
- [ ] ⚠️ **Recalcular `dim.Score = puntaje(dim.Checks)` Y el `rep.Score` global.** Es el bug que el
      design marcó como el más probable: sin esto el check queda de adorno.
- [ ] Verificar que `puntaje()` sea alcanzable desde `internal/mcp` (es minúscula, vive en
      `internal/memory`). **Si no lo es, resolverlo sin exportarla de más**: la fracción se puede
      recalcular en el llamador, que es una línea, antes que ampliar la superficie pública de
      `memory` por un detalle interno.
- [ ] **Cierre de T2:** `go build ./...` + `go vet ./...` en verde.

## T3 — Tests, uno por invariante, cada uno con su sabotaje

Van en `internal/mcp/` siguiendo la convención de `readiness_medido_test.go`. **Cada test comprueba
el denominador antes que el numerador** (anti-mutación-vacua).

- [ ] Un grafo podrido no puntúa 1.0 → sabotaje: `Pass: true` fijo.
- [ ] **El puntaje refleja el check** (dimensión y global) → sabotaje: agregar el check sin
      recalcular `Score`.
- [ ] La evidencia trae los cinco crudos → sabotaje: guardar el porcentaje en vez del crudo.
- [ ] Donde no aplica, no se agrega el check y el puntaje no sube → sabotaje: agregarlo igual.
- [ ] Los dos checks de volumen quedan con su `Name` y su semántica → sabotaje: renombrar uno.
- [ ] El umbral es proporcional → 1 archivo stale de 1.395 **pasa**; sabotaje: volverlo binario.
- [ ] **Una dimensión observada SIN archivos que medir** (nodos con `path = ''`, árbol sin
      indexables) no gana el check → sabotaje: `if false && enGrafo == 0 && enDisco == 0`.
      *Salió en `verify`, leyendo el diff: es la segunda puerta de la trampa anterior.*
- [ ] **Cierre de T3:** los OCHO sabotajes verificados en ROJO uno por uno, y el árbol limpio
      después (confirmar con `git status` que ningún sabotaje quedó aplicado).

## T4 — Entrega

- [ ] `CHANGELOG.md`, sección `[Unreleased]`: una viñeta que diga explícitamente que **el puntaje de
      un proyecto con el grafo podrido baja**, para que nadie lo lea como una regresión. ⚠️ Antes de
      escribirla, comprobar que la rama no esté detrás de un release-cut.
- [ ] Bump semántico.
- [ ] `go test ./... -count=1`. ⚠️ Tres rojos conocidos de esta máquina son por contención, no por
      defecto: identificarlos y no confundirlos con trabajo propio.
- [ ] Rama + commit (asunto imperativo de UNA idea, ≤50 chars) + PR. **El merge lo pide el dueño.**
- [ ] Antes del push: `musubi receipt emit`.

## Fuera de esta checklist

El check del grafo en `musubi_doctor` (fuera de alcance por spec) y los cabos TP1, TP2, TP3 y TP5.
