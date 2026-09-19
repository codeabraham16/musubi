#!/usr/bin/env bash
#
# frescura-de-la-referencia.sh — ejercita `deploy/edad-de-la-referencia.sh` sobre la topología que
# lo rompió: un WORKTREE, donde el `FETCH_HEAD` del árbol y el del directorio común son archivos
# DISTINTOS.
#
# NO SIMULA LA TOPOLOGÍA: la arma. Crea un repo de origen, lo clona, le agrega un worktree y hace
# los fetch de verdad. Una prueba que escribiera los `FETCH_HEAD` a mano estaría fijando lo que YO
# creo que git hace, que es justo lo que este defecto demostró que no se puede suponer.
#
# Uso:  bash deploy/pruebas/frescura-de-la-referencia.sh <raíz-del-repo>
set -euo pipefail

RAIZ="${1:?falta la raíz del repo}"
GUION="$RAIZ/deploy/edad-de-la-referencia.sh"
[ -x "$GUION" ] || { echo "no encuentro $GUION ejecutable"; exit 1; }

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
export GIT_CONFIG_GLOBAL="$TMP/gitconfig"
export GIT_CONFIG_SYSTEM=/dev/null
git config --global user.email vigia@prueba
git config --global user.name  vigia
git config --global init.defaultBranch main

# ── el origen ────────────────────────────────────────────────────────────────────────────────
git init --quiet "$TMP/origen"
echo uno > "$TMP/origen/a.txt"
git -C "$TMP/origen" add a.txt
git -C "$TMP/origen" commit --quiet -m primero

# ── el clon (su .git es el directorio COMÚN) y su worktree ───────────────────────────────────
git clone --quiet "$TMP/origen" "$TMP/clon"
git -C "$TMP/clon" worktree add --quiet --detach "$TMP/vigia" HEAD

COMUN="$(git -C "$TMP/clon" rev-parse --git-common-dir)"
case "$COMUN" in /*) ;; *) COMUN="$TMP/clon/$COMUN" ;; esac
PROPIO="$(git -C "$TMP/vigia" rev-parse --git-dir)"
case "$PROPIO" in /*) ;; *) PROPIO="$TMP/vigia/$PROPIO" ;; esac

if [ "$COMUN" = "$PROPIO" ]; then
  echo "el worktree comparte gitdir con el clon: la topología que este arnés existe para probar no se armó"
  exit 1
fi
echo "topología armada: el worktree tiene su propio gitdir"

# ── caso 1 · el común VIEJO y el del worktree FRESCO ─────────────────────────────────────────
# Es el caso real que estaba en rojo: el vigía fetchea cada 6 h en SU árbol, y el fetch del
# checkout compartido quedó de hace días.
git -C "$TMP/vigia" fetch --quiet origin '+refs/heads/main:refs/remotes/origin/main'
[ -f "$PROPIO/FETCH_HEAD" ] || { echo "el fetch del worktree no escribió su propio FETCH_HEAD"; exit 1; }
touch -d '60 hours ago' "$COMUN/FETCH_HEAD" 2>/dev/null || touch -t "$(date -d '60 hours ago' +%Y%m%d%H%M)" "$COMUN/FETCH_HEAD"

EDAD="$(bash "$GUION" "$TMP/vigia")"
if [ "$EDAD" != "0" ]; then
  echo "con el común de 60 h y el propio recién traído, el guion dijo ${EDAD}h y tenía que decir 0"
  echo "  (eso es medir la frescura del fetch de OTRO árbol)"
  exit 1
fi
echo "el fetch propio gana al común viejo"

# ── caso 2 · LOS DOS VIEJOS ⇒ tiene que decir viejo ──────────────────────────────────────────
# Sin este caso, un guion que devolviera 0 SIEMPRE pasaría el caso 1 y la guarda no mediría nada.
touch -d '60 hours ago' "$PROPIO/FETCH_HEAD" 2>/dev/null || touch -t "$(date -d '60 hours ago' +%Y%m%d%H%M)" "$PROPIO/FETCH_HEAD"
EDAD="$(bash "$GUION" "$TMP/vigia")"
if [ "$EDAD" -lt 59 ]; then
  echo "con los DOS FETCH_HEAD de 60 h el guion dijo ${EDAD}h: estaría devolviendo 0 sin mirar nada"
  exit 1
fi
echo "con los dos viejos dice viejo"

# ── caso 3 · SIN NINGÚN FETCH_HEAD ⇒ vacío, que no es cero ───────────────────────────────────
rm -f "$PROPIO/FETCH_HEAD" "$COMUN/FETCH_HEAD"
EDAD="$(bash "$GUION" "$TMP/vigia")"
if [ -n "$EDAD" ]; then
  echo "sin ningún FETCH_HEAD el guion dijo '${EDAD}' y tenía que no decir nada: un 0 ahí significa «se trajo recién» sobre un checkout que quizá no fetcheó nunca"
  exit 1
fi
echo "sin FETCH_HEAD no inventa un cero"

# ── caso 4 · un checkout NORMAL (sin worktree) sigue andando ─────────────────────────────────
git -C "$TMP/clon" fetch --quiet origin '+refs/heads/main:refs/remotes/origin/main'
EDAD="$(bash "$GUION" "$TMP/clon")"
if [ "$EDAD" != "0" ]; then
  echo "en un checkout normal recién fetcheado el guion dijo ${EDAD}h y tenía que decir 0"
  exit 1
fi
echo "el checkout normal sigue andando"
