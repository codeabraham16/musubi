#!/usr/bin/env bash
#
# construir.sh — arma el binario del cerebro con una versión DERIVADA, nunca tipeada.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ EXISTE: DOS SESIONES NUMERANDO DISTINTO EL MISMO CÓDIGO
#
# El 2026-08-30 el redespliegue imprimió `de: 0.115.0-reparto.cc2ae9c` → `a: 0.133.0-rename...`,
# y eso se lee como un salto de dieciocho versiones. No lo era: era el MISMO tronco. Una sesión
# leía el archivo VERSION (0.115.0) y la otra —la mía— venía inventando números a mano, seis
# despliegues seguidos.
#
# ESO NO ES COSMÉTICO. El `de:`/`a:` del script de redespliegue es el ÚNICO registro de qué
# estuvo corriendo, y hoy fue exactamente lo que permitió descubrir que dos sesiones se estaban
# pisando los binarios. Con números que no son comparables entre sí, ese aviso pasa a mentir: un
# rollback real se ve igual que dos esquemas de numeración conviviendo.
#
# La versión se DERIVA de dos cosas que no se pueden discutir: el archivo VERSION y el commit.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# EL SUFIJO `-sucio` NO ES DECORACIÓN
#
# Un binario construido con cambios sin commitear no se puede reconstruir después: el commit que
# anuncia NO es el código que corre. Hoy la etiqueta ajena se pudo auditar justamente porque su
# commit existía y se podía mirar. Un build sucio rompe eso, así que lo dice en su propio nombre —
# no se prohíbe (a veces hace falta probar algo rápido), se DECLARA.
#
# Uso:   ./deploy/construir.sh [etiqueta-del-track] [ruta-de-salida]
# Ej:    ./deploy/construir.sh rename /tmp/musubi-rename
set -euo pipefail

cd "$(dirname "$0")/.."

[[ -f VERSION ]] || { echo "no encuentro el archivo VERSION en la raíz del repo" >&2; exit 1; }
BASE="$(tr -d '[:space:]' < VERSION)"
COMMIT="$(git rev-parse --short HEAD)"
SUCIO=""
git diff --quiet && git diff --cached --quiet || SUCIO="-sucio"

ETIQUETA="${1:-}"
SALIDA="${2:-./musubi}"
VERSION="${BASE}${ETIQUETA:+-$ETIQUETA}.${COMMIT}${SUCIO}"

echo "▶ versión: $VERSION"
[[ -n "$SUCIO" ]] && echo "! el árbol tiene cambios SIN COMMITEAR: este binario no se va a poder reconstruir" >&2

# LA CLAVE PÚBLICA DE RELEASE SE INYECTA ACÁ, y su ausencia es deliberada por default: un binario
# de desarrollo NO puede verificar la procedencia de una actualización, y `musubi update` se niega
# a instalar sin poder verificarla. Sólo el build que se publica la lleva.
#
#   MUSUBI_RELEASE_PUBKEY=<64 hex> ./deploy/construir.sh flota /tmp/musubi
#
# El par se genera UNA vez con `go run ./deploy/cmd/clave-release`, y la privada vive fuera de
# línea: si estuviera en el CI, quien comprometa el CI firma lo que quiera y la firma no compra
# nada — sería un sha256 más caro.
LDFLAGS="-X main.version=$VERSION"
if [[ -n "${MUSUBI_RELEASE_PUBKEY:-}" ]]; then
  LDFLAGS="$LDFLAGS -X main.clavePublicaDeReleaseHex=$MUSUBI_RELEASE_PUBKEY"
  echo "▶ con clave pública de release embebida"
else
  echo "! sin clave pública de release: este binario NO va a poder auto-actualizarse (a propósito)" >&2
fi

go build -ldflags "$LDFLAGS" -o "$SALIDA" ./cmd/musubi
echo "✓ $SALIDA"

# ─────────────────────────────────────────────────────────────────────────────────────────────
# EL BINARIO SE INTERROGA SÓLO SI ESTA MÁQUINA PUEDE EJECUTARLO
#
# `"$SALIDA" version` era incondicional, así que una compilación cruzada
# —`GOOS=windows ./deploy/construir.sh flota musubi-nuevo.exe`, que es como se arma el agente de
# Windows desde acá— moría en «cannot execute binary file» con `set -e` DESPUÉS de haber
# escrito el binario perfectamente. El guion terminaba en rojo sobre un trabajo que salió bien, y
# ni siquiera llegaba a imprimir el sha256, que es lo único que el otro lado necesita para
# comprobar que le llegó lo que se compiló.
#
# Un guion que reporta fracaso cuando tuvo éxito es peor que uno que no reporta nada: enseña a
# ignorar su código de salida, y ése es el que decide si un despliegue sigue.
DESTINO_OS="${GOOS:-$(go env GOHOSTOS)}"
DESTINO_ARCH="${GOARCH:-$(go env GOHOSTARCH)}"
if [[ "$DESTINO_OS" == "$(go env GOHOSTOS)" && "$DESTINO_ARCH" == "$(go env GOHOSTARCH)" ]]; then
  "$SALIDA" version
else
  echo "▶ compilación cruzada para $DESTINO_OS/$DESTINO_ARCH: no se puede correr acá, así que la"
  echo "  versión no se comprueba. Se llama $VERSION y hay que verificarla EN LA MÁQUINA DESTINO."
fi
sha256sum "$SALIDA"
