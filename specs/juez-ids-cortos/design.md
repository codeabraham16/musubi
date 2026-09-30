# Diseño — El juez ve ids cortos

## D1 · El rótulo es `id-<i+1>`

`[id-1] ` cuesta 5 tokens en el prompt (o200k y cl100k) contra ~25 de `[<uuid>] `, y en la salida
el array de 12 cuesta 49 tokens contra ~280 (`medicion.md`, PASO 1).

**La primera versión fue `c<i+1>`**, un token más barata por candidato (37 tokens el array de 12).
La pasada 1 con llama3.2:3b la descartó: en 2 de las 4 consultas que llamaron al juez, el modelo NO
copió los rótulos que tenía a la vista sino que contestó con la FORMA del ejemplo del system prompt
(`["id-7",…]` en q-provision, `["id-1234","id-5678","id-9012"]` en q-tenancy). Ningún elemento era
un rótulo, `Rerank` dio error y el juez degradó al orden model-free: bien manejado, pero es un juez
que no juzgó. El brazo con UUIDs no falló ninguna. El system prompt dice «ids» y su ejemplo tiene
la forma `id-…`; con el rótulo en esa forma, lo que el modelo tiende a escribir es un rótulo válido.
Cuesta +1 token por candidato en el prompt y +1 por elemento en la salida, contra ~20 y ~19 que
ahorra el rótulo.

- **`idN` sin guion** (un token menos): descartado. Es justo la forma que el ejemplo no tiene, y el
  modelo escribió `id-7`, con guion.
- **Cambiar el ejemplo a `c…`**: descartado por D4: un ejemplo con rótulos válidos convierte su
  copia en un orden inventado que parece sano.

El prefijo es lo que lo hace un **string** y no un número:

