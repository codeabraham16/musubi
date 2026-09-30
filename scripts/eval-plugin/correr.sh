#!/usr/bin/env bash
# correr.sh — la suite que mide si el plugin de Musubi mejora al agente, con `claude plugin eval`:
# cada caso corre CON el plugin y SIN él, y el Δ entre los dos brazos es lo que aporta.
#
# SIN --confirmar NO GASTA NADA: arma el plugin de prueba con sus casos, muestra el plan, la cuenta
# del costo y el comando exacto, y sale. Con --confirmar corre la evaluación, que sí cuesta plata
# (ver README.md de esta carpeta). Leé el README antes de la primera corrida.
#
# Uso: scripts/eval-plugin/correr.sh [--confirmar] [--casos GLOB] [--corridas N] [--techo-usd USD]
#          [-j N] [--modelo M] [--modelo-juez M] [--claude RUTA] [--musubi RUTA] [--conservar]
set -euo pipefail

aqui="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

uso() {
	sed -n '2,10p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
	cat <<'FIN'

  --confirmar        corre la evaluación de verdad (gasta; sin esto sólo muestra el plan)
  --casos GLOB       sólo los casos cuyo nombre coincide (el --case de plugin eval)
  --corridas N       corridas por caso y por brazo (1..50; por defecto 1, el piloto)
  --techo-usd USD    techo de gasto de la corrida (por defecto 5); al pasarlo no arranca nada más
  -j N               corridas simultáneas (1..8; por defecto 1: cada una es un claude entero)
  --modelo M         modelo del agente (fijalo para comparar corridas entre sí)
  --modelo-juez M    modelo de los graders llm (por defecto el de plugin eval: haiku)
  --claude RUTA      el claude a usar (o CLAUDE_BIN); hace falta uno con `plugin eval`
  --musubi RUTA      el binario de musubi del plugin y de los andamios (o MUSUBI_BIN, o el del PATH)
  --conservar        deja las carpetas de cada corrida para mirarlas (--keep-temp)
FIN
}

falla() {
	printf 'correr.sh: %s\n' "$*" >&2
	exit 1
}

confirmar=0
casos=''
corridas=1
techo=5
concurrencia=1
modelo=''
modelo_juez=''
claude_bin="${CLAUDE_BIN:-}"
musubi_bin="${MUSUBI_BIN:-}"
conservar=0
while [ "$#" -gt 0 ]; do
	case "$1" in
	--confirmar) confirmar=1 ;;
	--casos) casos="${2:?falta el valor de --casos}"; shift ;;
	--corridas) corridas="${2:?falta el valor de --corridas}"; shift ;;
	--techo-usd) techo="${2:?falta el valor de --techo-usd}"; shift ;;
	-j) concurrencia="${2:?falta el valor de -j}"; shift ;;
	--modelo) modelo="${2:?falta el valor de --modelo}"; shift ;;
	--modelo-juez) modelo_juez="${2:?falta el valor de --modelo-juez}"; shift ;;
	--claude) claude_bin="${2:?falta el valor de --claude}"; shift ;;
	--musubi) musubi_bin="${2:?falta el valor de --musubi}"; shift ;;
	--conservar) conservar=1 ;;
	-h | --help) uso; exit 0 ;;
	*) falla "opción desconocida: $1 (mirá --help)" ;;
	esac
	shift
done

case "${corridas}" in
'' | *[!0-9]*) falla "--corridas tiene que ser un entero de 1 a 50: ${corridas}" ;;
esac
[ "${corridas}" -ge 1 ] && [ "${corridas}" -le 50 ] || falla "--corridas tiene que ir de 1 a 50: ${corridas}"
case "${concurrencia}" in
'' | *[!0-9]*) falla "-j tiene que ser un entero de 1 a 8: ${concurrencia}" ;;
esac
[ "${concurrencia}" -ge 1 ] && [ "${concurrencia}" -le 8 ] || falla "-j tiene que ir de 1 a 8: ${concurrencia}"
[[ ${techo} =~ ^[0-9]+([.][0-9]+)?$ ]] || falla "--techo-usd tiene que ser un número: ${techo}"
LC_ALL=C awk -v t="${techo}" 'BEGIN { exit !(t > 0) }' || falla "--techo-usd tiene que ser mayor que cero"

# --- Las herramientas -----------------------------------------------------------------------------

