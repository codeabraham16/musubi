# SDD spec — renaissance-f5-reproducibilidad

Fase 5 de [renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md).

## Los dos hechos medidos

1. **Cinco maneras de pedir lo mismo → solape Jaccard 0,09** sobre los 16 pedidos del set dorado, con
   tres pedidos en **0,00**: dos formas de pedir lo mismo sin un solo patrón en común. No hay «la
   respuesta del motor»; hay una lotería por redacción.
2. **El motor castiga la especificidad.** Con **256 bytes** de contexto extra se pierden dos tercios
   del corpus; con 10 KB el solape cae a **0,00**. Cuanto más detalle da el usuario, peor material
   recibe — exactamente al revés de lo que debería.

## Las dos causas, y por qué se atacan separadas

**Causa A · el ranking finge una precisión que no tiene.** En una consulta real el pool se reparte
entre 0,643 y 0,515: dentro de esa banda hay decenas de candidatos separados por milésimas. Una
reformulación mueve el vector lo suficiente como para dar vuelta esos empates, y el top-6 se rehace
entero. El orden entre candidatos que difieren por menos que el ruido de una paráfrasis **ya era
arbitrario**; lo que se arregla no es hacerlo correcto sino hacerlo **consistente**.

**Causa B · el contexto de relleno arrastra el vector.** Un pedido de diseño ocupa unos cientos de
caracteres; lo que viene después suele ser contexto para el agente, no para la búsqueda. Ese texto
promedia el vector hasta sacarlo del vecindario del pedido.

## Contrato

- `designResolucionSim`: por debajo de esa diferencia dos candidatos se consideran **empatados**, y el
  empate se rompe por un criterio estable (el id), no por el azar del ordenamiento.
- `designConsultaMax`: tope de caracteres que viajan al embebedor.
- El brief declara `query_normalized: {chars_originales, chars_usados}` cuando la consulta se acotó.
  Como todo recorte en este motor: **se declara**.

## Invariantes

### I-REP1 · el ruido no cambia el material
Una consulta con contexto irrelevante pegado atrás devuelve **el mismo corpus** que la consulta
limpia.

**Sabotaje:** subir `designConsultaMax` por encima del largo del ruido ⇒ el ruido vuelve a entrar al
embebedor y el corpus cambia. El test compara contra el caso normalizado Y contra el no normalizado
para que no pase por casualidad.

### I-REP2 · un empate se resuelve igual siempre
Dos candidatos separados por menos que `designResolucionSim` salen en el mismo orden en toda corrida,
sin importar cuál llegó primero del motor de búsqueda.

**Sabotaje:** invertir el orden de llegada de dos candidatos empatados ⇒ el resultado no puede
cambiar. Con la cuantización apagada, cambia.

### I-REP3 · el recorte de la consulta se declara
Si la consulta se acortó, el brief lo dice con el largo original y el usado. Un recorte mudo hace
creer que se buscó lo que se escribió.

**Sabotaje:** acortar sin poblar el campo ⇒ el test falla.

### I-REP4 · una consulta normal no se toca
Por debajo del tope, la consulta viaja tal cual y el campo de normalización no aparece.

## Métricas

| Métrica | Antes | Esperado |
|---|---|---|
| M1 estabilidad de paráfrasis (sonda) | 0,09 | sube |
| Solape con +4 KB de ruido (sonda/ataque) | 0,33 | **1,00** |
| M3, M4, M6 | — | no empeoran |

⚠ **La magnitud de M1 no se puede medir acá.** Depende del embebedor y del acervo reales. Los
invariantes prueban el MECANISMO con un embebedor determinista; el número sale de la sonda después de
desplegar. Decir otra cosa sería medir al embebedor falso.

## Fuera de alcance

Reescribir la consulta (expansión, sinónimos, sub-consultas): eso necesita un modelo o un léxico, y el
camino caliente es model-free. Y la atomización del acervo —1.438 tarjetas casi indistinguibles— es la
causa de fondo de que la banda de similitud sea tan angosta; eso lo ataca el foso (F6/F7), no esta fase.
