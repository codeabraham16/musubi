# Tasks — S4 · La telemetría del host

Primer slice de la **Fase 1**. Suite entera verde (19 paquetes), vet limpio, cross-compila a
Windows y macOS.

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | `Muestra` + validación + serialización. Lo desconocido es `null`, nunca 0 | `internal/fleet/muestra.go` |
| T2 | Colector Linux: `/proc/stat`, `/proc/meminfo`, `/proc/loadavg`, `statfs`, `/sys/class/thermal` | `internal/fleet/colector_linux.go` |
| T3 | Stub honesto para los OS sin colector: **error, no ceros** | `internal/fleet/colector_otros.go` |
| T4 | Migración **30**: `last_sample`, una COLUMNA y no una tabla de series | `internal/memory/migrations.go` |
| T5 | El latido lleva la muestra, en el MISMO `UPDATE` | `internal/memory/devices.go`, `internal/mcp/fleet_http.go` |
| T6 | El agente recolecta y reporta | `cmd/musubi/agent.go` |
| T7 | `musubi_fleet_metrics`, gateada por la compuerta de S3 | `internal/mcp/methods_fleet.go` |

## Invariantes, y qué los custodia

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **D1** | `TestLaPrimeraMuestraNoInventaUnPorcentajeDeCPU` | arrancar con `tienePrevio: true` → ✅ falla (reportó 13,9 %) |
| D1/D3 | `TestLoDesconocidoViajaComoNullHastaLaRespuesta` | reemplazar los nil por 0 en `filaDeMetricas` |
| D3 | `TestLaTemperaturaEsNilSinSensor` | — |
| **D4** | `TestSinColectorElAgenteLateIgualYNoMandaCeros` | devolver `Muestra{}` en vez de error |
| D5 | `TestLaMuestraSeAtribuyeAlTokenYNoAlCuerpo` | usar un `device_id` del cuerpo |
| D5 | `TestElCuerpoConMuestraNoLlevaIdentidad` | agregar el hostname al JSON del agente |
| **D6** | `TestElServidorNoLeeElCuerpoEnteroAMemoria` | quitar el `LimitReader` → ✅ falla (20 MiB vs 1,2 MiB) |
| D6 | `TestUnCuerpoDemasiadoGrandeSeRechazaSinTumbarElLatido` | — |
| **D7** | `TestUnCuerpoInvalidoNoTumbaElLatido` | 400 ante JSON roto → ✅ falla |
| **D8** | `TestSinLaCapacidadMetricsLaMuestraSeDescarta` | quitar `d.Permite(CapMetrics)` → ✅ falla |
| **D9** | `TestLeerMetricasExigeLaCapacidadPorMaquina` | quitar `PuedeSobreDevice` del filtro |
| D10 | `TestLaUltimaMuestraViveEnLaFilaYNoSeBorraSola` | escribir la columna siempre → ✅ falla |
| — | `TestElColectorMidePorDeVerdadYNoDevuelveCeros` | devolver una `Muestra` vacía |
| — | `TestLaMemoriaUsadaSaleDeMemAvailableYNoDeMemFree` | usar `MemFree` → ✅ falla |
| — | `TestElDiscoUsadoCoincideConDf` | derivar disponible de total−usado |
| — | `TestUnaMuestraIlegibleNoRompeElListado` | propagar el error de parseo |

## 🔴 Dos pruebas que no sabían fallar, y qué enseñaron

**1 · El disco no coincidía con `df`, y mi comentario decía que sí.** La primera versión calculaba
usado como `Blocks − Bavail` afirmando ser «el mismo número que `df`». No lo era: se iba un
**29,8 %** — los 25,6 GB de reserva de root sobre los 502 GB de esta máquina. Lo encontró la
prueba que contrasta contra `df` de verdad, no contra la idea que yo tenía de `df`.

La resolución no fue elegir uno: **`df` muestra tres columnas justamente porque Usado + Disponible
≠ Total.** Ahora se reportan las tres, y cada una responde una pregunta distinta —`DiscoUsado` es
lo que un operador verifica contra `df`; `DiscoDisponible` es lo que dispara la alerta, porque un
disco al 95 % con 5 % reservado ya no acepta una escritura más—. Y hay una aserción que exige que
**no** sumen el total, para que nadie «simplifique» derivando uno del otro.

**2 · El tope del cuerpo: el chequeo y el `LimitReader` no hacen lo mismo.** La prueba mandaba un
cuerpo grande y esperaba el rechazo; pasaba **con y sin** `LimitReader`, porque el chequeo de
tamaño rechaza igual. O sea: el chequeo es lo que **rechaza**, el `LimitReader` es lo que evita
**leer megabytes a memoria**, y la prueba sólo cubría el primero.

