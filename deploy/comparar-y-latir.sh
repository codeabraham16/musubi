#!/usr/bin/env bash
#
# comparar-y-latir.sh — corre `verificar-despliegue.sh` y EMPUJA un latido con su resultado (A115).
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# POR QUÉ EXISTE
#
# `verificar-despliegue.sh` es la ÚNICA defensa que caza «arreglado en el repo, nunca llegado a la
# máquina», y hasta el 2026-09-05 corría sólo cuando un humano se acordaba. Medido ese día: cero
# menciones en `.github/`, cero timers y cero entradas de cron, en las dos máquinas. Mientras
# tanto las reglas de flota llevaban desde el 4-09 desplegadas a medias —24 de 27— con su alarma
# en VERDE, y el guion del redespliegue llevaba desde el 2-09 a la mitad del del repo con su
# verificación de la migración muerta. Los dos aparecieron en la MISMA corrida, y esa corrida salió
# de casualidad.
#
# LO QUE ESTE GUION AGREGA NO ES CORRERLO: ES QUE SE NOTE CUANDO NO SE CORRIÓ.
#
# Un timer que se apaga no avisa. Poner la comparación en un timer y nada más movería el silencio
# de lugar: en vez de «nadie se acordó» sería «el timer está muerto», igual de invisible. Por eso
# cada corrida empuja dos gauges y hay alertas que miran ESO. El latido lo evalúa el Prometheus del
# servidor, o sea que si la máquina que corre esto se apaga, suena allá — que es la diferencia
# entre un dead-man y el vigilante adentro del cajón (B13).
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ EL TIMESTAMP VIAJA EXPLÍCITO Y NO SE CALCULA CON `timestamp()`
#
# La tentación es empujar sólo el resultado y preguntar la frescura con
# `time() - timestamp(last_over_time(musubi_verificacion_despliegue_resultado[7d]))`. NO FUNCIONA,
# y este repo ya lo pagó: `timestamp()` devuelve la hora de EVALUACIÓN de la muestra, no la hora en
# que el dato entró, así que una serie muerta contesta «hace 0 minutos» para siempre. Un
# dead-man construido sobre eso nunca dispara — es exactamente el modo de falla que este guion
# viene a cerrar, un piso más abajo. Por eso `musubi_verificacion_despliegue_ultima_seconds` lleva
# el reloj adentro del valor.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# EL LATIDO SE EMPUJA AUNQUE EL VERIFICADOR FALLE, Y ÉSA ES LA MITAD QUE IMPORTA
#
# Si sólo latiera cuando todo está bien, «divergió» y «no corrió» serían la misma señal y se
# arreglan distinto. El código de salida del verificador viaja EN el latido:
#
#   0 → coincide          1 → produccion diverge          2 → quedaron eslabones sin verificar
#
# Uso:
#   MUSUBI_SSH=musubi-server ./deploy/comparar-y-latir.sh
#
# Variables:
#   MUSUBI_SSH=<host>   REQUERIDA. El host del cerebro: por ahí sale la comparación y por ahí
#                       entra el latido (Prometheus escucha SÓLO en loopback, a propósito).
#   PROM_URL=<url>      por defecto http://127.0.0.1:9099, resuelta EN el servidor.
#
# Sale con el código del verificador, no con el del empuje: lo que este guion reporta es el estado
# del despliegue. Que el empuje falle se dice por stderr y se cuenta aparte — si se lo tragara,
# el dead-man dispararía sin que nadie sepa por qué.
# ════════════════════════════════════════════════════════════════════════════════════════════
set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOST="${MUSUBI_SSH:-}"
PROM_URL="${PROM_URL:-http://127.0.0.1:9099}"

if [ -z "$HOST" ]; then
  printf 'falta MUSUBI_SSH: hace falta el host del cerebro para comparar y para empujar el latido.\n' >&2
  printf 'Ej:  MUSUBI_SSH=musubi-server %s\n' "$0" >&2
  exit 2
