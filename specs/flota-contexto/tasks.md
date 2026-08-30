# Tasks — S14 · El cruce con la memoria (fase 5, slice 2)

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | El dominio: `Termino`, `OrigenDeTermino`, `TerminosDeContexto`, `Enlace`, `HuecosDelContexto`, los topes y el recorte de sufijos de unidad | `internal/fleet/contexto.go` |
| T2 | Las dos lecturas de memoria con ventana: `ObservacionesEnVentana` (con el predicado canónico) y `CodigoTocadoEnVentana` | `internal/memory/contexto.go` |
| T3 | `parseFechaDeMemoria`: acepta los DOS formatos, porque el driver convierte al leer y no al comparar | `internal/memory/contexto.go` |
| T4 | Los dos métodos en la interfaz del motor | `internal/memory/backend.go` |
| T5 | La tool `musubi_fleet_contexto` (readOnly) | `internal/mcp/methods_contexto.go`, `internal/mcp/registry.go` |
| T8 | **`ObservacionesQueNombran`**: el enlace por término busca la FRASE, no el OR de sus tokens | `internal/memory/contexto.go` |
| T9 | **Los servicios DECLARADOS por una persona van primero** entre los términos: un host enumera decenas de units y el tope se llenaba con ellas | `internal/fleet/contexto.go`, `internal/mcp/methods_contexto.go` |
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
| **K13** | `TestElEnlacePorTerminoBuscaLaFraseYNoSusPedazos` | volver a `SearchObservationsFTS` (OR de tokens); armar la frase uniendo con OR → ✅ fallan los dos |
| **K14** | `TestUnServicioDeclaradoGanaLaRanuraAntesQueUnaUnitDelSistema` · `TestElServicioDeclaradoLlegaALosTerminosAunqueElHostEnumereMuchos` | recorrer los reportados antes que los declarados; mandar todos a `reportados` sin mirar `sv.Declarado` → ✅ fallan |
| **K15** | `TestElFragmentoMuestraDondeAparecioElTermino` | volver a `o.content` (el principio de la nota) en vez de `snippet()` → ✅ falla |
| — | `TestElCruceTraeMemoriaCodigoYActividad` · `TestElContextoNoEsUnOraculoDeMaquinasAjenas` · `TestUnaVentanaVaciaSigueDiciendoQueNoMiro` · `TestLasLecturasDeContextoRechazanUnaVentanaInvalida` | — |

**23 sabotajes ejecutados en este slice.** Cinco no rompieron a la primera, y los cinco fueron
**pruebas mías demasiado flojas**, no código débil — que es peor, porque no se nota:

- **`U`** (comparar la ventana en RFC3339) quedaba verde con una ventana de 24 h: `2026-08-30
  13:50:03` y `2026-08-30T12:50:03Z` caen del mismo lado porque **manda la fecha, no la hora**. La
  guarda se rehízo en `internal/memory` con la hora FIJADA y una ventana del mismo día, donde el
  espacio contra la `T` sí decide.
- **`AC`** (sacar el predicado de visibilidad) quedaba verde porque **ninguna prueba sembraba una
  observación tapada**. La Muralla 2 estaba sin custodiar por esta puerta.
- **`AH`** (contar el mínimo en bytes) quedaba verde porque elegí «año»: 4 bytes y 3 runas, que
  pasa contando de las dos formas. Con «ño» —3 bytes, 2 runas— las dos cuentas se separan.
- **`AL`** (no mirar `sv.Declarado`) quedaba verde porque el servicio declarado se llamaba
  `alturitomarca` y ganaba la ranura **por orden alfabético**. Con `zzz…` adelante, la única forma
  de que entre es que alguien mire quién lo puso.

## Los dos arreglos que salieron de USAR la tool, no de leerla

La primera corrida contra la flota real —`musubi-server`, 24 h— mostró dos defectos que ninguna
prueba iba a encontrar:

**El enlace por término era EVIDENCIA INVENTADA.** `buildFTSQuery` une los tokens con OR, que es
correcto para el recall y falso para un enlace: `avahi-daemon` se buscaba como `"avahi" OR
"daemon"`, así que una nota sobre decisiones de roadmap quedó enlazada a `avahi-daemon`. Un
`ventana` mal puesto agrega ruido; un `termino` mal puesto **afirma que el texto nombra algo que no
nombra**, que es el único error que esta tool no se puede permitir. Ahora se busca como FRASE, sin
respaldo a OR — el respaldo devolvería justo los enlaces falsos que esto elimina.

**El servicio que importaba no llegaba a los términos.** Las doce ranuras se gastaron en
`avahi-daemon`, `NetworkManager-wait-online` y compañía, y `alturito20` —el único servicio del que
hay algo escrito— quedó afuera. El criterio no es adivinar cuál importa: `Declarado` ya significa
«una persona puso esto acá».

## Lo que este slice deja abierto

| # | Qué | Registro |
|---|---|---|
| A61 | Los dos formatos de fecha conviven en la misma base. Unificarlos sería mejor y no se hace acá: tocar cómo se escribe `created_at` afecta a nueve consultas de recall que hoy andan | `specs/control-de-flota/ABIERTO.md` |
