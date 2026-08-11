# Catálogo de modos de falla

Formas reales en que este sistema falla, con su severidad y su mitigación. Se escribe para
consultarlo cuando algo se comporta raro, y para que una falla nueva tenga dónde ir.

Cada entrada sale de un caso **medido en este proyecto**, no de un catálogo genérico. La fecha es
cuándo se observó. Si una entrada deja de ser cierta, se corrige acá — no se agrega una segunda que
la contradiga.

> La forma (severidad S1/S2/S3, síntoma → causa → mitigación) está tomada de
> [loop-engineering](https://github.com/cobusgreyling/loop-engineering). El contenido es nuestro.

## Severidad

| | Significa |
|---|---|
| **S1 — Molesto** | Tiempo o tokens perdidos, sin daño a nadie |
| **S2 — Dañino** | Código malo mergeado, decisión tomada sobre un dato falso, fatiga de alarmas |
| **S3 — Crítico** | Seguridad, pérdida de datos, incidente en producción |

---

## Medir bien en el sitio equivocado

**Síntoma.** Una medición sale impecable, con aritmética correcta, y responde una pregunta que
nadie hizo. Nada falla; el número se publica.

**Severidad.** S2 — se decide sobre él.

**Caso (2026-08-10).** El banco del juez de pertinencia comparaba `lexical` contra `lexical+juez`
sin embedder, pero el cerebro central hace búsqueda híbrida desde el 2026-07-28. El delta le
acreditaba al juez todo lo que ya aportaba el vector: +142 %. Medido contra la base real, el número
honesto era +114 % — y venía con un costo de 8,5 s por consulta que el primero nunca mostró.

**Causas.**
- La base de comparación se eligió cuando era la de producción, y nadie la revisó cuando cambió.
- Nada en la prueba ataba el brazo de control a lo que corre de verdad.

**Mitigaciones.**
- Que el invariante fije la BASE, no sólo el resultado (`TestElJuezSeMideSobreLaBaseDeProduccion`).
- Ante la falta del insumo que hace válida la medición, **saltearla con motivo** en vez de degradar
  en silencio a una configuración que nadie corre.
- Publicar el costo junto al beneficio. Una mejora sin su precio se usa donde no corresponde.

---

## Capacidad desplegada que nadie puede invocar

**Síntoma.** Una mejora se implementa, se mergea y se despliega — y la métrica no se mueve.

**Severidad.** S1 → S2 (se da por resuelto algo que sigue costando).

**Caso (2026-08-10).** El modo rápido de `musubi_doctor` (`deep:false`) llegó al cerebro central y
el sondeo siguió costando 746 ms. El parámetro existía en el handler pero **no figuraba en el
`inputSchema` de `tools/list`**: ningún cliente MCP podía descubrirlo.

**Causas.**
- El default retrocompatible (ausente ⇒ comportamiento viejo) hace que la omisión no falle nunca.
- Se probó el comportamiento y no el catálogo. Una prueba de comportamiento pasa igual, porque el
  handler acepta el parámetro aunque no se anuncie.

**Mitigaciones.**
- Toda capacidad nueva necesita DOS pruebas: que **se anuncie** y que **haga algo**.
- Después de un deploy, medir el efecto. Desplegar no es lo mismo que que sirva.

---

## La ventana rodante que miente

**Síntoma.** Se aplica un arreglo y la métrica no baja, así que se concluye que no llegó — y se
manda a arreglar algo que ya está arreglado.

**Severidad.** S1 (trabajo duplicado) → S2 si dispara un cambio innecesario.

**Caso (2026-08-10, dos veces el mismo día).** La media de `musubi_doctor` en
`musubi_tool_usage days=1` seguía en 746 ms horas después del arreglo. La ventana de 24 h todavía
contenía sobre todo llamadas de ANTES. Ya rotada: **96 llamadas contra 1.550, un 94 % menos**. Lo
mismo había pasado con `save_observation`.

**Mitigación.** La media de una ventana rodante no sirve para juzgar un cambio reciente. O se espera
a que rote, o se deriva la latencia de las llamadas nuevas por diferencia de totales:
`(n+1)·media_nueva − n·media_vieja`.

---

## Sabotaje vacuo

**Síntoma.** Se rompe el código a propósito para comprobar que la prueba se ponga en rojo, sigue en
verde, y se concluye que la prueba es mala.

**Severidad.** S1, pero envenena el juicio: lleva a reescribir pruebas que estaban bien.

**Causas.**
- La mutación no altera el comportamiento que la prueba observa (es vacua).
- **El archivo no cambió.** Una expresión regular multilínea que no matcheó deja todo intacto y el
  verde es simplemente el de siempre.
- El fixture dispara otra guarda antes de llegar a la que se quería probar.

**Mitigación.** Antes de concluir nada, verificar que el archivo cambió de verdad. Ante un sabotaje
que no rompe, la primera hipótesis es la mutación, no la prueba.

---

## Estado rancio en la memoria

**Síntoma.** El recall devuelve notas que describen un estado que ya no existe, y alguien actúa
sobre ellas.

**Severidad.** S1 → S2 (se decide sobre fantasmas).

**Caso (2026-08-11).** 368 relaciones pendientes de veredicto en el cerebro central, repartidas
entre 150 notas, con confianza mediana 0,71. Backlog anterior al arreglo que dejó de emparejar
notas sin relación real.

**Mitigaciones.**
- Anclar las notas de estado a un símbolo con `origin_paths`: el recall las marca rancias cuando el
  código se mueve.
- Resolver la contradicción con `musubi_judge` en vez de guardar una nota nueva que la contradiga —
  el recall no distingue solo cuál gana.
- Cuando `supersedes` oculta una nota, mirar qué otras la citaban: quedan huérfanas.

---

## Un aviso que no se anuncia con su precio

**Síntoma.** Una capacidad cara se usa dentro de un bucle porque nada decía cuánto costaba.

**Severidad.** S2 (gasto y latencia en el camino caliente).

**Caso (2026-08-10).** El juez de pertinencia mejora el primer resultado un 114 % y cuesta 8,5 s por
consulta. La única perilla era global: encenderla lo metía en cada recall, incluido el sondeo.

**Mitigación.** Cuando una capacidad tiene un costo que el llamador debe poder rechazar, la decisión
baja a la llamada (tri-estado: ausente ⇒ manda la config), y la descripción del parámetro dice el
precio.

---

## Cómo agregar una entrada

Una falla entra acá cuando **se observó**, no cuando se imagina. Formato: síntoma → severidad →
caso con fecha → causas → mitigaciones. Si no hay un caso real, no es una entrada de este catálogo:
es un anti-patrón, y va en otro lado.
