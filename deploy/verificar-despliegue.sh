#!/usr/bin/env bash
# verificar-despliegue.sh — compara lo que el repo DICE contra lo que el servidor CORRE (A73).
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# POR QUÉ EXISTE
#
# `TestLaCadenaDeAlertasSeVigilaASiMisma` pasaba en verde mientras el job `alertmanager` no estaba
# desplegado. Sin ese scrape, `alertmanager_notifications_failed_total` no existe y
# `CadenaDeAlertasFallando` —la alerta que vigila que las alertas se entreguen— no podía dispararse
# NUNCA. La guarda leía `deploy/prometheus/prometheus.yml`; el servidor corría otro archivo.
#
# Medido el 2026-09-02: 29 reglas cargadas contra 31 en el repo. Las dos que faltaban eran
# justamente `CadenaDeAlertasFallando` y `MaquinaQueNoAlcanzaSuDestino`.
#
# Las pruebas de Go no pueden cerrar eso: no tienen el servidor delante. Éste sí, y pregunta por
# las APIs —no por los archivos—, porque un archivo correcto que Prometheus no releyó se ve igual
# que uno bueno. Es exactamente el error que costó una hora el 2026-08-31: un `sed -i` sobre un
# bind-mount cambió el inodo, el contenedor siguió leyendo el archivo viejo, y la recarga contestó
# 200 — honesta y perfectamente inútil.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# LO QUE ESTE SCRIPT NO MIRA, DICHO ACÁ Y NO DESCUBIERTO DESPUÉS
#
#   · El CONTENIDO de cada regla. Compara nombres y cantidades: una alerta cuyo umbral cambió en
#     el repo y no en producción tiene el mismo nombre y no se ve desde acá.
#   · Los archivos del servidor. Se le pregunta a Prometheus qué CARGÓ, que es la única respuesta
#     que importa; un archivo correcto sin recargar no se distingue de uno viejo, y así tiene que
#     ser.
#   · Los scrapes de sitio (`/etc/prometheus/scrapes/*.yml`). Son por sitio a propósito y el repo
#     sólo trae el `.ejemplo`, así que no hay contra qué compararlos.
#   · Lo que corre en las máquinas de la flota. Eso lo dice `musubi_fleet_device_agent_stale` (A68).
#   · Que el mensaje LLEGUE. La sección «cadena de alertas» comprueba que cada eslabón conteste y
#     que Alertmanager tenga rutas; que Telegram reciba el mensaje sólo lo prueba el watchdog
#     externo, y decir «cadena viva» por esto sería el mismo error de un piso más arriba.
#
# Un informe que calla lo que no mira se lee como si lo hubiera mirado. Es el mismo hallazgo de
# A66, y por eso esta lista está arriba y no al final.
# ════════════════════════════════════════════════════════════════════════════════════════════
#
# Uso:
#   ./deploy/verificar-despliegue.sh                    # corriendo EN el servidor
#   MUSUBI_SSH=musubi-server ./deploy/verificar-despliegue.sh   # desde afuera
#
# Variables, todas opcionales y todas explícitas a propósito:
#   MUSUBI_SSH=<host>          pregunta por ssh en vez de por la red (Prometheus es loopback)
#   PROM_URL=<url>             por defecto http://127.0.0.1:9099  (9090 es Cockpit, ver abajo)
#   ALERT_URL=<url>            por defecto http://127.0.0.1:9093
#   MUSUBI_TLS_INSECURE=1      acepta un certificado propio. NO está por defecto: aceptar sin
#                              mirar convierte «no sé con quién hablo» en un verde.
#   MUSUBI_HTTP_BEARER=<tok>   token, si el endpoint pide credencial. Viaja por stdin de curl,
#   MUSUBI_HTTP_USUARIO=<u>    no por la línea de comandos, para que no quede en la tabla de
#   MUSUBI_HTTP_CLAVE=<c>      procesos ni en el historial del shell.
#
# Sale 0 si todo coincide y todo se pudo preguntar, 1 si hay divergencia, y 2 si algo quedó SIN
# VERIFICAR — que no es lo mismo que estar bien, y por eso no comparte código con el verde.

set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROM_URL="${PROM_URL:-http://127.0.0.1:9099}"
ALERT_URL="${ALERT_URL:-http://127.0.0.1:9093}"
SSH_HOST="${MUSUBI_SSH:-}"

# ESTE SCRIPT COMPARA CONTRA EL REPO, así que sin el repo no puede contestar nada. Se dice acá y
# se corta: un informe que sale en verde porque no encontró con qué comparar es peor que ninguno.
if [ ! -f "$REPO/VERSION" ] || [ ! -d "$REPO/deploy" ]; then
  printf 'no encuentro el repositorio en %s.\n' "$REPO" >&2
  printf 'Corrélo desde el árbol del repo: MUSUBI_SSH=musubi-server ./deploy/verificar-despliegue.sh\n' >&2
  exit 2
