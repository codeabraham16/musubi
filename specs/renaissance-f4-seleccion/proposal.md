# F4 · Selección

Fase 4 de [renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md). Spec en `spec.md`.

## En una línea

El motor traía material y lo entregaba tal como salía del ranking. Esta fase le agrega **criterio**:
el método sigue al pedido, los artículos con desarrollo real dejan de ser inalcanzables, y el top-k
deja de ser k maneras de decir lo mismo.

## Lo que NO se puede afirmar todavía

Las invariantes están probadas y la suite está verde, pero **la magnitud de la mejora no está
medida**. M3 (precisión temática) y M8 (cobertura) dependen del embebedor real y del acervo real de
1.736 entradas; el banco estructural corre sobre FTS, donde la selección por relevancia
deliberadamente no se aplica.

Eso se mide con la sonda **después de desplegar**, y es exactamente para lo que existe. Hasta
entonces lo honesto es decir que el mecanismo está y que su efecto está sin cuantificar.
