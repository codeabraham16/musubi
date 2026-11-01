---
artifact: spec
schema_version: "1.0"
change: readiness-ve-la-frescura-del-grafo
status: draft
---

# Especificación — la dimensión `grafo` de `musubi_readiness` mide si el índice sirve, no sólo si existe

## Requisitos

- **R1** — La dimensión `grafo` de `musubi_readiness` **DEBE** incluir, además de los dos checks de
  volumen que ya tiene, al menos un check que falle cuando el índice esté desactualizado respecto
  del disco. Hoy ningún check de ninguna dimensión mira la frescura del grafo.

- **R2** — La evidencia de la dimensión `grafo` **DEBE** incluir los tres contadores de frescura
  (`stale`, `ghosts`, `missing`) con sus valores crudos, igual que ya expone `nodos` y `aristas`.
  Es la regla que el propio `musubi_readiness` declara en su `note`: *«la evidencia de cada
  dimensión son los números crudos»*.

  > ⚠️ **Y la contracara, que esta spec NO escribió: donde la frescura no se midió, sus contadores
  > NO se publican.** Un `ghosts: 0` al lado de `frescura_no_medida` se lee como «se midió y está
  > sano», que es lo contrario de lo que pasó. Así que el **DEBE** de arriba rige en la rama que
  > mide, y en cualquier rama que publique `frescura_no_medida` **se omiten los cinco** — los tres
  > contadores y los dos denominadores.
  >
  > Los cinco, y no sólo los tres, porque en la rama del 0/0 esos ceros son **el cero ambiguo** que
  > todo este cambio existe para desarmar: `graphFreshness` tiene un retorno temprano por falla de
  > la consulta que ocurre **antes** de caminar el disco, así que por esa vía `archivos_en_disco: 0`
  > no es una medición sino un valor de fallo. Desde `enriquecerGrafo` ese caso es indistinguible
  > del proyecto que de verdad no tiene nada, así que el diagnóstico va en el **texto**, que nombra
  > las dos cosas que pueden haber pasado, y no en un número que un consumidor automático lee como
  > «cero problemas». Que `graphFreshness` distinga «falló la consulta» de «cero» —un retorno más, o
  > un error— queda **fuera de alcance**: es un cambio de firma que esta spec no previó.
  >
  > Sin esta cláusula las dos ramas de «no medida» publicaban formas DISTINTAS —la de alcance sólo
  > la declaración, la del 0/0 los cinco ceros— y la regla vivía únicamente en el comentario de
  > `I4`: un contrato escrito por un test en vez de por la spec, que es como se vuelve invisible.
  > Hoy la cumplen las dos, y `I9` la custodia con su propio sabotaje.

- **R3** — Los checks de frescura **DEBEN** ser proporcionales con un umbral declarado, no binarios.
  Un solo archivo recién creado y todavía sin indexar no puede tirar abajo la dimensión. Es la
  convención que el resto de `readiness` ya usa (`«69/2580 fallaron (2.7 %, tope 5 %)»`), y el
  umbral **DEBE** viajar en el texto del check para que se pueda discutir.

- **R4** — Cuando la frescura **NO SE PUEDE MEDIR** —el árbol está fuera de alcance, que es el caso
  del cerebro central, donde el grafo es federado y sus archivos no están en disco— la dimensión
  **DEBE** declararlo en la evidencia y **NO DEBE** agregar los checks de frescura. Un check que
  pasa porque no se midió nada sube el puntaje por no mirar: es exactamente el cero ambiguo que
  `profundidadDe` ya documentó como *«el valor de fallo disfrazado de valor tranquilizador»*.

  > **Calificador — el motivo que se publica tiene que ser el de ESTA rama, y no cualquiera.**
  > R4 y R8 publican las dos la misma clave, `frescura_no_medida`, con textos distintos, y están en
  > serie: si el flujo le llega, el de R8 **sobrescribe** el de R4. Y los dos textos mandan al
  > operador a lados opuestos — «el árbol no está en este disco» no tiene remedio; «no hay un solo
  > archivo contra el que medir» dice que indexe—, así que publicar el equivocado manda a trabajar
  > a quien no tiene trabajo.
  >
  > Esto salió de medir, no de leer: el invariante está protegido por **dos** guardas de
  > `arbolFueraDeAlcance()`, una en `enriquecerGrafo` y otra dentro de `graphFreshness`, así que
  > quitarle el `return` a la primera **no deja pasar el check** —la segunda vuelve con los cinco
  > en cero y el flujo sale por R8— y el custodio de R4 quedaba **VERDE bajo sabotaje**. Lo cazó
  > `arnes -overlay`; `-validar` no lo ve, porque la directiva apuntaba a un literal que existe y
  > es único. Un invariante con dos guardas no se custodia mirando lo que las dos comparten: se
  > custodia mirando lo que las distingue.

