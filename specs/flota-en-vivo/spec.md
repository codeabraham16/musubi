# flota-en-vivo — spec

## Comportamiento

- El daemon local con sync configurado (`SyncConfig.HasDestination()`) y `flota_vivo` no apagado
  corre `RunFlotaVivo`: suscriptor in-process del live feed que junta eventos y los empuja como
  JSON a `POST <central>/api/flota` con el bearer del sync.
- Sólo se empuja lo NUEVO desde el arranque (el backlog del ring no viaja: «en vivo» no es
  «historia», y un restart no re-manda).
- El central valida el token, sanea cada evento y lo publica en su propio live feed con
  `origen: "flota"`. De ahí en más es un evento más del stream que el panel ya muestra.

## Invariantes (cada uno con su test visto ROJO bajo sabotaje)

- **I1 — sólo el trabajo viaja.** El remitente filtra `kind == "trabajo"` ANTES de mandar: el
  sondeo (99,92 % del tráfico local) no cruza la red ni una vez. Sabotaje: quitar el filtro.
- **I2 — la identidad la sella el server.** `principal` y `project` del evento publicado salen
  del TOKEN autenticado, nunca del body: un batch que declara `principal: "gio"` sale publicado
  con el principal del token. Sabotaje: copiar el principal del body.
- **I3 — el receptor acota.** Batch > tope (256 eventos) o body > 256 KiB ⇒ 400/413, sin
  publicar nada. Sabotaje: quitar el tope del batch.
- **I4 — sin contenido, por construcción.** El decode es estricto (`DisallowUnknownFields`): un
  evento con un campo extra (`content`, `args`, lo que sea) rechaza el batch entero con 400.
  Es la mitad receptora del invariante L1 del feed. Sabotaje: decode laxo.
- **I5 — el kind lo decide el server.** `clasificarTool` se recomputa al recibir: un batch que
  declara `musubi_sync_pull` como `trabajo` se publica como `sondeo`. Sabotaje: confiar en el
  kind del cliente.

## No-objetivos

- Persistencia en el central (el feed es el PRESENTE; la historia de cada máquina vive en su
  ledger local). Un evento de flota no toca la base.
- Reintentos con cola durable en el remitente: es telemetría best-effort, no memoria. Lo que se
  pierde en un corte se pierde — el sync de MEMORIA sí tiene outbox durable, esto no lo necesita.
- Cambios de panel: el evento llega con principal y el censo existente lo mapea.
