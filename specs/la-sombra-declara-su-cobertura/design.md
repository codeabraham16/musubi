---
artifact: design
schema_version: "1.0"
change: la-sombra-declara-su-cobertura
status: draft
---

# Diseño técnico — la tabla de la sombra declara su cobertura

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | **Derivar la cobertura por SQL** en el momento de reportar | (a) persistir `ajenos`/`tirados` del worker en una tabla nueva; (b) escribir una fila marcadora en `shadow_verdicts` por descarte | El recorte es una propiedad del **corpus**, no de una corrida: derivarlo lo hace durable y siempre al día sin migración. (a) agrega esquema y pierde el delta si el proceso muere de golpe; (b) contamina la tabla que se agrupa por `heur_relation` y obliga a filtrar el marcador en cada consulta futura |
| D2 | La consulta vive en `internal/memory/shadow.go` como `ShadowCoverage()`, al lado de `ShadowAgreementByRelation`, y entra a la interfaz `ConflictDetector` (que `StorageBackend` embebe) | Ponerla en `cmd/musubi/conflicts.go` | El paquete `memory` es el dueño del SQL; el comando sólo formatea. Y entrar a la interfaz la deja disponible para un lector futuro (el doctor, `musubi_readiness`) sin mover nada |
| D3 | «Re-detectable» = `visibleObsPredicateDe(alias)` aplicado a **las dos puntas** por JOIN | Escribirlo a mano con alias | `obsrank.go:62-63` ya resuelve exactamente este caso (las dos puntas de una arista) con ese helper, y el comentario de `scope.go:51` documenta que el **único** sitio que lo escribió a mano salió mal: le faltaba `quarantined = 0`, y la centralidad del recall se calculaba sobre un grafo con observaciones en cuarentena |
| D4 | «Medible» se arma desde `RelStatusPending` y `RelResolvedByHeuristic`, concatenadas al SQL en Go | Literales dentro de la consulta | Es el patrón que `setConservandoVeredicto` ya estableció en este mismo expediente, y por el mismo motivo: si la constante cambia, el SQL cambia con ella en vez de desalinearse en silencio. Son texto del propio código, nunca entrada de nadie |
| D5 | La cobertura se imprime **siempre**, no sólo cuando el recorte pasa un umbral | Mostrarla sólo si el recorte supera, digamos, el 10 % | Un umbral agrega una rama que hay que custodiar y que puede esconder justo el caso informativo. Y un recorte chico también es información: «medí 890 de 895» es una afirmación fuerte que hoy el comando no puede hacer |
| D6 | El `--json` pasa de **array** a **objeto** `{coverage, by_relation}` | Mantener el array y agregar una bandera aparte | Medido: **cero consumidores**. Los únicos usos de `ShadowAgreementByRelation` son este comando y tres tests de `internal/mcp/shadow_test.go`; nada parsea la salida. Una bandera aparte dejaría la cobertura opcional, que es justo lo que este cambio quiere evitar. Se declara en el CHANGELOG como cambio de forma |
| D7 | Sin scope por tenant, igual que `ShadowAgreementByRelation` | Filtrar por `project_id` | `shadow_verdicts` **no tiene** `project_id` (ver el CREATE TABLE de la migración), así que un numerador scopeado no existe, y un denominador scopeado contra un numerador federado daría una tasa falsa. El comando es CLI local sobre el workspace. Queda declarado, no supuesto |

## Enfoque de implementación

Una consulta con tres `COUNT` sobre el mismo `FROM`, más el complemento por resta:

- `Relaciones` — todas las filas de `observation_relations`
- `Redetectables` — las dos puntas visibles
- `Medibles` — re-detectables y (`pending` o firmadas por el detector)
- `FirmaAjena` — re-detectables con un veredicto que no es del detector
- `PuntaNoVisible` — `Relaciones - Redetectables`
- `AutoAplicadas` — **subconjunto** de la anterior: firmadas por el detector y con el target
  supersedido por su propio source. Es la que explica el cero, y se cuenta por su **efecto**
  (`target.superseded_by = source_id`) y no por la etiqueta `supersedes`, porque una fila
  re-arbitrada después seguiría teniendo el supersedido aplicado
