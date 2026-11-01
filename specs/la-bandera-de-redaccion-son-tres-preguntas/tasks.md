# Tareas — La bandera de redacción son tres preguntas

Worktree: `C:/Proyectos/musubi-wt-redaccion` · rama `fix/la-bandera-de-redaccion-son-tres-preguntas`
desde `origin/main` (`674d3366`). Compilar con **`GOWORK=off`**, como el CI.

⚠️ **T3–T6 son un bloque que compila sólo al completarse.** No se puede borrar el campo sin
haber cableado sus tres lecturas, así que el árbol queda roto entre T3 y T6. Cada tarea es
cerrable y verificable por separado, pero el `go build` verde es el criterio de T6, no de T3.

## Grupo A — La pieza nueva (compila sola, y se verifica sola)

- [x] **T1 · `arbolAusenteEn(dir string) bool` en `internal/mcp/methods_codegraph.go`**, con el
      comentario de D1 completo: por qué pregunta si hay **archivos indexables** —y no si el
      directorio tiene la firma del central, que es lo que pedía esta tarea y `verify` refutó
      midiendo: esa firma **la planta el propio arranque**— y por qué ante la duda dice
      presente. 🔴 **El directorio vacío cambió de lado con el criterio**: hoy cuenta como
      AUSENTE, no como presente, y eso sólo es sostenible porque el índice re-mide. El
      contrato del «fantasma legítimo» de `codegraph_stale_central_test.go:26-29` sigue
      verde, pero por otro camino: ese test construye con `NewMcpServer` y usa el cero-value.
      **Cierre:** `GOWORK=off go build ./internal/mcp` verde.

- [x] **T2 · Test propio del medidor**, con los cuatro casos de la tabla de D1 como subtests:
      sólo `.musubi` ⇒ ausente · vacío ⇒ presente · con `go.mod` ⇒ presente · ilegible ⇒
      presente. **Cierre:** `GOWORK=off go test ./internal/mcp -run <nombre> -count=1` verde,
      y el caso «sólo `.musubi`» falla si se invierte el `return` final.

## Grupo B — La separación (el bloque que rompe y repara)

- [x] **T3 · Los dos campos en `internal/mcp/server.go`**, reemplazando `forceRedact`
      (`:363-367`): `pozoCompartido` (política) y `arbolAusente` (hecho medido), cada uno con
      su comentario de POR QUÉ y, en `arbolAusente`, la nota de que el cero-value significa
      «hay árbol» a propósito.

- [x] **T4 · Los tres predicados**: `redactaTodoIngest()` y `pisaElScopeExplicito()` sobre
      `pozoCompartido`; `arbolFueraDeAlcance()` pasa a `return s.arbolAusente`. El nombre y
      la firma del tercero **no cambian** — es lo que mantiene intactos sus 5 call sites y la
      directiva de `frescura_cruza_maquinas_test.go:282`.

- [x] **T5 · Las tres lecturas de producción** a su predicado: `methods.go:326` →
      `pisaElScopeExplicito()`, `methods.go:2467` → `redactaTodoIngest()`,
      `methods_codegraph.go:354` → el cuerpo nuevo.

- [x] **T6 · La asignación de `http.go:934` → dos líneas**, y borrar `forceRedact` del struct:
      ```go
      s.pozoCompartido = !loopback || cfg.ForceRedact    // IDÉNTICO al de hoy: fail-closed
      s.arbolAusente   = arbolAusenteEn(s.projectPath)   // medido, una vez
      ```
      **Cierre:** `GOWORK=off go build ./...` verde y `GOWORK=off go vet ./internal/mcp` limpio.
      El compilador debe listar **las 10** asignaciones de test como errores — si lista menos,
      falta alguna por un grep que asumió el receptor `s.`.

## Grupo C — Reapuntar los tests que ya existen

