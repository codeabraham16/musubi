# Spec — S4 · La telemetría del host

Primer slice de la **Fase 1**. Depende de la Fase 0 entera: S1 (a qué máquina), S2 (el latido y
las dos puertas), S3 (la compuerta). El latido pasa de decir «estoy viva» a decir «cómo estoy».

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
// internal/fleet — el dominio de la medición.
type Muestra struct {
    Tomada     time.Time
    CPUPct     *float64  // nil = DESCONOCIDO (ver D1), nunca un 0 inventado
    NumCPU     int
    MemTotal, MemUsada, SwapTotal, SwapUsada uint64
    MemLibre   *uint64   // MemFree. nil = este OS no lo expone (ver D11)
    DiscoTotal, DiscoUsado                   uint64
    Load1, Load5, Load15                     float64
    UptimeSeg                                uint64
    TempC      *float64  // nil = sin sensor
    NumProcesos int      // PROCESOS, no hilos. 0 = no medido (ver D12)
}

type Colector interface{ Tomar() (Muestra, error) }   // seam por OS
func NuevoColector() Colector                          // build tags

// internal/mcp — el latido ahora puede traer una muestra.
POST /fleet/heartbeat   { "muestra": {...} }   // cuerpo ACOTADO
musubi_fleet_metrics                            // gateado por la compuerta de S3
```

---

## Lo que se midió antes de diseñar nada

En esta máquina, el 2026-08-26:

```
/proc/stat     cpu 931771 2364 439752 8613590 173159 0 7588 0 0 0   <- JIFFIES ACUMULADOS
/proc/meminfo  MemTotal 7843996 kB · MemAvailable 4745104 kB · SwapTotal 9941140 kB
/proc/loadavg  1.64 2.33 2.31 5/1181 94909
/proc/uptime   8623.50 86136.11
/proc/net/dev  lo, wlp2s0 (contadores acumulados)
/sys/class/thermal/thermal_zone0/temp  27800  (miligrados)
12 CPUs
```

**El hallazgo que decide el diseño:** `/proc/stat` no da un porcentaje, da **contadores
acumulados desde el arranque**. Un porcentaje de CPU es la DERIVADA entre dos lecturas — no
existe «el uso de CPU ahora mismo» sin recordar la lectura anterior. Eso no es un detalle de
implementación: es lo que hace que D1 sea un invariante y no una comodidad.

---

## H1 · Medir de verdad, o decir que no se sabe

### D1 — El % de CPU de la PRIMERA muestra es `null`, nunca 0

El colector guarda la lectura anterior y calcula el delta. La primera vez no hay anterior, así
que **no hay porcentaje**, y el campo viaja como `null`.

Inventar un `0.0` sería peor que no medir: un panel que muestra 0 % en una máquina que arrancó
recién es indistinguible de una máquina ociosa, y esa confusión aparece justo cuando alguien está
mirando por qué algo se cayó. Mismo criterio que `last_seen: null` en S1.

### D2 — El porcentaje es el promedio del INTERVALO, y se dice cuál fue

Al calcular sobre el delta entre latidos, el número es el promedio real de esos ~30 s. No se
duerme dentro del colector para muestrear dos veces: bloquear el agente para producir un número
más «instantáneo» es pagar latencia por una precisión que nadie pidió.

### D3 — Un sensor ausente es `null`, no un cero

Sin `thermal_zone0` no hay temperatura. `TempC` viaja `null` y el consumidor decide qué hacer.
Una flota con máquinas heterogéneas tiene sensores en unas y en otras no.

### D11 — `mem_libre` es MemFree y NO es la contracara de `mem_usada`

`MemUsada` sale de **MemAvailable** y `MemLibre` de **MemFree**: entre las dos vive el page cache,
y con el fixture real la diferencia son **3,5 GB** (85 % contra 40 % de RAM usada). Tener las dos
en la struct vuelve tentador derivar una de la otra, y por eso hay dos pruebas custodiando la
resta —`TestElParseoRemotoTampocoUsaMemFree` y `TestLaMemoriaUsadaSaleDeMemAvailableYNoDeMemFree`.

Viaja como puntero porque **Windows y macOS no la pueden dar sin mentir**:
`MEMORYSTATUSEX.ullAvailPhys` es el análogo de MemAvailable, no de MemFree, y en macOS haría falta
mach. `nil` = «este sistema no la expone», que es distinto de «no le queda nada libre».

**Lo que NO se valida, a propósito:** `mem_libre <= mem_total - mem_usada`. Parece obvia y es
falsa —MemAvailable descuenta los watermarks del kernel y puede ser MENOR que MemFree—, y como una
muestra inválida se descarta entera (D7), la aserción de más le costaría toda la telemetría a un
servidor recién arrancado. La única regla es `mem_libre <= mem_total`.

### D12 — `num_procesos` cuenta PROCESOS, y por eso no sale de `/proc/loadavg`

El 4º campo de `/proc/loadavg` («5/1181») está leído, a mano y es **hilos**: da entre 3 y 5 veces
más. El conteo sale de filtrar el listado de `/proc` por «el nombre es todo dígitos» —los tgid—,
en `ContarPids`, que comparten las tres fuentes (local, Tier B por SSH, Tier C por ADB).

Es un `int` y no un puntero, como `NumCPU`: una máquina encendida siempre tiene al menos un
proceso, así que el **0 no es ambiguo** y significa «no medido». La traducción a `null` ocurre en
la frontera (`enteroONull`), y en Prometheus la serie directamente no se emite.

### D4 — En un OS sin colector, el agente lo DICE

Windows y macOS no tienen colector en este slice. El stub **devuelve un error explícito**, no
una muestra de ceros. Un dashboard que pinta 0 % de CPU en todos los Windows es peor que uno que
dice «esta máquina no reporta métricas todavía»: el primero se cree, el segundo se arregla.

---

## H2 · La muestra viaja sin poder mentir sobre quién la manda

### D5 — El cuerpo trae MEDICIONES, nunca IDENTIDAD

B4 de S2 sigue intacto: la identidad sale del token. El cuerpo del latido tiene un solo campo
(`muestra`) y no hay dónde poner un `device_id`. Una máquina no puede reportar las métricas de
otra.

### D6 — El cuerpo está ACOTADO

Un agente corre en la superficie más expuesta de la flota. Un cuerpo sin tope es un DoS con
forma de telemetría. Tope chico y explícito; lo que lo exceda se rechaza sin leerlo entero.

### D7 — Un cuerpo inválido no tumba el latido

JSON roto, campos de más, números absurdos: el latido **sigue registrando que la máquina está
viva** y descarta la muestra. Estar viva y saber medirse son cosas distintas, y un agente con un
colector roto no debe desaparecer del inventario.

---

## H3 · La compuerta de S3 finalmente gatea algo

### D8 — Sin la capacidad `metrics`, la máquina no reporta

Una máquina a la que no se le concedió `metrics` puede latir (sigue viva) pero su muestra se
**descarta**. La capacidad no es decorativa: es lo que decide si el dato se guarda.

### D9 — Leer las métricas exige la capacidad, POR MÁQUINA

`musubi_fleet_metrics` devuelve sólo las máquinas donde `PuedeSobreDevice(p, d, metrics)` es
cierto. Es el primer consumidor real de la compuerta de S3, con sus tres lados: tenencia ∧
concesión ∧ aparato.

---

## H4 · Musubi guarda el PRESENTE; la historia es de otro

### D10 — Se guarda la ÚLTIMA muestra, no una serie

Una columna en la fila del dispositivo, escrita en el mismo `UPDATE` que ya hace el latido: cero
escrituras extra. **Musubi no es una base de series temporales** y no va a serlo — es la misma
separación que el proyecto ya eligió entre el ledger (la HISTORIA) y el feed en vivo (el
PRESENTE). Cuando haga falta historia, la guarda Prometheus, que ya está desplegado en
`deploy/prometheus/`.

---

## H5 · Lo que este slice NO hace

- **No exporta a Prometheus.** Es el paso natural siguiente y merece su propio slice: el scrape
  tiene UNA credencial y ve todo, así que cruzarlo con la compuerta por-máquina de S3 es una
  decisión de diseño, no un `for` sobre las filas.
- **No hay colector de Windows ni de macOS.** El seam y los build tags quedan puestos; los
  colectores son sus propios slices (D4 hace que su ausencia se vea en vez de mentir).
  **Esto acota el alcance declarado del track: hoy sólo Linux reporta.**
- **No mide por proceso ni por interfaz de red.** El agregado del host primero; el detalle,
  cuando haya una pregunta que lo pida. El **conteo** de procesos sí entró (D12); la lista de
  procesos no, y no va a entrar: es cardinalidad sin techo.
- **No mide `mem_libre` ni procesos en macOS, ni `mem_libre` en Windows.** Está declarado en el
  encabezado de cada colector y custodiado por la tabla de capacidades de `colector_test.go`, que
  falla si alguna plataforma empieza —o deja— de medir algo.
- **No alerta.** Las alertas son S10.
- **Cero dependencias nuevas.** `/proc` + `syscall.Statfs`, stdlib. `gopsutil` daría los tres
  OS de una — y sería la 7ª dependencia directa de un repo que tiene 6 y un `observability.go`
  escrito a propósito con «cero dependencias nuevas». Se prefiere el seam y un colector honesto
  por OS.
