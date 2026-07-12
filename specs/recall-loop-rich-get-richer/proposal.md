# Proposal — recall-loop-rich-get-richer

## Intención

Cerrar **N4** (la auditoría lo marcó *"estructural, missed"*): **el ranker del recall se alimenta de su propia salida.**

Cada recall llama a `bumpAccess`, que sobre lo que **acaba de devolver** hace:

```sql
SET last_accessed = CURRENT_TIMESTAMP, access_count = access_count + 1
```

Y esos **mismos dos campos** alimentan dos de los términos RRF del score:

| Señal del ranking | De dónde sale | ¿La escribe el ranker? |
|---|---|---|
| `effectiveRecency` → término de **recencia** | `last_accessed` (si existe) | ❌ **sí** |
| `freqRank` → término de **frecuencia** | `access_count` | ❌ **sí**, y **nunca decae** |

Es un lazo cerrado con realimentación positiva: **lo que el ranker muestra se vuelve más mostrable.** Lo recuperado sube de rango ⇒ se vuelve a recuperar ⇒ sube más. La memoria nueva o poco usada no puede entrar.

## Medido en la base real (409 observaciones activas)

| | |
|---|---|
| El **10% más accedido** concentra | **62%** de TODOS los accesos |
| **Nunca** accedidas | **69%** (282 de 409) |
| Ya no rankean por su fecha de creación | **31%** (tienen `last_accessed`) |

Ese último número es el corazón: `effectiveRecency` devuelve `last_accessed` **si existe**. O sea, **una memoria de hace 6 meses que el ranker mostró hace 5 minutos es "más reciente" que una escrita ayer.** La novedad genuina pierde contra la circularidad.

## El criterio: señales EXÓGENAS vs ENDÓGENAS

La distinción que ordena todo el cambio:

- **Exógena** — el ranker **no la puede cambiar**: `created_at` (cuándo se escribió), el texto, el vector. Un prior legítimo.
- **Endógena** — **la escribe el propio ranker**: `last_accessed`, `access_count`. Usarla para rankear es circular **por definición**.

**Un lazo endógeno sin decaimiento es un acumulador desbocado.** Con decaimiento, es un integrador con fuga: se auto-limita. La cura no es prohibir el uso del acceso, es que **no pueda acumularse para siempre**.

## Lo que NO se toca: el olvido

`decay.go` **también** usa el acceso, y ahí está **bien**: el refuerzo de Ebbinghaus (B3) alarga la vida media de lo que usás, para que no se olvide. Eso es **deliberado y correcto** — y no es circular, porque el olvido no es el que elige qué mostrar.

**Dos usos del mismo dato, uno legítimo y otro circular.** Este cambio toca **sólo el ranking**; la retención queda intacta.

## Alcance

- **N4.a — Recencia = NOVEDAD.** El término de recencia del RRF pasa a usar **`created_at`** (exógeno), no `last_accessed`. Corta el eslabón más directo: hoy, cualquier cosa recién mostrada salta a rango de recencia 0.
- **N4.b — Frecuencia = TASA, no total.** El término de frecuencia pasa de `access_count` (total, monótono, nunca baja) a una **tasa de uso** `access_count / (edad + 1)`. Para seguir arriba hay que ser útil **últimamente**, no haberlo sido **alguna vez**. Convierte el acumulador desbocado en uno **con fuga**.
  - Ojo: como `freqRank` es un **rango**, cualquier transformación **monótona** del contador (p. ej. `log`) da **exactamente el mismo rango**. Amortiguar el valor no sirve: hay que cambiar el **orden**. Por eso la tasa (que sí lo cambia) y no un `log`.
- `bumpAccess` **no se toca**: sigue alimentando la retención (Ebbinghaus).

## Fuera de alcance (explícito)

- No se toca `decay.go`, ni la retención, ni el refuerzo de Ebbinghaus.
- No se elimina `bumpAccess` ni las columnas: siguen siendo datos válidos (y los usa el olvido).
- No se tocan los otros términos RRF (léxico, vector, grafo, co-ocurrencia, importancia).

## Estrategia de rollback

Cambio acotado a dos comparadores dentro de `scoreCandidates` (funciones puras). Revertir el PR restaura el comportamiento exacto. Sin migración de esquema, sin cambio de datos, sin config nueva.

## Riesgos

- **Se pierde el prior de "locality de sesión"** (lo que acabo de mirar sigue siendo relevante). Es intencional: ese prior es **exactamente** el lazo circular. La tasa de uso conserva la parte legítima ("esto se usa seguido **últimamente**") sin permitir el lock-in.
- **Cambia el ranking del recall** para las observaciones con acceso previo (31% de la base). Es el objetivo, pero hay que verificar que el golden por propiedades de `recalleval` (monotonía, MRR>0) siga verde.
- La tasa necesita un **suavizado** en el denominador (`edad + 1`): sin él, una observación recién creada (edad ≈ 0) con 1 acceso daría una tasa que explota. Hay que testear ese borde.
