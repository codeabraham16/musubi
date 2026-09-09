#!/usr/bin/env bash
#
# sufijo-sucio.sh — comprueba que `construir.sh` MARQUE un binario que se lleva código sin commitear.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# POR QUÉ EXISTE
#
# La cabecera de `construir.sh` dice que un build sucio «no se prohíbe, se DECLARA», porque un
# binario con cambios sin commitear no se puede reconstruir después: el commit que anuncia NO es el
# código que corre. Ese contrato se apoyaba en `git diff --quiet && git diff --cached --quiet`, y
# **`git diff` no ve los archivos SIN TRACKEAR**. `go build` compila todo el `.go` del directorio,
# rastreado o no.
#
# Así que el caso PEOR —código que se despliega y no está en NINGÚN commit, el único que no se
# puede auditar después— era exactamente el que el guion daba por limpio. Medido el 2026-09-05:
#
#   git status --short           →  ?? cmd/musubi/zz_chivato.go
#   el chequeo viejo             →  LIMPIO
#   versión estampada            →  0.131.0-flota.ac75fec, SIN -sucio
#   sha256                       →  CAMBIÓ: el archivo sin trackear viajó adentro
#   vcs.modified que embute Go   →  true
#
# Y en ese momento la frase «sin sufijo -sucio, o sea que no se lleva trabajo de nadie» ya estaba
# escrita en un registro y en un mensaje a gio, sobre un binario que se iba a desplegar.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# LO QUE ESTE ARNÉS EXIGE, Y POR QUÉ ES ASÍ Y NO UN GREP
#
#   · SE MIDE LA CONDUCTA, NO EL TEXTO. Un grep de `--porcelain` lo satisfacería un comentario —el
#     defecto dominante de este repo— y además prohibiría cualquier otra forma correcta de
#     resolverlo (hoy el sufijo se CORRIGE contra `vcs.modified`, que no menciona «untracked»).
#   · LAS DOS DIRECCIONES. Un árbol limpio NO puede salir marcado: sin ese caso, «marcar siempre»
#     pasaría la prueba y el sufijo dejaría de significar algo. Ojo con lo que cubre cada caso, que
#     lo descubrí saboteando: como el sufijo se CORRIGE contra `vcs.modified`, una conjetura rota en
#     cualquiera de las dos direcciones queda curada sola — así que el caso 1 sólo se pone rojo si
#     además el relink correctivo está roto. Eso no es un hueco del arnés, es defensa en dos capas;
#     pero conviene saber que el caso 2 es el que muerde primero.
#   · SE COMPRUEBA QUE EL ARCHIVO VIAJÓ, comparando los dos sha256. Sin eso el arnés mediría la
#     etiqueta y no el peligro: lo que importa es que el binario CAMBIÓ, y la etiqueta existe para
#     avisarlo.
#   · EN UN CLON, NO EN UN WORKTREE. En un worktree despegado Go no estampa `vcs` en absoluto —el
#     módulo queda en `(devel)`—, así que ahí el sello no dice «limpio»: no dice NADA, y una prueba
#     que corriera ahí mediría la ausencia y la leería como respuesta.
#   · EL `construir.sh` QUE SE PRUEBA ES EL DEL ÁRBOL DE TRABAJO, commiteado dentro del clon. Si se
#     probara el de HEAD, un arreglo sin commitear se verificaría contra la versión vieja: verde
#     sobre código que no es el que se está por commitear.
#
# Uso:  ./deploy/pruebas/sufijo-sucio.sh [raíz-del-repo]
set -uo pipefail

RAIZ="$(cd "${1:-$(dirname "${BASH_SOURCE[0]}")/../..}" && pwd)"
[[ -f "$RAIZ/VERSION" && -x "$RAIZ/deploy/construir.sh" ]] || { echo "✗ $RAIZ no parece la raíz del repo"; exit 2; }
command -v go >/dev/null || { echo "✗ sin go en el PATH"; exit 2; }

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
CLON="$TMP/clon"

SHA="$(git -C "$RAIZ" rev-parse HEAD)"
git clone --quiet --local --no-hardlinks "$RAIZ" "$CLON" || { echo "✗ no se pudo clonar"; exit 2; }
git -C "$CLON" checkout --quiet "$SHA" || { echo "✗ no se pudo posicionar en $SHA"; exit 2; }