fi

# Todo lo que se compara sale de leer JSON, y eso lo hace python3. Sin él, cada chequeo devolvería
# vacío y el vacío se leería como «no hay nada mal». Se corta acá, con el nombre de lo que falta.
if ! command -v python3 >/dev/null 2>&1; then
  printf 'falta python3 en esta máquina: es lo que lee las respuestas de las APIs.\n' >&2
  printf 'Instalalo (apt install python3) o corré el script en el servidor.\n' >&2
  exit 2
fi

rojo()  { printf '  \033[31m✘ %s\033[0m\n' "$1"; DIVERGE=1; }
verde() { printf '  \033[32m✔ %s\033[0m\n' "$1"; }
gris()  { printf '  \033[90m· %s\033[0m\n' "$1"; }
titulo(){ printf '\n\033[1m%s\033[0m\n' "$1"; }
# dudoso — lo que NO SE PUDO comprobar. No es verde ni rojo: es «no vi», y sale con 2. Existe
# porque el modo de falla que trajo este script hasta acá es siempre el mismo: una consulta que no
# se pudo hacer y un informe que igual terminó en verde.
dudoso(){ printf '  \033[33m? %s\033[0m\n' "$1"; SIN_VERIFICAR=1; }
DIVERGE=0
SIN_VERIFICAR=0

# ── CÓMO SE PREGUNTA ────────────────────────────────────────────────────────────────────────
# Un GET devuelve TRES cosas y las tres hacen falta: el cuerpo, el código HTTP y el error de curl.
# «no contestó», «contestó 401» y «contestó un HTML de otro servicio» son fallas distintas, se
# arreglan distinto, y un script que sólo mira el cuerpo las confunde a las tres con «vacío».
CUERPO="$(mktemp)"; CUERPO_TMP="$(mktemp)"; CURL_ERR="$(mktemp)"
trap 'rm -f "$CUERPO" "$CUERPO_TMP" "$CURL_ERR"' EXIT
HTTP_CODIGO=000

# La config de curl viaja por STDIN (`curl -K -`), no por la línea de comandos: si alguien exporta
# un token, no queda en la tabla de procesos del servidor ni en el historial del shell.
config_curl() {
  printf 'silent\nshow-error\nmax-time = 15\n'
  # Certificado propio: se acepta SÓLO si el operador lo pidió. Por defecto, un certificado que no
  # valida es un ROJO con su razón, no un verde con una advertencia que nadie lee.
  [ "${MUSUBI_TLS_INSECURE:-0}" = "1" ] && printf 'insecure\n'
  if [ -n "${MUSUBI_HTTP_BEARER:-}" ]; then
    printf 'header = "Authorization: Bearer %s"\n' "$MUSUBI_HTTP_BEARER"
  elif [ -n "${MUSUBI_HTTP_USUARIO:-}" ]; then
    printf 'user = "%s:%s"\n' "$MUSUBI_HTTP_USUARIO" "${MUSUBI_HTTP_CLAVE:-}"
  fi
  return 0
}

# pedir_http <url> — deja el cuerpo en $CUERPO, el código en $HTTP_CODIGO (000 si no hubo
# respuesta) y lo que dijo curl en $CURL_ERR.
pedir_http() {
  local url="$1"
  : >"$CUERPO"; : >"$CURL_ERR"
  if [ -n "$SSH_HOST" ]; then
    config_curl | ssh -o BatchMode=yes -o ConnectTimeout=10 "$SSH_HOST" \
      "curl -K - -w '\n%{http_code}' $(printf '%q' "$url")" >"$CUERPO" 2>"$CURL_ERR"
  else
    config_curl | curl -K - -w '\n%{http_code}' "$url" >"$CUERPO" 2>"$CURL_ERR"
  fi
  HTTP_CODIGO="$(tail -n1 "$CUERPO" | tr -dc '0-9')"
  : "${HTTP_CODIGO:=000}"
  # El código viaja pegado al final del cuerpo (`-w`); se lo saca para que el cuerpo siga siendo
  # JSON válido.
  sed '$d' "$CUERPO" >"$CUERPO_TMP" && cat "$CUERPO_TMP" >"$CUERPO"
  return 0
}

# pedir <url> — la forma vieja, para los bloques que sólo quieren el cuerpo. Pasa por pedir_http
# para que el TLS y las credenciales valgan también acá.
pedir() { pedir_http "$1"; cat "$CUERPO"; }

# ¿Contestó una PÁGINA WEB en vez de una API? Es EL síntoma del 9090: Cockpit vive ahí en casi
# todos los servidores Linux, contesta 200 con una UI, y quien la ve cree que Prometheus anda.
parece_html() { head -c 400 "$CUERPO" | grep -qiE '<!doctype html|<html|<head'; }

