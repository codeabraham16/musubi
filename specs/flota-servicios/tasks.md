# Tasks — S12 · La entidad Servicio

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | El dominio: `EstadoServicio` (los cuatro, y el cuarto es el que importa), `SaludServicio`, `Servicio`, `ReporteServicio`, `EstadoActual()`, `Fresco()`, `RecortarReporte()`, los techos y las validaciones | `internal/fleet/servicio.go` |
| T2 | La migración **36** (`services_inventario_por_maquina`), apendeada tras la 35, con `IF NOT EXISTS` en la tabla y en los tres índices | `internal/memory/migrations.go` |
| T3 | El almacén: alta a mano, upsert del agente, listado aislado, poda por ausencia, revocación | `internal/memory/servicios.go` |
| T4 | El rol `ServiceStore`, embebido en `StorageBackend` | `internal/memory/backend.go` |
| T5 | `RevocarDevice` arrastra los servicios de la máquina, en la MISMA transacción | `internal/memory/devices.go` |
| T6 | La puerta del dispositivo: el bloque `servicios` del latido, su techo propio (el de la muestra sigue siendo suyo), la compuerta `metrics`, la poda y la NOTA de vuelta al agente | `internal/mcp/fleet_http.go` |
| T7 | Las dos tools: `musubi_fleet_services` (readOnly) y `musubi_fleet_service_declare` (admin) | `internal/mcp/methods_servicios.go`, `internal/mcp/registry.go` |
| T8 | Las cinco guardas que se ponen rojas solas al agregar una tool readOnly, y el golden regenerado | `read_concurrency_test.go`, `read_surface_class_test.go`, `motor_sin_candado_test.go`, los dos README, `testdata/toolslist.golden.json` |
| T9 | La columna del panel, con sus tres estados y las cuatro marcas distintas | `cmd/musubi/flota.go`, `cmd/musubi/assets/flota.html` |

## Invariantes

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **S1** | `TestUnServicioSinSaludEstaDesconocidoYNoDetenido` · `TestUnServicioSinSaludSeInformaDesconocidoYNoDetenido` | en `EstadoActual`, devolver `EstadoDetenido` cuando `Salud` es nil → ✅ fallan las dos |
| **S2** | `TestUnServicioDeclaradoYSinMedirEsUnEstadoLegitimo` | hacer que `SaludDesdeTexto("")` devuelva error → ✅ falla |
| **S3** | `TestUnServicioConNoticiasViejasDejaDeEstarFrescoSinCambiarDeEstado` · `TestFrescoConRelojCeroNoInventaVida` | sacar la guarda de `UltimoReporte.IsZero()` de `Fresco` → ✅ falla |
| **S4** | `TestElProyectoDelServicioSaleDelDeviceYNoDelPedido` | copiar `s.ProjectID` del argumento en vez de resolver el device → ✅ falla |
| **S5** | `TestUnaMaquinaNoPuedeReportarLosServiciosDeOtra` | sacar el `AND device_id = ?` del UPDATE → ✅ falla |
| **S6** | `TestDeclararUnServicioEnLaMaquinaDeOtroTenantNoRevelaQueExiste` · `TestFiltrarPorUnaMaquinaQueNoVesRespondeVacioYNoUnError` · `TestElAltaSobreUnaMaquinaRevocadaNoRevelaQueExistio` | usar `args.Project` directo en vez de `writeOriginFor` → ✅ falla |
| **S7** | `TestUnPrincipalSinMetricsNoVeLosServiciosDeEsaMaquina` · `TestReadSurfaceClassIsolation` (caso hostil `{"project":"web"}`, marcador `VICTIMSERVICIO`) | filtrar sólo por proyecto y no llamar a `PuedeSobreDevice` → ✅ fallan las dos |
| **S8** | `TestNoExisteColumnaDeEstadoEnServices` · `TestElServicioNoGuardaUnEstadoDerivable` | agregarle a la migración una columna `healthy INTEGER` → ✅ falla |
| **S10** | `TestDosMaquinasPuedenTenerCadaUnaSuPostgres` | declarar el único como `(project_id, name)` → ✅ falla |
| **S12** | `TestRevocarUnaMaquinaSacaSusServiciosDelListado` | dejar `RevocarDevice` como estaba → ✅ falla |
| **S13** | `TestUnReporteDeServicioNoTienePorDondePasarIdentidad` · `TestUnCuerpoConServiciosSigueSinLlevarIdentidad` | renombrar el tag `nombre` a `name` en `ReporteServicio` → ✅ fallan las dos |
| **S14** | `TestUnLatidoConDemasiadosServiciosDescartaElBloqueYSigueValiendo` · `TestUnCuerpoDemasiadoGrandeSeRechazaSinTumbarElLatido` | devolver 400 cuando el bloque se pasa del techo; medir la muestra después de deserializar → ✅ fallan |
| **S14c** | `TestElAgenteSeEnteraDeQuePasoConSuInventario` | que la nota devuelva `""` en la rama de la capacidad que falta, o en la del techo → ✅ fallan |
| **S15** | `TestPodarServiciosAusentesConListaVaciaNoBorraNada` · `TestLaPodaPorAusenciaCorreDesdeElLatidoYUnLatidoMudoNoVaciaNada` | quitar el early-return de `len(vivos) == 0` → ✅ fallan las dos |
| **S16** | `TestLaPaginaDeFlotaNoDibujaUnServicioDesconocidoComoDetenido` · `TestLosServiciosSeAgrupanPorSuMaquina` | darle a `desconocido` la misma marca que a `detenido` → ✅ falla |
| **S17** | `TestSiFallaLaToolDeServiciosIgualSeVeLaFlota` | propagar el error de la tercera llamada desde `handlerFlota` → ✅ falla |
| — | `TestMigracionV36CreaLosServiciosSinPerderLaFlota` | declararla con `version: 35` → ✅ falla |
| — | `TestUnaSaludIlegibleNoRompeElListado` | devolver el error de `SaludDesdeTexto` desde `escanearServicio` → ✅ falla |
| **S14b** | `TestLaUltimaSaludViveEnLaFilaYNoSeBorraSola` · `TestUnReporteInvalidoNoSeLlevaPuestosALosDemas` | escribir `last_health = ?` directo en vez del CASE; y descartar el reporte con salud ilegible en vez de guardarlo `desconocido` → ✅ fallan |
| — | `TestUnServicioRevocadoNoResucitaNiSigueRecibiendoTelemetria` | sacar el `AND revoked = 0` del UPDATE del upsert → ✅ falla |
| — | `TestDeclararUnServicioExigeAdmin` | sacar el `p.isAdmin()` de `toolFleetServiceDeclare` → ✅ falla |
| — | `TestUnaMaquinaSinMetricsNoRegistraServiciosPeroLateIgual` | sacar la compuerta `CapMetrics` de `guardarServiciosDelLatido` → ✅ falla |