- `Medidas` — filas de `shadow_verdicts`

`PuntaNoVisible` se deriva **por resta** y no por una segunda consulta con `NOT EXISTS`: es el
complemento exacto de `Redetectables` sobre el mismo total, así que una consulta aparte podría
devolver un número que no cierra con los otros tres. La aritmética cierra por construcción:
`Medibles + FirmaAjena == Redetectables` y `Redetectables + PuntaNoVisible == Relaciones`. Y «por
construcción» hay que leerlo literal: cierran porque son **tautologías** —una resta y una
partición—, así que **no son evidencia de que los números sean los correctos**. El plan de
pruebas lo mide.

El comando imprime la cabecera antes de la tabla **y también** en el camino del libro vacío, que es
el que hoy atribuye la causa equivocada.

## Contratos / interfaces

```go
// ShadowCoverageReport dice sobre qué parte del corpus puede hablar la tabla del modo sombra.
type ShadowCoverageReport struct {
	Relaciones     int `json:"relaciones"`
	Redetectables  int `json:"redetectables"`
	Medibles       int `json:"medibles"`
	FirmaAjena     int `json:"firma_ajena"`
	PuntaNoVisible int `json:"punta_no_visible"`
	AutoAplicadas  int `json:"auto_aplicadas"`
	Medidas        int `json:"medidas"`
}

// En la interfaz ConflictDetector (internal/memory/backend.go:157), al lado de ShadowAgreementByRelation:
ShadowCoverage() (ShadowCoverageReport, error)
```

Salida JSON del comando (D6): un objeto con `coverage` y `by_relation`.

Tags explícitos en minúscula por la regla 6 del repo: un DTO con tags sólo en mayúscula no falla al
serializar, guarda campos vacíos, y eso se lee como un dato.

## Trade-offs

- **Se gana** un denominador durable y auditable sin esquema nuevo, y el mensaje del libro vacío deja
  de culpar a la config.
- **Se cede** la foto por corrida: la cobertura dice cómo está el corpus **ahora**, no qué vio una
  corrida concreta del worker. Para la pregunta que motiva todo esto —«¿el umbral está bien
  puesto?»— el ahora es lo que importa; para auditar una corrida puntual sigue estando la línea de
  log del apagado.
- **Se cede** la compatibilidad de forma del `--json`, con cero consumidores medidos.

⚠️ **Dos cifras corregidas, una de ellas mía, y la segunda cambió el diseño.**

1. El CHANGELOG del #784 afirma que «996 de los 1.098 pares de la base viva eran re-detectables con
   firma ajena». Ese número **sobreestimaba el denominador**: no exigía que las **dos** puntas
   fueran visibles, así que contaba como re-detectable casi todo el corpus.

2. La primera versión de este design dijo «895 re-detectables, 893 con firma ajena, 2 medibles». Lo
   corrigió **el código al correrlo**: casi todas las re-detectables tienen firma ajena, así que el
   techo es un puñado de pares (el 2026-11-02: **4 medibles de 901** re-detectables sobre 1.138
   relaciones; la cifra se mueve en el día, la forma no). Las
   dos filas que el detector firmó solo (`heuristic`) son `supersedes` con el target
   supersedido por su propio source, así que **no son re-detectables** y nunca estuvieron en el
   conjunto. Yo las había contado sobre el libro entero y restado de las re-detectables: mezclar dos
   denominadores.

3. **Y las dos veces el número salió mal por el mismo defecto de MÉTODO, no por la aritmética:** se
   midió copiando `.musubi/memory.db` **sin su `-wal`**. En modo WAL las escrituras recientes viven en
   ese archivo lateral, así que una copia del `.db` solo devuelve una foto **rancia que se lee como
   actual** —y `immutable=1` la reproduce a pedido—. Medido: sin el `-wal` la consulta daba `medibles
   0` y `status='pending'` sin una sola fila; con el `-wal` (15,8 MB) da **4 medibles** y **4 filas**
   `pending`. El juez de SQL tenía razón y el que leía mal era yo. Y lo que lo cerró no fue re-medir
   con otra consulta: fue **correr el comando** contra la base completa y leer su salida.

