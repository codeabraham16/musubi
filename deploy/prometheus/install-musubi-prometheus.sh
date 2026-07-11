#!/usr/bin/env bash
#
# install-musubi-prometheus.sh — provisiona Prometheus para monitorear el CEREBRO CENTRAL de
# Musubi. Reproduce, idempotente, el montaje de un Prometheus systemd nativo que scrapea el
# /metrics del cerebro y evalúa las reglas de deploy/musubi-alerts.yml. Cierra el hueco de la
# auditoría: /metrics exponía contadores ricos pero nada disparaba sobre ellos.
#
# Corre EN EL MISMO server que el cerebro (scrapea 127.0.0.1:7717 por loopback). Re-ejecutable
# sin romper nada: no pisa datos, revalida la config con promtool antes de (re)arrancar.
#
# Uso:
#   sudo ./install-musubi-prometheus.sh
#
# Variables de entorno opcionales (con defaults):
#   PROM_VERSION   versión de Prometheus a instalar              (default: 2.53.2, LTS)
#   PROM_ADDR      bind de la UI de Prometheus                   (default: 127.0.0.1:9090)
#   PROM_USER      usuario de sistema que corre Prometheus       (default: prometheus)
#   PROM_RETENTION retención de la TSDB                          (default: 30d)
#   MUSUBI_ADDR    dónde está el /metrics del cerebro            (default: 127.0.0.1:7717)
#   MUSUBI_ENV     EnvironmentFile del cerebro (de donde sale el token)  (default: /etc/musubi/musubi.env)
#   TOKEN_VAR      nombre de la variable del token en MUSUBI_ENV (default: MUSUBI_TOKEN)
#
set -euo pipefail

PROM_VERSION="${PROM_VERSION:-2.53.2}"
PROM_ADDR="${PROM_ADDR:-127.0.0.1:9090}"
PROM_USER="${PROM_USER:-prometheus}"
PROM_RETENTION="${PROM_RETENTION:-30d}"
MUSUBI_ADDR="${MUSUBI_ADDR:-127.0.0.1:7717}"
MUSUBI_ENV="${MUSUBI_ENV:-/etc/musubi/musubi.env}"
TOKEN_VAR="${TOKEN_VAR:-MUSUBI_TOKEN}"

ETC="/etc/prometheus"
DATA="/var/lib/prometheus"
UNIT="/etc/systemd/system/prometheus.service"
TOKEN_FILE="$ETC/musubi.token"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log(){ printf '\033[36m▶ %s\033[0m\n' "$*"; }
ok(){  printf '\033[32m✓ %s\033[0m\n' "$*"; }
die(){ printf '\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "Corré con sudo/root."
command -v curl &>/dev/null    || die "Falta 'curl'."
command -v sha256sum &>/dev/null || die "Falta 'sha256sum'."
command -v tar &>/dev/null     || die "Falta 'tar'."

case "$(uname -m)" in
  x86_64)        ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "Arquitectura no soportada: $(uname -m)" ;;
esac

# ── 1. Usuario de sistema ────────────────────────────────────────────────────
if ! id "$PROM_USER" &>/dev/null; then
  log "Creando usuario de sistema '$PROM_USER'"
  useradd --system --no-create-home --shell /usr/sbin/nologin "$PROM_USER"
fi

# ── 2. Binarios de Prometheus (download + verify sha256 del release oficial) ──
NAME="prometheus-${PROM_VERSION}.linux-${ARCH}"
BASE="https://github.com/prometheus/prometheus/releases/download/v${PROM_VERSION}"
log "Descargando Prometheus v$PROM_VERSION ($ARCH)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$BASE/${NAME}.tar.gz" -o "$tmp/prom.tar.gz"
# Prometheus publica sha256sums.txt en cada release; verificamos contra él (no hash hardcodeado).
curl -fsSL "$BASE/sha256sums.txt" -o "$tmp/sha256sums.txt"
want="$(awk -v f="${NAME}.tar.gz" '$2==f{print $1}' "$tmp/sha256sums.txt")"
[ -n "$want" ] || die "No encontré el checksum de ${NAME}.tar.gz en sha256sums.txt"
got="$(sha256sum "$tmp/prom.tar.gz" | awk '{print $1}')"
[ "$want" = "$got" ] || die "Checksum no coincide (want=$want got=$got)"
ok "Checksum verificado"
tar -xzf "$tmp/prom.tar.gz" -C "$tmp"
install -m 0755 "$tmp/$NAME/prometheus" /usr/local/bin/prometheus
install -m 0755 "$tmp/$NAME/promtool"   /usr/local/bin/promtool
if command -v restorecon &>/dev/null; then restorecon -v /usr/local/bin/prometheus /usr/local/bin/promtool || true; fi
ok "Instalado: $(/usr/local/bin/prometheus --version 2>&1 | head -1)"

# ── 3. Directorios ───────────────────────────────────────────────────────────
install -d -m 0755 "$ETC" "$ETC/rules"
install -d -o "$PROM_USER" -g "$PROM_USER" -m 0750 "$DATA"

