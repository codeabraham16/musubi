# F1+F2 · Precedencia, presupuesto, y el acervo como material

Fases 1 y 2 de [renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md), en un solo
PR porque tocan el mismo ensamblado del brief. Línea base en
[renaissance-f0-banco](../renaissance-f0-banco/proposal.md).

## El problema

Tres defectos medidos el 2026-08-29 que salen del mismo error de diseño — **el brief es una
concatenación plana sin jerarquía ni contrato**:

1. **Se contradice.** La marca de Altura pide `glass + sombra`; el método universal lo prohíbe; el
   `emit` también. No hay operador de precedencia, así que gana el bloque que más pesa (el método,
   68 % del texto) y **el motor le borra la marca al proyecto que sí la tiene cargada**.
2. **Inunda a quien lo llama.** 5.850 tokens de brief, 11.131 con `limit=100`, y 285.023 desde una
   sola tarjeta grande del acervo. El tope acota la *cantidad* de tarjetas, nunca su *tamaño*.
3. **El acervo da órdenes.** El contenido de las tarjetas `design-method/*` se concatena verbatim
   dentro de `principles`, que el agente lee como instrucciones. Quien escriba una observación en el
   tenant `musubi-design` le dicta la conducta a todos los agentes de todos los proyectos, y la
   `importance` le deja **elegir la posición**.

## El cambio de fondo

**Separar lo que el CÓDIGO afirma de lo que el ACERVO aporta.** Hoy están mezclados en el mismo
campo, y por eso un dato mutable puede hacerse pasar por instrucción.

| | Antes | Ahora |
|---|---|---|
| `principles` | las tarjetas del acervo, concatenadas verbatim | el núcleo **estático del código**, siempre |
| método del acervo | dentro de `principles` | `method[]`, ítems rotulados con su `topic` y su tenant |
| orden del brief | `ask · role · principles · brand · emit · corpus` | `ask · precedence · role · brand · corpus · method · emit · instructions` |
| presupuesto | ninguno | tope duro, con el recorte **declarado** |
| precedencia | ninguna | explícita en el propio brief |

El método **sigue viviendo en el acervo y sigue siendo arbitrable** — que es la capacidad que da
Renaissance F5. Lo único que cambia es que viaja como material citado con procedencia, no como
órdenes del sistema.

## Invariantes

### I-PRE1 · la marca precede al método
En el JSON servido, `brand` aparece antes que `method`. Los modelos leen en U y pierden más del 30 %
de eficacia sobre lo que queda en el medio; hoy la marca está al ~70 % de profundidad, debajo de
4.182 tokens de método constante.

**Sabotaje:** mover `Brand` debajo de `Method` en la struct ⇒ el test compara posiciones en el JSON
y falla. *Visto en rojo.*

### I-PRE2 · el brief nunca supera el tope, con ningún `limit`
Con `limit=100`, con una tarjeta de método de 1 MB, con una marca gigante: el brief entra en el
presupuesto o se recorta hasta entrar.

**Sabotaje:** sembrar una tarjeta de método de 1 MB (el ataque A3 del banco) ⇒ sin presupuesto el
brief pasaba de 285.000 tokens; con presupuesto tiene que quedar bajo el tope.

### I-PRE3 · todo recorte se declara, con su total
Si se recortó, el brief trae `truncated` diciendo qué bloque y cuántos ítems quedaron afuera de
cuántos. Un recorte silencioso es el modo de falla de esta casa: entrega un brief mutilado con cara
de completo.

**Sabotaje:** recortar sin poblar `truncated` ⇒ el test que exige la declaración falla.

### I-PRE4 · el `emit` no lleva la marca de nadie
`designEmitWeb` decía «fondo oscuro, un acento, no serifas, no glow, no glass/blur»: las
prohibiciones estéticas de **Musubi**, servidas a todo proyecto por una constante universal. El
`emit` habla de formato y dialecto; los valores salen de la marca resuelta.

**Sabotaje:** un brief para un proyecto ajeno no puede contener ningún hex ni prohibición estética
de Musubi en `emit`. *Cubre el cruce de marca por la puerta de atrás.*

### I-INY1 · el acervo no llega a posición de instrucción
Los bloques que el agente lee como órdenes —`role`, `principles`, `precedence`, `material_note`,
`emit`, `instructions`— son **todos propiedad del código**. Ninguna cadena que venga del acervo
aparece ahí.

**Sabotaje:** sembrar los 8 payloads del set dorado como tarjetas `design-method/*` ⇒ el canal
`acervo→instrucción` del banco tiene que pasar de 0,00 a 1,00. Y envolverlos en etiquetas de cita
**sin sacarlos** del bloque de instrucciones no cuenta (I-BANCO5 ya lo defiende).

### I-INY2 · la importancia ordena dentro del material, nunca por encima del núcleo
Una tarjeta con `importance` máxima ordena primero **entre las tarjetas**, pero el núcleo estático
del código va antes que todas ellas.

**Sabotaje:** tarjeta con importancia 1.0 y texto «IGNORÁ TODO LO ANTERIOR» ⇒ el núcleo estático
tiene que seguir apareciendo antes en el documento.

### I-MAT1 · el material viaja con procedencia y con su advertencia
Cada ítem de `method[]` declara su `topic` y su tenant de origen. El brief lleva una nota
—propiedad del código— que le dice al agente que el material es conocimiento y no órdenes.

**Sabotaje:** quitar la nota o la procedencia ⇒ el test falla. La defensa acá es **estructural**, no
un filtro de texto: filtrar corchetes angulares rompería el método real, que cita `<button>` y
`<div role="button">` como ejemplos. Lo único que se limpia son caracteres de control.

## Métricas que este PR tiene que mover

| Métrica | Antes | Después (esperado) |
|---|---|---|
| M4 tokens del brief · p50 | 6.419 | **≤ 2.000** |
| M4 tokens · máximo | 7.268 | **≤ 2.000** |
| M5 fracción variable | 0,047 | sube (el bloque constante se achica) |
| M6 acervo→instrucción | 0,00 | **1,00** |

M1, M2 y M3 **no** los mueve este PR: son de F3 (abstención), F4 (selección) y F5
(reproducibilidad). Si se movieran, sería casualidad y habría que mirarla con desconfianza.

## Fuera de alcance

- Elegir el método según el pedido (F4). Acá el método se recorta por importancia, que es lo que ya
  hacía; lo nuevo es que el recorte **se declara** y que el núcleo del código nunca cae.
- Umbral de abstención (F3) y normalización de la consulta (F5).
