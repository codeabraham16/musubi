# F1+F2 · Precedencia, presupuesto, y el acervo como material

Fases 1 y 2 de [renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md), en un solo PR
porque tocan el mismo ensamblado del brief. La spec completa está en `spec.md`.

## En una línea

El brief dejaba que el acervo le diera órdenes al agente, se contradecía consigo mismo cuando la marca
de un proyecto discrepaba del método universal, y no tenía ningún tope de tamaño. Esta fase le pone
las tres cosas que le faltaban: **una regla de precedencia declarada, un presupuesto duro con el
recorte declarado, y una frontera entre lo que afirma el código y lo que aporta la memoria.**

## Qué se resuelve, con su evidencia

| Defecto medido el 2026-08-29 | Cómo se cierra |
|---|---|
| La marca de Altura pide `glass + sombra`; el método lo prohíbe; el `emit` también. Sin regla, gana el que más pesa — el método, con el 68 % del texto | `precedence` declara *lex specialis*: la marca del proyecto le gana al método universal |
| La marca viajaba al ~70 % de profundidad, la peor posición para leer | el orden pasa a `ask · precedence · material_note · role · principles · brand · corpus · method · emit` |
| `limit=100` → 11.131 tokens; una tarjeta grande → 285.023 | presupuesto duro de 2.600 tokens, tope por tarjeta, y todo recorte declarado con su total |
| `designEmitWeb` y `designEmitPainter` imponían la estética de Musubi a todo proyecto | el emit habla de formato; la paleta y las prohibiciones salen de la marca resuelta |
| El contenido del acervo se concatenaba dentro de `principles`, que el agente lee como órdenes | `principles` pasa a ser el núcleo estático del código; el acervo viaja en `method[]` con procedencia |

## Lo que NO hace

No elige el método según el pedido (F4), no tiene umbral de abstención (F3), y no normaliza la
consulta (F5). Si M1, M2 o M3 se movieran con este PR sería casualidad y habría que desconfiar.