## 🔴 Las tres pruebas que NO sabían fallar, y qué escondían

El paso de sabotaje encontró **tres pruebas decorativas escritas en este mismo slice**. Ninguna de
las tres se arregló bajando la vara: las tres escondían algo real.

1. **El `CASE WHEN salud = ''` era código MUERTO.** Un reporte con la salud inválida se descartaba
   antes de llegar al UPDATE, así que la rama que conserva la última medición no se ejecutaba
   nunca — y la prueba pasaba igual con el CASE borrado. El arreglo NO fue la prueba: fue el
   diseño (**S14b**). Un nombre bueno con una salud ilegible ahora registra el servicio como
   `desconocido` en vez de desaparecer, que es lo que hace falta el día que `systemctl show` falle
   por permisos en una máquina que sí sabe listar sus units.
2. **`AND revoked = 0` no lo custodiaba el listado.** Sin la cláusula, el UPDATE encuentra la fila
   revocada y le pisa `last_report` y `last_health` — pero **no la resucita**, así que el listado
   sigue vacío y la prueba seguía verde. Queda una fila dada de baja acumulando mediciones frescas
   que nadie mira, y el día que alguien la reactive lee un estado viejo como si fuera de ahora. La
   prueba ahora mira la FILA.
3. **La compuerta `metrics` de la PUERTA la tapaba la de la LECTURA.** Con el gate del latido
   borrado, la fila se escribía igual y `musubi_fleet_services` la filtraba después por su propia
   compuerta: verde probando la defensa equivocada, con la escritura sin permiso pasando
   desapercibida. La prueba ahora pregunta por `ServiciosDeDevice`, que no compuertea.

## Lo que queda fuera

- **El agente no ENUMERA servicios todavía** (**A42**). Leer systemd (D-Bus o `systemctl show`), el SCM de
  Windows o el socket de Docker es un slice propio, con su seam por OS y su tabla de capacidades,
  igual que los colectores. S12 define el modelo, el almacén, la puerta y la vista; lo que reporta
  hoy es lo que se declara a mano o una máquina que ya sepa mandar el bloque.
- **Los servicios no se exportan a Prometheus ni tienen reglas de alerta** (**A43**).
  `fleet_prometheus.go` no se toca. Si el objetivo es enterarse cuando un bot se cae, la mitad del
  valor está ahí — y hay que decirlo por adelantado en vez de descubrirlo cuando el primer servicio
  muera en silencio.
- **No hay política de flota sobre la salud de un servicio** (**A44**). `fleet_policy_state` tiene clave
  `(policy, device_id)` y una política sobre un SERVICIO no entra en esa clave sin migrar la tabla.
  Anotarlo ahora es barato; cerrarse la puerta, no.
- **No se guarda historia ni bitácora de transiciones** (**B5**). Musubi guarda el presente; la historia la
  guarda Prometheus.
- **No hay foreign keys** — **por diseño**: serían las primeras del repo y sólo para esta tabla.
- **No se inventó una `fleet.Cap` nueva** — **por diseño**: leer = `CapMetrics`; accionar = `CapExec`,
  que ya tiene la allowlist por argv encima.
- **No se agregaron botones al panel** — **por diseño**: el invariante I4 de `specs/flota-panel/spec.md`
  no se toca en un slice de visualización.
- **No se tocó el bundle WebGL** — **por diseño**: la tabla va en HTML plano, en `flota.html`.
- **Cero dependencias nuevas**: todo con la biblioteca estándar y el `uuid` que ya estaba.
