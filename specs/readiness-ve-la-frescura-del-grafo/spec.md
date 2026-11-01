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

- **R3** — Los checks de frescura **DEBEN** ser proporcionales con un umbral declarado, no binarios.
  Un solo archivo recién creado y todavía sin indexar no puede tirar abajo la dimensión. Es la
  convención que el resto de `readiness` ya usa (`«69/2580 fallaron (2.7 %, tope 5 %)»`), y el
  umbral **DEBE** viajar en el texto del check para que se pueda discutir.

- **R4** — Cuando la frescura **NO SE PUEDE MEDIR** —el árbol está fuera de alcance, que es el caso
  del cerebro central, donde el grafo es federado y sus archivos no están en disco— la dimensión
  **DEBE** declararlo en la evidencia y **NO DEBE** agregar los checks de frescura. Un check que
  pasa porque no se midió nada sube el puntaje por no mirar: es exactamente el cero ambiguo que
  `profundidadDe` ya documentó como *«el valor de fallo disfrazado de valor tranquilizador»*.

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
  > Es **exactamente** el caso 0/0 y no uno más amplio: con cero en el grafo y archivos en disco,
  > `missing/enDisco` ya da 100 % y el check cae en rojo solo; con archivos en el grafo y cero en
  > disco, todos salen fantasmas y también. Los cuatro cuadrantes, revisados uno por uno.

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
- **Then** la dimensión `grafo` sigue puntuando **1.0**, ahora con cinco checks en verde en vez de
  dos, y la evidencia trae los tres contadores en cero

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
- [ ] **¿Qué umbral?** R3 pide umbrales proporcionales; el valor concreto lo fija `design`. El resto
  de `readiness` usa 5 % para errores y rechazos, pero `missing` y `stale` no son lo mismo entre sí
  —un archivo nuevo sin indexar es rutina, un archivo fantasma es un borrado que nadie reconcilió—
  así que pueden no compartir umbral.
- [ ] **¿Tres checks o uno?** Afecta el puntaje: `puntaje()` es la fracción de checks cumplidos, así
  que agregar tres a los dos existentes cambia el peso de cada uno. Lo decide `design`.

## Nota de método

Durante la medición del costo, un script propio contó **1 archivo fantasma** mientras `musubi_map`
reportaba `ghosts: 0`. Perseguido hasta el final: hay un nodo en `code_graph_nodes` con `path = ''`,
y `filepath.Join(root, "")` devuelve el directorio raíz, que falla al leerse como archivo. **La
herramienta tenía razón y el script estaba mal**: `GraphFileFingerprintsCtx` filtra con
`WHERE path != ''`, así que ese nodo nunca entra al cómputo. Queda anotado porque la próxima
medición artesanal sobre `code_graph_nodes` va a tropezar con lo mismo.