# Por qué no hubo respuesta, en las palabras de curl y con qué hacer al respecto.
razon_muda() {
  local err
  # UNA sola línea, porque curl agrega tres de contexto para el certificado y un mensaje de cinco
  # renglones se deja de leer entero. Pero NO la primera a secas: por el camino MUSUBI_SSH el
  # stderr de ssh y el de curl caen mezclados en el mismo archivo, y ssh escribe primero
  # («Warning: Permanently added ... to the list of known hosts.» sale en toda conexión nueva).
  # Esa línea no tiene ninguna de las palabras del case, así que un certificado inválido caería en
  # la rama genérica y al operador se le mostraría el warning de known_hosts como si fuera la razón.
  # Por eso: la línea de curl si la hay, y recién si no, la primera no vacía (un fallo puramente de
  # ssh —«Permission denied (publickey)»— sigue reportándose).
  err="$(grep -m1 '^curl:' "$CURL_ERR" 2>/dev/null || true)"
  [ -z "$err" ] && err="$(grep -m1 . "$CURL_ERR" 2>/dev/null || true)"
  err="$(printf '%s' "$err" | tr -d '\r' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')"
  case "$err" in
    *ertificate*|*SSL*|*TLS*|*elf-signed*|*elf\ signed*)
      printf 'el certificado no valida (%s). Si es el certificado propio del servidor y lo esperabas, repetí con MUSUBI_TLS_INSECURE=1; si no lo esperabas, hay otra cosa contestando ahí' "$err" ;;
    "") printf 'sin respuesta y sin error de curl (timeout o puerto cerrado)' ;;
    *)  printf '%s' "$err" ;;
  esac
}

printf '\033[1mverificar-despliegue\033[0m — repo %s contra %s\n' "$REPO" "${SSH_HOST:-127.0.0.1}"

# ── 1 · LA CADENA DE ALERTAS, ESLABÓN POR ESLABÓN ───────────────────────────────────────────
# Va PRIMERO a propósito. Si Prometheus no contesta, las comparaciones de abajo no pueden decir
# nada y el script corta; que corte DESPUÉS de haber nombrado el eslabón roto es la diferencia
# entre un diagnóstico y un «no se pudo consultar» a secas.
#
# Medido el 2026-09-02 desde la laptop: el 9093 no respondía y el 9090 devolvía un HTML. Las dos
# cosas son ciertas y ninguna es una falla del servidor — el 9093 es loopback A PROPÓSITO (la API
# de Alertmanager silencia alertas sin credencial: exponerla sería repartir el botón de apagado) y
# el 9090 es Cockpit, no Prometheus, que escucha en el 9099. Este bloque las dice con esas
# palabras, en vez de dejar un timeout suelto para que alguien lo interprete a las 3 de la mañana.
#
# REGLA DEL BLOQUE: nada acá pasa a verde por silencio. Lo que no se pudo preguntar sale amarillo
# («?») y el script termina en 2. Un chequeo que no distingue «está bien» de «no miré» es el
# agujero que este archivo entero existe para tapar.
titulo "cadena de alertas"

# Lo que el repo declara, UNA vez: sirve para el conteo de acá y para las huérfanas de la sección 2.
TODAS_DECLARADAS="$(cat "$REPO"/deploy/musubi-alerts*.yml \
  | grep -E '^[[:space:]]*-[[:space:]]+alert:' | sed -E 's/.*alert:[[:space:]]*//' | sort -u)"
N_DECLARADAS="$(printf '%s\n' "$TODAS_DECLARADAS" | grep -c . || true)"
# Las de los archivos que se declaran `# despliegue: siempre`. Son las que no admiten excusa: las
# condicionales pueden faltar con razón, y mezclarlas haría que el conteo denuncie una decisión.
OBLIGATORIAS="$(for f in "$REPO"/deploy/musubi-alerts*.yml; do
  case "$(sed -n 's/^#[[:space:]]*despliegue:[[:space:]]*//p' "$f" | head -1)" in
    siempre) grep -E '^[[:space:]]*-[[:space:]]+alert:' "$f" | sed -E 's/.*alert:[[:space:]]*//' ;;
  esac
done | sort -u | grep . || true)"
N_OBLIGATORIAS="$(printf '%s\n' "$OBLIGATORIAS" | grep -c . || true)"

# (a) ¿PROMETHEUS ESTÁ AHÍ, Y ES PROMETHEUS? ─────────────────────────────────────────────────
PROM_VIVO=no
pedir_http "$PROM_URL/-/ready"
if parece_html; then
  rojo "en $PROM_URL contesta una PÁGINA WEB, no Prometheus (HTTP $HTTP_CODIGO). En el 9090 vive Cockpit en casi todos los servidores Linux: se ve una UI y parece que anda. Prometheus escucha en 127.0.0.1:9099 — usá MUSUBI_SSH=musubi-server, o abrí el túnel: ssh -N -L 9099:127.0.0.1:9099 -L 9093:127.0.0.1:9093 usuario@musubi-server"