# ── 4. Config + reglas (desde este repo; la config es del repo, no autogenerada) ──
install -m 0644 "$HERE/prometheus.yml" "$ETC/prometheus.yml"
# Las reglas viven un nivel arriba (deploy/musubi-alerts.yml), compartidas con el runbook.
if [ -f "$HERE/../musubi-alerts.yml" ]; then
  install -m 0644 "$HERE/../musubi-alerts.yml" "$ETC/rules/musubi-alerts.yml"
else
  die "No encontré ../musubi-alerts.yml junto al script. Cloná el repo completo."
fi

# ── 5. Token del cerebro → credentials_file (solo lectura para el user de Prometheus) ──
[ -f "$MUSUBI_ENV" ] || die "No existe $MUSUBI_ENV. ¿Está instalado el cerebro (install-musubi-brain.sh)?"
TOKEN_VAL="$(grep -E "^${TOKEN_VAR}=" "$MUSUBI_ENV" | head -1 | cut -d= -f2-)"
[ -n "$TOKEN_VAL" ] || die "No pude leer $TOKEN_VAR de $MUSUBI_ENV (¿está vacío?)."
umask 077
printf '%s' "$TOKEN_VAL" > "$TOKEN_FILE"      # sin newline: Prometheus usa el contenido tal cual
chown "$PROM_USER:$PROM_USER" "$TOKEN_FILE"
chmod 600 "$TOKEN_FILE"
ok "Token del cerebro copiado a $TOKEN_FILE (0600, $PROM_USER)"

# El scrape target usa MUSUBI_ADDR; si no es el default, lo reflejamos en la config.
if [ "$MUSUBI_ADDR" != "127.0.0.1:7717" ]; then
  sed -i "s|127.0.0.1:7717|$MUSUBI_ADDR|" "$ETC/prometheus.yml"
fi
chown -R "$PROM_USER:$PROM_USER" "$ETC"

# ── 6. Validar ANTES de arrancar (así un typo no deja el servicio en crashloop) ──
log "Validando config y reglas con promtool"
/usr/local/bin/promtool check config "$ETC/prometheus.yml" >/dev/null || die "prometheus.yml inválido"
/usr/local/bin/promtool check rules "$ETC/rules/musubi-alerts.yml" >/dev/null || die "musubi-alerts.yml inválido"
ok "Config y 7 reglas válidas"

# ── 7. Servicio systemd ──────────────────────────────────────────────────────
log "Escribiendo unit systemd (bind $PROM_ADDR)"
cat > "$UNIT" <<EOF
[Unit]
Description=Prometheus (monitoreo del cerebro Musubi)
After=network-online.target musubi-brain.service
Wants=network-online.target

[Service]
Type=simple
User=$PROM_USER
Group=$PROM_USER
ExecStart=/usr/local/bin/prometheus \\
  --config.file=$ETC/prometheus.yml \\
  --storage.tsdb.path=$DATA \\
  --storage.tsdb.retention.time=$PROM_RETENTION \\
  --web.listen-address=$PROM_ADDR \\
  --web.enable-lifecycle
ExecReload=/bin/kill -HUP \$MAINPID
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA
ProtectControlGroups=true
ProtectKernelTunables=true
ProtectKernelModules=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now prometheus.service
ok "Servicio systemd habilitado y arrancado"

# ── 8. Verificación ──────────────────────────────────────────────────────────
sleep 2
PROM_URL="http://${PROM_ADDR}"
if curl -fsS "$PROM_URL/-/ready" >/dev/null 2>&1; then
  ok "Prometheus responde: $PROM_URL"
else
  die "Prometheus no responde. Revisá:  journalctl -u prometheus -n 40 --no-pager"
fi
# ¿El target del cerebro está UP? (si no, casi seguro es el token o el bind del cerebro)
sleep 3
if curl -fsS "$PROM_URL/api/v1/targets?state=active" 2>/dev/null | grep -q '"job":"musubi".*"health":"up"' \
   || curl -fsS "$PROM_URL/api/v1/query?query=up%7Bjob%3D%22musubi%22%7D" 2>/dev/null | grep -q '"value":\[.*"1"\]'; then
  ok "Target 'musubi' UP: Prometheus está scrapeando el cerebro"
else
  log "El target 'musubi' aún no figura UP. Chequealo en $PROM_URL/targets"
  log "Causas típicas: token incorrecto en $TOKEN_FILE, o el cerebro no bindea $MUSUBI_ADDR."
fi

echo
ok "PROMETHEUS LISTO. Reglas activas en $PROM_URL/alerts"
echo "  • UI (loopback):   $PROM_URL   — exponela por la malla o por túnel SSH si la querés remota."
echo "  • Para NOTIFICAR (paginar): sumá Alertmanager + un canal y descomentá 'alerting:' en $ETC/prometheus.yml"
echo "  • Runbook por alerta: deploy/RUNBOOK.md"
