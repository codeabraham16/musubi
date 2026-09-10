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
#   · Los archivos de CONFIGURACIÓN del servidor. Se le pregunta a Prometheus qué CARGÓ, que es la
#     única respuesta que importa; un archivo correcto sin recargar no se distingue de uno viejo, y
#     así tiene que ser. **Esa razón vale para lo que lee un daemon y NO para un guion de shell**:
#     no hay nadie que lo relea, el archivo ES lo que corre en el momento en que se lo invoca. Por
#     confundir los dos casos, el `redesplegar-cerebro.sh` del servidor quedó a la mitad del del
#     repo y con su verificación de la migración muerta desde el esquema 38 —pasó así en los seis
#     redespliegues del 4 y el 5 de septiembre de 2026 (A111)—, y este script, que existe justo
#     para cruzar repo contra producción, no lo miraba. Los guiones derivados SÍ se comparan, abajo.
#   · Los scrapes de sitio (`/etc/prometheus/scrapes/*.yml`). Son por sitio a propósito y el repo
#     sólo trae el `.ejemplo`, así que no hay contra qué compararlos.
#   · Lo que corre en las máquinas de la flota. Eso lo dice `musubi_fleet_device_agent_stale` (A68).
#   · **La POSTURA de transporte no puede poner esto en rojo por default.** La sección del final
#     informa con `~` si el cerebro sirve HTTP en claro y el veredicto sigue en 0, porque no es
#     deriva: el repo no declara TLS y ésa es la configuración elegida. Dicho de frente: un verde
#     de este script NO significa «el bearer viaja cifrado». Se exige con `MUSUBI_EXIGIR_TLS=1`,
#     y ahí sí cuenta como divergencia. Está acá y no sólo abajo porque «informa y no puede
#     fallar» es la forma exacta de un falso verde, que es el defecto que este script vino a
#     cerrar — y la única defensa contra su propia excepción es que esté escrita donde se lee el
#     contrato, no donde se lee el resultado.
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

# detalle — imprime QUÉ, renglón por renglón, DESDE ESTA SHELL Y NO DESDE UN PROCESO EFÍMERO.
#
# LA MITAD ACCIONABLE DEL INFORME NO LLEGABA AL JOURNAL, Y ESO COSTÓ TRES DÍAS.
#
# Acá había cinco `printf '%s\n' "$x" | sed 's/^/      falta: /'`. Funcionan perfecto en una
# terminal. Bajo systemd NO: journald resuelve a qué unidad pertenece cada línea leyendo
# `/proc/<pid>/cgroup` CUANDO LA RECIBE, y `sed` —un proceso de pipeline que vive milisegundos—
# **ya murió**. Medido el 2026-09-08 sobre el journal real de `musubi-comparar.service`:
#
#     journalctl --user -u musubi-comparar.service | grep -cE 'falta:|firing:|down:|sobra:'  ->  0
#     journalctl --user                            | grep -cE 'falta:|firing:|down:|sobra:'  -> 42
#
# Y los metadatos de una de esas 42 lo dicen entero: `_COMM=sed`, `_SYSTEMD_UNIT` **ausente**,
# `_SYSTEMD_USER_UNIT` **ausente**, `_SYSTEMD_CGROUP` **ausente**. Sobrevive `SYSLOG_IDENTIFIER`
# porque va en el fd del stream y no se resuelve desde `/proc`.
#
# EL DAÑO NO ES COSMÉTICO. El unit file documenta `journalctl --user -u musubi-comparar.service`
# como LA forma de leer esto. Quien la seguía veía «desplegado A MEDIAS: faltan 1 de 28» y **nunca
# el nombre**; veía «disparadas ahora:» seguido de nada; no veía qué target estaba caído. El
# titular llegaba y el nombre no, así que para saber qué hacer había que volver a correrlo A MANO
# — que es exactamente el paso manual que este guion existe para eliminar. La regla que faltaba
# era `InventarioDeServiciosIncompleto`, y estuvo tres días escrita en el journal sin que la
# lectura documentada pudiera mostrarla.
#
# `printf` es un BUILTIN: lo ejecuta bash, que vive toda la corrida, así que journald sí puede
# resolver su cgroup. El `while` va con un here-string y no con un pipe, por dos motivos: un pipe
# metería la lectura en una subshell efímera —el defecto de nuevo— y además `ssh` sin `-n` ya nos
# enseñó lo que cuesta que un bucle comparta stdin con otro (ver `corre_alla`).
detalle() {
	prefijo="$1"
	while IFS= read -r _linea; do
		[ -n "$_linea" ] || continue
		printf '      %s%s\n' "$prefijo" "$_linea"
	done <<DETALLE
$2
DETALLE
}
# dudoso — lo que NO SE PUDO comprobar. No es verde ni rojo: es «no vi», y sale con 2. Existe
# porque el modo de falla que trajo este script hasta acá es siempre el mismo: una consulta que no
# se pudo hacer y un informe que igual terminó en verde.
dudoso(){ printf '  \033[33m? %s\033[0m\n' "$1"; SIN_VERIFICAR=1; }

