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

# ── 0 · LA ESPERA DE RED, QUE `After=network-online.target` NO HACE ──────────────────────────
#
# EL 2026-09-08 ESTA UNIDAD QUEDÓ `failed` A LOS 4 m 53 s DE UN ARRANQUE. El timer tiene
# `Persistent=true`, así que al encender recupera el disparo perdido; el tailnet todavía no estaba
# ruteable y el primer `ssh` murió con `Network is unreachable` en menos de un segundo. No esperó
# nada: se rindió de una.
#
# EL `After=network-online.target` DE LA UNIDAD NO PROTEGE, POR TRES MOTIVOS INDEPENDIENTES:
#   1. Es una unidad de USUARIO, y en ese manager `network-online.target` es `not-found`
#      (`systemctl --user show network-online.target -p LoadState` → not-found). Medido.
#   2. Le falta el `Wants=`. Un `After=` suelto sólo ORDENA contra algo que ya esté en la
#      transacción; nadie tira ese target, así que sería no-op aun en un manager de sistema.
#      Los otros cinco lugares del repo que lo usan escriben LAS DOS líneas.
#   3. Aunque las dos anteriores se arreglaran, seguiría sin servir: el destino es una IP de
#      tailnet (100.64/10) y `network-online.target` nunca prometió que Tailscale esté arriba.
#      En esta máquina `tailscale-wait-online.service` está *disabled*.
#
# Así que la espera se hace ACÁ, donde se puede medir y probar, y no en una directiva que se lee
# como una guarda. El default es NO esperar: una corrida a mano tiene que fallar rápido como
# siempre. La unidad opta por la espera poniendo `MUSUBI_ESPERA_RED`.
ESPERA_RED="${MUSUBI_ESPERA_RED:-0}"
if [ "$ESPERA_RED" -gt 0 ] 2>/dev/null; then
  T0="$(date +%s)"
  AVISADO=0
  until ssh -n -o BatchMode=yes -o ConnectTimeout=5 "$HOST" true 2>/dev/null; do
    if [ "$(( $(date +%s) - T0 ))" -ge "$ESPERA_RED" ]; then
      printf '✘ %s sigue sin contestar después de %ss de espera — sigo igual, para que el fallo se VEA\n' \
        "$HOST" "$ESPERA_RED" >&2
      break
    fi
    if [ "$AVISADO" -eq 0 ]; then
      printf '· %s todavía no contesta; espero hasta %ss (arranque: el tailnet tarda en levantar)\n' \
        "$HOST" "$ESPERA_RED" >&2
      AVISADO=1
    fi
    sleep 5
  done
  [ "$AVISADO" -eq 1 ] && printf '· seguí a los %ss\n' "$(( $(date +%s) - T0 ))" >&2
fi

# ── 1 · La comparación ──────────────────────────────────────────────────────────────────────
# La salida se muestra ENTERA: este guion no resume nada. Un resumen es una segunda opinión sobre
# lo que el verificador ya dijo, y dos fuentes de verdad sobre la misma pregunta se pudren.
# LA REFERENCIA VIAJA POR ARCHIVO, NO POR LA SALIDA. El verificador escribe acá contra qué árbol
# comparó; se hace `source` y no se parsea nada. Ver el bloque del canal en verificar-despliegue.sh.
REF_ENV="$(mktemp)"
trap 'rm -f "$REF_ENV"' EXIT
MUSUBI_REF_SALIDA="$REF_ENV" MUSUBI_SSH="$HOST" "$REPO/deploy/verificar-despliegue.sh"
CODIGO=$?

# `REF_CONFIABLE` vacío significa QUE NO SE SUPO, y no «no era confiable»: un verificador viejo no
# escribe el archivo. Los dos son distintos y la diferencia decide si la serie sale o no sale —
# la regla del export de este repo: si el valor es DESCONOCIDO, la línea NO SE EMITE. Un 0
# inventado sería indistinguible de un 0 medido, que es la alerta perdida de siempre.
REF_CONFIABLE=""
if [ -s "$REF_ENV" ]; then
  # shellcheck disable=SC1090
  . "$REF_ENV" 2>/dev/null || REF_CONFIABLE=""
fi

