#!/usr/bin/env bash
# firmar-windows.sh — firma el .exe con Authenticode usando un certificado PROPIO, sin comprar nada.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# POR QUÉ ESTO ALCANZA, Y DÓNDE NO
#
# Un certificado comprado sirve para que una máquina AJENA confíe en el binario sin que nadie le
# diga nada. Dentro de una flota propia eso no hace falta: las máquinas se enrolan a mano y con
# administrador, y en ese mismo acto se les puede instalar el certificado del editor. Es lo que
# hace cualquier empresa con su PKI interna, y cuesta cero.
#
# LO QUE ESTO NO COMPRA, dicho antes y no después:
#
#   · Reputación de SmartScreen. Se gana con el historial del certificado ante Microsoft, y un
#     certificado que Microsoft no conoce arranca —y se queda— en cero.
#
#   · Confianza en una máquina donde nadie instaló este certificado. Ése es exactamente el caso
#     «desplegar en un cliente cuyas máquinas no administro», y ahí no hay sustituto gratis.
#     Cuando llegue ese día lo paga el contrato, no el bolsillo.
#
#   · La excepción de NordVPN de A31, que NO es un problema de firma. Está medido: el MISMO
#     binario sin firmar latió como `musubi.exe` y dio WSAEACCES como `musubi-nuevo.exe`. Si la
#     firma fuera el discriminante, los dos habrían fallado igual — la firma era la misma (ninguna).
#     Es una lista blanca por RUTA. Un certificado, pago o propio, no la mueve.
#
# LO QUE SÍ COMPRA, y es lo que la Ola 3 necesita: una IDENTIDAD DE EDITOR ESTABLE. El sha256 de
# un release cambia en cada versión; el editor no. Con eso una regla de AppLocker/WDAC, o una
# excepción de Defender, se escriben UNA vez y sobreviven a los autoupdates por anillos. Sin firma
# hay que reautorizar hash por hash en cada release, en cada máquina.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# ORDEN: PRIMERO ESTE GUION, DESPUÉS `firmar-release.sh`
#
# Firmar cambia los bytes del .exe. Si el manifiesto ed25519 se arma antes, su sha256 deja de
# corresponder al archivo publicado y `musubi update` falla con «hash mismatch», que no dice nada
# sobre la causa. Este guion ABORTA si ve un manifiesto ya armado al lado.
#
# Uso:
#   ./deploy/firmar-windows.sh --crear-certificado <dir-fuera-del-repo>   # una sola vez
#   ./deploy/firmar-windows.sh <exe> <cert.crt> <clave.key>
#
# Sellado de tiempo (opcional, exige red): MUSUBI_TSA=http://timestamp.digicert.com
# Sin sello, las firmas dejan de valer cuando venza el certificado (10 años). Con sello, siguen
# valiendo — pero contactar la TSA desde la máquina donde está la clave rompe el «fuera de línea».
# Queda en OPT-IN a propósito: la disciplina de la clave pesa más que un vencimiento a diez años.
# ════════════════════════════════════════════════════════════════════════════════════════════
set -euo pipefail

cd "$(dirname "$0")/.."
REPO="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