# nucleo_de_version — el MISMO núcleo que `fleet.NucleoDeVersion`, y por eso está escrito acá con
# su tabla al lado en `deploy/pruebas/version-parseable.sh`, que corre LAS DOS y las compara.
#
# ESTO ES UNA REIMPLEMENTACIÓN Y NO SE PUEDE EVITAR: el verificador corre sin Go —a veces contra un
# servidor que no lo tiene— así que no puede llamar al parser de verdad. Lo que sí se puede evitar
# es que las dos DIVERJAN sin que nadie se entere, y eso es lo que custodia el arnés.
#
# Divergía en tres cosas, las tres medidas el 2026-09-09, y las tres daban FALSO ROJO sobre un
# binario del release correcto:
#
#   · `${VER_VIVA%%-*}` no valida que queden TRES componentes, así que `0.139.7.abc1234` —lo que
#     emite construir.sh sin track— devolvía la cadena entera como «núcleo».
#   · no sacaba el prefijo `v`, así que `v0.106.0-28-gdf2ec21` daba `v0.106.0`. Esa familia
#     (`git describe`) es una de las DOS que el Go declara tolerar, y está enrolada en producción.
#   · cortaba sólo en `-` y no en `-` o `+`, así que `0.130.0+build5` devolvía la cadena entera.
#
# Devuelve 1 cuando no puede parsear. Un núcleo vacío NO alcanza para señalarlo: el llamador lo
# compararía contra `VERSION` y daría distinto, o sea que volvería a decir «diverge».
nucleo_de_version() {
  # El orden es el del Go: espacios, después el prefijo `v`, después el corte.
  _v="$(printf '%s' "$1" | tr -d '[:space:]')"
  _v="${_v#v}"
  _v="${_v%%[-+]*}"
  # EXACTAMENTE tres. El primer `case` descarta cuatro o más —que es la forma que emite
  # `construir.sh` sin track— y el segundo exige que haya tres.
  case "$_v" in *.*.*.*) return 1 ;; esac
  case "$_v" in *.*.*) : ;; *) return 1 ;; esac
  _a="${_v%%.*}"; _r="${_v#*.}"; _b="${_r%%.*}"; _c="${_r#*.}"
  # Cada componente por separado, y no la concatenación: con `0..0` la concatenación da `00`,
  # que es numérica y no vacía, así que un solo chequeo sobre el pegado lo daría por bueno.
  for _p in "$_a" "$_b" "$_c"; do
    case "$_p" in ''|*[!0-9]*) return 1 ;; esac
  done
  printf '%s.%s.%s' "$_a" "$_b" "$_c"
}
# tibio — una POSTURA que no es del gusto de nadie pero que HOY es la configuración elegida. No es
# rojo (nada divergió del repo) y no puede ser verde (el riesgo existe). Sale por su propia
# variable para que el veredicto no mezcle «esto se desplegó mal» con «esto está así a propósito y
# todavía nadie decidió cambiarlo». Con MUSUBI_EXIGIR_TLS=1 pasa a contar como divergencia: el día
# que la migración esté hecha, el que la hizo prende la variable y esto queda custodiado.
tibio(){ printf '  \033[33m~ %s\033[0m\n' "$1"; POSTURA=1; }
DIVERGE=0
SIN_VERIFICAR=0
POSTURA=0
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

# ── 0 · LA REFERENCIA: ¿CONTRA QUÉ SE COMPARA TODO LO DE ABAJO? ──────────────────────────────
#
# ESTA SECCIÓN EXISTE PORQUE EL RESTO DEL GUION NO SE PUEDE CREER SIN ELLA.
#
# Todo lo que sigue compara producción contra `$REPO/...` — o sea contra EL ÁRBOL DE TRABAJO: el
# `VERSION` de la sección 5, los cinco archivos de reglas, los guiones derivados. El árbol de
# trabajo es lo que haya checkouteado en ese momento: una rama vieja, un merge a medio hacer, un
# archivo editado y sin commitear. El guion nunca lo decía, y quien lee el informe supone `main`.
#
# MEDIDO EL 2026-09-10, Y POR ESO ESTÁ ESTO ACÁ. El cerebro corría 0.139.7 y `origin/main` había
# cortado 0.140.1 tres horas antes. La corrida de las 08:12 imprimió «cerebro en 0.139.7 — mismo
# release que el repo (0.139.7)» y salió 0. No mintió: comparó contra el árbol, parado en una rama
# con el VERSION viejo. La ÚNICA defensa automática contra «arreglado en el repo, nunca llegado a
# la máquina» quedó ciega justo cuando el checkout no está en el último main — que es el estado
# NORMAL de un repo en el que se trabaja, no una excepción rara.
#
# ES LA FORMA DE A113 OTRA VEZ: un número que viaja junto a lo que vigila no lo vigila. Allá eran
# dos archivos de reglas que el despliegue copiaba juntos, así que se quedaban viejos juntos; acá
# son la referencia y el verificador, que salen del mismo checkout.
#
# POR QUÉ `dudoso` Y NO `rojo`: un checkout que no es `origin/main` no dice que producción esté
# mal. Dice que ESTA CORRIDA NO PUEDE CONTESTAR la pregunta. Es «no vi», que en este guion sale
# con 2 y a propósito no comparte código con el verde.
#
# Y POR QUÉ EL GUION HACE EL FETCH: sin traer la referencia, «HEAD == origin/main» se contesta
# contra un `origin/main` local que puede ser de la semana pasada — o sea, se compara el repo
# contra sí mismo. Traerla es parte de hacer la pregunta, no un efecto secundario. Sólo mueve el
# ref de seguimiento; no toca el árbol ni el HEAD. Con `MUSUBI_SIN_FETCH=1` no se trae, y entonces
# se informa la edad de lo que hay.
titulo "la referencia (contra qué se compara todo lo de abajo)"

REF_EDAD_MAX_H="${MUSUBI_REF_EDAD_MAX_H:-24}"

if ! git -C "$REPO" rev-parse --git-dir >/dev/null 2>&1; then
  dudoso "el árbol de $REPO no es un repositorio git: se compara contra los archivos que haya ahí, sin saber de qué commit salieron"
