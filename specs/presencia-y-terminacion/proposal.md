# Presencia y terminación — los dos ejes que faltaban

## Por qué, y de dónde salió

No de una teoría: de **preguntarle al usuario con diseños delante**. En una prueba a ciegas eligió
uno de cada condición —o sea, el contenido del brief no determinaba lo que prefiere— y sobre los
tres dijo lo mismo: *«no son tan potentes»*. Preguntado qué les faltaba, nombró **presencia visual**
y **terminación de producto real**.

Medido después contra el acervo, la carencia era literal. De **1.736 entradas**:

| tema | tarjetas |
|---|---|
| escala tipográfica | 126 |
| transiciones / pulido | 95 |
| matriz de estados | 38 |
| personalidad, carácter | 26 |
| **un momento focal** | **14** |
| **terminación, detalle fino** | **9** |
| **contraste dramático** | **7** |

**El cerebro estaba lleno de «cómo no equivocarse» y vacío de «cómo hacer que impacte».** Y ninguno
de los 19 ejes lo cubría: `jerarquia` habla de ordenar y `color` de paleta, no de fuerza ni de
acabado. Ese conocimiento era **inalcanzable por ruteo aunque existiera**.

## Qué entra

1. **Dos ejes nuevos:** `presencia` y `terminacion`, con su descripción para embeber y su vocabulario.
2. **28 tarjetas nuevas** en el acervo compartido (14 por eje), sembradas en el tenant `musubi-design`.
3. **`project_id` en `musubi_save_observation`.** El handler ya lo leía y lo pasaba por
   `writeOriginFor` desde siempre — lo que faltaba era **declararlo en el schema**, así que el
   parámetro existía y no se podía invocar. Sin esto, la única puerta al acervo compartido era
   `musubi_ingest_url`, que necesita una URL.
4. **El slug declara el eje.** `design-corpus/<eje>-loquesea` etiqueta esa tarjeta con ese eje, sin
   depender del vocabulario.

## Por qué el slug, y qué se intentó antes

Al sembrar las 28 primeras, **sólo 6 y 5 de 14 caían por vocabulario**. La causa: el vocabulario lo
inventé yo en vez de sacarlo del material — el mismo error que ya había evitado con las
descripciones de eje y repetí acá.

Derivarlo del material **tampoco sirvió**: las palabras frecuentes en esas tarjetas y raras en el
resto del acervo eran «patrón», «elemento», «tamaño», «datos» — genéricas de diseño, habrían
etiquetado medio acervo.

El slug resuelve el caso que el vocabulario no puede: **quien escribe una tarjeta necesita una
manera exacta de decir a qué eje pertenece.** El vocabulario sigue existiendo para las 1.438
tarjetas viejas que nadie va a renombrar. Los dos suman: el slug es preciso, el vocabulario amplio.

## Invariantes

**I-EJE6 · el slug declara el eje, y el vocabulario no puede ser la única vía.**
Sabotaje: sacar el camino del slug ⇒ una tarjeta que nombra su eje en el topic y no repite su
vocabulario en el cuerpo queda sin etiquetar. *Verificado en rojo.*

⚠️ **El fixture de este invariante falló dos veces antes de servir.** La primera versión usaba el
topic real `presencia-un-solo-protagonista`, que trae **dos palabras del vocabulario en el propio
slug** —«presencia» y «protagonista»— así que se etiquetaba por el otro camino y el sabotaje pasaba
verde. Ahora el fixture **verifica su propia premisa**: comprueba primero que sin el prefijo del eje
la tarjeta NO se etiqueta.

## Fuera de alcance

- **Medir si mueve M1 o M3.** No debería: el ruteo elige por eje y estas tarjetas amplían lo que hay
  DENTRO de dos ejes nuevos. Lo que cambia es que un pedido de presencia deja de rutear a `a11y` y
  uno de terminación deja de rutear a `movil` — verificado en producción antes del deploy.
- **Etiquetar el resto del acervo por similitud.** Medido aparte: subiría la cobertura de 38 % a
  71 %, pero M1 sólo +0,02 y M8 peor. Va al plan siguiente si alguna vez tiene sentido.
