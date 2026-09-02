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
#
# Un informe que calla lo que no mira se lee como si lo hubiera mirado. Es el mismo hallazgo de
# A66, y por eso esta lista está arriba y no al final.
# ════════════════════════════════════════════════════════════════════════════════════════════
#
# Uso:
#   ./deploy/verificar-despliegue.sh                    # corriendo EN el servidor
#   MUSUBI_SSH=musubi-server ./deploy/verificar-despliegue.sh   # desde afuera
#
# Sale 0 si todo coincide, 1 si hay divergencia, 2 si no pudo preguntar.

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

rojo()  { printf '  \033[31m✘ %s\033[0m\n' "$1"; DIVERGE=1; }
verde() { printf '  \033[32m✔ %s\033[0m\n' "$1"; }
gris()  { printf '  \033[90m· %s\033[0m\n' "$1"; }
titulo(){ printf '\n\033[1m%s\033[0m\n' "$1"; }
DIVERGE=0

# pedir <url> — un GET contra el servidor, por ssh si hace falta.
pedir() {
  if [ -n "$SSH_HOST" ]; then
    ssh -o BatchMode=yes -o ConnectTimeout=10 "$SSH_HOST" "curl -sS -m 15 '$1'"
  else
    curl -sS -m 15 "$1"
  fi
}

printf '\033[1mverificar-despliegue\033[0m — repo %s contra %s\n' "$REPO" "${SSH_HOST:-127.0.0.1}"

# ── 1 · LAS REGLAS DE ALERTA ────────────────────────────────────────────────────────────────
titulo "reglas de alerta"

REGLAS_JSON="$(pedir "$PROM_URL/api/v1/rules")"
if [ -z "$REGLAS_JSON" ] || ! printf '%s' "$REGLAS_JSON" | grep -q '"status"'; then
  printf '  no se pudo consultar %s/api/v1/rules\n' "$PROM_URL" >&2
  printf '  (desde afuera del servidor hace falta MUSUBI_SSH=<host>: Prometheus escucha en loopback)\n' >&2
  exit 2
fi

CARGADAS="$(printf '%s' "$REGLAS_JSON" | python3 -c '
import sys, json
d = json.load(sys.stdin)["data"]["groups"]
for g in d:
    for r in g["rules"]:
        if r.get("type") == "alerting":
            print(r["name"])
' | sort -u)"

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

# LO QUE CORRE Y EL REPO NO TIENE. Se calcula UNA vez contra la unión de los cuatro archivos: una
# regla vieja que quedó cargada no aparece como «sobrante» de cada archivo por separado.
TODAS_DECLARADAS="$(cat "$REPO"/deploy/musubi-alerts*.yml | grep -E '^[[:space:]]*-[[:space:]]+alert:' | sed -E 's/.*alert:[[:space:]]*//' | sort -u)"
huerfanas="$(comm -13 <(printf '%s\n' "$TODAS_DECLARADAS") <(printf '%s\n' "$CARGADAS"))"
if [ -n "$huerfanas" ]; then
  rojo "hay reglas CARGADAS que el repo ya no tiene (quedaron de un despliegue anterior):"
  printf '%s\n' "$huerfanas" | sed 's/^/      sobra: /'
fi

# ── 2 · LOS SCRAPES ─────────────────────────────────────────────────────────────────────────
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

# ── 3 · EL MENSAJE DE TELEGRAM ──────────────────────────────────────────────────────────────
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

# ── 4 · LA VERSIÓN DEL CEREBRO ──────────────────────────────────────────────────────────────
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
if [ "$DIVERGE" -eq 0 ]; then
  printf '\n\033[32mlo que corre coincide con lo que el repo declara\033[0m (dentro de lo que esto mira; ver el encabezado)\n'
  exit 0
fi
printf '\n\033[31mproducción diverge del repo\033[0m — arriba está qué y en qué dirección\n'
exit 1
