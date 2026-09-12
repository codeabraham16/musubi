# Propuesta — La señal vectorial en el hook por turno

Item 02 del informe de la Ola 4. **Esta propuesta no recomienda construir todavía**: recomienda
**empezar por otra punta que la que estaba escrita**, y dice cuál. Todo lo que sigue está medido el
2026-09-11 sobre la tabla real `potion-multilingual-128M` (488 MB + 17 MB de `tokenizer.json`).

## La pregunta

El hook `musubi turn --hook-mode` es un **proceso efímero** —uno nuevo por prompt— con techo de
**10 s** en `.claude/settings.json`. Hoy corre **sin señal vectorial**: `embedderCaroDeConstruir`
apaga el embebedor cuando la tabla es estática, porque construirlo cuesta segundos y gigabytes.

La señal vale la pena: medido aparte, **nDCG@1 0,294 → 0,353**.

> ¿Hay alguna forma de entregarla bajo el techo?

**Sí.** El piso medido es **~1013 ms**, contra un techo de 10 s. Eso antes no se sabía.

## Lo medido

Un proceso fresco por medición (importa: ver «El método», abajo).

| etapa | hoy | con la tabla mapeada |
|---|---|---|
| `ReadFile(model.safetensors)` | 1019 ms | **0,06 ms** |
| `parseStaticTable` (copia a `[]float32`) | 534 ms | **0,5 ms** |
| `ReadFile(tokenizer.json)` | 26 ms | 26 ms |
| `loadTokenizerBytes` | 983 ms | 983 ms |
| identidad de la tabla (`staticTableChecksum`) | 86 ms | **2000-3035 ms** ← |
| **`NewStaticProvider` punta a punta** | **3298 ms · 1325 MB RSS** | **1013 ms · 188 MB RSS** |

### 1 · «Mapear la tabla» **solo** deja el arranque peor, no mejor

Era el camino escrito en `cmd/musubi/embed.go`, y está **incompleto de una forma que invierte el
resultado**.

El mmap vuelve la tabla prácticamente gratis: **1553 ms → 0,6 ms**, y la saca del heap. Pero
`staticTableChecksum` **recorre las 488 MB enteras** para derivar la identidad, y sobre un mapeo eso
**obliga al kernel a traer cada página**: pasa de 86 ms (sobre un buffer ya residente) a **2-3
segundos**, arrastrando 292-452 MB de RSS.

O sea: el checksum se come solo toda la ganancia del mapeo, y más. **El mmap sólo rinde si la
identidad deja de tocar la tabla entera.**

El checksum no se puede sacar: existe por **N1** —re-destilar la tabla in-place tiene que cambiar el
`model_id`, o los vectores viejos siguen pareciendo compatibles y el ranking se corrompe **en
silencio**—. Lo que sí se puede es **muestrearlo**:

| identidad | costo | RSS |
|---|---|---|
| completa (la de hoy), 488 MB | 2000-3035 ms | +292-452 MB |
| muestreada, 1024 ventanas de 4 KB | 75 ms | +82 MB |
| muestreada, 256 ventanas de 4 KB | **18-29 ms** | **+27 MB** |

Ojo con el RSS: 256 ventanas son **1 MB leído** y mueven **27 MB**, porque el *readahead* del kernel
trae una ventana entera por cada página tocada.

**Lo que una identidad muestreada pierde, dicho de frente:** detecta una re-destilación (que cambia
prácticamente todos los vectores) con probabilidad ~1, pero **no** detecta una edición quirúrgica
entre puntos de muestreo. Para la amenaza que N1 describe —«re-destilé la tabla y la dejé en el
mismo lugar»— alcanza; para un adversario, no. Si eso no se acepta, la alternativa es cachear la
identidad completa en un sidecar y revalidarla con la muestra.

### 2 · Arreglado eso, la tabla deja de importar y **el tokenizer pasa a ser todo**