# claude: hace falta uno con `plugin eval` y con los flags de los que depende el aislamiento. Pedir
# la ayuda no gasta nada, y es lo único que este guion le pide a claude sin --confirmar.
if [ -z "${claude_bin}" ]; then
	claude_bin="$(command -v claude || true)"
fi
[ -n "${claude_bin}" ] && [ -x "${claude_bin}" ] || falla "no encuentro claude: pasale --claude RUTA"
# (Sin `| head` bajo pipefail: si cortara antes de que el otro termine de escribir, el SIGPIPE
# haría fallar la asignación y el guion saldría callado.)
claude_version="$("${claude_bin}" --version 2>&1 || true)"
claude_version="${claude_version%%$'\n'*}"
ayuda="$("${claude_bin}" plugin eval --help 2>&1)" ||
	falla "este claude (${claude_version}) no tiene \`plugin eval\`: hace falta 2.1.269 o más nuevo; pasale --claude RUTA"
for flag in --no-publish --ablation --max-cost-usd --mocks --scaffold --trust-plugin --allow-tools --output-dir --threshold --runs --case; do
	[[ ${ayuda} == *"${flag}"* ]] ||
		falla "el \`plugin eval\` de este claude (${claude_version}) no conoce ${flag}: hace falta 2.1.269 o más nuevo; pasale --claude RUTA"
done

# git: plugin eval se niega con uno anterior a 2.31.
git_version="$(git --version | awk '{ print $3 }')"
LC_ALL=C awk -v v="${git_version}" 'BEGIN { split(v, p, "."); exit !(p[1] > 2 || (p[1] == 2 && p[2] >= 31)) }' ||
	falla "git ${git_version} es anterior a 2.31: plugin eval no corre con él"

# musubi: el MISMO binario arma el plugin (queda escrito en sus hooks y en su .mcp.json) y siembra la
# memoria en los andamios. Con dos binarios distintos, la base sembrada podría tener un esquema que el
# daemon del plugin no abre.
if [ -z "${musubi_bin}" ]; then
	musubi_bin="$(command -v musubi || true)"
fi
[ -n "${musubi_bin}" ] || falla "no encuentro musubi: pasale --musubi RUTA"
musubi_bin="$(readlink -f "${musubi_bin}")"
[ -x "${musubi_bin}" ] || falla "no es ejecutable: ${musubi_bin}"
musubi_version="$("${musubi_bin}" version 2>&1)" || falla "${musubi_bin} version salió con error: ${musubi_version}"
musubi_version="${musubi_version%%$'\n'*}"

# --- El entorno de la corrida ---------------------------------------------------------------------

# Se saca por NOMBRE todo lo que apunte a una memoria, a un cerebro o a una credencial de Musubi
# (*MUSUBI*: MUSUBI_TOKEN, MUSUBI_CENTRAL_URL, MUSUBI_HOME, ALTURA_MUSUBI_TOKEN_FILE...) y lo que ata
# el proceso a la sesión de Claude Code desde la que se lo lanza. plugin eval ya filtra el entorno
# con su propia lista, pero esa lista deja pasar casi todo CLAUDE_CODE_*: esto no depende de ella.
sin_entorno=()
quitadas=()
for variable in $(compgen -e); do
	case "${variable}" in
	*MUSUBI* | CLAUDE_PROJECT_DIR | CLAUDE_PLUGIN_ROOT | CLAUDE_PLUGIN_DATA | CLAUDE_CODE_MESSAGING_* | \
		CLAUDE_CODE_SESSION_ID | CLAUDE_CODE_CHILD_SESSION | CLAUDE_CODE_SESSION_ATTENDED | CLAUDE_PID)
		sin_entorno+=(-u "${variable}")
		quitadas+=("${variable}")
		;;
	esac
done

# El control, contra lo que de verdad le llega a un proceso lanzado igual que plugin eval: `env`
# lista también los nombres que bash no importa como variables y que el bucle de arriba no ve.
quedan=''
if env -0 true >/dev/null 2>&1; then
	while IFS= read -r -d '' entrada; do
		case "${entrada%%=*}" in *MUSUBI*) quedan="${quedan} ${entrada%%=*}" ;; esac
	done < <(env ${sin_entorno[@]+"${sin_entorno[@]}"} env -0)
else
	# Sin `env -0`, un valor con saltos de línea puede dar un falso positivo; nunca uno negativo.
	while IFS= read -r entrada; do
		case "${entrada%%=*}" in *MUSUBI*) quedan="${quedan} ${entrada%%=*}" ;; esac
	done < <(env ${sin_entorno[@]+"${sin_entorno[@]}"} env)
