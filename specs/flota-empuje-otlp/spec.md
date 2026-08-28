# Spec — S11 · El empujador OTLP

El cerebro EMPUJA la telemetría de la flota a un receptor OTLP, además de seguir sirviéndola en
`/metrics`. Contrato observable; cada invariante tiene una prueba que **sabe fallar**.

```
cada 30s ->  POST http://127.0.0.1:9099/api/v1/otlp/v1/metrics
             {"resourceMetrics":[{ ...las mismas musubi_fleet_device_* que /metrics... }]}
```

---

## La tesis, en una línea

**El empuje no es un segundo camino de export: es el mismo camino con otra boca.** Comparte con
`/metrics` la selección de máquinas (`devicesVisiblesParaMetricas`), la tabla de series
(`seriesDeFlota`) y el juego de labels (`labelsDeFlota`). Lo único suyo es el sobre JSON, el POST y
el lazo.

Si el empujador tuviera su propio `for` sobre `ListarDevices` o su propia copia de la tabla, el
diseño ya estaría mal: dos exportadores discrepan, y la discrepancia se descubre semanas después,
cuando dos dashboards muestran cosas distintas.

---

## H1 · La autoridad

### E1 — El empujador actúa con la autoridad de un principal NOMBRADO, nunca con `nil`

**Es el riesgo número uno del slice, y compila sin quejarse.** Un lazo interno no tiene request, así
que no hay `principalFrom(ctx)` que valga: el empujador nace sin principal. Y con `nil`,
`proyectosVisibles` marca `federado=true` y `PuedeSobreDevice` devuelve `true` incondicionalmente
—es la confianza del stdio local—, o sea que un empujador descuidado exportaría la telemetría de
**todos los tenants** a un endpoint externo, sin romper ninguna prueba existente y sin una línea de
log.

`armarPayloadOTLP` **rechaza** el nil en vez de tratarlo como «ve todo».

### E2 — Sin principal válido, el servidor no arranca

Tres casos, los tres de arranque: endpoint sin principal, principal inexistente, principal sin
ninguna concesión `metrics`. De las dos formas de enterarse, «el servidor no arranca» es mucho más
barata que «los gráficos están vacíos y nadie sabe desde cuándo» — un empuje mudo se ve, desde
afuera, idéntico a todo tranquilo.

### E3 — El tenant sale de la fila, jamás de lo que la máquina declare

Un latido puede decir que es de otro proyecto: no tiene dónde aterrizar. El label `project` sale de
`devices.project_id`, que se fijó al enrolar. Es B4/D5 llegando hasta el export.

---

## H2 · Lo que viaja

### E4 — El empuje exporta exactamente lo mismo que el scrape

Mismo principal y mismo reloj ⇒ el conjunto (serie × labels × valor) es idéntico.

### E5 — Lo desconocido no se emite como cero, tampoco en OTLP

La regla central del export, en el otro formato: sin valor, no hay punto; sin puntos, no hay
métrica. Y su reverso: un `up` que vale **0** sí viaja, con su cero escrito — de él vive
`MaquinaCaida`. Por eso `asDouble`/`asInt` son punteros: con `float64` + `omitempty`, el 0 legítimo
perdería su campo y llegaría un punto sin valor.

### E6 — Los labels son los cuatro, y salen de la fila

`{device, project, tier, os}`, en ese orden y ninguno más. Nada de lo que la máquina reporta de sí
misma (versión del agente, dirección, id de RustDesk) entra como atributo: eso la dejaría
re-etiquetándose sola, y la cardinalidad de la serie la acota el cerebro.

### E7 — El empuje no lleva las métricas del servidor

`musubi_tool_*`, `musubi_sync_*` y `musubi_fleet_policy_actions_total` se quedan en `/metrics`.
Están detrás de auth desde la auditoría 2026-07-26 #9, y la de políticas **no lleva etiqueta de
máquina a propósito**: empujarlas a un store sin credencial deshace esa corrección por la otra
puerta.

### E8 — El sobre tiene la forma de la especificación

