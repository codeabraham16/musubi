# Tasks — S14 · El cruce con la memoria (fase 5, slice 2)

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | El dominio: `Termino`, `OrigenDeTermino`, `TerminosDeContexto`, `Enlace`, `HuecosDelContexto`, los topes y el recorte de sufijos de unidad | `internal/fleet/contexto.go` |
| T2 | Las dos lecturas de memoria con ventana: `ObservacionesEnVentana` (con el predicado canónico) y `CodigoTocadoEnVentana` | `internal/memory/contexto.go` |
| T3 | `parseFechaDeMemoria`: acepta los DOS formatos, porque el driver convierte al leer y no al comparar | `internal/memory/contexto.go` |
| T4 | Los dos métodos en la interfaz del motor | `internal/memory/backend.go` |
| T5 | La tool `musubi_fleet_contexto` (readOnly) | `internal/mcp/methods_contexto.go`, `internal/mcp/registry.go` |
| T6 | `hechosVisiblesPara` y `resolverDeviceUnico` EXTRAÍDAS de S13 para que las dos tools compartan UNA compuerta y UNA resolución | `internal/mcp/methods_cronologia.go`, `internal/mcp/methods_contexto.go` |
| T7 | Las cinco guardas que se ponen rojas solas al agregar una tool readOnly, y el golden regenerado | `read_concurrency_test.go`, `read_surface_class_test.go`, `motor_sin_candado_test.go`, los dos README, `testdata/toolslist.golden.json` |

## Invariantes

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **K2** | `TestElEnlacePorTerminoNoSeConfundeConElDeVentana` | que el enlace por ventana pise al de término → ✅ falla |
| **K3** | `TestElContextoDeclaraQueEsCorrelacionYNoCausa` · `TestLosHuecosDelContextoDicenQueNoEsCausa` | devolver nil desde `HuecosDelContexto` → ✅ fallan |
| **K5** | `TestElNombreDeLaMaquinaSiempreEsUnTermino` | agregar los servicios antes que la máquina → ✅ falla |
| **K6** | `TestElSufijoDeUnidadNoLlegaALaBusqueda` | no recortar el sufijo; recortar puntos genéricamente (`.altura`) → ✅ fallan los dos |
| **K7** | `TestLosTerminosDeServicioSeCompuertanConMetrics` | armar los términos sin preguntar por `metrics`; hacer que `servicios_ocultos` diga false siempre → ✅ fallan |
| **K8** | `TestElContextoSaleDeLaMemoriaDeLaMaquinaYNoDeLaDeQuienPregunta` · `TestLasLecturasDeContextoRespetanElProyecto` · `TestReadSurfaceClassIsolation` (marcador `VICTIMOBS`) | usar `s.scopedCtx(ctx)`; sacar el scope de cada una de las dos consultas → ✅ fallan |
| **K9** | `TestElContextoNoTraeObservacionesTapadas` | sacar `visibleObsPredicate` del WHERE → ✅ falla |
| **K10** | `TestLaActividadDelContextoSeCompuertaComoLaCronologia` | contar los hechos sin pasar por `hechosVisiblesPara` → ✅ falla |
| **K11 · K12** | `TestLaVentanaDeMemoriaUsaElFormatoDeSQLiteYNoRFC3339` | comparar la ventana en RFC3339; aceptar sólo el formato crudo al parsear → ✅ fallan los dos |
| — | `TestLosTerminosNoSeRepiten` · `TestUnTerminoDemasiadoCortoNoEntra` | deduplicar sensible a mayúsculas; contar el mínimo en bytes → ✅ fallan |
| — | `TestUnaNotaLargaSeRecortaConMarca` | recortar sin dejar la marca → ✅ falla |
| — | `TestElCruceTraeMemoriaCodigoYActividad` · `TestElContextoNoEsUnOraculoDeMaquinasAjenas` · `TestUnaVentanaVaciaSigueDiciendoQueNoMiro` · `TestLasLecturasDeContextoRechazanUnaVentanaInvalida` | — |

**18 sabotajes ejecutados en este slice.** Tres no rompieron a la primera, y los tres fueron
**pruebas mías demasiado flojas**, no código débil:

- **`U`** (comparar la ventana en RFC3339) quedaba verde con una ventana de 24 h: `2026-08-30
  13:50:03` y `2026-08-30T12:50:03Z` caen del mismo lado porque **manda la fecha, no la hora**. La
  guarda se rehízo en `internal/memory` con la hora FIJADA y una ventana del mismo día, donde el
  espacio contra la `T` sí decide.
- **`AC`** (sacar el predicado de visibilidad) quedaba verde porque **ninguna prueba sembraba una
  observación tapada**. La Muralla 2 estaba sin custodiar por esta puerta.
- **`AH`** (contar el mínimo en bytes) quedaba verde porque elegí «año»: 4 bytes y 3 runas, que
  pasa contando de las dos formas. Con «ño» —3 bytes, 2 runas— las dos cuentas se separan.

## Lo que este slice deja abierto

| # | Qué | Registro |
|---|---|---|
| A61 | Los dos formatos de fecha conviven en la misma base. Unificarlos sería mejor y no se hace acá: tocar cómo se escribe `created_at` afecta a nueve consultas de recall que hoy andan | `specs/control-de-flota/ABIERTO.md` |