Esa corrección **agregó el balde `AutoAplicadas`**, porque un «0 medibles» sin explicación reproduce
el defecto que este cambio existe para arreglar. Y no es un caso raro: el repo ya lo había medido en
`relations_backfill.go:32-37` —exigir visibilidad «excluiría TODA auto-resolución exitosa», y en el
cerebro central se comía 50 de 73 pares de señal—, que es exactamente por qué `conflicts backfill`
a propósito NO exige visibilidad.

Verificado contra la fuente y no supuesto: la detección exige visibilidad en **las dos** puntas
(`internal/memory/conflicts.go:497` para el source, con el comentario que documenta la fuga de 12
relaciones pendientes con el source supersedido; `internal/memory/conflicts.go:649` para los
candidatos), así que `Redetectables` es exactamente el conjunto que la detección puede volver a
emitir. Y el predicado de «medible» es el mismo que aplica el worker: `encolarSombra` encola si
`Status == pending || ResolvedBy == heuristic` (`internal/mcp/shadow.go:185`).

## Plan de pruebas

Un custodio por requisito, cada uno verificado **rojo bajo sabotaje del invariante que declara**:

| requisito | custodio | sabotaje que lo tiene que enrojecer |
|---|---|---|
| R1/R5 | la cobertura separa firma ajena de punta no visible | contar la punta invisible como firma ajena |
| R2 | la cobertura sale del corpus, no de contadores | por construcción: no hay estado que leer |
| R3 | un par con la target en cuarentena no es re-detectable | reemplazar el helper por un predicado sin `quarantined` |
| R4 | una fila firmada por el detector es medible; una de un agente no | invertir el literal de la firma |
| R5c | la auto-resolución aplicada se cuenta aparte y como subconjunto | contarla sumando en lugar de como subconjunto, o por la etiqueta en vez del efecto |
| R6 | el JSON trae `coverage` y `by_relation` con tags en minúscula | serializar el struct sin tags |
| R7 | con el libro vacío la salida declara el techo de lo medible | devolver temprano sin la cobertura |
| R8 | `SaveShadowVerdict` computa `agree` adentro | hacer que `agree` salga del caller |

Más el **control NOOP** que el expediente anterior enseñó a correr: reescribir una condición de
forma equivalente y verla **verde**, para descartar que un rojo venga de SQL que dejó de parsear.

Y la aritmética como invariante propio: `Medibles + FirmaAjena == Redetectables` y
`Redetectables + PuntaNoVisible == Relaciones`. ⚠️ **El panel midió que esas dos identidades
NO detectan una consulta mal escrita, y la frase que lo afirmaba acá era falsa.** Las dos son
**tautologías**: `PuntaNoVisible` se *deriva* por resta, así que la segunda no puede fallar —a lo
sumo escupe un negativo—, y la primera es la partición del `CASE`, cuyas dos ramas son
complementos exactos sobre el mismo conjunto (y con `medible` evaluando a NULL las dos caen al
`ELSE`). Medido sobre la base viva: reescribir `COUNT(1)` como `COUNT(DISTINCT r.source_id)` —la
forma «natural» de contar si uno piensa en pares por fuente— reporta **394 re-detectables en
vez de 901** y **las dos identidades cierran igual** (1+393==394 y 394+744==1138). Sólo se
romperían si una fuente mezclara medible con firma ajena dentro de lo re-detectable, y en el corpus
de hoy esas fuentes son **cero**: la mutación mentiría callada.

Lo que hace que los números estén bien **no es la aritmética**: es que `observations.id` es
`PRIMARY KEY` (`internal/memory/database.go:513`), así que los dos JOIN son muchos-a-uno y no
amplifican —medido, `COUNT(1) == COUNT(DISTINCT r.id) == 901`—. Eso ya estaba dicho en los
trade-offs del proposal («dos JOIN por id (clave primaria)»), y aun así el mérito se le había
atribuido acá a la aritmética. Las identidades sirven de chequeo de **consistencia**, no de
**correctitud**: lo que hay que verificar aparte es `COUNT(1) == COUNT(DISTINCT r.id)` y que
`PuntaNoVisible` no salga negativo.

