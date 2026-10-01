# Embebedores locales contra POTION, sobre la memoria real

> Unidad `e4e92462` del lote `48a47614`, rama `medir/embeddings-locales` (sale de `origin/main`
> `704c5013`; el arnés está en `982eb0df` y `cc85ddc1`). Medido entre el 2026-09-30 y el
> 2026-10-01 en `davantis` (Linux, i5-1235U, 7,6 GB de RAM compartidos con otras sesiones que
> compilaban). La memoria real se usó como **copia de sólo lectura**, se borró al terminar cada
> corrida y no salió de la máquina: el arnés exige un Ollama en loopback. Acá van sólo agregados,
> sin texto de la memoria ni ids de observaciones.

## Lo que hay que llevarse

1. **granite-97m le gana a POTION en la memoria real, y por mucho.** La comparación es pareada,
   consulta por consulta, sobre la misma copia (88 consultas):
   - MRR 0,587 contra 0,428: Δ +0,159, IC95 [+0,081, +0,238]. Mejoran 44 consultas y empeoran 14.
   - recall@10 0,485 contra 0,393: Δ +0,092 [+0,036, +0,146].
   - nDCG@10 0,437 contra 0,335: Δ +0,102 [+0,051, +0,152].

   Contra el léxico solo, el MRR sube +0,122 [+0,037, +0,206]. **Es el primer embebedor medido
   que le suma algo al léxico en la memoria real** (§2).
2. **El vector de POTION no le suma nada medible al léxico en la memoria real.** Se midieron tres
   copias de la memoria, y en las tres empeoran más consultas de las que mejoran (en MRR, 16 contra
   38, 18 contra 37 y 14 contra 41). La diferencia de medias no se separa del cero. Un vector
   constante, que es un embebedor roto a propósito, quedó a 0,044 de POTION sin separarse (IC95
   [−0,117, +0,030]). Este instrumento no distingue a POTION de un vector sin señal (§3).
3. **Lo que cuesta granite (§4):**
   - ~350 ms por consulta (p50), contra ~1 ms de POTION.
   - Ollama corriendo, con ~1 GB de RSS en su llama-server.
   - Re-embeber la memoria una vez: 27 min estimados sin carga y 53 medidos con la máquina en swap.
   - 2,5 a 3,2 s de carga en frío. Se repite seguido, porque Ollama descarga el modelo a los 5 min
     sin uso.
4. **Los otros tres candidatos no entran en esta laptop.** granite-311m, harrier-270m y bge-m3
   tardarían 88, 245 y 313 min en re-embeber la memoria, y bge-m3 además mandó 1,6 GB de su
   llama-server a swap. bge-m3 no tiene números de calidad: la corrida que lo incluía murió en el
   límite de 2 h sin terminar de re-embeber (§4 y §5).
5. **Hoy POTION embebe un documento por un tercio de lo que cuesta granite, y la culpa no es del
   modelo: es del tokenizador.** POTION tokeniza por el camino del mapa, y el índice que ya está en
   el repo da los mismos ids ~100× más rápido. En la corrida de calidad, POTION re-embebió a 530 ms
   por doc y granite a 1.588 (§4). Arreglarlo no cambia la calidad, pero abarata guardar y
   re-embeber.
6. **No se cambió nada.** Adoptar granite, como opción o por defecto, es una decisión de producto
   con costos reales. Las condiciones están en §6.

---

## 1. Qué se midió y cómo

- **Arnés:** `TestEmbebedoresLocalesVsPotion`, en `internal/recalleval/embebedores_locales_test.go`.
  Se saltea si faltan `MUSUBI_EMBED_CANDIDATOS` o `MUSUBI_POTION_DIR`. Corre un modelo de Ollama
  por vez: con 7 GB compartidos, dos modelos cargados a la vez miden la presión de memoria y no el
  modelo.
- **Fixtures:**
  - *dorado:* 26 docs y 12 consultas, con etiquetas escritas a mano.
  - *real:* una copia de `.musubi/memory.db` hecha con la API de backup de SQLite, en modo sólo
    lectura. Según el día tuvo entre 1.986 y 2.070 docs (~6,6 M caracteres), y siempre las mismas
    88 consultas. **Las etiquetas salen del `topic_key`, no de un juicio humano:** miden si el
    motor junta lo que la memoria agrupó bajo el mismo tema.
