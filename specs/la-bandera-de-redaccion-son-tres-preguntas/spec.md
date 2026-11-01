# Spec — La bandera de redacción son tres preguntas

Vocabulario RFC 2119: **DEBE** / **NO DEBE** / **DEBERÍA** / **PUEDE**.

## El punto de partida: el helper ya fue un paso, le falta el cuerpo

`arbolFueraDeAlcance()` **no** es el defecto: es un arreglo a medio camino. El comentario
de `codegraph_miss_explicado_test.go` cuenta su historia — `cgStale` y `graphFreshness`
preguntaban cada uno por su lado, `pistaDelMiss` era la tercera que lo necesitaba y no lo
tenía, y el helper las unificó. Lo que quedó sin terminar es su **cuerpo**:

```go
func (s *McpServer) arbolFueraDeAlcance() bool { return s.forceRedact }
```

Por eso esta spec **conserva el nombre y la firma del helper**: el recableado de la
tercera pregunta ocurre en un cuerpo de función, con sus 5 call sites intactos.

## Invariantes

### De la separación

| # | Invariante | Test | Sabotaje que lo pone en rojo |
|---|---|---|---|
| S1 | Las tres preguntas **DEBEN** poder divergir: existe al menos una configuración alcanzable donde una vale `true` y otra `false`. Hoy es inexpresable. | `TestLasTresPreguntasPuedenDivergir` | atar dos de los tres campos a la misma expresión |
| S2 | El predicado del árbol **DEBE** salir del disco, **NO DEBE** salir del bind ni de la política de redacción. Dos servidores con el **mismo** bind y distinto disco **DEBEN** dar distinta respuesta. | `TestElArbolSeMideDelDiscoNoDeLaPolitica` | devolver el cuerpo viejo: `return s.forceRedact` |
| S3 | Un daemon **local** con la redacción activa **DEBE** seguir midiendo la frescura del árbol: `cgStale` **DEBE** poder devolver `true` y `graphFreshness` **DEBE** poder contar distinto de `(0,0,0)`. | `TestUnLocalQueRedactaSigueMidiendoLaFrescura` | `return s.forceRedact` (hoy devuelve `(0,0,0)` y `stale=false`) |

S3 es **el caso que hoy no se puede escribir**, y por eso es el invariante que justifica el
cambio: es la regresión silenciosa que la bandera compartida habilita.

### De no-regresión (lo que este cambio NO DEBE cambiar)

| # | Invariante | Test | Sabotaje |
|---|---|---|---|
| N1 | Un secreto crudo **NUNCA DEBE** entrar sin redactar cuando la política de redacción está activa, **sea cual sea el scope que declare el cliente**. | `TestForceRedactScrubsLocalIngest` (ya existe) | sacar el `redactIfForced` del camino de ingest |
| N2 | Sin la clave `force_redact` declarada en config, el valor de la pregunta #1 **DEBE** ser **idéntico** al de hoy. El default es **fail-closed**: la separación no afloja la redacción. | `TestSinClaveDeclaradaLaRedaccionNoSeAfloja` | invertir el default, o hacerlo depender de la #3 |
| N3 | El central (grafo federado, sin árbol en disco) **NO DEBE** reportar podredumbre: `cgStale` **DEBE** dar `false` y `graphFreshness` **DEBE** dar `(0,0,0)`. | `TestCgStaleNotMarkedOnSharedCentral` (ya existe) | devolver `false` siempre en el helper |
| N4 | En el central, el hint de `pistaDelMiss` **DEBE** decir «no tengo el árbol» y **NO DEBE** afirmar «es de otra rama». | `TestEnElCentralElMissNoAfirmaQueElArchivoEsDeOtraRama` (ya existe) | `return false` en el helper |
| N5 | La huella de `methods.go` **DEBE** seguir eligiendo la del grafo cuando no hay árbol, en vez de degradar a `frescuraDesconocida`. | el sabotaje de `frescura_cruza_maquinas_test.go:282` (ya existe) | borrar la rama `huella, ref = s.huellaDelGrafo` |
| N6 | El golden de `tools/list` **NO DEBE** moverse: este cambio no agrega, saca ni renombra ninguna tool. | `TestToolsListGolden` (ya existe) | cualquier cambio de superficie MCP |

## Escenarios

### E1 — el caso que hoy es imposible (S3)

**Dado** un daemon en una máquina de una persona, con el repo clonado en disco y
`force_redact: true` en su `config.yaml`,
**cuando** se pide `musubi_code_graph` de un símbolo cuyo archivo **cambió** en disco
después de indexarse,
**entonces** el nodo **DEBE** venir marcado `stale: true`.

