# Tasks — S1 · El registro de dispositivos

Fase 0 del track **Control de flota**. 32 pruebas nuevas, 1.262 líneas, suite completa verde.

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | Dominio puro: `Tier`, `Cap`, la matriz tier→capacidades, `Device`, validación de alta | `internal/fleet/device.go` |
| T2 | Credencial del dispositivo: `NuevoToken` (crypto/rand) + `HashToken` (SHA-256) | `internal/fleet/device.go` |
| T3 | Migración **29**: tabla `devices` + índice por proyecto + único parcial por credencial + único por (proyecto, nombre) | `internal/memory/migrations.go` |
| T4 | Store: alta, resolución por token, por nombre, listado, latido, revocación | `internal/memory/devices.go` |
| T5 | Guarda de versión de esquema actualizada 28 → 29, con su línea de changelog | `internal/memory/outbox_test.go` |

**Dirección de dependencias:** `memory → fleet`. `fleet` no importa `memory` (dominio puro, como
`internal/skills`). Misma dirección que la ya existente `memory → codeintel`.

## Invariantes, y qué los custodia

| Inv | Test | Sabotaje que lo hace fallar — **verificado corriéndolo** |
|---|---|---|
| A1 | `TestAltaIgnoraElIDQueDeclaraElCliente` | dejar que el cliente elija su `id` → ✅ falla |
| A1 | `TestIdentidadSeDerivaDelToken` | — |
| A1 | `TestTokenVacioNoAutenticaNiConUnaFilaQueHasheoElVacio` | quitar la guarda de token vacío → ✅ falla |
| A2 | `TestElTokenCrudoNoSeGuardaEnNingunaColumna` | guardar `token` en vez de `HashToken(token)` → ✅ falla |
| A3 | `TestDosDevicesNoPuedenCompartirCredencial` | quitar el índice único parcial |
| A3 | `TestVariosTierBSinCredencialConviven` | hacer el índice único TOTAL en vez de parcial |
| A4 | `TestTierNoAdmiteLoQueSuHardwareNoTiene` | dar `screen` a Tier B en la matriz → ✅ falla (3 tests) |
| A4 | `TestAltaRechazaCapacidadQueElTierNoPuedeCumplir`, `TestFilaConCapImposibleNoLaHonraIgual` | ídem → ✅ fallan |
| A5 | `TestDeviceCeroNoPermiteNada` | que `Permite` devuelva true con `Caps` vacío |
| A5 | `TestDeviceRevocadoNoPermiteNadaAunqueConserveSusCaps` | quitar la guarda `d.Revoked` de `Permite` → ✅ falla |
| A6 | `TestAltaSinProyectoFalla`, `TestAltaSinProyectoNoLlegaALaBase` | quitar el chequeo de `ProjectID` → ✅ falla |
| A7 | `TestListarAislaPorProyecto` | quitar el `WHERE project_id = ?` |
| A7 | `TestListarSinProyectoNoDevuelveLasFilasHuerfanas` | quitar la guarda de proyecto vacío → ✅ falla |
| A8 | `TestNoExisteColumnaOnline` | agregar una columna `online` a la migración |
| A8 | `TestLatidoSePersisteYElEstadoSeDeriva` | — |
| A8 | `TestEnLineaConRelojCeroNoInventaVida` | quitar `LastSeen.IsZero()` de `EnLinea` → ✅ falla |
| A9 | `TestRevocarCortaElAccesoYConservaLaHistoria` | cambiar el UPDATE por un DELETE → ✅ falla |
| A9 | `TestListadoEscondeRevocadosSalvoQueSePidan` | — |
| A10 | `TestLatidoDeUnDeviceQueYaNoEstaNoEsError` | devolver error cuando `RowsAffected` es 0 |

## Tres guardas que NO estaban fijadas por su prueba, y cómo se arreglaron

Es el hallazgo más útil del slice, y sale de **correr** los sabotajes en vez de suponerlos. Tres
pruebas pasaban con la guarda y **también sin ella**: eran decoración. En los tres casos el
comentario del código además afirmaba algo falso sobre para qué servía la guarda.

**1 · `EnLinea` y `LastSeen.IsZero()`.** El comentario decía que la guarda es lo que hace que un
dispositivo sin latidos figure caído. Falso: `time.Duration` **satura en ~292 años** (max int64 de
nanosegundos), así que `ahora.Sub(cero)` da `2562047h47m` y supera cualquier umbral realista — el
caso ya sale fail-closed por aritmética. La guarda sirve para el otro caso: un llamador con un
**reloj cero**, donde `cero.Sub(cero)` es 0 y entra en cualquier umbral. Prueba nueva:
`TestEnLineaConRelojCeroNoInventaVida`.

**2 · `DevicePorToken` y el token vacío.** El comentario decía que sin la guarda una petición sin
credencial autenticaría como el primer Tier B. Falso: los Tier B se guardan con `token_sha256`
vacío y `HashToken("")` es `e3b0c442…` — nunca coinciden. La guarda es la **segunda línea** de una
defensa cuya primera está en `AltaDevice`: si alguien lo "simplifica" para hashear siempre, todo
device sin credencial comparte el hash del vacío y ahí sí hay bypass. La prueba nueva **inserta esa
fila a mano** para simular exactamente ese error.

**3 · `ListarDevices` y el projectID vacío.** El comentario decía que la guarda evita devolver
«todos». Falso: el `WHERE project_id = ?` ya lo evita, porque ninguna fila legítima tiene proyecto
vacío. Lo que la guarda tapa son las filas **huérfanas** si alguna vez existieran (backfill,
reparación a mano, un NOT NULL aflojado). La prueba nueva inserta una huérfana a mano.

Las tres guardas se quedaron —son defensa en profundidad legítima— pero ahora el comentario dice
**lo que hacen de verdad** y la prueba **falla si se las saca**.

## La guarda de esquema hizo su trabajo

`TestMigrationV11OutboxSchema` fija `latestSchemaVersion()` a un literal, con este comentario:
*«agregar una migración obliga a tocar este número. Que sea molesto es el punto»*. La 29 lo rompió
al primer intento. Se actualizó a 29 con su línea de changelog, como las 6 anteriores.

## Verificado

```
go build ./...        ✓
go vet ./...          ✓ sin hallazgos
go test -race ./internal/fleet/   ✓
go test ./... -count=1            ✓ 19 paquetes verdes, 0 fallos
```

## Lo que queda fuera, a propósito

- ~~**Ninguna tool MCP.**~~ **HECHO en S2/S3.** `musubi_fleet_*` llega con S2/S3 — sin agente y sin scopes no hay a quién
  exponérselas.
- ~~**Ninguna métrica de host.**~~ **HECHO en S4.** Es S4. Este slice sólo crea *a qué máquina* atribuirla.
- ~~**Sin CLI.**~~ **HECHO en S5** (`musubi ctl`) y **S5b/S5c** (`musubi shell`). `musubi ctl` es del plano de terminal (S5).
- **Cero dependencias nuevas.** Dominio y store son stdlib + `uuid`, que ya estaba.

## Siguiente

**S2 · `musubi agent`**: enrolamiento contra el cerebro (consume `AltaDevice`, recibe el token una
vez) y heartbeat (consume `LatirDevice`). Cross-OS por build tags, como ya hace
`procvivo_{windows,unix}.go`. Es el que le da un productor real a esta tabla.
