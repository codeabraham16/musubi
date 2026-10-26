#!/usr/bin/env bash
# firmar-y-subir.sh — firma el manifiesto ed25519 de un release de Musubi y, SÓLO con tu visto
# bueno explícito, lo sube. Generaliza el one-off ~/firmar-y-subir-0143.sh a una herramienta
# versionada: saca el dolor de tipear el viaje, sin mover una sola decisión de seguridad.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# LA CLAVE LA PONÉS VOS, Y NO SE GUARDA NI VIAJA
#
# La privada de firma vive OFFLINE (Vaultwarden → musubi-infra → «Musubi — clave de firma de
# releases»), como un PEM PKCS#8 CIFRADO. Este guion la recibe como argumento, o sola de una ruta
# por defecto / la env var MUSUBI_RELEASE_KEY; te pide la contraseña por teclado (getpass, nunca por
# línea de comandos), la descifra a un hex temporal en modo 600 que SHREDEA al salir, y nunca
# escribe la clave descifrada en disco permanente, ni al CI, ni al repo.
#
# DÓNDE GUARDAR LA .pem: en disco está BIEN tenerla PORQUE está cifrada — la contraseña (que NO se
# guarda: la tipeás cada vez) es el secreto que la abre. Guardala FUERA del repo, en modo 600; la
# ruta por defecto es ~/.config/musubi/clave-firma-release.pem (o apuntá MUSUBI_RELEASE_KEY). Más
# fuerte aún: en un USB que montás sólo para firmar. Lo que NUNCA va a disco permanente ni a la
# línea de comandos: la contraseña, y la clave ya descifrada (el hex).
# Ése es el punto de firmar: si la clave estuviera en un pipeline automático, quien lo comprometa
# firma lo que quiera y la firma no compra nada. Por eso este paso NO se automatiza del todo: lo
# corrés vos, con la clave montada el rato que dura.
#
# ANTES DE FIRMAR verifica que la pública derivada de tu clave sea la que está incrustada en los
# binarios publicados (release.yml inyecta -X main.clavePublicaDeReleaseHex). Si no coincide, es la
# clave equivocada y `musubi update` rechazaría la firma en cada máquina — así que ABORTA.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# LA SUBIDA ESTÁ GATEADA (regla de oro de la sala de mando)
#
# `gh release upload` es una acción HACIA AFUERA. Sin --subir, el guion firma y PARA: te deja el
# manifiesto listo e imprime el comando. Con --subir, te muestra exactamente qué va a empujar y
# exige que tipees `subir` en la terminal. Una aprobación no se extiende al próximo release.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# AUTHENTICODE ES OPCIONAL
#
# `musubi update` sólo exige la firma ed25519 del manifiesto (cmd/musubi/update.go). El Authenticode
# de los .exe (firmar-windows.sh, necesita osslsigncode) es un extra para AppLocker/WDAC/Defender,
# no un requisito del update. Por defecto se saltea. Con --authenticode <cert.crt> <clave.key> se
# hace ANTES del manifiesto (firmar cambia los bytes del .exe y su sha256), y entonces también se
# re-suben los .exe firmados y sus .sha256.
#
# Uso (la [clave.pem] es opcional si está en la ruta por defecto o en MUSUBI_RELEASE_KEY):
#   ./deploy/firmar-y-subir.sh                             # firma v$(cat VERSION) con la clave por defecto, NO sube
#   ./deploy/firmar-y-subir.sh --subir                     # firma y, con tu OK tipeado, sube
#   ./deploy/firmar-y-subir.sh [clave.pem] v0.145.0        # clave y/o tag explícitos
#   ./deploy/firmar-y-subir.sh [clave.pem] --verificar-clave   # sólo comprueba que es LA clave
#   ./deploy/firmar-y-subir.sh [clave.pem] --pubkey <64hex>    # si rotaste la clave de firma
#   ./deploy/firmar-y-subir.sh [clave.pem] --authenticode <cert.crt> <clave.key>
# ════════════════════════════════════════════════════════════════════════════════════════════
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.." || { echo "no pude entrar a la raíz del repo" >&2; exit 1; }
RAIZ="$(pwd)"
REPO="codeabraham16/musubi"
# Pública de firma incrustada en los binarios publicados (release.yml, ldflag
# -X main.clavePublicaDeReleaseHex). Si algún día rotás la clave, pasá la nueva con --pubkey.
PUB_ESPERADA="c0ced6314433e735395f9efe5c20ffea2fe315a9b1ac02d463d4d1a69fff0ac8"

