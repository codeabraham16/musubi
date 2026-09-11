#!/usr/bin/env bash
# firmar-release.sh — arma el manifiesto de un release y lo firma con la clave ed25519 offline.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# LA CLAVE PRIVADA NO VIVE ACÁ, NI EN EL CI, NI EN EL REPO
#
# Ése es el punto entero de firmar. Si la clave estuviera en el pipeline, quien comprometa el
# pipeline firma lo que quiera y la firma no compra nada — sería un sha256 más caro.
#
# Vive fuera de línea (una llave USB, un gestor de secretos con aprobación humana) y este guion se
# corre A MANO, en la máquina de quien publica, con la clave montada el rato que dura.
#
#   ./deploy/firmar-release.sh <version> <clave-privada> <dir-con-los-binarios>
#
# Genera, al lado de los binarios:
#   manifest.json      versión + sha256 de cada asset
#   manifest.json.sig  la firma, en hex
#
# CÓMO SE CREA EL PAR (una sola vez, y la privada NO se copia a ningún servidor):
#   go run ./deploy/cmd/clave-release            # imprime privada y pública
# La pública se inyecta al compilar:
#   go build -ldflags "-X main.clavePublicaDeReleaseHex=<64 hex>"
# ════════════════════════════════════════════════════════════════════════════════════════════
set -euo pipefail

VERSION="${1:-}"; CLAVE="${2:-}"; DIR="${3:-}"
[ -n "$VERSION" ] && [ -n "$CLAVE" ] && [ -n "$DIR" ] || {
  echo "uso: $0 <version> <archivo-de-clave-privada> <directorio-con-binarios>" >&2; exit 2; }
[ -f "$CLAVE" ] || { echo "no encuentro la clave privada en $CLAVE" >&2; exit 2; }
[ -d "$DIR" ]   || { echo "no encuentro el directorio $DIR" >&2; exit 2; }

# El permiso de la clave se COMPRUEBA, no se asume: una clave de firma legible por todo el mundo es
# una clave que ya no vale. Es barato y es exactamente el descuido que se comete con prisa.
#
# PERO LA PREGUNTA NO SE HACE IGUAL EN LOS DOS SISTEMAS, y hasta el 2026-09-10 se hacía con un modo
# POSIX en los dos. Medido en Windows, sobre NTFS: git-bash sólo mapea el bit de sólo-lectura, así
# que `chmod 600` deja **644** y `chmod 400` deja **444**. Un `case` de `400|600` ABORTA SIEMPRE —
# sobre una clave que puede estar perfectamente protegida por su ACL, como estaba la de la prueba
# (sólo SYSTEM, Administrators y el dueño). Una guarda que no puede pasar nunca no protege nada:
# se saltea a mano, con prisa y con la clave montada, que es exactamente cuando se pierde.
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    # El equivalente en Windows es si algún principal AMPLIO tiene entrada en la ACL. Se traduce a
    # SID antes de comparar porque los nombres de los grupos están TRADUCIDOS: en un Windows en
    # español `Everyone` es `Todos` y `Users` es `Usuarios`, así que un grep por el nombre inglés
    # daría verde sobre una clave abierta de par en par.
    RUTA_WIN=$(cygpath -w "$CLAVE")
    # S-1-1-0 Todos · S-1-5-32-545 Usuarios · S-1-5-11 Usuarios autenticados · S-1-5-4 Interactivo.
    # Un SID que no se pudo traducir cuenta como amplio: falla en cerrado, no lo pasa por alto.
    AMPLIOS=$(powershell -NoProfile -NonInteractive -Command \
      "(Get-Acl -LiteralPath '$RUTA_WIN').Access | ForEach-Object { try { \$_.IdentityReference.Translate([System.Security.Principal.SecurityIdentifier]).Value } catch { 'SIN-TRADUCIR' } }" \
      2>/dev/null | tr -d '\r' | grep -E '^(S-1-1-0|S-1-5-32-545|S-1-5-11|S-1-5-4|SIN-TRADUCIR)$' || true)
    [ -z "$AMPLIOS" ] || {
      echo "la clave privada es alcanzable por $(echo "$AMPLIOS" | tr '\n' ' ')(Todos / Usuarios / Usuarios autenticados)." >&2
      echo "Quitales el acceso antes de firmar:  icacls \"$RUTA_WIN\" /inheritance:r /grant:r \"%USERNAME%:R\"" >&2
      exit 2; }
    ;;
  *)
    # `stat -c` es de GNU. En macOS/BSD la misma pregunta se hace con `-f %Lp`, y el `-c` devuelve
    # `stat: illegal option -- c`: medido en el runner de macOS el 2026-09-10, o sea que este
    # chequeo NUNCA corrió fuera de Linux. Y fallaba de la peor manera — bajo `set -euo pipefail`
    # la asignación mataba el guion mostrando la ayuda de `stat`, sin una línea que dijera qué
    # estaba intentando hacer. Tercera guarda del mismo archivo que no podía informar.
    MODO=$(stat -c %a "$CLAVE" 2>/dev/null || stat -f %Lp "$CLAVE" 2>/dev/null || true)
    # No poder leer el modo NO es «seguí»: se aborta diciéndolo. Firmar sin saber cómo está
    # protegida la clave es justamente lo que esta guarda existe para impedir.
    [ -n "$MODO" ] || {
      echo "no pude leer el modo de $CLAVE: ni \`stat -c\` (GNU) ni \`stat -f\` (BSD) contestaron." >&2
      echo "No firmo sin saber cómo está protegida la clave." >&2
      exit 2; }
    case "$MODO" in 400|600) ;; *) echo "la clave privada tiene modo $MODO: ponela en 600 antes de firmar" >&2; exit 2;; esac
    ;;
