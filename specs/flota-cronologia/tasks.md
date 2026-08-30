# Tasks — S13 · La cronología de una máquina (fase 5, slice 1)

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | El dominio: `Ventana` (+`Valida`, `Contiene`, `Normalizada`, `VentanaHasta`), `Hecho`, `TipoDeHecho`, `PlanoDeFlota`, `CapDeHecho`, `PlanoDeHecho`, `TipoDeArgv`, `OrdenarHechos`, `HuecosDeLaCronologia` | `internal/fleet/cronologia.go` |
| T2 | Los nombres de las operaciones internas (`OpPantalla`, `OpAvisar`, `OpPreguntar`, `OpShell`, `PrefijoOperacionInterna`) bajan al dominio; los tres constantes de `mcp` pasan a aliasarlos | `internal/fleet/cronologia.go`, `internal/mcp/methods_pantalla.go`, `internal/mcp/methods_shell.go` |
| T3 | El saneo del argv se unifica en `fleet.ArgvDeBitacora`; `ocultarArgvDePantalla` delega y deja de ser una segunda implementación | `internal/fleet/cronologia.go`, `internal/mcp/methods_pantalla.go` |
| T4 | El almacén: `CronologiaDeDevice` con la ventana en el `WHERE` de las tres consultas, tope por fuente y después al total | `internal/memory/cronologia.go` |
| T5 | El método en la interfaz del motor | `internal/memory/backend.go` |
| T6 | Las fechas de flota se guardan normalizadas a UTC: la comparación léxica pasa de costumbre a regla | `internal/memory/comandos.go`, `sesiones.go`, `shell.go`, `devices.go`, `servicios.go` |
| T7 | La tool `musubi_fleet_cronologia` (readOnly): resolución de máquina por proyecto, compuerta por hecho, los tres contadores y `no_visto` | `internal/mcp/methods_cronologia.go`, `internal/mcp/registry.go` |
| T8 | Las cinco guardas que se ponen rojas solas al agregar una tool readOnly, y el golden regenerado | `read_concurrency_test.go`, `read_surface_class_test.go`, `motor_sin_candado_test.go`, los dos README, `testdata/toolslist.golden.json` |
| T9 | `respText` del barrido de aislamiento deja de abortar ante un `RpcError` y verifica el marcador TAMBIÉN en el mensaje de error | `internal/mcp/read_surface_class_test.go` |
| T10 | **`Comando.EstadoActual`**: el vencimiento de un comando se DERIVA al leer, cableado en las DOS superficies. `Comando.Vencido` existía desde S5 sin ningún llamador | `internal/fleet/comando.go`, `internal/memory/cronologia.go`, `internal/mcp/methods_exec.go` |

