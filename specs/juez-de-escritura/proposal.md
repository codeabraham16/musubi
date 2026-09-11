# Propuesta — La economía del juez de escritura

Último item abierto de la Ola 3. **Esta propuesta recomienda NO construir lo que su nombre dice**, y
construir otra cosa. Todo lo que sigue está medido sobre la base real el 2026-09-11, ventana
2026-07-26 → 2026-09-11 (47 días, 306 relaciones).

## La pregunta

El detector de conflictos encola pares de observaciones que podrían contradecirse, y alguien los
arbitra a mano con `musubi_judge`. La idea del «juez de escritura» es que un LLM arbitre **al
guardar**, en vez de encolar.

> ¿Vale lo que cuesta?

Hoy hay **dos caminos al motor y los dos son de lectura** (`musubi_ask` y el juez de pertinencia del
recall). Éste sería el primero de escritura, y la escritura es camino caliente.

## Lo medido

### 1 · El rendimiento de la cola es 3,3 %

De las 306 relaciones arbitradas en toda la historia:

| veredicto | n | % |
|---|---|---|
| `related` | 161 | 52,6 % |
| `compatible` | 82 | 26,8 % |
| `not_conflict` | 42 | 13,7 % |
| `scoped` | 11 | 3,6 % |
| `conflicts_with` | 5 | 1,6 % |
| `supersedes` | 5 | 1,6 % |

**10 de 306 exigieron acción.** Las otras 296 costaron atención y contestaron «no hay nada que
hacer».

### 2 · La tasa de llegada se está acelerando

| mes | detectadas | accionables |
|---|---|---|
| 2026-07 | 1 | 1 |
| 2026-08 | 71 | 0 |
| 2026-09 (11 días) | 234 | 9 |

**~21 pares por día** en septiembre, para ~0,8 accionables por día.

### 3 · Ninguna señal barata los separa

Ésta es la que decide, y salió al revés de lo esperado:

| | accionables (n=9) | no accionables (n=290) |
|---|---|---|
| `lex` mediana | 0,316 | **0,324** |
| `lex` rango | [0,300 … 0,723] | [0,232 … 0,678] |
| coseno mediana | 0,889 | 0,856 |
| coseno rango | [0,767 … 0,920] | [0,760 … **0,964**] |

Los accionables tienen la mediana **léxica más baja** que los demás, y el máximo de coseno lo tiene
el grupo *no* accionable. **Ningún umbral sobre `lex` o `cosine` puede subir la precisión sin tirar
los que importan.** (n=9 es poco; lo que el dato descarta es un umbral evidente, no toda esperanza.)

### 4 · El 75 % de la cola sale de la exención de los commits, y no es gratis sacarla

`complementaryPair` saltea el par cuando el **target** es del libro mayor —un commit no se puede
tachar—, pero `historicalRecord` está **exento del guardia de dominios** a propósito: un commit es
evidencia sobre el mundo y puede envejecer una nota de cualquier tema.

| | pares | accionables | precisión |
|---|---|---|---|
| con un commit en el par | **228** | 6 | **2,6 %** |
| nota contra nota | 78 | 4 | **5,1 %** |

El commit está del lado `source` en **228 de 228**: cero del lado `target`, que confirma que la
mitad existente de la regla funciona.

**La tentación es sacar la exención. No es una mejora, es un canje**: −74 % de trabajo, −60 % de
hallazgos (6 de los 10 salieron de ahí). Documentarlo importa porque es la decisión que alguien va a
querer tomar mirando sólo la columna de precisión.

## Lo que esto recomienda

**NO un juez en el camino de escritura.** Dos razones, y la segunda es la que manda:

1. **El 96,7 % de las llamadas no compra nada.** Es barato en dólares (21/día) y caro en todo lo
   demás.
2. **La escritura es camino caliente.** Este repo ya pagó esa lección completa: cablear algo caro al
   hook por turno lo llevó de 0,29 s a 23 s contra un techo de 10 s, y el turno se quedaba **sin
   memoria inyectada** — peor que no haber agregado nada. Un juez en `save_observation` es la misma
   forma con otro nombre.

**SÍ un drenador asíncrono**, colgado del mantenimiento que ya existe (`scheduler.go`), acotado por
el freno de gasto del motor que ya existe. La misma cantidad de llamadas, fuera del camino caliente,
y sin que nadie espere.

Y el argumento de fondo, que sale del punto 3: **el juez es lo único que puede separar lo accionable
de lo que no**, porque separar exige leer el contenido y ninguna señal barata lo logra. O sea que un
juez automático es lo que vuelve **asequible** la configuración actual del detector. Sin él, la
jugada racional es apretar el detector y resignar el 60 % de los hallazgos.

## Lo que falta decidir, y es del dueño

Gastar ~21 llamadas por día del presupuesto del motor, todos los días, para encontrar ~0,8
relaciones accionables. Eso no lo decide una medición: la medición dice **cuánto cuesta y cuánto
rinde**, y quién paga decide si vale.

## Riesgos, dichos de frente

- **n=9 accionables es poco.** El punto 3 descarta un umbral evidente; no prueba que no exista
  ninguna señal. Una re-medición con más datos puede cambiarlo.
- **El rendimiento medido depende de quién arbitró.** Los veredictos los pusieron sesiones de
  agente, no un panel independiente; un árbitro más severo habría marcado más `conflicts_with`.
- **La aceleración de llegada (1 → 71 → 234) puede ser del uso, no del detector.** Septiembre fue un
  mes de mucha escritura. Antes de dimensionar un drenador conviene ver si la tasa se sostiene.
