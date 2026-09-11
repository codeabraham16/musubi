#!/usr/bin/env bash
# podar-ramas.sh — decide qué ramas se pueden borrar, POR CONTENIDO y no por ancestría.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# POR QUÉ NO ALCANZA CON `git branch --merged`
#
# `--merged` responde por ANCESTRÍA: «¿este commit está en la historia de main?». Y este repo
# mergea con SQUASH, así que una rama cuyo contenido entró entero a `main` sigue contestando «no
# mergeada» PARA SIEMPRE — su commit nunca va a ser ancestro de nada.
#
# Medido: `fix/powershell-escape-zombis` está idéntica en `origin/main` y `--merged` la reporta
# como pendiente. Si la poda se decidiera así, esas ramas se acumularían sin fin; y si alguien
# las borrara igual «porque el contenido está», estaría decidiendo por corazonada.
#
# LA PREGUNTA CORRECTA ES DE CONTENIDO: ¿queda algo en esta rama que `main` no tenga? Se contesta
# con `git diff origin/main...RAMA` — los cambios que la rama introdujo y main no— o, para una
# rama squasheada, comparando el ÁRBOL. Acá se usa el diff de tres puntos y se exige que esté
# VACÍO: si no hay una sola línea que main no tenga, no hay nada que perder.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# ESTE GUION NO BORRA NADA POR DEFECTO, Y ESO ES A PROPÓSITO
#
# Imprime el veredicto de cada rama y el comando exacto para borrar las podables. Borrar 57 ramas
# es irreversible en la práctica —el reflog caduca— y el repo ya tiene un cabo abierto (A122) que
# dice exactamente eso: «trabajo pago sin mergear que vive en un solo disco».
#
# Con `--borrar` sí borra, y ANTES crea un bundle con todo lo que va a tocar. El bundle es la red;
# sin red no se salta.
# ════════════════════════════════════════════════════════════════════════════════════════════
set -uo pipefail

BASE="${BASE:-origin/main}"
BORRAR=0
SOLO_RAMAS=0
case "${1:-}" in
  --borrar) BORRAR=1 ;;
  # SACAR UN WORKTREE ES LO ÚNICO DE ACÁ QUE PUEDE MOLESTAR A OTRO: si una sesión está trabajando
  # adentro, se queda sin piso a mitad de camino. Las ramas que NINGÚN worktree tiene tomadas no
  # le pueden sacar nada a nadie, así que se pueden podar sin coordinar.
  --borrar-solo-ramas) BORRAR=1; SOLO_RAMAS=1 ;;
esac

cd "$(git rev-parse --show-toplevel)" || exit 2

# SE ACTUALIZA LA REFERENCIA ANTES DE DECIDIR. Un `origin/main` rancio hace que ramas ya
# integradas parezcan tener trabajo único — el repo ya pagó ese error con otro nombre.
git fetch -q origin main 2>/dev/null || {
  printf '✘ no se pudo actualizar %s; sin una referencia fresca este veredicto no vale\n' "$BASE" >&2
  exit 2
}

# Las ramas que un worktree tiene tomadas NO se tocan: borrarlas deja el worktree huérfano.
TOMADAS="$(git worktree list --porcelain | sed -n 's|^branch refs/heads/||p' | sort -u)"

PODABLES=""
n_podables=0
n_vivas=0
n_tomadas=0

while read -r rama; do
  [ -z "$rama" ] && continue
  [ "$rama" = "main" ] && continue
  if printf '%s\n' "$TOMADAS" | grep -qx -- "$rama"; then
    printf '  \033[36m⊘\033[0m %-52s la tiene un worktree\n' "$rama"
    n_tomadas=$((n_tomadas + 1))
    continue
  fi
  # EL DIFF DE TRES PUNTOS: lo que la rama introdujo y la base no tiene. Vacío = no hay nada que
  # perder, aunque el commit no sea ancestro (squash).
  if [ -z "$(git diff --stat "$BASE...$rama" 2>/dev/null)" ]; then
    printf '  \033[32m✔\033[0m %-52s su contenido ya está en %s\n' "$rama" "$BASE"
    PODABLES="$PODABLES $rama"
    n_podables=$((n_podables + 1))
  else
    lineas="$(git diff --shortstat "$BASE...$rama" 2>/dev/null)"
    printf '  \033[33m●\033[0m %-52s TIENE TRABAJO PROPIO:%s\n' "$rama" "$lineas"
    n_vivas=$((n_vivas + 1))
  fi
done <<EOF
$(git for-each-ref --format='%(refname:short)' refs/heads/)
EOF

printf '\n%d podables · %d con trabajo propio · %d tomadas por un worktree\n' \
  "$n_podables" "$n_vivas" "$n_tomadas"