else
  case "$HTTP_CODIGO" in
    200) verde "Prometheus responde y está listo — $PROM_URL/-/ready"; PROM_VIVO=si ;;
    401|403)
      dudoso "Prometheus pide credencial (HTTP $HTTP_CODIGO) y no hay ninguna exportada: exportá MUSUBI_HTTP_BEARER=<token> —o MUSUBI_HTTP_USUARIO/MUSUBI_HTTP_CLAVE— y repetí. Hasta entonces la cadena queda SIN VERIFICAR, que no es lo mismo que estar bien" ;;
    503)
      rojo "Prometheus contesta pero NO está listo (503 en /-/ready): está arrancando o releyendo la config. Nada de lo de abajo es concluyente hasta que dé 200; esperá un minuto y repetí" ;;
    000)
      rojo "$PROM_URL no contestó — $(razon_muda). Prometheus escucha SÓLO en loopback: corré esto en el servidor, o con MUSUBI_SSH=musubi-server, o abrí el túnel ssh -N -L 9099:127.0.0.1:9099 -L 9093:127.0.0.1:9093 usuario@musubi-server" ;;
    *)
      rojo "$PROM_URL/-/ready devolvió HTTP $HTTP_CODIGO, que no es «listo». Confirmá que PROM_URL apunte a Prometheus (127.0.0.1:9099) y no a otro servicio" ;;
  esac
fi

# (a bis) ¿CUÁNTAS REGLAS TIENE CARGADAS, contra las que el repo declara? ────────────────────
# Se le pregunta a la API por lo que CARGÓ, no a los archivos del disco: un archivo correcto que
# Prometheus no releyó se ve idéntico a uno bueno. La foto que sale de acá es la que usa toda la
# sección 2, para no comparar contra dos estados distintos del servidor.
REGLAS_JSON=""
CARGADAS=""
if [ "$PROM_VIVO" != si ]; then
  dudoso "no se contaron las reglas cargadas: Prometheus no contestó (arriba está por qué)"
else
  pedir_http "$PROM_URL/api/v1/rules"
  REGLAS_JSON="$(cat "$CUERPO")"
  CARGADAS="$(printf '%s' "$REGLAS_JSON" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]["groups"]
for g in d:
    for r in g["rules"]:
        if r.get("type") == "alerting":
            print(r["name"])
' 2>/dev/null | sort -u)"
  if [ "$HTTP_CODIGO" != 200 ] || [ -z "$CARGADAS" ]; then
    REGLAS_JSON=""
    rojo "$PROM_URL/api/v1/rules devolvió HTTP $HTTP_CODIGO y ninguna regla legible: Prometheus está vivo y NO tiene alertas cargadas, o la respuesta no es la que se espera. Mirala a mano: curl -s $PROM_URL/api/v1/rules | head -c 300"
  else
    N_CARGADAS="$(printf '%s\n' "$CARGADAS" | grep -c . || true)"
    faltan_obl="$(comm -23 <(printf '%s\n' "$OBLIGATORIAS" | grep . || true) <(printf '%s\n' "$CARGADAS" | grep . || true))"
    n_faltan_obl="$(printf '%s\n' "$faltan_obl" | grep -c . || true)"
    if [ "$n_faltan_obl" -eq 0 ]; then
      verde "$N_CARGADAS reglas cargadas; el repo declara $N_DECLARADAS y las $N_OBLIGATORIAS que se despliegan «siempre» están todas (el resto es condicional; el detalle, en la sección 2)"
    else
      rojo "faltan $n_faltan_obl de las $N_OBLIGATORIAS reglas que se despliegan «siempre» (hay $N_CARGADAS cargadas contra $N_DECLARADAS declaradas):"
      printf '%s\n' "$faltan_obl" | sed 's/^/      falta: /'
    fi
  fi
fi

# (a ter) LAS TRES QUE VIGILAN LA CADENA MISMA ───────────────────────────────────────────────
# El tablero puede estar entero y aun así nadie enterarse cuando la cadena se corta: estas tres son
# la única red debajo de la red. Que existan en el repo no prueba nada —el hallazgo es exactamente
# que nadie podía confirmar que estuvieran CARGADAS—, así que se pregunta por cada una.
#
# Dos viven en `musubi-alerts-flota.yml`, que es condicional: se despliega si el cerebro expone
# `musubi_fleet_*`. Esa condición se puede RESOLVER preguntándole a Prometheus por la métrica, y
# resolverla convierte un «no sé» en un rojo o en un verde. Si tampoco se puede resolver, sale
# amarillo: «no cargada, y no pude saber si correspondía» es una respuesta honesta; «verde» no.
HAY_FLOTA=indeterminado
if [ "$PROM_VIVO" = si ]; then
  pedir_http "$PROM_URL/api/v1/query?query=count(musubi_fleet_device_up)"
  if [ "$HTTP_CODIGO" = 200 ]; then
    HAY_FLOTA="$(python3 -c '