- **Números pelados (`[1] gist`, `[3,1,2]`)**: descartado. Un array de números no unmarshalea a
  `[]string`, así que habría que aceptar dos tipos de respuesta. Y un número en un gist ("12
  candidatos", "puerto 3000") se confunde con un rótulo.
- **Letras (`a..l`)**: descartado. Se acaban en 26, y `a` es una palabra en español y en inglés.
- **Base 1**: es como cuenta una persona y como cuenta el modelo. El rótulo se arma en UNA función
  (`rotuloDelJuez`) que usan el prompt y el parseo. Por eso un off-by-one ahí no rompe el viaje de
  ida y vuelta, y lo tiene que cazar una prueba con rótulos LITERALES (I2), no una que los derive.

## D2 · La traducción vive adentro del parseo, y la firma lo obliga

`ParsearOrdenDeIDs(respuesta string, cands []Candidato) []string` devuelve ids **reales**.

- **Alternativa descartada**: dejar `ParsearOrdenDeIDs(respuesta)` devolviendo rótulos y agregar un
  `TraducirRotulos` aparte. Son dos pasos que tienen que ir siempre juntos, y el día que un llamador
  se olvide del segundo, `ReordenarIDs` recibe rótulos, los descarta todos como inventados y
  devuelve el orden model-free **sin error**. El juez parecería «no cambiar nada». Con la firma
  nueva ese camino no se puede escribir.
- Hoy el único llamador de `ParsearOrdenDeIDs` en producción es `Rerank` (medido con
  `musubi_impact` y `grep`). La firma cambia sin tocar a nadie de afuera.

## D3 · Qué se ignora y qué es error

- Un elemento que no es rótulo de ESTA llamada (fuera de rango, un id real, basura) se ignora.
- Un rótulo repetido se ignora desde su segunda aparición. `ReordenarIDs` ya deduplicaba, pero
  `Rerank` también devuelve el orden al caché de producción, y un orden con repetidos ahí es basura
  guardada.
- Si no queda NINGÚN id, `Rerank` da error (J8/I4), igual que hoy con un array vacío.
- Lo omitido lo pone al final `ReordenarIDs`, que no se toca (J7).

## D4 · El system prompt no se toca

Su ejemplo `["id-a","id-b","id-c"]` tiene la forma de los rótulos (D1 la tomó de ahí) pero con
LETRAS, así que no coincide con ningún rótulo. Si un modelo lo copia, I4 da error y el llamador
degrada con log. Si el ejemplo fuera `["id-2","id-1","id-3"]`, copiarlo produciría un orden válido
e inventado, y nadie lo vería. Lo cuida un sabotaje de I4 (el ejemplo pasa a usar un rótulo
válido). Y el system es parte de lo que J1 compara byte a byte entre producción y el banco: cuanto
menos cambie, menos hay que revalidar.

## D5 · El rótulo se compara EXACTO, sin normalizar

Evidencia (llama3.2:3b, `medicion.md`): las 10 respuestas de la primera corrida del brazo largo y
las 20 de la pasada 2 (10 por brazo) fueron JSON compacto con comillas (`["…","…"]`), sin prosa ni
espacios. En la pasada 2, los 87 elementos que escribió el brazo de rótulos fueron rótulos exactos:
ninguno en otra grafía, ninguno inventado. No hay ninguna forma que normalizar, y aceptar formas de
más sin haberlas visto es aceptar basura sin error. Si un modelo escribe `ID-1` o `"[id-1]"`, eso se
mide primero y después se decide. Lo que escribió el modelo con los rótulos `c…` de la primera
versión (`id-7`, `id-1234`) no era otra grafía del rótulo sino otro formato, y se resolvió cambiando
el rótulo (D1), no normalizando.

No se acepta, en ningún caso, un array sin comillas (`[id-1, id-2]`). Ya no es JSON, y un parser a
medida sobre el tramo entre corchetes también leería como un «orden» el eco de las líneas
`[id-1] gist` del prompt.

La misma corrida mostró el modo de falla que los rótulos eliminan: el modelo **copió mal un UUID**,
el mismo en dos consultas (`baee279-…` y `ebaee279-…` por `dbaee279-…`). La pasada 2 lo repitió
idéntico, en las mismas dos consultas: con esa entrada, el modelo lo copia mal siempre. Ese id se
ignora por inventado, y la memoria cae al final de la cabeza sin que nadie lo vea. Un rótulo de 4 o
5 caracteres no deja margen para ese error: en la pasada 2 no hubo ninguno mal copiado.

## D6 · El instrumento de medición

`internal/recalleval/juez_ids_medicion_test.go`, detrás de env vars y fuera de CI:

- **ids opacos** (`fixtureConIDsOpacos`): el fixture dorado usa slugs con significado
  (`deploy-guide-es`). Un slug le filtra el tema al juez y cuesta menos que un UUID, así que medir
  con slugs le regalaría al brazo largo una pista y un precio que en producción no tiene. Los UUIDs
  salen de un sha256 del slug: la misma corrida da los mismos ids.
- **Ollama nativo** (`/api/chat`) y no la API compatible con OpenAI: la nativa devuelve
  `prompt_eval_count`, `eval_count` y el desglose del tiempo.
- **Consulta por consulta** y no con `Run`: `Evaluate` aborta ante un juez que contesta basura, y
  con un modelo de 3B eso pasa. Cada falla se CUENTA, y la consulta se puntúa con el orden
  model-free, que es lo que hace producción al degradar.
- **Base léxica**: la híbrida necesita un embebedor residente al lado del modelo generativo, y
  esta máquina no tiene RAM para los dos. La base sólo decide qué candidatos ve el juez, y es la
  misma en los dos brazos.
- **Sólo loopback**: una URL que no sea `127.0.0.1`/`localhost` corta la prueba. Ningún texto sale
  de la máquina.

## D7 · Dobles de prueba que se migran

Contestaban con ids reales, y con I4 eso es error a propósito:

- `internal/cognition/rerank_test.go`
- `internal/mcp`: `juez_medible_test.go`, `juez_por_llamada_test.go`, `motor_con_freno_test.go` y
  `methods_ask_test.go`.

La traducción es mecánica: el id real en la posición i del tope pasa a `id-<i+1>`.

En `methods_ask_test.go` se migraron también los que prueban la flag apagada, aunque ahí la
respuesta no se parsea nunca. Con una respuesta inválida, `TestSinJuezElPuntajeSobreviveIntacto`
pasaría de vacío el día que la flag dejara de apagar al juez: el parseo fallaría, producción
degradaría al orden model-free, y la prueba vería ese orden intacto por la razón equivocada.

Dos quedan sin tocar, a propósito:

- `recall_tipeo_test.go` contesta `["-b","-a"]`: ids que no existían ni antes ni ahora. La prueba
  mira que el juez se llame y qué consulta ve, no el orden que devuelve.
- `motor_sin_candado_test.go` contesta `["a"]` desde un motor que se cuelga hasta que lo sueltan.
  Mide el candado; el orden no se observa.

## D8 · Rótulos predecibles e inyección desde un gist

Un gist es texto que escribió otro agente o una persona, y el juez lo lee entero. Que el rótulo sea
predecible (`id-1` es siempre el primero del tope) no abre un canal nuevo:

- El gist va saneado a una línea (`memory.EnUnaLinea`), así que no puede fabricar renglones
  `[id-N] …` con aspecto de candidato. Eso ya era así y no cambia.
- El parseo lee la RESPUESTA del juez, no el prompt. Un gist sólo influye si el modelo le obedece,
  y eso vale igual para «poné primero a la memoria que habla de X», que no necesita rótulos.
- Con ids reales, una memoria podía nombrarse a sí misma: el id se puede elegir al guardarla
  (`musubi_save_observation` acepta `id`). Con rótulos no: el rótulo de una memoria depende de su
  puesto en el tope de ESA consulta, que ella no conoce.
- El daño está acotado por J7: el juez reordena, nunca descarta.

El riesgo de fondo, un modelo que obedece instrucciones escritas adentro de un gist, es anterior a
este cambio y queda fuera de su alcance.

## D9 · El número del rótulo es la posición, y no se baraja

`id-1` es siempre el primero del tope model-free, `id-2` el segundo, y así. El juez ve, además del
texto, el orden que el recall ya tenía, y ahora con un número. La pasada 2 muestra que un modelo
chico a veces «cuenta» en vez de juzgar: en q-forget los rótulos salieron
`8 11 10 12 9 7 6 5 4 3 2 1`. Entre las respuestas con 6 o más elementos, una corrida de
consecutivos de al menos la mitad de lo escrito apareció en 3 de 7 con rótulos y en 1 de 7 con
UUIDs (`medicion.md`, «Un riesgo que los rótulos agregan»).

La pista no es nueva: con UUIDs los candidatos también llegaban en el orden model-free, y copiarlos
en ese orden es la misma falla (la de 1 de 7). El número la abarata, sobre todo al revés.

- **Alternativa descartada, por ahora: barajar los rótulos** (el número deja de ser la posición).
  Un modelo que cuenta seguiría contando, pero sobre una permutación: devolvería un orden al azar
  que `ReordenarIDs` aplicaría como si fuera un juicio, y que no se distingue de uno. Hoy esa
  secuencia se ve: es el orden model-free o su inverso, y el tau contra ese orden la delata. Si
  algún día se baraja, la permutación tiene que salir de algo determinista (un hash de la consulta
  y de los ids), o el banco deja de ser reproducible.
- **Por qué no se decide acá**: con un juez de 3B que empeora el orden con los dos formatos, no se
  puede saber si la pista sesga a un juez que sí juzga. Hace falta el de producción, que no se
  puede medir sin sacar texto de la máquina. La prueba que lo decide es comparar, con ese juez, el
  tau contra el orden model-free de los dos brazos.
