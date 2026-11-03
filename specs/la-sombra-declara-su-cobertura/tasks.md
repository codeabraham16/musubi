---
artifact: tasks
schema_version: "1.0"
change: la-sombra-declara-su-cobertura
status: implemented
---

# Tareas — la tabla de la sombra declara su cobertura

## Implementación
- [x] T1 — `ShadowCoverageReport` + `ShadowCoverage()` con el SQL derivado · _archivos:_ `internal/memory/shadow.go`
- [x] T2 — Sumar `ShadowCoverage()` a la interfaz · _archivos:_ `internal/memory/backend.go` · _depende de:_ T1
- [x] T3 — La cabecera de cobertura en los DOS caminos (tabla y libro vacío) y el `--json` como objeto · _archivos:_ `cmd/musubi/conflicts.go` · _depende de:_ T2
- [x] T4 — Actualizar la prosa del rastro, que hoy manda a mirar el journal · _archivos:_ `internal/mcp/shadow.go` · _depende de:_ T3

## Pruebas
- [x] T5 — Custodios de R3/R4/R5 y de la aritmética · _archivos:_ `internal/memory/shadow_cobertura_test.go`
- [x] T6 — Custodio de R8: `agree` se computa adentro · _archivos:_ `internal/memory/shadow_cobertura_test.go`
- [x] T7 — Custodios de R6/R7 sobre la salida del comando · _archivos:_ `cmd/musubi/conflicts_cobertura_test.go`
- [x] T8 — Sabotear cada invariante y verificar el ROJO, con el contenido en disco antes de creerle
  al resultado, más el control NOOP · **14/14 con el veredicto esperado**, y encontró dos agujeros
  propios: (a) el sabotaje de R4 pasaba VERDE porque mutar la firma del balde INTERCAMBIABA dos
  filas sin mover los totales —mutación vacua—, y lo cerró desbalancear las cantidades (dos firmas
  propias contra una ajena), y (b) la guarda «el disco cambió» era `git diff --stat`, que lista
  estos archivos SIEMPRE porque el expediente está sin commitear: pasaba igual con el sabotaje sin
  aplicar. Ahora compara el texto releído del disco, y una fila de control con un sabotaje idéntico
  verifica que la guarda siga viva.
- [x] T8b — Pasar los 12 sabotajes al ARNÉS DEL REPO (`// arnes: archivo=/de=/a=` pegadas a la
  prosa de cada prueba), que es el único que los vuelve a correr: un sabotaje que vive en un script
  del scratchpad se cumplió una vez y después no es auditable. El censo del árbol
  (`TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre`) los cuenta: **2.253 → 2.265 anclas, 2.240 →
  2.252 mecanizadas**, y encontró DOS defectos que mi arnés propio no veía:
  - **`gofmt` corrompe la directiva**: convierte `''` en `”` y `` `` `` en `“` dentro de un
    comentario —la regla de comillas tipográficas que heredó de godoc—, así que tres de mis `de`
    dejaron de apuntar a donde decían y sus sabotajes NO SE APLICARÍAN. Reproducido con un archivo
    de cuatro líneas. Se escapan como `\x27`/`\x60`, que `strconv.Unquote` devuelve idénticos y
    gofmt ya no reconoce como par.
  - El censo deriva de `git ls-files`, o sea del ÍNDICE: con los dos archivos de prueba sin
    `git add`, las 12 anclas **no existían para la herramienta** y el conteo no se movía un dígito.
- [x] T8c — Correr los sabotajes LEYENDO LAS DIRECTIVAS DEL REPO y no la tabla del script, que
  desde T8b es un derivado escrito a mano: **12/12 en rojo**, cada uno con su `--- FAIL:` nombrado
  (un rojo de build no cuenta). La primera versión del lector se quedaba con la última directiva de
  cada pila, leía 7 de 12 y reportaba «7/7»: el mismo verde que no mide nada que el arnés caza.

