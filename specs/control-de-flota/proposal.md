# Proposal — La flota entra al cerebro (RMM sobre Musubi)

> Track, no slice. Un RMM estilo AnyDesk/RustDesk/TacticalRMM, pero PROPIO y
> colgado del cerebro que ya existe. Este documento es el mapa; cada slice de
> abajo será su propia spec con su contrato observable.

## El hueco

Musubi ve su **memoria** y su **propio uso**. No ve las **máquinas**. El tailnet
ya conecta los equipos y el cerebro ya los autentica por token, pero un equipo
hoy "existe" en Musubi sólo como *origen de sync de memoria* — no como algo que
puedas **monitorear**, **ejecutar** ni **ver la pantalla**. Falta:

1. Un **registro de dispositivos** como entidad de primera clase.
2. **Telemetría del host** (CPU/RAM/disco/red/temp/procesos), no del proceso Musubi.
3. Un canal para **ejecutar** en ellos (comando + terminal interactiva).
4. **Pantalla** remota.

## Lo que ya existe (verificado en código, NO se rebuildea)

El backbone de un RMM ya está construido para otra cosa y sirve tal cual:

| Necesidad RMM | Ya en Musubi |
|---|---|
| Malla privada entre equipos | Tailscale + allowlist en `deploy/connect-brain-{linux,windows}.*` (cerebro en :7717) |
| API remota / control | `internal/mcp/http.go` — dispatch MCP sobre HTTP, bind remoto **exige bearer token**, TLS opcional |
| Tokens / acceso especial | `methods_admin_tokens.go` + `musubi_token_new/list/revoke` + rate-limit (`authlimit.go`) + admin gate |
| Métricas + alertas | `/healthz` `/readyz` `/metrics` Prometheus (`internal/mcp/observability.go`) + `deploy/prometheus/` + `deploy/musubi-alerts.yml` |
| Feed en vivo | SSE en `internal/mcp/livefeed.go` (+ riel local, spec `riel-local`) |
| Dashboard visual | WebGL "cerebro" (`cmd/musubi/cerebro.go`, `dashboard.go`) |
| Registro multi-máquina + sync | `syncclient.go`, `skillfed.go`, modelo cerebro central |
| Auditoría inmutable | usage ledger + livefeed = quién hizo qué, cuándo |

**Conclusión de la medición: ~70% del transporte, identidad, métricas y panel ya
está. El track agrega 3 planos de capacidad y un registro, no un sistema nuevo.**

## "Todo lo controlable" son 3 tiers, y el tier decide qué podés hacer

Decisión del dueño: Linux + Windows + macOS + Android + *todo lo controlable por
terminal/MCP/API*. Eso NO es homogéneo. Se declara honesto para no prometer
"acceso completo" donde el hardware no lo da:

- **Tier A — Agente nativo** (Linux/Windows/macOS): binario `musubi agent` corre
  en el host. Telemetría + exec/PTY + bridge a pantalla. Control máximo.
- **Tier B — Por protocolo** (routers, NAS, IoT, servers sin agente): el cerebro
  (o un agente vecino) los maneja por su protocolo nativo — SSH, SNMP, MQTT,
  Redfish/IPMI. Sin binario en el device. Alcance = lo que el protocolo permite.
- **Tier C — Móvil** (Android/iOS): Android por ADB/companion; iOS muy limitado
  (perfil/MDM). Se documenta el techo real: un móvil no da PTY root ni pantalla
  libre. No se vende humo.

## Los 3 planos de capacidad

1. **Telemetría** — todos los tiers, con distinto detalle. Host → cerebro →
   Prometheus + dashboard de flota. Reusa el pipeline Prometheus que ya existe.
2. **Control / terminal** — Tier A: PTY completo; Tier B: por protocolo; Tier C:
   limitado. Comando suelto + shell interactiva, **todo al ledger**.
3. **Pantalla** — Tier A (+ Tier C parcial). Decisión del dueño: **integrar
   RustDesk self-hosted** (relay `hbbs`/`hbbr` en el VPS). Musubi NO reimplementa
   captura/encoding/WebRTC: orquesta inventario, inicia la sesión, aplica política
   y audita. Reinventar el motor de pantalla son meses y un campo minado; se descarta.