# ── 1 bis · LA OTRA COMPROBACIÓN, QUE NO LA CORRÍA NADIE ────────────────────────────────────
#
# `verificar-cobertura.sh` es el hermano de `verificar-despliegue.sh` y contesta la pregunta de un
# nivel más adentro: aquél dice «¿está la regla cargada?» y éste «¿esa regla vigila a ESTA
# máquina?». La diferencia se midió el 2026-09-02, con las 35 reglas desplegadas y todas sus
# métricas presentes: 13 de 19 dimensiones en las Windows.
#
# SU HERMANO GANÓ TIMER, LATIDO Y DEAD-MAN CON A115; ÉL QUEDÓ EN «CUANDO ALGUIEN SE ACUERDE», que
# es exactamente la condición que A115 existió para eliminar. Y «cuando alguien se acuerde» no es
# una cadencia baja: es CERO, porque nadie se acuerda de correr un guion que no falla.
#
# Corre ACÁ y no en un timer propio: la comparación ya abre las sesiones ssh y ya consulta
# Prometheus. Un segundo timer sería una segunda cosa que puede quedar sin `enable-linger`, sin
# `Persistent`, o simplemente sin instalar — o sea un segundo silencio posible por ninguna ventaja.
#
# SU CÓDIGO VIAJA APARTE Y NO SE FUSIONA CON EL DE ARRIBA. Son dos preguntas distintas y se
# arreglan distinto: un 1 de allá es «producción diverge» y un 1 de acá es «hay máquinas con
# dimensiones sin vigilar». Fusionarlos con un `||` daría un número que no dice cuál de las dos
# falló, que es el defecto que este repo persigue con nombre propio.
printf '\n'
MUSUBI_SSH="$HOST" "$REPO/deploy/verificar-cobertura.sh"
CODIGO_COBERTURA=$?

# ── 2 · El latido ───────────────────────────────────────────────────────────────────────────
# `service.name` → label `job` y `service.instance.id` → label `instance`, que es como el receptor
# OTLP de Prometheus mapea los atributos de recurso. La instancia es el HOSTNAME de quien comparó:
# si mañana comparan dos máquinas, se ven las dos y se sabe cuál se calló.
AHORA="$(date +%s)"
QUIEN="$(hostname -s 2>/dev/null || echo desconocido)"

# `timeUnixNano` va como STRING. Mandarlo como número hace que Prometheus conteste 400 y el empuje
# muera en silencio con la configuración perfecta — está medido y escrito en internal/mcp/fleet_otlp.go.
NANOS="${AHORA}000000000"

# AUSENTE cuando no se supo, presente cuando sí. Se arma como fragmento porque el sobre es un
# heredoc y meterle un `if` adentro obligaría a escribir el JSON dos veces — que es la forma en la
# que este contrato ya perdió campos una vez.
METRICA_REF=""
if [ -n "$REF_CONFIABLE" ]; then
  # SE ARMA CON HEREDOC Y NO CON UNA CADENA CON COMILLAS ESCAPADAS, por el mismo motivo que el
  # sobre de abajo. `TestNingunBloqueDePowerShellEscapaComillasConBarra` delimita los bloques de
  # PowerShell por una comilla doble pegada a una simple, y el `trap` de arriba produce ese par al
  # cerrar; desde ahi la guarda lee como PowerShell todo el bash que sigue. Una comilla escapada
  # con barra aca caeria adentro de ese bloque falso. El heredoc no lleva escapes y ademas se lee
  # igual que el JSON que produce.
  METRICA_REF="$(cat <<JSONREF
,
  {"name":"musubi_verificacion_referencia_confiable",
   "description":"1 si se comparo contra origin/main limpio y recien traido, 0 si el arbol no era esa referencia. AUSENTE si el verificador no lo dijo",
   "gauge":{"dataPoints":[{"timeUnixNano":"$NANOS","asDouble":$REF_CONFIABLE}]}}