## Docs / cierre
- [x] T9 — `CHANGELOG.md` en `[Unreleased]`: la cobertura, el cambio de forma del `--json` y las DOS
  cifras corregidas.
  ⚠️ **AL REBASAR, ESTE ARCHIVO TIENE DOS DESTINOS Y NINGÚN `--ours`/`--theirs` ACIERTA LOS DOS.** Las
  líneas nuevas van a `[Unreleased]`, pero la fe de erratas (la referencia `conflicts.go:83`→`:107` y
  la cifra «996 de 1.098», que era de más) corrige una viñeta del PR #784 que el release-cut del #787
  ya movió DENTRO de `## [0.148.0]`: tiene que quedar **ahí**, junto a lo que corrige. `--theirs`
  devolvería todo 0.148.0 a `[Unreleased]` y **desharía el release-cut en silencio**. Se resuelve
  construyendo (`origin/main` + las 6 líneas nuevas en `[Unreleased]` + la línea corregida en su
  sitio), y se verifica **a mano, posición por posición**, porque NINGÚN custodio del repo lo caza.
  🔴 **MEDIDO el 2026-11-02, y esta línea afirmaba lo contrario.** `git rebase origin/main` pasó
  « Successfully rebased » SIN pedir resolver nada y enterró las dos viñetas DENTRO de `## [0.148.0]`
  —líneas 43 y 48, con `[Unreleased]` en la 8—, más un `### Changed` DUPLICADO en la misma sección.
  Con ese CHANGELOG puesto en el árbol, `TestChangelogRealTieneLaEstructuraEsperada`,
  `TestReadmeNovedadesAlDia` (y sus dos subtests) y `TestSinPublicarDeLosReadmeReales` dieron los
  cinco **PASS**. Por qué: el README de 0.148.0 se renderiza del bloque CURADO (`### Destacado`,
  `notas-release/main.go:160`) y las viñetas caen en `### Added`/`### Changed`, que no se listan ni
  suman al «Y N más» (`:54`, `:99`); el parser rechaza una VERSIÓN repetida (`:373`) pero no un
  GRUPO repetido, porque `Titulares` es un `map` y los dos `### Changed` se funden; y
  `comprobarSinPublicar` mide el `base=` del bloque opcional del README, no dónde quedó la viñeta.
  Los dos archivos pesan **los mismos 832.024 bytes** —es el mismo texto reubicado—, así que ni el
  tamaño lo delata. **La señal de alarma es la AUSENCIA de conflicto**, y la sonda son las
  POSICIONES: cada viñeta nueva entre `[Unreleased]` y `## [0.148.0]`, la errata DESPUÉS de
  `## [0.148.0]`, y la cantidad de secciones `## [` igual a la de main (139). Un custodio que
  denuncie la viñeta enterrada —y el grupo duplicado— es un cabo del plan siguiente.
  ⚠️ **Y EL CHANGELOG NO VIAJA SOLO: `README.md` y `README.en.md` SE DERIVAN DE ÉL.** Resolver sólo el
  CHANGELOG deja el bloque `<!-- novedades:inicio -->` apuntando a la versión anterior y
  `TestReadmeNovedadesAlDia` cae en los DOS README («tiene: v0.147.0 · 28 oct 2026 / debería:
  v0.148.0 · 2 nov 2026»). Fue el ÚNICO rojo de la suite del 2026-11-02, y no era del delta de este
  PR: era el pre-resuelto a medias. Los dos README se toman de main (`git show origin/main:README.md`)
  y **no hay que correr `-readme`**: el generador IGNORA `[Unreleased]` —sólo abre versión con
  `## [X.Y.Z] - fecha`, ver `notas-release/main.go:307`—, así que ni las 6 líneas nuevas ni la fe de
  erratas de dentro de `[0.148.0]` desfasan el bloque. MEDIDO montando el escenario completo del
  rebase en el worktree: los dos custodios en VERDE, y restaurado byte a byte después (`cmp` de los
  tres archivos). El verde informa porque es el mismo custodio que acababa de morder.