- [x] **T7 · Clasificar las 10 asignaciones con la tabla de D4**, no por el nombre de la
      variable. Las 4 del árbol pasan a `arbolAusente = true`
      (`codegraph_miss_explicado_test.go:369`, `codegraph_missing_test.go:124`,
      `codegraph_stale_central_test.go:33`, `frescura_cruza_maquinas_test.go:249`); las 6
      restantes a `pozoCompartido = true`.
      ⚠️ `recibo_obs_test.go:221` se llama `central` y es **redacción**.
      **Cierre:** `GOWORK=off go test ./internal/mcp -count=1` verde, con los 9 invariantes de
      no-regresión pasando sin que se haya tocado su cuerpo.

- [x] **T8 · Actualizar la directiva del arnés** de `codegraph_miss_explicado_test.go:351-352`
      al cuerpo nuevo del helper. **Cierre:** el `de=` matchea el fuente del disco — se
      comprueba con un `grep -F` del texto del `de=` contra `methods_codegraph.go`, porque un
      `de=` que no matchea no da rojo: da un verde que no mide nada.

- [x] **T9 · Corregir el comentario de `despliegue_cerebro_test.go:198`**, que cita literal
      `s.forceRedact = !loopback || cfg.ForceRedact`. El custodio del #777 sigue valiendo
      entero; lo único que cambia es el nombre del campo que el comentario nombra.

## Grupo D — Los invariantes nuevos

- [x] **T10 · `internal/mcp/tres_preguntas_test.go`** con S1, S2 y S3 de la spec. El que
      importa es **S3**: un server con `pozoCompartido = true` **y** árbol en disco debe
      seguir devolviendo `stale: true` para un archivo cambiado. Es el caso que hoy no se
      puede escribir.