else
  if [ -z "${MUSUBI_SIN_FETCH:-}" ]; then
    if git -C "$REPO" fetch --quiet origin "+refs/heads/main:refs/remotes/origin/main" 2>/dev/null; then
      gris "referencia traída de origin/main recién"
    else
      dudoso "no se pudo traer origin/main (sin red, sin remoto o sin permiso): la referencia es la que había guardada, y su edad va abajo"
    fi
  else
    gris "MUSUBI_SIN_FETCH=1: no se trajo la referencia, se usa la que hay"
  fi

  if ! git -C "$REPO" rev-parse --verify --quiet origin/main >/dev/null 2>&1; then
    dudoso "este árbol no tiene la referencia origin/main: no hay contra qué medir el checkout, así que ningún «coincide» de abajo dice contra qué"
  else
    REF_RAMA="$(git -C "$REPO" rev-parse --abbrev-ref HEAD 2>/dev/null || echo '?')"
    REF_HEAD="$(git -C "$REPO" rev-parse --short HEAD 2>/dev/null || echo '?')"
    REF_MAIN="$(git -C "$REPO" rev-parse --short origin/main 2>/dev/null || echo '?')"
    REF_ATRAS="$(git -C "$REPO" rev-list --count HEAD..origin/main 2>/dev/null || echo '?')"
    REF_ADELANTE="$(git -C "$REPO" rev-list --count origin/main..HEAD 2>/dev/null || echo '?')"
    # EL SUCIO CUENTA LOS NO TRACKEADOS TAMBIÉN. Un archivo de reglas nuevo y sin agregar cambia
    # lo que este guion compara, y ya nos costó una vez: el sufijo `-sucio` de `construir.sh` no
    # veía los archivos sin trackear y declaró limpio un binario que se llevaba código de más.
    REF_SUCIO="$(git -C "$REPO" status --porcelain 2>/dev/null | wc -l | tr -d ' ')"

    # La edad de la referencia se mide por el último fetch, no por la fecha del commit: un
    # `origin/main` de hace una semana puede apuntar a un commit de hoy y seguir estando viejo.
    REF_EDAD_H=""
    GITDIR="$(git -C "$REPO" rev-parse --git-common-dir 2>/dev/null || echo '')"
    case "$GITDIR" in
      "") ;;
      /*) ;;
      *) GITDIR="$REPO/$GITDIR" ;;
    esac
    if [ -n "$GITDIR" ] && [ -f "$GITDIR/FETCH_HEAD" ]; then
      REF_EDAD_H=$(( ( $(date +%s) - $(stat -c %Y "$GITDIR/FETCH_HEAD" 2>/dev/null || echo 0) ) / 3600 ))
    fi

    if [ "$REF_ATRAS" = "0" ] && [ "$REF_ADELANTE" = "0" ] && [ "$REF_SUCIO" = "0" ]; then
      verde "el árbol es origin/main exacto ($REF_MAIN) y está limpio: todo lo de abajo compara contra eso"
    else
      MOTIVO=""
      [ "$REF_ATRAS" != "0" ]    && MOTIVO="$MOTIVO, le faltan $REF_ATRAS commits de origin/main"
      [ "$REF_ADELANTE" != "0" ] && MOTIVO="$MOTIVO, tiene $REF_ADELANTE commits que origin/main no"
      if [ "$REF_SUCIO" = "1" ]; then MOTIVO="$MOTIVO, y 1 archivo sin commitear"
      elif [ "$REF_SUCIO" != "0" ]; then MOTIVO="$MOTIVO, y $REF_SUCIO archivos sin commitear"; fi
      dudoso "el árbol NO es origin/main: está en «$REF_RAMA» ($REF_HEAD)${MOTIVO}. Todo lo de abajo compara contra ESE árbol, así que un «coincide» no dice que producción esté al día con main"
    fi

    if [ -n "$REF_EDAD_H" ] && [ "$REF_EDAD_H" -gt "$REF_EDAD_MAX_H" ]; then
      dudoso "la referencia origin/main se trajo hace ${REF_EDAD_H} h (el techo es ${REF_EDAD_MAX_H} h): compararse contra ella es compararse contra un main viejo"
    fi
  fi
fi

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
      detalle 'falta: ' "$faltan_obl"
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

# (a quater) LAS RECORDING RULES DEL SLA ─────────────────────────────────────────────────────
# TODO LO DE ARRIBA ES CIEGO A ESTE ARCHIVO, y por eso hace falta un bloque aparte.
#
# `CARGADAS` filtra por `type == "alerting"` y `TODAS_DECLARADAS` hace glob de `musubi-alerts*.yml`:
# las once recording rules del SLA no aparecen en ninguna de las dos listas. Y son el número que se
# le FACTURA a un cliente. A93, medido el 2026-09-05: ningún guion las instalaba, ninguna alerta las
# contaba, y las edades de las series delataban tres instalaciones a mano en tres momentos distintos
# (11,9 h de historia en una regla y 6,25 h en otra del mismo archivo).
#
# Lo que hace peligroso al caso es que un `avg30d` viejo sigue devolviendo un número plausible: no
# hay síntoma. Así que la comparación es contra el repo y por conteo, igual que las otras dos.
N_SLA_REPO="$(grep -cE '^[[:space:]]*-[[:space:]]+record:' "$REPO/deploy/musubi-recording.yml" || true)"
if [ -z "$REGLAS_JSON" ]; then
  dudoso "no se contaron las recording rules del SLA: no hay lista de reglas que mirar (el repo declara $N_SLA_REPO)"
else
  N_SLA_VIVAS="$(printf '%s' "$REGLAS_JSON" | python3 -c '
import sys, json
try:
    g = json.load(sys.stdin)["data"]["groups"]
except Exception:
    print(""); sys.exit(0)
print(sum(1 for x in g if "musubi-recording.yml" in x.get("file", "")
          for r in x["rules"] if r.get("type") == "recording"))
' 2>/dev/null)"
  : "${N_SLA_VIVAS:=}"
  if [ -z "$N_SLA_VIVAS" ]; then
    dudoso "no pude contar las recording rules del SLA en la respuesta de Prometheus (el repo declara $N_SLA_REPO)"
  elif [ "$N_SLA_VIVAS" = "$N_SLA_REPO" ]; then
    verde "SLA: $N_SLA_VIVAS recording rules cargadas, las mismas $N_SLA_REPO que declara el repo"
  elif [ "$N_SLA_VIVAS" = 0 ] && [ "$HAY_FLOTA" = no ]; then
    gris "SLA sin cargar, y así corresponde: musubi-recording.yml es condicional y el cerebro no expone musubi_fleet_*"
  elif [ "$N_SLA_VIVAS" = 0 ] && [ "$HAY_FLOTA" = indeterminado ]; then
    dudoso "el SLA no está cargado y no pude resolver si correspondía (musubi-recording.yml es condicional y no pude preguntar por musubi_fleet_device_up)"
  elif [ "$N_SLA_VIVAS" = 0 ]; then
    rojo "el SLA NO está cargado y su condición SÍ se cumple: las $N_SLA_REPO recording rules de musubi-recording.yml quedaron sin desplegar, y los avg30d que se le muestran al cliente no existen o los está calculando otra cosa"
  else
    rojo "el SLA tiene $N_SLA_VIVAS recording rules cargadas y el repo declara $N_SLA_REPO: quedó una versión vieja del archivo. El número que se factura sigue saliendo plausible, así que esto es la única señal"
  fi
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
      detalle 'down: ' "$(printf '%s\n' "$TARGETS" | tail -n +2)"
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
  # LA LÍNEA DICE QUÉ ALERTA **Y EN QUÉ MÁQUINA**, PORQUE EL NOMBRE SOLO NO SIRVE PARA ACTUAR.
  #
  # Acá había un `Counter` por `alertname` que TIRABA `device` y `project`. El informe decía
  # `firing: AgenteCaidoConMaquinaViva (1)` y quien lo leía no sabía en cuál de las cuatro
  # máquinas — o sea el mismo defecto que el commit de ayer (8f2fb99) arregló para `falta:`:
  # «el titular llega y lo accionable no». `detalle` tiene cinco call sites y CUATRO ya llevaban
  # identidad; éste era el que faltaba. La guarda estaba en N-1 de N, en el mismo archivo y un
  # día después.
  #
  # EL TECHO SE DECLARA, NO SE APLICA EN SILENCIO. Una tormenta de cuarenta máquinas no puede
  # tapar el resto del informe, pero un recorte mudo se lee como «son cinco» — que es la misma
  # mentira que este guion existe para no decir. Por eso sale «y N más».
  DISPARADAS="$(python3 -c '
import sys, json
from collections import defaultdict
a = json.load(sys.stdin)["data"]["alerts"]
print("OK")
TECHO = 5
g = defaultdict(list)
for x in a:
    if x.get("state") != "firing":
        continue
    l = x.get("labels", {})
    # `device` es la etiqueta de las alertas de flota; `instance` la de las de scrape; `job` el
    # ultimo recurso. Se prueban en ese orden porque es el de mayor a menor especificidad, y una
    # alerta del cerebro (sin device) tiene que seguir saliendo aunque sea sin identidad.
    quien = l.get("device") or l.get("instance") or l.get("job") or ""
    proy = l.get("project", "")
    if quien and proy:
        quien = "%s/%s" % (proy, quien)
    g[l.get("alertname", "?")].append(quien)
for nombre in sorted(g):
    n = len(g[nombre])
    quienes = sorted(q for q in g[nombre] if q)
    if not quienes:
        print("%s (%d)" % (nombre, n))
    elif len(quienes) <= TECHO:
        print("%s (%d) - %s" % (nombre, n, ", ".join(quienes)))
    else:
        print("%s (%d) - %s y %d mas" % (nombre, n, ", ".join(quienes[:TECHO]), len(quienes) - TECHO))
' <"$CUERPO" 2>/dev/null)"
  if [ "$HTTP_CODIGO" != 200 ] || [ "$(printf '%s\n' "$DISPARADAS" | head -1)" != OK ]; then
    rojo "no se pudo leer $PROM_URL/api/v1/alerts (HTTP $HTTP_CODIGO): no se sabe si hay algo disparado"
  else
    lista="$(printf '%s\n' "$DISPARADAS" | tail -n +2 | grep . || true)"
    if [ -z "$lista" ]; then
      gris "ninguna alerta disparada en este momento"
    else
      gris "disparadas ahora (estado del minuto, no divergencia con el repo):"
      detalle 'firing: ' "$lista"
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
    detalle 'falta: ' "$faltan"
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
  detalle 'sobra: ' "$huerfanas"
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
  # `-n` por la misma razón que en `corre_alla`: ninguno de los `ssh` de este guion quiere stdin,
  # salvo el de la línea ~140 que SÍ le pipea `config_curl`. Ponerlo en los que no lo necesitan es
  # lo que evita que el defecto vuelva el día que alguien mueva esta línea adentro de un bucle.
  VER_VIVA="$(ssh -n -o BatchMode=yes -o ConnectTimeout=10 "$SSH_HOST" 'musubi version 2>/dev/null' | awk '{print $2}')"
else
  VER_VIVA="$(musubi version 2>/dev/null | awk '{print $2}')"
fi

if [ -z "$VER_VIVA" ]; then
  # `dudoso`, no `gris`: el encabezado promete «sale 0 si todo coincide Y TODO SE PUDO PREGUNTAR».
  # Con `gris` esto salía 0 sin haber comparado nada — el mismo falso verde que el resto del script
  # cierra en todos sus otros caminos, colado justo en el último. No poder preguntar la versión es
  # grave por sí solo: es el único chequeo que dice si el binario que corre es el que se desplegó.
  dudoso "no se pudo preguntar la versión del cerebro (\`musubi version\`): no se comparó contra el repo ($VER_REPO). Si es por ssh, probá \`MUSUBI_SSH=<host>\`; si es local, que \`musubi\` esté en el PATH del que corre esto"
else
  # Se compara el NÚCLEO, por lo mismo que agent_stale (A68): el cerebro se redespliega varias
  # veces por día desde commits distintos del mismo release, y comparar commits daría rojo siempre.
  if nucleo="$(nucleo_de_version "$VER_VIVA")"; then
    if [ "$nucleo" = "$VER_REPO" ]; then
      verde "cerebro en $VER_VIVA — mismo release que el repo ($VER_REPO)"
    else
      rojo "cerebro en $VER_VIVA y el repo declara $VER_REPO"
    fi
  else
    # NO PODER PARSEAR NO ES DIVERGIR, Y DECIRLO MAL MANDA A ARREGLAR LO QUE NO ESTÁ ROTO.
    #
    # Antes esto caía en el `else` de arriba y salía `rojo "cerebro en X y el repo declara Y"`. Con
    # una versión de cuatro componentes —lo que emite `construir.sh` sin track— el núcleo quedaba
    # siendo la cadena ENTERA, nunca igualaba a `VERSION`, y un binario del release CORRECTO se
    # reportaba como «producción diverge del repo». Falso rojo, y con la causa equivocada escrita.
    #
    # Es `dudoso` y no `rojo` por la misma razón que el `VER_VIVA` vacío de arriba: no poder
    # comparar no es lo mismo que comparar y que dé distinto. Y el aviso es real, no cosmético —
    # esa forma además apaga `musubi_fleet_device_agent_stale` para TODA la flota.
    dudoso "la versión del cerebro ($VER_VIVA) no se puede parsear: no se comparó contra el repo ($VER_REPO). Se espera MAJOR.MINOR.PATCH con lo demás detrás de un \`-\` o un \`+\`; una versión de cuatro componentes sale de \`construir.sh\` sin track, y esa forma además apaga \`musubi_fleet_device_agent_stale\` para toda la flota"
  fi
fi

# ── Postura de transporte: ¿el bearer viaja cifrado? ────────────────────────────────────────
#
# NO es una comprobación de deriva: el repo no declara TLS, así que nada está "mal desplegado".
# Es una POSTURA, y está acá porque el informe de arriba se lee como un certificado de salud y
# hoy omitía que el token de admin viaja en texto plano.
#
# Lo cubre WireGuard (el tailnet cifra el tramo), y eso alcanza para dormir tranquilo y NO alcanza
# para una auditoría: no hay cifrado de extremo a extremo, y cualquier proceso del propio servidor
# que pueda hablarle al loopback ve el bearer. `tailscale cert` emite un certificado válido de
# Let's Encrypt para el nombre del nodo sin abrir nada — medido el 2026-09-03 en musubi-server: el
# usuario `musubi` lo obtiene sin sudo.
#
# El agente ya sabe verificar contra un nombre discando una IP (MUSUBI_BRAIN_TLS_NAME), que es el
# nudo que hacía imposible esta migración: con NordVPN el DNS de la malla no resuelve los nombres.
printf '\n\033[1mpostura de transporte\033[0m\n'
CFG_REMOTO="${MUSUBI_CFG:-/home/musubi/musubi-brain/.musubi/config.yaml}"
if [ -n "$SSH_HOST" ]; then
  POSTURA_TLS="$(ssh -n -o BatchMode=yes -o ConnectTimeout=10 "$SSH_HOST" \
    "grep -E '^[[:space:]]*(allow_insecure_token|tls_cert_file|tls_key_file)[[:space:]]*:' $(printf '%q' "$CFG_REMOTO") 2>/dev/null | tr -d ' '" 2>/dev/null || true)"
else
  POSTURA_TLS="$(grep -E '^[[:space:]]*(allow_insecure_token|tls_cert_file|tls_key_file)[[:space:]]*:' "$CFG_REMOTO" 2>/dev/null | tr -d ' ' || true)"
fi
if [ -z "$POSTURA_TLS" ]; then
  dudoso "no se pudo leer $CFG_REMOTO: no sé si el cerebro sirve TLS (probá MUSUBI_CFG=<ruta>)"
elif printf '%s' "$POSTURA_TLS" | grep -q '^tls_cert_file:.\+'; then
  verde "el cerebro tiene certificado configurado — el bearer viaja cifrado de extremo a extremo"
elif printf '%s' "$POSTURA_TLS" | grep -q '^allow_insecure_token:true'; then
  if [ "${MUSUBI_EXIGIR_TLS:-0}" = "1" ]; then
    rojo 'allow_insecure_token: true con MUSUBI_EXIGIR_TLS=1 — se exigió TLS y el cerebro sirve HTTP en claro'
  else
    tibio 'allow_insecure_token: true — el bearer viaja en texto plano (lo cifra el tailnet, no el transporte). Migrar: `tailscale cert` en el nodo, tls_cert_file/tls_key_file en el config, MUSUBI_BRAIN_TLS_NAME en los agentes, y prender MUSUBI_EXIGIR_TLS=1 acá'
  fi
else
  verde 'allow_insecure_token no está en true'
fi

# ── 6 · ¿VUELVE SOLA DE UN REBOOT? ──────────────────────────────────────────────────────────
#
# Medido el 2026-09-03, y el resultado contradijo lo que el plan suponía: la cadena SÍ vuelve. En
# el reboot del 2026-08-31 03:17:10, Alertmanager arrancó a las 03:18:05 —55 segundos después— y
# el relay de pantalla con él. Lo hace `podman-restart.service`, que NO filtra por
# `restart-policy=always` como suele creerse, sino por `should-start-on-boot=true`, y ESE filtro
# incluye `unless-stopped`. Por eso `restart: unless-stopped` alcanza y NO hay que cambiarlo por
# `always`: `always` levanta hasta lo que alguien paró a propósito, que es peor.
#
# LO QUE SE VIGILA ACÁ NO ES LA DISPONIBILIDAD, ES LA REPRODUCIBILIDAD. Esa configuración —el
# linger del usuario y el servicio habilitado— se puso A MANO y no vive en ningún archivo del
# repo. Un servidor reconstruido, o una migración al VPS, la pierde EN SILENCIO: todo sigue
# andando hasta el primer corte de luz, y ahí la cadena no vuelve y el watchdog externo tampoco,
# porque el watchdog vive adentro de la misma máquina que se apagó.
#
# Es ROJO y no amarillo: no es «no pude preguntar», es una divergencia real contra el estado que
# el despliegue declara querer.
printf '\n\033[1mvuelve sola de un reboot\033[0m\n'
corre_alla() {  # corre un comando en el servidor si hay SSH_HOST, o acá si no
  if [ -n "$SSH_HOST" ]; then
    # EL `-n` NO ES DECORACIÓN: SIN ÉL ESTE INFORME DEJA DE MIRAR COSAS Y NO LO DICE.
    #
    # `ssh` sin `-n` lee stdin y se lo lleva entero. Cuando `corre_alla` se llama DESDE ADENTRO de
    # un bucle que lee de un heredoc —el de «guiones derivados», línea ~766— el primer `ssh` se
    # come el resto de la lista, el `read` no encuentra más renglones y el bucle termina.
    #
    # No falla: TERMINA. No hay línea roja, ni amarilla, ni verde — no hay línea. Medido el
    # 2026-09-08: las últimas 3 corridas compararon `/usr/local/bin/musubi-backup` y NUNCA
    # `/usr/local/sbin/redesplegar-cerebro.sh`, que es exactamente el archivo cuya deriva FUE A111.
    # O sea que el agujero que A111 cerró volvió a quedar sin vigilancia, y el informe se veía igual.
    #
    # Es intermitente porque depende de si el `ssh` alcanza a leer antes de que el `read` lo haga,
    # y por eso pasó tres corridas sin que nadie lo notara. La cabecera de este guion ya lo dice
    # con todas las letras: «un informe que calla lo que no mira se lee como si lo hubiera mirado».
    ssh -n -o BatchMode=yes -o ConnectTimeout=10 "$SSH_HOST" "$1" 2>/dev/null || true
  else
    eval "$1" 2>/dev/null || true
  fi
}
LINGER="$(corre_alla 'loginctl show-user "$(id -un)" --property=Linger 2>/dev/null')"
# LOS DOS UNITS POR SEPARADO, y el que manda es el de USUARIO. Los contenedores de acá son
# rootless (corren como el usuario del cerebro, no como root), así que el `podman-restart` del
# sistema no los levanta. Juntar las dos salidas en una sola variable daba un VERDE FALSO cuando
# el de usuario estaba apagado y el del sistema no — se cazó saboteando esta misma sección.
PRESTART="$(corre_alla 'systemctl --user is-enabled podman-restart.service 2>/dev/null')"
PRESTART_SIS="$(corre_alla 'systemctl is-enabled podman-restart.service 2>/dev/null')"

if [ -z "$LINGER" ] && [ -z "$PRESTART" ]; then
  dudoso "no se pudo preguntar si la cadena vuelve de un reboot (hace falta correr esto EN el servidor, o con MUSUBI_SSH=<host>)"
else
  if printf '%s' "$LINGER" | grep -q 'Linger=yes'; then
    verde "linger activo — los servicios de usuario sobreviven al cierre de sesión"
  elif [ -z "$LINGER" ]; then
    dudoso "no se pudo leer el linger del usuario"
  else
    rojo "linger APAGADO: al cerrar sesión se caen los servicios de usuario, y en el próximo reboot la cadena no vuelve. Arreglo: loginctl enable-linger \$(id -un)"
  fi

  # `= enabled` exacto, no `grep enabled`: «disabled» no contiene «enabled» pero «enabled-runtime»
  # sí, y ése NO sobrevive un reboot — es justo el caso que este chequeo existe para cazar.
  if [ "$PRESTART" = "enabled" ]; then
    verde "podman-restart (usuario) habilitado — los contenedores con should-start-on-boot vuelven al arrancar"
  elif [ -z "$PRESTART" ]; then
    dudoso "no se pudo leer el estado de podman-restart.service del usuario"
  else
    rojo "podman-restart del USUARIO está en «$PRESTART»: después de un reboot los contenedores rootless NO vuelven solos —ni Prometheus, ni Alertmanager, ni el watchdog externo, que vive adentro de esta misma máquina—. El unit del sistema (hoy «${PRESTART_SIS:-?}») NO los cubre: son rootless. Arreglo: systemctl --user enable podman-restart.service"
  fi
fi

# ── Los guiones DERIVADOS: el archivo que se va a correr contra el que el repo declara ──────
#
# POR QUÉ ESTO ESTÁ ACÁ Y NO EN LA LISTA DE «lo que no se mira» (A111).
#
# El encabezado de este script dice que NO mira los archivos del servidor, y da la razón: se le
# pregunta a Prometheus qué CARGÓ, porque un archivo correcto que el daemon no releyó se ve igual
# que uno bueno. Esa razón es cierta para una configuración que un daemon lee al arrancar.
#
# PARA UN GUION DE SHELL NO APLICA, y confundir los dos casos costó lo siguiente: medido el
# 2026-09-05, `/home/musubi/redesplegar-cerebro.sh` tenía 9690 bytes contra 19469 del repo, y su
# verificación de la migración decía `[[ "$ESQUEMA" -ge 37 ]]` cuando la base ya iba por 46 — o
# sea VACUAMENTE CIERTA: pasaba sin comprobar nada, y pasó así en los seis redespliegues del 4 y
# del 5. El arreglo estaba en el repo hacía días. No hay daemon que relea un guion: el archivo ES
# lo que corre, en el momento en que alguien lo invoca.
#
# Y LA DERIVA NO FUE GENERAL, QUE ES LO QUE LA HIZO INVISIBLE. De los dos guiones derivados,
# `musubi-backup` coincidía byte a byte con el repo y el del redespliegue no. La diferencia no es
# suerte: el backup lo instala `install-musubi-brain.sh` detrás de una compuerta de sha256, y el
# del redespliegue llegó a mano y no lo instalaba nadie. Un guion sin instalador no tiene cómo
# actualizarse, y sin esta sección tampoco tenía cómo delatarse.
#
# CUATRO RESPUESTAS Y NO DOS. «coincide», «difiere», «no está» y «no pude preguntar» se arreglan
# de cuatro maneras distintas, y las últimas dos son las que un verificador tiende a confundir con
# un verde. `corre_alla` se traga los errores con `|| true`, así que una respuesta VACÍA acá
# significa «no pude preguntar» y sale por `dudoso`, nunca por verde.
titulo "guiones derivados (el repo contra el archivo que se corre)"

# sha_alla <ruta> — el sha256 del archivo en el servidor, o AUSENTE, o ILEGIBLE, o vacío si no se
# pudo preguntar. Los tres estados van por stdout y el cuarto es la ausencia de stdout: mezclarlos
# es exactamente cómo un chequeo remoto termina en verde sin haber mirado.
sha_alla() {
  corre_alla "if [ ! -e '$1' ]; then echo AUSENTE; elif [ ! -r '$1' ]; then echo ILEGIBLE; else sha256sum '$1' 2>/dev/null | awk '{print \$1}'; fi"
}

# La tabla de guiones derivados: <archivo del repo>|<ruta canónica en el servidor>.
# La custodia `TestCadaGuionQueSeInstalaEnElServidorSeCompara`: todo destino que un instalador
# escriba con `install` tiene que aparecer acá. Una lista a mano sin guarda es cómo se coló A93.
GUIONES_DERIVADOS="deploy/musubi-backup.sh|/usr/local/bin/musubi-backup
deploy/redesplegar-cerebro.sh|/usr/local/sbin/redesplegar-cerebro.sh"

# Si falta `sha256sum` allá, TODAS las respuestas vienen vacías y todas dirían «no pude preguntar»
# por la misma causa. Se pregunta una vez para poder nombrarla, en vez de repetir N veces un
# diagnóstico que no distingue entre «no hay ssh» y «no hay sha256sum».
HAY_SHA_ALLA="$(corre_alla 'command -v sha256sum >/dev/null 2>&1 && echo si')"

while IFS='|' read -r rel destino; do
  [ -n "$rel" ] || continue
  if [ ! -f "$REPO/$rel" ]; then
    rojo "$rel no existe en el repo, y $destino se compara contra él: o se renombró el guion y esta tabla quedó vieja, o se borró y el servidor sigue corriendo una copia que ya no tiene fuente"
    continue
  fi
  SHA_REPO="$(sha256sum "$REPO/$rel" | awk '{print $1}')"
  SHA_ALLA="$(sha_alla "$destino")"
  case "$SHA_ALLA" in
    "")
      if [ "$HAY_SHA_ALLA" != "si" ] && [ -n "$SSH_HOST" ]; then
        dudoso "no se pudo comparar $destino: en el servidor no hay \`sha256sum\` (probá con \`shasum -a 256\`, o corré esto EN el servidor)"
      else
        dudoso "no se pudo preguntar por $destino (hace falta correr esto EN el servidor, o con MUSUBI_SSH=<host>)"
      fi ;;
    AUSENTE)
      rojo "$destino NO existe en el servidor. El repo declara $rel y allá no hay nada: si alguien necesita ese guion hoy, no lo tiene — y si tiene una copia en otra ruta, es una copia que nadie compara" ;;
    ILEGIBLE)
      dudoso "$destino existe pero no se pudo leer con el usuario de esta sesión: no se comparó" ;;
    "$SHA_REPO")
      verde "$destino coincide byte a byte con $rel" ;;
    *)
      rojo "$destino DIFIERE de $rel — allá $SHA_ALLA, acá $SHA_REPO. No hay daemon que relea un guion: eso es lo que corre la próxima vez que alguien lo invoque. Para ver qué cambió:  ${SSH_HOST:+ssh $SSH_HOST }cat $destino | diff - $REPO/$rel" ;;
  esac
done <<GUIONES
$GUIONES_DERIVADOS
GUIONES

# ── LOS ARCHIVOS DE REGLAS, POR CONTENIDO Y NO SÓLO POR NOMBRE ───────────────────────────────
#
# LA SECCIÓN 2 COMPARA LOS **NOMBRES** DE LAS REGLAS CARGADAS. ESO DEJA PASAR EL CASO QUE MÁS
# DUELE: mismo juego de nombres, distinto NÚMERO adentro.
#
# Ya pasó tres veces. La tercera fue el 2026-09-08 y la causé yo desplegando a mano: copié
# `musubi-alerts-flota.yml` (27→28 reglas) y NO su hermano `musubi-alerts.yml`, que es el que
# lleva el umbral cruzado `!= 28`. El informe decía `✔ musubi-alerts.yml — sus 23 reglas están
# cargadas` —cierto, los 23 nombres estaban— mientras el archivo desplegado difería en el renglón
# que decide, y `ReglasDeFlotaSinDesplegar` quedó disparando con razón sin que el verificador
# pudiera explicar por qué. La segunda fue el 2026-09-05: `!= 24` contra `!= 27`, **con los dos
# archivos pesando los mismos 23912 bytes**.
#
# Y LA PREMISA QUE SOSTENÍA TODO ESTO NO LA CUSTODIABA NADA. `musubi-alerts.yml:374-377` dice que
# las dos guardas cruzadas funcionan porque «el despliegue copia los dos archivos JUNTOS, así que
# envejecen juntos». Es cierto para `preparar.sh`, que los instala en el mismo bloque — y es una
# CONVENCIÓN DEL PROCEDIMIENTO, no una guarda: un despliegue a mano de un solo archivo la rompe, y
# nada se enteraba. Comparar por sha256 no impide romperla; hace que romperla se VEA, que es lo
# único que este guion puede prometer.
#
# El sha es válido porque los archivos se copian VERBATIM (`preparar.sh` usa `install`, sin
# sustituciones). El único que se edita al instalar es `alertmanager.yml` —el `chat_id`— y ése no
# está acá.
DIR_REGLAS_ALLA="$(corre_alla 'for d in "$HOME/musubi-prometheus/rules" /etc/musubi-prometheus/rules /etc/prometheus/rules; do [ -d "$d" ] && { printf "%s\n" "$d"; break; }; done')"
if [ -z "$DIR_REGLAS_ALLA" ]; then
  # NO SE CALLA. Un «no encontré dónde mirar» que no se dice se lee como «miré y estaba bien»,
  # que es el defecto que este guion entero viene a cerrar.
  dudoso "no se encontró el directorio de reglas en el servidor (probé \$HOME/musubi-prometheus/rules, /etc/musubi-prometheus/rules y /etc/prometheus/rules): los archivos de reglas se compararon sólo por NOMBRE, así que un umbral cambiado con los mismos nombres NO se habría visto"
elif [ "$HAY_SHA_ALLA" != si ]; then
  dudoso "no se compararon los archivos de reglas por contenido: en el servidor no hay \`sha256sum\`"
else
  for f_r in "$REPO"/deploy/musubi-alerts*.yml "$REPO"/deploy/musubi-recording.yml; do
    [ -f "$f_r" ] || continue
    nombre_r="$(basename "$f_r")"
    cond_r="$(sed -n 's/^#[[:space:]]*despliegue:[[:space:]]*//p' "$f_r" | head -1)"
    sha_repo_r="$(sha256sum "$f_r" | awk '{print $1}')"
    sha_serv_r="$(sha_alla "$DIR_REGLAS_ALLA/$nombre_r")"
    case "$sha_serv_r" in
      "")
        dudoso "no se pudo preguntar por $nombre_r en $DIR_REGLAS_ALLA: no se comparó su contenido" ;;
      AUSENTE)
        # Que no esté puede ser correcto: los condicionales sólo se instalan si su condición se
        # cumple. La diferencia la declara el propio archivo, igual que en la sección 2.
        case "$cond_r" in
          siempre) rojo "$nombre_r NO está en $DIR_REGLAS_ALLA y se declara «siempre»: el archivo que el repo da por desplegado no existe en el servidor" ;;
          condicional*) gris "$nombre_r — no está en el servidor, y así corresponde: $cond_r" ;;
          *) rojo "$nombre_r no está en el servidor y no declara su condición de despliegue (# despliegue:)" ;;
        esac ;;
      ILEGIBLE)
        dudoso "$nombre_r existe en $DIR_REGLAS_ALLA pero no se pudo leer con el usuario de esta sesión: no se comparó" ;;
      "$sha_repo_r")
        verde "$nombre_r coincide byte a byte con deploy/$nombre_r" ;;
      *)
        rojo "$nombre_r DIFIERE del repo — allá $sha_serv_r, acá $sha_repo_r. Ojo: sus reglas pueden figurar CARGADAS más arriba y ser cierto, porque eso compara NOMBRES; lo que cambia acá es el contenido —un umbral, un \`for:\`, una anotación—. Para ver qué:  ${SSH_HOST:+ssh $SSH_HOST }cat $DIR_REGLAS_ALLA/$nombre_r | diff - $REPO/deploy/$nombre_r" ;;
    esac
  done
