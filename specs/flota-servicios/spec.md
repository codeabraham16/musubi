# Spec — S12 · La entidad Servicio

Un **Servicio** es una unidad que corre **EN** una máquina de la flota: una unit de systemd, un
servicio de Windows, un contenedor. Pertenece a un `device` y **hereda su `project_id`**. Guarda el
**PRESENTE** —igual que `last_sample`—, nunca historia.

```
latido ->  POST /fleet/heartbeat  {"servicios":[{"nombre":"postgresql.service","salud":{...}}]}
lectura -> musubi_fleet_services  {"services":[{"nombre":…,"device":…,"estado":…,"fresco":…}]}
```

---

## Cuál de las dos «flotas» es ésta

En este mismo servidor «flota» significa dos cosas (registro **B17**). La sección **Flota** del CRM
inventaría *bots, puentes y servicios publicados a mano*; ésta es la flota de **máquinas que se
miden solas**, y un `fleet.Servicio` es una unidad que corre adentro de una de ellas.

Comparten el nombre y no comparten nada más. Está escrito en el encabezado de
`internal/fleet/servicio.go` y en el comentario de la migración 36, que son los dos lugares donde
alguien va a llegar buscando.

---

## H1 · Los cuatro estados, y el cuarto es el que importa

### S1 — `desconocido` NO es `detenido`

Una máquina que no pudo enumerar sus servicios —el agente arrancó a medias, systemd no contestó, el
usuario no tiene permiso— **no está diciendo que el postgres esté caído**. Confundirlos es el mismo
modo de falla que el 0 % de CPU que se cree y no se arregla, y es el que despierta a alguien a las
4 de la mañana por nada.

La decisión vive en UNA función del dominio, `Servicio.EstadoActual()`, y no en la fila de la tool
ni en el panel: con tres consumidores decidiendo cada uno qué significa «sin salud», la mitad de la
flota diría una cosa y la otra mitad otra.

### S2 — «declarado y todavía sin muestras» es un estado legítimo

Un servicio que alguien dio de alta a mano y que nadie midió todavía **tiene que poder existir**:
`UltimoReporte` en cero, `Salud` nil, `ultimo_reporte: null` en la respuesta, y ni un error en el
camino. `SaludDesdeTexto("")` devuelve `(nil, nil)` por eso.

### S3 — el frescor es un eje APARTE del estado

`estado` es lo ÚLTIMO que se supo; `fresco` es si eso sigue valiendo. Un «corriendo» con un reporte
de hace dos días no está corriendo: está **sin noticias**. Colapsarlos —pintar `desconocido` cuando
el dato envejece— perdería la única pista de qué estaba pasando cuando se dejó de saber.

Se DERIVA al servir (`Servicio.Fresco`), con el umbral de la máquina. No hay columna.

---

## H2 · La tenencia

### S4 — el `project_id` sale del DEVICE, nunca del pedido

`AltaServicio` resuelve la máquina y **copia** su proyecto, ignorando lo que declare el cliente. Sin
foreign keys —y no hay ni una en el repo— un servicio atribuido a un proyecto distinto del de su
máquina es perfectamente representable, y esa desalineación es una fuga de tenant con la forma
exacta de **A6**.

### S5 — una máquina sólo puede tocar sus propios servicios

`ReportarServicios` recibe el `deviceID` derivado del **TOKEN** y todo lo que toca lleva
`AND device_id = ?`. Sin esa guarda, cualquier máquina de la flota puede reportar que el postgres de
producción está caído: un error de **seguridad**, no de datos.

### S6 — declarar en la máquina de otro tenant se rechaza con el MISMO texto que si no existiera

`writeOriginFor` sella el proyecto y el device se resuelve **por nombre dentro de ese proyecto**,
así que la máquina ajena simplemente no aparece. El mensaje es palabra por palabra el de
`musubi_fleet_revoke`: distinguir «ajena» de «inexistente» convertiría la tool en un **oráculo** de
qué máquinas tiene el vecino.

Lo mismo con el filtro `device` de la lectura, que responde **vacío** y no un error.

### S7 — la compuerta de lectura es POR MÁQUINA

`musubi_fleet_services` filtra cada servicio con `PuedeSobreDevice(p, d, fleet.CapMetrics)` sobre SU
device. Filtrar sólo por proyecto dejaría ver el inventario de una máquina sobre la que esta
credencial no tiene nada concedido.

**Se reusa `metrics` y no se inventa una Cap nueva.** Qué corre en una máquina es telemetría del
host, del mismo peso que su uso de CPU. Y el costo de la Cap nueva sería alto y silencioso: la
matriz por tier, la lista `todas` de `capsQuePuede` —cuyo orden dibuja la columna «admite / puedo»—
y seis bucles exhaustivos en tres paquetes. Peor: el barrido de aislamiento le da al atacante
`metrics`, `exec` y `screen` sobre `"*"` a propósito, así que con una Cap nueva ese barrido pasaría
probando la **compuerta de capacidades** en vez de la **tenencia** — verde por el motivo
equivocado.

---

## H3 · La forma de la tabla

### S8 — no existe una columna de estado

Ni `healthy`, ni `up`, ni `activo`. El estado se deriva de `last_health` y de la EDAD de
`last_report`. Un booleano guardado se queda en `true` para siempre cuando la cosa muere de golpe,
que es justo cuando querés saber que se cayó. Lo custodia una prueba de **forma** que recorre
`PRAGMA table_info(services)`, y su gemela sobre la struct del dominio.

