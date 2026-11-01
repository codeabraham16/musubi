---
artifact: proposal
schema_version: "1.0"
change: readiness-ve-la-frescura-del-grafo
status: draft
---

# Propuesta — `musubi_readiness` deja de puntuar un grafo podrido como sano

## Intención

La dimensión `grafo` de `musubi_readiness` se titula «El código está indexado» y lo verifica con dos
checks de VOLUMEN: `nodos > 0` y `aristas > 0`. Puntúa **1.0 con un solo nodo y una sola arista**.
Los tres indicadores que dicen si ese índice todavía SIRVE —`stale`, `ghosts`, `missing`— se
calculan en `graphFreshness` ([methods_codegraph.go:1701](../../internal/mcp/methods_codegraph.go))
y su único consumidor es `toolMap`, una tool de exploración que hay que llamar a propósito.

Eso le quita el piso a un argumento que está escrito en el código. El comentario de
[methods_codegraph.go:371](../../internal/mcp/methods_codegraph.go) justifica el fail-safe «ante la
duda, PRESENTE» así: *«equivocarse hacia presente es RUIDOSO —`graphFreshness` reporta fantasmas en
masa—»*. Ese ruido existe, pero **sólo suena dentro de `musubi_map`**. La superficie cuyo trabajo
es avisar que algo está mal no lo mira, así que el fail-safe se apoya en una alarma que nadie
escucha.

Es el patrón que este repo ya tiene nombrado dos veces: una señal que se calcula y no se lee.

## Alcance

- **Incluye:** que la dimensión `grafo` de `musubi_readiness` incorpore los tres indicadores de
  frescura como checks y como evidencia, de modo que un grafo podrido no pueda puntuar 1.0.
- **No incluye:**
  - Cambiar `graphFreshness`, `toolMap` ni el fail-safe de `arbolFueraDeAlcance()`. Lo que se
    arregla es QUIÉN LEE la señal, no la señal.
  - Agregar un check al `musubi_doctor`. Sus 16 checks hoy no cubren el grafo de código, y eso es
    un hueco real —`anchor_coverage` es de anclas de observaciones, los `stale_*` son de gists y
    conflictos—, pero es otra superficie y otro contrato (`Repair`). Va al plan siguiente.
  - Los otros cuatro cabos abiertos (TP1, TP2, TP3, TP5).

## Enfoque

`dimensionGrafo` vive en `internal/memory` y es método de `*DbEngine`; `graphFreshness` vive en
`internal/mcp` y es método de `*McpServer` porque necesita el disco (`FileFingerprint`,
`walkSourceTree`), el `projectPath` y el scope. **El engine no puede llamarla, y ésa es la razón
por la que hoy no están conectadas: no es un olvido, es una frontera de capas.** Proponer «que
`dimensionGrafo` la llame» sería proponer algo que no compila.

La estrategia es inyectar desde arriba. `toolReadiness`
([methods_readiness.go:32](../../internal/mcp/methods_readiness.go)) hoy es un paso-a-través: pide
`rep` al engine y lo devuelve tal cual. Ahí —ya del lado MCP, con disco y scope disponibles— se
enriquece la dimensión `grafo` con los tres indicadores antes de serializar. Cada capa sigue
midiendo lo que puede medir sola.

## Impacto

- Áreas/archivos afectados: `internal/mcp/methods_readiness.go` (el enriquecido), sus tests, y
  posiblemente `internal/memory/readiness.go` si hace falta un campo para distinguir «no medido»
  de «medido en cero». `CHANGELOG.md` en `[Unreleased]`.
- Compatibilidad: el contrato de `musubi_readiness` es aditivo —checks y claves de evidencia
  nuevas dentro de una dimensión que ya existe—, así que la cabina y el CRM no se rompen. **Pero
  el puntaje de un proyecto con el grafo podrido BAJA**, que es exactamente el punto. Hay que
  decirlo en el CHANGELOG: un tablero que hoy muestra 1.0 puede mostrar menos sin que nada se haya
  roto.
- `musubi_readiness` es read-only y así se queda; no toca `registry.go`.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|--------|------------|
| **Inventar podredumbre donde no la hay.** En el cerebro central el grafo es federado y sus archivos no están en disco: medir frescura ahí daría «todo fantasma». | `graphFreshness` YA devuelve `(0,0,0)` cuando `arbolFueraDeAlcance()`. Pero un `(0,0,0)` de «no aplica» es indistinguible de uno de «está impecable», y creerle al cero es el error que este repo ya documentó en `profundidadDe`. La dimensión tiene que distinguir los dos casos, no promediarlos. |
| **Un cero ambiguo que puntúa como éxito** — el mismo bug que el radio ciego de `detect_changes`. | Los checks de frescura sólo se agregan cuando la medición APLICA. Donde no aplica, se declara en la evidencia y no se agrega un check que pase gratis. |
| El costo: `graphFreshness` hace `walkSourceTree()` + un `FileFingerprint` por archivo del grafo. `musubi_readiness` hoy es barata. | Medir el costo real en este repo (16.787 nodos) antes de cerrar `design`, y declararlo. Si duele, la decisión es explícita, no accidental. |
| Un test que pasa sin medir nada. | Regla 8 del repo: cada invariante con su sabotaje verificado en ROJO, y el sabotaje tiene que atacar el invariante que el test declara. |

## Criterio de éxito

1. Con el grafo podrido (`stale`/`ghosts`/`missing` > 0), `musubi_readiness` **no** puntúa 1.0 en la
   dimensión `grafo`, y la evidencia dice cuántos de cada uno.
2. Con el grafo sano, sigue puntuando 1.0 — medido contra este repo, que hoy da `stale:0, ghosts:0,
   missing:0` con 16.787 nodos.
3. Donde la medición NO aplica (árbol fuera de alcance), la dimensión lo DECLARA en vez de puntuar
   un cero tranquilizador.
4. Cada invariante con su sabotaje en rojo verificado.

## Evidencia que motivó esta propuesta (medida el 2026-11-01, este repo)

- `musubi_map` → `stale:0, ghosts:0, missing:0`, 16.779 nodos.
- `musubi_readiness` → dimensión `grafo`, score **1.0**, evidencia completa: `{nodos: 16787,
  aristas: 42540, aristas_por_tipo: {...}}`. Ni una palabra de frescura.
- `musubi_doctor` → 16 checks, **ninguno** del grafo de código.
- Contraejemplo buscado y encontrado, acotado: `profundidadDe`
  ([methods_detect.go:226](../../internal/mcp/methods_detect.go)) **sí** consume
  `GraphFileFingerprintsCtx` y declara `RadioCiego`. Pero es por diff y no global, y mira sólo el
  eje cobertura: un archivo STALE está en el mapa de fingerprints y pasa esa comprobación igual.
  Cubre parte de `missing`; no cubre `stale` ni `ghosts`.