fi
[ -z "${quedan}" ] || falla "después de filtrar el entorno todavía le llegan al proceso:${quedan}"

# --- El plugin de prueba --------------------------------------------------------------------------

# Una carpeta fija, fuera del repo, que se rehace entera en cada corrida. Sólo se borra si la marca
# (al lado, no adentro: `musubi agente instalar` no pisa una carpeta con archivos ajenos) dice que
# la armó este guion; se escribe ANTES de instalar, así una instalación a medias también se limpia.
base="${XDG_CACHE_HOME:-${HOME}/.cache}/musubi-eval-plugin"
plugin="${base}/plugin"
marca="${base}/.plugin-armado-por-correr-sh"
if [ -e "${plugin}" ]; then
	[ -f "${marca}" ] || falla "${plugin} existe y no lo armó este guion: no lo toco"
	rm -rf "${plugin}"
fi
mkdir -p "${base}"
repo_del_guion="$(git -C "${aqui}" rev-parse --show-toplevel 2>/dev/null || true)"
repo_de_la_base="$(git -C "${base}" rev-parse --show-toplevel 2>/dev/null || true)"
if [ -n "${repo_de_la_base}" ] && [ "${repo_de_la_base}" = "${repo_del_guion}" ]; then
	falla "la carpeta de trabajo (${base}) queda dentro de este repo"
fi
: >"${marca}"

"${musubi_bin}" agente instalar --dir "${plugin}" --settings "${base}/settings-descartable.json" \
	--sin-permisos --compactar-en 0 >"${base}/instalar.log" 2>&1 ||
	falla "musubi agente instalar falló: $(tail -n 3 "${base}/instalar.log")"
grep -q '"name": *"musubi"' "${plugin}/.claude-plugin/plugin.json" ||
	falla "el plugin armado no se llama musubi: sus tools no serían mcp__plugin_musubi_musubi__*"
grep -q '"musubi": *{' "${plugin}/.mcp.json" ||
	falla "el plugin armado no declara el servidor musubi en .mcp.json"

