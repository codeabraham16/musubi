# Diseño — Dominios ajenos no se juzgan

## Dónde va

`internal/memory/conflicts.go`, en el loop de `DetectRelations` — el **único** lugar donde nacen
todas las relaciones. Ahí ya viven dos guardas estructurales, y la nueva se suma como tercera capa:

```go
if c.projectID != src.projectID { continue }   // aislamiento por tenant
if complementaryPair(src, c)    { continue }   // par histórico (#203)
if dominiosAjenos(src, c)       { continue }   // ESTA
```

Ponerla ahí y no en el scoring es lo que la hace **imposible de saltear**: no depende de que quien
escriba el próximo camino de detección se acuerde.

## La función

```go
func dominioDe(topicKey string) string   // primer segmento, hasta la primera '/'
func dominiosAjenos(a, b obsRow) bool    // dominios distintos y ninguno es git-commit
```

Decide por el `topic_key` y **no mira el contenido**, igual que `complementaryPair`. Eso no es una
simplificación: el parecido entre dos auditorías de dominios distintos es alto y *correcto* —misma
estructura, mismo vocabulario de método— y mirar el texto sólo llevaría a la conclusión equivocada.
El texto es justamente lo que engaña acá.

## Por qué el primer segmento y no el `topic_key` entero

Porque la convención del proyecto ya es jerárquica: `gio/auditoria-terminales`,
`gio/donde-trabaja-realmente-gio`, `gio/perfil-operativo`. Dos notas de `gio/*` **sí** pueden
contradecirse —y de hecho la contradicción de tres bandas sobre el directorio de trabajo de gio fue
exactamente eso—, así que cortar por `topic_key` completo mataría señal real. El primer segmento es
el nivel donde «hablan de lo mismo» todavía significa algo.

## La excepción: los registros históricos

Un **registro histórico** —un commit, o un contrato SDD— no es un dominio temático: es lo que pasó
o lo que se acordó. Por eso puede volver obsoleta una nota de cualquier tema, y por eso queda
exento de la guarda. El predicado ya existía: `historicalRecord(topicKey)`.

El código ya reconocía esta asimetría: `complementaryPair` documenta que un commit «feat: migrar de
X a Y» SÍ puede envejecer una nota que decía «usamos X».

### La mitad que encontró un test, no yo

La primera versión eximía **sólo** a `git-commit`, porque eso era lo único que el dato exigía: la
única señal cross-dominio de toda la historia es `git-commit` × `bugs`. Con esa versión, la guarda
evitaba 163 relaciones (33 %) con cero señal perdida — un número mejor.

Y rompía `TestSoloLasCreenciasSeReemplazan/contrato -> nota`, que sella justamente que una creencia
SÍ se puede reemplazar. Su mensaje: *«el destino es una CREENCIA y SÍ se puede reemplazar: la guarda
se pasó de ancha, es un martillo»*.

La red del PR #203 hizo exactamente lo que se construyó para hacer. La lección importa más que el
número: **el dato dijo que ese caso nunca ocurrió, y el invariante dice que debe seguir siendo
posible.** No son lo mismo, y cuando chocan gana el invariante — un caso que no ocurrió en 494
relaciones puede ocurrir mañana, y el costo de permitirlo son 35 relaciones más de cola.

## Simetría

`dominiosAjenos` es simétrica, a diferencia de `complementaryPair`, que mira sólo el destino
**a propósito** (su pregunta es «¿se puede tachar esto?», que sí tiene lado). Acá la pregunta es
«¿hablan del mismo tema?», que no lo tiene: cuál de las dos se guardó última es un accidente del
orden de escritura.

## Qué se midió antes de escribir una línea

Sobre las 494 relaciones de la memoria de dogfood:

| | |
|---|---|
| Relaciones totales | 494 |
| Señal total (`supersedes` + `conflicts_with`) | 45 |
| Señal del mismo dominio | 44 |
| Señal cruzando dominios | 1 (`git-commit` × `bugs`) |
| Relaciones que la guarda evitaría crear | **128 (26 %)** |
| **Señal perdida** | **0** |
| De las 36 pendientes | 30 evitadas, 6 quedan |

La medición se corrió con la **misma función** que corre en producción, no con una copia de su
lógica en el test — una copia habría podido divergir y dar un número que no describe lo que se
mergea.

## Riesgos, dichos de frente

- **La guarda depende de la disciplina de los `topic_key`.** Si alguien guarda todo bajo `notas/`,
  el dominio deja de discriminar y la guarda no filtra nada. No rompe: degrada al comportamiento
  actual.
- **Al revés también:** si dos notas del MISMO tema se guardan bajo dominios distintos —
  `cognicion/x` y `roadmap/x`— su contradicción real deja de detectarse. Es el caso que la medición
  no encontró ni una vez en 494 relaciones, pero es el modo de falla honesto de esta decisión, y por
  eso está escrito acá y no sólo en el commit.
- **No arregla el ruido del mismo dominio.** De las 36 pendientes, 5 quedan. Bajar ese también
  exigiría mirar contenido, que es otro problema y otra fase.