## Lo que la verificación agregó al diseño

Trece cosas que no estaban en el plan y que el arnés obligó a cambiar. Las siete primeras salieron
de la primera vuelta de `verify`; de la 8 a la 13, del bucle de corrección que abrió el tally
`no_real` 2–1:

1. **El custodio de R4 era vacuo y el sabotaje lo probó.** Con un par medible-por-firma y uno de
   firma ajena, mutar la constante del balde **intercambia** las dos filas y los totales no se
   mueven: el sabotaje pasaba VERDE sobre un código genuinamente roto. Lo cerró desbalancear las
   cantidades —dos firmas propias contra una ajena—, que enrojece con *cualquier* firma a la que se
   mute la constante, no sólo con la que yo hubiera elegido para el sabotaje. Un custodio que sólo
   compara agregados no distingue una permutación de un defecto.
2. **Los sabotajes viven en el repo, no en un script.** Los 12 quedaron como anclas mecanizadas y
   el censo del árbol los valida en cada corrida. De paso denunció que `gofmt` convierte `''` en
   `”` dentro de un comentario, lo que rompe la directiva en silencio: el `de` deja de existir y su
   verde se lee igual que «la guarda cubre».
3. **La guarda del arnés propio tampoco medía.** «El disco cambió» estaba implementado como `git
   diff --stat`, y en este árbol los cuatro archivos del expediente figuran en el diff **siempre**
   porque están sin commitear: la guarda pasaba igual con el sabotaje sin aplicar. Se compara el
   texto releído del disco, y una fila de control con un sabotaje idéntico verifica que la guarda
   siga viva.
4. **Las dos identidades aritméticas son tautologías, y este documento afirmaba lo contrario.** El
   panel midió que `Medibles + FirmaAjena == Redetectables` y `Redetectables + PuntaNoVisible ==
   Relaciones` **cierran igual** sobre una consulta con un error del **56 %** en el titular (mutar
   `COUNT(1)` a `COUNT(DISTINCT r.source_id)` reporta 394 re-detectables en vez de 901, y las dos
   identidades siguen diciendo que cierra). No es un descuido de la implementación: una es una
   **resta** y la otra es la **partición de un `CASE`**, así que no pueden fallar por construcción
   —a lo sumo escupen un negativo—. Lo que sostiene los números es el `PRIMARY KEY` de
   `observations.id`, que hace los dos JOIN muchos-a-uno; el comentario de
   `internal/memory/shadow.go:215` ya lo decía («El JOIN es por clave primaria») y los trade-offs
   del proposal también, así que **el código nunca tuvo el defecto: lo tenía la prosa que se
   publica con él**. La lección es la misma familia que el punto 1: **un invariante que no puede
   fallar se lee como la red que atrapa el error**, y así es como un criterio de éxito termina
   satisfecho por un defecto.
5. **El principio del punto 1 se había aplicado a UNA de las dos mitades del predicado.** El
   custodio de R4 se arregló desbalanceando la rama `heuristic` —dos firmas propias contra una
   ajena—, y la rama `pending` se quedó sin ese tratamiento. Medido: confundir las dos
   constantes de la misma familia (`RelStatusPending` por `RelStatusResolved` en
   `internal/memory/shadow.go:221`, declaradas juntas en `internal/memory/relations_obs.go:45-46`)
   **deja los dos paquetes en verde** —`internal/memory` 66,8 s y `cmd/musubi` 90,3 s, exit 0,
   igual que la línea base—, porque la mutación **permuta** en
   vez de anular: saca la fila `pending` del balde medible y mete una `resolved` que estaba en firma
   ajena, neto cero. Y el expediente **ya tenía** un sabotaje declarado para esa misma rama
   (`internal/memory/shadow_cobertura_test.go:65-67`, que la reescribe como `1 = 0`): enrojece, pero porque
   **desbalancea**. Un sabotaje declarado puede cubrir la rama correcta y seguir sin medirla. R4 es
   un requisito declarado (`spec.md:25-28`: «Si el filtro de procedencia cambia, la cobertura DEBE
   cambiar con él»), así que esto se arregló **dentro** de este PR y no en el siguiente.