- [x] **T11 · La matriz de sabotaje en las tres direcciones.** Sabotear cada pregunta por
      separado y comprobar que **sólo** caen los tests de esa:
      | Sabotaje | Deben caer | Deben quedar verdes |
      |---|---|---|
      | `redactaTodoIngest()` → `false` | los 6 de redacción | los 4 del árbol, S2, S3 |
      | `pisaElScopeExplicito()` → `false` | el del scope forzado | los del árbol y los de redacción |
      | `arbolFueraDeAlcance()` → `false` | los 4 del árbol, S2 | los 6 de redacción |
      Un test de redacción que cae al sabotear el árbol **no está midiendo la redacción**, y
      con la bandera de hoy es exactamente lo que haría. Es el riesgo que este repo ya midió
      con M39/M50 en `internal/motor`, donde hizo falta sabotear en las dos direcciones para
      poder afirmar que ninguna resolución tapaba a la otra.
      **Cierre:** las tres corridas con su conteo exacto de `--- FAIL:`, y `go vet` en 0 antes
      de cada una para descartar que el rojo sea un sabotaje que no compila.

      **MEDIDO el 2026-10-31** — cada dirección es el predicado a `false`, el paquete `internal/mcp`
      ENTERO con `-count=1`, y antes de cada corrida la doble verificación de que el archivo cambió
      y de que **compila** (`go test -c -gcflags=-e`), porque un rojo de compilación se lee igual que
      un rojo de invariante y no dice lo mismo:

      | Dirección | `--- FAIL:` distintos | Quiénes |
      |---|---|---|
      | **D1** `redactaTodoIngest()` → `false` | **7** | los 5 de redacción (`TestCodegraphPushRedactaConForceRedact` +1 sub, `TestForceRedactCoversAllIngest` +2 sub, `TestLogErrorRedactsAndAttributes`) + **S1** |
      | **D2** `arbolFueraDeAlcance()` → `false` | **13** | los 5 del árbol (`TestCgStaleNotMarkedOnSharedCentral`, `TestElCentralJuzgaLaFrescuraContraElGrafoPublicado` +2 sub, `TestEnElCentralElMissNoAfirmaQueElArchivoEsDeOtraRama`, `TestGraphFreshnessNoJuzgaEnElCentralCompartido`) + **S2** + **H1** (`TestElArbolQueLlegaDespuesDelArranqueSeMide`, +4 sub) + el test de la deuda |
      | **D3** `pisaElScopeExplicito()` → `false` | **3** | **S1**, **S4** y el test de la deuda |

      🔴 **RE-MEDIDA el 2026-11-01, y dos de las tres direcciones se movieron.** La primera
      corrida dio **7 · 8 · 2** y quedó superada por los custodios que entraron después.
      **D2 subió 5** porque el panel adversarial pidió **H1** —el árbol que llega DESPUÉS del
      arranque, que ninguno de los cinco tests originales podía ver— y su custodio cruza
      política × modo: un padre más cuatro subtests, los cinco en rojo bajo el mismo sabotaje.
      **D3 subió 1, y NO por una guarda**: el sabotaje del scope reescribe la línea donde ancla
      la directiva de `tres_preguntas_test.go:384`, así que el censo denuncia daño al corpus
      igual que ya lo hacía en D2. La diferencia entre las dos conviene anotarla: la de D2
      rompe una directiva de `codegraph_miss_explicado_test.go` y la de D3 rompe una
      **registrada en este mismo archivo**. Descontando las dos, las guardas reales por
      dirección son **4 · 6 · 2**, y la base sin sabotear quedó en **0** — que es la mitad que
      suele faltar: sin ella un rojo no distingue «el sabotaje funcionó» de «ya estaba roto».

      **EL CRUCE ES CERO, que era la pregunta:** ningún test de redacción cayó al sabotear el árbol,
      y ninguno del árbol cayó al sabotear la redacción. Las tres preguntas están separadas de verdad,
      no sólo en los nombres de los campos.

      **Pero la matriz destapó lo que esta misma tabla daba por cubierto, y hay que decirlo:** la fila
      del scope prometía «el del scope forzado», en singular, **como si existiera**. No existía. En la
      primera corrida D3 tiró **1 solo** `--- FAIL:` en todo el paquete, y era S1 — que mira el
      predicado, no su efecto. O sea: se podía borrar el forzado del scope y `internal/mcp` entero
      quedaba verde salvo un test que no observa el comportamiento. El motivo es **defensa en
      profundidad**, la trampa que este repo ya tiene nombrada (`internal/motor`, 2026-10-28): el scope
      forzado y la redacción forzada cierran el **mismo hueco por dos caminos**, así que apagar uno
      deja los tests del otro en verde —el contenido se redacta igual— y ninguno distingue cuál actuó.
      Lo único que la redacción **no** toca es el **scope persistido**, y de ahí salió
      **S4 `TestElScopeForzadoSeVeEnLaFilaGuardada`** (no estaba en el plan: lo pidió esta matriz), que
      lee la fila por SQL con `filaObs` y trae su control del escape hatch. Con S4, D3 pasó de 1 a 2.

      **Dos lecturas que el conteo pelado no da, y que cambian lo que significa:**
      · El test de la deuda (`TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre`) cae en D2 **y en D3**
        por **daño al corpus, no por el invariante**: el sabotaje de la matriz reescribe la línea
        entera de la función y destruye el `de=` de una directiva que ancla ahí, que deja de aparecer
        exactamente una vez. En D2 esa directiva vive en `codegraph_miss_explicado_test.go:351` y es
        **ad-hoc de esta matriz**; en D3 vive en `tres_preguntas_test.go:384` y **sí está registrada**.
        Ninguno de los dos rojos hay que leerlo como «esa pregunta tiene un custodio más» — pero el de
        D3 es, además, la prueba de que el censo hace su trabajo sobre el corpus propio.
      · **S1 cae en D1 y D3 a propósito**, porque afirma los tres predicados juntos: custodia que una
        *configuración alcanzable* los separe. Y **no** cae en D2 por una razón que importa: el sabotaje
        del árbol empuja hacia «presente», que es justo lo que S1 espera ahí. Ese hueco lo cubre S2, que
        sí cae en D2 — y es la razón por la que van los dos y no uno.