# ── LOS WORKTREES, QUE SON LA MITAD DEL PROBLEMA ────────────────────────────────────────────
#
# Una rama que un worktree tiene tomada no se puede borrar, así que sin esta parte la poda se
# frena en seco: medido el 2026-09-11, 80 de 130 ramas estaban tomadas por un worktree y el guion
# sólo podía podar las 49 que quedaban.
#
# UN WORKTREE ES DESECHABLE CUANDO LAS DOS COSAS: su rama no aporta nada que `main` no tenga, Y su
# árbol está limpio. Las dos hacen falta — un árbol sucio puede tener trabajo sin commitear, y
# este repo ya perdió tiempo con eso: «un agente muerto deja el sabotaje puesto», cuatro worktrees
# sucios con dos sabotajes, un scratch y trabajo real sin commitear.
printf '\n[1mworktrees[0m\n'
n_desechables=0
n_mirar=0
while read -r ruta; do
  [ -z "$ruta" ] && continue
  case "$ruta" in */worktrees/*) ;; *) continue ;; esac
  rama="$(git -C "$ruta" symbolic-ref --quiet --short HEAD 2>/dev/null || echo '')"
  sucio="$(git -C "$ruta" status --short 2>/dev/null | head -1)"
  propio=""
  [ -n "$rama" ] && propio="$(git diff --stat "$BASE...$rama" 2>/dev/null)"
  if [ -z "$sucio" ] && [ -z "$propio" ] && [ -n "$rama" ]; then
    printf '  [32m✔[0m %-46s desechable\n' "$(basename "$ruta")"
    n_desechables=$((n_desechables + 1))
  else
    motivo="contenido propio"
    [ -n "$sucio" ] && motivo="ÁRBOL SUCIO"
    [ -z "$rama" ] && motivo="HEAD suelto"
    printf '  [33m●[0m %-46s %s\n' "$(basename "$ruta")" "$motivo"
    n_mirar=$((n_mirar + 1))
  fi
done <<EOF
$(git worktree list --porcelain | sed -n 's|^worktree ||p')
EOF
printf '\n%d worktree(s) desechables · %d para mirar\n' "$n_desechables" "$n_mirar"
if [ "$n_desechables" -gt 0 ] && [ "$BORRAR" != "1" ]; then
  printf 'Para sacarlos:  %s --borrar\n' "$0"
fi

[ "$n_podables" -eq 0 ] && exit 0

if [ "$BORRAR" != "1" ]; then
  printf '\nNo se borró nada. Con bundle de respaldo primero:\n'
  printf '  %s --borrar-solo-ramas   # ramas sueltas; no toca ningún worktree\n' "$0"
  printf '  %s --borrar              # además saca los worktrees desechables\n' "$0"
  printf '\nOJO CON EL SEGUNDO si hay otra sesión trabajando: sacarle el worktree de abajo la deja\n'
  printf 'sin piso a mitad de camino. Las ramas sueltas no le pueden sacar nada a nadie.\n'
  exit 0
fi

# LA RED ANTES DEL SALTO. Un bundle con todas las ramas que se van a borrar, fechado.
RESPALDO="$HOME/musubi-poda-$(date +%Y%m%d-%H%M%S).bundle"
# shellcheck disable=SC2086
if ! git bundle create "$RESPALDO" $PODABLES 2>/dev/null; then
  printf '✘ no se pudo crear el bundle de respaldo en %s: no se borra nada\n' "$RESPALDO" >&2
  exit 1
fi
printf '\n✔ respaldo en %s\n' "$RESPALDO"
printf '  OJO: está en ESTE disco. Si lo que te preocupa es perder la máquina, copialo afuera.\n'

# LOS WORKTREES PRIMERO: una rama tomada por un worktree no se puede borrar, así que al revés
# la poda de ramas dejaría afuera justo a las que más sobran.
[ "$SOLO_RAMAS" = "1" ] && printf '  (modo --borrar-solo-ramas: no se toca ningún worktree)\n'
[ "$SOLO_RAMAS" = "1" ] || while read -r ruta; do
  [ -z "$ruta" ] && continue
  case "$ruta" in */worktrees/*) ;; *) continue ;; esac
  rama="$(git -C "$ruta" symbolic-ref --quiet --short HEAD 2>/dev/null || echo '')"
  [ -z "$rama" ] && continue
  [ -n "$(git -C "$ruta" status --short 2>/dev/null | head -1)" ] && continue
  [ -n "$(git diff --stat "$BASE...$rama" 2>/dev/null)" ] && continue
  git worktree remove "$ruta" >/dev/null 2>&1 && printf '  sacado worktree %s\n' "$(basename "$ruta")"
done <<EOF
$(git worktree list --porcelain | sed -n 's|^worktree ||p')
EOF

# Y AHORA SÍ LAS RAMAS. Se recalcula qué está tomado: sacar los worktrees liberó ramas que en el
# primer barrido figuraban intocables.
TOMADAS="$(git worktree list --porcelain | sed -n 's|^branch refs/heads/||p' | sort -u)"
while read -r rama; do
  [ -z "$rama" ] && continue
  [ "$rama" = "main" ] && continue
  printf '%s\n' "$TOMADAS" | grep -qx -- "$rama" && continue
  [ -n "$(git diff --stat "$BASE...$rama" 2>/dev/null)" ] && continue
  git branch -D "$rama" >/dev/null 2>&1 && printf '  borrada %s\n' "$rama"
done <<EOF
$(git for-each-ref --format='%(refname:short)' refs/heads/)
EOF
