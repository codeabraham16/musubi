---
artifact: spec
schema_version: "1.0"
change: grafo-codigo-f5-incremental-stale
status: archived
---

# Especificación — F5: índice incremental + poda de fantasmas + visibilidad de staleness

## Requisitos
Cada requisito es verificable y atómico. Vocabulario RFC 2119.

- **R1** — `musubi_codegraph_index` DEBE ofrecer un modo **incremental** además del **full**.
  Con `mode:"incremental"` reconcilia el grafo con el working tree; sin `mode` (o
  `mode:"full"`) mantiene el comportamiento actual (re-derivar todos los paquetes).
- **R2** — En modo incremental el sistema DEBE re-derivar **sólo** los paquetes con al menos un
  archivo `.go` **modificado** (fingerprint en disco ≠ el guardado) o **nuevo** (path no
  presente en el grafo), y DEBE **saltar** los paquetes sin cambios. Si nada cambió, DEBE
  re-derivar **0 paquetes**.
- **R3** — El sistema DEBE clasificar modificado/nuevo/fantasma reusando el `src_fingerprint`
  guardado por archivo y `FileFingerprint` del disco, **sin** depender de git ni de un cursor
  de commit.
- **R4** — El índice incremental DEBE **podar** (borrar nodos y aristas) de los archivos que
  están en el grafo pero ya **no existen** en disco (borrados/renombrados).
- **R5** — `cgStale` DEBE reportar **stale** (no fresco) un nodo cuyo archivo no exista o sea
  ilegible. (Hoy un archivo faltante se reporta fresco: bug de correctitud a corregir.)
- **R6** — La poda DEBE estar acotada por `project_id`: nunca borrar filas de otro tenant ni
  filas sin atribución de credencial (mismo criterio de escritura que `UpsertPackageGraphFrom`).
- **R7** — El resumen del índice DEBERÍA reportar cuántos paquetes se refrescaron y cuántos
  archivos se podaron; el incremental DEBERÍA además distinguir los paquetes saltados.
- **R8** — `musubi_map` DEBE reportar el conteo de archivos **stale** y **fantasma** del grafo,
  para señalar cuándo conviene re-indexar.
- **R9** — Los cambios DEBEN ser **aditivos**: el modo full sigue siendo el default y el shape
  de las respuestas existentes no se rompe (sólo se agregan campos).
- **R10** — La derivación y la invalidación DEBEN seguir **model-free** (AST + fingerprint),
  sin enviar código a un modelo.

## Escenarios
Formato Given/When/Then (un escenario por comportamiento observable).

### Escenario: incremental sin cambios no re-deriva nada
- **Given** un grafo ya indexado (full) y ningún archivo tocado
- **When** corro `codegraph_index` con `mode:"incremental"`
- **Then** el resumen reporta `packages` (refrescados) = 0 y el grafo queda idéntico

### Escenario: archivo modificado re-deriva sólo su paquete
- **Given** un grafo indexado con `pkg/a.go` (Alpha→beta) y `otro/z.go`
- **When** modifico `pkg/a.go` y corro incremental
- **Then** se re-deriva el paquete `pkg` (no `otro`) y el nodo `Alpha` deja de estar stale

### Escenario: archivo nuevo se incorpora
- **Given** un grafo indexado
- **When** agrego `pkg/c.go` con `func Gamma()` y corro incremental
- **Then** aparece el nodo `pkg/c.go#func:Gamma`

### Escenario: archivo borrado se poda (fantasma)
- **Given** un grafo con nodos/aristas de `pkg/b.go`
- **When** borro `pkg/b.go` del disco y corro incremental
- **Then** `code_graph`/`impact` ya no devuelven símbolos de `pkg/b.go` y el resumen reporta `pruned ≥ 1`

### Escenario: cgStale marca stale un archivo faltante
- **Given** un nodo `pkg/b.go#type:T` en el grafo
- **When** `pkg/b.go` no existe en disco y consulto `code_graph` de ese nodo (sin re-indexar)
- **Then** el nodo se reporta `stale: true` (antes: `false`)

### Escenario: map reporta stale y fantasmas
- **Given** un grafo indexado; luego modifico un archivo y borro otro **sin** re-indexar
- **When** llamo `musubi_map`
- **Then** la respuesta incluye `stale ≥ 1` y `ghosts ≥ 1`

### Escenario: la poda respeta el aislamiento por tenant
- **Given** dos proyectos con un mismo `path` en sus grafos
- **When** el proyecto A poda ese path
- **Then** las filas del proyecto B permanecen intactas

## Fuera de alcance
- Marcador de consistencia a nivel commit-SHA / versión global del grafo.
- Auto-refresh del grafo dentro del hook precheck o en cada lectura.
- Detección de renames a nivel de nodo (se modela como borrado + alta).
- Aristas nuevas, CALLS cross-paquete o multi-lenguaje (F1/F4).

## Preguntas abiertas
- [ ] Nombre del argumento del modo: `mode:"incremental"|"full"` (elegido, extensible) vs. un
      booleano `incremental`. Se resuelve en design.
