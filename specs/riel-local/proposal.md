# Proposal — El riel también ve lo local

## El hueco

El riel en vivo del panel muestra lo que pasa en el cerebro **central** y nada más. El feed
(`internal/mcp/livefeed.go`) vive DENTRO del `McpServer`, y el único que lo expone por HTTP es
`ListenAndServeHTTP` — o sea `musubi serve`. Los daemons stdio locales publican sus eventos a un
feed **que nadie escucha**: se construyó el emisor y no se encendió el receptor.

Efecto práctico: trabajás contra la memoria local todo el día y el riel no muestra nada tuyo.

## Lo que se midió antes de proponer nada

Sobre el ledger local de este repo, el 2026-08-22:

```
109.687 invocaciones HOY
    musubi_phase        36.516
    musubi_sync_status  36.516     <- el MISMO numero exacto: firma de sondeo
    musubi_work         36.516
    --- trabajo de una persona ---
    musubi_conflicts        52
    musubi_doctor           50
    musubi_judge            15
    musubi_memory_expand    10
    musubi_save_observation  9
```

**136 de 109.687 son trabajo real (0,12 %).** El resto sale de procesos `bridge -watch` huérfanos de
musubi-body — hallazgo ya abierto, despachado a su terminal, y mitigado acá matando los 7 huérfanos
del día. Importa para este diseño por dos razones:

1. **La clasificación no es opcional.** `work`, `phase` y `sync_status` ya están en `toolsDeSondeo`,
   así que el riel las separa del trabajo. Sin eso, el riel local sería una cascada ilegible.
2. **Hay 7 daemons stdio vivos a la vez** (medido). Cualquier diseño tiene que aguantar N escritores
   concurrentes, no uno.

## Por qué no alcanza con leer el ledger

Es exactamente lo que se descartó al construir el feed, y por tres razones medidas:

- el buffer vuelca a disco cada 10 s (`defaultLedgerFlushSeconds`),
- `created_at DEFAULT CURRENT_TIMESTAMP` estampa la hora del **INSERT**, o sea la del volcado —
  llegaron a compartir marca 23 filas,
- la columna tiene resolución de **1 segundo**.

Eso es historia con retraso, no presente. El ledger sigue siendo el lugar correcto para la historia;
lo que falta es el presente.

## Por qué un archivo por daemon y no uno compartido

Medido: **7 daemons concurrentes**. Un único `live.jsonl` con 7 escritores en Windows es contención
de escritura y líneas entrelazadas; y truncarlo para acotarlo mientras otros lo tienen abierto es
peor. Con un archivo por proceso cada uno es dueño del suyo: no hay contención, cada uno acota el
propio, y el que lee no necesita coordinar con nadie.

## Forma

```
daemon (pid 15980) --> .musubi/live/15980.jsonl  --.
daemon (pid 15884) --> .musubi/live/15884.jsonl  --+--> musubi dashboard (sigue el directorio)
daemon (pid  1348) --> .musubi/live/1348.jsonl   --'            |
                                                                v
                                                   riel: local + central, marcados
```

El panel ya sabe mezclar dos orígenes: hoy pinta el backlog del central y los eventos que llegan
después. Sumar `local` es una procedencia más en el mismo riel, no un riel nuevo.

## Lo que NO entra

- **No se toca el ledger.** Historia y presente siguen separados a propósito.
- **No se arregla el `-watch` de musubi-body.** Es de su terminal; acá sólo se convive con el ruido
  clasificándolo.
- **No se pinta el grafo con eventos locales.** El grafo dibuja la memoria local y un evento sólo
  dispara un pulso; esa regla no cambia.
