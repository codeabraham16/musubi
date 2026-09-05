#!/usr/bin/env bash
#
# poda-puntos-de-retorno.sh — ejercita la poda de A87 de `redesplegar-cerebro.sh`.
#
# EXTRAE LA FUNCIÓN DEL ARCHIVO REAL, no de una copia: una prueba con su propia copia del código
# se queda verde mientras el original se rompe, que es la forma exacta de prueba decorativa que
# este repo persigue. La corre `internal/mcp/despliegue_poda_test.go`, así que entra en CI.
#
# Uso:  ./deploy/pruebas/poda-puntos-de-retorno.sh deploy/redesplegar-cerebro.sh
set -uo pipefail
SCRIPT="${1:?falta la ruta de redesplegar-cerebro.sh}"
[[ -r "$SCRIPT" ]] || { echo "✗ no puedo leer $SCRIPT"; exit 1; }

# Los helpers del guión real no hacen falta acá; se silencian para que la salida sea la de la prueba.
log(){ :; }; ok(){ :; }; aviso(){ :; }
eval "$(sed -n '/^podar_puntos_de_retorno(){/,/^}$/p' "$SCRIPT")"
declare -F podar_puntos_de_retorno >/dev/null || {
  echo "✗ no pude extraer podar_puntos_de_retorno de $SCRIPT."
  echo "  O la función cambió de nombre, o dejó de empezar en columna 0 — y entonces esta prueba"
  echo "  no está mirando nada. Arreglá la extracción antes de creerle a un verde."
  exit 1
}

fallos=0
comprobar(){ # $1 qué · $2 esperado · $3 obtenido
  if [[ "$2" == "$3" ]]; then printf '  ✓ %s\n' "$1"
  else printf '  ✗ %s\n     esperaba: %s\n     obtuvo  : %s\n' "$1" "$2" "$3"; fallos=$((fallos+1)); fi
}
listar(){ (cd "$1" && ls -1 | tr '\n' ' ' | sed 's/ $//'); }

# ── 1 · ordena por NOMBRE y no por fecha ───────────────────────────────────────────────────────
# Los mtime van AL REVÉS del nombre a propósito: el más viejo por nombre es el más nuevo por fecha.
# Es el caso real medido en el servidor — `cp -a` preserva el mtime del origen, así que dos
# binarios apartados en corridas distintas de la MISMA versión comparten fecha. Una poda que
# ordene por mtime borra los tres equivocados y este caso se pone rojo.
D=$(mktemp -d); i=0
for s in 20260901-100000 20260901-110000 20260902-090000 20260903-080000 \
         20260903-200000 20260904-003000 20260904-011500 20260904-120000; do
  : > "$D/pre-redespliegue-$s.db"
  touch -d "2026-09-10 -$i hour" "$D/pre-redespliegue-$s.db"; i=$((i+1))
done
RETENER=5 podar_puntos_de_retorno "t" "$D" 'pre-redespliegue-*.db' "$D/pre-redespliegue-20260904-120000.db"
comprobar "borra los 3 más viejos POR NOMBRE, no por fecha" \
  "pre-redespliegue-20260903-080000.db pre-redespliegue-20260903-200000.db pre-redespliegue-20260904-003000.db pre-redespliegue-20260904-011500.db pre-redespliegue-20260904-120000.db" \
  "$(listar "$D")"
rm -rf "$D"

# ── 2 · con menos o igual que la retención no toca nada ────────────────────────────────────────
D=$(mktemp -d)
for s in 20260901-100000 20260902-100000 20260903-100000; do : > "$D/pre-redespliegue-$s.db"; done
RETENER=5 podar_puntos_de_retorno "t" "$D" 'pre-redespliegue-*.db' "$D/pre-redespliegue-20260903-100000.db"
comprobar "3 archivos con retención 5: no borra nada" "3" "$(ls -1 "$D" | wc -l)"
rm -rf "$D"

# ── 3 · el punto de retorno de ESTA corrida sobrevive incluso con retención 0 ───────────────────
# No es hipotético: con el sabotaje del caso 1 (orden por fecha) el de esta corrida CAÍA en la
# lista de borrado, y lo salvó esta guarda. Sin ella, un despliegue se queda sin vuelta atrás.
D=$(mktemp -d)
for s in 20260901-100000 20260902-100000 20260904-120000; do : > "$D/pre-redespliegue-$s.db"; done
RETENER=0 podar_puntos_de_retorno "t" "$D" 'pre-redespliegue-*.db' "$D/pre-redespliegue-20260904-120000.db"
comprobar "con retención 0 sobrevive el de ESTA corrida" \
  "pre-redespliegue-20260904-120000.db" "$(listar "$D")"
rm -rf "$D"

# ── 4 · un directorio vacío no explota ni borra ────────────────────────────────────────────────
D=$(mktemp -d)
RETENER=5 podar_puntos_de_retorno "t" "$D" 'pre-redespliegue-*.db' "$D/nada.db"
comprobar "directorio vacío: sin error y sin borrar" "0" "$(ls -1 "$D" | wc -l)"
rm -rf "$D"

echo
if (( fallos )); then echo "✗ $fallos caso(s) en rojo"; exit 1; fi
echo "✓ los 4 casos en verde"
