# Tasks — S4b · Export a Prometheus y cierre de cabos

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | El exportador de flota, gateado por la compuerta de S3 | `internal/mcp/fleet_prometheus.go` |
| T2 | `/metrics` CAPTURA el principal en vez de descartarlo | `internal/mcp/http.go` |
| T3 | `ProyectosConDevices` para el barrido federado | `internal/memory/devices.go` |
| T4 | **Cabo**: autorreporte del agente (versión + dirección del tailnet) | `cmd/musubi/agent.go`, `internal/mcp/fleet_http.go`, `internal/memory/devices.go` |
| T5 | **Cabo**: `README.en.md` regenerado (8 → 16 dominios, 27 → 66) y guarda bilingüe | `README.en.md`, `internal/mcp/readme_toolcount_test.go` |
| T6 | 9 reglas de alerta de flota + la credencial documentada | `deploy/musubi-alerts.yml`, `deploy/prometheus/prometheus.yml` |

## Invariantes

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **E1** | `TestUnValorDesconocidoNoSeExportaComoCero` | devolver `(0, true)` con CPUPct nil → ✅ falla |
| E2 | `TestUnaMaquinaSinMuestraAportaSoloSuEstado` | emitir series de una muestra inexistente |
| **E3** | `TestElScrapeExportaSoloLoQueEsaCredencialPuedeVer` | quitar `PuedeSobreDevice` → ✅ falla |
| **E4** | `TestUnAdminSinGrantsNoExportaNadaYLoDice` | ídem → ✅ falla |
| E5 | `TestElScrapeNoCruzaTenants` | ignorar la tenencia en `proyectosVisibles` |
| **E6** | `TestUnNombreConComillasNoCorrompeElScrape` | interpolar sin `citarLabel` → ✅ falla |
| **E7** | `TestLosBytesGrandesNoSalenEnNotacionCientifica` | formatear con `%v` → ✅ falla |
| **E9** | `TestElAutorreporteSoloTocaLaFilaDelToken` | usar un `device_id` del cuerpo → ✅ falla |
| E9 | `TestElAutorreporteVacioNoBorraLoAnterior`, `...NoDependeDeLaCapacidadMetrics`, `...SeRecorta` | escribir siempre; gatear por `metrics` |
| E9 | `TestLaDireccionReportadaNoEsLoopback` | devolver la primera IP sin filtrar |
| **E10** | `TestReadmeToolCountMatchesRegistry` (bilingüe) | bajar el conteo del EN → ✅ falla |

## 🔴 Un bug de ORDEN que las pruebas agarraron

El autorreporte quedó **después** del `return` de «no vino muestra». Consecuencia: un agente en un
OS sin colector manda `{"version":"..."}` sin muestra, salía por ese return y **nunca se
identificaba** — justo la máquina de la que menos se sabe era la que se quedaba anónima.

Las tres pruebas del autorreporte fallaron antes de aplicarle ningún sabotaje, que es la forma en
que una prueba avisa que el código no hace lo que su comentario dice. El orden ahora está
documentado como invariante en el propio handler.

## Verificado end to end, con scrape real

```
SCRAPE con credencial `prometheus` (read=all + metrics:["*"]):
  musubi_fleet_device_up{device="kernel-pc",project="casa",tier="A",os="linux"} 1
  musubi_fleet_device_memory_used_bytes{...} 3287011328
  musubi_fleet_device_disk_available_bytes{...} 390231924736
  musubi_fleet_device_temperature_celsius{...} 27.8
  musubi_fleet_device_uptime_seconds{...} 16402
  ...16 series, cpu_percent AUSENTE  <- cada `--once` es un proceso nuevo: no hay lectura previa

Con el agente en BUCLE (mismo proceso ⇒ sí hay lectura previa):
  musubi_fleet_device_cpu_percent{device="kernel-pc",...} 10.05

Credencial de Prometheus SIN grants  -> 0 series + el comentario que explica qué falta
Token LEGACY (admin sin grants)      -> 0 series, mismo trato
Métricas del SERVIDOR                -> 41 líneas, intactas para todos

Inventario: address='100.114.63.7'  <- el tailnet real de esta máquina
```

## Cabos que quedan ABIERTOS, y por qué

| Cabo | Estado |
|---|---|
| **Colector de Windows / macOS** | Abierto. El seam y los build tags están; cross-compila. **Hoy sólo Linux reporta métricas.** Es lo que más acota el alcance prometido del track. |
| **Alertmanager** | Abierto y ya estaba anotado en `prometheus.yml`: las reglas se evalúan y se ven en la UI, pero no notifican. |
| **Selectores por tag en los grants** | Abierto por diseño: no hay un caso real que lo pida. |
| **Tools para administrar los grants por red** | Abierto por diseño: otorgarse capacidades a uno mismo por la red merece más cuidado. |
| **Métricas por proceso / por interfaz** | Abierto por diseño: el agregado del host primero. |
| **S5 exec/PTY · S6 RustDesk · S7 Tier B · S8 móvil · S9 panel · S10 alertas** | Slices futuros del track. |
