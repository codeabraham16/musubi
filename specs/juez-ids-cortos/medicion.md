# Medición — El juez ve ids cortos

Medido el 2026-09-30 en la laptop `davantis` (i5-1235U, 12 hilos, 7,5 GiB de RAM compartida con
otras tres sesiones que corrían pruebas pesadas al mismo tiempo). Sin red: nada de esto salió de la
máquina.

## En una línea

Con rótulos `id-1..id-N` en lugar de UUIDs, el juez lee un 41,5 % menos y escribe un 82,6 % menos
(tiktoken, sin modelo). Con un juez local (llama3.2:3b), cada llamada tardó 0,43 veces lo que con
UUIDs (mediana pareada), y fue más rápida en las 10 consultas que llamaron al juez. No hubo fallas de
formato ni rótulos mal copiados. Con UUIDs, en cambio, el modelo copió mal el mismo id en dos
consultas, igual en las dos corridas.

**La exactitud no se puede comparar con este juez.** Uno de 3B empeora el orden con los dos formatos:
MRR de 0,663 sin juez, 0,649 con UUIDs y 0,613 con rótulos, sobre 12 consultas. La diferencia entre
brazos sale casi entera de una sola consulta. La latencia de producción no se midió (ver «Lo que
esto dice y lo que no»).

## PASO 1 — Tokens, sin modelo

### Cómo

- `TestVolcarPromptsDelJuez` (`MUSUBI_JUEZ_VOLCADO=<dir>`) corre el banco con un juez que sólo
  graba, y vuelca el par (system, user) de cada llamada. Es el prompt que manda producción (J1).
  Se volcó con dos binarios: el de `origin/main` (704c5013, ids reales) y el nuevo (rótulos).
- Variantes: ids `uuid` (`fixtureConIDsOpacos`, la forma de producción) y `slug` (los ids del
  fixture tal cual), sobre la base `lexical` y la `hybrid` (potion-multilingual-128M, que es la base
  de producción). Top-K por defecto: 12.
- La **salida esperada** es el array JSON compacto con TODOS los ids en el orden en que se vieron:
  lo mínimo que el juez tiene que escribir.
- Tokenizador: tiktoken `o200k_base` y `cl100k_base`. No hay uno de Claude local.
- **Control**: los volcados reales del binario nuevo coinciden BYTE A BYTE con la simulación
  (cambiar cada `[<id>] ` al principio de renglón por `[id-<n>] `) en las dos variantes léxicas. La
  híbrida del binario nuevo no se volcó, porque el embebedor ocupa ~0,5 GB que no había. Su número
  es la simulación, que ese control valida.

### Resultados (o200k)

| Variante (llamadas · candidatos) | Ids | Entrada | Salida | Total | Ahorro |
|---|---|---:|---:|---:|---:|
| **uuid · híbrida** (12 · 144) | reales | 7033 | 3381 | 10414 | |
| | `c1..cN` (1ª versión) | 3970 | 444 | 4414 | −57,6 % |
| | **`id-1..id-N`** | **4114** | **588** | **4702** | **−54,8 %** |
| uuid · léxica (10 · 92) | reales | 4708 | 2165 | 6873 | |
| | `id-1..id-N` | 2845 | 378 | 3223 | −53,1 % |
| slug · híbrida (12 · 144) | reales | 4364 | 700 | 5064 | |
| | `id-1..id-N` | 4114 | 588 | 4702 | −7,1 % |
| slug · léxica (10 · 92) | reales | 3002 | 447 | 3449 | |
| | `id-1..id-N` | 2845 | 378 | 3223 | −6,6 % |

- **Por llamada** (uuid · híbrida, 12 candidatos): la entrada baja de 586,1 a 342,8 tokens
  (−41,5 %) y la salida de 281,8 a 49,0 (−82,6 %).
- **Por candidato**: −20,3 tokens de entrada y −19,4 de salida.
- **En caracteres**: −20,5 % de entrada y −81,2 % de salida.
- Con `cl100k` da lo mismo a un punto: el total baja de 10707 a 4962 (−53,7 %).
- `id-N` cuesta 1 token más por candidato que `cN` en la entrada y 1 más por elemento en la
  salida (588 contra 444 en la salida). D1 explica por qué se paga.
