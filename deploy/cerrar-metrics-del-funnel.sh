#!/usr/bin/env bash
# cerrar-metrics-del-funnel.sh — A130. Cierra `/metrics` y `/debug/pprof` del lado PÚBLICO del
# Funnel que sirve OpenObserve, sin tocar `/config` y sin apagar el Funnel.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ ESTE GUION EXISTE Y NO ES UN `sed` A MANO
#
# La cadena es `Funnel :8443 (Tailscale) → 127.0.0.1:5081 → Caddy (usuario mon) → OpenObserve`, y
# ese Funnel está publicado A PROPÓSITO: es por donde el frontend del ERP de Altura —que vive en
# Vercel, o sea en internet— alcanza a OpenObserve. APAGARLO ROMPE EL ERP. Lo único que sobra es
# que dos rutas de diagnóstico contesten sin credencial.
#
# LO QUE SE CIERRA, Y POR QUÉ SÓLO ESO (medido el 2026-09-21, endpoint por endpoint):
#
#     /web/   200   /healthz 200   /config 200   /metrics 200   /api/** 401 ✔   /debug/pprof/ 401
#
# `/metrics` entrega 88 series (versión, memoria, caché, volumen de consultas). No hay nombres de
# streams, ni usuarios, ni datos: es huella para buscar un CVE y patrón de actividad.
#
# ⚠️ `/config` NO SE CIERRA, y ésa es la parte que casi sale mal. La propuesta obvia era bloquear
# las tres rutas abiertas. Antes de entregarla se midió el bundle del frontend
# (`/web/assets/index-*.js`, 4,7 MB): **`/config` aparece 13 veces y `/metrics` cero**. El frontend
# pide `/config` al arrancar, así que cerrarlo habría roto la UI que el Funnel existe para servir
# — el arreglo causando exactamente el daño que el problema no estaba causando. Lo que `/config`
# publica (la versión exacta) se acota ACTUALIZANDO OpenObserve, no tapando la ruta.
#
# REGLA GENERAL QUE SALIÓ DE ACÁ: antes de cerrar una ruta «que nadie usa», grepeá el bundle del
# cliente que la consume.
#
# POR QUÉ ES UN GUION Y NO UNA RECETA EN PROSA: el Caddyfile vive en el home de `mon` (0700), así
# que quien lo aplica no es quien lo escribió. Una receta de cinco pasos ejecutada por otro es
# donde se pierde el paso 4. Acá el respaldo, la validación y la reversa van juntos o no van.
#
# NO EDITA A CIEGAS: aborta si `reverse_proxy` no aparece EXACTAMENTE una vez, y es idempotente
# (si ya hay un `@cerradas`, no toca nada).
#
#     sudo -u mon bash deploy/cerrar-metrics-del-funnel.sh
# ────────────────────────────────────────────────────────────────────────────────────────────
set -u
CF=${CADDYFILE:-/home/mon/caddy-rum/Caddyfile}
BK=$CF.antes-de-cerrar-metrics-$(date +%Y%m%d)
BASE=${BASE_OPENOBSERVE:-http://127.0.0.1:5081}
paso() { printf "\n=== %s ===\n" "$1"; }
mal()  { printf "  !! %s\n" "$1"; exit 2; }
codigo() { curl -so /dev/null -w "%{http_code}" --max-time 6 "$BASE$1"; }

paso "0 · foto de ANTES (contra esto se compara el final)"
for u in /metrics /debug/pprof/ /web/ /config; do printf "  %-16s %s\n" "$u" "$(codigo "$u")"; done
[ -r "$CF" ] || mal "no puedo leer $CF — corrélo como mon o con sudo"

paso "1 · respaldo"
cp -a "$CF" "$BK" || mal "no pude respaldar"; echo "  $BK"

paso "2 · ancla (aborta si no es inequívoca)"
grep -q "@cerradas" "$CF" && { echo "  ya aplicado. No se toca nada."; exit 0; }
N=$(grep -c "reverse_proxy" "$CF"); echo "  líneas reverse_proxy: $N"
[ "$N" -eq 1 ] || mal "esperaba EXACTAMENTE 1 reverse_proxy y hay $N — no edito a ciegas.
     Mirá el archivo y ubicá el bloque a mano:  sudo -u mon cat $CF"

paso "3 · insertar el matcher antes del reverse_proxy"
awk '
/reverse_proxy/ && !hecho {
  print "\t# A130: cerrado al lado PÚBLICO. El :5081 sale a internet por el Funnel del :8443 y"
  print "\t# /metrics contestaba 200 SIN credencial (88 series de actividad del monitoreo)."
  print "\t# Medido sobre el bundle del frontend: /metrics aparece 0 veces, /config 13 — por eso"
  print "\t# /config NO se cierra: el frontend lo pide al arrancar y taparlo rompe la UI del ERP."
  print "\t@cerradas path /metrics /debug/pprof /debug/pprof/*"
  print "\trespond @cerradas 404"
  hecho = 1
}
{ print }
' "$BK" > "$CF" || mal "falló la inserción"
diff -q "$BK" "$CF" >/dev/null && mal "el archivo NO cambió — el ancla no se aplicó"
echo "  insertado:"; diff "$BK" "$CF" | sed "s/^/    /" | head -12

paso "4 · VALIDAR antes de recargar (la red de seguridad)"
if command -v caddy >/dev/null 2>&1; then
  caddy validate --config "$CF" --adapter caddyfile >/tmp/cv.log 2>&1 \
    || { cp -a "$BK" "$CF"; tail -3 /tmp/cv.log; mal "caddy validate lo rechazó — revertido, no se recargó"; }
  echo "  validate OK"
else
  echo "  (caddy no está en el PATH de este usuario; se valida al recargar)"
fi

paso "5 · recargar"
systemctl --user restart caddy-rum.service || mal "no pude reiniciar caddy-rum.service"
sleep 3

paso "6 · verificar LAS DOS MITADES — una sola miente"
M=$(codigo /metrics); P=$(codigo /debug/pprof/); W=$(codigo /web/); C=$(codigo /config)
printf "  /metrics       %s   (esperado 404)\n" "$M"
printf "  /debug/pprof/  %s   (esperado 404)\n" "$P"
printf "  /web/          %s   (esperado 200 — ESTO es lo que no hay que romper)\n" "$W"
printf "  /config        %s   (esperado 200 — sigue abierto a propósito)\n" "$C"
if [ "$W" != "200" ]; then
  paso "REVERSA AUTOMÁTICA: el ERP dejó de responder"
  cp -a "$BK" "$CF"; systemctl --user restart caddy-rum.service; sleep 3
  printf "  /web/ tras revertir: %s\n" "$(codigo /web/)"
  mal "revertido. El respaldo quedó en $BK"
fi
[ "$M" = "404" ] && { echo; echo "  LISTO: /metrics cerrado y el ERP intacto."; } \
                 || { echo; echo "  !! /metrics sigue en $M — el matcher no tomó efecto."; }
