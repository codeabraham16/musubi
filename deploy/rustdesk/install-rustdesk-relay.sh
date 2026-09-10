#!/usr/bin/env bash
#
# install-rustdesk-relay.sh — deja hbbs/hbbr corriendo como systemd en este servidor.
#
# Es el mismo estilo que install-musubi-brain.sh: systemd nativo, sin docker, para que el relay
# se administre con las mismas herramientas que el resto del server.
#
# Uso:  sudo ./install-rustdesk-relay.sh [--bind <ip>]
#
# --bind acota a qué interfaz escuchan. POR DEFAULT se ata a la IP del TAILNET si la encuentra,
# y NO a 0.0.0.0: un relay de pantalla abierto al mundo es exactamente lo que no se quiere por
# accidente. Para el acceso híbrido —máquinas que no pueden entrar a la malla— hay que pedirlo
# explícito con --bind 0.0.0.0, y ahí conviene leer el README antes.
set -euo pipefail

VERSION="${RUSTDESK_SERVER_VERSION:-1.1.14}"
DESTINO="/opt/rustdesk-server"
USUARIO="rustdesk"
BIND=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bind) BIND="${2:-}"; shift 2 ;;
    *) echo "argumento desconocido: $1" >&2; exit 1 ;;
  esac
done

log(){ printf '\033[36m▶ %s\033[0m\n' "$*"; }
ok(){  printf '\033[32m✓ %s\033[0m\n' "$*"; }
aviso(){ printf '\033[33m! %s\033[0m\n' "$*"; }
die(){ printf '\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "hay que correrlo como root (systemd + /opt)"

# ── Bind: tailnet por default ────────────────────────────────────────────────
if [[ -z "$BIND" ]]; then
  BIND="$(ip -4 -o addr show 2>/dev/null | awk '{print $4}' | cut -d/ -f1 \
          | awk -F. '$1==100 && $2>=64 && $2<=127' | head -1 || true)"
  if [[ -n "$BIND" ]]; then
    ok "atando al tailnet: $BIND"
  else
    BIND="127.0.0.1"
    aviso "no se encontró IP de tailnet: se ata a 127.0.0.1."
    aviso "Para el acceso híbrido pasá --bind <ip> a conciencia (leé deploy/rustdesk/README.md)."
  fi
fi
# ── Binarios ─────────────────────────────────────────────────────────────────
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  PAQUETE="rustdesk-server-linux-amd64.zip" ;;
  aarch64) PAQUETE="rustdesk-server-linux-arm64v8.zip" ;;
  *) die "arquitectura no soportada por los binarios oficiales: $ARCH" ;;
esac

# EL PAQUETE DEL RELAY TAMPOCO SE INSTALA SIN VERIFICAR (hermano del paso 1 de
# deploy/install-musubi-brain.sh).
#
# Hasta acá no se verificaba NADA: se bajaba el zip de un release ajeno y se instalaban `hbbs` y
# `hbbr` —que quedan corriendo como unidades systemd de este servidor— con lo que viniera. Es el
# mismo fail-open que tenía el binario del cerebro, un piso más abajo en privilegio pero con la
# agravante de que el origen no es nuestro.
#
# EL PIN VIVE ACÁ, y no hay alternativa: rustdesk NO publica checksums en sus releases (medido el
# 2026-09-10: los 18 assets del tag 1.1.14 son .deb y .zip, ni un .sha256 ni un SHA256SUMS). Es el
# mismo criterio que BACKUP_SHA256 en install-musubi-brain.sh — el número vive en el archivo que el
# operador ya confía porque lo corre con sudo desde su clone— y acá además es la única opción.
# Estos dos se computaron bajando los zips oficiales el 2026-09-10.
# Para sumar una versión: bajá el zip, `sha256sum`, y agregá la fila.
PINES_RUSTDESK="
1.1.14 rustdesk-server-linux-amd64.zip bfee54d3c5dce834ef00906b412d0e8738712d7d9f07393b9ce66d998833d540
1.1.14 rustdesk-server-linux-arm64v8.zip d19fdb711621ad96e794ebc7899dc80d6829c9ae871483df520fb78a48c2d7ac
"
# Se resuelve ANTES de bajar seis megas: si no hay pin, el operador se entera ya.
# RUSTDESK_SHA256 es la salida para una versión sin fila —el operador DA el sha, no saltea la
# comprobación—. No hay variable para instalar sin verificar, por el mismo motivo que en el
# instalador del cerebro: una puerta así termina copiada en un runbook y la verificación queda
# muerta en todas las máquinas mientras se ve viva. Y eso NO es una promesa escrita: en
# internal/mcp, despliegue_verificacion_forma_test.go exige que el `install` de hbbs/hbbr esté
# dominado por la comparación del sha256 —envolver esto en un `if` nuevo lo rompe aunque el `if`
# venga apagado— y despliegue_verificacion_corrida_test.go corre el caso que tiene que frenar una
# vez por cada variable de entorno que este bloque lee.
QUIERO_SHA="${RUSTDESK_SHA256:-$(printf '%s' "$PINES_RUSTDESK" | awk -v v="$VERSION" -v p="$PAQUETE" '$1==v && $2==p {print $3}')}"
if [[ ! "$QUIERO_SHA" =~ ^[0-9a-f]{64}$ ]]; then
  die "no tengo sha256 con el que verificar rustdesk-server $VERSION / $PAQUETE, así que NO instalo nada. Estos binarios quedan como servicios systemd de este servidor y rustdesk no publica checksums, así que sin este número no hay forma de saber qué se instaló. Salidas: (a) usá una versión con pin (hoy: $(printf '%s' "$PINES_RUSTDESK" | awk 'NF{printf "%s ", $1}' | tr ' ' '\n' | sort -u | tr '\n' ' ')); (b) bajá el zip por un camino que confíes, sacale el sha256 y agregá la fila a PINES_RUSTDESK acá; (c) para una corrida sola:  RUSTDESK_SHA256=<sha256> sudo $0"