- **Los slugs del fixture no sirven para medir esto**: ahorran 7 %, porque un slug es corto y
  además le cuenta el tema al juez. Por eso la medición corre por defecto con ids opacos (D6).

### Margen

- **Contra llama3.2** (Ollama devuelve sus propias cuentas), en las 8 llamadas de la pasada 1:
  - la entrada de llama3.2 es la de `cl100k` más 25 tokens exactos, que son la plantilla del chat;
  - la salida es la de tiktoken más 0 o 1, que es el fin de secuencia;
  - la reducción de entrada según llama3.2 difiere de la de `cl100k` en 1,6 a 3,0 puntos (q-rrf:
    −40,1 % contra −41,7 %).

  Para ese modelo, tiktoken es la cuenta real.
- **Contra el motor de producción** (Claude) no se puede medir sin sacar texto de la máquina. El
  margen declarado es de ±25 % sobre los absolutos. Las reducciones relativas deberían sostenerse,
  porque el costo viene de los hex del UUID, que cualquier BPE parte en muchos tokens. **Es una
  suposición, no una medida.**

## PASO 3 — Exactitud y latencia con un juez local

### Condiciones

- Modelo llama3.2:3b (Q4_K_M, 2,3 GB) en Ollama 0.32.14, sobre `127.0.0.1`. Parámetros:
  `num_ctx` 2048, `temperature` 0 y `seed` 42.
- **Esos parámetros no lo vuelven bit-determinista.** El brazo de UUIDs repitió las cuentas de
  tokens de una corrida anterior en las 4 consultas con juez (q-rrf 624/278, q-ivf 283/67,
  q-provision 564/214, q-tenancy 247/74). Repitió también la respuesta en 3 de ellas. En
  q-provision, dos elementos vecinos salieron en el orden inverso. Una diferencia de una posición
  entre brazos puede ser, entonces, ruido del propio modelo en CPU.
- Base léxica (D6) y variante uuid, sobre las 12 consultas del dorado. Cada consulta corre en un
  proceso aparte. Los dos brazos van **intercalados por consulta y alternando cuál arranca**
  (ABBA), así que la carga del modelo, el swap y la temperatura pesan igual en los dos.
- Cada brazo es un binario de prueba distinto con el MISMO instrumento. El de ids reales se
  compiló con `-overlay` sobre el `rerank.go` de `origin/main`. Sus prompts volcados son idénticos
  byte a byte a los del binario de `origin/main`.
- **La máquina estaba en swap durante toda la medición**, así que la latencia ABSOLUTA no dice
  nada de producción. Sólo vale la comparación pareada entre brazos. El swap es zram de 7,5 G,
  más un swapfile de 2 G.

### Memoria y swap

La spec pedía ≥ 1,5 GB disponibles antes de cargar el juez. La máquina tiene 7,5 GiB de RAM y
9708 MB de swap. Otras sesiones corrían pruebas al mismo tiempo.

| Tanda | Al empezar (disponible · swap usado) | Antes de cada llamada (disponible · swap usado) | Peor lectura de la bitácora | Al terminar |
|---|---|---|---|---|
| Pasada 1 | **1375 MB** · 4832 MB | 67–236 MB · 6107–7186 MB | 66 MB · 7276 MB | 2109 · 5923 MB, tras `ollama stop` |
| Pasada 2 | 1563 MB · 5307 MB | 6–332 MB · 6538–7570 MB | 105 MB · 9047 MB | cortada por la guardia |
| Pasada 2, continuación | 2262 MB · 4662 MB | 237–790 MB · 5244–6411 MB | 216 MB · 6339 MB | 227 · 5799 MB, modelo descargado |

- **La pasada 1 arrancó por debajo del umbral**: 1375 MB contra los 1500 que pedía la spec. No la
  hice esperar, y debí hacerlo. La pasada 2 sí esperó 551 s, hasta tener ≥ 1500 MB en 3 lecturas
  seguidas.
- **El peor momento fue el de la pasada 2**: 9143 de 9708 MB de swap usados (564 MB libres),
  leído a mano a las 15:33, durante q-backup.
- **La guardia cortó la pasada 2 en q-backup**, con 8 de las 12 consultas hechas en los dos
  brazos, porque el swap libre estuvo 30 s por debajo de 1500 MB. Mató la prueba por PID, descargó
  el modelo y borró la salida a medias.