### S9 — no hay serie temporal (B5 sigue en pie)

Musubi guarda el presente. 40 máquinas × 40 servicios cada 30 s son millones de filas que nadie
consulta salvo para graficar; la historia la guarda Prometheus.

### S10 — el único es `(project_id, device_id, name)`

El nombre de un servicio sólo es único **dentro de su máquina**: dos hosts pueden correr cada uno su
`postgres`. Con `(project_id, name)`, el segundo no se podría enrolar — y el síntoma sería «el alta
falla en la máquina nueva», que nadie asocia con un índice. La unicidad la decide el ÍNDICE y no un
`SELECT` previo: entre un SELECT y un INSERT hay una carrera y la base no la tiene.

### S11 — no hay foreign keys

Serían las primeras del repo, y sólo para esta tabla. La integridad se sostiene en el ALTA
(resolver el device y copiar su proyecto) y en un escaneo tolerante, nunca en el esquema. El
aislamiento va por el `project_id` **denormalizado**, como `device_commands` y `screen_sessions`:
un JOIN ataría cada lectura a que la fila de la máquina siga existiendo con el mismo proyecto.

### S12 — revocar una máquina revoca sus servicios

`RevocarDevice` arrastra `UPDATE services SET revoked = 1 WHERE device_id = ?` en la **misma
transacción**. Se eligió sobre el JOIN al leer (contradice el patrón denormalizado) y sobre «no
hacer nada» (un servicio visible de una máquina revocada es exactamente lo que no querés ver
después de un incidente). Las filas **quedan**, como la del device.

---

## H4 · La puerta del dispositivo

### S13 — el latido no gana identidad

`fleet.ReporteServicio` no tiene NINGÚN campo de identidad: ni device, ni project, ni id. Que no
tenga por dónde pasarlos es la garantía, no la disciplina. Y los tags van en **castellano**
(`nombre`, no `name`) porque `agent_test.go` barre el JSON entero buscando `"name"`.

### S14 — un bloque de servicios sobrado no tumba el latido

Es **D7** con otra ropa: estar viva y saber enumerarse son cosas distintas. Se descarta el bloque
ENTERO y no se trunca: un inventario a medias haría que la poda por ausencia dé de baja los
servicios que quedaron afuera del corte.

El techo del cuerpo pasó a `latidoMaxBytes`, pero **la muestra conserva el suyo**
(`fleet.MuestraMaxBytes`), medido sobre el JSON crudo. Con un solo techo compartido, una muestra de
100 KiB entraría por la puerta que se abrió para las units.

### S14b — un nombre bueno con una salud ilegible NO se descarta

Los dos errores no son el mismo error. Sin **nombre** no hay servicio del que hablar y el reporte se
saltea; sin **salud** sí lo hay —la máquina lo nombró— así que el servicio se guarda como
`desconocido`, con `last_report` avanzado y la última salud buena intacta.

Pasa de verdad: `systemctl list-units` da los nombres y `systemctl show` puede fallar por permisos
en la misma corrida. Descartando el reporte entero, esa máquina no tendría inventario **nunca** y
nadie sabría por qué.

Es la misma asimetría que **D7**, un nivel más abajo: «la máquina está viva» / «la máquina supo
medirse» se vuelve «el servicio existe» / «la máquina supo medirlo».

### S14c — el bloque descartado NO se descarta en silencio

La respuesta del latido lleva una nota `servicios`, gemela de la de `muestra`: «guardados: N
nuevos, M actualizados, K de baja» o «descartados: …» con el motivo. Sin ella, un bloque que se cae
por la capacidad que falta o por el techo se ve —**desde la máquina**— idéntico a uno que nunca se
mandó, y quien puede arreglarlo es justamente el que no lee los logs del cerebro.

### S15 — la poda por ausencia con lista vacía no borra nada

Calcado de `PodarEstadoDePoliticas`: «este device no reportó ningún servicio» es también lo que se
ve cuando el agente arrancó a medias. Vaciar el inventario por eso es irreversible; conservarlo
cuesta unas filas.

---

## H5 · La vista

### S16 — un guion nunca es un cero, y hay TRES cosas que se dibujan distinto

| lo que se dibuja | qué significa |
|---|---|
| `—` | la tool de servicios no contestó (permisos, cerebro viejo). **No sabemos.** |
| `0` | la tool contestó y esta máquina no tiene ningún servicio declarado. **Es un dato.** |
| `n/m` + fichas | n corriendo con noticias frescas de m, y una ficha por cada uno que no está sano |

Las cuatro marcas de estado son **distintas entre sí** —lo custodia una prueba que las compara— así
que un `desconocido` (?) nunca se dibuja como un `detenido` (■). Lo que no está fresco va en ámbar
con un ⧗ y **conserva su estado**: la ficha sigue diciendo lo último que se supo, y el color dice
que eso ya tiene polvo.

### S17 — si falla la tool de servicios, la flota se sigue viendo

Tercera llamada, error ignorado, molde del de las métricas. Propagarlo haría que un problema de
permisos sobre los servicios borre la FLOTA entera de la pantalla.

### S18 — cero botones

El invariante I4 de `specs/flota-panel/spec.md` sigue en pie: reiniciar un servicio se hace con
`musubi_fleet_exec`, que deja su línea en la bitácora.