- [~] T10 — Bump menor en `VERSION` — **NO CORRESPONDE, y la tarea estaba mal planteada.** Se había
  hecho (0.147.0 → 0.148.0) y se DESHIZO el 2026-11-02, porque en este repo un PR de feature no toca
  `VERSION`: el bump lo hace el corte de release (`deploy/cmd/preparar-release`). Medido sobre main,
  `9b6446d8` feat, `7f64f14d` fix, `8102caac` feat y `4f37c832` feat **no la tocan**; el único que la
  toca es `d8f10eff chore(release): cortar 0.148.0`. Hacerlo acá además colisionaba con ese cut —los
  dos se apropiaban del 0.148.0— y arrastraba regenerar los dos `.syso` binarios. Al rebasar, los
  cuatro archivos (`VERSION`, `cmd/musubi/versioninfo.json`, los dos `.syso`) heredan los de main y
  salen del diff solos: el change-set bajó de 15 archivos a 11. La regla 7 de `musubi-rules` todavía
  dice «bump semántico», que era la práctica hasta agosto; es un cabo del plan siguiente, no de este PR.
- [x] T11 — Verificar contra la spec: cada R# con su custodio. Dos huecos, los dos cerrados acá
  porque el requisito está DECLARADO y la regla 8 del repo pide el custodio del invariante declarado:
  **R4** tenía uno vacuo en la rama `pending` (permutar la constante pasaba verde en los dos
  paquetes) y **R1/R6/R7** están declarados a nivel COMANDO (`spec.md:12`, `:42`, `:45-46`) y sólo
  tenían custodios de las PIEZAS. **R2** queda declarado SIN custodio y escrito en «Fuera de
  alcance»: no hay sabotaje que compile.
- [x] T13 — (la agregó la verify) Extraer la costura a `emitirShadow(w, cob, res, asJSON)` y
  custodiar los TRES caminos. Tres mutaciones del cableado dejaban `./cmd/musubi/` entero en verde;
  las tres quedan como anclas del arnés, cada una roja en su propio subtest.