- **La continuación arrancó 3 min después**, con una guardia más estricta:
  - para empezar pide ≥ 2500 MB de swap libre;
  - lee cada 5 s;
  - corta con 2 lecturas malas.

  Corrió las 4 consultas que faltaban, siguiendo el mismo orden ABBA. Cada consulta se corrió
  entera, con sus dos brazos, en una sola de las dos tandas, así que ningún par mezcla condiciones.

### Pasada 1 — ids reales contra `c1..cN` (cortada)

Se cortó a la mitad (6 de 12 consultas), porque el formato ya había fallado:

| Consulta | Sin juez (RR) | Ids reales | `c1..cN` | Tokens de salida (reales → `c`) | Pared en s (reales → `c`) |
|---|---:|---:|---:|---|---|
| q-rrf | 1,000 | 0,083 | 0,083 | 278 → 38 | 214,5 → 72,2 |
| q-ivf | 1,000 | 0,333 | 0,250 | 67 → 14 | 63,6 → 45,8 |
| q-provision | 0,333 | 1,000 | **falla** (0,333) | 214 → 42 | 221,3 → 58,3 |
| q-tenancy | 1,000 | 0,500 | **falla** (1,000) | 74 → 17 | 92,4 → 43,4 |
| q-fts, q-redaction | 1,000 | no llaman al juez | | | |

**El hallazgo es cómo falló `c1..cN`.** En 2 de las 4 consultas que llamaron al juez, el modelo no
copió los rótulos que tenía a la vista, sino que contestó con la FORMA del ejemplo del system prompt
(`["id-a","id-b","id-c"]`):

- en q-provision escribió `["id-7","id-8","id-9",…]`;
- en q-tenancy escribió `["id-1234","id-5678","id-9012"]`.

Ningún elemento era un rótulo, así que `Rerank` dio error y el juez degradó al orden model-free
(I4/J9). La degradación funcionó como debía, pero el juez no juzgó. De los 29 elementos que
escribió el brazo `c`, 10 tenían el número de un candidato con otro prefijo (`id-7` en lugar de
`c7`) y 3 eran inventados. El brazo de UUIDs no falló ninguna consulta, y en esta pasada no copió
mal ningún id.

Copiar UUIDs también falla. En la corrida completa anterior del brazo de UUIDs (12 consultas, 10
llamadas, 92 candidatos vistos), de los 85 elementos que escribió el modelo:

- 82 eran exactos;
- 1 estaba repetido;
- 2 eran el MISMO UUID mal copiado, en dos consultas distintas: `baee279-…` (se comió la primera
  letra) y `ebaee279-…` (la cambió), en lugar de `dbaee279-…`.

Un id mal copiado se ignora por inventado, y esa memoria cae al final de la cabeza sin que nadie lo
vea.

De ahí sale D1: el rótulo toma la forma `id-<n>`, que es lo que el modelo tiende a escribir. El
ejemplo conserva las letras, así que copiarlo sigue siendo un error (D4).

### Pasada 2 — ids reales contra `id-1..id-N`

Doce consultas: 10 llamaron al juez y 2 no (q-fts y q-redaction). Las dos tandas (ver «Memoria y
swap») se agregan juntas.

**Exactitud** (base léxica, variante uuid):

| Brazo | MRR | nDCG@1 | nDCG@3 | nDCG@5 | nDCG@10 | R@10 |
|---|---:|---:|---:|---:|---:|---:|
| Sin juez | 0,663 | 0,583 | 0,561 | 0,596 | 0,630 | 0,750 |
| Ids reales | 0,649 | 0,500 | 0,562 | 0,582 | 0,611 | 0,750 |
| `id-1..id-N` | 0,613 | 0,500 | 0,509 | 0,545 | 0,560 | 0,667 |

- Sin juez y con ids reales dan **exactamente** lo que dio la corrida completa anterior del brazo de
  UUIDs (0,663 y 0,649). El brazo reprodujo su propio resultado.
- Por consulta, el RR es igual en 6 de las 10 que llamaron al juez. Los rótulos ganan en 2
  (q-tenancy: 0,500 → 1,000; q-forget: 0,167 → 0,250) y pierden en 2 (q-backup: 0,200 → 0,100;
  q-semantic: 1,000 → 0,083).
- La diferencia de MRR (−0,036) sale casi entera de q-semantic, que por sí sola resta 0,076. Con
  10 consultas y un empate 2 a 2 no hay diferencia medible entre brazos, en ningún sentido.
