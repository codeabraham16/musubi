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

# El modo de la clave se COMPRUEBA, no se asume: una clave de firma legible por todo el mundo es
# una clave que ya no vale. Es barato y es exactamente el descuido que se comete con prisa.
MODO=$(stat -c %a "$CLAVE")
case "$MODO" in 400|600) ;; *) echo "la clave privada tiene modo $MODO: ponela en 600 antes de firmar" >&2; exit 2;; esac

echo "▶ armando el manifiesto de $VERSION"
python3 - "$VERSION" "$DIR" <<'PY' > "$DIR/manifest.json"
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
ES_ASSET = re.compile(r"^musubi(-[a-z0-9]+)+(\.exe)?$")

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
    if not ES_ASSET.match(n):
        continue
    p = os.path.join(d, n)
    if not os.path.isfile(p):
        continue
    with open(p, "rb") as f:
        assets[n] = hashlib.sha256(f.read()).hexdigest()
if not assets:
    sys.exit("no hay ningún asset que firmar en %s (se esperan archivos `musubi-<plataforma>-<arch>`)" % d)
# separators sin espacios: la MISMA forma canónica que produce json.Marshal en Go, que es contra
# la que se verifica. Un espacio de más acá es una firma que no valida allá.
sys.stdout.write(json.dumps({"version": version, "assets": assets}, sort_keys=True, separators=(",", ":")))
PY

echo "▶ firmando"
python3 - "$CLAVE" "$DIR/manifest.json" <<'PY' > "$DIR/manifest.json.sig"
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
