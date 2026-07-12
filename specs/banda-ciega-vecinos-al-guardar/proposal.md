# Proposal — banda-ciega-vecinos-al-guardar

## Intención

**Musubi tiene una banda ciega donde viven las contradicciones, y hoy no mira ahí.** Encontrado juzgando la cola real, no en teoría: la memoria de la deep-research decía *«NordVPN y Tailscale NO pueden coexistir de forma confiable»* y la solución posterior lo **dio vuelta** — y **nunca se creó relación entre ambas**.

Medido sobre las 436 observaciones reales (94.830 pares):

| | valor | puerta | ¿pasa? |
|---|---|---|---|
| **coseno** del par que se contradice | **0.806** | 0.85 | ❌ |
| **jaccard** léxico | 0.213 | 0.30 | ❌ |

Pasó **por debajo de las dos puertas**. Y sin embargo ese 0.806 es **más similar que el 99% de todos los pares**: no es una señal débil perdida en el ruido, es de las más fuertes que hay.

## El diagnóstico (y por qué el piso NO está mal calibrado)

> El piso de 0.85 se midió sobre **duplicados** (los casi-idénticos dan ~0.99). Pero **una contradicción no es un duplicado**: decir *lo contrario* usa **otras palabras**. Vive estructuralmente **más abajo** en la escala.

El detector está afinado para encontrar **redundancia** — y la contradicción no es redundancia, es su opuesto. De ahí sale una **banda ciega ≈[0.80, 0.85)**: *«mismo tema, dicho distinto»*. **El falso negativo no es un bug del umbral: es una consecuencia de pedirle a UN umbral que haga DOS trabajos.**

## Por qué bajar el piso NO es la solución (medido)

| piso | pares | vs. hoy |
|---|---|---|
| **0.85** (hoy) | 356 | — |
| 0.82 | 638 | ×1.8 |
| **0.80** (dejaría entrar la contradicción) | **1.024** | **×2.9** |
| 0.75 | 3.665 | ×10.3 |

Bajar a 0.80 **triplica la cola**: ~3 pendientes extra **por cada memoria nueva**. Es volver a llenarla del ruido que #203 acaba de sacar — comerse la cola por la otra punta. Filtrar por prefijo del `topic_key` tampoco salva: el **67%** de la banda igual comparte prefijo.

**Conclusión: el piso no se toca.**

## La distinción que resuelve el trade-off

La falla real **no fue que el detector no decidiera** — fue que **nunca le mostró el par al agente**. Ese día el agente escribió la memoria de la SOLUCIÓN *sabiendo* que daba vuelta la conclusión anterior. Si Musubi le hubiera dicho «ojo, estas 2 memorias hablan de esto mismo», la habría marcado en el acto.

> **Encolar** una relación cuesta caro: exige un veredicto y **vive en la cola** hasta que alguien lo dé.
> **Mostrarle** los vecinos al agente **en el momento de guardar** cuesta **~cero**: el agente ya está ahí, ya tiene el contexto, y puede actuar o ignorar al instante.

Hoy esas dos cosas están **fusionadas en una sola**. La banda ciega no debería **encolar** — debería **mostrar**.

Es el principio del track, completado: *automatizá los hechos, delegá las interpretaciones* — **y para delegar bien, hay que MOSTRARLE al que juzga.**

## Alcance

- Al guardar una observación, además de las relaciones que ya se detectan, Musubi calcula los **vecinos de la banda ciega** (coseno en `[bandaFloor, CosineFloor)`) y los **muestra al agente** en la respuesta del `save`, con una pregunta explícita: *«¿alguna de estas queda superada por lo que acabás de guardar?»*.
- **NO se persiste ninguna relación.** No se crea `pending`, no se toca la cola, no se oculta nada.
- Sólo en el camino **explícito del agente** (`musubi_save_observation`), donde hay alguien en el loop que puede responder. **NO** en los caminos automáticos (captura de commits, error→fix): ahí no hay nadie a quien mostrarle.

## Fuera de alcance (explícito)

- NO se cambia el piso de coseno, ni el AND-gate, ni el gate de novedad, ni las guardas de #203.
- NO se crean relaciones nuevas ni se modifica la cola.
- NO se intenta **decidir** si hay contradicción (eso es evaluación de predicados ⇒ techo semántico ⇒ **el agente**).

## Estrategia de rollback

Un piso de banda en 0 (o >= CosineFloor) apaga la feature: no se muestran vecinos y el `save` responde exactamente como hoy. Sin migración, sin cambio de esquema, sin datos tocados.

## Riesgos

- **Ruido en la respuesta del `save`.** Si se muestran demasiados vecinos, el agente los ignora — la misma erosión que combatimos, mudada de la cola al output. Mitigación: **techo duro** de vecinos mostrados (los más similares primero) y un texto **corto**.
- **Falso sentido de cobertura.** Mostrar vecinos NO garantiza que se detecte toda contradicción: sólo cubre las que caen en la banda. Una contradicción con coseno < 0.80 sigue invisible. Hay que **decirlo**, no venderlo como resuelto.
- **La banda es una heurística, no una verdad.** El 0.80 sale de UNA medición sobre UNA memoria (436 obs). Debe ser **configurable**.