- **Acuerdo** entre los dos órdenes finales: mismo primero en 5 de 10, orden idéntico en 1 de 10,
  tau de Kendall medio de 0,21. Con este juez, cambiar el formato cambia el orden. No se sabe si
  lo cambia para mejor o para peor.

**Formato** (10 llamadas por brazo, 92 candidatos vistos):

| Brazo | Fallas del juez | Exactos | Repetidos | Mal copiados | Otra grafía | Inventados | Omitidos |
|---|---:|---:|---:|---:|---:|---:|---:|
| Ids reales | 0 | 79 | 1 | **2** | 0 | 0 | 13 |
| `id-1..id-N` | 0 | 87 | 0 | 0 | 0 | 0 | 5 |

Los 2 mal copiados son el MISMO UUID, `dbaee279-…`, y son los mismos de la corrida anterior:
`baee279-…` en q-bugfix y `ebaee279-…` en q-forget. No es azar. Con esa entrada, el modelo lo copia
mal siempre, y esa memoria cae al final de la cabeza sin que nada lo diga.

**Tokens** (lo que cuenta Ollama, promedio por llamada): la entrada baja de 517,1 a 329,6 (−36,3 %)
y la salida, de 194,8 a 36,8 (−81,1 %). La entrada baja menos que en el PASO 1 porque la plantilla
del chat suma 25 tokens fijos a cada llamada, y porque la base léxica trae menos candidatos.

**Latencia**, pareada por consulta:

| Consulta | Primero | Pared (s), ids reales → rótulos | Razón | Salida (tokens) | Entrada (tokens) |
|---|---|---|---:|---|---|
| q-rrf | reales | 76,5 → 36,8 | 0,48 | 278 → 50 | 624 → 386 |
| q-ivf | reales | 24,2 → 23,6 | 0,97 | 67 → 18 | 283 → 209 |
| q-provision | rótulos | 73,5 → 22,5 | 0,31 | 214 → 42 | 564 → 359 |
| q-tenancy | reales | 28,7 → 14,0 | 0,49 | 74 → 10 | 247 → 185 |
| q-bugfix | reales | 88,5 → 37,6 | 0,42 | 260 → 50 | 662 → 417 |
| q-deploy | rótulos | 105,6 → 34,3 | 0,32 | 286 → 50 | 639 → 388 |
| q-backup | reales | 56,9 → 16,7 | 0,29 | 269 → 42 | 617 → 363 |
| q-forget | rótulos | 50,6 → 22,3 | 0,44 | 169 → 50 | 659 → 417 |
| q-identity | reales | 19,4 → 11,2 | 0,58 | 73 → 14 | 255 → 194 |
| q-semantic | rótulos | 60,3 → 16,0 | 0,27 | 258 → 42 | 621 → 378 |

- Con rótulos, la llamada fue más rápida en **10 de 10** consultas. La razón pareada da una mediana
  de 0,43, y la media geométrica también da 0,43.
- En promedio, la pared bajó de 58,4 a 23,5 s: generar, de 32,3 a 6,8 s, y leer el prompt, de 25,6
  a 16,2 s. El modelo generaba a ~6 tokens/s en los dos brazos, así que casi todo el ahorro es
  escribir menos.
- Cuando los rótulos corrieron primero, la razón fue más baja (mediana 0,32, contra 0,49 con los
  reales primero). El orden ABBA reparte ese efecto entre los dos brazos, y aun con los reales
  primero la llamada con rótulos fue más rápida en 6 de 6.

**Un riesgo que los rótulos agregan: el número es la posición.** `id-1` es el primero del orden
model-free, así que un modelo que «cuenta» escribe una secuencia en vez de juzgar. En q-forget, los
rótulos dieron `8 11 10 12 9 7 6 5 4 3 2 1`, con una corrida de 7 números consecutivos:

- entre las respuestas con 6 o más elementos, una corrida de consecutivos de al menos la mitad de lo
  escrito apareció en 3 de 7 con rótulos y en 1 de 7 con UUIDs;
- el tau medio contra el orden model-free fue de −0,25 con rótulos y de −0,07 con UUIDs, con 4 y 3
  respuestas de 10 en −0,5 o menos: los dos brazos tienden a invertir.