- **R5** — Los dos checks de volumen existentes (`hay símbolos indexados`, `hay relaciones, no sólo
  una lista`) **DEBEN** seguir intactos, con su mismo `Name` y su misma semántica. El contrato de
  `musubi_readiness` es consumido por la cabina y el CRM; esto es aditivo.

- **R6** — `musubi_readiness` **DEBE** seguir siendo read-only y **NO DEBE** cambiar su
  clasificación en `registry.go`. Su llamador declarado (la cabina, `write=none`) tiene que poder
  seguir invocándola.

- **R7** — El **comportamiento** de `graphFreshness` —su cómputo, su fail-safe
  `arbolFueraDeAlcance()` y su política de no inventar podredumbre— **NO DEBE** cambiar. Lo que este
  cambio arregla es quién LEE la señal, no la señal.

  > **Ajustado en `design` (D5), y queda registrado acá porque la versión anterior de este requisito
  > decía «`graphFreshness` no debe modificarse» a secas.** Eso chocaba con R3: una proporción
  > necesita denominador, y los denominadores de los tres ejes son distintos (`stale` y `ghosts` se
  > miden sobre los archivos del grafo; `missing`, sobre los indexables del disco). Las tres salidas
  > eran recalcularlos —duplicar el costo para obtener dos números que la función acaba de tirar—,
  > escribir un helper al lado —dos formas de preguntar lo mismo en el mismo archivo, contra lo que
  > el propio comentario de `graphFreshness` advierte— o **ampliar la firma**, que es aditiva y no
  > toca el cuerpo. Se eligió la tercera. R7 protege la semántica, que es lo que importaba; la firma
  > no era el invariante.

- **R8** — Cuando **NO HAY UN SOLO ARCHIVO contra el que medir** —cero archivos en el grafo y cero
  indexables en disco— la dimensión **DEBE** declararlo en la evidencia y **NO DEBE** agregar el
  check de frescura, *aunque la dimensión esté observada*.

  > **Apareció en `verify`, leyendo el diff del propio cambio; no estaba ni en la spec ni en el
  > design.** Es la **segunda puerta** al premio que R4 cierra, y la guarda de R4 no la cubre porque
  > mira `Observed`: `GraphStatsCtx` cuenta **todos** los nodos, pero `GraphFileFingerprintsCtx`
  > filtra `WHERE path != ''` ([codegraph.go](../../internal/memory/codegraph.go)), así que un grafo
  > cuyos nodos no declaran archivo da `nodos > 0` —la dimensión queda observada— con cero archivos
  > para comparar. **No es hipotético: este repo tiene hoy un nodo con `path = ''`**, encontrado
  > midiendo el costo de esta misma función. El mismo estado llega por otra vía: si la consulta de
  > fingerprints falla (base bloqueada), `graphFreshness` devuelve todo en cero, **los dos
  > denominadores incluidos**. Con los dos en cero las tres proporciones valen 0 por construcción y
  > el check pasaba en VERDE con el `Why` «0/0 desactualizados (0.0 %) · 0/0 fantasmas (0.0 %) · 0/0
  > sin indexar (0.0 %) · tope 5 %» — medido, no inferido.
  >
  > Es **exactamente** el caso 0/0 y no uno más amplio, pero **el motivo no es el que esta spec
  > escribió primero**. Los cuatro cuadrantes, ahora MEDIDOS uno por uno en `verify`:
  >
  > | `enGrafo` | `enDisco` | Qué pasa | Medido |
  > |---|---|---|---|
  > | > 0 | > 0 | los tres ejes se miden; rojo si el peor pasa el tope | sí |
  > | 0 | > 0 | `missing/enDisco` da 100 % y **cae en rojo solo** | sí |
  > | > 0 | 0 | **DEPENDE** — ver abajo | sí |
  > | 0 | 0 | las tres proporciones valen 0 por construcción y el check pasaba en VERDE | sí |
  >
  > ⚠️ **El cuadrante (grafo > 0, disco = 0) NO cae en rojo solo: puede dar VERDE.** Esta spec
  > afirmaba que «todos salen fantasmas y también», y es FALSO. `ghosts` se cuenta leyendo cada
  > archivo del grafo **directamente del disco**, mientras `enDisco` sale de `walkSourceTree`, que
  > **saltea** `testdata`, `vendor`, `node_modules`, `dist`, `coverage` y todo directorio con punto
  > ([methods_codegraph.go](../../internal/mcp/methods_codegraph.go)). Un archivo del grafo que vive
  > en un directorio salteado **existe, se lee y coincide**: `ghosts` = 0, `stale` = 0, y
  > `missing` = 0 por loop vacío. Medido: `Pass=true`, score **0.67**, con `archivos_en_disco: 0`.
  > La segunda vía está documentada en el propio repo: `IndexableForGraph` depende de los build
  > tags, así que un binario sin treesitter sobre un repo TS mide `enDisco = 0` con el grafo lleno.
  > La afirmación original **sí** vale para el archivo BORRADO —medido: 1/1 fantasmas, 100 %, rojo,
  > score 0.33— y es falsa para el «legible pero no caminado».
  >
  > Eso **no mueve la guarda**, y conviene decir por qué: en ese cuadrante los dos ejes que tienen
  > un denominador real **sí se midieron** y dieron 0, así que el verde no es el premio por no
  > mirar que R8 existe para impedir. Lo que estaba mal era la justificación, no la condición.

