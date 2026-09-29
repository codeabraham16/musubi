#!/usr/bin/env bash
#
# respaldo-espera-la-red.sh — ejercita la espera de red de `musubi-backup.sh` con un reloj falso.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ EXISTE (A139)
#
# El timer del respaldo de la laptop es `Persistent=true`: si la máquina estaba apagada a la hora,
# el respaldo corre apenas arranca. El 2026-09-29 a las 10:21:13 corrió antes de que levantara el
# tailnet, y el rsync a musubi-server murió con «Network is unreachable». Un minuto después la red
# estaba. Ahora el guion espera al host del destino, con techo, si la unidad se lo pide con
# `MUSUBI_ESPERA_RED`.
#
# SE CORRE EL GUION DE VERDAD, no una copia de su espera: una réplica de la lógica prueba la
# réplica. Lo que se falsea es el MUNDO: delante del PATH van un `ssh`, un `rsync`, un `rclone`, un
# `date` y un `sleep` falsos que comparten un reloj escrito en un archivo. `sleep 5` adelanta ese
# reloj en vez de dormir, así que cinco minutos de espera se prueban en un segundo, y la «red»
# levanta cuando el reloj llega a la hora que diga cada caso. Todo lo demás (`du`, `find`, `cp`)
# es el de verdad.
#
# Salidas: 0 = todo en orden · 1 = el guion hace algo mal · 2 = no se pudo medir.
#
# Uso:  ./deploy/pruebas/respaldo-espera-la-red.sh [ruta/a/musubi-backup.sh]
set -uo pipefail

GUION="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/musubi-backup.sh}"
[[ -r "$GUION" ]] || { echo "✗ no puedo leer $GUION"; exit 2; }

FALLOS=0
ok()  { echo "  ✓ $1"; }
mal() { echo "✗ $1"; FALLOS=$((FALLOS+1)); }

TMP="$(mktemp -d)" || { echo "✗ no pude crear una carpeta temporal"; exit 2; }
trap 'rm -rf "${TMP:?}"' EXIT

DATE_REAL="$(command -v date)"
[[ -x "$DATE_REAL" ]] || { echo "✗ no encuentro el date de verdad"; exit 2; }
BIN="$TMP/bin"
mkdir -p "$BIN" || { echo "✗ no pude crear $BIN"; exit 2; }

