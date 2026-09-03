#!/usr/bin/env bash
#
# redesplegar-cerebro.sh — reemplaza el binario del cerebro central, con vuelta atrás automática.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ ESTE DESPLIEGUE NO ES COMO LOS OTROS
#
# En este servidor `musubi-brain.service` y `musubi-agente.service` COMPARTEN EJECUTABLE
# (/usr/local/bin/musubi). Así que «actualizar el agente» no existe: cualquier cambio del binario
# es un redespliegue del cerebro, con su ventana de indisponibilidad.
#
# Y el esquema es UNA PUERTA DE UNA SOLA DIRECCIÓN. `applyMigrations` es fail-closed: un binario
# viejo se NIEGA a abrir una base migrada por uno nuevo. Eso está bien —evita que dos versiones
# se pisen— pero significa que **volver atrás el binario no alcanza**: hay que volver atrás
# también la base. Por eso este script saca su propio respaldo antes de tocar nada, y el rollback
# restaura LAS DOS COSAS.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# CÓMO VERIFICA, Y POR QUÉ NO ALCANZA `systemctl is-active`
#
# Ya pasó en este servidor: `systemctl is-active` decía `active` y el proceso corría un binario
# BORRADO (`/proc/PID/exe -> ...(deleted)`), o sea que el reemplazo no había tomado efecto y todo
# se veía bien. Acá se compara el INODO del ejecutable del proceso contra el del archivo en disco.
#
# Uso:  sudo ./redesplegar-cerebro.sh /ruta/al/binario-nuevo <sha256-esperado>
set -uo pipefail