fi

id -u "$USUARIO" &>/dev/null || useradd --system --home-dir "$DESTINO" --shell /usr/sbin/nologin "$USUARIO"
mkdir -p "$DESTINO"

if [[ ! -x "$DESTINO/hbbs" ]]; then
  log "Descargando rustdesk-server $VERSION ($ARCH)"
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
  curl -fsSL -o "$tmp/s.zip" \
    "https://github.com/rustdesk/rustdesk-server/releases/download/$VERSION/$PAQUETE" \
    || die "no se pudo descargar el relay"
  TENGO_SHA="$(sha256sum "$tmp/s.zip" | awk '{print $1}')"
  [[ "$TENGO_SHA" == "$QUIERO_SHA" ]] || die "el paquete de rustdesk-server $VERSION NO coincide con el sha256 esperado (quiero=$QUIERO_SHA tengo=$TENGO_SHA). NO se instaló nada. O la descarga se truncó, o el asset del release cambió: no lo desempaquetes a mano hasta saber cuál de las dos es."
  ok "checksum del paquete verificado ($PAQUETE)"
  ( cd "$tmp" && unzip -q s.zip )
  find "$tmp" -type f \( -name hbbs -o -name hbbr \) -exec install -m 0755 {} "$DESTINO"/ \;
  [[ -x "$DESTINO/hbbs" && -x "$DESTINO/hbbr" ]] || die "el paquete no traía hbbs/hbbr"
fi
chown -R "$USUARIO:$USUARIO" "$DESTINO"
ok "binarios en $DESTINO"

# ── systemd ──────────────────────────────────────────────────────────────────
# NoNewPrivileges + ProtectSystem: el relay no necesita nada del sistema salvo su directorio.
for svc in hbbs hbbr; do
  extra=""
  [[ "$svc" == "hbbs" ]] && extra=" -r ${BIND}:21117"
  cat > "/etc/systemd/system/rustdesk-$svc.service" <<UNIT
[Unit]
Description=RustDesk $svc (relay de pantalla de la flota Musubi)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$USUARIO
WorkingDirectory=$DESTINO
ExecStart=$DESTINO/$svc -k _ $extra
Restart=always
RestartSec=5
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
UNIT
done

systemctl daemon-reload
systemctl enable --now rustdesk-hbbs rustdesk-hbbr
sleep 2
systemctl is-active --quiet rustdesk-hbbs || die "hbbs no arrancó: journalctl -u rustdesk-hbbs"
systemctl is-active --quiet rustdesk-hbbr || die "hbbr no arrancó: journalctl -u rustdesk-hbbr"
ok "hbbs y hbbr activos"

# ── La clave pública: el dato que va en cada cliente ─────────────────────────
CLAVE="$DESTINO/id_ed25519.pub"
for _ in $(seq 1 20); do [[ -f "$CLAVE" ]] && break; sleep 0.5; done
[[ -f "$CLAVE" ]] || die "hbbs no generó su clave: journalctl -u rustdesk-hbbs"

echo
ok "Relay listo. En CADA máquina de la flota, configurá el cliente RustDesk:"
echo "    ID Server: ${BIND}:21116"
echo "    Key:       $(cat "$CLAVE")"
echo
aviso "Antes de dar por hecho que hay consentimiento, leé deploy/rustdesk/README.md §1:"
aviso "el «confirmar antes de conectar» lo aplica RustDesk, NO Musubi."