uso() {
  cat <<'AYUDA'
Uso ([clave.pem] es opcional si está en ~/.config/musubi/clave-firma-release.pem o en MUSUBI_RELEASE_KEY):
  ./deploy/firmar-y-subir.sh                                 # firma v$(cat VERSION) con la clave por defecto, NO sube
  ./deploy/firmar-y-subir.sh --subir                         # firma y, con tu OK tipeado, sube
  ./deploy/firmar-y-subir.sh [clave.pem] v0.145.0            # clave y/o tag explícitos
  ./deploy/firmar-y-subir.sh [clave.pem] --verificar-clave   # sólo comprueba que es LA clave
  ./deploy/firmar-y-subir.sh [clave.pem] --pubkey <64hex>    # si rotaste la clave de firma
  ./deploy/firmar-y-subir.sh [clave.pem] --authenticode <cert.crt> <clave.key>
AYUDA
}

PEM=""; TAG=""; SUBIR=0; SOLO_VERIFICAR=0
AUTHENTICODE=0; CERT=""; CKEY=""
while [ $# -gt 0 ]; do
  case "$1" in
    --subir)          SUBIR=1 ;;
    --verificar-clave) SOLO_VERIFICAR=1 ;;
    --pubkey)         [ $# -ge 2 ] || { echo "--pubkey necesita 64 hex" >&2; exit 2; }; PUB_ESPERADA="$2"; shift ;;
    --authenticode)   [ $# -ge 3 ] || { echo "--authenticode necesita <cert.crt> <clave.key>" >&2; exit 2; }; AUTHENTICODE=1; CERT="$2"; CKEY="$3"; shift 2 ;;
    -h|--help)        uso; exit 0 ;;
    v[0-9]*.[0-9]*.[0-9]*) TAG="$1" ;;
    [0-9]*.[0-9]*.[0-9]*)  TAG="v$1" ;;
    -*)               echo "opción no reconocida: $1" >&2; uso; exit 2 ;;
    *)                if [ -z "$PEM" ]; then PEM="$1"; else echo "argumento de más: $1" >&2; exit 2; fi ;;
  esac
  shift
done

# La clave: explícita por argumento, o por env MUSUBI_RELEASE_KEY, o en la ruta convencional.
# SIEMPRE es la .pem CIFRADA — tenerla en disco está bien porque el cifrado la protege; el secreto
# que la abre es la contraseña, que NO se guarda (la tipeás cada vez).
CLAVE_DEFECTO="${XDG_CONFIG_HOME:-$HOME/.config}/musubi/clave-firma-release.pem"
if [ -z "$PEM" ]; then
  if   [ -n "${MUSUBI_RELEASE_KEY:-}" ]; then PEM="$MUSUBI_RELEASE_KEY"
  elif [ -f "$CLAVE_DEFECTO" ];          then PEM="$CLAVE_DEFECTO"
  fi
fi
[ -n "$PEM" ] || {
  echo "no me pasaste la clave y no la encontré sola." >&2
  echo "Guardá tu .pem CIFRADA fuera del repo (modo 600) y hacé UNA de estas:" >&2
  echo "  · dejala en la ruta por defecto:   $CLAVE_DEFECTO" >&2
  echo "  · o apuntá MUSUBI_RELEASE_KEY a su ruta" >&2
  echo "  · o pasámela como argumento:       ./deploy/firmar-y-subir.sh <clave.pem> ..." >&2
  echo >&2; uso >&2; exit 2; }
