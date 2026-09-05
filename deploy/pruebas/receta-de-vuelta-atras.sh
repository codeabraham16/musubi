#!/usr/bin/env bash
#
# receta-de-vuelta-atras.sh — ejercita `imprimir_vuelta_atras` de `redesplegar-cerebro.sh`.
#
# POR QUÉ EXISTE: EL GUION IMPRIMÍA UNA SOLA RECETA, Y LA DESTRUCTIVA
#
# La receta a mano llevaba `cp -a $RESPALDO $BASE` SIEMPRE, aunque la corrida no hubiera migrado
# —el caso común: el 2026-09-05 fue 46 → 46—. Restaurar el snapshot ahí es innecesario y descarta
# todo lo que el cerebro escribió desde el despliegue: medido a los seis minutos de esa corrida,
# +57 tool_invocations, +5 device_commands, +1 screen_session. Una vuelta atrás se hace HORAS
# después, no en el minuto siguiente, y a ~9,5 invocaciones por minuto eso son miles de filas de
# la memoria compartida del equipo. Nada avisaba: la línea peligrosa es la que se ve inofensiva.
#
# EXTRAE LA FUNCIÓN DEL ARCHIVO REAL, no de una copia, por el mismo motivo que
# `poda-puntos-de-retorno.sh`: una prueba con su propia copia se queda verde mientras el original
# se rompe. La corre `internal/mcp/despliegue_receta_test.go`, así que entra en CI.
#
# Uso:  ./deploy/pruebas/receta-de-vuelta-atras.sh deploy/redesplegar-cerebro.sh
set -uo pipefail
SCRIPT="${1:?falta la ruta de redesplegar-cerebro.sh}"
[[ -r "$SCRIPT" ]] || { echo "✗ no puedo leer $SCRIPT"; exit 1; }

# `aviso` del guion real escribe a stderr; acá va a stdout para poder revisar el texto completo,
# que es justo donde vive la advertencia que importa.
aviso(){ printf '%s\n' "$*"; }
eval "$(sed -n '/^imprimir_vuelta_atras(){/,/^}$/p' "$SCRIPT")"
declare -F imprimir_vuelta_atras >/dev/null || {
  echo "✗ no pude extraer imprimir_vuelta_atras de $SCRIPT."
  echo "  O la función cambió de nombre, o dejó de empezar en columna 0 — y entonces esta prueba"
  echo "  no está mirando nada. Arreglá la extracción antes de creerle a un verde."
  exit 1
}

SERVICIOS=(musubi-brain.service musubi-sync.timer)
BIN_VIEJO=/opt/musubi/musubi.antes-de-20260905-015017
DESTINO=/opt/musubi/musubi
RESPALDO=/var/lib/musubi/pre-redespliegue-20260905-015017.db
BASE=/home/musubi/musubi-brain/.musubi/memory.db
RETENER=5

fallos=0
tiene(){    # $1 qué · $2 aguja · $3 pajar
  if [[ "$3" == *"$2"* ]]; then printf '  ✓ %s\n' "$1"
  else printf '  ✗ %s\n     no encontré: %s\n' "$1" "$2"; fallos=$((fallos+1)); fi
}
no_tiene(){ # $1 qué · $2 aguja · $3 pajar
  if [[ "$3" != *"$2"* ]]; then printf '  ✓ %s\n' "$1"
  else printf '  ✗ %s\n     NO debía aparecer: %s\n' "$1" "$2"; fallos=$((fallos+1)); fi
}

# ── 1 · la corrida NO migró: la base NO se toca ────────────────────────────────────────────────
echo "· esquema 46 → 46 (no migró)"
VERSION_BASE=46 ESQUEMA=46
SALIDA="$(imprimir_vuelta_atras 2>&1)"
no_tiene "no ofrece restaurar la base"        "cp -a $RESPALDO $BASE"  "$SALIDA"
no_tiene "ni borrar el wal/shm"               "rm -f $BASE-wal"        "$SALIDA"
tiene    "sí restaura el binario"             "cp -a $BIN_VIEJO $DESTINO" "$SALIDA"
tiene    "para los servicios"                 "systemctl stop ${SERVICIOS[*]}" "$SALIDA"
tiene    "y los arranca"                      "systemctl start ${SERVICIOS[*]}" "$SALIDA"
tiene    "dice que NO hay que restaurar"      "NO HAY QUE RESTAURAR LA BASE" "$SALIDA"
tiene    "y por qué: se pierde lo escrito"    "descartaría todo lo que el cerebro escribió" "$SALIDA"

# ── 2 · la corrida SÍ migró: sin la base no hay vuelta ─────────────────────────────────────────
echo "· esquema 45 → 46 (migró)"
VERSION_BASE=45 ESQUEMA=46
SALIDA="$(imprimir_vuelta_atras 2>&1)"
tiene    "ofrece restaurar la base"           "cp -a $RESPALDO $BASE"  "$SALIDA"
tiene    "con el wal/shm"                     "rm -f $BASE-wal"        "$SALIDA"
tiene    "y el chown, que si falta no arranca" "chown musubi:musubi $BASE" "$SALIDA"
tiene    "restaura el binario también"        "cp -a $BIN_VIEJO $DESTINO" "$SALIDA"
tiene    "nombra las dos versiones de esquema" "45 → 46"               "$SALIDA"
tiene    "y AVISA que restaurar cuesta"       "TIENE UN COSTO"         "$SALIDA"

# ── 3 · la diferencia entre las dos recetas es EXACTAMENTE la línea de la base ──────────────────
# Sin esto, las dos ramas podrían divergir en cualquier otra cosa —un servicio que se olvida de
# arrancar, un chown que falta— y las comprobaciones de arriba no lo verían.
echo "· lo único que cambia entre las dos es la base"
VERSION_BASE=46 ESQUEMA=46; SIN="$(imprimir_vuelta_atras 2>&1 | grep -E '^ +(systemctl|cp -a|rm)')"
VERSION_BASE=45 ESQUEMA=46; CON="$(imprimir_vuelta_atras 2>&1 | grep -E '^ +(systemctl|cp -a|rm)')"
DIFF="$(diff <(printf '%s\n' "$SIN") <(printf '%s\n' "$CON") | grep '^>' | sed 's/^> *//')"
if [[ "$DIFF" == "cp -a $RESPALDO $BASE && rm -f $BASE-wal $BASE-shm && chown musubi:musubi $BASE" ]]; then
  echo "  ✓ la única línea de más es la de la base"
else
  echo "  ✗ las dos recetas difieren en más que la base:"; printf '     %s\n' "$DIFF"; fallos=$((fallos+1))
fi

echo
if ((fallos)); then echo "✗ $fallos comprobación(es) fallaron"; exit 1; fi
echo "✓ la receta de vuelta atrás distingue si la corrida migró"