Con la tabla mapeada y la identidad muestreada, la tabla cuesta **0,6 ms**. El piso queda en
1013 ms, de los cuales **~1009 ms son el tokenizer** — el **97 %**.

Nadie lo tenía señalado. Todo el foco estaba en las 488 MB de la tabla, que resultan ser la parte
fácil.

### 3 · El costo del tokenizer **no** es parsear de más (hipótesis mía, refutada)

`loadTokenizerBytes` deserializa el `tokenizer.json` **entero dos veces**: una para sacar
`model.type` y otra, en la rama Unigram, para el vocabulario. Con 17 MB y 500.353 piezas eso se lee
como desperdicio evidente, y el arreglo parece obvio: tomar `model` como `json.RawMessage` en la
primera pasada.

**Medido en tres procesos frescos, el «arreglo» es 34-38 % MÁS LENTO** (383-390 ms → 520-530 ms), y
coincide con lo que da contar el trabajo:

| | recorridos del documento | copias de 17 MB |
|---|---|---|
| hoy | 2 | 0 |
| con `RawMessage` | 3 | 1 |

`json.RawMessage` **copia**, y el objeto `model` son casi los 17 MB enteros; después sacarle el
`type` cuesta otro recorrido. Mientras tanto, el decodificador de Go **saltea** los campos que no le
pedís sin materializarlos, y eso ya sale más barato que copiarlos.

El costo real está en **construir el mapa de 500.353 entradas** y sus scores. Bajarlo pide un
**caché binario del tokenizer ya armado**, que es obra aparte — y es por donde hay que empezar.

### 4 · El recurso escaso no es el CPU: es la memoria

La máquina donde se midió tiene **7,6 GB de RAM con 8,5 GB ya en swap**. El arranque de hoy pide
**1325 MB de RSS** porque la tabla entra **dos veces** —los bytes crudos de `ReadFile` **más** la
copia a `[]float32` de `parseStaticTable`— y esa plata no está.

Visto así, el número que importa no es 3298 ms → 1013 ms. Es **1325 MB → 188 MB**, y el mmap lo
consigue porque la tabla pasa a ser memoria **respaldada por archivo**: descartable, compartida
entre procesos, y que no compite por swap.

## El método, porque casi me come dos veces

**Una medición por proceso.** Estas pruebas dejan entre 0,5 y 1,3 GB tocados; corridas en el mismo
binario se contaminan. No es teoría:

- El desglose de etapas pasó de **4,9 s a 121 s** al correrlo junto con las demás.
- La comparación del tokenizer **dio vuelta el signo**: −35 % en procesos frescos, **+41 %** en la
  corrida conjunta.
- Y antes de eso, la primera corrida le adjudicó **1712 ms** al checksum cuando aislado son **86 ms**
  — un artefacto del GC marcando un heap de 1,3 GB **debajo** de la etapa cronometrada.

Tres veces el instrumento dijo algo que no era. Lo que se sostiene son **las razones entre etapas y
los órdenes de magnitud**, no los milisegundos: el mismo `ReadFile` midió 1019 ms con la máquina
holgada y 3356 ms con ella cargada.

## Lo que esto recomienda

**El orden, que es lo que cambia:**

1. **El tokenizer primero** — caché binario del tokenizer ya construido. Es el 97 % del piso, y es
   el único de los tres que hoy no tiene camino conocido.
2. **La identidad después** — muestreada, o cacheada en sidecar. Sin esto el paso 3 no rinde.
3. **El mmap al final en el orden, pero no es el que menos importa** — ver «La alternativa del
   daemon» más abajo: es el único de los tres que resuelve el caso de **N procesos**, que es el que
   esta máquina tiene de verdad (cinco daemons vivos). Necesita además una mitad Windows
   (`CreateFileMapping`), porque `syscall.Mmap` no existe ahí.