- **Configs:**
  - *léxica*.
  - *híbrida:* la de producción, sin MMR ni corrector de tipeo, con el piso de coseno en 0,30.
  - *híbrida con piso 0:* el 0,30 se calibró con los cosenos de POTION, y medir sin piso separa
    «el modelo es peor» de «el piso no le queda».
- **Re-embeber** usa el mismo camino que producción (`EmbedBackfill`), cronometrado.
- **Estadística:**
  - Pareada por consulta, contra POTION y contra el léxico del mismo motor, en MRR, recall@5,
    recall@10 y nDCG@10.
  - IC95 por bootstrap (10.000 réplicas, semilla fija). p es la prueba del signo exacta a dos
    colas.
  - **La flecha ↑/↓ aparece sólo cuando el IC95 no toca el cero.**
- **Controles del instrumento** (si cualquiera falla, la prueba sale roja):
  - El brazo léxico tiene que dar idéntico consulta por consulta entre los dos motores. Dio
    idéntico en todas las comparaciones que se corrieron.
  - Un embebedor roto (`constante`) tiene que salir peor que POTION con un IC95 que no toque el
    cero. Ver §3.
  - Las cinco directivas de sabotaje del arnés dieron rojo por el motivo correcto, medido sobre
    `982eb0df` (`go run ./deploy/cmd/arnes -correr -paquete ./internal/recalleval`).
- **Costo:** un microbench aparte, fuera de la prueba, con `/api/embed` de Ollama sobre 84
  fragmentos de 3.200 caracteres de `CHANGELOG.md` (no de la memoria): 20 de a uno y 4 lotes de
  16. A eso se suman los costos que la prueba anota en cada corrida.

## 2. granite-97m contra POTION (2026-10-01, copia de 2.070 docs)

Fixture real, híbrida de producción, 88 consultas. G/P/E son las consultas que mejoran, empeoran
o empatan.

| Métrica | granite-97m | POTION | Δ [IC95] | p (signo) | G/P/E |
|---|---|---|---|---|---|
| MRR | 0,587 | 0,428 | **+0,159** [+0,081, +0,238] ↑ | < 0,001 | 44/14/30 |
| recall@5 | 0,341 | 0,246 | **+0,096** [+0,039, +0,153] ↑ | < 0,001 | 38/11/39 |
| recall@10 | 0,485 | 0,393 | **+0,092** [+0,036, +0,146] ↑ | 0,002 | 33/12/43 |
| nDCG@10 | 0,437 | 0,335 | **+0,102** [+0,051, +0,152] ↑ | 0,002 | 51/23/14 |

Lo mismo contra el léxico solo, en el mismo motor:

| Métrica | granite-97m | léxico | Δ [IC95] | G/P/E |
|---|---|---|---|---|
| MRR | 0,587 | 0,465 | **+0,122** [+0,037, +0,206] ↑ | 45/18/25 |
| recall@5 | 0,341 | 0,270 | **+0,071** [+0,014, +0,128] ↑ | 35/12/41 |
| recall@10 | 0,485 | 0,363 | **+0,122** [+0,065, +0,180] ↑ | 38/11/39 |
| nDCG@10 | 0,437 | 0,330 | **+0,106** [+0,052, +0,160] ↑ | 53/21/14 |

- **En el dorado no se separa nada.** El MRR da 0,706 contra 0,709, y el recall@10 0,917 contra
  0,833 con IC95 [0, +0,208]. Con 12 consultas, los dos rinden parecido.
- **El piso de producción no le recorta nada a granite.** La híbrida y la de piso 0 dan idéntico en
  las cuatro métricas, porque sus cosenos quedan arriba de 0,30. Con POTION, en cambio, sacar el
  piso empeora el MRR de 0,428 a 0,408. Si se adopta granite, el piso hay que recalibrarlo con sus
  cosenos: así como está, no filtra nada.

## 3. Lo que suma POTION, y hasta dónde ve este instrumento

La híbrida de POTION contra el léxico, en MRR, sobre las tres copias de la memoria:

| Copia | Docs | Léxico | Híbrida POTION | Δ [IC95] | p (signo) | G/P/E |
|---|---|---|---|---|---|---|
| 2026-09-30 | 1.986 | 0,483 | 0,443 | −0,039 [−0,101, +0,022] | 0,004 | 16/38/34 |
| 2026-10-01, 09:51 | 2.069 | 0,465 | 0,463 | −0,002 [−0,060, +0,058] | 0,014 | 18/37/33 |
| 2026-10-01, 10:47 | 2.070 | 0,465 | 0,428 | −0,038 [−0,095, +0,021] | < 0,001 | 14/41/33 |

- **El patrón se repite en las tres:** por cada consulta que el vector de POTION mejora, empeora más
  de dos. Aun así, la media no se separa del cero.
- **El control de sabotaje dio rojo, y eso es un resultado.** Sobre la copia del 2026-09-30, la
  híbrida con el vector constante dio MRR 0,399, contra 0,443 de POTION: Δ −0,044, IC95 [−0,117,
  +0,030], p = 0,389. El arnés sí usa el embebedor, porque el ranking se movió. Pero con 88
  consultas **no separa la señal de POTION de un vector sin señal**. Contra el léxico, el constante
  sí sale peor: Δ −0,083 [−0,150, −0,016]. El criterio se fijó antes de correr y no se aflojó.
- **El mismo instrumento separa a granite de POTION con margen:** Δ +0,159, con el IC95 arrancando
  en +0,081. La resolución de 88 consultas queda entre esas dos distancias. Una diferencia de
  ~0,04 de MRR no se puede leer, y una de ~0,16 sí.
- **Hay variabilidad entre copias, y no se aisló la causa.** Entre las dos copias del 2026-10-01:
  - la memoria sumó un doc;
  - las 88 consultas son las mismas;
  - el léxico no se movió (0,465);
  - la híbrida de POTION bajó de 0,463 a 0,428.

  No se verificó si fue el doc nuevo subiendo en muchas consultas o un desempate no determinista
  en la fusión. Por eso **vale lo pareado dentro de una misma corrida**. Aun contra el mejor número
  que dio POTION (0,463), granite queda 0,124 arriba.

## 4. Costos

**Microbench** (2026-10-01): sin otra corrida pesada, un modelo por vez, sobre 84 fragmentos de
3.200 caracteres. Un doc de la memoria tiene ~3.200 caracteres en promedio.

| Modelo | Licencia | Dim | Carga en frío | De a uno, p50 | En lote, por doc | Re-embeber 1.922 docs (estimado) | llama-server: RSS / swap |
|---|---|---|---|---|---|---|---|
| granite-97m (F16) | Apache-2.0 | 384 | 2,46 s | 1.013 ms | 829 ms | 26,6 min | 1.061 / 0 MB |
| granite-311m (Q8_0) | Apache-2.0 | 768 | 2,74 s | 3.086 ms | 2.747 ms | 88,0 min | 1.637 / 0 MB |
| harrier-270m (Q8_0) | MIT | 640 | 1,82 s | 7.100 ms | 7.654 ms | 245,2 min | 2.311 / 376 MB |
| bge-m3 | MIT | 1.024 | 4,38 s | 8.521 ms | 9.784 ms | 313,4 min | 692 / 1.589 MB |

POTION es MIT, de dimensión 256, y corre dentro del proceso, sin pasar por Ollama.

**En la corrida de calidad** la máquina estaba bajo presión: el swap pasó de 3,5 a 5,5 GB, contando
el de otras sesiones.

| | POTION | granite-97m |
|---|---|---|
| Consulta, p50 / p95 (n = 100) | 1 / 6 ms | 352 / 450 ms |
| Primera llamada | armar la tabla: 5,1 s | 3,2 s en frío (incluye cargar el modelo) |
| Re-embeber la memoria real (2.006 docs) | 1.064 s (530 ms/doc) | 3.185 s (1.588 ms/doc) |
| Un doc suelto, p50 / p95 (n = 16) | 489 / 3.736 ms | 1.824 / 13.831 ms |
| Memoria | 793 MB de RSS en el proceso, y otros 433 en swap | 247 MB de modelo en Ollama; la memoria disponible bajó de 2.943 a 2.202 MB |

