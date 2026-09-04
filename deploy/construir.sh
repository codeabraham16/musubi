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
"$SALIDA" version
sha256sum "$SALIDA"