**Y los dos se juntan mejor de lo que parece:** si el caché binario del tokenizer (paso 1) se
escribe en un formato **mapeable**, deja de ser heap anónimo y pasa a compartirse igual que la
tabla. Hoy el tokenizer son ~178 MB de mapa por proceso —×5 daemons, ~890 MB que no se comparten
con nadie—. Con los dos artefactos respaldados por archivo, N procesos cuestan casi lo mismo que
uno, y ése es el único diseño que escala con la forma real de esta máquina.

**Dos precondiciones del atajo, que `unsafe.Slice` no te pregunta.** Aliasar el blob como
`[]float32` en vez de convertirlo elemento por elemento es de donde salen los 534 ms, y vale sólo
si:

- **la máquina es little-endian**, y
- **el blob arranca en un offset múltiplo de 4**. Sale de `8 + largo del header JSON +
  data_offsets[0]`, y **nada en el formato safetensors obliga a que eso sea múltiplo de 4**: en la
  tabla de POTION da 88 por cómo la escribió HuggingFace, no por garantía. Con un offset
  desalineado, `unsafe.Slice` a `*float32` es **comportamiento indefinido** — anda en x86 y puede
  romper en ARM, que es donde corre `test-cross (macos-latest)`.

La implementación tiene que **chequear las dos y caer al camino de copia** si alguna no da. Las dos
están asertadas en `arranque_mmap_unix_test.go`.

### CORRECCIÓN (2026-09-12) — los daemons SÍ tenían la tabla, y el error fue mío

**Lo que decía esta sección estaba mal, y el argumento que sostenía se da vuelta.** Decía: «hay cinco
`musubi daemon` vivos y sus RSS son 6, 6, 11, 6 y 8 MB: **ninguno tiene la tabla**». De ahí salían
«la alternativa no es un daemon, es N» y «5 × 1,3 GB» como escenario **futuro**.

**Medí `VmRSS` y `VmRSS` no cuenta lo que está swapeado.** Los mismos procesos, mirados bien:

| | `VmRSS` | `VmSwap` | anónimo real |
|---|---|---|---|
| daemon 1 | 82 MB | **594 MB** | **664 MB** |
| daemon 2 | 82 MB | **594 MB** | **664 MB** |
| daemon 3 | 82 MB | **593 MB** | **663 MB** |

664 MB = 488 MB de tabla `[]float32` + ~178 MB del mapa del tokenizer. **Los dos artefactos, exactos.**
Y `safetensors` **no aparece** en `/proc/PID/maps`: no está mapeada, está leída a heap anónimo,
exactamente como manda `os.ReadFile` en `static.go:55`.

Confirmado además en directo, arrancando un daemon nuevo en este repo:

```
t=4s   RssAnon=   10 MB              (recién arrancado)
t=8s   RssAnon= 1156 MB + Swap=197 MB = 1353 MB anónimo   ← safetensors_mapeado=0
```

**De 10 MB a 1,35 GB en menos de 8 segundos.** Después el GC libera los bytes crudos y se estabiliza
en ~664 MB.

**La ironía, dicha de frente:** dos secciones más abajo este mismo documento avisa que «RSS MIENTE
SOBRE LAS PÁGINAS COMPARTIDAS». Caí en el hermano exacto de esa trampa — **RSS también miente sobre
las páginas anónimas swapeadas** — mientras escribía la advertencia.

### Lo que la corrección cambia

**Los `N × 1,3 GB` no son un escenario futuro: ya se están pagando.** Tres daemons × 664 MB ≈
**2 GB** de tabla duplicada, casi toda empujada a swap. Y el swap de esta máquina es **zram**
(`/dev/zram0`, 4,6 G usados comprimidos a 2,4 G), o sea **RAM real, comprimida, ahora mismo**, con
341 MB libres.