## Invariantes

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **C3** | `TestUnaOperacionDePantallaNoSeLeMuestraAQuienSoloPuedeEjecutar` | que `TipoDeArgv` devuelva siempre `HechoComando` (clasificar por tabla) → ✅ falla |
| **C4** | `TestLaCompuertaEsPorHechoYNoPorLaLista` | compuertar la lista entera con `fleet.CapExec` → ✅ falla en las dos direcciones |
| **C5** | `TestCadaTipoDeHechoMostrableTieneCapacidadYPlano` · `TestUnaOperacionInternaDesconocidaNoSeLeMuestraANadie` · `TestUnaOperacionInternaNuevaNoSeMuestraYSeCuentaAparte` | darle un `case` a `HechoSinClasificar` en `CapDeHecho`; devolver `HechoComando` para lo desconocido → ✅ fallan |
| **C5** | `TestTodaOperacionInternaDelCodigoEstaClasificada` | renombrar el `case OpShell` a una operación inexistente → ✅ falla nombrando el archivo |
| **C6** | `TestSinNingunPlanoVisibleSeExplicaEnVezDeDevolverVacio` | sacar la guarda `algunPlanoVisible` → ✅ falla |
| **C7** | `TestLaCronologiaDeUnaMaquinaRevocadaSeSigueLeyendo` | usar `PuedeSobreDevice` en vez de `PuedeVerHistorialDeDevice` → ✅ falla |
| **C8** | `TestLaCronologiaNoEsUnOraculoDeMaquinasAjenas` · `TestReadSurfaceClassIsolation` (caso hostil, marcador `VICTIMSCRIPT`) | sacar `project_id = ?` del `WHERE` → ✅ falla el de tenants |
| **C9** | `TestUnaVentanaInvalidaNoSeConvierteEnTraemeTodo` · `TestLaCronologiaRechazaUnaVentanaInvalida` | que `Valida` devuelva nil; sacar la validación del motor → ✅ fallan |
| **C10** | `TestLaVentanaSeAplicaEnLaConsultaYNoDespues` · `TestLaVentanaViajaEnLaConsultaYNoEnGo` | sacar `AND creado >= ? AND creado < ?` del `WHERE` (dejándolo compilando) → ✅ fallan |
| **C11** | `TestLaVentanaEsSemiabierta` | usar `!t.After(v.Hasta)` en `Contiene` → ✅ falla |
| **C12** | `TestLaVentanaSeNormalizaHaciaAfuera` · `TestLoQueAcabaDePasarEntraEnLaVentana` · `TestLaCronologiaIncluyeLoQueAcabaDePasar` | truncar las dos puntas hacia abajo; sacar `Normalizada` del motor y de la tool → ✅ fallan (con UNA sola de las dos quitadas NO falla: cada capa tiene su guarda, y por eso hay dos pruebas) |
| **C13** | `TestLaVentanaQueVuelveEsLaQueSeAplico` | devolver los argumentos crudos en vez de la ventana normalizada → ✅ falla |
| **C14** | `TestElTopeQueCortaSeDeclara` · `TestUnaOperacionInternaNuevaNoSeMuestraYSeCuentaAparte` | devolver `truncado: false` siempre; sumar lo sin clasificar a `ocultos_por_permiso` → ✅ fallan |
| **C15** | `TestLaCronologiaDeclaraLoQueNoVio` | devolver nil desde `HuecosDeLaCronologia` → ✅ falla |
| **C16** | `TestLaDuracionDiceSiSeSabe` | devolver sólo la duración, sin el booleano → ✅ falla |
| **C17** | `TestElArgvDeBitacoraNuncaLlevaLaContrasena` · `TestUnaOperacionDePantallaNoSeLeMuestraAQuienSoloPuedeEjecutar` | que `ArgvDeBitacora` devuelva el argv tal cual → ✅ falla |
| **C18** | `TestUnComandoPendienteYViejoSeMuestraExpirado` · `TestLasDosSuperficiesMuestranVencidoUnComandoQueNadieLevanto` | que `EstadoActual` devuelva `c.Estado`; volver a poner el crudo en la bitácora; sacar la derivación de la cronología; devolver `expirado` siempre (control positivo) → ✅ fallan los cuatro |
| — | `TestOrdenarHechosEsEstableYDelMasNuevoAlMasViejo` | sacar el desempate por referencia → ✅ falla |
| — | `TestLasOperacionesInternasDeHoyEstanClasificadas` · `TestLaCronologiaCruzaLosTresPlanos` · `TestHorasConDesdeOHastaEsUnError` · `TestVentanaHastaAplicaLosDefaults` | dejar que `horas` le gane a `desde` en silencio → ✅ falla |

**26 sabotajes ejecutados.** Dos no rompieron a la primera y los dos enseñaron algo:

- **`I`** (sacar `Normalizada` del motor) no rompió porque la tool también normaliza. La defensa
  doble es correcta —`CronologiaDeDevice` es parte de la interfaz del motor y la puede llamar
  alguien que no pase por la tool—, pero la prueba sólo cubría una de las dos capas. Se agregó
  `TestLaCronologiaIncluyeLoQueAcabaDePasar` en `internal/memory` para que cada línea tenga su
  guarda.
- **`K` y `L`** rompieron por no COMPILAR la primera vez (variables sin usar), que no prueba nada.
  Se rehicieron con `_, _ = desde, hasta` para que el sabotaje compile y sea la PRUEBA la que
  falle.

## Lo que este slice deja abierto

| # | Qué | Registro |
|---|---|---|
| A59 | La bitácora no distingue el origen AUTOMÁTICO del manual: una política y una persona escriben en la misma tabla con la misma forma, y la diferencia se lee del nombre del principal por convención | `specs/control-de-flota/ABIERTO.md` |
| A60 | Un comando `entregado` que nunca reporta se queda así para siempre. No se puede derivar con la regla de `pendiente`: su reloj es el `timeout` del comando, no `ComandoVidaMax` | `specs/control-de-flota/ABIERTO.md` |
