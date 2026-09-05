# Tasks — S11 · El empujador OTLP

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | **El refactor que hace que haya UN export y no dos**: `serieDeFlota` + `seriesDeFlota` (las 19 series), `devicesVisiblesParaMetricas` y `labelsDeFlota` salen de `renderFlota` a funciones compartidas. La salida de `/metrics` no cambia ni un byte | `internal/mcp/fleet_prometheus.go` |
| T2 | El sobre OTLP/JSON a mano (structs + `encoding/json`), el armado del payload y los atributos | `internal/mcp/fleet_otlp.go` |
| T3 | El cliente: validación del destino al arranque, POST con timeout propio, clasificación del fallo | `internal/mcp/fleet_otlp.go` |
| T4 | El lazo: ticker propio, un empuje en vuelo, principal resuelto en cada tick | `internal/mcp/fleet_otlp.go`, `internal/mcp/http.go` |
| T5 | La config, apagada por default, y su ejemplo comentado | `internal/config/config.go` (`OTLPPushConfig`), `.musubi/config.example.yaml` |
| T6 | Las validaciones de ARRANQUE, en dos tiempos como las políticas | `internal/mcp/scheduler_flota.go` |
| T7 | Las tres series de auto-vigilancia del empuje, por el TIRÓN | `internal/mcp/observability.go`, `internal/mcp/http.go` |
| T8 | `--web.enable-otlp-receiver` en los dos caminos de instalación + la duplicación declarada | `deploy/prometheus/install-musubi-prometheus.sh`, `deploy/docker/compose.yml`, `deploy/prometheus/prometheus.yml` |

## Invariantes

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **E1** | `TestArmarPayloadConPrincipalNilNoExporta` · `TestElEmpujeNoCruzaTenants` | sacar el `if p == nil` de `armarPayloadOTLP` → ✅ falla, y el payload que imprime lleva las máquinas de LOS DOS tenants |
| **E2** | `TestElEmpujeNoArrancaSinPrincipalNombrado` (3 casos) | borrar `validarPrincipalDeEmpuje` de `vincularRegistroDeFlota` → ✅ falla |
| **E3** | `TestElProyectoDeLaSerieSaleDeLaFilaYNoDeLoQueDeclaraLaMaquina` | en `labelsDeFlota`, tomar `project` de un campo AUTORREPORTADO (`d.Address`) → ✅ falla |
| **E4** | `TestElEmpujeYElScrapeExportanLasMismasSeriesYLosMismosValores` | que `armarPayloadOTLP` se saltee una serie (`musubi_fleet_device_up`), o sea: tabla propia del push → ✅ falla |
| **E5** | `TestUnValorDesconocidoNoViajaComoCeroEnElPayload` · `TestUnUpEnCeroViajaConSuCero` | `(0, true)` en vez de `(0, false)` en `processes`; y `AsDouble float64` con `omitempty` en vez de `*float64` → ✅ fallan |
| **E6** | `TestElPayloadNoTomaLabelsDelAutorreporte` | agregar `agent_version` a `atributosOTLP` → ✅ falla |
| **E7** | `TestElEmpujeNoLlevaLasMetricasDelServidor` | agregarle al payload una métrica del servidor bien formada (`musubi_tool_calls_total`) → ✅ falla |
| **E8** | `TestElSobreOTLPTieneLaFormaDeLaEspecificacion` (+ `puntosDelPayload`, que exige la forma en TODAS las pruebas) | `timeUnixNano` como número → ✅ falla |
| **E9** | `TestNingunaUnidadRenombraLaSerieEnPrometheus` | ponerle unidad `"1"` a `musubi_fleet_device_up` → ✅ falla |
| **E10** | `TestElPayloadUsaUnSoloReloj` | sellar cada punto con un `time.Now()` propio en vez de con `ahora` → ✅ falla |
| **E11** | `TestUnPrometheusColgadoNoFrenaNingunaTool` | envolver `empujarUnaVez` en `s.dispatchMu.Lock()` → ✅ falla (la tool tardó 5 s, el timeout del POST) |
| **E12** | `TestDosTicksDeEmpujeNoSeSolapan` | reemplazar el `CompareAndSwap` por un `Store(true)` que nunca saltea → ✅ falla (llegan 2 requests) |
| **E13** | `TestRevocarAlPrincipalDelEmpujeLoApagaEnElActo` | cachear el `*Principal` en el struct la primera vez y reusarlo en cada tick → ✅ falla (siguió mandando datos) |
| **E14** | `TestElErrorDelEmpujeNoLlevaLaCredencialNiElCuerpoDelDestino` · `TestUnDestinoRemotoSinTLSNoArranca` · `TestUnaURLConUserinfoSeRechaza` | volcar `resp.Body` en el error (el eco del proxy trae el bearer de vuelta); quitar la guarda de loopback; quitar la de userinfo → ✅ fallan las tres |
| **E15** | `TestLaFallaDelEmpujeSeVeDesdeMetrics` · `TestLasSeriesDelEmpujeSalenPorElMetricsDeVerdad` | emitir `last_success` en 0 cuando nunca hubo uno; y sacar `s.renderEmpuje` del handler de `/metrics` → ✅ fallan |
| **E16** | `TestUnBarridoTruncadoSeAnuncia` | truncar sin avisar (cambiarle la clave al `avisarUnaVez`) → ✅ falla |
| **E17** | `TestElReceptorOTLPSeHabilitaEnLosDosLugaresYLaDuplicacionEstaDeclarada` | comentar el flag en el compose y dejarlo sólo en el unit systemd → ✅ falla |
| — | `TestUn404DiceQueFaltaElFlagDeProm` | un error genérico de «HTTP 404» sin nombrar el flag → ✅ falla |
| — | `TestSinEndpointElEmpujeNiSiquieraSeConfigura` | que `Activo()` deje de exigir endpoint (el empuje nace encendido) → ✅ falla |
| — | `TestElEmpujeNoCruzaTenants` + `TestElScrapeExportaSoloLoQueEsaCredencialPuedeVer` | sacar `PuedeSobreDevice` de `devicesVisiblesParaMetricas` → ✅ fallan LAS DOS: el barrido es compartido, así que la puerta trasera se abre en los dos caminos a la vez y las dos guardas lo cazan |