import sys, json
try:
    r = json.load(sys.stdin)["data"]["result"]
except Exception:
    print("indeterminado"); sys.exit(0)
print("si" if r else "no")
' <"$CUERPO" 2>/dev/null)"
    : "${HAY_FLOTA:=indeterminado}"
  fi
fi

if [ -n "$CARGADAS" ]; then
  for centinela in CadenaDeAlertasFallando FlotaSinTelemetria ReglasDelCerebroSinDesplegar; do
    archivo="$(grep -lE "^[[:space:]]*-[[:space:]]+alert:[[:space:]]*$centinela\$" "$REPO"/deploy/musubi-alerts*.yml | head -1)"
    if printf '%s\n' "$CARGADAS" | grep -qx "$centinela"; then
      verde "$centinela — cargada y evaluándose"
    elif [ -z "$archivo" ]; then
      rojo "$centinela no está cargada Y el repo ya no la declara: la cadena perdió una de sus tres guardias y nadie lo va a avisar"
    elif ! grep -q '^#[[:space:]]*despliegue:[[:space:]]*condicional' "$archivo"; then
      rojo "$centinela NO está cargada, y $(basename "$archivo") se despliega «siempre»: la vigilancia de la cadena está incompleta en producción"
    elif [ "$HAY_FLOTA" = si ]; then
      rojo "$centinela NO está cargada y su condición SÍ se cumple (el cerebro expone musubi_fleet_*): $(basename "$archivo") quedó sin desplegar"
    elif [ "$HAY_FLOTA" = no ]; then
      gris "$centinela sin cargar, y así corresponde: $(basename "$archivo") es condicional y el cerebro no expone musubi_fleet_*"
    else
      dudoso "$centinela no está cargada y no pude resolver si correspondía ($(basename "$archivo") es condicional y no pude preguntar por musubi_fleet_device_up)"
    fi
  done
else
  dudoso "no se pudo confirmar si CadenaDeAlertasFallando, FlotaSinTelemetria y ReglasDelCerebroSinDesplegar están cargadas: no hay lista de reglas que mirar"
fi

# (b) LOS TARGETS: quién está siendo scrapeado y quién no ────────────────────────────────────
# Un target caído no es «una métrica menos»: toda alerta que dependa de él deja de poder disparar,
# y en la UI eso se ve igual que «todo tranquilo».
if [ "$PROM_VIVO" != si ]; then
  dudoso "no se miraron los targets: Prometheus no contestó"
else
  pedir_http "$PROM_URL/api/v1/targets?state=active"
  TARGETS="$(python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
act = d.get("activeTargets") or []
mal = [t for t in act if t.get("health") != "up"]
print("%d %d" % (len(act) - len(mal), len(act)))
for t in mal:
    l = t.get("labels") or {}
    print("%s (%s) — %s" % (l.get("job", "?"), l.get("instance", "?"),
                            t.get("lastError") or t.get("health") or "sin detalle"))
' <"$CUERPO" 2>/dev/null)"
  if [ "$HTTP_CODIGO" != 200 ] || [ -z "$TARGETS" ]; then
    rojo "no se pudo leer $PROM_URL/api/v1/targets (HTTP $HTTP_CODIGO): no se sabe qué se está scrapeando, y sin scrape no hay alerta que pueda disparar"
  else
    conteo="$(printf '%s\n' "$TARGETS" | head -1)"
    n_up="${conteo%% *}"; n_tot="${conteo##* }"
    if [ "$n_tot" -eq 0 ]; then
      rojo "Prometheus no tiene NINGÚN target activo: no está scrapeando nada. Todas las reglas evalúan sobre el vacío y ninguna puede disparar"
    elif [ "$n_up" -eq "$n_tot" ]; then
      verde "$n_up/$n_tot targets up"
    else
      rojo "$n_up/$n_tot targets up — los que no responden dejan ciegas a las alertas que dependen de ellos:"
      printf '%s\n' "$TARGETS" | tail -n +2 | sed 's/^/      down: /'
    fi
  fi
fi

# (c) QUÉ ESTÁ DISPARADO AHORA MISMO ─────────────────────────────────────────────────────────
# Esto es INFORMATIVO y no cuenta como divergencia: es el estado de este minuto, no del despliegue.
# Lo que sí cuenta es no poder preguntarlo.
if [ "$PROM_VIVO" != si ]; then
  dudoso "no se miró qué alertas están disparadas: Prometheus no contestó"
else
  pedir_http "$PROM_URL/api/v1/alerts"
  DISPARADAS="$(python3 -c '
import sys, json
from collections import Counter
a = json.load(sys.stdin)["data"]["alerts"]
print("OK")
c = Counter(x["labels"].get("alertname", "?") for x in a if x.get("state") == "firing")
for nombre, n in sorted(c.items()):
    print("%s (%d)" % (nombre, n))
