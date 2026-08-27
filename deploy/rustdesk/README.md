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

## Instalación del relay

```bash
sudo ./install-rustdesk-relay.sh
```

Deja `hbbs` y `hbbr` como servicios systemd, y te imprime la **clave pública** del servidor —el
dato que cada cliente necesita para registrarse contra TU relay y no contra el público.

## Configurar un cliente para que use TU relay

En cada máquina, tras instalar RustDesk:

```
Configuración → Red → ID/Relay Server
  ID Server:  <ip-del-tailnet-o-dominio>:21116
  Key:        <la clave pública que imprimió el instalador>
```

O por línea de comandos, que es lo que conviene si lo estás automatizando:

```bash
rustdesk --config <cadena-de-configuración>
```

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
`rustdesk --password`. Para que eso funcione y sea lo que se espera:

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