# El guion QUE SE PRUEBA es el del árbol de trabajo. Se commitea adentro del clon para que el árbol
# quede LIMPIO: si quedara como cambio sin commitear, el caso «limpio» arrancaría sucio y el arnés
# mediría otra cosa.
cp -- "$RAIZ/deploy/construir.sh" "$CLON/deploy/construir.sh"
git -C "$CLON" -c user.email=arnes@local -c user.name=arnes commit -q -am "el construir.sh a probar" 2>/dev/null
[[ -z "$(git -C "$CLON" status --porcelain)" ]] || { echo "✗ el clon no quedó limpio: el arnés mediría otra cosa"; git -C "$CLON" status --short; exit 2; }

version_de() { "$1" version 2>/dev/null | awk '{print $2}'; }

# ── 1 · ÁRBOL LIMPIO: no puede salir marcado ─────────────────────────────────────────────────
( cd "$CLON" && ./deploy/construir.sh arnes "$TMP/limpio" ) >"$TMP/log-limpio" 2>&1 || {
  echo "✗ el build del árbol limpio falló:"; sed 's/^/    /' "$TMP/log-limpio"; exit 1; }
V_LIMPIO="$(version_de "$TMP/limpio")"
if [[ "$V_LIMPIO" == *-sucio ]]; then
  echo "✗ un árbol LIMPIO salió marcado como sucio: $V_LIMPIO"
  echo "  Con esto el sufijo deja de significar algo — «marcar siempre» no es una guarda."
  exit 1
fi
echo "  ✓ árbol limpio → $V_LIMPIO (sin marca, como debe)"

# ── 2 · UN `.go` SIN TRACKEAR: tiene que salir marcado, Y tiene que cambiar el binario ────────
printf 'package main\n\n// lo pone deploy/pruebas/sufijo-sucio.sh: codigo que no esta en ningun commit.\nvar chivatoDelArnesDeSufijo = "presente"\n' \
  > "$CLON/cmd/musubi/zz_chivato_del_arnes.go"
[[ -n "$(git -C "$CLON" status --porcelain)" ]] || { echo "✗ git no ve el archivo que acabo de crear"; exit 2; }

( cd "$CLON" && ./deploy/construir.sh arnes "$TMP/sucio" ) >"$TMP/log-sucio" 2>&1 || {
  echo "✗ el build con el archivo sin trackear falló:"; sed 's/^/    /' "$TMP/log-sucio"; exit 1; }
V_SUCIO="$(version_de "$TMP/sucio")"

# EL ARCHIVO TIENE QUE HABER VIAJADO. Si los sha coinciden, `go build` no lo compiló y entonces la
# premisa del arnés es falsa: no habría nada que declarar, y el caso 2 estaría midiendo humo.
S1="$(sha256sum "$TMP/limpio" | cut -d' ' -f1)"
S2="$(sha256sum "$TMP/sucio"  | cut -d' ' -f1)"
if [[ "$S1" == "$S2" ]]; then
  echo "✗ el binario NO cambió con el archivo sin trackear ($S1)."
  echo "  O sea que go build no lo compiló, y este arnés no está midiendo el peligro que dice medir."
  exit 2
fi
echo "  ✓ el archivo sin trackear VIAJÓ: el sha cambió (${S1:0:12}… → ${S2:0:12}…)"

if [[ "$V_SUCIO" != *-sucio ]]; then
  echo "✗ UN BINARIO QUE SE LLEVA CÓDIGO SIN COMMITEAR NO SALIÓ MARCADO: $V_SUCIO"
  echo '  git diff no ve los archivos sin trackear, y go build los compila igual. Ese binario'
  echo "  no se puede reconstruir —el commit que anuncia no es el código que corre— y su versión"
  echo "  dice que sí. Salida del build:"
  sed 's/^/    /' "$TMP/log-sucio"
  exit 1
fi
echo "  ✓ con un .go sin trackear → $V_SUCIO (marcado)"

echo "✓ el sufijo declara el código que no está en ningún commit, y no marca un árbol limpio"