`resourceMetrics[].scopeMetrics[].metrics[].gauge.dataPoints[]`; `timeUnixNano` **string**; `asInt`
**string** y `asDouble` número; atributos `{key, value:{stringValue}}`; ninguna métrica con
`dataPoints` vacío.

### E9 — La unidad no puede renombrar la serie

El receptor OTLP de Prometheus **normaliza el nombre con la unidad**: a un gauge con unidad `"1"`
le agrega el sufijo `_ratio`, y `musubi_fleet_device_up` llegaría como
`musubi_fleet_device_up_ratio` — con las 12 reglas de alerta evaluándose y sin disparar nunca. La
regla: o no se declara unidad, o el nombre ya la lleva en el sufijo (`_bytes`, `_seconds`,
`_celsius`, `_percent`).

### E10 — Un solo reloj

El instante con el que se decide `up` es el que sella todos los puntos.

---

## H3 · El lazo no puede lastimar al cerebro

### E11 — Un destino colgado no frena ninguna tool

Timeout propio y corto en el `http.Client`, ningún candado del servidor. Un cerebro que se cuelga
porque el sistema de métricas no contesta es peor que no tener métricas.

### E12 — Un empuje en vuelo a la vez

Un destino más lento que el tick acumularía goroutines y payloads en un proceso que vive días.

### E13 — Revocar al principal del empuje lo apaga en el acto

El principal se resuelve **en cada tick** contra el snapshot vigente del registro. Una política
fantasma queda inerte (falla cerrada); un empujador fantasma **sigue mandando datos**.

### E14 — El secreto entra por referencia y no se escribe en ningún lado

El bearer viene de la variable que nombra `auth_token_env`. Se rechazan al arranque: un `http://`
remoto sin `allow_insecure_token`, y una URL con userinfo. Ni el token ni el **cuerpo de la
respuesta del destino** entran en el error que se logea — un proxy mal configurado puede devolver
el request, y ese request lleva el bearer.

### E15 — La falla del empuje se ve desde el tirón

`musubi_push_failures_total`, `musubi_push_datapoints` y `musubi_push_last_success_seconds` salen
por `/metrics`, no adentro del propio empuje: un mecanismo de monitoreo cuya única forma de avisar
de su propia muerte es él mismo no avisa nunca. `last_success` se **omite** mientras nunca haya
habido un empuje aceptado — un 0 sería el unix epoch, que se lee como un bug del panel y no como
«esto nunca funcionó».

### E16 — El truncado se anuncia

El push no tiene dónde poner un comentario que el parser ignore, así que el aviso va al log, una
vez (es un estado, no un evento).

---

## H4 · El despliegue

### E17 — El receptor se habilita en los DOS caminos, y la duplicación está decidida

**Prometheus no acepta OTLP por defecto.** Sin `--web.enable-otlp-receiver` el POST devuelve 404 y
el empuje muere en silencio con la configuración del cerebro perfecta. El flag va en el instalador
systemd **y** en el compose.

Y si el mismo Prometheus scrapea `/metrics` y recibe el push, las mismas series entran por dos
caminos (`instance="musubi-brain"` contra `instance="musubi-otlp-push"`) y las 12 reglas de flota
disparan **doble**. La receta —`metric_relabel_configs` que descarta `musubi_fleet_device_.*` del
scrape— queda escrita en `prometheus.yml`, que es donde la va a leer quien encienda el push.

**No se habilita `--web.enable-remote-write-receiver`**: es una puerta de escritura anónima sobre
los datos de los que viven las alertas (inyectar `musubi_fleet_device_up 1` apaga `MaquinaCaida` y
`FlotaSinTelemetria`), y el empuje no la necesita.

---

## Nace apagado

`fleet.otlp.endpoint` vacío ⇒ no hay empuje, no hay validación, no hay lazo. Encender una salida de
datos **hacia afuera** es una decisión que alguien toma, no un default que se hereda al actualizar.
El tirón de `/metrics` sigue funcionando igual, encendido o apagado esto.