- **Re-embeber con granite tardó el doble de lo que estimaba el microbench:** 53 min contra 27. Esa
  diferencia es la presión de memoria de la máquina, no el modelo.
- **El costo por doc de POTION es un problema de implementación.** Su tokenizador va por el camino
  del mapa, que prueba cada prefijo de hasta 186 runas en cada posición del texto. El índice de
  piezas (`tokenizer.idx`), que hoy usa sólo la consulta liviana, da los mismos ids ~100× más
  rápido: ~0,65 ms contra 77–88 ms cada 1.000 caracteres. Con ese arreglo, POTION re-embebería la
  memoria en segundos, y la comparación de costos contra granite cambia de escala.
- **Hoy el hook por turno no embebe la consulta:** `NewProviderDeConsulta` no tiene llamadores
  fuera de las pruebas. Con granite, los ~350 ms por consulta los pagaría `musubi_recall`, y cada
  observación nueva pagaría su embebido al guardarse (1 a 1,8 s por doc). El turno no paga nada. Si
  algún día el turno usa el vector, con granite pagaría eso en cada turno, o ~3 s en frío, contra
  un techo de 10 s.

## 5. Qué no se midió, y por qué

- **bge-m3 no tiene números de calidad.** La corrida que lo incluía, el 2026-09-30 junto con el
  vector constante, la cortó el límite de 2 h de los trabajos de fondo antes de que bge-m3
  terminara de re-embeber la memoria real. De esa corrida quedaron POTION y el constante. Después,
  el microbench estimó 313 min de re-embebido y mostró 1,6 GB en swap: en esta laptop no es viable.
- **granite-311m y harrier-270m no tuvieron corrida de calidad**, por costo: 88 y 245 min estimados
  sólo para re-embeber.
- **El servidor no se midió.** La máquina de referencia es la laptop.
- **No hay calidad medida con etiquetas humanas sobre la memoria real.** Las del fixture real salen
  del `topic_key`, y un dorado de 12 consultas no alcanza para separar nada.
- **El arreglo del tokenizador de POTION no se implementó:** esta unidad sólo medía.

## 6. Recomendación (la decisión es del usuario)

1. **Lo barato primero: el tokenizador de POTION.** No cambia la calidad, abarata ~100× el embebido
   de cada doc y no agrega dependencias. Antes de cambiarlo hay que correr las pruebas de identidad
   bit a bit y comparar los ids de los dos caminos sobre texto real largo.
2. **granite-97m es el único candidato que vale la pena, y como opción, no por defecto,** hasta
   resolver tres cosas:
   - **Depende de Ollama.** Hoy la búsqueda vectorial corre sin red ni servicios aparte. Antes de
     ofrecer granite hay que comprobar qué hace `musubi_recall` con Ollama caído: tiene que
     degradar a léxico, no romper.
   - **El piso de coseno** hay que recalibrarlo con los cosenos de granite (§2).
   - **El re-embebido inicial** lleva 30 a 50 min en esta laptop, una sola vez.
3. **Que el vector de POTION no le sume al léxico en la memoria real no autoriza a apagarlo.** El
   dorado dice lo contrario, aunque sin separarse, y las etiquetas por `topic_key` son una vara
   angosta. Sí autoriza a no esperar de POTION lo que da granite.

## Cómo se reproduce

```bash
MUSUBI_EMBED_CANDIDATOS=granite-97m \
MUSUBI_POTION_DIR=<directorio de la tabla POTION> \
MUSUBI_OLLAMA_URL=http://127.0.0.1:11434 \
MUSUBI_FIXTURE_DB=<COPIA de memory.db, nunca la base viva> \
MUSUBI_EMBED_SALIDA=<resumen.json> \
go test ./internal/recalleval -run '^TestEmbebedoresLocalesVsPotion$' -v -count=1 -timeout 110m
```

Va un candidato por corrida, con `ollama stop` entre una y otra. El JSON de salida no lleva texto
de la memoria. Con granite-97m, la corrida entera tardó 74 min en esta laptop.