## Seguridad como primera clase (esto separa "RMM" de "RAT")

Es diseño, no anexo. Un plano de exec + pantalla + "acceso completo" sobre red ES
la forma de un RAT si el modelo de autorización es flojo. Sobre la propia flota,
en el propio tailnet, con tokens, es legítimo — y el diseño lo sostiene:

- **Tokens con alcance por dispositivo Y por capacidad**: `metrics` ≠ `exec` ≠
  `screen`. Extiende el sistema de tokens existente con scopes; no lo reemplaza.
- **Auditoría inmutable**: cada exec y cada sesión de pantalla va al usage ledger
  + livefeed. "Quién, en qué device, qué, cuándo" es no negociable.
- **Consentimiento por device**: attended (el device aprueba en pantalla) vs
  unattended (pre-autorizado por política). Configurable por máquina.
- **Acceso híbrido** (decisión del dueño): Tailscale por default; relay público
  (RustDesk) SÓLO para devices marcados, con su propio gate.
- **Kill-switch en caliente**: `musubi_token_revoke` corta TODO acceso a un device
  al instante. Ya existe; se cablea a los 3 planos.

## Slices (cada uno será su propia spec)

**Fase 0 — Fundación (desbloquea los 3 planos a la vez):**
- **S1 · Registro de dispositivos** — device como entidad 1ª clase (id, tier, OS,
  tags, visto por última vez). Nuevo `internal/fleet`. Sin esto, ningún plano
  tiene "a qué máquina".
- **S2 · `musubi agent`** — modo nuevo que se enrola con el cerebro (token) y late
  (heartbeat). Cross-OS por build tags, como ya hace `procvivo_{windows,unix}.go`.
- **S3 · Scopes de token** — capabilities (`metrics|exec|screen`) por device sobre
  los tokens existentes.

**Fase 1 — Los 3 planos en paralelo (recién acá tiene sentido "los tres a la vez"):**
- **S4 · Telemetría host** (Tier A) — colector (gopsutil) → push → `/metrics` de
  flota + panel.
- **S5 · Exec/PTY auditado** (Tier A) — comando + shell interactiva, cada byte al ledger.
- **S6 · RustDesk self-hosted** — `hbbs`/`hbbr` en el VPS; Musubi orquesta, inicia
  y audita la sesión.

**Fase 2 — Ampliar alcance:**
- **S7 · Tier B por protocolo** (SSH/SNMP/MQTT/Redfish) — controlar lo sin-agente.
- **S8 · Tier C móvil** (Android ADB / companion) — con el techo documentado.
- **S9 · Dashboard de flota** — extender el cerebro WebGL con la vista de MÁQUINAS,
  no sólo neuronas de memoria.

**Fase 3 — Operación:**
- **S10 · Alertas + políticas de flota** — extender `musubi-alerts.yml`, auto-heal.

## Los 4 caminos de acceso (los que pediste)

- **Visual** → dashboard de flota (extiende cerebro) + RustDesk para pantalla.
- **Terminal** → `musubi ctl <device> exec …` / shell interactiva desde el CLI,
  sobre el tailnet.
- **API / MCP** → tools nuevas (`musubi_fleet_list`, `_exec`, `_screen_start`,
  `_metrics`): cualquier agente controla por MCP con token con scope.
- **Desde donde sea** → tailnet (default) o relay público (híbrido), con token/código.

## Lo que explícitamente NO se hace

- **No** se reimplementa el motor de pantalla (captura/encoding/NAT). Se integra RustDesk.
- **No** se promete "acceso completo" en Tier B/C donde el protocolo/OS no lo da.
- **No** se abre un relay público por default: sólo por device marcado (híbrido).

## Estado

Borrador para revisión del dueño. Al aprobar: se detalla **S1** (registro de
dispositivos) a `spec.md` + `tasks.md` con contrato observable y pruebas que saben
fallar, y arranca la Fase 0.