## Grupo E — Entrega

- [x] **T12 · Viñeta en `CHANGELOG.md` bajo `[Unreleased] → ### Fixed`.** No se toca la
      versión: el bump lo hace el release-cut, como en el #777. ⚠️ Si `main` corta una versión
      mientras esta rama vive, la viñeta se mete **dentro** de la versión tagueada sin dar
      conflicto — hay que mirar dónde quedó antes de pushear.

- [x] **T13 · Suite completa**: `GOWORK=off go test ./... -count=1` (sin `-count=1` el verde
      puede ser cacheado) y el golden de `tools/list` **sin mover**, que es la señal de que no
      se colaron cambios de superficie MCP.

      **MEDIDO el 2026-10-31**, árbol limpio de sabotajes (verificado por compilación después de
      revertir la matriz):
      · `GOWORK=off go test ./... -count=1` → **rc=0**: 35 paquetes `ok`, 5 sin tests, **cero**
        `FAIL` y cero `panic` en las 40 líneas de salida.
      · `GOWORK=off go vet ./...` → **rc=0**, sin una sola línea.
      · `gofmt -l internal/mcp/ internal/config/` → **vacío**.
      · **El golden no se movió**: `git status --porcelain internal/mcp/testdata/` sale vacío, con
        los tres goldens (`contrato-tools`, `toolslist`, `toolslist-completo`) intactos. Era lo que
        había que comprobar: este cambio no toca la superficie MCP, sólo quién decide qué adentro.
      · `-race` **NO CORRIÓ ACÁ, y hay que decirlo porque salió con rc=0**: devolvió
        `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` y terminó en 0 igual — un
        verde que no midió nada, del tipo que este repo persigue. El motivo no es un accidente:
        `go env CGO_ENABLED` da `0` y en esta máquina no hay `gcc`, `clang` ni
        `x86_64-w64-mingw32-gcc`, y el propio CI lo tiene **decidido y escrito**
        (`.github/workflows/ci.yml:180`: Windows va sin `-race` «a propósito: evita depender de
        cgo/gcc en Windows»). Quien sí lo mide es el job canónico de ubuntu
        (`ci.yml:97`, `go test -race -count=1 ... ./...`), y ahí va a pasar este cambio.
        Mientras tanto, el argumento determinista — 🔴 **reescrito en `verify`, porque el que
        estaba acá quedó FALSO y era el peligroso de los dos**. Decía que los dos campos se
        escriben en el arranque y que «la primera goroutine de servicio nace ~80 líneas
        después, antes de que exista cualquier lector». Era cierto **mientras el hecho se
        midiera una sola vez**. Hoy `remedirArbol` lo re-mide desde `musubi_codegraph_index`,
        que declara `lockSelf` —el despachador NO le toma candado—, así que esa re-escritura
        **sí** concurre con los lectores del predicado. Por eso `arbolAusente` es un
        `atomic.Bool` y no un `bool`: el tipo lo hace libre de carrera **por construcción**,
        que es más fuerte que una corrida de `-race`. El costo se midió: **2,66 ns/op** a ocho
        hilos, contra **48,85** de un mutex, que además empeora con la contención.
        `pozoCompartido` sigue siendo un `bool` plano, porque ése sí se escribe una sola vez
        en el arranque, en `http.go:935`.

- [ ] **T14 · `musubi receipt emit`** y PR. El merge es del dueño; el push se pide.

## Lo que queda explícitamente afuera

- El **paso 3** del expediente de `force_redact`.
- Un marcador de origen **por nodo** en `memory.GraphNode`, que es lo que permitiría a un
  cerebro tener nodos propios y federados a la vez. Pide columna nueva: otro PR.
- Darle a la pregunta #2 una clave de config propia. El predicado queda listo para el día que
  aparezca el caso que la pida.