## Escenarios

### Escenario: el grafo está podrido y la dimensión deja de ser perfecta
- **Given** un proyecto con el grafo indexado (nodos > 0, aristas > 0) y con archivos cuyo
  fingerprint en disco ya no coincide con el guardado
- **When** se llama `musubi_readiness`
- **Then** la dimensión `grafo` puntúa **menos de 1.0**, su evidencia trae `stale` > 0, y el check
  que falló dice cuántos sobre cuántos y contra qué umbral

### Escenario: el grafo está sano y nada cambia para bien ni para mal
- **Given** este repo tal como está hoy, medido: `stale:0, ghosts:0, missing:0`, 16.787 nodos,
  42.540 aristas
- **When** se llama `musubi_readiness`
- **Then** la dimensión `grafo` sigue puntuando **1.0**, ahora con **tres** checks en verde en vez
  de dos —`D1` decidió UN chequeo compuesto y no tres—, y la evidencia trae los tres contadores en
  cero

### Escenario: hay archivos que el grafo nunca vio
- **Given** un proyecto donde una parte del árbol es indexable y no tiene un solo nodo —el caso
  medido el 2026-09-05 en este mismo repo: 213 de 798 `.go` (26,7 %), `internal/fleet` entero
- **When** se llama `musubi_readiness`
- **Then** la dimensión `grafo` **NO** puntúa 1.0, y la evidencia dice `missing: 213`. Hoy ese
  mismo estado puntúa 1.0, porque `nodos > 0` y `aristas > 0` se cumplen igual

### Escenario: la frescura no aplica y la dimensión no finge haberla medido
- **Given** un cerebro central, donde `arbolFueraDeAlcance()` es verdadero y los archivos del grafo
  federado no están en el disco local
- **When** se llama `musubi_readiness`
- **Then** la dimensión `grafo` conserva **exactamente** sus dos checks de volumen, **no** agrega
  los de frescura, y la evidencia declara por qué no se midió
- **Y** su puntaje es el mismo que da hoy — no sube por no haber mirado, ni baja por una podredumbre
  inventada

### Escenario: un archivo del grafo fue borrado del disco
- **Given** un proyecto donde un archivo indexado ya no existe o no se puede leer
- **When** se llama `musubi_readiness`
- **Then** la evidencia trae `ghosts` > 0 y el check correspondiente lo refleja

### Escenario: la dimensión está observada y no hay nada que medir
- **Given** un grafo cuyos nodos no declaran archivo (`path = ''`), de modo que `nodos > 0` y la
  dimensión queda **observada**, y un árbol en disco sin un solo archivo indexable
- **When** se llama `musubi_readiness`
- **Then** la dimensión **no** agrega el check de frescura, y la evidencia declara que no hubo nada
  contra qué medir
- **Y** su puntaje es el de los checks que sí tiene: el check ausente no se cuenta como cumplido

## Fuera de alcance