**Y hay una guarda que ya existe y que estos dos caminos no consultan.** `cmd/musubi/turn.go:666`
pregunta `embedderCaroDeConstruir(cfg, root)` **antes** de construir, y degrada a léxico si la
construcción es cara. `runDaemon` (`main.go:466`) y `runServe` (`main.go:343`) llaman a
`resolveEmbedder` **sin ninguna guarda**. Es el patrón de la lección aprendida en N-1 de N caminos.

### La tercera salida, que es más barata que las dos que este documento comparaba

**Construcción PEREZOSA del embebedor en `runDaemon` y `runServe`.** Nada necesita un vector hasta
que alguien llame una tool semántica; hoy la tabla se carga **al arrancar**, siempre, en cada daemon.

Lo único que ata la construcción al arranque es la identidad:

```go
main.go:358  if embedding.Enabled(embedder) {
main.go:362      engine.SetVectorModelID(embedder.Name())
main.go:363      engine.WarnOnEmbedModelSwitch(embedder.Name())
```

**y esa identidad ya está persistida en la base**: `internal/memory/meta.go:118`,
`MetaEmbedModel = "embed_model_id"`, con `GetMeta`/`SetMeta`. Se puede leer **sin tocar la tabla**.

Construir perezoso borraría los ~2 GB de hoy **sin IPC nueva, sin mmap, y sin mitad Windows**. Es,
por lejos, la mejor relación entre lo que cuesta y lo que devuelve — y no la propuso nadie, ni yo.

### Y una corrección al encuadre del techo de 10 s

Este documento repite que el piso de 1013 ms «entra bajo el techo de 10 s». **El techo de 10 s es el
umbral de MATAR, no el presupuesto de experiencia.** El hook corre en `UserPromptSubmit` con una
persona esperando, y hoy cuesta **0,29 s**. Llevarlo a ~1,3 s es **4,5× en cada prompt**. «Entra» no
es lo mismo que «conviene», y acá se usaron como sinónimos.

Peor: el mmap **no toca** esa cifra. En la tabla de medición de arriba, `loadTokenizerBytes` mide
983 ms **en las dos columnas**. El piso es 97 % tokenizer, así que hasta que exista el caché binario
del tokenizer —lo que este documento llama «el único de los tres sin camino conocido»— el camino
mmap le deja al hook ~1 segundo por turno.

### Los dos caminos son ORTOGONALES, que es lo que ninguna versión anterior dijo

- **El mmap ataca las N copias de la tabla** (los ~2 GB medidos arriba). **No toca el costo del hook.**
- **El daemon ataca que un proceso efímero no puede pagar la construcción.** **No toca el costo de
  los N daemons**, que seguirían cargando la tabla igual.
- **La construcción perezosa ataca lo primero, hoy, y es la más barata de las tres.**

### Un número de este documento que se midió en caliente

El estado estacionario de esta máquina es **cache fría**: `mincore` sobre la tabla da **0 de 125.089
páginas residentes (0,00 %)**. Los números de mmap de más arriba se midieron con la cache caliente,
porque el mismo proceso acababa de leer el archivo.

En frío y medido: la identidad muestreada cuesta **~49 ms** (no 18-29), y tocar las ~120 filas que
consume un `Embed` real cuesta **~14 ms**. **Buena noticia igual, y hay que decirla: la tabla no es
el problema ni siquiera en frío.** El problema del mmap sigue siendo el tokenizer.

## Lo que hay en el repo por esto

- `internal/embedding/arranque_real_test.go` — desglose por etapa + la refutación del doble parseo.
- `internal/embedding/arranque_mmap_unix_test.go` — el piso con mmap, el costo de la identidad, y
  **la única aserción de la tanda**: que aliasar el blob como `[]float32` da valores **idénticos bit
  a bit** a convertirlo elemento por elemento (128.090.368 valores). Eso es lo que hace legítimo el
  atajo, y es justo el supuesto que compila, pasa, y devuelve basura en silencio si la máquina no es
  little-endian.

Ninguna de las dos aserta tiempos: los tiempos son de la máquina, no del código.