- [x] T12 — Puerta completa con `GOWORK=off`, `-tags treesitter`, `-count=1`: `gofmt -l .` limpio,
  `go build` y `go vet` en rc=0, `go test ./...` en **36 paquetes `ok` + 5 sin tests, 0 FAIL** — los
  tres del PR: `cmd/musubi` 122,9 s · `internal/mcp` 186,0 s · `internal/memory` 99,4 s —, y el arnés
  `-validar` en rc=0 con «✓ las 2256 directivas apuntan a un literal que existe y es único».
  🔄 **RE-CORRIDA SOBRE EL ÁRBOL FINAL el 2026-11-02**, porque la puerta de arriba se midió antes de
  deshacer el bump de T10 y una suite vieja no juzga el árbol que se entrega: `go test ./...` en
  **36 `ok` + 5 sin tests = 41, que es exactamente `go list ./...`** (la suma importa: prueba que no
  faltó un paquete) y **0 líneas `FAIL`** — `cmd/musubi` 442,8 s · `internal/mcp` 949,6 s ·
  `internal/memory` 635,2 s, más lentos que arriba por correr contra otras mediciones en la misma
  máquina. `gofmt -l .` limpio. Arnés `-validar` rc=0.
  🔴 **EL TOTAL ABSOLUTO DE ANCLAS CADUCA —no lo clavo más.** Fue 2256, después 2263, y sobre el
  árbol que se entrega (`703b16e0`, medido por el arnés en la puerta final) es **2292 anclas · 2279
  mecanizadas · 0 ROTAS · 12 exentas · 1 en prosa · 99,4 % ejecutable**. No cambió nada mal: entre
  `078fc784` (donde se midió 2263) y `703b16e0` **no cambió un solo `.go`**, así que el 2263 era de
  ANTES del rebase y caducó cuando éste trajo las anclas de `#786` (`methods_spec_coverage_test.go`,
  40). Es la lección [id:f41e3a63]: un total sobre algo que crece caduca. Lo que **no** caduca y es
  lo que este PR controla: aporta **84 directivas `// arnes:`** — 27 en `conflicts_cobertura_test.go`
  y 57 en `shadow_cobertura_test.go` —, **0 ROTAS**, y la cobertura ejecutable se mantiene en 99,4 %.
  ⚠️ **Y SE VERIFICÓ CON EL ÍNDICE PUESTO, QUE ES LO QUE VE EL CI.** Dos guardas del repo derivan de
  `git ls-files` y no del disco, así que con los archivos en `AM` medían de menos: el índice tenía
  15 y 40 líneas `arnes:` donde el disco tenía 27 y 57. Con `git add -A` (seguro acá: cero untracked)
  las dos quedaron en VERDE —`TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre` (`internal/mcp`,
  PASS 6,1 s, «1 en prosa (techo 1)», justo en el techo) y
  `TestNingunaPruebaAveriguaQueHayEnElRepoCaminandoElDisco` (`internal/arbol`, PASS)— y el censo pasó
  a COINCIDIR con el arnés (los dos sobre el mismo índice), que antes del `add` no coincidía —el
  valor exacto es el de arriba y caduca; lo que importa es que coinciden—. Ninguno de los dos archivos
  nuevos usa `filepath.WalkDir`, que es lo que habría enrojecido la segunda.
  La **bidireccionalidad** de la colisión de `shadow_cobertura_test.go` quedó MEDIDA y no razonada:
  quitar el `colision_ok` del `1 = 0` hace aparecer «:57 pisa a :64», y quitar el de la permutación
  hace aparecer «:65 pisa a :57». Las dos direcciones son reales, así que ninguna de las dos
  declaraciones es rancia. Y el arnés imprime UNA dirección por vez —la otra ya está silenciada—,
  o sea que el aviso visible es una **cota inferior** del número de direcciones y no el número.
  🔄 **Y UNA TERCERA CORRIDA, SOBRE EL ÁRBOL REBASADO —el que se entrega— el 2026-11-02.**
  `go test ./...` sobre `078fc784` dio **4 `--- FAIL` en `cmd/musubi`** y `rc_real=1`, y **ninguno
  era del delta**: son pruebas con presupuesto de RELOJ que cayeron por contención de la máquina.
  Aisladas pasan las cuatro: `TestElBucleSeDetieneAlSerRevocado` 5,02 s → **0,32 s** ·
  `TestCerebroSaludaInicializacionSinDiscar` 4,41 s → **0,40 s** ·
  `TestUnExitDistintoDeCeroEsResultadoNoError` 6,63 s → **0,21 s** —su propio mensaje dice
  «excedió su timeout de 5s»— · `TestNoHayShellImplicito` 5,82 s → **0,09 s**. Factores de 11× a
  64×. Y el paquete ENTERO, solo y con la máquina quieta: **`ok musubi/cmd/musubi 108,764 s`**,
  0 FAIL, contra los **938 s** de la corrida saturada — 8,6×. El log de esa corrida trae además un
  `powershell: no terminó en 5s y hubo que matarlo`, que es la misma causa vista de costado. El
  control que descarta el delta es la corrida de arriba: paquete entero, delta puesto, `ok` 442,8 s.
  ⚠️ **Y LA NOTIFICACIÓN DE FIN DE TAREA DIJO `exit code 0` LAS TRES VECES**, incluidas las dos en
  que el archivo decía `rc_real=1`: el canal que avisa y el canal que mide son distintos. La sonda
  es `grep -v '^ok ' | grep -v 'no test files'` sobre el ARCHIVO, y la suma `ok + sin tests` tiene
  que dar los **41** de `go list ./...` — con un paquete en FAIL la suma da 40, que es cómo se ve
  el rojo en el conteo. `grep -c '^ok'` no alcanza, y `grep '^FAIL'` sin el `-v` se come los
  `--- FAIL:` indentados.

## Forecast de review
- Líneas estimadas: ~450 (≈120 de código, el resto tests y specs) · ¿Chained PRs recomendado? **no**:
  es una sola idea y partirlo dejaría la cabecera sin su consulta.