' <"$CUERPO" 2>/dev/null)"
  if [ "$HTTP_CODIGO" != 200 ] || [ "$(printf '%s\n' "$DISPARADAS" | head -1)" != OK ]; then
    rojo "no se pudo leer $PROM_URL/api/v1/alerts (HTTP $HTTP_CODIGO): no se sabe si hay algo disparado"
  else
    lista="$(printf '%s\n' "$DISPARADAS" | tail -n +2 | grep . || true)"
    if [ -z "$lista" ]; then
      gris "ninguna alerta disparada en este momento"
    else
      gris "disparadas ahora (estado del minuto, no divergencia con el repo):"
      printf '%s\n' "$lista" | sed 's/^/      firing: /'
    fi
  fi
fi

# (d) EL ÚLTIMO ESLABÓN: ALERTMANAGER ────────────────────────────────────────────────────────
# Se pregunta DOS veces y por dos cosas distintas, porque fallan por separado:
#   · si Prometheus lo VE (`/api/v1/alertmanagers`) — sin eso las reglas se evalúan, se ven en la
#     UI y no notifican a nadie. Es el modo de falla que costó S4b entero.
#   · si Alertmanager CONTESTA y tiene ruta — un Alertmanager vivo sin ruta raíz recibe todo y no
#     entrega nada.
if [ "$PROM_VIVO" != si ]; then
  dudoso "no se pudo saber si Prometheus ve algún Alertmanager: Prometheus no contestó"
else
  pedir_http "$PROM_URL/api/v1/alertmanagers"
  N_AM="$(python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
print(len(d.get("activeAlertmanagers") or []))
' <"$CUERPO" 2>/dev/null)"
  if [ "$HTTP_CODIGO" != 200 ] || [ -z "$N_AM" ]; then
    rojo "no se pudo leer $PROM_URL/api/v1/alertmanagers (HTTP $HTTP_CODIGO): no se sabe si las alertas tienen a quién entregarse"
  elif [ "$N_AM" -eq 0 ]; then
    rojo "Prometheus no ve NINGÚN Alertmanager: las reglas se evalúan, se ven en la UI de alertas y no notifican a nadie. Revisá el bloque alerting: de prometheus.yml y que el contenedor esté arriba"
  else
    verde "Prometheus ve $N_AM Alertmanager(s) activos"
  fi
fi

pedir_http "$ALERT_URL/api/v2/status"
if parece_html; then
  rojo "en $ALERT_URL contesta una página web, no la API de Alertmanager (HTTP $HTTP_CODIGO): apuntá ALERT_URL a 127.0.0.1:9093"
else
  case "$HTTP_CODIGO" in
    200)
      RUTAS="$(python3 -c '
import sys, json, re
cfg = (json.load(sys.stdin).get("config") or {}).get("original") or ""
raiz, hijas, receptores, en_route, en_rec = "", 0, 0, False, False
for l in cfg.splitlines():
    if re.match(r"^route:", l):
        en_route, en_rec = True, False
        continue
    if re.match(r"^receivers:", l):
        en_rec, en_route = True, False
        continue
    if re.match(r"^[A-Za-z]", l):
        en_route = en_rec = False
    if en_route:
        m = re.match(r"^\s+-?\s*receiver:\s*[\x27\"]?([^\x27\"\s]+)", l)
        if m:
            if raiz:
                hijas += 1
            else:
                raiz = m.group(1)
    if en_rec and re.match(r"^\s*-\s*name:", l):
        receptores += 1
print("%s %d %d" % (raiz or "-", hijas, receptores))
' <"$CUERPO" 2>/dev/null)"
      if [ -z "$RUTAS" ]; then
        rojo "Alertmanager contestó pero no se pudo leer su configuración viva ($ALERT_URL/api/v2/status): no se sabe si tiene rutas cargadas"
      else
        read -r am_raiz am_hijas am_recep <<<"$RUTAS"
        if [ "$am_raiz" = "-" ]; then
          rojo "Alertmanager responde y su config viva NO tiene ruta raíz (route:): recibe alertas y no entrega ninguna"
        elif [ "$am_recep" -eq 0 ]; then
          rojo "Alertmanager responde, enruta a «$am_raiz» y no declara NINGÚN receptor: no hay a dónde mandar el mensaje"
        else
          verde "Alertmanager responde: raíz → «$am_raiz», $am_hijas rutas hijas, $am_recep receptores cargados"
        fi
      fi ;;
    401|403)
      dudoso "Alertmanager pide credencial (HTTP $HTTP_CODIGO): exportá MUSUBI_HTTP_BEARER=<token> —o MUSUBI_HTTP_USUARIO/MUSUBI_HTTP_CLAVE— y repetí. Sin eso el último eslabón queda SIN VERIFICAR" ;;
    000)
      rojo "$ALERT_URL no contestó — $(razon_muda). Alertmanager escucha SÓLO en loopback y a propósito: su API silencia alertas sin pedir credencial. Corré esto en el servidor, o con MUSUBI_SSH=musubi-server, o por el túnel: ssh -N -L 9093:127.0.0.1:9093 usuario@musubi-server" ;;
    *)
      rojo "$ALERT_URL/api/v2/status devolvió HTTP $HTTP_CODIGO: no se pudo confirmar que el último eslabón esté vivo" ;;
  esac