La versión nueva cuenta los bytes que el otro lado acepta: **1,2 MiB con la guarda, 20 MiB sin
ella.** Un orden de magnitud de separación.

Es la tercera vez en este track que sale el mismo patrón (S1: tres guardas sin fijar; S2: el e2e
sin control negativo). **Una guarda de defensa en profundidad casi nunca la fija el test del
camino feliz: hay que simular el error del que protege, o medir lo que sólo ella cambia.**

## Una corrección al barrido de aislamiento que evitó un falso verde

Al agregar `musubi_fleet_metrics` al barrido cross-tenant, el control falló: el admin federado no
veía nada porque **el rol admin no otorga capacidades de flota** (C1 de S3).

El arreglo obvio —darle grants al admin— habría dejado el test **pasando por la razón
equivocada**: el atacante seguiría sin `metrics`, así que la compuerta lo frenaría *antes* que la
tenencia, y el barrido quedaría verde **aunque el aislamiento por proyecto estuviera roto**.

Se le dieron los grants **también al atacante**. Ahora lo único que se interpone entre él y la
telemetría de `web` es la tenencia, que es lo que ese barrido existe para custodiar.

## Verificado end to end, midiendo esta máquina de verdad

```
1. enroll kernel-pc caps:[metrics]
2. PRIMER latido      -> muestra guardada · cpu_pct=null  (no hay contra qué restar)
                         mem=40.7%  disco=17.2%  load1=3.85  num_cpu=12  uptime=9904s
3. agente en bucle cada 2 s, con 6 bucles ocupados sobre 12 cores:
     t+ 2s  cpu= 50.0%   t+ 4s  cpu= 59.4%   t+ 6s  cpu= 59.0%
     t+ 8s  cpu= 60.5%   t+10s  cpu= 19.9%   <- se murió la carga
4. observador SIN grants -> devices vistos: 0 · sin_permiso: 1
5. el MISMO observador   -> kernel-pc admite=[metrics] puedo=[] online=True
6. máquina sin `metrics` -> "latido registrado · muestra descartada: esta máquina no tiene
                             concedida la capacidad `metrics`", y sigue figurando online
```

6 de 12 cores ocupados dan ~50 %: el número **sigue la carga real**, no es una constante. Y los
puntos 4-6 son la compuerta de S3 haciendo su trabajo por primera vez sobre algo que importa.

## Lo que queda fuera, a propósito

- ~~**Sin export a Prometheus.**~~ **HECHO en S4b** (`internal/mcp/fleet_prometheus.go`), con la
  credencial del scrape resuelta como decía acá.
- ~~**Sin colector de Windows ni macOS.**~~ **HECHOS en S4c**; lo que sigue abierto no es el
  código sino **A1/A2/A3**: CPU y memoria en macOS (mach), temperatura en Windows (WMI), y que
  NADIE los corrió en hardware real.
- **Sin métricas por proceso ni por interfaz** (**B4**). El agregado del host primero. El
  **conteo** de procesos sí entró después (ver abajo); la LISTA de procesos sigue afuera.
- ~~**`procs` y `mem_free` del colector externo se perdían en el corte.**~~ **HECHO en U1**:
  `mem_libre` (*uint64, MemFree) y `num_procesos` (int, procesos y no hilos) viven en
  `fleet.Muestra`, los miden Linux y —sólo procesos— Windows, viajan como `null` hasta la tool y
  como serie AUSENTE en Prometheus cuando no se midieron. El inventario de los 22 campos del
  colector externo quedó en `internal/fleet/testdata/colector-externo.json`, custodiado por
  `TestNingunCampoDelColectorExternoSePierdeSinMotivo`: un campo que se borre o se renombre pone
  la suite roja con el nombre del culpable.
- **La columna de procesos no está en el panel de flota** (**A38**). El dato viaja hasta la tool y
  hasta Prometheus; `cmd/musubi/assets/flota.html` todavía no lo dibuja.
- **`num_cpu` sigue publicándose crudo y un 0 se lee «esta máquina no tiene CPUs»** (**A39**).
  Es el mismo arreglo que se le hizo a `num_procesos` con `enteroONull`, en otro diff.
- **Cero dependencias nuevas.** `gopsutil` daría los tres OS de una y sería la 7ª dependencia
  directa de un repo que tiene 6. Se prefirió el seam y un colector honesto por OS.

## Siguiente

De la Fase 1 quedan **S5 (exec/PTY)** y **S6 (RustDesk self-hosted)**. La compuerta ya los
espera: `CapExec` y `CapScreen` existen, se conceden y se verifican — lo único que falta es lo que
pasa por ellas. También quedó abierto el slice del **export a Prometheus**, que es lo que
convierte este presente en historia.
