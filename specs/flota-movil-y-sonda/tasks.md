# Tasks — S8 · Tier C (móvil) y la sonda remota

Cierra también **A15** y **A16**, que estaban asignados a S7b: el mismo refactor sirve para
Android y para Tier B, así que se hicieron juntos.

Suite entera verde, vet limpio, cross-compila en las tres plataformas.

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | El parseo de `/proc` a partir de **contenido**, sin build tag y con fixtures reales | `internal/fleet/procparse.go` |
| T2 | El colector local refactorizado para usar **ese mismo** parseo | `internal/fleet/colector_linux.go` |
| T3 | Colector remoto: **una sola invocación** trae todo, por SSH o por ADB | `internal/fleet/remoto.go` |
| T4 | Transporte ADB + traducción de sus fallos | `internal/fleet/remoto.go` |
| T5 | El techo de iOS, dicho **antes de intentar nada** | `internal/fleet/remoto.go`, `methods_sonda.go` |
| T6 | `musubi_fleet_probe` + estado de CPU por dispositivo en el cerebro | `internal/mcp/methods_sonda.go` |

## La idea que hace que esto valga tres slices en uno

**Android es Linux.** Tiene el mismo `/proc` del otro lado de un `adb shell cat`. Y un Tier B
tiene el mismo `/proc` del otro lado de un `ssh cat`.

Separando el **parseo** de la **lectura**, las tres fuentes comparten el mismo código. Sin eso
habría tres parseos de `/proc/meminfo` desincronizándose — y el bug de `MemFree` contra
`MemAvailable`, el que hace que un Linux sano aparezca al 95 % para siempre, habría que arreglarlo
tres veces. **Dos de esas tres no se pueden probar desde acá**, así que el parseo compartido es
también la única forma de cubrirlas.

Una sola invocación remota (`cat A; echo SEP; cat B; ...`) en vez de seis: son seis viajes de red
y seis `fork+exec` por dispositivo y por sondeo.

## `probe` ≠ `metrics`, y es la misma separación de siempre

`musubi_fleet_probe` **va a buscar**; `musubi_fleet_metrics` **lee lo último traído**. Igual que
el ledger es la historia y el feed en vivo el presente; igual que Musubi guarda la última muestra
y Prometheus la serie.

Mezclarlas —que `metrics` salga a la red cuando le falta un dato— haría que una lectura barata se
vuelva impredecible: a veces microsegundos, a veces treinta segundos y un timeout.

## Invariantes

| Test | Sabotaje — **verificado corriéndolo** |
|---|---|
| `TestElParseoDeProcProduceLosMismosNumerosVengaDeDondeVenga` | usar `MemFree` → ✅ falla |
| `TestElParseoRemotoTampocoUsaMemFree` | ídem → ✅ falla |
| `TestUnaLecturaIncompletaNoInventaNumeros` | fijar el total sin el usado → ✅ falla |
| `TestUnaSalidaQueNoEsLinuxSeRechaza` | aceptar cualquier texto → ✅ falla |
| `TestUnRouterQueNoTieneProcNoProduceUnaMuestraDeCeros` | ídem, end to end → ✅ falla |
| `TestUnIPhoneNoSeIntentaSondearYSeDicePorQue` | quitar la guarda `EsIOS` → ✅ falla |
| `TestLosTierANoSeSondean` | sondear también los Tier A → ✅ falla |
| `TestSondearExigeLaCapacidadMetrics` | quitar la compuerta → ✅ falla |
| `TestUnDispositivoInalcanzableNoQuedaVivo`, `TestLaSondaMideUnTierBYGuardaLaMuestra` | — |
| `TestSeReconoceIOSParaDecirLaVerdadTemprano`, `TestLosFallosDeADBSeTraducen` | — |
| `TestElParseoDeDfAguantaElNombreLargo` | posiciones fijas en vez de buscar tres números |

## 🔴 Dos cosas que las pruebas encontraron

**1 · La validación de «esto es un Linux» era de PRESENCIA, no semántica.** Un router de firmware
propietario que contesta `Welcome to RouterOS` con exit 0 pasaba el filtro, y se habría guardado
una **Muestra de ceros como si fuera una medición**: el panel mostrando 0 % de CPU, RAM y disco en
un router que anda perfectamente. Ahora se exige que `/proc/stat` parsee como jiffies o que
`meminfo` traiga un `MemTotal`.

**2 · El matcheo de `device not found` de adb nunca iba a matchear.** El mensaje real es
`error: device '<serial>' not found`, **con el serial en el medio**. Lo agarró la prueba porque usa
el mensaje real de adb y no una versión idealizada.

## El techo de iOS: declarado, no disimulado

iOS **no expone `/proc`, no permite ejecutar código ajeno y no da métricas sin un MDM** con perfil
de supervisión — que es un producto entero, con su ceremonia de inscripción y su certificado.

Musubi puede tener un iPhone en el **inventario** (existe, es de alguien, está en un proyecto) y
**no puede medirlo ni controlarlo**. La sonda lo dice **antes de intentar nada**: un error de adb
mandaría a alguien a depurar el cable cuando el problema es que la cosa no se puede.

El mensaje aclara de quién es el techo: *«no es una limitación de Musubi, es de la plataforma»*.

## Verificado end to end

```
tres dispositivos SIN agente: Tier B linux · Android · iPhone

SONDA:
  ✓ nas-linux    [ssh]      cpu=None mem=49.6% disco=17.8% uptime=1492s
  ✗ tele-android [adb]      "no está instalado `adb` en el cerebro..."
  ✗ iphone-gio   [ninguno]  "iOS no expone /proc ni permite ejecutar nada sin un MDM..."

SEGUNDO sondeo, con carga real:
  ✓ cpu=15.0%   ← la derivada apareció: ya había contra qué restar

fleet_metrics (lectura barata, sin red):
  nas-linux  online=True  cpu=14.98  mem=49.9%  temp=27.8  antiguedad=0s
  aun_sin_reportar: 2
```

## Lo que queda fuera

- **Pantalla en Android** (scrcpy sobre ADB) (**A18 → S8b**; su sombra ya la tapó **S6c**) — el tier lo admite en la matriz de S1 y el motor
  sería otro distinto del de RustDesk. → `ABIERTO.md`
- **Exec en Tier C** (**B16**) — la matriz de S1 no lo concede, y sigue sin concederlo: en Android depende
  de que ADB esté habilitado, y prometerlo al enrolar sería mentir.
- **SNMP / MQTT / Redfish** (**A17 → S7c**) — siguen abiertos.
- **Cero dependencias nuevas.**
