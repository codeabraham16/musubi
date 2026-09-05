#!/usr/bin/env bash
#
# precedencia-del-token.sh — comprueba, CORRIENDO musubi-tool.sh de verdad, que la credencial que
# viaja es la que A101 decidió: gana el archivo, y un archivo roto o de varias líneas es un error.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# POR QUÉ ESTA PRUEBA ES DE COMPORTAMIENTO Y NO DE GREP
#
# El 2026-09-05 se auditaron las 46 guardas del repo que miran archivos de despliegue con `grep`:
# SIETE no se ponían rojas con su propio sabotaje declarado, todas por la misma razón —el texto que
# buscaban vivía en un comentario, en un mensaje de error, en un prefijo o en un vecino—. Una
# guarda de texto sobre este guion tendría el mismo problema: la palabra «archivo» aparece treinta
# veces acá adentro y ninguna de ellas decide cuál credencial sale por el socket.
#
# Lo único que contesta la pregunta es levantar un oidor, correr el guion, y MIRAR QUÉ BEARER
# LLEGÓ. Es lo que hace esto.
#
# Uso:  ./deploy/pruebas/precedencia-del-token.sh
set -uo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOL="$AQUI/../musubi-tool.sh"
TMP="$(mktemp -d)"
PUERTO="${PUERTO:-8913}"
OIDOR=""
limpiar(){ [ -n "$OIDOR" ] && kill "$OIDOR" 2>/dev/null; rm -rf "$TMP"; }
trap limpiar EXIT INT TERM

fallas=0
ok(){   printf '  \033[32m✓\033[0m %s\n' "$1"; }
mal(){  printf '  \033[31m✗ %s\033[0m\n' "$1"; fallas=$((fallas+1)); }

[ -x "$TOOL" ] || { echo "no encuentro musubi-tool.sh en $TOOL"; exit 2; }

# El oidor contesta un JSON-RPC válido y ESCRIBE el Authorization que recibió. No valida nada:
# lo que se mide es qué credencial eligió el guion, no si el cerebro la acepta.
python3 - "$PUERTO" "$TMP/auth.log" <<'PY' &
import http.server, json, sys
puerto, registro = int(sys.argv[1]), sys.argv[2]
class H(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", 0)))
        with open(registro, "a", encoding="utf-8") as f:
            f.write((self.headers.get("Authorization") or "(sin cabecera)") + "\n")
        c = json.dumps({"jsonrpc": "2.0", "id": "cli",
                        "result": {"content": [{"type": "text", "text": "{}"}]}}).encode()
        self.send_response(200); self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(c))); self.end_headers(); self.wfile.write(c)
    def log_message(self, *a): pass
http.server.HTTPServer(("127.0.0.1", puerto), H).serve_forever()
PY
OIDOR=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do
  (exec 3<>/dev/tcp/127.0.0.1/"$PUERTO") 2>/dev/null && break
  sleep 0.3
done
(exec 3<>/dev/tcp/127.0.0.1/"$PUERTO") 2>/dev/null || { echo "el oidor no levantó en $PUERTO"; exit 2; }

printf 'tok-DEL-ARCHIVO\n'                > "$TMP/uno.token"
printf 'tok-primera\ntok-segunda\n'       > "$TMP/dos.token"

corre(){ # $1 MUSUBI_TOKEN · $2 MUSUBI_TOKEN_FILE → deja la salida en $SALIDA y el código en $CODIGO
  : > "$TMP/auth.log"
  SALIDA="$(env MUSUBI_TOKEN="$1" MUSUBI_TOKEN_FILE="$2" MUSUBI_CENTRAL_URL="http://127.0.0.1:$PUERTO" \
    "$TOOL" musubi_fleet_list '{}' 2>&1)"
  CODIGO=$?
  BEARER="$(tail -1 "$TMP/auth.log" 2>/dev/null || true)"
}

echo "▶ precedencia del token (A101: gana el archivo)"

# ── 1 · LAS DOS PUESTAS: tiene que viajar la del ARCHIVO ────────────────────────────────────
corre "de-la-VARIABLE" "$TMP/uno.token"
if [ "$BEARER" = "Bearer tok-DEL-ARCHIVO" ]; then
  ok "con las dos puestas viaja la del archivo"
else
  mal "con las dos puestas viajó «$BEARER»; se esperaba «Bearer tok-DEL-ARCHIVO». La variable le ganó al archivo: A101 al revés."
fi

# ── 2 · SÓLO LA VARIABLE: sigue funcionando ─────────────────────────────────────────────────
corre "de-la-VARIABLE" ""
if [ "$BEARER" = "Bearer de-la-VARIABLE" ]; then
  ok "sin archivo, la variable se usa igual"
else
  mal "sin archivo tenía que viajar la variable; viajó «$BEARER»"
fi

# ── 3 · ARCHIVO ROTO + VARIABLE BUENA: error, y NO se cae a la variable ─────────────────────
#
# Es la mitad peligrosa de invertir la precedencia. Caer a la variable convertiría una
# configuración rota en una credencial silenciosa DISTINTA de la que se pidió — y con la variable
# buena puesta ni siquiera fallaría, así que nadie se entera de que el archivo está roto hasta la
# próxima rotación, cuando ya no hay a qué volver.
corre "de-la-VARIABLE" "$TMP/no-existe"
if [ "$CODIGO" -ne 0 ] && [ -z "$BEARER" ]; then
  ok "archivo roto: falla y no manda nada"
else
  mal "con el archivo roto salió con código $CODIGO y mandó «$BEARER»: cayó a la variable en silencio"
fi

# ── 4 · ARCHIVO DE VARIAS LÍNEAS: error, y NO manda la primera ──────────────────────────────
#
# Antes mandaba la primera línea con exit=0 (medido el 2026-09-05). Y la primera línea es el token
# MÁS VIEJO, porque el formato de lista se apenda: elegía justo el que la rotación retira.
corre "" "$TMP/dos.token"
if [ "$CODIGO" -ne 0 ] && [ -z "$BEARER" ]; then
  ok "archivo de dos líneas: falla y no manda la primera"
else
  mal "con dos líneas salió con código $CODIGO y mandó «$BEARER»: eligió una credencial a ciegas"
fi

# ── 5 · NINGUNA: error claro ────────────────────────────────────────────────────────────────
corre "" ""
if [ "$CODIGO" -ne 0 ] && echo "$SALIDA" | grep -q "falta la credencial"; then
  ok "sin credencial: lo dice"
else
  mal "sin credencial salió con código $CODIGO y dijo: $SALIDA"
fi

echo
if [ "$fallas" -eq 0 ]; then
  printf '\033[32m✓ las 5 comprobaciones pasan\033[0m\n'
  exit 0
fi
printf '\033[31m✗ %d comprobación(es) fallaron\033[0m\n' "$fallas"
exit 1