6. **Los custodios del comando medían las piezas y ninguno la costura.** Los cuatro de
   `cmd/musubi/conflicts_cobertura_test.go` llaman a los helpers directo —`imprimirLibroVacio(&buf,
   …)`, `imprimirCobertura(&buf, …)`— o se arman el DTO a mano, así que **ninguno atraviesa el
   cableado** de `runConflictsShadow`, que además escribía a `os.Stdout` hardcodeado y no era
   observable. Medido: **tres mutaciones del cableado dejaban `go test ./cmd/musubi/` entero en
   verde** — emitir el array pelado en vez del sobre `{coverage, by_relation}`, pasarle al libro
   vacío un `ShadowCoverageReport{}` en cero, y derivar la cobertura sin llevarla a la salida. La
   segunda es la peor: hace que el comando vuelva a decir «techo CERO» teniendo techo 4, que es
   **exactamente** el defecto que R7 vino a arreglar. Y es el mismo principio que el comentario de
   `imprimirCobertura` ya tenía escrito —«escribe en un Writer y no en stdout directo para que su
   custodio pueda leer el texto»—, aplicado a los helpers y **no a la unión**. Entra en este PR
   porque R1, R6 y R7 están declarados a nivel COMANDO (`spec.md:12`, `:42`, `:45-46`): la costura
   se extrajo a `emitirShadow(w, cob, res, asJSON)` (`cmd/musubi/conflicts.go:87-112`), con **un
   solo caller en producción** —`cmd/musubi/conflicts.go:77`—, y las tres mutaciones quedaron
   declaradas como anclas del arnés, cada una **roja en su propio subtest**, con la restauración de
   cada sabotaje verificada por `cmp -s` contra el respaldo.
   ⚠️ **El «se movieron N líneas sin cambiar lógica» de una extracción NO es auditable** y conviene
   no leerlo como si lo fuera: el estado pre-extracción nunca se commiteó, así que no existe el
   diff contra el que medirlo. Lo verificable es el estado de HOY —el cuerpo de la función y su
   único caller, los dos con ruta y línea—, y es lo que esta viñeta cita en su lugar.
7. **Las referencias `archivo.go:línea` de los artefactos se verificaron contra DOS estados, y una
   ya estaba rota.** Chequear la ref contra el disco no alcanza: si apunta a código nuevo, el disco
   la confirma y el daño queda igual. Hay que compararla también contra `origin/main` —si main
   tenía otra cosa en esa línea, la movió este PR—. Medido así sobre el `CHANGELOG` y todos los
   expedientes de `specs/`: **138 refs estables** y **8 que difieren**, las ocho apuntando a código
   nuevo de este PR y verificadas una por una. El hallazgo: al extraer `emitirShadow`, la línea de
   la tabla se corrió de `:83` a `:107`, y eso dejó **`CHANGELOG.md:49` —la viñeta del PR #784, ya
   mergeado y publicado— apuntando a un comentario**. `git status` no ve nada de eso, porque el
   archivo que queda roto es el que NO se tocó. Corolario: **una extracción de función no es un
   cambio local** —mueve el numerador de toda ref posterior del archivo, incluidas las de entradas
   inmutables ya publicadas—. Y de paso: cinco refs de este expediente usaban basename desnudo
   (`shadow.go:215`) en un repo con **dos** `shadow.go` y **dos** `conflicts.go`, así que quedaron
   calificadas con la ruta completa que el resto del documento ya usaba.
   ⚠️ **Fuera del alcance de este PR, el mismo verificador deja deuda MEDIDA en el repo**: 13 refs
   ambiguas por basename y 1 fuera de rango (`internal/memory/codepath.go:31`, con 16 líneas) en
   otros expedientes, más 9 desplazadas por trabajo ajeno en `internal/mcp/methods.go` y
   `internal/mcp/registry.go`. No se tocan acá —no son de este cambio—, pero el verificador es
   candidato a custodio de CI.