- Agregar un check del grafo a `musubi_doctor`. Sus 16 checks no cubren el grafo de código
  (`anchor_coverage` es de anclas de observaciones; los `stale_*` son de gists y conflictos). Es un
  hueco real, pero es otra superficie y otro contrato —el doctor expone `Repair`, y «reparar» un
  grafo stale significa re-indexar—, así que va al plan siguiente.
- Re-indexar automáticamente al detectar podredumbre. Esta spec hace que la señal se VEA; actuar
  sobre ella es una decisión aparte.
- Los otros cabos abiertos: TP1, TP2, TP3, TP5.

## Preguntas abiertas

- [x] **¿Cuánto cuesta?** Resuelto por medición antes de diseñar. `graphFreshness` lee cada archivo
  del grafo entero y lo sha256ea (`FileFingerprint` = `os.ReadFile` + `sha256`), más un
  `walkSourceTree()`. Medido sobre este repo: **1.395 archivos, 17,9 MB, ~0,26 s de hashing +
  ~0,03 s de walk ≈ 0,28 s**, y eso en Python, o sea un techo — Go es más rápido en las dos
  mitades. Para una tool de diagnóstico que no está en el camino caliente, es aceptable. **Decisión:
  se calcula siempre, sin flag.** Si algún día duele, el número está acá para discutirlo.
- [x] **¿Qué umbral?** **Resuelto por `D2`: 5 %, el mismo para los tres ejes, y DERIVADO de los
  commits reales de este repo** —mediana 4 archivos (0,29 % del grafo), p90 15, p95 22, máximo
  medido 65 (4,66 %)—, no elegido a ojo. La duda era si `missing` y `stale` debían compartir umbral,
  porque no son lo mismo: un archivo nuevo sin indexar es rutina, un fantasma es un borrado que
  nadie reconcilió. La decisión fue UNO solo, y el motivo es `D1`: el chequeo es uno y mira el PEOR
  de los tres, así que separar los topes sin separar los chequeos no cambiaría ningún veredicto.
- [x] **¿Tres checks o uno?** **Resuelto por `D1`: UNO compuesto.** Afectaba el puntaje y así quedó:
  `puntaje()` es la fracción de checks cumplidos, así que la dimensión pasa de 2 a 3 chequeos y cada
  uno vale 1/3 en vez de 1/2 —un proyecto con nodos y sin aristas sube de 0.5 a 0.67—. Con tres la
  distorsión sería peor (0.5 → 0.8). Y el motivo de fondo no es el peso: los tres ejes se curan con
  el MISMO acto, así que tres chequeos sugerirían tres remedios donde hay uno.
- [ ] 🔴 **¿Qué hace el chequeo cuando un eje no tiene denominador? — DECISIÓN DEL DUEÑO, abierta a
  propósito.** No la previó ningún requisito y la midió el panel en `verify`: con archivos en el
  grafo y **cero** indexables en disco —un `.go` que vive en un directorio que `walkSourceTree`
  saltea, o un binario sin los build tags de treesitter sobre un repo que no sabe indexar— el eje
  `missing` divide por cero, aporta 0 % **sin haber medido nada**, y el chequeo pasa en VERDE con
  `archivos_en_disco: 0` a la vista. Los otros dos ejes sí se midieron y dieron 0, así que el verde
  es DEFENDIBLE —los dos lentes que lo miraron coinciden en que no es una violación—, y por eso
  este PR **no cambia la conducta: la documenta como está medida**. Las dos salidas honestas, y la
  elección es del dueño: **(a)** mantener el chequeo sobre los ejes que sí tienen denominador y
  DECLARAR en la evidencia y en el `Why` que `missing` no era medible —no tira información real, y
  es lo que recomienda el lente de contrato—; **(b)** ampliar la guarda a `enDisco == 0` y no medir
  frescura en ese estado, al precio de perder los dos ejes que sí se podían medir.

## Nota de método

Durante la medición del costo, un script propio contó **1 archivo fantasma** mientras `musubi_map`
reportaba `ghosts: 0`. Perseguido hasta el final: hay un nodo en `code_graph_nodes` con `path = ''`,
y `filepath.Join(root, "")` devuelve el directorio raíz, que falla al leerse como archivo. **La
herramienta tenía razón y el script estaba mal**: `GraphFileFingerprintsCtx` filtra con
`WHERE path != ''`, así que ese nodo nunca entra al cómputo. Queda anotado porque la próxima
medición artesanal sobre `code_graph_nodes` va a tropezar con lo mismo.
