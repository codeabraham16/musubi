# Tasks — S6 · La pantalla, sobre RustDesk self-hosted

**Cierra la Fase 1.** Suite entera verde (4 pasadas completas), vet limpio, cross-compila.

## Hecho

| # | Qué | Dónde |
|---|---|---|
| T1 | Dominio de la sesión — **sin campo para la contraseña**, y eso es el diseño | `internal/fleet/pantalla.go` |
| T2 | Migración **32**: `screen_sessions` + `rustdesk_id` | `internal/memory/migrations.go` |
| T3 | Store: abrir, marcar, listar. Ninguna firma recibe una contraseña | `internal/memory/sesiones.go` |
| T4 | `musubi_fleet_screen` y `musubi_fleet_sessions` | `internal/mcp/methods_pantalla.go` |
| T5 | El agente aplica la contraseña y **programa su propio vencimiento** | `cmd/musubi/pantalla.go` |
| T6 | Los dos agujeros laterales, cerrados (abajo) | `internal/mcp/methods_exec.go` |
| T7 | Relay `hbbs`/`hbbr` como systemd + el manual honesto | `deploy/rustdesk/` |

## Invariantes

| Inv | Test | Sabotaje — **verificado corriéndolo** |
|---|---|---|
| **G1** | `TestLaSesionNoTieneDondeGuardarLaContrasena` | agregarle un campo `Password` → ✅ falla |
| **G1** | `TestLaContrasenaNoQuedaEnNingunaTabla` | guardarla en la sesión |
| **G1** | `TestElResultadoQueVaALaBitacoraNoTraeLaContrasena` | no pasar el error por `sinSecreto` → ✅ falla |
| **G2** | `TestAlVencerSeReemplazaLaContrasenaNoSeBorra` | borrarla en vez de reemplazarla → ✅ falla |
| **G4** | `TestSinCapacidadScreenNoHaySesion` | gatear con `CapExec` → ✅ falla |
| G5 | `TestUnTierBNuncaTienePantalla` | ignorar la matriz de tiers |
| G7 | `TestLaSesionQuedaAuditadaConQuienPidioMirar` | registrar recién al confirmar |
| **G8** | `TestLaBitacoraDeSesionesExigeLaCapacidad` | no filtrar por `screen` → ✅ falla |
| — | `TestLaBitacoraDeComandosNoFiltraLaContrasenaDePantalla` | quitar `ocultarArgvDePantalla` → ✅ falla |
| — | `TestConExecNoSePuedeFabricarUnaSesionDePantalla` | quitar la guarda `musubi:` → ✅ falla |
| — | `TestLaOperacionInternaSeInterceptaYNoSeLanzaComoBinario` | no interceptar → ✅ falla |
| — | `TestLaSesionPasaAActiva...`, `...QuedaFallidaSiLaMaquinaNoPudo` | no marcar la sesión → ✅ fallan |
| — | `TestLaContrasenaEsFuerteYDictable`, `TestUnaSesionSolicitadaQueVencio...` | — |

## 🔴 Dos agujeros LATERALES que no estaban en la spec

La spec cuidaba la tabla de sesiones. El riesgo real estaba al lado:

**1 · La bitácora de COMANDOS filtraba la contraseña.** La contraseña tiene que llegar a la
máquina de alguna forma, y viaja en el argv de un comando — que la tabla de comandos guarda tal
cual. **G1 se caía sin que nadie tocara la tabla de sesiones**: Musubi no la guardaba, pero la
mostraba. Se oculta en toda superficie que muestre la bitácora, conservando el id de sesión para
poder cruzar las dos.

**2 · Con `exec` se podía fabricar una sesión de pantalla.** El canal de comandos lo comparten los
dos planos y el agente distingue por el primer argumento. Alguien con `exec` podía encolar un
`musubi:pantalla` a mano y acuñarse una sesión **sin tener `screen`** — o sea saltarse media
compuerta usando la otra mitad. Que sean permisos distintos deja de ser cierto si uno puede
fabricar los mensajes del otro. `musubi_fleet_exec` ahora rechaza todo `musubi:*`.

