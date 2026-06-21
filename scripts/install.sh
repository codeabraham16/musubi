#!/usr/bin/env bash
# Instalador interactivo de Musubi para Linux/macOS.
#
# Interactivo (pregunta local/global):
#   curl -fsSL https://raw.githubusercontent.com/codeabraham16/musubi/main/scripts/install.sh | bash
# No interactivo:
#   curl -fsSL .../install.sh | MUSUBI_SCOPE=global bash
#   ./install.sh --scope local --dir /ruta/al/repo
#
# Variables / flags reconocidos:
#   MUSUBI_SCOPE / --scope   = local | global   (si falta y hay TTY se pregunta; sin TTY -> global)
#   MUSUBI_DIR   / --dir      = carpeta del proyecto (default: carpeta actual)
#   MUSUBI_NOSETUP=1 / --no-setup = no correr 'musubi setup'
#   MUSUBI_BINARY            = ruta a un binario musubi ya descargado (evita la descarga)
set -euo pipefail

REPO="codeabraham16/musubi"
SCOPE="${MUSUBI_SCOPE:-}"
DIR="${MUSUBI_DIR:-$PWD}"
NOSETUP="${MUSUBI_NOSETUP:-}"

# --- Parseo de flags ---
while [ $# -gt 0 ]; do
  case "$1" in
    --scope) SCOPE="$2"; shift 2 ;;
    --dir) DIR="$2"; shift 2 ;;
    --no-setup) NOSETUP=1; shift ;;
    *) echo "Flag desconocido: $1"; exit 1 ;;
  esac
done

# --- Resolver alcance ---
if [ -z "$SCOPE" ]; then
  if [ -t 0 ]; then
    echo ""
    echo "Donde queres instalar Musubi?"
    echo "  [L] Solo este repo (local, NO toca la PC ni el PATH)"
    echo "  [G] Global en tu usuario (~/.local/bin)"
    printf "Eleccion (L/G): "
    read -r resp
    case "$(printf '%s' "$resp" | tr '[:lower:]' '[:upper:]')" in
      G) SCOPE=global ;;
      *) SCOPE=local ;;
    esac
  else
    SCOPE=global
  fi
fi
if [ "$SCOPE" != "local" ] && [ "$SCOPE" != "global" ]; then
  echo "Alcance invalido: '$SCOPE' (usa local|global)"; exit 1
fi

DIR="$(cd "$DIR" && pwd)"

# --- Detectar OS / arquitectura ---
case "$(uname -s)" in
  Linux*)  OS=linux ;;
  Darwin*) OS=darwin ;;
  *) echo "SO no soportado: $(uname -s)"; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "Arquitectura no soportada: $(uname -m)"; exit 1 ;;
esac
ASSET="musubi-${OS}-${ARCH}"

# --- Obtener el binario ---
TMP="$(mktemp)"
if [ -n "${MUSUBI_BINARY:-}" ] && [ -f "${MUSUBI_BINARY:-}" ]; then
  cp "$MUSUBI_BINARY" "$TMP"
  echo "Usando binario provisto: $MUSUBI_BINARY"
else
  URL="https://github.com/$REPO/releases/latest/download/$ASSET"
  echo "Descargando $ASSET ..."
  if curl -fsSL "$URL" -o "$TMP" 2>/dev/null; then
    :
  elif command -v gh >/dev/null 2>&1; then
    echo "Descarga directa fallo (repo privado?). Probando con gh CLI..."
    gh release download --repo "$REPO" --pattern "$ASSET" --output "$TMP" --clobber
  else
    echo "No se pudo descargar $ASSET. Instala gh CLI y autenticate, o usa MUSUBI_BINARY."; exit 1
  fi
fi

# --- Instalar segun alcance ---
if [ "$SCOPE" = "global" ]; then
  INSTALL_DIR="${MUSUBI_INSTALL_DIR:-$HOME/.local/bin}"
  mkdir -p "$INSTALL_DIR"
  EXE="$INSTALL_DIR/musubi"
  cp "$TMP" "$EXE"; chmod +x "$EXE"
  echo "Musubi (GLOBAL) instalado en $EXE"
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo "Agrega $INSTALL_DIR a tu PATH." ;;
  esac
  # Persistir MUSUBI_BIN: hace que el .mcp.json portable resuelva el binario aunque
  # cambie la ruta o el usuario. Al reinstalar se re-evalua y todos los proyectos siguen.
  PROFILE="${HOME}/.profile"
  if [ ! -f "$PROFILE" ] || ! grep -q 'MUSUBI_BIN=' "$PROFILE"; then
    printf 'export MUSUBI_BIN="%s"\n' "$EXE" >> "$PROFILE"
    echo "MUSUBI_BIN agregado a $PROFILE (abri una shell nueva para que tome efecto)."
  fi
else
  BIN_DIR="$DIR/.musubi/bin"
  mkdir -p "$BIN_DIR"
  EXE="$BIN_DIR/musubi"
  cp "$TMP" "$EXE"; chmod +x "$EXE"
  GI="$DIR/.gitignore"
  if [ ! -f "$GI" ] || ! grep -qF '.musubi/bin/' "$GI"; then
    printf '%s\n' '.musubi/bin/' >> "$GI"
  fi
  echo "Musubi (LOCAL) instalado en $EXE (no se toco el PATH ni la PC)."
fi
rm -f "$TMP"

# --- Setup del proyecto ---
if [ "$NOSETUP" != "1" ]; then
  echo "Preparando el proyecto en $DIR ..."
  ( cd "$DIR" && "$EXE" setup )
fi

echo ""
if [ "$SCOPE" = "local" ]; then
  echo "Listo (LOCAL): Musubi vive en '$DIR/.musubi/' y no dejo nada en tu PC."
  echo "Para desinstalar: borra la carpeta .musubi/ del repo."
else
  echo "Listo (GLOBAL): usa 'musubi setup' en cualquier otro repo para sumarlo."
fi
echo "Reabri el proyecto en Claude Code y el server 'musubi' cargara solo."
