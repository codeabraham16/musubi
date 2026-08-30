# SDD design — renaissance-f5-reproducibilidad

## Causa A · el ranking fingía precisión

En una consulta real el pool se reparte entre 0,643 y 0,515: decenas de candidatos separados por
milésimas. Una reformulación mueve el vector lo justo para dar vuelta esos empates y rehacer el top-6
entero.

`estabilizarOrden` cuantiza la similitud a `designResolucionSim` (0,005) y, dentro de cada escalón,
ordena por id con un sort estable. La afirmación que lo sostiene: **entre dos candidatos que difieren
por menos que el ruido de una paráfrasis, el motor no sabe cuál es mejor.** Cuando no se sabe,
contestar siempre lo mismo es estrictamente mejor que contestar cualquier cosa — el caller puede
confiar en lo que recibe, y una diferencia en el brief pasa a significar una diferencia en el pedido.

0,005 preserva las diferencias reales: sobre el spread medido de 0,13 dentro de una consulta quedan
~26 escalones.

## Causa B · el relleno arrastraba el vector

**Un tope de caracteres no alcanzaba, y el test lo demostró.** Con un pedido de 50 caracteres y un
tope de 600 entran 550 de relleno: el vector sigue arrastrado y el corpus cambia igual. Fue el primer
diseño de esta fase y hubo que tirarlo.

`normalizarConsulta` corta por **oraciones** (`designConsultaFrases` = 2) y deja el tope de caracteres
como segunda red para una oración kilométrica. Un pedido de diseño cabe en una o dos oraciones
—«tabla densa de inventario con lotes, filtros y alertas de stock bajo»— y lo que viene después es
contexto para el agente, no para la búsqueda.

Es una heurística y por eso el recorte **se declara**: `query_normalized` dice de cuántos caracteres a
cuántos, y el caller ve exactamente con qué se buscó.

## Un hallazgo del propio test: el eco se comía el corpus

`ask` devolvía el pedido crudo. Con un pedido largo eso consumía presupuesto y **le sacaba lugares al
material** — el brief con contexto traía menos patrones que el mismo pedido sin contexto. Ahora `ask`
lleva la consulta normalizada: quien llamó ya tiene su prompt entero (lo escribió), devolvérselo
completo era presupuesto gastado en nada. De paso `ask` es el primer campo del brief, la posición de
máxima atención, así que cuanto menos texto ajeno viva ahí, mejor.

## Lo que quedó fuera

- **Reescribir la consulta** (expansión, sinónimos, sub-consultas) necesita un modelo o un léxico, y
  el camino caliente es model-free.
- **Fusión de rankings semántico + léxico (RRF)** se evaluó y se descartó: choca con el piso de F3,
  que compara similitudes, y un puntaje de fusión no es una similitud. Meterlo acá habría vuelto
  ambiguo un invariante que recién se estabilizó.
- **La atomización del acervo** —1.438 tarjetas casi indistinguibles— es la causa de fondo de que la
  banda de similitud sea tan angosta. Eso lo ataca el foso (F6/F7), no esta fase.
