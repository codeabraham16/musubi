# SDD design — renaissance-f1f2-precedencia-y-material

## Archivos

| Archivo | Qué cambia |
|---|---|
| `internal/mcp/methods_design.go` | la struct del brief se reordena y gana `precedence`, `material_note`, `method[]`, `truncated`; `designMethod()` → `designMethodCards()`; nuevo presupuesto; `designEmitWeb` y `designEmitPainter` sin marca |
| `internal/mcp/banco_diseno.go` | `bloquesDe` y `bloquesDeInstruccion` aprenden los campos nuevos |
| `internal/mcp/methods_design_precedencia_test.go` | **nuevo** — I-PRE1, I-PRE2, I-PRE3, I-PRE4, I-MAT1 |
| `internal/mcp/methods_design_ataque_test.go` | A1 y A3 pasan de afirmar la vulnerabilidad a defender el arreglo |
| `internal/mcp/testdata/banco-umbrales.json` | umbrales apretados |

## Las tres piezas

**1 · La frontera código/acervo.** `principles` deja de ser un buffer donde se concatenaban las
tarjetas y pasa a ser el núcleo estático del código. El método del acervo viaja en `method[]`, cada
ítem con `topic` y `fuente`. La lista de bloques que el agente lee como órdenes —`role`,
`principles`, `precedence`, `material_note`, `emit`, `instructions`— queda **enumerada explícitamente**
en `bloquesDeInstruccion`, junto a la struct, para que se mantenga a mano y no se deduzca.

La defensa es **estructural, no un filtro**: `sanearMaterial` limpia sólo caracteres de control.
Filtrar corchetes angulares habría roto el método real, que cita `<button>` y `<div role="button">`
como ejemplos — y un filtro siempre se puede rodear.

**2 · El orden.** El JSON sale `ask · target · precedence · material_note · role · principles ·
brand · corpus · method · emit · instructions · truncated`. Contrato primero, regla específica
después, material general al final.

**3 · El presupuesto.** `aplicarPresupuesto` es un bucle: declara el recorte, mide, y si no entra
cede un ítem. Cede en el mismo orden que la precedencia declara — método, corpus, y la marca al
final, recortada por lo que exactamente sobra y con un aviso ruidoso, porque un doc de marca lleva
sus prohibiciones al final.

Dos cosas que la primera versión hacía mal y el banco agarró:

- **Trimeaba y recién después declaraba**, así que el aviso de recorte empujaba el brief por encima
  del tope: 2.628 y 2.656 tokens contra un presupuesto de 2.600. Ahora se declara en cada vuelta y se
  mide con la declaración puesta.
- **Sólo tocaba la marca si pasaba `designBrandBudget`**, así que un brief con marca mediana y sin
  material que ceder se quedaba por encima del tope. Ahora recorta exactamente lo que sobra.

## Lo que se midió

| Métrica | F0 | F1+F2 |
|---|---|---|
| M4 tokens · p50 | 6.419 | **2.457** |
| M4 tokens · máximo | 7.268 | **2.598** (tope duro 2.600) |
| M5 fracción variable | 0,047 | **0,146** |
| M6 acervo→instrucción | 0,00 | **1,00** |

⚠ **Corrección dentro del propio PR:** la primera lectura de M5 dio 0,22, pero `bloquesDe` todavía no
conocía `precedence`, `material_note` ni `method`, así que el bloque de método quedaba fuera de la
cuenta. Con la definición completa da **0,146**. Sigue siendo 3,1× la línea base, pero el 0,22 era un
artefacto de la medición y no una mejora.

## Riesgos

1. **El presupuesto recorta método que hacía falta** → hoy cede por importancia, que es lo que ya
   hacía; F4 lo reemplaza por selección según el pedido. El recorte se declara, así que el caller ve
   qué le falta en vez de recibir un brief mutilado con cara de completo.
2. **Un consumidor parsea `principles` esperando el método del acervo** → se buscó: no hay
   consumidores del brief fuera del paquete. El campo sigue existiendo y sigue trayendo método
   (el núcleo), así que un lector laxo no se rompe.
3. **La nota anti-inyección no es una garantía** — un modelo puede desobedecerla. Por eso la defensa
   principal es que el material **no entra** al bloque de instrucciones; la nota es el segundo anillo.
