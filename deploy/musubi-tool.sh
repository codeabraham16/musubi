#!/usr/bin/env bash
# ════════════════════════════════════════════════════════════════════════════════════════════
# musubi-tool.sh — invoca UNA tool del cerebro central desde la línea de comandos.
#
# Existe porque las operaciones de flota que hace una PERSONA —declarar un servicio, enrolar un
# Tier B, revocar una máquina— no tenían más camino que armar el JSON-RPC a mano. Un procedimiento
# que se escribe a mano cada vez se escribe mal una de cada cinco, y la que sale mal es la que se
# hace apurado.
#
#   ./musubi-tool.sh musubi_fleet_list '{}'
#   ./musubi-tool.sh musubi_fleet_service_declare '{"device":"musubi-server","nombre":"alturito20"}'
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# EL TOKEN NUNCA VA EN LA LÍNEA DE COMANDOS
#
# Sale de una variable de entorno o de un archivo, y viaja a curl por su CONFIG (-K -), no por
# argv. Un token en argv lo ve cualquier `ps` de la máquina y queda en el historial del shell —
# que es exactamente donde no querés una credencial de admin.
#
#   MUSUBI_TOKEN=...            el valor, o
#   MUSUBI_TOKEN_FILE=/ruta     un archivo modo 600 con el valor (recomendado)
#   MUSUBI_CENTRAL_URL          default http://127.0.0.1:7717
# ════════════════════════════════════════════════════════════════════════════════════════════
set -euo pipefail

TOOL="${1:-}"
ARGS="${2:-{\}}"
URL="${MUSUBI_CENTRAL_URL:-http://127.0.0.1:7717}"

if [ -z "$TOOL" ]; then
  echo "uso: $0 <nombre_de_tool> '<json de argumentos>'" >&2
  echo "ej:  $0 musubi_fleet_list '{}'" >&2
  exit 2
fi

TOKEN="${MUSUBI_TOKEN:-}"
if [ -z "$TOKEN" ] && [ -n "${MUSUBI_TOKEN_FILE:-}" ]; then
  TOKEN="$(cat "$MUSUBI_TOKEN_FILE")"
fi
if [ -z "$TOKEN" ]; then
  echo "falta la credencial: exportá MUSUBI_TOKEN o MUSUBI_TOKEN_FILE" >&2
  exit 2
fi

# El JSON se arma con python y no con printf: los argumentos llevan comillas, y concatenar a mano
# es cómo se manda `{"device":"a"b"}` sin enterarse. Además valida el JSON de entrada ANTES de
# mandarlo, así un error de tipeo se ve acá y no como un -32700 del otro lado.
CUERPO="$(python3 -c '
import json, sys
try:
    args = json.loads(sys.argv[2])
except json.JSONDecodeError as e:
    sys.exit("los argumentos no son JSON válido: %s" % e)
print(json.dumps({"jsonrpc": "2.0", "id": "cli", "method": "tools/call",
                  "params": {"name": sys.argv[1], "arguments": args}}))
' "$TOOL" "$ARGS")"

printf 'header = "Authorization: Bearer %s"\n' "$TOKEN" \
  | curl -sS -K - -X POST "$URL/mcp" \
      -H "Content-Type: application/json" \
      --data-binary "$CUERPO" \
  | python3 -c '
import json, sys
try:
    r = json.load(sys.stdin)
except json.JSONDecodeError:
    sys.exit("el cerebro no devolvió JSON (¿URL o token mal?)")
if "error" in r:
    e = r["error"]
    sys.exit("ERROR %s: %s" % (e.get("code"), e.get("message")))
# El resultado de una tool viene envuelto en `content[].text` con el JSON adentro. Se desenvuelve
# acá para que la salida sea legible en una terminal y filtrable con jq.
res = r.get("result", {})
for c in res.get("content", []):
    if c.get("type") == "text":
        try:
            print(json.dumps(json.loads(c["text"]), indent=2, ensure_ascii=False))
        except json.JSONDecodeError:
            print(c["text"])
        break
else:
    print(json.dumps(res, indent=2, ensure_ascii=False))
'
