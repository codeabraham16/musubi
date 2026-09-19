#!/usr/bin/env bash
#
# edad-de-la-referencia.sh — cuántas HORAS hace que este checkout trajo `origin/main`.
#
# Imprime un entero, o nada si no hay ningún `FETCH_HEAD` que mirar. Existe como guion aparte —y no
# como diez líneas adentro de `verificar-despliegue.sh`— para que su arnés
# (`deploy/pruebas/frescura-de-la-referencia.sh`) pueda ejercitar ESTA lógica y no una copia suya.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# POR QUÉ MIRA DOS DIRECTORIOS Y SE QUEDA CON EL MÁS NUEVO
#
# `git` escribe `FETCH_HEAD` en el directorio del ÁRBOL que hizo el fetch, no en el compartido:
#
#     --git-dir        → .git/worktrees/<nombre>/FETCH_HEAD   ← lo escribe el fetch de ESTE árbol
#     --git-common-dir → .git/FETCH_HEAD                      ← lo escribe el fetch de OTRO árbol
#
# En un checkout normal los dos son el MISMO directorio y da igual cuál se mire. En un WORKTREE no,
# y ahí estaba el defecto: mirando sólo el común, un vigía que fetchea cada 6 h reportaba «la
# referencia se trajo hace 59 h» —la edad del último fetch hecho desde el checkout compartido— y
# marcaba el eslabón como SIN VERIFICAR. Medido el 2026-09-19: cuatro corridas seguidas en rojo
# sobre un árbol que estaba al día.
#
# SE TOMA EL MÁS NUEVO Y NO EL DEL WORKTREE A SECAS, y el motivo no es simetría: la referencia
# `refs/remotes/origin/main` es COMPARTIDA —vive en el directorio común y la actualiza cualquiera de
# los dos fetch—, así que su frescura real es la del más reciente. Quedarse sólo con el del worktree
# daría el falso viejo al revés: un vigía recién creado, que todavía no fetcheó, con el común fresco.
set -euo pipefail

REPO="${1:-.}"

mtime_mas_nuevo=0
for gd in "$(git -C "$REPO" rev-parse --git-dir 2>/dev/null || echo '')" \
          "$(git -C "$REPO" rev-parse --git-common-dir 2>/dev/null || echo '')"; do
  case "$gd" in
    "") continue ;;
    /*) ;;
    *) gd="$REPO/$gd" ;;
  esac
  [ -f "$gd/FETCH_HEAD" ] || continue
  m="$(stat -c %Y "$gd/FETCH_HEAD" 2>/dev/null || echo 0)"
  if [ "$m" -gt "$mtime_mas_nuevo" ]; then mtime_mas_nuevo="$m"; fi
done

# SIN NINGÚN `FETCH_HEAD` NO SE IMPRIME NADA, y eso NO es «cero horas». Un 0 ahí diría «se trajo
# recién» sobre un checkout que puede no haber fetcheado nunca, que es exactamente el caso en que
# más importa avisar. El llamador distingue vacío de número.
if [ "$mtime_mas_nuevo" -gt 0 ]; then
  echo $(( ( $(date +%s) - mtime_mas_nuevo ) / 3600 ))
fi