8. **Un sabotaje que no compila se lee IDÉNTICO a un custodio que funciona, y dos de los míos eran
   eso.** `rc=1` y ni un solo `--- FAIL`: el rojo lo firmaba el compilador. Las dos mutaciones
   dejaban una variable local sin usar —`declared and not used: cob` y `: asJSON`—, porque reemplazar
   un argumento por una constante borra el único uso de la variable que lo alimentaba. Y eran las
   mutaciones que yo mismo había declarado como anclas, así que el arnés habría propagado el defecto.
   Dos arreglos: las anclas pasaron a mutaciones que **conservan el uso**
   (`memory.ShadowCoverageReport{Medidas: cob.Medidas}` en vez de `{}`, y `!*asJSON` en vez de
   `false`), y el harness de sabotaje **aborta el caso** si el log trae `build failed`, `declared and
   not used`, `cannot use` o `undefined:`. Es la trampa que la skill nombra —«antes de concluir que
   rompió algo, verificá que el archivo haya cambiado de verdad»— un paso más adentro: el archivo
   **sí** cambió, y el rojo igual no era del custodio.
9. **Un fixture AGREGADO no puede medir una constante cuando el corpus tiene una sola instancia de
   la clase que la mutación nombra — y agregarle una fila por clase lo EMPEORA.** El custodio de R4
   tiene P=2 pendientes, H=2 del detector y A=1 ajena, y declara Medibles=4 / FirmaAjena=1. Medido:
   mutar la constante del balde a `<> 'agent'` deja esos dos números **idénticos**, porque con una
   sola firma ajena —y justo la que la mutación nombra— `<> 'agent'` y `= 'heuristic'` son el **mismo
   predicado sobre ese corpus**. El arreglo intuitivo es sumar una fila por cada firma que falta, y
   es peor: con P=2, H=2, A=1, U=1 aparece una unión de igual tamaño —H+A+U = 4 = P+H—, así que un
   predicado que midiera «tiene cualquier firma» devolvería los mismos totales. **Un agregado no
   distingue dos subconjuntos distintos que suman lo mismo.** La vía es una **celda por clase, con
   UNA fila y un motor fresco** (`NewDbEngine(t.TempDir())` por subtest): con una sola fila no hay
   suma que empate. Y la evidencia de que el hueco era real, no una hipótesis: con cada una de las
   cuatro mutaciones puesta, **el custodio nuevo cae y el fixture agregado sigue VERDE** —`<> 'agent'`
   en el balde (4 celdas rojas), la rama decidida por ausencia de firma (1), la misma inversión en la
   subconsulta de auto-aplicadas (2), y aceptar cualquier supersedido en vez del propio source (1)—,
   cada una roja **sólo en el subtest que la declara**.
10. **A20 sí entra, y lo que estaba mal era mi lectura del veto.** Lo había mandado al plan siguiente
    creyendo que la única celda capaz de distinguir «el status es `pending`» de «no tiene firma»
    chocaba con el alcance que `proposal.md:58-59` declara fuera. No choca: lo vetado es un par
    `pending` **firmado**, y la celda que hace falta es `resolved` **sin firma**, que es otra cosa y
    que el esquema admite —verificado: `UpsertObsRelation` la guarda tal cual—. Con esa celda, mutar
    la primera rama a `COALESCE(r.resolved_by,'') = ''` enrojece, y es la **única** celda que lo
    hace: las demás dan el mismo resultado con las dos condiciones. Un criterio por ausencia de dato
    y un criterio por status coinciden en todo el corpus vivo y difieren en el estado que nadie
    produce todavía, que es exactamente donde hay que fijarlos.
