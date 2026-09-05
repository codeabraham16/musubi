# Spec — S1 · El registro de dispositivos

Primer slice del track **Control de flota** (`specs/control-de-flota/proposal.md`). Fase 0.

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
// internal/fleet — dominio puro (sin base, sin red), igual que internal/skills.
type Tier string   // "A" agente nativo · "B" por protocolo · "C" móvil
type Cap  string   // "metrics" | "exec" | "screen"

type Device struct { ID, Name, ProjectID string; Tier Tier; Caps []Cap; ... }

func NormalizarTier(string) (Tier, error)
func NormalizarCaps([]string) ([]Cap, error)
func CapsDelTier(Tier) []Cap
func (d Device) Permite(Cap) bool
func (d Device) EnLinea(ahora time.Time, umbral time.Duration) bool
func ValidarAlta(Device) error

// internal/memory — persistencia (migración 29 + tabla `devices`).
func (e *DbEngine) AltaDevice(d fleet.Device, token string) (fleet.Device, error)
func (e *DbEngine) DevicePorToken(token string) (fleet.Device, bool, error)
func (e *DbEngine) ListarDevices(projectID string, incluirRevocados bool) ([]fleet.Device, error)
func (e *DbEngine) LatirDevice(id string, ahora time.Time) error
func (e *DbEngine) RevocarDevice(projectID, name string) (bool, error)
```

**Dirección de dependencias:** `memory → fleet`. `fleet` no importa `memory` (dominio puro,
como `internal/skills`). Es la misma dirección que ya existe con `memory → codeintel`.

---

## Lo que se midió antes de escribir nada

Sobre este repo, el 2026-08-26:

- **`user_version` = 28.** La migración de este slice es la **29**; `shadow_verdicts` (v28) NO
  está en `database.go`, o sea que las tablas nuevas van **sólo** en `migrations.go`. Se sigue.
- **`/metrics` mide a Musubi, no a la máquina.** `internal/mcp/observability.go` cuenta
  invocaciones de tools y latencia del dispatch. No hay una sola métrica de CPU/RAM/disco del
  host. Es el hueco que justifica S4, y la razón de que un device sea una entidad nueva y no
  una etiqueta sobre las métricas que ya existen.
- **La identidad ya se deriva del token, nunca del cliente.** `internal/mcp/principals.go` lo
  hace así para las personas y `musubi_whoami` lo dice explícito. Este slice **reusa** ese
  invariante para las máquinas en vez de inventar uno nuevo (A1).
- **Una fila sin `project_id` se ve desde TODOS los tenants.** Está medido y documentado en
  `principals.go`: *«2 filas de test contaminando los 3 proyectos»*. Por eso A5 es fail-closed.

---

## H1 · La identidad de una máquina no la declara la máquina

### A1 — El `device_id` lo asigna el cerebro y se deriva del token

El agente **no elige su id**. El alta la hace el cerebro, genera el id y devuelve el token
crudo **una sola vez**; de ahí en más el device se identifica presentando el token, y el id sale
de buscar su hash. Un `id` que llegue en el cuerpo de una petición se ignora.

Sin esto, un device puede afirmar ser otro y toda la telemetría, el exec y la auditoría quedan
atribuidos a la máquina equivocada. Es exactamente el invariante que `principals.go` ya sostiene
para las personas — acá se extiende a las máquinas.

### A2 — El registro guarda el SHA-256, nunca el token crudo

Mismo trato que `principals.yaml`. Un volcado de la tabla `devices` no entrega credenciales
usables. La prueba busca el token crudo en **todas** las columnas de la fila y exige no encontrarlo.

### A3 — Un token identifica a UN device

Índice único parcial sobre `token_sha256`. Dos devices con la misma credencial harían que la
auditoría no pueda distinguirlos, que es lo mismo que no auditar.

---

## H2 · El tier decide lo que se puede prometer

### A4 — Una capacidad que el tier no sabe honrar es un error de alta, no una sorpresa de runtime

La matriz es explícita y fail-closed:

| Tier | metrics | exec | screen |
|---|---|---|---|
| **A** agente nativo (Linux/Win/macOS) | sí | sí | sí |
| **B** por protocolo (SSH/SNMP/MQTT/Redfish) | sí | sí | **no** — un router no tiene framebuffer |
| **C** móvil (Android/iOS) | sí | **no** | sí (parcial) |

Dar de alta un device Tier B con `screen` **falla en el alta**. La alternativa —aceptarlo y fallar
cuando alguien pida la pantalla— convierte una promesa imposible en un bug intermitente a las 3 AM.

### A5 — Las capacidades de flota NO se derivan del rol de memoria

Un principal con `write=any` sobre la **memoria** no obtiene `exec` sobre **las máquinas**. Son
ejes distintos y el default es **ninguna capacidad**. Un `Device` cero no permite nada.

Es la decisión de seguridad central del track: sin esta separación, administrar la memoria del
equipo se convierte silenciosamente en root sobre toda la flota, y eso es el puente de privilegio
que separa un RMM de un RAT.

---

## H3 · Multi-tenancy, con el mismo eje que la memoria

### A6 — `project_id` es obligatorio (fail-closed)

Un device sin proyecto sería visible desde todos los tenants — el bug ya medido en el cerebro
real con las observaciones. El alta sin `project_id` falla.

### A7 — El listado aísla por proyecto

Los devices del proyecto A no aparecen listando el proyecto B.

---

## H4 · El estado que se guarda es el que se puede sostener

### A8 — «En línea» se DERIVA de `last_seen`; no se guarda

No hay columna `online`. Es la misma clase de mentira que un PID que quedó escrito: una máquina
que muere sin despedirse deja el booleano en `true` para siempre. `EnLinea(ahora, umbral)` lo
calcula, y el umbral lo elige quien pregunta.

Precedente propio: la poda de `riel-local` (A7) existe porque un proceso que muere de golpe no
avisa. Un device apagado a la fuerza es el mismo caso.

### A9 — Revocar corta el acceso sin borrar la historia

`revoked = 1` deja de autenticar de inmediato, y la fila **queda**. Borrarla perdería a quién
pertenecía la telemetría y las sesiones que ya ocurrieron — que es justo lo que una auditoría
necesita después de un incidente.

### A10 — El latido no puede fallar la operación

`LatirDevice` sobre un device inexistente o revocado no es un error del llamador: no actualiza y
lo informa. Un heartbeat que tira excepciones convierte un device viejo en una cascada de ruido.

---

## H5 · Lo que este slice NO hace

- **No expone tools MCP.** `musubi_fleet_*` llega con S2/S3, cuando exista el agente y los scopes.
- **No recolecta ni una métrica de host.** Eso es S4; acá sólo existe *a qué máquina* atribuirla.
- **No ejecuta nada en ningún lado.** Exec es S5, pantalla es S6.
- **No agrega dependencias.** Dominio y persistencia son stdlib + lo que ya está en `go.mod`.
  (`gopsutil`, si entra, es una discusión de S4 y hay que darla ahí: este repo tiene 6
  dependencias directas y `observability.go` se escribió a propósito con «cero dependencias nuevas».)
