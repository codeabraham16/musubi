---
artifact: spec
schema_version: "1.0"
change: mcp-readonlyhint
status: draft
---

# Especificación — Emitir `readOnlyHint` en el catálogo MCP

## Requisitos
Cada requisito es verificable y atómico. Vocabulario RFC 2119.

- **R1** — `tools/list` DEBE incluir `annotations.readOnlyHint: true` en toda tool cuyo spec declare
  `readOnly == true` (p.ej. `musubi_recall_facts`).
- **R2** — `tools/list` DEBE incluir `annotations.readOnlyHint: true` en toda tool cuyo spec declare
  `ro == roLeeConEscrituraIncidental`, **aunque** `readOnly == false` (`musubi_recall`,
  `musubi_memory_expand`, `musubi_recall_code`). Es el caso que motiva el cambio.
- **R3** — `tools/list` NO DEBE incluir la clave `annotations` en una tool que no cumple R1 ni R2
  (mutador puro, p.ej. `musubi_save_observation`). Lo correcto es la **ausencia** de la clave
  (`omitempty`), no `readOnlyHint: false`.
- **R4** — La derivación DEBE vivir en el único emisor `handleToolsList`, de modo que los tres
  servidores (central, relé, agente) emitan el mismo hint sin un segundo catálogo que sincronizar.
- **R5** — Para CADA tool del catálogo, el valor emitido DEBE ser exactamente
  `readOnly || ro == roLeeConEscrituraIncidental`. Invariante de consistencia barrible sobre todo el
  catálogo (dormidas incluidas con `MUSUBI_TOOLS_ALL`).
- **R6** — `catalog_sha256` NO DEBE cambiar por este cambio, y `contrato-tools.golden.json` tampoco:
  la anotación no entra al cable que marshalea `huellaDelCatalogo`, que serializa `entries[i].Tool`
  (el spec crudo), no la salida de `handleToolsList`.
- **R7** — El cambio DEBE ser aditivo: una tool sin hint DEBE serializar **byte-idéntico** a hoy
  (`omitempty` en `annotations` y en el campo del hint).

## Escenarios
Formato Given/When/Then (un escenario por comportamiento observable).

### Escenario: lectura pura declara el hint
- **Given** `musubi_recall_facts` con `readOnly: true`
- **When** se pide `tools/list`
- **Then** su entry trae `annotations.readOnlyHint == true`

### Escenario: lectura con escritura incidental declara el hint (el que motiva el cambio)
- **Given** `musubi_recall` con `readOnly: false` y `ro: roLeeConEscrituraIncidental`
- **When** se pide `tools/list`
- **Then** su entry trae `annotations.readOnlyHint == true`

### Escenario: mutador puro no declara anotación
- **Given** `musubi_save_observation` con `readOnly: false` y `ro: roDesdeReadOnly`
- **When** se pide `tools/list`
- **Then** su entry NO trae la clave `annotations`

### Escenario: consistencia sobre todo el catálogo
- **Given** el catálogo completo (dormidas incluidas, `MUSUBI_TOOLS_ALL`)
- **When** se recorre cada entry
- **Then** `annotations.readOnlyHint == true` si y sólo si `readOnly || ro == roLeeConEscrituraIncidental`

### Escenario: la huella no se mueve
- **Given** el registro antes y después del cambio
- **When** corren `TestContratoPorToolFijado` y `TestLaHuellaCubreElEfecto`
- **Then** quedan verdes sin regenerar `contrato-tools.golden.json`, y `catalog_sha256` es idéntico

## Fuera de alcance
- Los demás hints del sobre MCP (`destructiveHint`, `idempotentHint`, `openWorldHint`).
- Reclasificar cualquier tool: los ejes `readOnly`/`ro` quedan como están.
- La autorización del dispatch (RLock/Lock): no cambia qué puede llamar un principal lector.

## Preguntas abiertas
- [x] ¿La anotación mueve la huella? **No** — `handleToolsList` trabaja sobre una copia local;
  `huellaDelCatalogo` marshalea el spec crudo. Verificado en `identity.go:32`.
- [ ] Confirmar en implementación que la clave serializada es exactamente `annotations` con
  `readOnlyHint` (camelCase del protocolo MCP), no snake_case.
