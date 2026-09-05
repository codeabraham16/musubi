# Tasks — S2 · El agente y el latido

Fase 0 del track **Control de flota**, completa. Suite entera verde (19 paquetes).

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | `DeviceStore`: rol nuevo del seam (interfaces chicas, compuestas) | `internal/memory/backend.go` |
| T2 | `musubi_fleet_enroll` / `_list` / `_revoke` | `internal/mcp/methods_fleet.go`, `registry.go` |
| T3 | **La puerta del dispositivo**: `POST /fleet/heartbeat`, auth contra `devices` | `internal/mcp/fleet_http.go`, `http.go` |
| T4 | `musubi agent`: latido, backoff acotado, parada al ser revocado | `cmd/musubi/agent.go`, `main.go` |
| T5 | Las 5 guardas del proyecto, declaradas a conciencia (abajo) | varios `_test.go`, `README.md` |

## Invariantes, y qué los custodia

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **B1** | `TestTokenDeDispositivoNoAbreElMCP` | que `/mcp` caiga a `DevicePorToken` si el registro no resuelve → ✅ falla |
| **B2** | `TestTokenDePersonaNoLate` | resolver contra `opt.registry` en `handlerLatido` → ✅ falla |
| B3 | `TestElRechazoNoDiceCualExistio` | ídem → ✅ falla |
| B4 | `TestElCuerpoDelLatidoNoPuedeSuplantar`, `TestElLatidoNoMandaCuerpoYLlevaElToken` | leer un `device_id` del cuerpo |
| B5 | `TestUn401SeClasificaComoRevocado…`, `TestElBucleSeDetieneAlSerRevocado` | tratar el 401 como fallo transitorio → ✅ fallan las dos |
| B6 | `TestLaPuertaDelDispositivoTieneLockout` | quitar el limiter del handler |
| B7 | `TestElCerebroInalcanzableEsReintentableNoRevocado` | clasificar un error de red como revocado → ✅ falla |
| B7 | `TestElClienteDelLatidoTieneTimeout` | quitarle el `Timeout` a `clienteLatido` → ✅ falla |
| B8 | `TestEnrolarYRevocarSonAdmin` | quitar el gate `isAdmin` |
| B9 | `TestEnrolarNoPuedeElegirElTenantAjeno` | usar `args.Project` en vez de `writeOriginFor` |
| B10 | `TestListarNoCruzaTenants`, **`TestReadSurfaceClassIsolation`** | devolver `declarado` sin mirar las capacidades |
| B11 | `TestOnlineSeCalculaConElUmbralQuePideElLlamador` | fijar el umbral e ignorar `umbral_segundos` |

## 🔴 El e2e encontró lo que el unitario no podía: **un test que no verifica su propio control**

Con toda la suite en verde, el primer e2e contra procesos reales dio **FUGA**: el token del
dispositivo abría `/mcp` con HTTP 200.

No era una fuga. El log del arranque decía `auth=false`: el servidor había levantado **sin
autenticación** (bind loopback, `auth_token_env` sin configurar), así que una petición **sin
ningún header** también daba 200. El token del dispositivo "funcionaba" igual que `basura-total`.

**Es la misma lección de S1, un piso más arriba.** En S1 tres pruebas pasaban con y sin la guarda.
Acá el e2e no verificaba que la cosa que estaba probando estuviera siquiera encendida. Un test de
autenticación que no comprueba que la autenticación corre no prueba nada — y su forma de fallar es
la peor: da un FALSO POSITIVO alarmante que manda a buscar un bug inexistente.

**Regla:** todo test de una propiedad de seguridad arranca con un CONTROL NEGATIVO que prueba que
la defensa está activa. El e2e ahora empieza con `sin Authorization -> 401` y **aborta** si no lo
ve. Con el control puesto, los tres resultados reales:

```
CONTROL  /mcp sin Authorization        -> 401  ✓ la auth está encendida
         token DISPOSITIVO -> /mcp     -> 401  ✓ rechazado
         token PERSONA -> /heartbeat   -> 401  ✓ rechazado
         token DISPOSITIVO -> /heartbeat -> 200 ✓ late
```