crear_certificado() {
  local dir="${1:-}"
  [ -n "$dir" ] || { echo "uso: $0 --crear-certificado <directorio-fuera-del-repo>" >&2; exit 2; }
  mkdir -p "$dir"
  local abs; abs="$(cd "$dir" && pwd -P)"

  # La clave de firma NO vive en el repo, por la misma razón que la ed25519: lo que está en el
  # árbol se commitea solo tarde o temprano, y una clave de firma publicada es una clave muerta.
  case "$abs/" in
    "$REPO"/*) echo "ABORTADO: $abs está DENTRO del repo. La clave de firma vive fuera (un USB, un" >&2
               echo "          gestor de secretos). Lo que está en el árbol termina commiteado." >&2
               exit 2;;
  esac

  [ -e "$abs/musubi-editor.key" ] && { echo "ya existe $abs/musubi-editor.key — no lo piso" >&2; exit 2; }

  # ══════════════════════════════════════════════════════════════════════════════════════════
  # LAS TRES EXTENSIONES NO SON CEREMONIA: SON LO QUE HACE SEGURO INSTALARLO EN «ENTIDADES RAÍZ»
  #
  # Este certificado se va a instalar en el almacén raíz de cada máquina de la flota. Un
  # certificado raíz puede avalar TODO lo que la máquina verifique — incluido el TLS de cualquier
  # sitio— si no está acotado. Por eso:
  #
  #   CA:FALSE       no puede EMITIR otros certificados: sólo se avala a sí mismo.
  #   codeSigning    su único uso es firmar código. No sirve para hacerse pasar por un servidor.
  #
  # Las dos van marcadas `critical`: un verificador que no entienda la restricción tiene que
  # RECHAZAR el certificado, no ignorarla. `confiar-editor-windows.ps1` las vuelve a comprobar
  # antes de importar, porque el que instala no tiene por qué confiar en el que generó.
  # ══════════════════════════════════════════════════════════════════════════════════════════
  openssl req -x509 -newkey rsa:3072 -sha256 -days 3650 -nodes \
    -keyout "$abs/musubi-editor.key" -out "$abs/musubi-editor.crt" \
    -subj "/CN=Musubi/O=Musubi/OU=Agente de flota" \
    -addext "basicConstraints=critical,CA:FALSE" \
    -addext "keyUsage=critical,digitalSignature" \
    -addext "extendedKeyUsage=critical,codeSigning" >/dev/null 2>&1

  chmod 600 "$abs/musubi-editor.key"
  echo "✓ $abs/musubi-editor.key   (privada, modo 600 — NO se copia a ningún servidor)"
  echo "✓ $abs/musubi-editor.crt   (pública — ésta sí va a cada máquina de la flota)"
  echo
  echo "Huella que hay que comparar en la máquina antes de confiar en él:"
  # Sin la etiqueta ni los dos puntos: exactamente la forma que compara confiar-editor-windows.ps1.
  openssl x509 -in "$abs/musubi-editor.crt" -noout -fingerprint -sha256 | cut -d= -f2 | tr -d ':'
  exit 0
}

[ "${1:-}" = "--crear-certificado" ] && { shift; crear_certificado "${1:-}"; }

EXE="${1:-}"; CERT="${2:-}"; CLAVE="${3:-}"
[ -n "$EXE" ] && [ -n "$CERT" ] && [ -n "$CLAVE" ] || {
  echo "uso: $0 <exe> <cert.crt> <clave.key>" >&2
  echo "     $0 --crear-certificado <directorio-fuera-del-repo>" >&2; exit 2; }
[ -f "$EXE" ]   || { echo "no encuentro el binario en $EXE" >&2; exit 2; }
[ -f "$CERT" ]  || { echo "no encuentro el certificado en $CERT" >&2; exit 2; }
[ -f "$CLAVE" ] || { echo "no encuentro la clave privada en $CLAVE" >&2; exit 2; }

# Mismo control que en firmar-release.sh: una clave de firma legible por todos ya no vale nada.
MODO=$(stat -c %a "$CLAVE")
case "$MODO" in 400|600) ;; *) echo "la clave privada tiene modo $MODO: ponela en 600 antes de firmar" >&2; exit 2;; esac

# Que el archivo sea realmente un PE. Firmar el binario de Linux por confundir un argumento es
# un error de dos segundos que se descubriría recién en la máquina Windows.
[ "$(head -c2 "$EXE")" = "MZ" ] || { echo "$EXE no es un ejecutable de Windows (no empieza con MZ)" >&2; exit 2; }

# EL ORDEN, COMPROBADO EN VEZ DE RECORDADO. Ver el encabezado: firmar después de armar el
# manifiesto lo invalida en silencio.
DIR="$(cd "$(dirname "$EXE")" && pwd -P)"
[ -e "$DIR/manifest.json" ] && {
  echo "ABORTADO: ya hay un manifest.json en $DIR." >&2
  echo "          Authenticode cambia los bytes del .exe: el sha256 del manifiesto dejaría de" >&2
  echo "          corresponder y \`musubi update\` fallaría con «hash mismatch». Firmá el .exe" >&2
  echo "          PRIMERO, y recién después corré firmar-release.sh." >&2
  exit 2; }

command -v osslsigncode >/dev/null || {
  echo "falta osslsigncode (firma Authenticode desde Linux, gratis):" >&2
  echo "    sudo apt install osslsigncode" >&2
  echo >&2
  echo "Alternativa sin instalar nada, desde cualquier Windows con la clave montada:" >&2
  echo "    Set-AuthenticodeSignature -FilePath musubi.exe -Certificate \$cert -HashAlgorithm SHA256" >&2
  exit 2; }

TS=()
[ -n "${MUSUBI_TSA:-}" ] && TS=(-ts "$MUSUBI_TSA")

echo "▶ firmando $EXE"
TMP="$(mktemp "${EXE}.firmado.XXXXXX")"
trap 'rm -f "$TMP"' EXIT
if ! SALIDA_FIRMA=$(osslsigncode sign -certs "$CERT" -key "$CLAVE" -h sha256 \
      -n "Musubi - agente de flota" "${TS[@]}" \
      -in "$EXE" -out "$TMP" 2>&1); then
  echo "$SALIDA_FIRMA" >&2
  echo "osslsigncode no pudo firmar — el binario original queda intacto" >&2
  exit 1
fi

# Verificar con el MISMO certificado como raíz: comprueba que la firma cierra contra QUIEN
# creemos que firmó, no sólo que hay una firma. Sin -CAfile esto pasaría con cualquier cadena.
#
# LA SALIDA SE IMPRIME SI FALLA, y no se traga. Un certificado autofirmado de entidad final como
# ancla de confianza es un caso de borde de OpenSSL, y «no verifica» a secas no distingue entre
# «la firma está mal» —que es grave— y «no me gusta el ancla» —que es una política de cadena.
# Con el motivo a la vista, la diferencia se ve en un segundo en vez de en una tarde.
if ! SALIDA_VERIF=$(osslsigncode verify -in "$TMP" -CAfile "$CERT" 2>&1); then
  echo "$SALIDA_VERIF" >&2
  echo "la firma no verifica contra $CERT — no piso el binario" >&2
  exit 1
fi

mv "$TMP" "$EXE"
trap - EXIT
chmod 755 "$EXE"
echo "✓ $EXE firmado y verificado"
sha256sum "$EXE"
echo
echo "Ahora sí: ./deploy/firmar-release.sh <version> <clave-ed25519> $DIR"
echo "Y en cada máquina, UNA vez: .\\confiar-editor-windows.ps1 -Certificado musubi-editor.crt"
