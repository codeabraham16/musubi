# flota-en-vivo — propuesta

## El problema, con la medición que lo destapó

El usuario trabaja en dos máquinas y el panel del central no muestra nada (2026-08-26, verificado
con captura de pantalla + captura del SSE). No es un bug del canal: el stream funciona — es que
**el trabajo de las terminales nunca viaja**. Cada terminal le habla a su cerebro LOCAL
(local-first, por diseño); al central sólo llegan la captura automática (un save cada varios
minutos), el sync (sondeo, oculto a propósito) y las llamadas directas vía `musubi-cerebro`.

La señal que falta **ya se emite**: `livefeed.go` publica cada invocación en el instante en que
termina (tool, outcome, ms, kind — jamás contenido, invariante L1), y cada máquina la muestra en
SU panel local. Lo único que no existe es el caño que la lleve al central.

## Lo que se propone

Un caño local→central de tres puntas:

1. **Remitente** (daemon local): se suscribe a su propio feed, filtra SOLO trabajo (medido: el
   99,92 % de las invocaciones locales son sondeo), y empuja lotes al central cada ~2 s con el
   MISMO token del sync. Best-effort estilo hooks: si el central no está, descarta y sigue.
2. **Receptor** (central): `POST /api/flota` — valida el token, **sella identidad y kind
   server-side** (el cliente no puede disfrazarse ni de quién es ni de qué clase es su evento),
   y publica en el feed del central con `origen: "flota"`.
3. **Panel**: gratis — el evento llega con principal, el censo lo mapea a su actor, la rama pulsa.

## Resultado

Guardás una nota en la laptop → 2 s después la rama de davantis se enciende en el panel del
central. El central pasa de «lo que me llega» a **la sala de observación de la flota** — la misma
premisa del cuerpo.

## Frontera de confianza (por qué el opt-in es el del sync)

La telemetría viaja activa por default SÓLO donde el sync ya tiene destino configurado
(`HasDestination()`): una máquina que ya empuja su MEMORIA COMPLETA al central no revela nada
nuevo contando nombres de tools. `flota_vivo: false` la apaga explícito. La máquina de gio
(sync apagado) queda afuera sola, sin config nueva.
