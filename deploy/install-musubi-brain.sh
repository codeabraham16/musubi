#!/usr/bin/env bash
#
# install-musubi-brain.sh — provisiona Musubi como CEREBRO CENTRAL (daemon MCP sobre
# HTTP) en un servidor Linux. Reproduce, idempotente, el montaje manual de la Fase 1:
# binario + workspace + bloque service + token + servicio systemd + contexto SELinux +
# firewall de la malla + verificación. Re-ejecutable sin romper nada (NO regenera el
# token si ya existe).
#
# Uso:
#   sudo ./install-musubi-brain.sh
#
# Variables de entorno opcionales (con defaults):
#   BRAIN_USER      usuario que corre el daemon; debe existir   (default: musubi)
#   BRAIN_HOME      workspace del cerebro = MUSUBI_HOME          (default: /home/$BRAIN_USER/musubi-brain)
#   BRAIN_ADDR      dirección de bind (tailnet-only vía fw)      (default: 0.0.0.0:7717)
#   MUSUBI_VERSION  tag de release o "latest"                   (default: latest)
#   MUSUBI_REPO     owner/repo de las releases                  (default: codeabraham16/musubi)
#
set -euo pipefail

BRAIN_USER="${BRAIN_USER:-musubi}"
BRAIN_HOME="${BRAIN_HOME:-/home/$BRAIN_USER/musubi-brain}"
BRAIN_ADDR="${BRAIN_ADDR:-0.0.0.0:7717}"
MUSUBI_VERSION="${MUSUBI_VERSION:-latest}"
MUSUBI_REPO="${MUSUBI_REPO:-codeabraham16/musubi}"
ENV_FILE="/etc/musubi/musubi.env"
BIN="/usr/local/bin/musubi"
UNIT="/etc/systemd/system/musubi-brain.service"
PORT="${BRAIN_ADDR##*:}"

log(){ printf '\033[36m▶ %s\033[0m\n' "$*"; }
ok(){  printf '\033[32m✓ %s\033[0m\n' "$*"; }
die(){ printf '\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "Corré con sudo/root."
id "$BRAIN_USER" &>/dev/null || die "El usuario '$BRAIN_USER' no existe. Crealo primero (useradd -m $BRAIN_USER)."
command -v curl &>/dev/null || die "Falta 'curl'."
command -v openssl &>/dev/null || die "Falta 'openssl'."

case "$(uname -m)" in
  x86_64)        ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "Arquitectura no soportada: $(uname -m)" ;;
esac

# ── 1. Binario ──────────────────────────────────────────────────────────────
if [ "$MUSUBI_VERSION" = latest ]; then
  URL="https://github.com/$MUSUBI_REPO/releases/latest/download/musubi-linux-$ARCH"
else
  URL="https://github.com/$MUSUBI_REPO/releases/download/$MUSUBI_VERSION/musubi-linux-$ARCH"
fi
log "Descargando binario ($ARCH): $URL"
tmp="$(mktemp)"; tmpsha="$(mktemp)"
curl -fsSL "$URL" -o "$tmp"
if curl -fsSL "$URL.sha256" -o "$tmpsha" && [ -s "$tmpsha" ]; then
  want="$(awk '{print $1}' "$tmpsha")"
  got="$(sha256sum "$tmp" | awk '{print $1}')"
  [ "$want" = "$got" ] || die "Checksum no coincide (want=$want got=$got)"
  ok "Checksum verificado"
fi
# 'install' (no 'mv') aplica el contexto correcto del destino; igual forzamos restorecon.
install -m 0755 "$tmp" "$BIN"
rm -f "$tmp" "$tmpsha"
if command -v restorecon &>/dev/null; then restorecon -v "$BIN" || true; fi   # SELinux (gotcha Fase 1)
ok "Binario instalado: $("$BIN" version)"

# ── 2. Workspace ────────────────────────────────────────────────────────────
if [ ! -f "$BRAIN_HOME/.musubi/config.yaml" ]; then
  log "Inicializando workspace en $BRAIN_HOME"
  install -d -o "$BRAIN_USER" -g "$BRAIN_USER" "$BRAIN_HOME"
  sudo -u "$BRAIN_USER" env MUSUBI_HOME="$BRAIN_HOME" "$BIN" init
else
  ok "Workspace ya existe: $BRAIN_HOME"
fi
CFG="$BRAIN_HOME/.musubi/config.yaml"

# ── 3. Bloque service (idempotente: siempre lo deja en el estado deseado) ────
# 'service:' es el último bloque del config generado por 'musubi init'.
log "Configurando bloque service (addr=$BRAIN_ADDR)"
cp -f "$CFG" "$CFG.bak"
sed -i '/^service:/,$d' "$CFG"
cat >> "$CFG" <<EOF
service:
    enabled: true
    addr: "$BRAIN_ADDR"
    auth_token_env: "MUSUBI_TOKEN"
    allow_insecure_token: true
    request_timeout_seconds: 60
EOF
chown "$BRAIN_USER:$BRAIN_USER" "$CFG"
ok "Bloque service configurado"

# ── 4. Token (NO se regenera si ya existe: romperia a los clientes) ─────────
install -d -m 0755 /etc/musubi
if [ ! -f "$ENV_FILE" ]; then
  log "Generando token"
  umask 077
  echo "MUSUBI_TOKEN=$(openssl rand -hex 32)" > "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  ok "Token generado en $ENV_FILE"
else
  ok "Token ya existe (no se regenera): $ENV_FILE"
fi

# ── 5. Servicio systemd ─────────────────────────────────────────────────────
log "Escribiendo unit systemd"
cat > "$UNIT" <<EOF
[Unit]
Description=Musubi cerebro central (MCP HTTP daemon)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$BRAIN_USER
Group=$BRAIN_USER
Environment=MUSUBI_HOME=$BRAIN_HOME
EnvironmentFile=$ENV_FILE
ExecStart=$BIN serve
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=$BRAIN_HOME
ProtectControlGroups=true
ProtectKernelTunables=true
ProtectKernelModules=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now musubi-brain.service
ok "Servicio systemd habilitado y arrancado"

# ── 6. Firewall de la malla (best-effort) ───────────────────────────────────
if command -v firewall-cmd &>/dev/null && systemctl is-active --quiet firewalld; then
  if ip link show tailscale0 &>/dev/null; then
    firewall-cmd --zone=trusted --add-interface=tailscale0 --permanent &>/dev/null || true
    firewall-cmd --reload &>/dev/null || true
    ok "firewalld: tailscale0 en zona 'trusted' (puerto $PORT solo alcanzable por la malla)"
  else
    log "tailscale0 aún no existe. Tras 'tailscale up', corré:"
    printf '    firewall-cmd --zone=trusted --add-interface=tailscale0 --permanent && firewall-cmd --reload\n'
  fi
fi

# ── 7. Verificación ─────────────────────────────────────────────────────────
sleep 1
if curl -fsS "http://127.0.0.1:$PORT/readyz" >/dev/null; then
  ok "Cerebro respondiendo: http://127.0.0.1:$PORT/readyz"
else
  die "El daemon no responde. Revisá:  journalctl -u musubi-brain -n 30 --no-pager"
fi

echo
ok "CEREBRO LISTO. Token para los clientes (guardalo seguro):"
cat "$ENV_FILE"
echo
echo "Siguiente: en cada dispositivo, connect-brain-linux.sh / connect-brain-windows.ps1"