fi

# LA COPIA VIEJA EN EL HOME DEL USUARIO DEL CEREBRO, que es una divergencia Y ADEMÁS otra cosa.
#
# `redesplegar-cerebro.sh` vivió en `/home/musubi/` y se corre con `sudo`. Esa combinación —un
# archivo en un directorio que escribe el usuario `musubi`, ejecutado como root— es un camino de
# escalada, y no es teórico en esta flota: `musubi_fleet_exec` en el servidor corre EXACTAMENTE
# como `uid=1000(musubi)` (medido en A111). O sea que quien alcance el canal del agente puede
# dejar código escrito ahí, y root lo corre en el próximo redespliegue. Por eso la ruta canónica
# pasó a `/usr/local/sbin`, que es de root, y por eso una copia sobreviviente es ROJA aunque su
# contenido esté al día: el problema no es qué dice, es quién puede reescribirla.
LEGADO="/home/musubi/redesplegar-cerebro.sh"
SHA_LEGADO="$(sha_alla "$LEGADO")"
case "$SHA_LEGADO" in
  "")       dudoso "no se pudo preguntar si quedó la copia vieja en $LEGADO" ;;
  AUSENTE)  verde "no quedó ninguna copia de redespliegue en el home de \`musubi\` (la ruta canónica es /usr/local/sbin, que es de root)" ;;
  # EL COMANDO QUE SE SUGIERE VA SIN `sudo`, Y EL PROPIO DIAGNÓSTICO DE ARRIBA DICE POR QUÉ.
  #
  # Decía `sudo rm`, y eso fallaba de dos formas distintas el 2026-09-09:
  #
  #   $ ssh musubi-server sudo rm /home/musubi/redesplegar-cerebro.sh
  #   sudo: a terminal is required to read the password; either use ssh's -t option…
  #
  # (1) Un `ssh` no interactivo no tiene TTY, así que `sudo` no puede pedir la contraseña — haría
  # falta `ssh -t`. (2) Y sobre todo: EL `sudo` NO HACE FALTA. Este hallazgo dice, con todas las
  # letras, que el archivo «está en un directorio que escribe el usuario `musubi`». Si `musubi`
  # escribe ahí, `musubi` lo borra: para desenlazar un archivo manda el permiso del DIRECTORIO, no
  # el del archivo. Medido: `/home/musubi` es `drwx------ musubi musubi`, y la sesión entra como
  # `musubi`.
  #
  # O sea que la sugerencia se contradecía con su propia medición, y encima no corría. Un informe
  # que nombra bien el problema y manda a un comando que falla gasta la confianza que se ganó en la
  # línea anterior — y lo peor es que el que lo lee no sabe si falló el diagnóstico o el remedio.
  #
  # `-f` y no `rm` a secas: si una corrida anterior con sudo dejó el archivo de root, `rm` pediría
  # confirmación por escribir sobre algo protegido y en un pipe eso se cuelga. El desenlace igual
  # funciona, porque lo autoriza el directorio.
  *)        rojo "quedó una copia en $LEGADO. Son DOS cosas: alguien puede correr ésa en vez de la canónica sin notarlo, y está en un directorio que escribe el usuario \`musubi\` —el mismo uid con el que corre \`musubi_fleet_exec\`— para un guion que se invoca con sudo. Sacala:  ${SSH_HOST:+ssh $SSH_HOST }rm -f $LEGADO" ;;
esac

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
# La postura NO cambia el código de salida por default, y decirlo es parte del informe: un verde
# que además tiene un `~` arriba no es un verde entero, y quien lo automatice tiene que poder
# leerlo sin adivinar. Se exige con MUSUBI_EXIGIR_TLS=1.
if [ "$POSTURA" -ne 0 ]; then
  printf '\033[33mcon una salvedad de postura\033[0m (el `~` de arriba): no divergió nada, pero el transporte está como está. Se exige con MUSUBI_EXIGIR_TLS=1\n'
fi
exit 0
