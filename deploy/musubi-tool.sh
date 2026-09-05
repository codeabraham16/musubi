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

# LA FUENTE DE LA CREDENCIAL SE NOMBRA, Y LA PRECEDENCIA SE AVISA (A65).
#
# `MUSUBI_TOKEN` GANA sobre `MUSUBI_TOKEN_FILE`, y eso es correcto —una variable puesta a mano es
# una decisión más reciente que un archivo— pero es silencioso, y ahí está el problema: una
# variable a medio setear de hace media hora le gana al archivo que acabás de crear, y el 401 que
# vuelve no menciona ninguna de las dos.
#
# Costó cuatro intentos el 2026-08-31, con el YAML, el hash, la ruta, el proceso y la recarga
# TODOS verificados correctos. La causa estaba en el shell, que era el único lugar donde nadie
# miró porque nada apuntaba ahí.
TOKEN="${MUSUBI_TOKEN:-}"
FUENTE="la variable MUSUBI_TOKEN"
if [ -z "$TOKEN" ] && [ -n "${MUSUBI_TOKEN_FILE:-}" ]; then
  TOKEN="$(cat "$MUSUBI_TOKEN_FILE")"
  FUENTE="el archivo $MUSUBI_TOKEN_FILE"
fi
if [ -z "$TOKEN" ]; then
  echo "falta la credencial: exportá MUSUBI_TOKEN o MUSUBI_TOKEN_FILE" >&2
  exit 2
fi
if [ -n "${MUSUBI_TOKEN:-}" ] && [ -n "${MUSUBI_TOKEN_FILE:-}" ]; then
  echo "aviso: MUSUBI_TOKEN y MUSUBI_TOKEN_FILE están las dos puestas. Se usa LA VARIABLE; el archivo ni se abre." >&2
  echo "       Si querías el archivo: unset MUSUBI_TOKEN" >&2
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

# EL CÓDIGO HTTP VIAJA CON EL CUERPO, en su propia última línea. Sin él, un 401 (que contesta
# `unauthorized` en texto plano) y una URL equivocada (que contesta un 404 en HTML) daban EL MISMO
# mensaje: «no devolvió JSON». Dos causas muy distintas con un solo diagnóstico, y ninguna de las
# dos veces que pasó era la que el mensaje nombraba.
RESPUESTA="$(printf 'header = "Authorization: Bearer %s"\n' "$TOKEN" \
  | curl -sS -K - -X POST "$URL/mcp" \
      -H "Content-Type: application/json" \
      --data-binary "$CUERPO" \
      -w '\n%{http_code}')"

printf '%s' "$RESPUESTA" | python3 -c '
import json, sys
crudo = sys.stdin.read()
cuerpo, _, codigo = crudo.rpartition("\n")
fuente, largo, url = sys.argv[1], sys.argv[2], sys.argv[3]

# UN 401 NO ES «URL O TOKEN MAL» (A65): son varias causas y la más probable no es ninguna de esas
# dos. Se nombran en orden de probabilidad REAL, medido: las dos veces que pasó el 2026-08-31, la
# causa fue una de las dos primeras.
if codigo == "401":
    sys.exit(
        "el cerebro RECHAZÓ la credencial (HTTP 401).\n"
        "  se usó: %s (%s caracteres)\n"
        "  causas, en orden de probabilidad:\n"
        "   1. la credencial es buena pero el cerebro TODAVÍA NO LA RECARGÓ. El vigía mira el\n"
        "      mtime de principals.yaml cada 10 s: si la acabás de dar de alta, esperá y reintentá.\n"
        "   2. estás mandando otra credencial de la que creés. Mirá la línea «se usó» de arriba:\n"
        "      MUSUBI_TOKEN le gana a MUSUBI_TOKEN_FILE, y una variable vieja gana en silencio.\n"
        "   3. el principal fue revocado, o su token_sha256 no es el SHA-256 de este token."
        % (fuente, largo))

if codigo not in ("200", ""):
    sys.exit("el cerebro contestó HTTP %s en %s/mcp\n  cuerpo: %s" % (codigo, url, cuerpo[:300]))

try:
    r = json.loads(cuerpo)
except json.JSONDecodeError:
    sys.exit("el cerebro contestó HTTP %s pero el cuerpo no es JSON:\n  %s" % (codigo, cuerpo[:300]))
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
' "$FUENTE" "${#TOKEN}" "$URL"
