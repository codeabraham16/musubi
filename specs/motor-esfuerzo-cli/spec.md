# Spec — `motor`: `--effort` desde el nivel del cliente

Extiende la spec del traductor de entrada (F3a) y del Lanzador (L6 / F2) de
`specs/motor-proxy-stream/spec.md`. Misma regla del repo: cada invariante lleva su test, y el
sabotaje ataca **el** invariante declarado y se verifica ROJO; un verde bajo sabotaje es una prueba
vacua.

Cinco invariantes, cinco sabotajes, **5/5 ROJO** verificado antes de revertir. El sabotaje se aplica
con el arnés (`// arnes:` en el test), y como cada anotación se aplica por Edit con match exacto, una
ancla mal escrita no se puede colar en silencio: o la anotación engancha el código real o el Edit
falla. Dos trampas del arnés que importan acá: una ancla que no engancha **no rompe nada** (verde
falso), y un eje que todos los casos de un test fijan en el mismo valor es un punto ciego — por eso
los casos de `--effort` afirman **valores esperados DISTINTOS por eje** (`xhigh→"xhigh"`, `max→"max"`
son distintos, no clampados a `high`).

## Invariantes del traductor de entrada (extiende F3a)

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| E30 | El nivel del cliente **llega** al Pedido: `PedidoDesdeProxy` puebla `Esfuerzo` con `esfuerzoDesdeReasoning(options.reasoning)`. Sin el cableado, el dato que el cliente mandó se pierde entre el proxy y el motor. | `TestElEsfuerzoSaleDelNivelYPasaDerechoAlCLI` (vía `desdeJSON`) | ROJO |
| E31 | Los cinco niveles de `--effort` **pasan derecho**, sin clampar. `low/medium/high/xhigh/max` → el mismo valor. Es la asimetría con el budget de thinking, que sí clampa `xhigh/max→high`: `--effort` los acepta nativos (medido 2026-11-07), así que recortarlos le robaría al usuario los dos niveles de arriba justo cuando los pide. | idem (casos `xhigh`, `max` con esperado distinto a `high`) | ROJO |
| E32 | `minimal` **se mapea a `low`**, el piso del CLI. pi tiene `minimal`; `--effort` no. Pasarlo crudo manda un `--effort minimal` que el CLI rechaza y mata el turno. Y un nivel **desconocido** cae a `""` (sin bandera), la caída segura. | idem (caso `minimal`→`low`; caso vacío→`""`) | ROJO |

La batería es una tabla de 8 casos sobre `desdeJSON`, que arma el JSON del proxy y lo pasa por
`PedidoDesdeProxy`, afirmando `p.Esfuerzo`. Cubre: sin `reasoning` (→`""`), los cinco válidos
(passthrough), `minimal` (→`low`), y un valor inventado (→`""`). Cada caso afirma un **valor
esperado propio**: no hay un eje fijado en común que tape un passthrough roto.

## Invariantes del Lanzador (extiende L6 / F2)

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| M52 | El argv **lleva `--effort <nivel>`** cuando el Pedido trae esfuerzo, como par consecutivo `[--effort, high]` después de `--model`. Sin la línea de `append`, la bandera no sale y el CLI usa su default: el selector del cliente queda sin efecto. | `TestElArgvLlevaLasBanderasQueHabilitanElProtocolo` (aserción `"--effort high"` sobre el argv aplanado) | ROJO |
| M53 | Un Pedido **sin esfuerzo NO emite `--effort`**. La guarda `if p.Esfuerzo != ""` es lo que lo garantiza: sin ella viajaría un `--effort ""`, que el CLI toma como un nivel vacío inválido y rechaza. Vacío tiene que ser *ausencia de bandera*, no *bandera con valor vacío*. | idem (bloque `pelado`: un `Pedido{}` cuyo argv NO debe contener `--effort`) | ROJO |

**Nota sobre M52/M53 — los dos lados del mismo `if`.** El test afirma las dos mitades en el mismo
lugar: con `Esfuerzo: "high"` exige el par consecutivo (sabotaje M52 = borrar el `append`), y con un
`Pedido{}` pelado exige que `--effort` **no aparezca** (sabotaje M53 = volver la guarda siempre
verdadera, que haría salir `--effort ""`). Afirmar sólo la presencia dejaría pasar una guarda rota;
afirmar sólo la ausencia dejaría pasar un `append` borrado. El par consecutivo —no `--effort` y
`high` sueltos en cualquier parte del argv— es lo que ata la bandera a su valor: un `Contains` laxo
daría verde si `high` apareciera por otro motivo (p. ej. dentro de `--model`).

## Lo que el compilador ya custodia, y no se saboteó

`Esfuerzo` es un `string` con cero y vacío indistinguibles para el compilador, así que no hay un
sabotaje "de tipo" que valga. Las cinco pruebas atacan comportamiento observable (el valor
normalizado, la presencia/ausencia de la bandera), no la compilación. Ninguna se pone roja por un
import huérfano ni por un campo sin usar — eso mediría al compilador, no al invariante.

## Evidencia reunida antes del debate

- `go build ./...` + `go vet ./...` + `go test ./internal/motor -count=1` → VERDE (con `GOWORK=off`,
  como el CI; `-count=1` para no creerle a un verde cacheado).
- 5/5 sabotajes con veredicto ROJO, revertidos.
- Alcance limpio: 5 archivos de `internal/motor` + `CHANGELOG.md`, nada más.
- Debate adversarial (panel `estandar`: 3 jueces / 2 rondas / quórum 2, dimensionado por
  `musubi_detect_changes`): 3/3 `real`, decidido, cerrado, `gated_choice` satisfecho.
