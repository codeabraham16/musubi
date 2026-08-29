# F5 · Reproducibilidad

Fase 5 de [renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md). Spec en `spec.md`.

## En una línea

Pedir lo mismo de dos maneras devolvía material distinto —solape 0,09, con tres pedidos en 0,00— y
agregar detalle al pedido lo empeoraba. Esta fase ataca las dos causas: **el ranking fingía una
precisión que no tiene**, y **el contexto de relleno arrastraba el vector**.

## Lo que NO se puede afirmar

**La magnitud no está medida.** Cuánto sube M1 depende del embebedor y del acervo reales; los
invariantes prueban el mecanismo con un embebedor determinista. El número sale de la sonda después de
desplegar. Afirmarlo desde acá sería medir al embebedor de prueba.