fi
if [ ! -x "$REPO/deploy/verificar-despliegue.sh" ]; then
  printf 'no encuentro deploy/verificar-despliegue.sh ejecutable en %s\n' "$REPO" >&2
  exit 2
fi

# ── 1 · La comparación ──────────────────────────────────────────────────────────────────────
# La salida se muestra ENTERA: este guion no resume nada. Un resumen es una segunda opinión sobre
# lo que el verificador ya dijo, y dos fuentes de verdad sobre la misma pregunta se pudren.
MUSUBI_SSH="$HOST" "$REPO/deploy/verificar-despliegue.sh"
CODIGO=$?

# ── 2 · El latido ───────────────────────────────────────────────────────────────────────────
# `service.name` → label `job` y `service.instance.id` → label `instance`, que es como el receptor
# OTLP de Prometheus mapea los atributos de recurso. La instancia es el HOSTNAME de quien comparó:
# si mañana comparan dos máquinas, se ven las dos y se sabe cuál se calló.
AHORA="$(date +%s)"
QUIEN="$(hostname -s 2>/dev/null || echo desconocido)"

# `timeUnixNano` va como STRING. Mandarlo como número hace que Prometheus conteste 400 y el empuje
# muera en silencio con la configuración perfecta — está medido y escrito en internal/mcp/fleet_otlp.go.
NANOS="${AHORA}000000000"
PAYLOAD="$(cat <<JSON
{"resourceMetrics":[{"resource":{"attributes":[
  {"key":"service.name","value":{"stringValue":"musubi-verificador"}},
  {"key":"service.instance.id","value":{"stringValue":"$QUIEN"}}]},
 "scopeMetrics":[{"scope":{"name":"musubi/verificador"},"metrics":[
  {"name":"musubi_verificacion_despliegue_ultima_seconds",
   "description":"Reloj de pared de la ultima comparacion repo-servidor",
   "gauge":{"dataPoints":[{"timeUnixNano":"$NANOS","asDouble":$AHORA}]}},
  {"name":"musubi_verificacion_despliegue_resultado",
   "description":"0 coincide, 1 diverge, 2 quedaron eslabones sin verificar",
   "gauge":{"dataPoints":[{"timeUnixNano":"$NANOS","asDouble":$CODIGO}]}}]}]}]}
JSON
)"

# El código HTTP se mira. Un POST que devuelve 400 y un POST que entrega salen los dos con 0 en
# `curl` si no se pide otra cosa, y confundirlos sería el mismo defecto de un piso más arriba.
HTTP="$(printf '%s' "$PAYLOAD" | ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" \
  "curl -sS -m 15 -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' --data-binary @- $PROM_URL/api/v1/otlp/v1/metrics" 2>/dev/null)"

case "$HTTP" in
  2*)
    printf '\n\033[32m✔ latido empujado\033[0m — resultado=%s desde %s (%s)\n' "$CODIGO" "$QUIEN" "$(date -d "@$AHORA" '+%Y-%m-%d %H:%M:%S')"
    ;;
  "")
    printf '\n\033[31m✘ el latido NO se empujó: no hubo respuesta de %s\033[0m\n' "$HOST" >&2
    printf '  Es un fallo REAL y no cosmético: sin latido, `ComparacionRepoServidorSinCorrer` va a\n' >&2
    printf '  disparar en unas horas diciendo que nadie comparó, cuando en verdad sí se comparó y el\n' >&2
    printf '  resultado fue %s. Probá:  ssh %s curl -sS %s/-/ready\n' "$CODIGO" "$HOST" "$PROM_URL" >&2
    ;;
  *)
    printf '\n\033[31m✘ el latido NO se empujó: Prometheus contestó HTTP %s\033[0m\n' "$HTTP" >&2
    printf '  Un 404 es que falta `--web.enable-otlp-receiver`; un 400 es el sobre OTLP mal armado\n' >&2
    printf '  (casi siempre `timeUnixNano` como número en vez de string — ver internal/mcp/fleet_otlp.go).\n' >&2
    ;;
esac

exit "$CODIGO"