# ── EL MUNDO FALSO ───────────────────────────────────────────────────────────────────────────
# Cada falso lee el reloj de $RELOJ y deja constancia en $REG/<nombre>: «reloj argumentos».
cat > "$BIN/date" <<'FIN'
#!/usr/bin/env bash
# `date +%s` es el reloj falso; cualquier otro formato va al date de verdad.
if [ "$#" -eq 1 ] && [ "$1" = "+%s" ]; then cat "$RELOJ"; exit 0; fi
exec "$DATE_REAL" "$@"
FIN
cat > "$BIN/sleep" <<'FIN'
#!/usr/bin/env bash
echo $(( $(cat "$RELOJ") + ${1%%[!0-9]*} )) > "$RELOJ"
FIN
cat > "$BIN/ssh" <<'FIN'
#!/usr/bin/env bash
ahora="$(cat "$RELOJ")"
echo "$ahora $*" >> "$REG/ssh"
# Si la marca del snapshot todavía no está, esta sonda corrió ANTES del respaldo local (R6).
[ -f "$MARCA_SNAPSHOT" ] || echo "$ahora" >> "$REG/ssh-antes-del-snapshot"
[ "$ahora" -ge "$T_SONDA" ] && exit 0
echo "ssh: connect to host servidor port 22: Network is unreachable" >&2
exit 255
FIN
cat > "$BIN/rsync" <<'FIN'
#!/usr/bin/env bash
ahora="$(cat "$RELOJ")"
echo "$ahora $*" >> "$REG/rsync"
[ "$ahora" -ge "$T_COPIA" ] && exit 0
echo "ssh: connect to host servidor port 22: Network is unreachable" >&2
echo "rsync: connection unexpectedly closed (0 bytes received so far) [sender]" >&2
exit 255
FIN
cat > "$BIN/rclone" <<'FIN'
#!/usr/bin/env bash
echo "$(cat "$RELOJ") $*" >> "$REG/rclone"
FIN
cat > "$BIN/musubi" <<'FIN'
#!/usr/bin/env bash
# Sólo sabe `backup --out DIR`, y como el de verdad imprime ÚNICAMENTE la ruta del snapshot.
[ "${1:-}" = backup ] && [ "${2:-}" = --out ] && [ -n "${3:-}" ] || { echo "musubi falso: sólo sé «backup --out DIR»" >&2; exit 64; }
mkdir -p "$3" && echo snapshot > "$3/memory.db.20260929T102113Z" || exit 1
echo "$3/memory.db.20260929T102113Z"
FIN
chmod +x "$BIN"/*

# correr <caso> <T_SONDA> <T_COPIA> <BACKUP_METHOD> <BACKUP_REMOTE> [MUSUBI_ESPERA_RED]
# El sexto argumento, si está, se pasa aunque sea vacío; si falta, la variable no existe.
correr() {
  CASO="$TMP/casos/$1"
  mkdir -p "$CASO/reg" "$CASO/home"
  echo 0 > "$CASO/reloj"
  : > "$CASO/reg/ssh"; : > "$CASO/reg/rsync"; : > "$CASO/reg/rclone"; : > "$CASO/reg/ssh-antes-del-snapshot"
  local espera=()
  [[ $# -ge 6 ]] && espera=("MUSUBI_ESPERA_RED=$6")
  env -i PATH="$BIN:$PATH" HOME="$CASO/home" DATE_REAL="$DATE_REAL" RELOJ="$CASO/reloj" \
    REG="$CASO/reg" T_SONDA="$2" T_COPIA="$3" MUSUBI_HOME="$CASO/home" MUSUBI_BIN="$BIN/musubi" \
    MARCA_SNAPSHOT="$CASO/home/.musubi/backups/.last_snapshot" \
    BACKUP_METHOD="$4" BACKUP_REMOTE="$5" ${espera[@]+"${espera[@]}"} \
    timeout 20 bash "$GUION" > "$CASO/salida" 2>&1
  echo $? > "$CASO/rc"
}
rc()      { cat "$CASO/rc"; }
salida()  { cat "$CASO/salida"; }
sondeos() { wc -l < "$CASO/reg/ssh" | tr -d ' '; }
relojes() { awk '{printf "%s%s", (NR > 1 ? " " : ""), $1}' "$CASO/reg/$1"; }   # a qué hora corrió cada llamada
marca()   { [[ -f "$CASO/home/.musubi/backups/$1" ]]; }

echo "la espera de red del respaldo — con reloj falso"

# ── CONTROL DEL RELOJ FALSO ──────────────────────────────────────────────────────────────────
# Si el `date` o el `sleep` falsos no se usaran, las esperas de abajo correrían con el reloj de
# verdad —o no correrían— y todo lo que sigue mediría otra cosa. Eso no es un rojo del guion:
# es no haber podido medir.
echo 0 > "$TMP/reloj-control"
R="$(env -i PATH="$BIN:$PATH" DATE_REAL="$DATE_REAL" RELOJ="$TMP/reloj-control" \
  bash -c 'date +%s; sleep 5; date +%s; date -u +%Y-%m-%dT%H:%M:%SZ | grep -q "^2" && echo fecha-real')"
if [[ "$R" == $'0\n5\nfecha-real' ]]; then
  ok "el reloj falso avanza con sleep, y date sigue dando la fecha de verdad para las marcas"
else
  echo "✗ el reloj falso no anda (dio «${R//$'\n'/ }»): nada de lo que sigue mediría la espera"
  exit 2
fi

# ── CONTROL: LA RED ARRIBA, SIN ESPERA ───────────────────────────────────────────────────────
# El guion tiene que funcionar entero en este mundo falso antes de medir nada más.
correr control 0 0 rsync "respaldo@servidor:/srv/copias"
if [[ "$(rc)" == 0 && "$(relojes rsync)" == "0" ]] && marca .last_offhost; then
  ok "control: con la red arriba la copia sale en el acto y deja .last_offhost"
else
  echo "✗ control: con la red arriba la copia tenía que salir en el 0 (rc=$(rc), rsync en «$(relojes rsync)»):"
  salida | sed 's/^/    /'
  exit 2
fi

# ── E1 · SIN LA VARIABLE, EL GUION FALLA RÁPIDO: ES EL INCIDENTE ─────────────────────────────
correr sin-espera 12 12 rsync "respaldo@servidor:/srv/copias"
if [[ "$(rc)" == 1 && "$(relojes rsync)" == "0" && "$(sondeos)" == 0 ]] && marca .last_offhost_error; then
  ok "sin MUSUBI_ESPERA_RED no espera: copia en el 0 y falla como el 2026-09-29 (rc=1, .last_offhost_error)"
else
  mal "sin MUSUBI_ESPERA_RED el guion tiene que copiar en el acto y fallar: rc=$(rc), rsync en «$(relojes rsync)», $(sondeos) sondeos"
fi

# ── E2 · CON LA ESPERA, COPIA APENAS EL HOST CONTESTA ────────────────────────────────────────
correr espera 12 12 rsync "respaldo@servidor:/srv/copias" 300
if [[ "$(rc)" == 0 && "$(relojes ssh)" == "0 5 10 15" && "$(relojes rsync)" == "15" ]] && marca .last_offhost; then
  ok "con 300 s de espera sondea cada 5 s y copia apenas el host contesta: sondeos en 0 5 10 15, rsync en el 15"
else
  mal "con la red a los 12 s la copia tenía que salir en el 15: rc=$(rc), sondeos en «$(relojes ssh)», rsync en «$(relojes rsync)»"
fi
if salida | grep -q 'todavía no contesta; espero hasta 300s' && salida | grep -q 'seguí a los 15s'; then
  ok "el registro dice que esperó y cuánto"
else
  mal "el registro no dice que esperó ni cuánto: quien lea el journal no sabe por qué la copia tardó"
fi
AVISOS="$(salida | grep -c 'todavía no contesta')"
if [[ "$AVISOS" == 1 ]]; then
  ok "avisa que espera una sola vez, no una por sonda"
else
  mal "el aviso de la espera salió $AVISOS veces y tenía que salir una: cinco minutos de espera serían sesenta líneas en el journal"
fi

# ── E4 · QUÉ SE SONDEA: EL HOST DEL DESTINO, POR EL MISMO SSH QUE USA RSYNC ──────────────────
ARGS="$(sed -n 1p "$CASO/reg/ssh" | cut -d' ' -f2-)"
if [[ "$ARGS" == "-n -o BatchMode=yes -o ConnectTimeout=5 respaldo@servidor true" ]]; then
  ok "sondea el host con su usuario y sin la ruta, sin pedir clave y sin leer la entrada"
else
  mal "la sonda tenía que ser «ssh -n -o BatchMode=yes -o ConnectTimeout=5 respaldo@servidor true» y fue «$ARGS»"
fi

# ── R6 · LA ESPERA NO DEMORA EL RESPALDO LOCAL ───────────────────────────────────────────────
# El snapshot y su marca salen ANTES de sondear: la red le importa a la copia off-host, no al
# respaldo local, que tiene que quedar hecho aunque la red no vuelva nunca. Cada sonda falsa anota
# si la marca ya estaba. Sin este control, mudar la espera arriba del snapshot dejaba todo lo
# demás en verde: lo midió la revisión de A139.
ANTES="$(wc -l < "$CASO/reg/ssh-antes-del-snapshot" | tr -d ' ')"
if [[ "$(sondeos)" -gt 0 && "$ANTES" == 0 ]]; then
  ok "la espera corre después del snapshot: las $(sondeos) sondas encontraron la marca .last_snapshot"
else
  mal "la espera tiene que correr después del snapshot: $ANTES de $(sondeos) sondas no encontraron .last_snapshot"
fi

# ── E3 · LA ESPERA VENCE: SE SIGUE IGUAL, Y EL FALLO SALE POR EL CAMINO DE SIEMPRE ───────────
correr vence 999999 0 rsync "respaldo@servidor:/srv/copias" 300
if [[ "$(rc)" == 0 && "$(relojes rsync)" == "300" ]] && salida | grep -q 'sigue sin contestar después de 300s'; then
  ok "al vencerse la espera sigue igual y copia en el 300: el techo no cuelga al respaldo ni lo corta"
else
  mal "al vencerse la espera el guion tiene que avisar y seguir con la copia: rc=$(rc), rsync en «$(relojes rsync)»"
fi
correr vence-y-falla 999999 999999 rsync "respaldo@servidor:/srv/copias" 300
if [[ "$(rc)" == 1 ]] && marca .last_offhost_error && salida | grep -q 'rsync falló'; then
  ok "si la red nunca vuelve, falla por el camino de siempre (rsync falló, rc=1, .last_offhost_error)"
else
  mal "si la red nunca vuelve el respaldo tiene que fallar como siempre: rc=$(rc)"
fi

# ── E5 · LOS DESTINOS QUE NO VAN POR SSH NO SE SONDEAN ───────────────────────────────────────
# La sonda nunca contesta en estos casos: si el guion sondeara, esperaría los 300 s enteros.
sin_sonda() {  # sin_sonda <caso> <método> <destino> <por qué>
  correr "$1" 999999 0 "$2" "$3" 300
  local copio=1
  case "$2" in
    rsync | rclone) [[ "$(relojes "$2")" == "0" ]] || copio=0 ;;
  esac
  if [[ "$(rc)" == 0 && "$(sondeos)" == 0 && "$copio" == 1 ]]; then
    ok "$2 a «$3»: no sondea y copia en el acto ($4)"
  else
    mal "$2 a «$3» tenía que copiar sin sondear ($4): rc=$(rc), $(sondeos) sondeos"
  fi
}
sin_sonda rclone rclone "remoto:musubi"                               "rclone no va por ssh"
if salida | grep -q 'la espera de red no aplica a este destino'; then
  ok "y lo dice: la espera no aplica a ese destino"
else
  mal "con un destino que no va por ssh el registro tiene que decir que la espera no aplica"
fi
sin_sonda daemon rsync  "servidor::modulo"                            "«host::módulo» es el daemon de rsync"
sin_sonda url    rsync  "rsync://servidor/modulo"                     "rsync:// es el daemon de rsync"
sin_sonda barra  rsync  "./copias:viejas"                             "una «/» antes del «:» es una ruta local"
sin_sonda local  rsync  "copias"                                      "sin «:» es una ruta local"
sin_sonda ipv6   rsync  "respaldo@[2001:db8:0:0:0:0:0:1]:/srv/copias" "un IPv6 entre corchetes no se separa con esta regla"
sin_sonda cp     cp     "$TMP/destino-cp"                             "cp copia a una ruta local"

# ── E6 · LO QUE NO ES UN NÚMERO POSITIVO NO ES UNA ESPERA ────────────────────────────────────
# `300s` es como escribe systemd una duración, y el guion no la toma: por eso la guarda de la unidad
# exige un entero.
for v in 0 -5 abc 300s ""; do
  correr "valor-${v:-vacio}" 999999 0 rsync "respaldo@servidor:/srv/copias" "$v"
  if [[ "$(rc)" == 0 && "$(sondeos)" == 0 && "$(relojes rsync)" == "0" ]]; then
    ok "MUSUBI_ESPERA_RED=«$v» no espera"
  else
    mal "MUSUBI_ESPERA_RED=«$v» no es una espera, y el guion sondeó $(sondeos) veces (rc=$(rc))"
  fi
done

echo
if (( FALLOS == 0 )); then echo "TODO OK"; exit 0
else echo "$FALLOS FALLO(S)"; exit 1; fi
