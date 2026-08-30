# SDD spec — renaissance-f4-seleccion

Fase 4 de [renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md).

## Los tres defectos, medidos

1. **El método no mira el pedido.** Verificado contra el central: el hash del bloque de método es
   IDÉNTICO para un ERP de escritorio, un juego móvil, una landing y un gráfico de series. En una
   grilla densa de escritorio el brief predica *EL PULGAR MANDA*, *EL DEDO NO ES UN CURSOR*, *EL
   GESTO SE ANUNCIA* y *EL RECTÁNGULO MIENTE* — 38 % del método, irrelevante, siempre.
2. **La profundidad del acervo es inalcanzable.** 1.438 micro-tarjetas de ~61 tokens contra 268
   artículos completos de ~3.057. En un pool de 58 candidatos salieron **58 tarjetas y 0 artículos**.
   Todo lo que tiene desarrollo real está enterrado.
3. **El top-6 son seis variaciones del mismo tema.** Para «tabla densa de lotes con filtros y
   alertas» devolvió: colapsar filas · filtros post-búsqueda · filtros drill-down · cortina de dos
   niveles · zebra stripes · alto flexible. Cuatro son la misma idea.

## El cambio

**Una sola búsqueda, dos salidas.** El pool ya trae tarjetas de método (se agranda a propósito para
que no compitan con los patrones). Hoy se descartan con `excludeTopicPrefix`; ahora se PARTE:

- lo que empieza con `design-method/` → el bloque `method`, ordenado por relevancia al pedido
- el resto → el corpus de patrones

El vector de la consulta ya se calcula para el corpus, así que seleccionar el método **no cuesta una
llamada más** al embebedor. Sigue siendo model-free y sigue siendo un solo viaje.

**El núcleo no hace falta protegerlo con una regla nueva**: F1+F2 ya lo dejó en `principles`, que es
código y viaja siempre. Por eso el método del acervo se puede elegir 100 % por relevancia sin riesgo
de quedarse sin criterio.

## Invariantes

### I-SEL1 · el método depende del pedido
Dos pedidos opuestos (un ERP de escritorio y un juego para teléfono) reciben bloques de método
DISTINTOS.

**Sabotaje:** volver a ordenar el método por importancia en vez de por relevancia ⇒ los dos bloques
vuelven a ser idénticos y el test falla. Es el test que hoy afirma lo contrario
(`TestAtaqueElMetodoIgnoraElPedido`) y que esta fase tiene que dar vuelta.

### I-SEL2 · el núcleo está siempre, gane o no por relevancia
`principles` (el núcleo estático del código) viaja aunque el acervo no aporte una sola tarjeta y
aunque el pedido sea de otro planeta.

**Sabotaje:** un pedido cuyo método relevante es cero ⇒ `principles` sigue completo.

### I-SEL3 · el top-k no colapsa en variaciones de lo mismo
La selección penaliza a un candidato por parecerse a los ya elegidos (MMR con solape léxico, sin
LLM). Seis resultados no pueden ser seis maneras de decir lo mismo.

**Sabotaje:** sembrar ocho tarjetas casi idénticas y una distinta ⇒ la distinta tiene que entrar al
top-k aunque su similitud cruda sea menor. Con λ=0 (MMR apagado) el test tiene que fallar.

### I-SEL4 · los artículos crudos tienen lugar reservado
Si hay artículos `ingested/*` en el pool, al menos uno entra al corpus servido. Sin reserva, 1.438
tarjetas cortas los desplazan siempre — es lo que se midió.

**Sabotaje:** un acervo con muchas tarjetas y pocos artículos ⇒ sin reserva salen 0 artículos; con
reserva sale al menos 1.

## Métricas que este PR tiene que mover

| Métrica | Antes | Esperado |
|---|---|---|
| M3 precisión temática @6 (sonda) | 0,22 | sube |
| M5 fracción variable (banco) | 0,146 | sube — el método deja de ser constante |
| M8 ids distintos servidos (sonda) | 190 | sube — diversidad + artículos |
| M4 tokens | 2.457 | no empeora (el presupuesto sigue mandando) |

M1 (paráfrasis) es de F5. Si sube acá, bien; no es el objetivo.

## Fuera de alcance

Normalizar la consulta larga (F5). Y por el camino léxico (FTS) no hay similitud que ordenar: ahí el
método sigue saliendo por importancia, igual que hoy — declarado, no escondido.