[ -f "$PEM" ] || { echo "no encuentro la clave en: $PEM" >&2; exit 2; }
# Higiene: la .pem está cifrada, pero igual conviene 600 (defensa en profundidad). Aviso, no aborto.
MODO_PEM="$(stat -c %a "$PEM" 2>/dev/null || stat -f %Lp "$PEM" 2>/dev/null || echo '?')"
case "$MODO_PEM" in 400|600|'?') ;; *) echo "⚠ $PEM tiene modo $MODO_PEM; conviene:  chmod 600 \"$PEM\"" >&2 ;; esac

if [ -z "$TAG" ]; then
  [ -f VERSION ] || { echo "no hay VERSION en $RAIZ ni pasaste un tag explícito" >&2; exit 2; }
  TAG="v$(tr -d '[:space:]' < VERSION)"
fi
VER="${TAG#v}"

# Un python que realmente EJECUTE y tenga cryptography (en Windows `python3` puede ser el alias
# fantasma de la Store: command -v lo ve, pero no corre nada).
PY=""
for c in python3 python; do
  if command -v "$c" >/dev/null 2>&1 && "$c" -c "import cryptography" >/dev/null 2>&1; then PY="$c"; break; fi
done
[ -n "$PY" ] || { echo "falta un python con el paquete cryptography (pip install cryptography)" >&2; exit 2; }

# En el camino de firma, comprobar que el release existe y tiene los 6 binarios ANTES de pedir la
# contraseña: si no está, no tiene sentido montar la clave.
if [ "$SOLO_VERIFICAR" -eq 0 ]; then
  ASSETS="$(gh release view "$TAG" --repo "$REPO" --json assets --jq '.assets[].name' 2>/dev/null || true)"
  [ -n "$ASSETS" ] || { echo "el release $TAG no existe en $REPO (lo crea release.yml al empujar el tag)" >&2; exit 2; }
  faltan=""
  for a in Musubi.exe Musubi-arm64.exe musubi-linux-amd64 musubi-linux-arm64 musubi-darwin-amd64 musubi-darwin-arm64; do
    grep -qxF "$a" <<<"$ASSETS" || faltan+=" $a"
  done
  [ -z "$faltan" ] || { echo "a $TAG le faltan binarios:$faltan — ¿terminó release.yml?" >&2; exit 2; }
  if grep -qxF manifest.json <<<"$ASSETS" && grep -qxF manifest.json.sig <<<"$ASSETS"; then
    echo "⚠ $TAG YA tiene manifest.json + .sig (ya estaba firmado). Re-firmar lo reemplaza (--clobber)."
  fi
fi

# Archivo temporal para la clave descifrada en hex: modo 600, se SHREDEA al salir pase lo que pase.
HEX="$(mktemp)"; chmod 600 "$HEX"
WORK=""
limpiar() {
  shred -u "$HEX" 2>/dev/null || rm -f "$HEX"
  if [ "${CONSERVAR_WORK:-0}" != "1" ] && [ -n "$WORK" ]; then rm -rf "$WORK"; fi
}
trap limpiar EXIT

echo "== Descifrando tu clave (te va a pedir la CONTRASEÑA) =="
PUB_ESPERADA="$PUB_ESPERADA" "$PY" - "$PEM" <<'PY' > "$HEX"
import os, sys, getpass
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
pem = open(sys.argv[1], "rb").read()
pw = getpass.getpass("Contraseña de la clave: ")
pw = pw.encode() if pw else None
try:
    k = serialization.load_pem_private_key(pem, password=pw)
except Exception as e:
    sys.exit("\nNo pude descifrar/cargar la clave: %s\n(¿contraseña correcta? ¿es la .pem de firma?)" % e)
if not isinstance(k, Ed25519PrivateKey):
    sys.exit("\nLa clave no es ed25519 (es %s): no es la de firma de release." % type(k).__name__)
pub = k.public_key().public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw).hex()
exp = os.environ["PUB_ESPERADA"].strip().lower()
if pub != exp:
    sys.exit("\nEsta clave NO es la de firma esperada:\n  pública derivada: %s\n  pública esperada: %s\n"
             "No firmo con la clave equivocada (el update la rechazaría en cada máquina)." % (pub, exp))
sys.stderr.write("Clave correcta: la pública coincide con la esperada.\n")
raw = k.private_bytes(serialization.Encoding.Raw, serialization.PrivateFormat.Raw, serialization.NoEncryption())
sys.stdout.write(raw.hex())
PY
[ -s "$HEX" ] || { echo "No se generó la clave descifrada (ver el error de arriba)." >&2; exit 1; }

