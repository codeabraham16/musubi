# Diseño — La bandera de redacción son tres preguntas

## Dimensión real del cambio (medida, y es más chica de lo que parecía)

El **campo** `s.forceRedact` se toca en **cinco** lugares de código. Los demás matches de un
grep son comentarios:

| Lugar | Qué es |
|---|---|
| `server.go:363-367` | la declaración del campo |
| `http.go:934` | la **única** escritura |
| `methods.go:326` | lectura — el scope forzado (pregunta #2) |
| `methods.go:2467` | lectura — dentro de `redactIfForced` (pregunta #1) |
| `methods_codegraph.go:354` | lectura — dentro de `arbolFueraDeAlcance` (pregunta #3) |

Los «22 + 1 + 5 = 28 sitios» del proposal **no hay que tocarlos**: todos pasan por los dos
helpers y por ese único `if`. El repo ya había hecho el trabajo de encapsular cada
consumidor; lo que faltaba era que la fuente dejara de ser una sola.

## D1 — El medidor del árbol: la firma del `MUSUBI_HOME` del central

**Decisión:** un `os.ReadDir` del nivel superior de `projectPath`. El árbol está **ausente**
sólo cuando el directorio contiene **exactamente `.musubi` y nada más** — que es la firma
del cerebro central. Todo lo demás, incluido un directorio **vacío**, cuenta como presente.

```go
// arbolAusenteEn dice si el grafo de este cerebro NO puede estar describiendo código de este
// disco. Devuelve true SÓLO para la firma del cerebro central: un MUSUBI_HOME que contiene
// exactamente `.musubi` y nada más (medido en vivo el 2026-10-31 contra
// /home/musubi/musubi-brain: cero entradas fuera de `.`, `..` y `.musubi`).
//
// ⚠️ UN DIRECTORIO VACÍO CUENTA COMO PRESENTE, y no es un descuido: el contrato ya está
// declarado en codegraph_stale_central_test.go, donde una instancia LOCAL con un t.TempDir()
// vacío y un nodo cuyo archivo no existe DEBE reportarse stale —«un fantasma legítimo»—. Un
// directorio sin nada no es un cerebro federado: es un proyecto que no se indexó todavía.
//
// Y ANTE LA DUDA DICE «PRESENTE», a propósito. Equivocarse hacia «presente» es ruidoso y se
// ve: graphFreshness empieza a contar fantasmas en masa y salta en el primer readiness.
// Equivocarse hacia «ausente» apaga la medición EN SILENCIO, y el indicador que uno miraría
// para notarlo —`ghosts: 0`— es exactamente lo que devuelve un grafo que dejó de mirar.
func arbolAusenteEn(dir string) bool {
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) == 0 {
		return false // ilegible o vacío ⇒ presente: la falla ruidosa es la buena
	}
	for _, e := range ents {
		if e.Name() != ".musubi" {
			return false // hay contenido propio ⇒ el árbol puede estar acá
		}
	}
	return true
}
```

> 🔴 **CORRECCIÓN DE LA FASE `verify` (2026-10-31).** El bloque de arriba quedó como historia: el
> panel adversarial refutó las dos decisiones que documenta, y lo que se implementó es otra cosa.
>
> **El criterio era circular.** «Exactamente `.musubi` y nada más» ES la firma del central, pero
> también es lo que el propio arranque del daemon planta en cualquier carpeta nueva
> (`ensureWorkspace` corre antes de `ListenAndServeHTTP`), así que la carpeta recién iniciada de
> cualquier máquina entraba en la rama del central. Y al revés: un `nohup.out` al lado del
> `.musubi` del central lo sacaba de esa rama. El criterio que quedó es **«no hay archivos
> indexables»**, con las mismas reglas que `walkSourceTree` —mismo `IndexableForGraph`, mismos
> directorios salteados— y un corte temprano en el primer indexable, así que en el caso común es
> O(1).
>
> **Y el directorio vacío cambió de lado.** Hoy cuenta como **ausente**, que es lo contrario de lo
> que afirma el comentario de arriba. Sólo es sostenible porque el hecho dejó de medirse una sola
> vez: `remedirArbol` corre en los dos modos del índice —`indexAllPackages` y
> `planearIncremental`—, o sea cada vez que el grafo vuelve a leer el disco, que es exactamente
> cuando el hecho pudo cambiar. El fail-safe «ante la duda, presente» se conserva donde sigue
> siendo cierto: una raíz **ilegible**. El contrato de `codegraph_stale_central_test.go` sigue
> verde porque ese test construye con `NewMcpServer` y usa el cero-value del campo.
>
> **Y la frase «salta en el primer readiness» era falsa.** `graphFreshness` no está conectado a
> `musubi_readiness`: la alarma que esa oración promete **no existe**. La asimetría sigue en pie
> —errar hacia presente es ruidoso y errar hacia ausente es silencioso— pero el síntoma del lado
> ruidoso se ve en `musubi_map`, no en un readiness. Conectar las dos cosas quedó como cabo para
> el plan siguiente (**TP4**).
>
> **Y LA TABLA DE ALTERNATIVAS DE ARRIBA QUEDÓ DESCALZADA, que es lo que pasa cuando cambia el
> criterio y no la tabla.** La fila (c) se descartó **por depender de los build tags de
> treesitter**, y el criterio que quedó —«no hay archivos indexables»— llama
> `codeintel.IndexableForGraph`, o sea que depende de ellos: la fila (d) ya no es «un `ReadDir`,
> sin recursión, sin depender de build tags», porque ese diseño es justo el que `verify` tiró.
> El borde está declarado en el comentario de `arbolAusenteEn`, y se acepta con un argumento
> que la fila (c) no tenía: un binario capado tampoco puede **poblar** el grafo con esos
> archivos, así que el próximo índice poda los nodos que dejarían de medirse y la ventana se
> cierra sola. Para `.go` el caso no existe: `IndexableForGraph` devuelve `true` sin mirar
> tags. Queda dicho acá porque una tabla que justifica un descarte con un criterio que el
> cambio después adopta es exactamente la clase de contradicción que nadie vuelve a leer.
>
> **Y LA LECCIÓN DE MÉTODO, que es la que se lleva el próximo diseño.** Los cinco tests que
> esta fase diseñó estaban **todos** construidos sobre `force_redact: true`. Es entendible —es
> la configuración donde vive el defecto original— y es también la razón por la que el hecho
> congelado no se vio: la regresión **no tiene nada que ver con la redacción**, así que ninguno
> de los cinco podía tropezarse con ella. Dos de los tres jueces, por separado, pidieron el
> MISMO test que faltaba y los dos le especificaron `ForceRedact: false`. Un eje que todos los
> tests de un cambio fijan en el mismo valor es un punto ciego **con la forma del cambio**, y
> no lo encuentra releyendo quien lo diseñó. El custodio final cruza política × modo: cuatro
> subtests, y el que importa es `sin_redaccion`.


Los cuatro casos reales, y los cuatro salen bien:

| `projectPath` | Medidor | Correcto porque |
|---|---|---|
| `/home/musubi/musubi-brain` (sólo `.musubi`) | **ausente** | es el central: grafo federado, sin fuentes |
| `t.TempDir()` vacío (`newTestServer`) | **presente** | preserva el «fantasma legítimo» ya declarado |
| `t.TempDir()` + `go.mod` + `pkg/a.go` (`proyectoIndexado`) | **presente** | hay fuentes y están indexadas |
| un repo clonado | **presente** | el caso del daemon local |

Si algún día el `MUSUBI_HOME` del central gana un archivo suelto, el medidor dirá «presente»
y el síntoma será ruidoso (fantasmas en masa), no silencioso. Esa es la dirección elegida.

### Alternativas descartadas

| # | Alternativa | Por qué se descarta |
|---|---|---|
| (a) | `hayRepoGit(projectPath)`, que **ya existe** en el paquete | **Descartada por medición.** `proyectoIndexado` arma su proyecto en `t.TempDir()`, que no está bajo ningún repo git: `hayRepoGit(os.TempDir())` = `false`, con `hayRepoGit(<worktree>)` = `true` como control. Apagaría la medición en toda la suite de codegraph y la dejaría verde sin medir |
| (b) | `walkSourceTree()` no vacío | Exacta, pero recorre el árbol entero y `cgStale` se llama **por nodo** |
| (c) | (b) una vez al arranque, cacheada | Era la que la spec dejaba en pie, y **falla en la dirección silenciosa**: depende de `codeintel.IndexableForGraph`, que para TS/JS/Py cuelga de `polyglotSupported` y por lo tanto de los **build tags de treesitter**. Un binario capado diría «no hay árbol» en un repo de puro TypeScript y apagaría la frescura sin aviso |
| (d) | **la elegida** | Un `ReadDir`, sin recursión, sin depender de build tags, y el modo de falla es el ruidoso |

**Cuándo se mide:** 🔴 **esto cambió en `verify`, y el párrafo original quedó como historia.**
Se mide **dos veces**: en el arranque (`http.go`, donde hoy se asigna `forceRedact`) y otra vez
en cada índice (`remedirArbol`, en los dos modos). Lo que decía era «una vez, en el arranque»,
y aceptaba como costo honesto que «no ve un `git clone` posterior al arranque» —en el central
no importa, `MUSUBI_HOME` no cambia, y en un cliente local el daemon se reinicia con la
sesión—. **El panel adversarial midió que ese costo no era aceptable**, por una razón que la
decisión original no podía ver todavía: con el criterio nuevo —«no hay archivos indexables»—
una carpeta de trabajo **normal** cae del lado «ausente», así que un `musubi init` antes del
`git clone` deja la frescura apagada para toda la vida del daemon. Y el re-medidor no es un
reloj: cuelga del índice, que es exactamente el momento en que el hecho pudo cambiar y en que
el recorrido del disco ya está pago.

### La polaridad del campo no es cosmética

El campo se llama `arbolAusente`, **no** `arbolEnDisco`, y la razón es el cero-value. Los
tests construyen el server con `NewMcpServer`, que **no pasa por `http.go`**, así que el
campo se queda en el valor cero de `bool`:

| Campo | Cero-value significa | Consecuencia en los tests que no lo asignan |
|---|---|---|
| `arbolEnDisco` | «no hay árbol» | el predicado **se invierte** en toda la suite, y hacia el lado silencioso |
| `arbolAusente` | «hay árbol» | **idéntico al comportamiento de hoy**, donde `forceRedact` arranca en `false` |

Con `arbolEnDisco` haría falta recordar inicializarlo en cada camino de construcción, y el
día que apareciera un camino nuevo el defecto sería silencioso. Con `arbolAusente` el
descuido es inofensivo.

## D2 — La #2 no cuelga de `TeamMode`: su contrato lo prohíbe

El proposal propuso colgar el forzado de scope de `s.memory.TeamMode`. **Se descarta, y lo
descarta el contrato declarado del propio campo** (`internal/config/config.go:95-101`):

> «Un scope explícito ('local'/'shared') **siempre se respeta (escape hatch)**; la redacción
> de secretos corre igual en el borde a 'shared'.»

Hacer que `TeamMode` **pise** un scope explícito es exactamente lo contrario de lo que
`TeamMode` promete. Sería romper el escape hatch de C5.2 para arreglar otra cosa.

**Decisión: tres predicados, dos orígenes.** La #1 y la #2 son la **misma** pregunta
—«¿este servidor es el pozo de más de una persona?»— aplicada a dos mecanismos distintos
(redactar el contenido, y rutear la observación). La #3 es otra pregunta y no comparte nada.

```go
// pozoCompartido: ¿este servidor es infra de más de una persona?  ← POLÍTICA declarada
// arbolAusente:   ¿el grafo describe código que no está acá?      ← HECHO medido

func (s *McpServer) redactaTodoIngest() bool    { return s.pozoCompartido }
func (s *McpServer) pisaElScopeExplicito() bool { return s.pozoCompartido }
func (s *McpServer) arbolFueraDeAlcance() bool  { return s.arbolAusente }
```

### Por qué tres predicados y no dos, si dos comparten origen

Porque el valor pedido era **poder cambiar una sin tocar las otras**, y eso lo da el
predicado nombrado, no el campo. Hoy `redactaTodoIngest` y `pisaElScopeExplicito` devuelven
lo mismo; el día que el forzado de scope necesite su propia política, se cambia **un cuerpo
de una línea** y nadie más se entera. **Medido en el árbol entregado:** 7 lecturas de los dos
campos en producción, repartidas 1 (redacción) · 1 (scope) · 5 (árbol), cada una detrás de su
predicado. Con la bandera de hoy eso es imposible: hay que tocar la expresión que alimenta a
las tres.

⚠️ **Y hay que decirlo claro, porque toca el alcance que se eligió:** la separación entrega
tres predicados independientes, pero **no inventa una tercera política**. Colgar la #2 de una
clave propia de config sería agregar una perilla que hoy nadie necesita y que nadie sabría
cómo poner — y la única candidata con significado real, `TeamMode`, está descartada arriba
por su propio contrato. Si más adelante aparece el caso que la pida, el predicado ya está
ahí esperándola.

## D3 — La #1 conserva su valor y su clave, exactamente

```go
s.pozoCompartido = !loopback || cfg.ForceRedact    // IDÉNTICO al de hoy: fail-closed
s.arbolAusente   = arbolAusenteEn(s.projectPath)   // medido, una vez
```

La expresión de `pozoCompartido` **no cambia**, y la clave YAML sigue siendo `force_redact`.
Renombrarla rompería los `config.yaml` ya desplegados —entre ellos el del central, que es el
que el PR #777 acaba de arreglar— por un cambio que es puramente de claridad interna. El
campo de `internal/config` tampoco se toca.

Lo que cambia es **de quién depende la #3**: deja de depender de esa expresión.

## D4 — Las 10 asignaciones de test, clasificadas una por una

Borrar `forceRedact` en vez de dejarlo como alias **es parte del diseño**: cada asignación de
test pasa a ser un error de compilación, así que ninguna puede quedar apuntando al campo
viejo por omisión. Lo único que puede salir mal es clasificarla **mal**, y para eso está el
sabotaje de la matriz.

Son **10**, medidas con `grep '\.forceRedact = '` sin asumir el receptor — un grep por
`s\.forceRedact` se pierde dos, porque usan `central.`:

| Archivo:línea | Pasa a | Por qué |
|---|---|---|
| `codegraph_miss_explicado_test.go:369` | `arbolAusente` | mide el hint del miss en el central |
| `codegraph_missing_test.go:124` | `arbolAusente` | mide `missing` del grafo federado |
| `codegraph_stale_central_test.go:33` | `arbolAusente` | mide `cgStale` de un nodo federado |
| `frescura_cruza_maquinas_test.go:249` | `arbolAusente` | su propio comentario lo dice: «el central: el árbol no está en este disco» |
| `codegraph_push_redact_test.go:40` | `pozoCompartido` | tapa un secreto en el push del grafo |
| `force_redact_test.go:30` | `pozoCompartido` | el test canónico de la redacción |
| `recibo_obs_test.go:159` | `pozoCompartido` | recibo de una obs redactada |
| `recibo_obs_test.go:221` | `pozoCompartido` | ⚠️ la variable se llama `central`, pero mide que una redacción **no** vaya a dead-letter |
| `redact_all_ingest_test.go:27` | `pozoCompartido` | redacción de todo el ingest |
| `telemetry_isolation_test.go:126` | `pozoCompartido` | aislamiento de telemetría |

⚠️ **La fila de `recibo_obs_test.go:221` es la trampa del lote.** Su variable se llama
`central`, que invita a clasificarla como árbol; lo que el test mide es que «una nota con un
secreto se empuja, el central la redacta, y el push cierra en nil». Es redacción. Clasificar
por el nombre de la variable habría puesto un test de redacción a custodiar el árbol, y
habría quedado **verde**.

## D5 — La directiva del arnés que hay que actualizar

⚠️ Además de las 10 de arriba. `codegraph_miss_explicado_test.go:351-352` cita el cuerpo literal del helper, así que su
`de=` deja de matchear. Pasa a:

```go
// arnes: de="func (s *McpServer) arbolFueraDeAlcance() bool { return s.arbolAusente }"
// arnes: a="func (s *McpServer) arbolFueraDeAlcance() bool { return false }"
```

El sabotaje sigue siendo el mismo —«que el helper devuelva `false` siempre»— y sigue atacando
el invariante que el test declara. `frescura_cruza_maquinas_test.go:282` **no se toca**:
cita el `if s.arbolFueraDeAlcance()` del call site, que se conserva palabra por palabra.

## D6 — Dónde viven los tests nuevos

Archivo nuevo `internal/mcp/tres_preguntas_test.go`, con los tres de la separación (S1, S2,
S3). Los seis de no-regresión ya viven en sus archivos y **no se mueven**: moverlos haría
ruido en el diff justo donde el reviewer necesita ver que no cambiaron.

## Lo que este diseño NO resuelve

- **El `walkSourceTree` sigue siendo la única enumeración exacta.** El medidor (d) es más
  grueso a propósito: contesta «puede haber árbol», no «el árbol está completo».
- **Nada distingue un grafo federado nodo por nodo.** `memory.GraphNode` tiene `Key`, `Kind`,
  `Name`, `Path`, `StartLine`, `EndLine`, `External` y `SrcFingerprint` — ningún marcador de
  origen. Un cerebro con nodos propios **y** federados a la vez seguiría teniendo una sola
  respuesta para todo el grafo. Resolverlo pide una columna nueva, y es otro PR.
