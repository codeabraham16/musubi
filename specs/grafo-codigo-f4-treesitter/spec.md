---
artifact: spec
schema_version: "1.0"
change: grafo-codigo-f4-treesitter
status: draft
---

# Especificación — Aristas TS/JS/Python vía tree-sitter (Track 20 · F4)

## Requisitos
- **R1** — Con el build tag `treesitter`, `DerivePackage` DEBE derivar, para `.ts/.tsx/.js/.jsx/.py`,
  nodos de símbolo (func/method/class/type/const/var) + un nodo `file`, con aristas `CONTAINS`.
- **R2** — DEBE emitir aristas `IMPORTS` (nodo `package`, `external` = import bare; relativo = in-project).
- **R3** — DEBE emitir aristas `CALLS` **intra-archivo**: caller = def que envuelve el call-site;
  callee = def homónimo del mismo archivo (match único). Ambiguo/no resuelto ⇒ se omite.
- **R4** — Ids de nodo con el MISMO esquema que Go (`path#kind:name`); confianza 1.0; `EXTRACTED`.
- **R5** — **Sin** el build tag, TS/Py NO emiten nada (default histórico) y `gotreesitter` NO se
  linkea (binario lean). El pase polyglot es un no-op.
- **R6** — Degradación sin pánico: archivo que no parsea ⇒ vacío; extensión no soportada ⇒ vacío
  en ambos builds.
- **R7** — Model-free, Go puro **sin cgo** (gotreesitter es 100% Go); ambos builds `go build`/`go test` verdes.

## Escenarios
### Escenario: aristas TS con el tag
- **Given** `-tags treesitter` y `a.ts` donde `Alpha` llama a `beta` (misma file) e importa `./util`
- **When** `DerivePackage`
- **Then** hay nodos `func:Alpha`/`func:beta`, `CONTAINS a.ts→Alpha`, y `CALLS Alpha→beta`

### Escenario: Python
- **Given** `-tags treesitter` y `b.py` con `alpha()` que llama a `beta()` e `import os`
- **When** `DerivePackage`
- **Then** `CALLS alpha→beta` y al menos una arista `IMPORTS`

### Escenario: default intacto
- **Given** build por default (sin tag), `a.ts`/`b.py`
- **When** `DerivePackage`
- **Then** 0 nodos y 0 aristas (comportamiento histórico); `gotreesitter` no linkeado

## Fuera de alcance
- CALLS cross-archivo TS/Py (diferido). Resolución de módulos. Cambiar el binario por default.

## Preguntas abiertas
- [ ] Imports TS incompletos (`ExtractImportsFromSource` dio 0 en un caso): ¿probar `ExtractImports(tree)`?
      (design: aceptar en F4; el núcleo símbolos+calls anda; refinar imports TS luego)