if [ "$SOLO_VERIFICAR" -eq 1 ]; then
  echo "✓ clave verificada: la pública coincide con $PUB_ESPERADA. No firmé ni bajé nada."
  exit 0
fi

# El manifiesto hashea LO QUE HAY en el directorio, y `musubi update` verifica contra lo que BAJA
# del release — así que se firman los binarios EXACTOS del release, no un build local (que saldría
# con otros bytes). Por eso se bajan a un directorio limpio y efímero.
WORK="$(mktemp -d)"
echo
echo "== 1/3  bajando los 6 binarios de $TAG a un dir temporal =="
gh release download "$TAG" --repo "$REPO" --dir "$WORK" \
  --pattern 'Musubi.exe' --pattern 'Musubi-arm64.exe' \
  --pattern 'musubi-linux-amd64' --pattern 'musubi-linux-arm64' \
  --pattern 'musubi-darwin-amd64' --pattern 'musubi-darwin-arm64'
ls -1 "$WORK"

SUBIR_EXTRA=()
if [ "$AUTHENTICODE" -eq 1 ]; then
  command -v osslsigncode >/dev/null 2>&1 || {
    echo "--authenticode pide osslsigncode (sudo apt install osslsigncode)." >&2
    echo "Sin él, el manifiesto ed25519 ya alcanza para 'musubi update'; corré sin --authenticode." >&2
    exit 2; }
  echo
  echo "== 1b/3  Authenticode sobre los .exe (ANTES del manifiesto: cambia sus bytes) =="
  for exe in Musubi.exe Musubi-arm64.exe; do
    [ -f "$WORK/$exe" ] && ./deploy/firmar-windows.sh "$WORK/$exe" "$CERT" "$CKEY"
  done
  # Los .exe cambiaron de bytes → hay que re-subir ellos y su .sha256 actualizado.
  for exe in Musubi.exe Musubi-arm64.exe; do
    if [ -f "$WORK/$exe" ]; then
      ( cd "$WORK" && sha256sum "$exe" > "$exe.sha256" )
      SUBIR_EXTRA+=("$WORK/$exe" "$WORK/$exe.sha256")
    fi
  done
fi

echo
echo "== 2/3  firmando el manifiesto de $VER =="
./deploy/firmar-release.sh "$VER" "$HEX" "$WORK"

FILES=("$WORK/manifest.json" "$WORK/manifest.json.sig")
[ "${#SUBIR_EXTRA[@]}" -gt 0 ] && FILES+=("${SUBIR_EXTRA[@]}")

echo
if [ "$SUBIR" -eq 1 ]; then
  echo "== 3/3  SUBIR a $TAG de $REPO =="
  echo "Voy a subir (con --clobber):"
  for f in "${FILES[@]}"; do echo "   $(basename "$f")"; done
  printf "Escribí 'subir' para confirmar (cualquier otra cosa cancela): "
  RESP=""; read -r RESP < /dev/tty 2>/dev/null || RESP=""
  if [ "$RESP" = "subir" ]; then
    gh release upload "$TAG" --repo "$REPO" --clobber "${FILES[@]}"
    echo
    echo "✓ subido. Verificá:  ./deploy/desplegar-release.sh $TAG   (tiene que decir «firmado»)"
  else
    echo "cancelado: no subí nada."
  fi
else
  CONSERVAR_WORK=1
  echo "== 3/3  FIRMADO — NO subido (falta --subir) =="
  echo "El manifiesto firmado quedó en: $WORK"
  echo "La firma ed25519 es determinista: subir ahora a mano, o re-correr con --subir, da el mismo .sig."
  echo
  echo "Para subir, re-corré con --subir, o a mano:"
  printf '   gh release upload %s --repo %s --clobber' "$TAG" "$REPO"
  for f in "${FILES[@]}"; do printf ' \\\n     %s' "$f"; done
  echo
  echo
  echo "(Cuando termines, borrá el temporal:  rm -rf \"$WORK\")"
fi
