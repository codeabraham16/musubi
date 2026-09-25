#!/usr/bin/env bash
#
# instalar-adjudicador-local.sh — instala el adjudicador B1 contra la memoria LOCAL de una máquina
# de desarrollo (Linux, systemd de usuario). Lo corre la persona dueña de la máquina, no el agente:
# deja una tarea que corre sola tres veces por día.
#
# Qué deja:
#   ~/.local/share/musubi-adjudicador/b1-adjudicar.sh   copia de deploy/adjudicador/b1-adjudicar.sh
#   ~/.local/share/musubi-adjudicador/mcp.json           `musubi daemon` por stdio, con MUSUBI_HOME
#   ~/.config/systemd/user/musubi-adjudicador.{service,timer}
#
# MUSUBI_HOME ES OBLIGATORIO EN EL mcp.json, y no es un detalle. Sin él el daemon toma el workspace
# del directorio desde el que arranca, y el timer arranca en $HOME: ahí `ensureWorkspace` planta un
# config y una memoria NUEVOS, y el adjudicador juzga una cola vacía de una memoria que nadie usa,
# con todo en verde. Es la sombra de A96. Lo custodia TestElAdjudicadorLocalFijaSuMemoria.
#
# Uso:  ./deploy/adjudicador/instalar-adjudicador-local.sh [--probar]
#       --probar  además de instalar, corre el adjudicador UNA vez ahora y muestra la salida.
#
# Variables opcionales:
#   MUSUBI_REPO   la carpeta cuya memoria (.musubi/memory.db) se adjudica (default: este repo)
#   MUSUBI_BIN    el binario de musubi (default: ~/.local/bin/musubi)
#   B1_LOTE       relaciones por corrida (default: 30)
set -euo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${MUSUBI_REPO:-$(cd "$AQUI/../.." && pwd)}"
BIN="${MUSUBI_BIN:-$HOME/.local/bin/musubi}"
LOTE="${B1_LOTE:-30}"
# El nombre del servidor MCP se define UNA vez: va al mcp.json y a B1_SERVIDOR de la unit. Si los
# dos difirieran, las herramientas permitidas (mcp__<nombre>__…) no existirían y el adjudicador
# no juzgaría nada, sin fallar.
SERVIDOR=musubi
DESTINO="$HOME/.local/share/musubi-adjudicador"
UNIDADES="$HOME/.config/systemd/user"

die() { echo "✗ $*" >&2; exit 1; }

echo "══ comprobaciones"
[[ -f "$REPO/.musubi/memory.db" ]] || die "no hay memoria en $REPO/.musubi/memory.db: ¿MUSUBI_REPO apunta al repo correcto?"
[[ -x "$BIN" ]] || die "no encuentro el binario de musubi en $BIN (fijá MUSUBI_BIN)"
command -v claude >/dev/null || die "no encuentro 'claude' en el PATH: el adjudicador es Claude Code sin interfaz"
command -v python3 >/dev/null || die "falta python3 (resume la salida de claude)"
[[ "$LOTE" =~ ^[0-9]+$ ]] || die "B1_LOTE tiene que ser un número (llegó '$LOTE')"
echo "  memoria:  $REPO/.musubi/memory.db"
echo "  musubi:   $BIN ($("$BIN" version 2>/dev/null | head -1))"
echo "  claude:   $(command -v claude) ($(claude --version 2>/dev/null | head -1))"

echo "══ instalar"
mkdir -p "$DESTINO" "$UNIDADES"
install -m 755 "$AQUI/b1-adjudicar.sh" "$DESTINO/b1-adjudicar.sh"
python3 - "$DESTINO/mcp.json" "$BIN" "$REPO" "$SERVIDOR" <<'PY'
import json, sys
destino, binario, repo, servidor = sys.argv[1:]
cfg = {"mcpServers": {servidor: {
    "type": "stdio",
    "command": binario,
    "args": ["daemon"],
    # Sin MUSUBI_HOME el daemon adjudica una memoria sombra en $HOME. Ver el encabezado.
    "env": {"MUSUBI_HOME": repo},
}}}
open(destino, "w").write(json.dumps(cfg, indent=2) + "\n")
PY
chmod 600 "$DESTINO/mcp.json"

cat >"$UNIDADES/musubi-adjudicador.service" <<EOF
[Unit]
Description=Musubi — adjudicador B1 de la memoria local ($REPO)

[Service]
Type=oneshot
Environment=PATH=$(dirname "$(command -v claude)"):/usr/local/bin:/usr/bin:/bin
Environment=B1_MCP_CONFIG=$DESTINO/mcp.json
Environment=B1_SERVIDOR=$SERVIDOR
ExecStart=/usr/bin/env bash $DESTINO/b1-adjudicar.sh $LOTE
Nice=10
EOF

# Persistent=true: una laptop apagada a la hora del disparo corre la corrida perdida al prender.
cat >"$UNIDADES/musubi-adjudicador.timer" <<'EOF'
[Unit]
Description=Dispara el adjudicador B1 de la memoria local tres veces por día

[Timer]
OnCalendar=*-*-* 04:30:00
OnCalendar=*-*-* 12:30:00
OnCalendar=*-*-* 20:30:00
RandomizedDelaySec=900
Persistent=true

[Install]
WantedBy=timers.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now musubi-adjudicador.timer
echo "  próximo disparo: $(systemctl --user list-timers musubi-adjudicador.timer --no-legend --no-pager | awk '{print $1, $2, $3}')"

if [[ "${1:-}" == --probar ]]; then
	echo "══ una corrida de prueba, ahora"
	systemctl --user start musubi-adjudicador.service || true
	journalctl --user -u musubi-adjudicador.service -n 40 --no-pager -o cat
fi
echo "Listo. Para ver cómo le fue:  journalctl --user -u musubi-adjudicador -n 50"
