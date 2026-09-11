# Cabos abiertos — riel local

**Nada queda abierto sin dueño.** Lo que este track dejó afuera a propósito vive acá, con número,
con el porqué, y con quién decide. Un cabo sin registro no es un cabo cerrado: es uno que nadie va
a volver a mirar.

**Este archivo cubre `specs/riel-local/` y nada más.** Es la decisión de alcance que tomó gio el
2026-09-11: **un registro por track**, no uno solo para todo el repo. El de «Control de flota» vive
en `specs/control-de-flota/ABIERTO.md` y no adopta a nadie de acá. La alternativa —un único archivo—
se descartó porque ése ya tiene más de 3900 líneas y sumarle los tracks vecinos lo vuelve ilegible,
que es otra forma de que un cabo se pierda.

**Los números llevan prefijo del track (`RL`) a propósito.** Con tres registros en el repo, un «A1»
suelto no diría de cuál archivo es; `RL1` sí. El número es la identidad: uno solo por cosa, para
siempre, y uno nuevo va por encima del máximo en uso (hoy **RL3**) sin reciclar los libres.

**Lo que sale de acá sale con evidencia.** Cerrar una fila pide lo mismo que en el registro de
flota: decir qué se midió, con qué, y cuándo — no «ya está».

| # | Cabo | Por qué quedó afuera, y qué haría falta para cerrarlo | Dueño |
|---|---|---|---|
| RL1 | **El `-watch` de musubi-body sigue fabricando huérfanos** | Es de la terminal de `musubi-body`, no de este track: acá sólo se convive con el ruido clasificándolo como `sondeo`. Se despachó con la tercera medición. **Queda abierto porque la convivencia no es un arreglo**: mientras el productor siga generando huérfanos, cualquiera que mire el riel sin saber esto va a leer ruido como señal. Cerrarlo es de la otra terminal, y este registro existe para que el pedido no se pierda entre las dos. | la terminal de `musubi-body` |
| RL2 | **El grafo no se pinta con eventos locales** | Dibuja la memoria local y un evento sólo PULSA; no repinta. Es una decisión de alcance del track y no un defecto — repintar con cada evento sería caro y nadie lo pidió. Lo que falta antes de tocarlo es la pregunta previa: **cuántos eventos por minuto llegan de verdad**, porque si son pocos el repintado es gratis y si son muchos ni conviene intentarlo. Sin ese número, cualquier diseño acá es una corazonada. | **gio** — decisión de alcance |
| RL3 | **El central no escribe spool** | Ya reparte por HTTP, así que escribir además un spool serían **~100.000 líneas diarias para nadie**: nadie lo lee hoy. Queda anotado y no hecho, con el número medido al lado para que la próxima vez que alguien lo proponga la discusión arranque desde ahí y no desde cero. Lo que lo reabriría es un lector concreto —una reconstrucción tras caída, una auditoría— y entonces el costo se compara contra algo. | **gio** — sólo si aparece un lector |
