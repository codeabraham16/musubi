# Cabos abiertos — profundidad derivada del diff

**Nada queda abierto sin dueño.** Lo que este track dejó afuera a propósito vive acá, con número,
con el porqué, y con quién decide. Un cabo sin registro no es un cabo cerrado: es uno que nadie va
a volver a mirar.

**Este archivo cubre `specs/profundidad-derivada-del-diff/` y nada más.** Es la decisión de alcance
que tomó gio el 2026-09-11: **un registro por track**, no uno solo para todo el repo. El de
«Control de flota» vive en `specs/control-de-flota/ABIERTO.md` y no adopta a nadie de acá. La
alternativa —un único archivo— se descartó porque ése ya tiene más de 3900 líneas y sumarle los
tracks vecinos lo vuelve ilegible, que es otra forma de que un cabo se pierda.

**Los números llevan prefijo del track (`PD`) a propósito.** Con tres registros en el repo, un «A1»
suelto no diría de cuál archivo es; `PD1` sí. El número es la identidad: uno solo por cosa, para
siempre, y uno nuevo va por encima del máximo en uso (hoy **PD4**) sin reciclar los libres.

**Tres de estas cuatro filas piden LA MISMA COSA: un número que todavía no se midió.** No es
casualidad y conviene verlo junto: este track calibró sus escalones contra un plan y no contra una
distribución, así que cada cabo es «esto hay que medirlo antes de tocarlo». Cerrarlos es sobre todo
salir a medir.

| # | Cabo | Por qué quedó afuera, y qué haría falta para cerrarlo | Dueño |
|---|---|---|---|
| PD1 | **`minima` va a ser raro en este repo hasta que alguien indexe el grafo** | Hay `.go` sin un solo nodo en el índice, y el piso de honestidad los eleva a `estandar`. **Es el comportamiento correcto y no un bug**: sin nodos no se puede afirmar que el cambio sea chico, y `estandar` es la respuesta honesta. Lo que falta antes de calibrar los escalones es el número de al lado: **cuántos archivos están fuera del índice**. Calibrar sin eso ajustaría los cortes para compensar una ceguera en vez de arreglarla. | quien retome el track |
| PD2 | **Los escalones no se calibraron contra el historial de PRs del repo** | Los cortes salen del plan, no de una distribución medida. Un barrido sobre los últimos N merges diría si `estandar` cae donde tiene que caer o si la mayoría termina en `profunda` — y si termina en `profunda`, el aparato pide revisión profunda casi siempre, que es lo mismo que no pedir nada. **El barrido es la tarea**, y hasta que exista los escalones son una hipótesis. | quien retome el track |
| PD3 | **La renumeración deja poco margen para F4** | `Rules` pasó de 3.120 a 3.936 runas sobre un umbral de 5.000: quedan **1.064** y F4 proyectaba unas 900. Entra, pero justo. **Conviene medirlo ANTES y no después**: descubrir que no entra a mitad de F4 obliga a renumerar con el trabajo a medio hacer, que es cuando renumerar sale caro. Cerrarlo es correr la proyección de F4 contra el número real. | quien retome el track |
| PD4 | **`TestElBucleSeDetieneAlSerRevocado` falla, y falla igual en `origin/main` limpio** | Medido 3 de 3 en las dos ramas, así que es **preexistente y ajeno a esta fase**: se anotó y no se arregló acá, que es lo correcto —arreglar de paso un defecto ajeno mezcla dos cosas en un mismo diff y esconde cuál rompió qué—. Pero un test que falla siempre deja de leerse, y arrastra a los de al lado. Cerrarlo es de quien sea dueño de ese bucle, y esta fila existe para que el pedido tenga a dónde ir. | **gio** — asignar dueño |
