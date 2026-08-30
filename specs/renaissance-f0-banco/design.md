# SDD design — renaissance-f0-banco

## Archivos

| Archivo | Qué es |
|---|---|
| `internal/mcp/testdata/banco-diseno.json` | el set dorado (pedidos + fuera de dominio + inyecciones) |
| `internal/mcp/testdata/banco-umbrales.json` | un umbral por métrica, con fecha y commit |
| `internal/mcp/banco_diseno.go` | tipos del set, carga, y el cálculo de las métricas — código normal, no `_test`, para que la sonda lo reuse |
| `internal/mcp/banco_diseno_test.go` | el banco estructural: fixture, corrida, comparación contra umbrales |
| `internal/mcp/sonda_diseno_test.go` | `//go:build sonda` — la sonda contra el central |

`banco_diseno.go` va como código de producción y no como helper de test **a propósito**: la sonda
vive detrás de un build tag y no puede importar símbolos de `_test.go`. Es la razón técnica, y de
paso deja el cálculo de métricas en un solo lugar auditable.

## El fixture del banco estructural

Un acervo sembrado en `t.TempDir()` que reproduce la **forma** del acervo real, no su tamaño:

- 8 tarjetas `design-method/*` (el núcleo: jerarquía, color, CTA, grilla, más motion, a11y, móvil,
  microcopy — para que la mezcla escritorio/móvil que hoy contamina el brief esté representada).
- 24 tarjetas `design-corpus/*` repartidas en los ejes del set dorado.
- 3 blobs `ingested/*` largos, para que la competencia tarjeta-vs-artículo exista en el fixture.
- 2 docs `diseno/marca`: uno estructurado con tokens (proyecto `banco-a`) y uno en prosa
  (`banco-b`), más un tercer proyecto sin marca para ejercitar el camino neutro.

Con `NoopProvider` el recall cae a FTS. **Eso es correcto para lo que mide este banco** (ensamblado,
tamaño, abstención, inyección) y explícitamente insuficiente para M1/M3/M8, que por eso viven en la
sonda. La alternativa —un embebedor falso con similitud inventada— mediría el falso.

## Cómo se calcula cada métrica

- **M4 tamaño**: `len(json.Marshal(brief))/4`, igual que la cuenta que usó la auditoría, para que los
  números sean comparables con el informe del 2026-08-29.
- **M5 fracción variable**: se piden dos briefs de pedidos con ejes disjuntos y mismo proyecto, y se
  compara bloque por bloque cuáles salieron idénticos. Lo idéntico es constante; el resto varía.
  Bloque a bloque y no por diff de bytes: un diff textual sobre JSON da ruido de comas y llaves.
- **M6 inyección**: el payload se busca en los campos que el agente lee **como instrucción**
  (`role`, `principles`, `emit`, `instructions`) y no en los que lee como material (`corpus`) ni en
  el eco del pedido (`ask`). Hoy `ask` sí es riesgo, y por eso el banco lo reporta aparte en vez de
  mezclarlo: F1 lo va a mover, y la métrica no puede darse por ganada por una mudanza.
- **M2 abstención**: `degraded == true` **o** `len(corpus) == 0`. Con FTS una consulta sin ningún
  término del acervo devuelve cero filas y hoy ya abstiene; con el embebedor real nunca lo hace. La
  sonda mide el caso que importa; el estructural fija el piso.

## Umbrales

```json
{ "fijado": "2026-08-29", "commit": "<sha>", "umbrales": {
    "m2_abstencion_min": 0.0, "m4_tokens_p50_max": 6200, "m4_tokens_max": 12000,
    "m5_fraccion_variable_min": 0.05, "m6_inyeccion_min": 0.0 } }
```

Arrancan en el valor medido hoy —incluidos los ceros vergonzosos— con margen de ruido. La regla es
**sólo se aprietan**: el PR de cada fase mueve su umbral y ese movimiento se ve en el diff.

## La sonda

Lee `MUSUBI_CENTRAL_URL` y `MUSUBI_TOKEN` del entorno; si falta alguno, `t.Skip`. Habla JSON-RPC por
`net/http` contra `/mcp`. Recorre el set dorado, calcula M1/M3/M7/M8 e imprime una tabla. **No
falla** por umbral: es un instrumento de medición, no una compuerta — el central puede estar
legítimamente en otro estado que el repo local, y una sonda que rompe el build por eso se apaga sola.

## Riesgos

1. **El fixture diverge del acervo real** y el banco valida un mundo que no existe → por eso las
   métricas que dependen del acervo viven en la sonda, y el fixture sólo sostiene las estructurales.
2. **M3 por ejes es grueso** y podría dar alto con corpus mediocre → se declara como aproximación; si
   deja de discriminar entre un corpus bueno y uno malo, se cambia por etiquetado.
3. **El set dorado envejece** al cambiar los proyectos → los pedidos se toman de pantallas que ya
   existen, y el archivo lleva la fecha de armado.