> Hoy viene `stale: false`, porque la política de redacción apaga la medición del disco. Y
> el indicador que uno miraría para comprobarlo —`ghosts: 0`— es exactamente lo que
> devuelve un grafo que dejó de mirar.

### E2 — el central no cambia (N3 + N4)

**Dado** el cerebro central, cuyo `MUSUBI_HOME` (`/home/musubi/musubi-brain`) no es un repo
git y no tiene el árbol de fuentes,
**cuando** se pide la frescura del grafo federado,
**entonces** `graphFreshness` **DEBE** devolver `(0, 0, 0)` y el hint de un símbolo ausente
**DEBE** explicar que el árbol no está en este cerebro.

### E3 — la redacción no se afloja (N1 + N2)

**Dado** un servidor con la política de redacción activa,
**cuando** un cliente manda una observación con un secreto crudo y declara `scope: local`,
**entonces** el secreto **DEBE** quedar redactado en el libro mayor, independientemente de
qué haya decidido la pregunta #2 sobre el scope.

> N1 y E3 son el cierre del PR #777 y **no se tocan**. Si la separación obliga a elegir
> entre la claridad y esta propiedad, gana esta propiedad.

## Criterio de aceptación del medidor (y un candidato ya descartado)

El medidor que elija el `design` **DEBE** devolver «árbol presente» para el proyecto que
arma `proyectoIndexado` (`codegraph_miss_explicado_test.go:24`): un `t.TempDir()` con
`go.mod` y `pkg/a.go` escritos e indexados. Es el helper del que cuelgan los tests de
codegraph del paquete, así que un medidor que lo dé por ausente **apaga la medición en toda
la suite** y los deja verdes sin medir nada.

⚠️ **Eso descarta el candidato (a) del proposal, `hayRepoGit(projectPath)`, y no por
análisis sino medido.** `t.TempDir()` resuelve a `C:\Users\…\AppData\Local\Temp\…` (en el
CI, `/tmp/…`), que no está bajo ningún repo git:

```
hayRepoGit(os.TempDir())  = false
hayRepoGit(<el worktree>) = true    ← control: la función sí detecta un repo real
```

El control importa: sin él, el `false` podría ser un defecto de la medición y no un hecho
sobre el directorio. Con él, el candidato (a) queda descartado por evidencia, no por
sospecha — era el contraejemplo que el proposal anticipó en prosa («una carpeta con código
y sin `.git`»), y su víctima concreta es la suite del propio paquete.

Queda en pie el candidato **(c)**: enumerar archivos indexables bajo `projectPath` **una
vez al arranque** y cachear. Da `true` para `proyectoIndexado` (hay `pkg/a.go`) y `false`
para el `MUSUBI_HOME` del central (sólo `.musubi/`), que son los dos extremos que hay que
separar.

## Las dos directivas del arnés que este cambio DEBE actualizar

El arnés sabotea por **texto literal**, así que cambiar el cuerpo del helper rompe el
`de=` que lo cita. Medido con `grep 'arnes:' internal/mcp/*_test.go`:

| Archivo:línea | Qué cita | Qué le pasa |
|---|---|---|
| `codegraph_miss_explicado_test.go:351-352` | `de="func (s *McpServer) arbolFueraDeAlcance() bool { return s.forceRedact }"` | **deja de matchear**: hay que actualizar el `de=` al cuerpo nuevo |
| `frescura_cruza_maquinas_test.go:282` | `de="if s.arbolFueraDeAlcance() {\n\t\t\thuella, ref = s.huellaDelGrafo"` | **sobrevive**, porque se conserva el nombre del helper y la forma del `if` |

Un `de=` que no matchea no es un rojo franco: es un sabotaje que no se aplica y un verde
que no mide nada. Por eso entra en la spec y no en una nota al pie.

## Riesgo de defensa en profundidad: medir en las tres direcciones

Las tres preguntas cuelgan hoy de la **misma** variable, así que un test que dice medir una
puede quedar verde al sabotear otra — es el riesgo que este repo ya midió con M39/M50 en
`internal/motor`, donde hizo falta sabotear en las dos direcciones para afirmar que ninguna
resolución tapaba a la otra.

Acá son tres, así que la verificación **DEBE** incluir la matriz completa: sabotear cada
una de las tres y comprobar que **sólo** caen los tests de esa pregunta. Un test de
redacción que cae al sabotear el árbol no está midiendo la redacción, y con la bandera
compartida de hoy es exactamente lo que haría.
