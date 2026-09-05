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

## A59, abierto y cerrado el mismo día

| # | Qué | Dónde |
|---|---|---|
| T11 | Migración **41** (`device_commands.origen`), `OrigenComando` con su lista blanca, y el campo en `Hecho` | `internal/memory/migrations.go`, `internal/fleet/comando.go`, `internal/fleet/cronologia.go` |
| T12 | El origen se marca en los CINCO lugares que encolan: exec, shell, pantalla (×3) y el motor de políticas | `internal/mcp/methods_exec.go`, `methods_shell.go`, `methods_pantalla.go`, `politicas.go` |
| T13 | `origen` y `automatico` en las DOS superficies, en null cuando no se sabe | `internal/mcp/methods_cronologia.go`, `internal/mcp/methods_exec.go` |

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **C19** | `TestUnOrigenDesconocidoNoEsPersonaNiAutomatico` · `TestElOrigenAutomaticoSeDistingueYLoDesconocidoNoSeInventa` | dibujar lo desconocido como persona en la cronología; y en la bitácora; hacer `EsAutomatico` lista NEGRA → ✅ fallan |
| **C20** | `TestLaAccionDeUnaPoliticaQuedaEnLaMismaBitacoraQueLasPersonas` | quitarle `Origen: OrigenPolitica` al motor de políticas → ✅ falla |
| **C21** | `TestElOrigenAutomaticoSeDistingueYLoDesconocidoNoSeInventa` | quitarle el origen a `musubi_fleet_exec`; no leer la columna al escanear → ✅ fallan |
| — | `TestUnOrigenRaroSeGuardaComoDesconocido` · `TestElHechoArrastraElOrigenDelComando` | devolver el valor tal cual desde `OrigenValido`; no copiar `Origen` en `HechoDeComando` → ✅ fallan |

**Ocho sabotajes más.** Tres enseñaron lo mismo que ya venía enseñando el día: **probar el CABLEADO
y no el campo**. Sembrar el comando a mano con el origen puesto verifica que el campo viaja, no que
alguien lo setea donde tiene que setearlo — así que quitarle el origen al motor de políticas
quedaba verde. La verificación se movió al test que ya recorría el camino entero, y el control de
`persona` pasó a entrar por `musubi_fleet_exec`.

**Y un tropiezo del método**: la tanda se pasó del tiempo ANTES de restaurar el último sabotaje, y
quedó aplicado en el árbol. Lo cazó el test siguiente, que falló por un motivo que no tenía nada
que ver con lo que probaba. Un sabotaje que no se revierte es un bug introducido a mano.
| A60 | Un comando `entregado` que nunca reporta se queda así para siempre. No se puede derivar con la regla de `pendiente`: su reloj es el `timeout` del comando, no `ComandoVidaMax` | `specs/control-de-flota/ABIERTO.md` |