## 🔴 Dos cosas que encontró el e2e y una la suite

**La sesión quedaba en `solicitada` para siempre.** El estado `activa` existía en el dominio y
**nada transicionaba a él**. Una bitácora que no distingue «se aplicó» de «la máquina no pudo» es
inútil justo cuando alguien dice «no me deja entrar». Ahora el resultado del comando cierra el
estado, leyendo el id de sesión del **argv guardado** y no del cuerpo que mandó el agente.

**Una prueba FLAKY, encontrada corriendo la suite completa 4 veces.**
`TestLaMemoriaUsadaSaleDeMemAvailableYNoDeMemFree` comparaba `m.MemUsada` contra un número
derivado de leer `/proc/meminfo` **por segunda vez**, exigiendo igualdad al byte. Entre las dos
lecturas la memoria se mueve —bajo carga, ~1,4 MB— y el test fallaba con el código bien.

Comparar dos lecturas independientes de una cantidad que cambia, al byte, es flaky por
construcción. Y no hacía falta: el bug que busca vive a escala de **gigabytes** (3,87 GB contra
7,38 GB). Ahora compara por cercanía, con un margen del 1 % de la RAM. **La aserción tiene que
discriminar a la escala en la que el bug vive, no más fina.**

## Verificado end to end, con un doble de RustDesk

```
1. sesión de 1 minuto  → rustdesk_id 123456789 · password HgpQRv7LDD4db6Hy (una vez)
2. llegó al cliente    → `rustdesk --password HgpQ…` ✓

3. ¿DÓNDE ESTÁ LA CONTRASEÑA?
     bitácora de sesiones ✓ no    bitácora de comandos ✓ no
     inventario           ✓ no    EL ARCHIVO .db ENTERO ✓ no está

4. `soporte` (exec, SIN screen)      → RECHAZADO
   fabricando la operación con exec  → RECHAZADO
   lo que ve en la bitácora          → ['musubi:pantalla','<id>','[oculto]']

5. VENCIMIENTO, aplicado por el AGENTE:
     17:11:58  --password <al azar, arranque>
     17:12:01  --password <LA-DE-LA-SESION>
     17:13:01  --password ngCwff3HfkkGsg69   ← 60s exactos, sin que el cerebro intervenga
```

El punto 3 es G1 verificado de la forma más fuerte posible: **la contraseña no está ni en el
archivo de la base**. El 5 es G2: el vencimiento no depende de que el cerebro siga vivo.

## Lo que Musubi NO hace, y está escrito en el manual

- **El consentimiento lo aplica RustDesk.** Musubi guarda la política y el despliegue la
  configura, pero **no la impone**. `deploy/rustdesk/README.md` §1 lo dice antes que nada:
  si tu política exige consentimiento y el cliente no está configurado así, no lo hay.
- **Musubi no ve ni graba la sesión.** El video va P2P o por el relay; Musubi no está en ese
  camino. Sabe que se autorizó, no qué pasó adentro.
- **El relay se ata al TAILNET por default**, no a `0.0.0.0`. Abrirlo al mundo hay que pedirlo.

## Lo que queda fuera

- ~~**Verificar el `rustdesk_id` contra el relay.**~~ **DESCARTADO en S6b: no era viable ni habría
  servido.** hbbs no expone API para eso, y aunque la expusiera sólo diría qué CONEXIÓN reclama el
  id ahora, no cuál de nuestras máquinas es. Se atacó por el otro lado —la COLISIÓN— que cubre a la
  vez el caso malicioso y el benigno frecuente (imágenes clonadas). Ver B12 en `ABIERTO.md`.
- **Grabación de sesión** (**A14**) — decisión legal antes que técnica.
- **Instalar RustDesk en las máquinas** — es material de despliegue, no código.
- **Cero dependencias nuevas.**