11. **El cableado del comando se midió sin refactorizar producción.** La extracción que el punto 6
    pedía para `emitirShadow` no hacía falta repetirla un salto más arriba: `workspaceDir()`
    (`cmd/musubi/setup.go:21-24`) delega en `workspaceDirConOrigen()` (`cmd/musubi/setup.go:39-50`),
    que mira `MUSUBI_HOME` **antes** que `CLAUDE_PROJECT_DIR` y que el cwd,
    así que `t.Setenv("MUSUBI_HOME", t.TempDir())` —patrón que el paquete ya usa en
    `cmd/musubi/setup_helpers_test.go:117`— deja ejercer `runConflictsShadow` **entero** contra una
    base sembrada, capturando `os.Stdout` con un `os.Pipe`. Importa como criterio: la alternativa
    cambiaba código ejecutable **para poder medirlo**, y un cambio hecho para que el test alcance es
    una hipótesis más que verificar, no una verificación.
12. **Agregar un custodio a una línea que ya tenía uno obliga a tocar las directivas VIEJAS, y el
    arnés lo exige en las dos direcciones.** `colision_ok` nombra **la prueba de la otra** directiva,
    no la propia, y declarar un nombre que no se pisa de verdad es lo que el censo llama «la
    respuesta quedó rancia» —peor que el aviso, porque afirma que alguien miró—. Los cuatro custodios
    nuevos abrieron **16 colisiones**; cerrarlas fueron 9 ediciones: 4 apuntando al fixture agregado y
    5 en las directivas viejas sumando el nombre del custodio nuevo. Y la respuesta no es «ya lo
    miré» sino el dato del punto 9: **con el `a` de cada una puesto, cae una sola de las dos**, que es
    lo que prueba que son dos guardas sobre la misma línea y no una contada dos veces.
    ⚠️ **Y esas 5 ediciones volvieron a romper una ref, que es la recaída exacta del punto 7**:
    agregar la prosa y la clave a las directivas viejas corrió 4 líneas hacia abajo todo lo que
    seguía en el archivo, y la cita de este mismo documento a la directiva del `1 = 0` —que estaba
    en `:61-63`— pasó a apuntar a otro lado. Medida y corregida a `:65-67`. Que haya vuelto a pasar
    **sabiendo** el corolario es el argumento del propio punto 7 para que el verificador sea un
    custodio de CI y no una disciplina de quien se acuerde.
13. **Un test que pasa `nil` donde producción pasa un slice vacío no-nil no custodia el camino que
    cree.** `ShadowAgreementByRelation` arranca con `out := []ShadowAgreement{}`
    (`internal/memory/shadow.go:113`) y devuelve **ese** slice cuando no hay filas, así que un libro
    vacío **nunca** llega como `nil` al comando. El subtest del libro vacío pasaba `nil`, y con eso
    mutar la guarda `len(res) == 0` a `res == nil` quedaba verde mientras el camino de R7 se volvía
    **inalcanzable en el binario**. Pasa a entrar por la misma puerta que producción.

14. **Las DOS recetas canónicas para medir el alcance de una rama fallan acá, cada una de un modo
    distinto, y la que funciona es la más humilde.** Esta rama está **3 commits detrás** de
    `origin/main` (`4f37c832`) y tiene **0 commits propios**: todo el trabajo está sin commitear.
    De ahí que `git diff origin/main` reporte **36 archivos, +1.925/−1.921** — esos 1.921 borrados
    son los archivos que `origin/main` **agregó** después del punto de corte, y el diff los muestra
    como si esta rama los hubiera borrado. Al juez de alcance casi le llega esa cifra como «lo que
    el PR toca», o sea un PR que roba trabajo ajeno. Y la receta de tres puntos
    —`git diff origin/main...HEAD`, que normalmente es la correcta porque resuelve la base de
    fusión— acá devuelve **VACÍO**, porque `git merge-base HEAD origin/main` **es HEAD** y el rango
    queda `HEAD..HEAD`. El alcance real sale de `git diff HEAD`: **15 archivos, +1.877/−20** (de
    los cuales `--cached` son +1.412: las otras 465 líneas son el bucle de corrección de la vuelta
    1 sin stagear). Regla: antes de creerle a un diff de alcance, verificar
    `git rev-list --count origin/main..HEAD`; si da **0**, el trabajo está en el árbol y no en
    commits, y cualquier rango de revisiones mide cero o mide de más.

