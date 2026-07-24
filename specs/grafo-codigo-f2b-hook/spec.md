---
artifact: spec
schema_version: "1.0"
change: grafo-codigo-f2b-hook
status: draft
---

# Especificación — El hook que responde antes de leer (Track 20 · F2-B)

## Requisitos
- **R1** — Con `MUSUBI_CODEGRAPH_HOOK` habilitado (`1|true|yes|on`), al `Read` de un archivo
  **indexado**, el hook DEBE inyectar en `additionalContext` su estructura: imports del archivo +
  funciones/métodos con a quién llaman (callees) y quién los llama (callers).
- **R2** — Sin el env var, el hook NO DEBE inyectar el grafo (comportamiento actual bit-a-bit).
- **R3** — Si el archivo NO está indexado (sin nodos), el mensaje del grafo DEBE ser "" aunque el
  env var esté activo (inerte hasta `musubi_codegraph_index`).
- **R4** — La salida DEBE ser compacta y acotada (≤10 símbolos, ≤5 refs c/u, nombres no node_keys)
  y contabilizarse en el ledger como `precheck_codegraph`.
- **R5** — Model-free (solo recorre el grafo), Go puro, sin cgo; build + tests verdes.

## Escenarios
### Escenario: hook habilitado sobre archivo indexado
- **Given** `MUSUBI_CODEGRAPH_HOOK=1` y `a.go` indexado (Alpha importa fmt y llama a beta)
- **When** el hook corre para `Read a.go`
- **Then** el contexto incluye "grafo de código", los imports (fmt) y `Alpha → beta` / `beta ← Alpha`

### Escenario: apagado por default
- **Given** SIN el env var, `a.go` indexado
- **When** el hook corre para `Read a.go`
- **Then** NO se inyecta el contexto del grafo

## Fuera de alcance
- Cambiar la config/hooks del usuario. Aristas TS/Py (F4). Mutar el grafo.

## Preguntas abiertas
- [ ] ¿Gate por env var o por config? (design: env var — cero plumbing, opt-in claro)