# Los casos: cada andamio.sh se arma con una cabecera (el binario y el caso), la biblioteca común y
# el cuerpo del caso. El resto del caso se copia tal cual.
mkdir -p "${plugin}/evals"
nombres=()
for dir in "${aqui}"/evals/*/; do
	caso="$(basename "${dir}")"
	destino="${plugin}/evals/${caso}"
	mkdir -p "${destino}"
	cp -R "${dir}." "${destino}/"
	{
		printf '#!/usr/bin/env bash\n'
		printf '# ARMADO por scripts/eval-plugin/correr.sh: no se edita acá.\n'
		printf 'ANDAMIO_BIN=%q\n' "${musubi_bin}"
		printf 'CASO=%q\n' "${caso}"
		cat "${aqui}/lib/andamio-comun.sh"
		cat "${dir}andamio.sh"
	} >"${destino}/andamio.sh"
	chmod +x "${destino}/andamio.sh"
	nombres+=("${caso}")
done
[ "${#nombres[@]}" -gt 0 ] || falla "no hay casos en ${aqui}/evals"

# --- Las tools que se conceden --------------------------------------------------------------------

# Write y Edit para los casos que escriben código. De Musubi, sólo las que trabajan sobre la memoria
# y el código DEL WORKSPACE: nada de flota, tokens, sincronización, cerebro ni motor cognitivo, que
# salen de la máquina o necesitan credenciales. La excepción es musubi_search_skills, que lee
# catálogos públicos: es lo que el arranque de un proyecto nuevo le pide al agente, y
# control-primer-contacto mide justamente eso.
tools_de_musubi=(
	musubi_recall musubi_memory_expand musubi_save_observation musubi_propose_observation
	musubi_judge musubi_corroborate musubi_discard_proposal musubi_conflicts
	musubi_search_keyword musubi_search_semantic musubi_entity_context
	musubi_recall_facts musubi_save_fact musubi_propose_facts musubi_save_code musubi_recall_code
	musubi_impact musubi_code_graph musubi_code_context musubi_codegraph_index musubi_map
	musubi_detect_changes musubi_sdd musubi_phase musubi_work
	musubi_list_skills musubi_save_skill musubi_log_skill_decision musubi_search_skills
)

# Una concesión con un nombre que no existe no da error: sólo no concede nada, y el caso mediría otra
# cosa. Se comparan contra las que ofrece el daemon, en una carpeta descartable. Ese daemon activa
# una memoria nueva, y el que activa una memoria consulta la API de GitHub por versiones nuevas: va
# con un proxy muerto en 127.0.0.1:9, para que el modo sin --confirmar no salga a la red.
sonda="$(mktemp -d)"
trap 'rm -rf "${sonda}"' EXIT
git -C "${sonda}" init -q .
printf '%s\n' \
	'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"correr-sh","version":"1"}}}' \
	'{"jsonrpc":"2.0","method":"notifications/initialized"}' \
	'{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' >"${sonda}/entrada.jsonl"
(cd "${sonda}" && env ${sin_entorno[@]+"${sin_entorno[@]}"} MUSUBI_HOME="${sonda}" HOME="${sonda}" \
	HTTPS_PROXY=http://127.0.0.1:9 https_proxy=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 \
	http_proxy=http://127.0.0.1:9 NO_PROXY= no_proxy= \
	"${musubi_bin}" daemon <entrada.jsonl >salida.jsonl 2>daemon.err) ||
	falla "no pude listar las tools del daemon de musubi: $(tail -n 3 "${sonda}/daemon.err")"
grep -q '"id":2,' "${sonda}/salida.jsonl" || falla "el daemon de musubi no contestó tools/list"
ofrecidas="$(grep -o '"name":"musubi_[a-z_]*"' "${sonda}/salida.jsonl" | sort -u || true)"
[ -n "${ofrecidas}" ] || falla "el daemon de musubi no ofreció ninguna tool (¿arrancó degradado?)"
permitidas=(Write Edit)
for tool in "${tools_de_musubi[@]}"; do
	case $'\n'"${ofrecidas}"$'\n' in
	*$'\n'"\"name\":\"${tool}\""$'\n'*) ;;
	*) falla "${musubi_version} no ofrece ${tool}: actualizá la lista de correr.sh" ;;
	esac
	permitidas+=("mcp__plugin_musubi_musubi__${tool}")
done

# --- La cuenta ------------------------------------------------------------------------------------

# Cada case.yaml declara en un comentario cuánto cuesta UNA corrida de UN brazo (mínimo, típico y
# máximo, en USD a precio de lista). Total = Σ de los casos elegidos × corridas × 2 brazos, más los
# jueces de los graders llm: 3 votos por grader, corrida y brazo, con el modelo chico (0,005 USD).
elegidos=()
suma_min=0
suma_tipico=0
suma_max=0
jueces=0
for caso in "${nombres[@]}"; do
	# shellcheck disable=SC2053 # el glob de --casos se compara como patrón, igual que --case
	if [ -n "${casos}" ] && [[ ${caso} != ${casos} ]]; then
		continue
	fi
	yaml="${plugin}/evals/${caso}/case.yaml"
	rango="$(sed -n 's/^# costo-usd-por-corrida: *\([0-9.][0-9.]*\) *\([0-9.][0-9.]*\) *\([0-9.][0-9.]*\).*/\1 \2 \3/p' "${yaml}")"
	[ -n "${rango}" ] || falla "${caso}/case.yaml no declara su costo (# costo-usd-por-corrida: MIN TÍPICO MAX)"
	read -r c_min c_tipico c_max <<<"${rango}"
	suma_min="$(LC_ALL=C awk -v a="${suma_min}" -v b="${c_min}" 'BEGIN { print a + b }')"
	suma_tipico="$(LC_ALL=C awk -v a="${suma_tipico}" -v b="${c_tipico}" 'BEGIN { print a + b }')"
	suma_max="$(LC_ALL=C awk -v a="${suma_max}" -v b="${c_max}" 'BEGIN { print a + b }')"
	for grader in "${plugin}/evals/${caso}"/graders/*.md; do
		if [ -f "${grader}" ] && grep -q '^type: *llm' "${grader}"; then
			jueces=$((jueces + 1))
		fi
	done
	elegidos+=("${caso}")
done
[ "${#elegidos[@]}" -gt 0 ] || falla "--casos '${casos}' no coincide con ningún caso: ${nombres[*]}"
corridas_totales=$((${#elegidos[@]} * corridas * 2))
costos="$(LC_ALL=C awk -v mn="${suma_min}" -v ti="${suma_tipico}" -v mx="${suma_max}" -v ju="${jueces}" -v n="${corridas}" '
	BEGIN { f = n * 2; j = ju * 3 * 0.005 * f; printf "%.2f %.2f %.2f", mn * f + j, ti * f + j, mx * f + j }')"
read -r costo_min costo_tipico costo_max <<<"${costos}"

# --- El comando -----------------------------------------------------------------------------------

resultados="${base}/resultados/$(date +%Y%m%d-%H%M%S)"
cabeza=("${claude_bin}" plugin eval "${plugin}")
# --ablation va explícito: el brazo SIN plugin es la mitad de la medición y su default ya cambió una
# vez. En 2.1.223 era `none` para un objetivo por ruta, que es como se evalúa acá; en 2.1.285 es
# `with-without` sólo si el plugin «resuelve» desde la ruta, y `none` si no.
opciones=(--no-publish --ablation with-without --scaffold --mocks off --trust-plugin --threshold 0
	-j "${concurrencia}" --runs "${corridas}" --max-cost-usd "${techo}" --output-dir "${resultados}")
if [ -n "${casos}" ]; then opciones+=(--case "${casos}"); fi
if [ -n "${modelo}" ]; then opciones+=(--model "${modelo}"); fi
if [ -n "${modelo_juez}" ]; then opciones+=(--judge-model "${modelo_juez}"); fi
if [ "${conservar}" = 1 ]; then opciones+=(--keep-temp); fi
# --allow-tools VA ÚLTIMO: toma una lista y se comería lo que viniera detrás.
concesion=(--allow-tools "${permitidas[@]}")

cat <<FIN
Suite de evals del plugin de Musubi
  claude:    ${claude_bin} (${claude_version})
  musubi:    ${musubi_bin} (${musubi_version})
  git:       ${git_version}
  plugin:    ${plugin}
  casos:     ${elegidos[*]}
  corridas:  ${corridas} por caso y por brazo, con y sin plugin: ${corridas_totales} en total
  techo:     ${techo} USD (--max-cost-usd: al pasarlo no arranca ninguna corrida más)
  estimado:  ${costo_tipico} USD típico, entre ${costo_min} y ${costo_max} (precio de lista de un modelo
             grande; con --modelo más chico baja; ver README)
  sin estas variables (por nombre): ${quitadas[*]:-ninguna}
  tools concedidas: Write, Edit y ${#tools_de_musubi[@]} de Musubi (mcp__plugin_musubi_musubi__*)
  resultados: ${resultados}

Comando:
FIN
printf '  env'
printf ' %q' ${sin_entorno[@]+"${sin_entorno[@]}"}
printf ' \\\n   '
printf ' %q' "${cabeza[@]}"
printf ' \\\n   '
printf ' %q' "${opciones[@]}"
printf ' \\\n   '
printf ' %q' "${concesion[@]}"
printf '\n\n'

if [ "${concurrencia}" -gt 2 ]; then
	printf 'OJO: con -j %s corren %s claude a la vez, cada uno con su daemon de musubi: mirá la RAM.\n\n' \
		"${concurrencia}" "${concurrencia}"
fi
if LC_ALL=C awk -v e="${costo_tipico}" -v t="${techo}" 'BEGIN { exit !(e > t) }'; then
	printf 'OJO: el estimado típico (%s USD) pasa el techo (%s USD): la corrida va a quedar PARCIAL.\n\n' \
		"${costo_tipico}" "${techo}"
fi

if [ "${confirmar}" != 1 ]; then
	echo "No se corrió nada. Para correrlo de verdad (GASTA), agregá --confirmar."
	exit 0
fi

mkdir -p "${base}/resultados"
set +e
env ${sin_entorno[@]+"${sin_entorno[@]}"} "${cabeza[@]}" "${opciones[@]}" "${concesion[@]}"
estado=$?
set -e
case "${estado}" in
0) echo "Terminó. Resultados en ${resultados} (aggregate-result.json y el reporte HTML)." ;;
2) echo "Terminó PARCIAL (techo de gasto o credencial rechazada): mirá partialReason en ${resultados}/aggregate-result.json." ;;
130 | 143) echo "Interrumpido. Lo que haya quedado está en ${resultados}." ;;
*) echo "plugin eval salió con ${estado}: un caso que no cargó, una corrida que no arrancó o una opción inválida (mirá arriba)." ;;
esac
exit "${estado}"
