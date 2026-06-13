#!/usr/bin/env bash
# Instalador de Musubi para Linux/macOS.
# Uso (una linea):
#   curl -fsSL https://raw.githubusercontent.com/codeabraham16/musubi/main/scripts/install.sh | bash
set -euo pipefail

REPO="codeabraham16/musubi"

# Detectar OS
case "$(uname -s)" in
  Linux*)  OS=linux ;;
  Darwin*) OS=darwin ;;
  *) echo "SO no soportado: $(uname -s)"; exit 1 ;;
esac

# Detectar arquitectura
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "Arquitectura no soportada: $(uname -m)"; exit 1 ;;
esac

ASSET="musubi-${OS}-${ARCH}"
INSTALL_DIR="${MUSUBI_INSTALL_DIR:-$HOME/.local/bin}"
mkdir -p "$INSTALL_DIR"
DEST="$INSTALL_DIR/musubi"

echo "Descargando $ASSET ..."
URL="https://github.com/$REPO/releases/latest/download/$ASSET"
if curl -fsSL "$URL" -o "$DEST" 2>/dev/null; then
  :
elif command -v gh >/dev/null 2>&1; then
  echo "Descarga directa fallo (repo privado?). Probando con gh CLI..."
  gh release download --repo "$REPO" --pattern "$ASSET" --output "$DEST" --clobber
else
  echo "No se pudo descargar $ASSET. Si el repo es privado, instala gh CLI y autenticate, o haz el repo publico."
  exit 1
fi

chmod +x "$DEST"
echo ""
echo "Musubi instalado en $DEST"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Agrega $INSTALL_DIR a tu PATH." ;;
esac
echo "Ahora, dentro de cualquier proyecto, corre:  musubi setup"