JSONREF
)"
fi
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
   "gauge":{"dataPoints":[{"timeUnixNano":"$NANOS","asDouble":$CODIGO}]}},
  {"name":"musubi_verificacion_cobertura_ultima_seconds",
   "description":"Reloj de pared de la ultima verificacion de cobertura por maquina",
   "gauge":{"dataPoints":[{"timeUnixNano":"$NANOS","asDouble":$AHORA}]}},
  {"name":"musubi_verificacion_tls_exigido",
   "description":"1 si esta corrida exigio TLS (MUSUBI_EXIGIR_TLS=1), 0 si acepto el transporte como este",
   "gauge":{"dataPoints":[{"timeUnixNano":"$NANOS","asDouble":${MUSUBI_EXIGIR_TLS:-0}}]}},
  {"name":"musubi_verificacion_cobertura_resultado",
   "description":"0 toda dimension aplicable esta vigilada, 1 hay huecos, 2 no se pudo medir",
   "gauge":{"dataPoints":[{"timeUnixNano":"$NANOS","asDouble":$CODIGO_COBERTURA}]}}$METRICA_REF]}]}]}
JSON
)"

# El código HTTP se mira. Un POST que devuelve 400 y un POST que entrega salen los dos con 0 en
# `curl` si no se pide otra cosa, y confundirlos sería el mismo defecto de un piso más arriba.
HTTP="$(printf '%s' "$PAYLOAD" | ssh -o BatchMode=yes -o ConnectTimeout=10 "$HOST" \
  "curl -sS -m 15 -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' --data-binary @- $PROM_URL/api/v1/otlp/v1/metrics" 2>/dev/null)"

case "$HTTP" in
  2*)
    printf '\n\033[32m✔ latido empujado\033[0m — despliegue=%s cobertura=%s desde %s (%s)\n' "$CODIGO" "$CODIGO_COBERTURA" "$QUIEN" "$(date -d "@$AHORA" '+%Y-%m-%d %H:%M:%S')"
    ;;
  "")
    printf '\n\033[31m✘ el latido NO se empujó: no hubo respuesta de %s\033[0m\n' "$HOST" >&2
    # EL TEXTO SE BIFURCA POR EL CÓDIGO, Y NO ES UN DETALLE DE REDACCIÓN.
    #
    # Este mensaje estaba escrito para UN caso —la comparación anduvo y sólo falló el POST— y se
    # imprimía FIJO también cuando el corte era aguas arriba. El 2026-09-08 el guion afirmó «en
    # verdad sí se comparó y el resultado fue 2» en la corrida donde no comparó NADA: los mismos
    # `ssh` que no llegaron al POST tampoco habían llegado a Prometheus. Seis renglones más arriba
    # decía «no se contaron las reglas», «no se miraron los targets», «Prometheus no contestó».
    #
    # Quien leyera el journal cuando sonara la alerta iba a buscar por qué el verificador quedó en
    # 2 —o sea, un problema de despliegue— en vez de por qué se cortó la red. Un guion que se
    # contradice a sí mismo en la misma salida manda a investigar la pista equivocada.
    if [ "$CODIGO" -eq 2 ]; then
      printf '  Y el MISMO corte se llevó puesta la comparación: el resultado %s de acá NO significa\n' "$CODIGO" >&2
      printf '  «producción diverge», significa «no se pudo preguntar». Arriba está, renglón por\n' >&2
      printf '  renglón, qué quedó sin mirar. No hay nada que reportar todavía: hay que volver a\n' >&2
      printf '  correrlo con red antes de sacar cualquier conclusión sobre el despliegue.\n' >&2
    else
      printf '  Es un fallo REAL y no cosmético: sin latido, `ComparacionRepoServidorSinCorrer` va a\n' >&2
      printf '  disparar en unas horas diciendo que nadie comparó, cuando en verdad sí se comparó y el\n' >&2
      printf '  resultado fue %s.\n' "$CODIGO" >&2
    fi
    printf '  Probá:  ssh %s curl -sS %s/-/ready\n' "$HOST" "$PROM_URL" >&2
    ;;
  *)
    printf '\n\033[31m✘ el latido NO se empujó: Prometheus contestó HTTP %s\033[0m\n' "$HTTP" >&2
    printf '  Un 404 es que falta `--web.enable-otlp-receiver`; un 400 es el sobre OTLP mal armado\n' >&2
    printf '  (casi siempre `timeUnixNano` como número en vez de string — ver internal/mcp/fleet_otlp.go).\n' >&2
    ;;
esac

exit "$CODIGO"
