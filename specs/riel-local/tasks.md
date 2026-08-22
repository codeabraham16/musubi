# Tasks — El riel también ve lo local

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | El vertedero: un archivo por PID, acotado, que se borra al salir | `internal/mcp/spool.go` |
| T2 | `LiveEvent.Origen` y el spool colgado de `publicarUso` | `internal/mcp/livefeed.go`, `server.go` |
| T3 | Encenderlo **sólo** en el daemon stdio, no en `serve` | `cmd/musubi/main.go` |
| T4 | El lector: sigue el directorio, se recupera del truncado, poda muertos | `cmd/musubi/livelocal.go` |
| T5 | PID vivo, por plataforma | `cmd/musubi/procvivo_{windows,unix}.go` |
| T6 | El riel existe sin central; `explicarSinCentral` reemplaza a `handlerStreamApagado` | `cmd/musubi/livestream.go`, `dashboard.go` |
| T7 | El panel muestra la procedencia y deja de confundir los `seq` de los dos orígenes | `assets/src/dashboard.mjs`, `dashboard.html` |

## Invariantes, y qué los custodia

| Inv | Test | Sabotaje que lo hace fallar |
|---|---|---|
| A1 | `TestLectorEntregaEventosConSuHoraReal` | el offset no avanza → re-entrega todo |
| A2 | `TestSpoolAguantaEscritoresConcurrentes` | ⚠️ **el candado NO lo cubre este test** — ver abajo |
| A3 | `TestLectorQueArrancaTardeVeLoAnterior` | arrancar al final del archivo (`tail -f`) |
| A5 | `TestSpoolRotoNoRompeAlLlamador` | quitar la guarda de receptor nil → pánico |
| A6 | `TestSpoolNoCreceSinFin` | chequear el tope DESPUÉS de escribir |
| A7 | `TestSpoolSeBorraAlCerrar`, `TestPodarSacaLosMuertosYNoLosVivos` | no borrar al cerrar · podar sin mirar si el proceso vive · podar sin gracia |
| — | `TestLectorSeRecuperaDeUnTruncado` | sacar la relectura desde cero |
| — | `TestLectorNoEntregaLineasAMedias` | entregar la línea incompleta |

**A2 tiene un hueco declarado.** Saboteé `escribir` quitándole el mutex y el test pasó **20 de 20**,
incluso forzando el truncado en medio de ocho goroutines. Sin `-race` esa carrera no se manifiesta a
esta escala. Al candado lo custodia la CI, que corre con `-race` — y eso no es una esperanza: el
mismo día que se escribió esto, `-race` atrapó en CI una carrera de un fixture que en esta máquina
había pasado verde. Está anotado en el propio test para que nadie confíe en una valla que no existe.

## Verificado end to end, no sólo por unidad

Panel sin central + un daemon stdio ejecutando tools de verdad:

```
"tool":"musubi_recall", "kind":"trabajo", "origen":"local"
"tool":"musubi_doctor", "ms":2054.099,    "origen":"local"
"tool":"musubi_doctor", "ms":1390.922,    "origen":"local"
```

Y el ciclo de vida del archivo, observado segundo a segundo mientras el daemon vivía: aparece a los
4 s, tiene su primer evento a los 6, y **desaparece a los 7** cuando el daemon sale — que es A7.

## Lo que queda fuera, a propósito

- **El `-watch` de musubi-body** sigue fabricando huérfanos. Es de su terminal; acá sólo se convive
  con el ruido clasificándolo `sondeo`. Despachado con la tercera medición.
- **El grafo no se pinta con eventos locales.** Dibuja la memoria local; un evento sólo pulsa.
- **El central no escribe spool.** Ya reparte por HTTP; hacerlo además serían ~100.000 líneas
  diarias para nadie.