fi

# ── 2 · LAS REGLAS DE ALERTA, ARCHIVO POR ARCHIVO ───────────────────────────────────────────
titulo "reglas de alerta"

# La foto es la de la sección 1: se pidió una vez y se compara contra ella. Si no se pudo pedir, se
# corta acá — comparar contra una lista vacía diría «no falta nada» sobre cero información.
if [ -z "$REGLAS_JSON" ]; then
  printf '  no se pudieron leer las reglas cargadas de %s (ver «cadena de alertas» arriba)\n' "$PROM_URL" >&2
  printf '  (desde afuera del servidor hace falta MUSUBI_SSH=<host>: Prometheus escucha en loopback)\n' >&2
  exit 2
fi

for f in "$REPO"/deploy/musubi-alerts*.yml; do
  nombre="$(basename "$f")"
  # La condición de despliegue la declara el propio archivo. La custodia
  # TestCadaArchivoDeReglasDeclaraCuandoSeDespliega, así que si falta, es un bug del repo.
  cond="$(sed -n 's/^#[[:space:]]*despliegue:[[:space:]]*//p' "$f" | head -1)"
  declara="$(grep -E '^[[:space:]]*-[[:space:]]+alert:' "$f" | sed -E 's/.*alert:[[:space:]]*//' | sort -u)"
  n_declara="$(printf '%s\n' "$declara" | grep -c . || true)"

  faltan="$(comm -23 <(printf '%s\n' "$declara") <(printf '%s\n' "$CARGADAS"))"
  n_faltan="$(printf '%s\n' "$faltan" | grep -c . || true)"

  if [ "$n_faltan" -eq 0 ]; then
    verde "$nombre — sus $n_declara reglas están cargadas"
  elif [ "$n_faltan" -eq "$n_declara" ]; then
    # NINGUNA cargada: o el archivo no se instaló, o no correspondía instalarlo. La diferencia la
    # da la línea `# despliegue:`, y confundirlas es lo que hace que un informe deje de leerse.
    case "$cond" in
      siempre) rojo "$nombre — NO está desplegado, y se declara «siempre»: sus $n_declara reglas no existen en producción" ;;
      condicional*) gris "$nombre — sin cargar, y así corresponde: $cond" ;;
      *) rojo "$nombre — no declara su condición de despliegue (# despliegue:) y no está cargado" ;;
    esac
  else
    rojo "$nombre — desplegado A MEDIAS: faltan $n_faltan de $n_declara"
    printf '%s\n' "$faltan" | sed 's/^/      falta: /'
  fi

  sobran="$(comm -13 <(printf '%s\n' "$declara") <(printf '%s\n' "$CARGADAS"))"
  : "${sobran:=}"
done

# LO QUE CORRE Y EL REPO NO TIENE. Se compara contra la UNIÓN de los cuatro archivos —calculada en
# la sección 1—: una regla vieja que quedó cargada no aparece como «sobrante» de cada archivo por
# separado.
huerfanas="$(comm -13 <(printf '%s\n' "$TODAS_DECLARADAS") <(printf '%s\n' "$CARGADAS"))"
if [ -n "$huerfanas" ]; then
  rojo "hay reglas CARGADAS que el repo ya no tiene (quedaron de un despliegue anterior):"
  printf '%s\n' "$huerfanas" | sed 's/^/      sobra: /'
fi

# ── 3 · LOS SCRAPES ─────────────────────────────────────────────────────────────────────────
titulo "scrapes"

JOBS_VIVOS="$(pedir "$PROM_URL/api/v1/targets?state=any" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]
vistos = set()
for k in ("activeTargets", "droppedTargets"):
    for t in d.get(k) or []:
        j = (t.get("labels") or t.get("discoveredLabels") or {}).get("job")
        if j: vistos.add(j)
print("\n".join(sorted(vistos)))
')"

# Los scrapes de SITIO entran por glob (`scrape_config_files`) y no se comparan: el repo trae el
# `.ejemplo` y cada sitio instala el suyo. Está dicho en prometheus.yml y se repite acá porque
# alguien va a mirar este bloque sin leer aquel archivo.
JOBS_REPO="$(grep -E '^[[:space:]]*-[[:space:]]*job_name:' "$REPO/deploy/prometheus/prometheus.yml" \
  | sed -E 's/.*job_name:[[:space:]]*"?([A-Za-z0-9_-]+)"?.*/\1/' | sort -u)"

