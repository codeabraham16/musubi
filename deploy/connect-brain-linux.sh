#!/usr/bin/env bash
#
# connect-brain-linux.sh — conecta ESTE dispositivo Linux al CEREBRO CENTRAL Musubi.
# Automatiza el "sistema del curl" del lado cliente: Tailscale, allowlist de NordVPN,
# el .mcp.json remoto del proyecto, el token en el entorno, y la verificación.
#
# Uso:
#   MUSUBI_TOKEN=<token-del-cerebro> ./connect-brain-linux.sh [directorio-del-proyecto]
#   # si no pasás el token por env, te lo pregunta.
#   # si no pasás directorio, usa el actual ($PWD).
#
# Variables opcionales:
#   BRAIN_IP    IP del cerebro en el tailnet   (default: 100.79.126.62)
#   BRAIN_PORT  puerto del cerebro             (default: 7717)
#
set -euo pipefail

BRAIN_IP="${BRAIN_IP:-100.79.126.62}"
BRAIN_PORT="${BRAIN_PORT:-7717}"
PROJECT_DIR="${1:-$PWD}"
BRAIN_URL="http://$BRAIN_IP:$BRAIN_PORT/mcp"

log(){ printf '\033[36m▶ %s\033[0m\n' "$*"; }
ok(){  printf '\033[32m✓ %s\033[0m\n' "$*"; }
die(){ printf '\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

# ── 1. Tailscale (instalar + unir a la malla) ───────────────────────────────
if ! command -v tailscale &>/dev/null; then
  log "Instalando Tailscale"
  curl -fsSL https://tailscale.com/install.sh | sh
fi
if ! tailscale status &>/dev/null; then
  log "Uniendo a la malla (te va a pedir login en el navegador)"
  sudo tailscale up
fi
ok "Tailscale activo"

# ── 2. NordVPN: dejar pasar la subred del tailnet (si NordVPN está presente) ─
if command -v nordvpn &>/dev/null; then
  log "NordVPN detectado: permitiendo la subred del tailnet (100.64.0.0/10)"
  nordvpn allowlist add subnet 100.64.0.0/10 2>/dev/null \
    || nordvpn whitelist add subnet 100.64.0.0/10 2>/dev/null \
    || log "No se pudo agregar el allowlist automáticamente; hacelo a mano si hace falta."
fi

# ── 3. Token ────────────────────────────────────────────────────────────────
if [ -z "${MUSUBI_TOKEN:-}" ]; then
  read -rsp "Pegá el MUSUBI_TOKEN del cerebro: " MUSUBI_TOKEN; echo
fi
[ -n "$MUSUBI_TOKEN" ] || die "Token vacío."
# Persistir en el perfil (por referencia, NUNCA en el .mcp.json).
PROFILE="${HOME}/.bashrc"; [ -n "${ZSH_VERSION:-}" ] && PROFILE="${HOME}/.zshrc"
if ! grep -q 'export MUSUBI_TOKEN=' "$PROFILE" 2>/dev/null; then
  echo "export MUSUBI_TOKEN=\"$MUSUBI_TOKEN\"" >> "$PROFILE"
  ok "MUSUBI_TOKEN agregado a $PROFILE (recargá la shell o 'source $PROFILE')"
fi
export MUSUBI_TOKEN

# ── 4. Escribir/mergear el .mcp.json del proyecto (entrada remota "cerebro") ─
command -v python3 &>/dev/null || die "Falta python3 para editar el .mcp.json."
log "Cableando .mcp.json en $PROJECT_DIR"
python3 - "$PROJECT_DIR/.mcp.json" "$BRAIN_URL" <<'PY'
import json, sys, os
path, url = sys.argv[1], sys.argv[2]
try:
    with open(path, encoding="utf-8") as f:
        cfg = json.load(f)
except Exception:
    cfg = {}
cfg.setdefault("mcpServers", {})
cfg["mcpServers"]["musubi-cerebro"] = {
    "type": "http",
    "url": url,
    "headers": {"Authorization": "Bearer ${MUSUBI_TOKEN}"},
}
os.makedirs(os.path.dirname(os.path.abspath(path)), exist_ok=True)
with open(path, "w", encoding="utf-8") as f:
    json.dump(cfg, f, indent=2, ensure_ascii=False)
    f.write("\n")
print("ok")
PY
ok ".mcp.json actualizado (entrada 'musubi-cerebro', token por \${MUSUBI_TOKEN})"

# ── 5. Verificación ─────────────────────────────────────────────────────────
log "Verificando alcance al cerebro"
if curl -fsS "http://$BRAIN_IP:$BRAIN_PORT/readyz" >/dev/null; then
  ok "Cerebro alcanzable: http://$BRAIN_IP:$BRAIN_PORT/readyz"
  if curl -fsS -H "Authorization: Bearer $MUSUBI_TOKEN" -X POST "$BRAIN_URL" \
       -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | grep -q '"tools"'; then
    ok "Autenticación OK: el cerebro devuelve el catálogo de tools"
  else
    die "Llega el readyz pero el token no autentica. Revisá el MUSUBI_TOKEN."
  fi
else
  die "No se alcanza el cerebro. ¿Tailscale conectado? ¿NordVPN dejando pasar 100.64.0.0/10? Probá: tailscale ping $BRAIN_IP"
fi

echo
ok "DISPOSITIVO CONECTADO. Este proyecto ($PROJECT_DIR) ya usa el cerebro central."