Con 10 consultas y un modelo de 3B esto es una señal, no una medida, y no se cambió nada por esto
(D9). Barajar los rótulos le sacaría la pista al modelo, pero un modelo que cuenta pasaría a
devolver un orden al azar que no se distingue de un juicio. Hoy, en cambio, esa secuencia se ve.
Para decidirlo hace falta el juez de producción, que acá no se puede medir.

## Lo que esto dice y lo que no

- **Lo que está medido: los rótulos cuestan menos y se copian mejor.** Sin modelo, el prompt baja
  un 41,5 % y la respuesta un 82,6 %. Con llama3.2:3b, la llamada tardó 0,43 veces lo que con UUIDs
  (10 de 10 consultas), sin fallas y sin rótulos mal copiados. Con UUIDs hubo 2 mal copiados entre 82
  elementos escritos, y se repitieron idénticos en dos corridas.
- **Un juez de 3B, en este fixture, empeora el orden con los dos formatos.** Sobre 12 consultas, el
  MRR fue de 0,663 sin juez, 0,649 con UUIDs y 0,613 con rótulos. En la pasada 1 (6 consultas) fue
  de 0,889 sin juez, 0,653 con UUIDs y 0,611 con `c`. Por eso la exactitud, acá, mide la
  **robustez del formato** (si el modelo devuelve rótulos que se reconocen) y no la calidad del
  juez de producción. Ese juez es sonnet, y en la medición del 2026-08-10 subió el nDCG@1 de 0,359
  a 0,769 (memoria 2debc6d5).
- **12 consultas no dan significancia.** Una diferencia de MRR de este tamaño es ruido (lo dice
  `specs/juez-medible`). La de acá, además, es un empate de 2 a 2 entre consultas.
- **Queda sin medir si el número del rótulo sesga al juez** («Un riesgo que los rótulos agregan»,
  en la pasada 2, y D9). Hace falta el juez de producción.
- **La latencia de producción no se midió.** La hipótesis de la unidad era bajar de ~8,5 s a
  2–4 s. Con lo medido el 2026-08-12 (memoria c8ee49e2), esos 8,5 s se reparten así:
  - **~4,3 s de piso fijo**: el proveedor spawnea el harness de claude-agent-sdk en CADA llamada;
  - **~3–4 s** para el modelo, entre leer el prompt y generar la respuesta.

  Los rótulos sólo recortan la segunda parte. Aunque toda esa parte fuera generar los ~280 tokens
  de UUIDs y bajara en la misma proporción que los tokens (−82,6 %), la llamada quedaría en ~5 s,
  no en 2–4 s. **Esto es inferencia**, no medida. Bajar de ~4,3 s exige sacarse el spawn de
  encima, que es otro trabajo (c8ee49e2).
- **Lo que sí se midió de la latencia** es la razón pareada entre brazos en llama3.2 local. Esa
  razón refleja sobre todo cuántos tokens hay que generar: el modelo generaba a 2–3 tokens/s en la
  pasada 1, con la máquina hundida en el swap, y a ~6 en la pasada 2. Un motor que genera 20 veces
  más rápido ahorra, en proporción, mucho menos.

## Cómo reproducir

Todo corre desde `internal/recalleval`, con un binario de prueba compilado con `go test -c`:

```bash
# PASO 1: volcar prompts (sin modelo). Con MUSUBI_POTION_DIR suma la base híbrida.
MUSUBI_JUEZ_VOLCADO=/tmp/volcado MUSUBI_POTION_DIR=~/.musubi/embeddings/potion-multilingual-128M \
  ./recalleval.test -test.run '^TestVolcarPromptsDelJuez$' -test.v

# PASO 3: una consulta con el juez de Ollama local (sólo loopback; otra URL corta la prueba).
MUSUBI_JUEZ_OLLAMA_MODELO=llama3.2:3b MUSUBI_JUEZ_VARIANTE=uuid MUSUBI_JUEZ_CONSULTA=q-rrf \
  MUSUBI_JUEZ_SALIDA=/tmp/q-rrf.json ./recalleval.test -test.run '^TestMedicionJuezOllamaLocal$' -test.v
ollama stop llama3.2:3b
```

Los tokens se cuentan con tiktoken sobre el volcado. Los scripts de agregado y el intercalado
vivieron en el scratchpad de la sesión y no se versionan. Lo que hacen está descrito arriba.
