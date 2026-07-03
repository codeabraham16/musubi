---
artifact: spec
schema_version: "1.0"
change: deteccion-de-cambios-de-codigo
status: draft
---

# Especificación — Detección de cambios de código

## Requisitos

### Extractor de estructura (`internal/codeintel`)
- **R1** — El extractor DEBE recibir `(path, content)` y devolver una lista de símbolos,
  cada uno con `{Name, Kind, StartLine, EndLine}` (líneas 1-based, inclusivas), derivados
  **exclusivamente del `content` provisto** (nunca de datos persistidos).
- **R2** — Para archivos `.go` el extractor DEBE usar `go/ast` y reportar funciones,
  métodos (con receiver), tipos, y declaraciones top-level con sus rangos de línea exactos.
- **R3** — Para `.ts/.tsx/.js/.jsx` y `.py` el extractor DEBE reconocer, con un escáner
  determinista, las construcciones inequívocas: `function`, `class`, `def`, y
  `export const NAME = (…) =>` / `const NAME = function`. Ante ambigüedad DEBE omitir el
  símbolo (no inventarlo).
- **R4** — Si el parseo falla o la extensión no está soportada, el extractor DEBE devolver
  lista vacía sin error (degradación a granularidad de archivo), y NO DEBE entrar en pánico.
- **R5** — Para `.go`, si el `content` no compila (estado intermedio de edición), el
  extractor DEBERÍA usar parseo tolerante y devolver los símbolos que sí pudo resolver;
  si no puede, aplica R4.
- **R6** — El extractor DEBE exponer `ExtractImports(path, content)` devolviendo los
  imports/`require` declarados; misma política de degradación (R4).

### Parser de diff
- **R7** — El sistema DEBE parsear la salida de `git diff` unified y extraer, por archivo,
  los rangos de línea del **lado nuevo** (`+c,d` de cada hunk `@@ -a,b +c,d @@`).
- **R8** — El parser DEBE clasificar cada archivo como `added`, `modified`, `deleted` o
  `renamed`, y DEBE ignorar archivos binarios.
- **R9** — El parser DEBE invocar git de forma model-free y determinista
  (`--no-color`, sin paginador) y NO DEBE depender de locale.

### Tool `musubi_detect_changes`
- **R10** — La tool DEBE aceptar un `ref` opcional (base de comparación; default: working
  tree contra `HEAD`) y un flag `staged` opcional (comparar el índice).
- **R11** — Para cada archivo modificado, la tool DEBE leer su **contenido actual**,
  extraer símbolos (R1–R5), y reportar como "cambiados" los símbolos cuyo rango
  `[StartLine, EndLine]` **solapa** algún rango de hunk del lado nuevo (R7).
- **R12** — La tool DEBE marcar cada gist de `code_memory` de un archivo tocado como
  `stale` si su `fingerprint` almacenado difiere del fingerprint actual del archivo.
- **R13** — La tool DEBE cruzar los archivos/símbolos cambiados con la memoria (recall) y
  listar las observaciones/decisiones/artefactos SDD que los referencian.
- **R14** — La salida DEBE ser un objeto estructurado y compacto: por archivo
  `{path, change_type, changed_symbols[], gist_stale, related_memory[]}`, más un resumen.
  NO DEBE incluir cuerpos de función, solo nombres y rangos.
- **R15** — La tool DEBE ser de solo-lectura (`readOnly=true` en el registry) y no mutar
  memoria.

### Integración
- **R16** — `musubi_save_code` DEBE, por defecto, auto-derivar el campo `symbols` con el
  extractor a partir del contenido actual del archivo, anclado al **mismo** fingerprint que
  guarda. Si el llamador pasa `symbols` explícito, ese valor PUEDE prevalecer (compat).
- **R17** — La directiva de la fase SDD `verify` DEBE referenciar `musubi_detect_changes`
  como paso para acotar qué verificar.
- **R18** — Registrar la tool DEBE actualizar el conteo de tools y los golden snapshots de
  `tools/list` de forma consistente; el build y la suite DEBEN quedar verdes.

## Escenarios

### Escenario: símbolo cambiado tras corrimiento de líneas (el caso que hundía la v. ingenua)
- **Given** `foo.go` con `Parse()` que hoy vive en las líneas 70–85, y un gist viejo que lo
  registraba en L42
- **When** el diff muestra un hunk en el lado nuevo en las líneas 74–78 y se corre `detect_changes`
- **Then** el símbolo `Parse` (re-derivado del archivo actual en L70–85) se reporta como
  cambiado, y NO se reporta ningún símbolo fantasma — porque los símbolos salen del mismo
  estado nuevo que el diff, ignorando por completo el L42 guardado

### Escenario: gist stale por fingerprint
- **Given** `bar.ts` con un gist en `code_memory` cuyo fingerprint es el de una versión anterior
- **When** `bar.ts` aparece modificado en el diff y se corre `detect_changes`
- **Then** el reporte marca `gist_stale=true` para `bar.ts`

### Escenario: archivo a medio editar que no compila
- **Given** `wip.go` con un error de sintaxis (edición en curso)
- **When** se corre `detect_changes` y `wip.go` está en el diff
- **Then** la tool degrada a granularidad de archivo (`changed_symbols=[]`) sin pánico ni error

### Escenario: cruce con decisiones
- **Given** una observación con topic_key `sdd/x/design` que menciona `internal/config/config.go`
- **When** un diff toca `internal/config/config.go` y se corre `detect_changes`
- **Then** el reporte incluye esa observación en `related_memory` para ese archivo

### Escenario: lenguaje no soportado
- **Given** un `styles.css` modificado
- **When** se corre `detect_changes`
- **Then** el archivo se reporta a nivel-archivo (`changed_symbols=[]`) sin error

## Fuera de alcance
- Aristas de código emitidas por el agente y grafo de llamadas cross-lenguaje (fase 2
  acotada, solo derivadas).
- tree-sitter, cgo, paridad de 158 lenguajes.
- Análisis de impacto transitivo (callers de callers) — requiere el grafo derivado, fuera aquí.
- Blame/autoría, cobertura, o métricas de complejidad.

## Preguntas abiertas
- [ ] ¿El escáner JS/TS/Py va como regex acotada o como un mini-tokenizer? (resolver en design;
      criterio: el que dé menos falsos con menos código)
- [ ] ¿`related_memory` se resuelve por búsqueda de substring del path/símbolo o por recall
      semántico? (design; probable: keyword por path exacto + símbolo, barato y preciso)
- [ ] ¿`detect_changes` sin repo git responde error claro o degradación? (probable: error claro)