## 🔴 La trampa que casi entra sola: la unidad `"1"`

La especificación de OTLP dice que lo adimensional lleva unidad `"1"`, y el contrato del slice la
pedía. **En Prometheus eso renombra la serie**: el receptor normaliza el nombre con la unidad y a un
gauge con `"1"` le agrega `_ratio`. `musubi_fleet_device_up` habría llegado como
`musubi_fleet_device_up_ratio`, con las 12 reglas de `musubi-alerts-flota.yml` evaluándose contra un
nombre que ya no existe: **sin fallar, sin avisar y sin disparar nunca** — el mismo modo de falla
que el renombre de `device` a `hostname`, por otra puerta.

Lo adimensional viaja con unidad vacía, y la regla («o vacía, o el nombre ya lleva el sufijo») quedó
como prueba en vez de como costumbre.

## Lo que queda fuera

- **La verificación contra un Prometheus 3.1.0 de verdad** (**A40**) — el empuje está probado contra
  un `httptest` que valida la forma, no contra el receptor real. La forma del sobre, el path y la
  normalización de unidades están tomados de la especificación de OTLP/JSON y de la documentación de
  Prometheus: leídos, no medidos.
- **Métricas del servidor por OTLP** — DESCARTADO en este slice, por diseño: están detrás de auth y
  `musubi_fleet_policy_actions_total` no lleva etiqueta de máquina a propósito (auditoría
  2026-07-26 #9). Empujarlas a un store sin credencial deshace esa corrección por la otra puerta.
- **`--web.enable-remote-write-receiver` y el Pushgateway** — DESCARTADOS por diseño. Remote-write es
  una puerta de ESCRITURA anónima sobre los datos de los que viven las alertas; el Pushgateway
  además rompe el envejecimiento (una máquina muerta seguiría publicando su último valor para
  siempre).
- **Reenviar muestras viejas o rellenar huecos** — DESCARTADO por diseño: lo que se empuja es lo que
  `renderFlota` emitiría en ese instante. Nada que sobreviva a la muerte de su fuente.
- **Histogramas y sumas** — DESCARTADO por diseño: todas las series de flota son gauges, igual que
  en el pull.
- **Un `metric_relabel_configs` ya escrito en `prometheus.yml`** — es **despliegue**: queda como
  receta comentada y no aplicada, porque aplicarla sin el push encendido rompería el scrape de flota
  que hoy funciona. La decisión es del operador, en el momento en que enciende el push.
- **Backoff con memoria entre ticks** (**A41**) — no hay: el reintento es el próximo tick, y el
  aviso de un fallo permanente sale una sola vez.
- **SDK de OpenTelemetry** — DESCARTADO: **Cero dependencias nuevas**. El OTLP/JSON se emite a mano,
  con el precedente de `internal/memory/otel.go`.
