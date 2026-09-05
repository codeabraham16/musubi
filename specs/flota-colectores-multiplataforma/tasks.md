# Tasks — S4c · Los colectores de Windows y macOS

Cierra el cabo que más acotaba el alcance prometido del track: hasta acá **sólo Linux reportaba
métricas**. Suite entera verde, vet limpio en las tres plataformas, cross-compila en 6
combinaciones OS/arch.

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | `Load1/5/15` pasan a **puntero** — Windows no tiene load average | `internal/fleet/muestra.go` + todos los consumidores |
| T2 | **Regla de los pares** (total y usado se fijan juntos o ninguno), validada | `internal/fleet/muestra.go` |
| T3 | La aritmética del % de CPU, extraída y **probada en cualquier plataforma** | `internal/fleet/cpudelta.go` |
| T4 | Colector de **Windows** vía `kernel32.dll` (`syscall.NewLazyDLL`) | `internal/fleet/colector_windows.go` |
| T5 | Parseo binario de sysctl, **sin build tag y probado** | `internal/fleet/sysctlparse.go` |
| T6 | Colector de **macOS** vía `syscall.Sysctl` + `statfs` | `internal/fleet/colector_darwin.go` |
| T7 | Pruebas del seam que corren en **las tres** | `internal/fleet/colector_test.go` |

## La estrategia que hace esto verificable sin un Mac ni un Windows

No tengo esas máquinas. Lo honesto no era escribir el código y decir «anda»: fue **separar lo
que se puede probar de lo que no**, y dejar del lado no probado la menor cantidad de código
posible.

- **`cpudelta.go`** — la derivada del % de CPU. Linux la saca de `/proc/stat` y Windows de
  `GetSystemTimes`: fuentes distintas, **misma aritmética**. Vive sin build tag y tiene 3 pruebas.
  Del lado de Windows queda «leer dos números y pasarlos».
- **`sysctlparse.go`** — el punto fijo de `vm.loadavg` y el `timeval` de `kern.boottime`. Es LA
  trampa del colector de macOS (leer los `uint32` crudos daría cargas de varios miles). Sin build
  tag, 4 pruebas. Del lado de darwin queda «pedir el buffer y pasarlo».
- **`colector_test.go`** — corre en las tres y exige lo mismo de todas.

**Lo que queda SIN verificar es la capa de syscalls**, y está anotado como A3 en
`specs/control-de-flota/ABIERTO.md`. No se declara «funciona en Windows»: se declara «compila,
su aritmética está probada, y falta correrlo en hardware real».

## Qué mide cada plataforma, y es una PRUEBA, no un README

`TestLoQueCadaPlataformaMideEstaDeclarado` fija la tabla. Si algún día llega el colector de mach y
macOS empieza a reportar CPU, **el test falla** y obliga a actualizarla.

| | CPU % | Carga | Memoria | Disco | Temp. | Uptime |
|---|---|---|---|---|---|---|
| **Linux** | sí | sí | sí | sí | si hay sensor | sí |
| **Windows** | sí | **no** (no existe en Windows) | sí | sí | no (A2) | sí |
| **macOS** | **no** (A1) | sí | **no** (A1) | sí | no | sí |
| otros | — | — | — | — | — | — (error explícito) |

Lo que no se mide viaja como `null`: la tool devuelve `null`, Prometheus **no emite la serie** y
el panel muestra un hueco. Nunca un cero.

## Invariantes

| Test | Sabotaje — **verificado corriéndolo** |
|---|---|
| `TestElDeltaDeCPUDevuelveNilCuandoNoSabe` (7 casos) | devolver 0 en vez de nil |
| `TestTrasUnRetrocesoElContadorSeRearma` | no actualizar el estado en el camino nil |
| `TestUnCeroMedidoNoEsLoMismoQueNoSaber` | colapsar «medí 0 %» con «no sé» |
| `TestParseLoadavgDivideElPuntoFijo` | leer los uint32 sin dividir por `fscale` |
| `TestParseLoadavgRechazaBasura`, `TestParseBoottimeRechazaLoImposible` | aceptar cualquier buffer |
| `TestParseBoottimeDevuelveElInstanteDeArranque` | confundir instante con duración (uptimes de 55 años) |
| `TestTodaPlataformaTieneColectorYNingunoMienteConCeros` | devolver `Muestra{}, nil` en vez de error |
| `TestLaReglaDeLosParesSeRespetaEnEstaPlataforma` | fijar `MemTotal` sin `MemUsada` |
| `TestLoQueCadaPlataformaMideEstaDeclarado` | cambiar la tabla → ✅ falla |

## 🔴 Dos cosas que las pruebas encontraron

**1 · El colector de Linux dibujaba una caída a cero falsa.** Al extraer la aritmética se vio que
el código viejo, ante contadores que RETROCEDEN (un reinicio entre latidos, una VM migrada, un
contador que da la vuelta), **acotaba el porcentaje a 0**. O sea: pintaba un 0 % justo después de
un reinicio — exactamente el artefacto que el resto del diseño evita. Ahora devuelve `nil` y
rearma la base.

**2 · Dos lecturas seguidas no producen porcentaje, y está bien.** La tabla de capacidades falló
al principio diciendo que Linux no medía CPU. No era el colector: dos lecturas dentro del mismo
tick del kernel devuelven los MISMOS contadores, el delta es cero y `delta` responde `nil`
(correctamente — dividir daría NaN). **El test culpaba al colector de no medir cuando no había
pasado nada que medir.** Ahora quema CPU en el medio, y eso quedó escrito en el propio test.

## Verificado

```
go build ./...            ✓
go vet ./...              ✓   (+ GOOS=windows, GOOS=darwin)
go test ./... -count=1    ✓   19 paquetes
cross-compile:  linux/amd64 · windows/amd64 · windows/arm64
                darwin/amd64 · darwin/arm64 · freebsd/amd64   ✓ (6/6)
```

## Lo que queda

Ver `specs/control-de-flota/ABIERTO.md`: **A1** (CPU y memoria en macOS por mach), **A2**
(temperatura en Windows por WMI) y **A3** (correrlos en hardware real) — los tres asignados a
**S4c**, que es este mismo slice continuado cuando haya con qué probarlo.
