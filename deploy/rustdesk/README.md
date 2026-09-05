# Relay de pantalla — RustDesk self-hosted

El plano visual del track «Control de flota» (S6). **Musubi no transporta video**: lo hace
RustDesk, y este directorio es lo que hace falta para que su relay corra en tu VPS.

## Qué corre dónde

```
  VPS (tu servidor)              cada máquina de la flota
  ┌────────────────────┐         ┌─────────────────────────┐
  │ hbbs  :21115-21116 │◄────────┤ cliente RustDesk        │
  │  (registro de IDs) │         │  + musubi agent         │
  │ hbbr  :21117       │◄───────►│                         │
  │  (relay si el NAT  │  video  │                         │
  │   no deja P2P)     │         └─────────────────────────┘
  └────────────────────┘
           ▲
           │ Musubi NO está en este camino.
           │ Lo que Musubi decide es QUIÉN puede pedir una sesión, y lo deja escrito.
```

**El video nunca pasa por Musubi.** Va directo entre los dos clientes (P2P) o por `hbbr` si el
NAT no deja. Musubi sabe que la sesión se autorizó; no sabe qué pasó adentro.

## Instalación del relay — dos caminos

### a) Contenedores, sin root (`preparar.sh`)

```bash
./preparar.sh            # o ./preparar.sh --bind <ip>
```

Es el camino que se usó en `musubi-server`, y por un motivo concreto: el instalador systemd
necesita root, y `sudo` ahí pide contraseña. **Un despliegue que exige root es un despliegue que
se queda sin hacer.** Podman rootless ya corre la cadena de alertas en ese servidor y los puertos
del relay son todos ≥1024, así que no hace falta ni un privilegio.

### b) systemd nativo (`install-rustdesk-relay.sh`)

```bash
sudo ./install-rustdesk-relay.sh
```

Para quien administra el server como server. Deja `hbbs` y `hbbr` como unidades systemd.

**Los dos atan al tailnet por defecto, los dos exigen la clave (`-k _`), y los dos imprimen la
clave pública al terminar** — que es el único dato que hace falta del otro lado. Que no se vayan a
la deriva lo custodia `internal/mcp/despliegue_relay_test.go`: si alguien cambia el default de uno
a `0.0.0.0`, o le saca el `-k _` a un servicio, la suite se pone roja.

La diferencia real entre los dos: el de contenedores **publica** los puertos atados a una IP, así
que hbbs ve la dirección reescrita por la red de podman en vez de la del cliente. En un tailnet
—donde todos los pares se alcanzan directo— eso no cambia nada. Para el **acceso híbrido**
(máquinas fuera de la malla, donde hbbs necesita la dirección real para adivinar el tipo de NAT)
hay que usar el instalador systemd.

## Configurar un cliente para que use TU relay

En cada máquina, tras instalar RustDesk:

```
Configuración → Red → ID/Relay Server
  ID Server:  <ip-del-tailnet-o-dominio>:21116
  Key:        <la clave pública que imprimió el instalador>
```

O escribiendo el archivo de configuración, que es lo que conviene si lo estás automatizando.
En Windows vive en `%APPDATA%\RustDesk\config\RustDesk2.toml`:

```toml
rendezvous_server = '<ip>:21116'

[options]
custom-rendezvous-server = '<ip>'
relay-server = '<ip>'
key = '<la clave pública que imprimió el instalador>'
```

### Este paso NO se hace por el canal de comandos, y es a propósito

Musubi podría escribir ese archivo con `musubi_fleet_exec` y reiniciar el cliente. **No lo hace.**
Cambiarle el servidor a un RustDesk que alguien está usando en ese momento le corta la sesión —y
si algo sale mal, le corta el acceso remoto a la máquina desde la que iba a arreglarlo. Es la
única pieza de todo el track cuyo despliegue tiene que hacer una persona que esté mirando.

**Lo que hay que tener claro antes de hacerlo:** el plano de pantalla de Musubi **ya funciona
contra el servidor público de RustDesk**. La compuerta, la contraseña acuñada, el vencimiento y la
bitácora son de Musubi y no dependen de quién sea el relay. Lo que se gana al mover el cliente a
tu relay es dejar de depender de infraestructura ajena para el video — que es una buena razón,
pero no es «que la pantalla funcione».

## Lo que hay que configurar en el cliente, y NO es opcional

### 1 · Consentimiento (*attended* vs *unattended*)

**Esto lo aplica RustDesk, no Musubi.** Musubi guarda la política como metadato del inventario y
la compuerta decide quién puede *pedir* una sesión — pero quien pregunta *«¿aceptás que te
miren?»* a la persona sentada frente a la máquina es el cliente de RustDesk.

- **Máquinas de personas** (portátiles, escritorios): dejá activado
  `Configuración → Seguridad → Confirmar antes de conectar`. Alguien tiene que poder decir que no.
- **Servidores sin nadie adelante**: desactivalo, o nadie va a poder conectarse nunca.

Si tu política exige consentimiento y RustDesk no está configurado así, **no lo hay** — que
Musubi lo tenga anotado no lo impone.

### 2 · Dejar que Musubi maneje la contraseña

`musubi_fleet_screen` **acuña una contraseña por sesión** y el agente la aplica con
`rustdesk --password`.

> **Dónde busca el agente ese binario.** En el PATH y en los lugares donde lo deja cada instalador
> oficial (`cmd/musubi/rustdesk_ruta.go`). En Windows importa: el instalador lo pone en
> `C:\Program Files\RustDesk\rustdesk.exe` y **no toca el PATH**. Durante todo un track el agente
> lo buscó sólo en el PATH, no lo encontró, y reportó "" — que arriba se lee como «esta máquina no
> tiene pantalla configurada». Dos máquinas con RustDesk instalado y corriendo figuraban sin
> `rustdesk_id`, y la ausencia se veía igual que si el programa no estuviera. Si tu instalación
> está en un lugar raro, `MUSUBI_RUSTDESK_BIN` con la ruta completa; si apunta a algo que no
> existe, el agente **falla y lo dice** en vez de seguir buscando en silencio.

Para que eso funcione y sea lo que se espera:

- **NO** pongas una contraseña permanente a mano. Si la ponés, existe un acceso que no pasa por
  la compuerta ni queda en la bitácora — o sea, exactamente lo que este diseño evita.
- Dejá la contraseña de un solo uso de RustDesk desactivada por la misma razón.

La contraseña que Musubi acuña **vence sola en la máquina**: el agente programa el reemplazo. Si
el cerebro se cae, la sesión se cierra igual.

### 3 · La red

Si tus máquinas ya están en el tailnet (como pide `deploy/connect-brain-*`), apuntá el ID Server a
la IP del tailnet y **no abras ningún puerto al mundo**. El relay público sólo hace falta para las
máquinas que no pueden entrar a la malla — el «acceso híbrido» del plan.

## Cómo se ve una sesión, de punta a punta

```
1. musubi_fleet_screen device=pc-gio
     → la compuerta chequea `screen` sobre ESA máquina
     → se registra la sesión (quién, cuándo, hasta cuándo)
     → se acuña una contraseña, va a la máquina por el canal de comandos
     → te la devuelve UNA vez, con el rustdesk_id
2. Abrís RustDesk, ponés el ID y esa contraseña.
3. A los 30 minutos el agente la reemplaza por una al azar. Se acabó.
4. musubi_fleet_sessions muestra que pasó. Nunca la contraseña: no existe guardada.
```