esac

# EL INTÉRPRETE SE ELIGE PROBÁNDOLO, NO PREGUNTANDO SI EXISTE. En Windows, `python3` existe como
# un alias de ejecución de la Microsoft Store: `command -v python3` lo encuentra y `command -v`
# devuelve 0, pero al ejecutarlo imprime «no se encontró Python» y no corre nada. Un guion que se
# conforma con que el archivo exista muere ahí, en la máquina de quien publica, con la clave
# privada montada. Por eso se le pide que ejecute algo antes de creerle.
PY_BIN=""
for c in python3 python; do
  if command -v "$c" >/dev/null 2>&1 && "$c" -c "import sys" >/dev/null 2>&1; then PY_BIN="$c"; break; fi
done
[ -n "$PY_BIN" ] || { echo "no encuentro un python que ejecute (probé python3 y python)" >&2; exit 2; }

echo "▶ armando el manifiesto de $VERSION"
"$PY_BIN" - "$VERSION" "$DIR" <<'PY' > "$DIR/manifest.json"
import hashlib, json, os, re, sys
version, d = sys.argv[1], sys.argv[2]

# ════════════════════════════════════════════════════════════════════════════════════════
# SE FIRMA UNA LISTA BLANCA, NO «TODO LO QUE HAYA EN EL DIRECTORIO»
#
# La primera versión hasheaba todos los archivos y saltaba sólo los del propio manifiesto.
# Se probó de punta a punta con la clave privada apoyada en el mismo directorio, y el
# manifiesto salió con `priv.key` adentro: el hash de la clave de firma, publicado en un
# archivo que va al release.
#
# El daño directo es acotado (es un hash, no la clave) y el modo de falla no: un guion que
# firma «lo que haya» publica lo que alguien dejó ahí sin querer — un .env, un backup, un
# binario a medio compilar de otra cosa. Y todo eso queda firmado por nosotros.
#
# Así que se nombra lo que es un asset, y lo demás no entra ni por descuido.
# ════════════════════════════════════════════════════════════════════════════════════════
# SE NOMBRAN UNO POR UNO, y no con un patrón. Acá había un `^musubi(-[a-z0-9]+)+(\.exe)?$` que
# parecía cubrirlos a todos y dejaba afuera a los DOS de Windows: `release.yml` los publica como
# `Musubi.exe` y `Musubi-arm64.exe` —con mayúscula, y el primero sin ningún guion—, así que el
# patrón no casaba con ninguno. El manifiesto salía sin ellos, `ShaDeAsset` no los encontraba, y
# `musubi update` en Windows se negaba a instalar un release perfectamente firmado. Nadie lo vio
# porque el guion no falla: firma lo que casa y calla lo que no.
#
# La lista está pineada contra `selfupdate.AssetName` por TestElFirmadorCubreLosAssetsReales: si
# alguien agrega una plataforma y no la agrega acá, el test lo dice antes que el usuario.
ASSETS = {
    "Musubi.exe", "Musubi-arm64.exe",
    "musubi-linux-amd64", "musubi-linux-arm64",
    "musubi-darwin-amd64", "musubi-darwin-arm64",
}

# Y ADEMÁS: si hay algo con pinta de secreto en el directorio, se ABORTA en vez de saltearlo.
# Saltearlo en silencio dejaría a alguien firmando con la clave al lado sin enterarse nunca de
# que la tuvo ahí; abortar lo obliga a moverla, que es lo que hay que hacer.
SOSPECHOSO = re.compile(r"(key|priv|secret|token|\.pem$|\.env)", re.I)
malos = [n for n in sorted(os.listdir(d)) if SOSPECHOSO.search(n)]
if malos:
    sys.exit("ABORTADO: el directorio a firmar contiene %s. Sacá los secretos de ahí antes de "
             "firmar: este guion publica un manifiesto y lo que se firma se publica." % ", ".join(malos))

assets = {}
for n in sorted(os.listdir(d)):
    if n not in ASSETS:
        continue
    p = os.path.join(d, n)
    if not os.path.isfile(p):
        continue
    with open(p, "rb") as f:
        assets[n] = hashlib.sha256(f.read()).hexdigest()
if not assets:
    sys.exit("no hay ningún asset que firmar en %s (se esperan los assets que publica release.yml)" % d)
# separators sin espacios: la MISMA forma canónica que produce json.Marshal en Go, que es contra
# la que se verifica. Un espacio de más acá es una firma que no valida allá.
sys.stdout.write(json.dumps({"version": version, "assets": assets}, sort_keys=True, separators=(",", ":")))
PY

echo "▶ firmando"
"$PY_BIN" - "$CLAVE" "$DIR/manifest.json" <<'PY' > "$DIR/manifest.json.sig"
import sys
try:
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    from cryptography.hazmat.primitives import serialization
except ImportError:
    sys.exit("falta el paquete `cryptography` (pip install cryptography), o firmá con `go run ./deploy/cmd/firmar`")
priv_hex = open(sys.argv[1]).read().strip()
k = Ed25519PrivateKey.from_private_bytes(bytes.fromhex(priv_hex))
sys.stdout.write(k.sign(open(sys.argv[2], "rb").read()).hex())
PY

echo "✓ $DIR/manifest.json"
echo "✓ $DIR/manifest.json.sig"
echo
echo "Subí LOS DOS al release, junto con los binarios. Sin el manifiesto firmado,"
echo "\`musubi update\` se niega a instalar: un release que no podemos probar que es nuestro no se instala."