for j in $JOBS_REPO; do
  if printf '%s\n' "$JOBS_VIVOS" | grep -qx "$j"; then
    verde "job $j — configurado"
  else
    rojo "job $j — el repo lo declara y Prometheus NO lo tiene: todo lo que dependa de sus métricas está ciego, no en verde"
  fi
done
gris "los scrapes de sitio (scrape_config_files) no se comparan: el repo sólo trae el .ejemplo"

# ── 4 · EL MENSAJE DE TELEGRAM ──────────────────────────────────────────────────────────────
titulo "alertmanager"

AM_CFG="$(pedir "$ALERT_URL/api/v2/status" | python3 -c '
import sys, json
print(json.load(sys.stdin)["config"]["original"])
' 2>/dev/null)"

if [ -z "$AM_CFG" ]; then
  rojo "no se pudo leer la configuración viva de Alertmanager en $ALERT_URL"
else
  # `parse_mode: ''` es una decisión medida, no un detalle: con Markdown o HTML, un nombre de
  # máquina con guión bajo rompe el envío ENTERO y Telegram devuelve 400. El mensaje no llega y
  # el error queda en el log de un contenedor.
  #
  # SE COMPRUEBA POR AUSENCIA, y eso no es pereza: la API devuelve la config ya marshalada y el
  # campo lleva `omitempty`, así que un parse_mode vacío NO APARECE. Buscar `parse_mode: ""` daba
  # rojo sobre una configuración correcta — se midió contra el servidor antes de escribir esto.
  if ! printf '%s' "$AM_CFG" | grep -qE "parse_mode:[[:space:]]*[^[:space:]'\"]"; then
    verde "parse_mode vacío — un nombre con guión bajo no puede tumbar el envío"
  else
    rojo 'parse_mode NO está vacío en producción: un guión bajo en un nombre de máquina rompe el mensaje entero (Telegram 400)'
  fi
  # La plantilla: se compara la PRIMERA LÍNEA, que es la que distingue «se rompió» de «se
  # arregló». Comparar el mensaje entero daría falso positivo por cualquier espacio.
  if printf '%s' "$AM_CFG" | grep -q 'EMPIEZA'; then
    verde "la plantilla distingue «empieza» de «se resolvió»"
  else
    rojo "la plantilla viva no distingue un alerta que EMPIEZA de una que SE RESOLVIÓ: los dos mensajes llegan iguales"
  fi
fi

# ── 5 · LA VERSIÓN DEL CEREBRO ──────────────────────────────────────────────────────────────
titulo "versión"

VER_REPO="$(tr -d '[:space:]' < "$REPO/VERSION")"
if [ -n "$SSH_HOST" ]; then
  VER_VIVA="$(ssh -o BatchMode=yes -o ConnectTimeout=10 "$SSH_HOST" 'musubi version 2>/dev/null' | awk '{print $2}')"
else
  VER_VIVA="$(musubi version 2>/dev/null | awk '{print $2}')"
fi

if [ -z "$VER_VIVA" ]; then
  gris "no se pudo preguntar la versión del cerebro (\`musubi version\`); queda sin comparar"
else
  # Se compara el NÚCLEO, por lo mismo que agent_stale (A68): el cerebro se redespliega varias
  # veces por día desde commits distintos del mismo release, y comparar commits daría rojo siempre.
  nucleo="${VER_VIVA%%-*}"
  if [ "$nucleo" = "$VER_REPO" ]; then
    verde "cerebro en $VER_VIVA — mismo release que el repo ($VER_REPO)"
  else
    rojo "cerebro en $VER_VIVA y el repo declara $VER_REPO"
  fi
fi

# ── El veredicto ────────────────────────────────────────────────────────────────────────────
if [ "$DIVERGE" -ne 0 ]; then
  printf '\n\033[31mproducción diverge del repo\033[0m — arriba está qué y en qué dirección\n'
fi
# Un «no pude preguntar» sale con 2 AUNQUE además haya divergencia, y aunque no la haya: quien
# automatice esto tiene que poder distinguir «vi algo mal» de «no vi». Confundirlos es exactamente
# cómo se coló el verde que dejó `CadenaDeAlertasFallando` sin desplegar durante semanas.
if [ "$SIN_VERIFICAR" -ne 0 ]; then
  printf '\n\033[33mquedaron eslabones SIN VERIFICAR\033[0m — los marcados con «?» arriba dicen qué falta para poder preguntarles. Esto NO es un verde\n'
  exit 2
fi
if [ "$DIVERGE" -ne 0 ]; then
  exit 1
fi
printf '\n\033[32mlo que corre coincide con lo que el repo declara y la cadena de alertas contesta entera\033[0m (dentro de lo que esto mira; ver el encabezado)\n'
exit 0