log(){ printf '\033[36m▶ %s\033[0m\n' "$*"; }
ok(){  printf '\033[32m✓ %s\033[0m\n' "$*"; }
aviso(){ printf '\033[33m! %s\033[0m\n' "$*"; }
# Código de salida del redespliegue. Se pone en 1 si el verificador divergió o no pudo mirar: el
# binario queda desplegado igual (por eso no se aborta), pero terminar en 0 sería decir que el
# redespliegue salió bien cuando su propia comprobación dice que no, o que no se sabe.
SALIDA=0
die(){ printf '\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

NUEVO="${1:-}"; SHA_ESPERADO="${2:-}"
DESTINO="/usr/local/bin/musubi"
HOME_CEREBRO="${MUSUBI_HOME:-/home/musubi/musubi-brain}"
BASE="$HOME_CEREBRO/.musubi/memory.db"
SERVICIOS=(musubi-brain musubi-agente musubi-dashboard)

[[ $EUID -eq 0 ]] || die "hay que correrlo como root: reemplaza $DESTINO y reinicia unidades del sistema"
[[ -n "$NUEVO" && -f "$NUEVO" ]] || die "uso: sudo $0 /ruta/al/binario-nuevo <sha256-esperado>"
[[ -n "$SHA_ESPERADO" ]] || die "falta el sha256 esperado. NO se despliega un binario sin verificar: una descarga truncada devuelve éxito igual (ya pasó)"

# ── El binario es el que se verificó, y no otro ──────────────────────────────────────────────
SHA_REAL="$(sha256sum "$NUEVO" | cut -d' ' -f1)"
[[ "$SHA_REAL" == "$SHA_ESPERADO" ]] || die "el sha256 no coincide.
    esperado: $SHA_ESPERADO
    real:     $SHA_REAL
  No se despliega. Volvé a subir el binario."
ok "sha256 verificado"

VERSION_NUEVA="$("$NUEVO" version 2>&1 | head -1)"
VERSION_VIEJA="$("$DESTINO" version 2>&1 | head -1)"
log "de:  $VERSION_VIEJA"
log "a:   $VERSION_NUEVA"

# ── El punto de retorno. Antes de tocar NADA. ────────────────────────────────────────────────
SELLO="$(date +%Y%m%d-%H%M%S)"
RESPALDO="$HOME_CEREBRO/.musubi/backups/pre-redespliegue-$SELLO.db"
BIN_VIEJO="/usr/local/bin/musubi.antes-de-$SELLO"

log "sacando el punto de retorno"
mkdir -p "$(dirname "$RESPALDO")"
# La API de respaldo de SQLite, no un `cp`: el cerebro está escribiendo y un cp en caliente puede
# dejar una base a medio escribir que parece buena hasta que se la necesita.
python3 - "$BASE" "$RESPALDO" <<'PY' || die "no se pudo respaldar la base. SIN RESPALDO NO SE SIGUE."
import sqlite3, sys
src, dst = sys.argv[1], sys.argv[2]
s = sqlite3.connect(src); d = sqlite3.connect(dst)
s.backup(d); d.close(); s.close()
PY
[[ -s "$RESPALDO" ]] || die "el respaldo salió vacío. No se sigue."
cp -a "$DESTINO" "$BIN_VIEJO" || die "no se pudo apartar el binario viejo"
VERSION_BASE="$(python3 -c "import sqlite3,sys;print(sqlite3.connect(sys.argv[1]).execute('PRAGMA user_version').fetchone()[0])" "$RESPALDO")"
ok "respaldo: $RESPALDO (esquema $VERSION_BASE)"
ok "binario viejo: $BIN_VIEJO"

# ── volver_atras deshace LAS DOS COSAS, porque el esquema no vuelve solo ─────────────────────
volver_atras(){
  aviso "VOLVIENDO ATRÁS: $1"
  systemctl stop "${SERVICIOS[@]}" 2>/dev/null
  cp -a "$BIN_VIEJO" "$DESTINO"
  cp -a "$RESPALDO" "$BASE"
  # El WAL y el SHM de la base migrada no pueden sobrevivir a la restauración.
  rm -f "$BASE-wal" "$BASE-shm"
  chown musubi:musubi "$BASE"
  systemctl start "${SERVICIOS[@]}" 2>/dev/null
  sleep 5
  if systemctl is-active --quiet musubi-brain; then
    aviso "vuelta atrás COMPLETA: binario $VERSION_VIEJA y base en esquema $VERSION_BASE"
  else
    die "LA VUELTA ATRÁS TAMBIÉN FALLÓ. El respaldo está en $RESPALDO y el binario en $BIN_VIEJO; hay que restaurarlos a mano y mirar: journalctl -u musubi-brain -n 50"
  fi
  exit 1
}

# ── El cambio ────────────────────────────────────────────────────────────────────────────────
log "deteniendo ${SERVICIOS[*]}"
systemctl stop "${SERVICIOS[@]}" || volver_atras "no se pudieron detener los servicios"

install -m 0755 -o root -g root "$NUEVO" "$DESTINO" || volver_atras "no se pudo instalar el binario nuevo"
ok "binario reemplazado"

log "arrancando (acá corre la migración 35 → 37)"
systemctl start "${SERVICIOS[@]}" || volver_atras "no arrancaron los servicios"
sleep 8

# ── VERIFICAR. Que arranque no prueba que quedó útil. ────────────────────────────────────────
log "verificando"

systemctl is-active --quiet musubi-brain || volver_atras "musubi-brain no quedó activo"

# El inodo, no el `is-active`: un proceso puede estar corriendo el binario BORRADO.
PID="$(systemctl show -p MainPID --value musubi-brain)"
INODO_PROC="$(stat -Lc %i "/proc/$PID/exe" 2>/dev/null || echo x)"
INODO_DISCO="$(stat -c %i "$DESTINO")"
[[ "$INODO_PROC" == "$INODO_DISCO" ]] || volver_atras "el proceso corre OTRO binario que el que está en disco (inodo $INODO_PROC vs $INODO_DISCO): el reemplazo no tomó efecto"
ok "el proceso corre el binario nuevo (inodo verificado)"

VERSION_CORRIENDO="$("$DESTINO" version 2>&1 | head -1)"
[[ "$VERSION_CORRIENDO" == "$VERSION_NUEVA" ]] || volver_atras "la versión en disco no es la que se instaló"

ESQUEMA="$(python3 -c "import sqlite3,sys;print(sqlite3.connect(sys.argv[1]).execute('PRAGMA user_version').fetchone()[0])" "$BASE" 2>/dev/null || echo 0)"
[[ "$ESQUEMA" -ge 37 ]] || volver_atras "la migración no corrió: el esquema quedó en $ESQUEMA y se esperaba 37"
ok "esquema migrado: $VERSION_BASE → $ESQUEMA"

# Que responda de verdad, no que el puerto esté abierto.
# Se exige 200, no «cualquier respuesta»: un cerebro que arrancó y contesta 503 está roto
# igual, y aceptar el 503 sería otra verificación que pasa por el motivo equivocado.
for i in $(seq 1 20); do
  CODIGO="$(curl -s -o /dev/null -w '%{http_code}' -m 5 http://127.0.0.1:7717/healthz 2>/dev/null || echo 000)"
  [[ "$CODIGO" == "200" ]] && break
  sleep 1
done
[[ "$CODIGO" == "200" ]] || volver_atras "/healthz devolvió $CODIGO (se esperaba 200) después de 20 s"
ok "el cerebro contesta sano (HTTP 200 en /healthz)"

for u in "${SERVICIOS[@]}"; do
  systemctl is-active --quiet "$u" && ok "$u activo" || aviso "$u NO quedó activo — miralo con: journalctl -u $u -n 30"
done

echo
ok "REDESPLIEGUE COMPLETO — $VERSION_NUEVA"
echo

# ── EL BINARIO NO ES LO ÚNICO QUE SE DESPLIEGA (A73) ─────────────────────────────────────────
#
# Las reglas de alerta viven en archivos aparte, se copian por otro camino, y NADA comparaba lo
# que corre contra lo que el repo dice. El 2026-09-02 se midió: 29 reglas cargadas contra 31, y
# las dos que faltaban eran `CadenaDeAlertasFallando` —la que vigila que las alertas se
# entreguen— y una recién escrita. Su guarda de repo pasaba en verde.
#
# El script vive en el repo, y este redespliegue se corre EN EL SERVIDOR (systemctl, /usr/local).
# Si el repo está acá, se corre; si no, se dice qué correr y desde dónde. Un redespliegue que
# calla esto se lee como si hubiera desplegado todo.
VERIFICAR="$(dirname "${BASH_SOURCE[0]}")/verificar-despliegue.sh"
if [[ -x "$VERIFICAR" ]]; then
  echo "─── comparando las reglas de alerta contra el repo ───"
  # Los dos códigos del verificador dicen cosas DISTINTAS y colapsarlos era afirmar de más: con 2
  # («no pude preguntar») el mensaje viejo aseguraba que las reglas NO eran las del repo, cuando lo
  # único cierto es que nadie las miró. Y como `aviso` sólo imprime, este redespliegue terminaba en
  # 0 en los dos casos: un redespliegue que dejó las reglas divergiendo se leía como exitoso.
  "$VERIFICAR"; RC_VERIF=$?
  case "$RC_VERIF" in
    0) ;;
    1) SALIDA=1; aviso "las reglas de alerta que corren NO son las del repo (ver arriba). El binario SÍ quedó desplegado; lo que falta es \`deploy/docker/preparar.sh\` y recargar Prometheus." ;;
    2) SALIDA=1; aviso "NO se pudo verificar el despliegue (ver los «?» de arriba): no es que esté mal, es que nadie lo miró. El binario SÍ quedó desplegado. Resolvé lo que pide cada «?» y volvé a correr \`$VERIFICAR\`." ;;
    *) SALIDA=1; aviso "el verificador terminó con un código inesperado ($RC_VERIF). El binario SÍ quedó desplegado, pero el despliegue quedó SIN verificar." ;;
  esac
else
  aviso "Falta comparar las reglas de alerta contra el repo. Desde la máquina que lo tiene:"
  echo "    MUSUBI_SSH=musubi-server ./deploy/verificar-despliegue.sh"
fi
echo
aviso "El punto de retorno queda guardado. Para volver atrás A MANO:"
echo "    systemctl stop ${SERVICIOS[*]}"
echo "    cp -a $BIN_VIEJO $DESTINO"
echo "    cp -a $RESPALDO $BASE && rm -f $BASE-wal $BASE-shm && chown musubi:musubi $BASE"
echo "    systemctl start ${SERVICIOS[*]}"
echo
aviso "Borralos recién cuando estés seguro: el esquema NO vuelve solo, y sin ese .db no hay vuelta."

# El exit va DESPUÉS del punto de retorno a propósito: quien corre esto necesita las instrucciones
# de vuelta atrás en pantalla aunque la verificación haya salido mal — sobre todo si salió mal.
exit "$SALIDA"