## Verificado end to end, contra dos procesos reales

`musubi serve` con auth + `musubi agent` de verdad, no un `httptest`:

```
1. enroll  → device_id 785f7b29… caps [metrics exec]
2. agent --once            → ✓ latido registrado
3. fleet_list              → total 1 · online 1 · umbral 90s · pc-gio silencio 1s
4. agent --interval 1 (bucle vivo)
5. fleet_revoke desde el cerebro
6. el agente se detuvo SOLO, sin que nadie lo matara:
     ! el cerebro rechazó la credencial: este dispositivo fue dado de baja.
     ■ agente detenido (no se reintenta: reintentar sería golpear el lockout del cerebro).
7. fleet_list --include_revoked → revoked:true, online:false, la fila SIGUE con su enrolled_at
```

El punto 6 es B5 funcionando en la única forma que importa: **el kill-switch se entiende desde la
máquina remota**, que es la que no se puede ir a apagar a mano.

## Las 5 guardas del proyecto, y qué se decidió en cada una

Agregar 3 tools rompió 5 tests a propósito. Ninguno se "arregló": en cada uno hubo una decisión.

1. **`TestToolReadOnlyClassification`** — `musubi_fleet_list` es readOnly. Lee `devices` y no
   escribe: `online` se CALCULA al servir, no hay UPDATE.
2. **`TestEveryReadOnlyToolClassified`** — `devices` tiene `project_id`, así que va al **barrido
   de aislamiento**, no a `noScopedRead`. Se sembró un `VICTIMDEVICE` en el tenant `web` y se
   agregó el caso HOSTIL (el atacante declara `project: web`). El barrido pasa: el inventario de
   una flota ajena no se filtra, y el admin federado sí lo ve. *El inventario de máquinas de otro
   proyecto es reconocimiento puro: le dibuja a un tenant el mapa de la infraestructura de otro.*
3. **`TestG8MapaDeAutorizacionIntacto`** — ampliación **deliberada**: un reader ahora puede ver el
   inventario de su proyecto (25 → 26 tools). El consumidor es la CABINA, igual que con
   `readiness` y `design`. Ve metadato operativo, nunca la credencial. Sigue sin poder enrolar ni
   revocar: **ver y tocar son cosas distintas, y ese mapa es donde queda escrito**.
4. **`TestReadmeToolCountMatchesRegistry`** — 62 → 65, y se agregó la fila **Flota** a la tabla por
   dominio: actualizar sólo el número habría dejado el README mintiendo por omisión.
5. **`TestToolsListGolden`** — regenerado. El diff es **puro agregado**: ni una línea `-`.

## Hallazgo lateral (no se tocó)

`README.en.md` dice **«27 tools»** cuando hay 65. La guarda sólo mira `README.md`, así que la
traducción lleva tiempo a la deriva. No se corrigió acá: cambiar el número dejaría un documento
que anuncia 65 y tabula 27, que es peor. **Merece su propio slice.**

## Lo que queda fuera, a propósito

- ~~**El agente no reporta versión ni dirección.**~~ **HECHO en S4b**: el autorreporte escribe
  `agent_version` y `address` sobre la fila PROPIA (`WHERE id = ? AND revoked = 0`), con el tope de
  tamaño que evita ensuciar el inventario y las etiquetas de Prometheus.
- ~~**Sin build tags.**~~ **HECHO en S4**: `colector_linux.go`, `colector_windows.go`,
  `colector_darwin.go` y `colector_otros.go`, cada uno con su `//go:build`. (El texto de abajo era
  correcto en su momento: llegaron con la captura de métricas, no antes.)
- **Cero dependencias nuevas.**

## Siguiente

**Fase 0 cerrada** salvo S3 (scopes de token por device+capacidad). Con el registro y un productor
vivo, **S4 (telemetría del host)** ya tiene dónde colgar: el latido pasa de decir «estoy viva» a
decir «cómo estoy».