15. **Una taxonomía de sabotaje no puede encontrar una aserción que nunca se escribió, y ese es el
    hueco que esta fase destapó.** El lente de alcance recorrió las siete marcas de trampa
    conocidas —`t.Skip`, aserciones comentadas, `t.Log` donde va `t.Error`, `t.Run` muerto, `_ =`
    que tira el error— y las siete dieron **cero**. La conclusión que colgó de eso («no hay trampa
    de verificación») era inválida, y el propio lente la retractó: las siete buscan una **marca
    sintáctica sobre una aserción presente**, y lo que falta acá no es una aserción debilitada sino
    una **ausente**. La octava categoría, que sí la encuentra: **campo producido y nunca aseverado
    en su valor**; se mide **mutando el producto**, no leyendo el test. Censo por campo de
    `ShadowCoverageReport` (comparaciones en `*_test.go` de todo el repo): `PuntaNoVisible` 7 ·
    `Redetectables` 5 · `AutoAplicadas` 5 · `Medibles` 4 · `FirmaAjena` 4 · `Relaciones` 2 (y una
    es de **otro** struct) · **`Medidas` 0**. En el motor el recuento es 6/7 con aserción (5
    directas + 1 transitiva); en la **salida** es 2/7, con 5 huérfanos. `Medidas` es el peor caso
    porque es el único **aritméticamente independiente**: sale de su propia subconsulta sobre otra
    tabla (`shadow.go:231`) y no participa de ninguna identidad, así que `(SELECT 0)` no desbalancea
    nada.

16. **Para decidir si un hallazgo nuevo bloquea la vuelta k+1, el criterio que aguanta es la
    PROCEDENCIA, no la «familia».** La vuelta 2 destapó cuatro mutantes vivos **medidos**
    (`conflicts.go:146`, `:153`, `:157` y `shadow.go:231`) sobre código que el PR ya traía. El
    criterio de «es de otra familia que lo que se abrió» no sirve: es opinable y abusable —con él
    cualquier lente aprueba cualquier mutante vivo llamándolo «aparte»—. El que decide es medible:
    `git show :ruta` contra el disco, y `comm` sobre qué agregó el bucle de corrección. Medido así,
    el bucle agregó **tres** tests y **los tres** llevan directiva `arnes:`; los tres **sin**
    directiva ya estaban en el índice antes de la vuelta 1. Ni introducidos ni reabiertos ⇒ van al
    plan siguiente **con su medición escrita**, no con silencio. Y la razón de fondo para no mover
    la vara a mitad del bucle: con «todo mutante vivo medido bloquea», un juez que sigue midiendo
    siempre encuentra uno más, **K=3 se agota siempre**, y el proceso degenera en rechazo por
    agotamiento automático — que es el «bucle sin salida» de la skill visto del otro lado.

17. ⚠️ **El veredicto es `real` 3–0 y aun así este PR NO satisface literalmente el criterio 2 de su
    propio `proposal.md:103-104`.** El criterio dice: «cada requisito de la spec tiene un custodio
    que se pone rojo bajo sabotaje del invariante que declara, con el sabotaje verificado por
    `diff`». Dos lentes independientes señalaron que la mitad «cuántos quedan fuera» de R1 —que el
    punto 6 de este documento declara «a nivel COMANDO»— **no tiene custodio de valor** en los tres
    baldes de `imprimirCobertura`, y eso está **medido**: permutar `c.PuntaNoVisible`→`c.FirmaAjena`
    (`:146`), `c.AutoAplicadas`→`c.PuntaNoVisible` (`:153`) y `c.Relaciones`→`c.Redetectables`
    (`:157`) deja la suite entera de `cmd/musubi` en verde. El alcance decreciente de la revisión
    gobierna la agenda del **bucle**, no la **aceptación** del PR: si la vara se aplica
    literalmente antes del merge, el PR no está ahí. No lo resuelve ningún tally — es una decisión
    de la persona. Cerrarlo es chico: aserciones de valor en
    `TestCoberturaNombraElMotivoDeCadaRecorte`, cuyo fixture ya trae las cifras.
