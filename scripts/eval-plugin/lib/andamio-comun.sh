# shellcheck shell=bash
# lib/andamio-comun.sh — lo que comparten los andamios (scaffold_script) de la suite.
#
# NO SE CORRE SOLO. correr.sh arma cada `evals/<caso>/andamio.sh` pegando tres cosas: una cabecera
# con ANDAMIO_BIN (el binario de musubi) y CASO, este archivo, y el cuerpo del caso. El arnés de
# `claude plugin eval` corre el resultado en el workspace VACÍO de cada corrida, como vos y fuera
# del sandbox, con un entorno chico (PATH, HOME = el home temporal de la corrida, TMPDIR,
# TERM=dumb) y 120 s de límite. Un código de salida distinto de cero deja la corrida en 0 con
# «scaffold failed»: por eso cada verificación de acá FALLA en vez de avisar. Medir un estado que no
# es el que creemos es peor que no medir.
#
# Qué garantiza, y por qué cada cosa:
#   - la memoria sembrada vive en `.musubi/` DE ESTA CARPETA y es la que va a abrir el plugin:
#     ningún ancestro puede tener `.git` ni `.musubi/config.yaml`, porque Musubi resolvería ESE;
#   - la memoria no sincroniza con ningún cerebro (sync apagado y sin URL) ni su daemon consulta
#     GitHub por versiones nuevas, y ningún musubi que corre el andamio sale a la red por HTTP;
#   - el dato que la pregunta necesita no queda en ningún archivo de TEXTO del workspace, o el
#     brazo sin plugin lo encontraría con Grep y el caso no mediría la memoria;
#   - el estado de la memoria es el que el caso dice (proyecto conocido o primer contacto), porque
#     el arranque de Musubi inyecta cosas muy distintas en uno y en otro.
set -euo pipefail

: "${ANDAMIO_BIN:?este andamio lo arma correr.sh: falta ANDAMIO_BIN}"
: "${CASO:?este andamio lo arma correr.sh: falta CASO}"

# Lo único que decide a qué memoria se habla es la carpeta de la corrida. Se quita por NOMBRE todo
# lo que podría apuntar a otra memoria, a un cerebro o a una credencial (el entorno del andamio ya
# viene casi vacío: esto es por las dudas). El patrón es *MUSUBI* y no MUSUBI_*, porque hay nombres
# como ALTURA_MUSUBI_TOKEN_FILE; por eso mismo la cabecera nombra al binario ANDAMIO_BIN.
for _variable in $(compgen -e); do
	case "${_variable}" in
	*MUSUBI* | CLAUDE_PROJECT_DIR | CLAUDE_PLUGIN_ROOT | CLAUDE_PLUGIN_DATA) unset "${_variable}" ;;
	esac
done
unset _variable

andamio_falla() {
	printf 'andamio %s: %s\n' "${CASO}" "$*" >&2
	exit 1
}

# Lo auxiliar va FUERA del workspace: ahí pasan las respuestas de Musubi, que traen el dato sembrado.
ANDAMIO_TMP="$(mktemp -d)"
trap 'rm -rf "${ANDAMIO_TMP}"' EXIT
case "${ANDAMIO_TMP}" in
"${PWD}"/*) andamio_falla "TMPDIR cae dentro del workspace (${ANDAMIO_TMP}): el agente vería lo que el andamio deja ahí" ;;
esac

if command -v timeout >/dev/null 2>&1; then
	ANDAMIO_RELOJ=(timeout 60)
else
	ANDAMIO_RELOJ=()
fi

# El andamio no sale a la red. El daemon que ACTIVA una memoria nueva consulta la API pública de
# GitHub para avisar si hay una versión nueva de musubi (update.check_interval_hours; medido: el
# CONNECT a api.github.com:443 sale aunque el daemon viva 0,2 s). Todo musubi que corre el andamio va
# con un proxy muerto en 127.0.0.1:9, así que lo que intente salir por HTTP se corta en la máquina.
ANDAMIO_SIN_RED=(env HTTPS_PROXY=http://127.0.0.1:9 https_proxy=http://127.0.0.1:9
	HTTP_PROXY=http://127.0.0.1:9 http_proxy=http://127.0.0.1:9 NO_PROXY= no_proxy=)

# El workspace de la corrida es la raíz de la memoria. La ruta viaja dentro de JSON armado a mano,
# así que no puede traer comillas ni barras invertidas.
case "${PWD}" in
*'"'* | *'\'*) andamio_falla "la carpeta de la corrida tiene comillas o barras invertidas: ${PWD}" ;;
esac
if [ -e .musubi ] || [ -e .git ]; then
	andamio_falla "el workspace no llegó vacío: ya hay .musubi o .git en ${PWD}"
fi

# Musubi toma la memoria del ancestro más cercano con `.musubi/config.yaml`, o la raíz de git. Si
# arriba del workspace hay cualquiera de los dos, el plugin abriría ESA memoria y no la sembrada.
andamio_verificar_ancestros() {
	local d
	d="$(dirname "${PWD}")"
	while :; do
		if [ -e "${d}/.git" ]; then
			andamio_falla "hay un .git en ${d}, arriba del workspace: Musubi tomaría ese repo como raíz"
		fi
		if [ -e "${d}/.musubi/config.yaml" ]; then
			andamio_falla "hay una memoria de Musubi en ${d}, arriba del workspace: el plugin abriría esa"
		fi
		if [ "${d}" = "/" ]; then
			break
		fi
		d="$(dirname "${d}")"
	done
}
andamio_verificar_ancestros

# git con identidad fija: el HOME de la corrida no tiene configuración.
andamio_git() {
	git -c user.name=eval -c user.email=eval@ejemplo.test -c init.defaultBranch=main \
		-c commit.gpgsign=false "$@"
}

andamio_commit() {
	andamio_git add -A
	andamio_git commit -q -m "$1"
}

# andamio_rpc MODO — manda a `musubi daemon` las llamadas que llegan por stdin (una por línea, con la
# forma {"name":...,"arguments":{...}}) y deja las respuestas en ${ANDAMIO_TMP}/salida.jsonl.
#   MODO=siembra: el daemon trabaja sobre la memoria de esta carpeta (MUSUBI_HOME), y la crea si falta.
#   MODO=plugin:  el daemon arranca como lo arranca el plugin en la corrida: sin MUSUBI_HOME, parado
#                 en la carpeta. Sirve para comprobar que el brazo con plugin va a ver lo sembrado.
# Falla si alguna llamada vuelve con error o si falta alguna respuesta.
andamio_rpc() {
	local modo="$1" entrada="${ANDAMIO_TMP}/entrada.jsonl" salida="${ANDAMIO_TMP}/salida.jsonl"
	local linea n=0 i
	printf '%s\n' \
		'{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"andamio-eval","version":"1"}}}' \
		'{"jsonrpc":"2.0","method":"notifications/initialized"}' >"${entrada}"
	while IFS= read -r linea || [ -n "${linea}" ]; do
		if [ -z "${linea}" ]; then
			continue
		fi
		n=$((n + 1))
		printf '{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":%s}\n' "$((n + 1))" "${linea}" >>"${entrada}"
	done
	case "${modo}" in
	siembra)
		MUSUBI_HOME="${PWD}" "${ANDAMIO_SIN_RED[@]}" ${ANDAMIO_RELOJ[@]+"${ANDAMIO_RELOJ[@]}"} "${ANDAMIO_BIN}" daemon \
			<"${entrada}" >"${salida}" 2>"${ANDAMIO_TMP}/daemon.err" ||
			andamio_falla "musubi daemon (siembra) salió con error: $(tail -n 3 "${ANDAMIO_TMP}/daemon.err")"
		;;
	plugin)
		"${ANDAMIO_SIN_RED[@]}" ${ANDAMIO_RELOJ[@]+"${ANDAMIO_RELOJ[@]}"} "${ANDAMIO_BIN}" daemon \
			<"${entrada}" >"${salida}" 2>"${ANDAMIO_TMP}/daemon.err" ||
			andamio_falla "musubi daemon (como el plugin) salió con error: $(tail -n 3 "${ANDAMIO_TMP}/daemon.err")"
		;;
	*) andamio_falla "andamio_rpc: modo desconocido ${modo}" ;;
	esac
	if grep -q '"isError":true' "${salida}" || grep -q '"error":{' "${salida}"; then
		andamio_falla "una llamada a Musubi volvió con error: $(grep -m 1 -e '"isError":true' -e '"error":{' "${salida}" | cut -c 1-300)"
	fi
	i=2
	while [ "${i}" -le "$((n + 1))" ]; do
		grep -q "\"id\":${i}," "${salida}" || andamio_falla "Musubi no contestó la llamada ${i} de ${n}"
		i=$((i + 1))
	done
}

# Crea la memoria de esta carpeta como la crea el plugin y le fija el proyecto: si el arnés llegara
# a mover el workspace, el proyecto derivado del nombre de la carpeta cambiaría y las notas
# sembradas pasarían a ser «de otro proyecto».
andamio_activar_memoria() {
	andamio_rpc siembra </dev/null
	if [ ! -f .musubi/config.yaml ]; then
		andamio_falla "musubi no creó .musubi/config.yaml en ${PWD}"
	fi
	if ! grep -q '^project_id:' .musubi/config.yaml; then
		printf '\nproject_id: eval-%s\n' "${CASO}" >>.musubi/config.yaml
	fi
	andamio_apagar_chequeo_de_version
}

# El daemon del plugin, en la corrida, corre FUERA del proxy muerto: con el chequeo de versión
# apagado en la config no consulta GitHub (hoy tampoco lo haría, porque el daemon que activó la
# memoria ya dejó marcada la consulta, pero esa marca nadie la verifica). El aviso de versión nueva
# va a stderr, al log del servidor MCP y no al agente: apagarlo no cambia lo que se mide.
andamio_apagar_chequeo_de_version() {
	awk '/^update:/ { s = 1; print; next }
		s && /^[^[:space:]]/ { s = 0 }
		s && /^[[:space:]]+check_interval_hours:/ { sub(/check_interval_hours:.*/, "check_interval_hours: -1") }
		{ print }' .musubi/config.yaml >"${ANDAMIO_TMP}/config.yaml" ||
		andamio_falla "no pude reescribir .musubi/config.yaml"
	cat "${ANDAMIO_TMP}/config.yaml" >.musubi/config.yaml
}

# Sin cerebro ni GitHub: la memoria sembrada no sale de esta carpeta y el daemon no sale a buscar
# versiones. Se lee del config que va a usar el daemon.
andamio_verificar_que_no_sale() {
	local bloque
	bloque="$(awk '/^sync:/{s=1; next} s && /^[^[:space:]]/{exit} s' .musubi/config.yaml)"
	grep -Eq '^[[:space:]]+enabled:[[:space:]]*false[[:space:]]*$' <<<"${bloque}" ||
		andamio_falla "sync.enabled no es false en .musubi/config.yaml"
	grep -Eq "^[[:space:]]+central_url:[[:space:]]*(\"\"|'')?[[:space:]]*\$" <<<"${bloque}" ||
		andamio_falla "sync.central_url no está vacío en .musubi/config.yaml"
	bloque="$(awk '/^update:/{s=1; next} s && /^[^[:space:]]/{exit} s' .musubi/config.yaml)"
	grep -Eq '^[[:space:]]+check_interval_hours:[[:space:]]*-[0-9.]+[[:space:]]*$' <<<"${bloque}" ||
		andamio_falla "update.check_interval_hours no es negativo en .musubi/config.yaml: el daemon del plugin consultaría GitHub"
}

# Ningún archivo de TEXTO del workspace (incluidos .git y .musubi) puede traer el dato: el brazo
# sin plugin lo encontraría con Grep. La base SQLite es binaria (Read la rechaza por extensión y
# ripgrep la saltea al recorrer); lo que queda en un -wal no, así que tampoco puede quedar uno.
andamio_verificar_sin_fuga() {
	local texto
	for texto in "$@"; do
		if grep -rIlF -- "${texto}" . >"${ANDAMIO_TMP}/fuga.txt" 2>/dev/null; then
			andamio_falla "«${texto}» quedó en archivos de texto del workspace: $(head -n 3 "${ANDAMIO_TMP}/fuga.txt" | tr '\n' ' ')"
		fi
	done
	if [ -s .musubi/memory.db-wal ]; then
		andamio_falla "quedó un .musubi/memory.db-wal con contenido: el dato sembrado podría leerse desde ahí"
	fi
}

# Entrada de hook como la que manda Claude Code.
andamio_entrada_de_arranque() {
	printf '{"session_id":"andamio-%s","transcript_path":"%s/vacio.jsonl","cwd":"%s","hook_event_name":"SessionStart","source":"startup"}' \
		"$1" "${ANDAMIO_TMP}" "${PWD}"
}

# andamio_arranque RAIZ N — corre el hook de arranque de Musubi sobre la memoria de RAIZ y deja lo
# que inyectaría en ${ANDAMIO_TMP}/arranque.txt.
andamio_arranque() {
	: >"${ANDAMIO_TMP}/vacio.jsonl"
	andamio_entrada_de_arranque "$2" |
		MUSUBI_HOME="$1" "${ANDAMIO_SIN_RED[@]}" ${ANDAMIO_RELOJ[@]+"${ANDAMIO_RELOJ[@]}"} "${ANDAMIO_BIN}" detect --hook-mode \
			>"${ANDAMIO_TMP}/arranque.txt" 2>"${ANDAMIO_TMP}/arranque.err" ||
		andamio_falla "musubi detect salió con error: $(tail -n 3 "${ANDAMIO_TMP}/arranque.err")"
}

# Las marcas del arranque de un proyecto que Musubi todavía no conoce: el bloque de autoconocimiento
# (no hay perfil) y la oferta de descubrir skills (primera vez). Van nombres de tools y de skills,
# que cambian menos que los títulos. control-primer-contacto comprueba que estas marcas SIGUEN
# detectando ese arranque: si un día dejan de hacerlo, ese caso falla en vez de medir otra cosa.
ANDAMIO_MARCAS_DE_PRIMER_CONTACTO='autoconocimiento|auto-descubrimiento|musubi_search_skills|analyze-project|project-profile'

# Proyecto conocido: el perfil ya está sembrado (lo siembra el caso con topic_key project/profile) y
# la oferta de descubrir skills ya se hizo una vez, como pasa desde la segunda sesión de un proyecto.
# Se consume con el hook de verdad y se comprueba con un segundo arranque, que tiene que traer la
# memoria y ninguna de las marcas de primer contacto.
andamio_estado_proyecto_conocido() {
	andamio_arranque "${PWD}" 1
	andamio_arranque "${PWD}" 2
	if grep -Eq "${ANDAMIO_MARCAS_DE_PRIMER_CONTACTO}" "${ANDAMIO_TMP}/arranque.txt"; then
		andamio_falla "el arranque todavía trae lo de un proyecto nuevo ($(grep -Eo "${ANDAMIO_MARCAS_DE_PRIMER_CONTACTO}" "${ANDAMIO_TMP}/arranque.txt" | sort -u | tr '\n' ' ')): ¿falta sembrar project/profile?"
	fi
	grep -q 'additionalContext' "${ANDAMIO_TMP}/arranque.txt" ||
		andamio_falla "el arranque de Musubi no inyecta nada: la memoria sembrada no llegaría al agente"
}

# Primer contacto: la memoria existe pero no hay perfil ni se ofreció nada todavía. Se comprueba
# sobre una COPIA, porque correr el hook acá consumiría la oferta que el caso quiere medir.
andamio_verificar_primer_contacto() {
	mkdir -p "${ANDAMIO_TMP}/copia"
	cp -R .musubi "${ANDAMIO_TMP}/copia/"
	andamio_arranque "${ANDAMIO_TMP}/copia" 1
	grep -q 'musubi_search_skills' "${ANDAMIO_TMP}/arranque.txt" ||
		andamio_falla "el arranque de esta memoria no ofrece descubrir skills (musubi_search_skills): o algo ya consumió la oferta, o las marcas de ANDAMIO_MARCAS_DE_PRIMER_CONTACTO quedaron viejas"
	grep -q 'analyze-project' "${ANDAMIO_TMP}/arranque.txt" ||
		andamio_falla "el arranque de esta memoria no pide el autoconocimiento (analyze-project): o ya tiene perfil, o las marcas de ANDAMIO_MARCAS_DE_PRIMER_CONTACTO quedaron viejas"
	rm -rf "${ANDAMIO_TMP}/copia"
}

# Deja la base asentada: todo en memory.db y nada en el -wal. El daemon que ACTIVA una memoria
# nueva sale sin volcar el -wal (medido: 7 de 8 veces queda memory.db con 4 KB y todo lo demás en el
# -wal), y el -wal, a diferencia de memory.db, el agente sí lo puede leer. `musubi tareas` abre la
# base, sólo lee y la cierra bien, que es lo que la vuelca; no toca el estado de primer contacto que
# mide control-primer-contacto (eso lo consume el hook de arranque, no abrir la base).
andamio_asentar_memoria() {
	MUSUBI_HOME="${PWD}" "${ANDAMIO_SIN_RED[@]}" ${ANDAMIO_RELOJ[@]+"${ANDAMIO_RELOJ[@]}"} "${ANDAMIO_BIN}" tareas --json \
		>/dev/null 2>"${ANDAMIO_TMP}/tareas.err" ||
		andamio_falla "musubi tareas no pudo abrir y cerrar la memoria: $(tail -n 3 "${ANDAMIO_TMP}/tareas.err")"
}

# El cierre de todo andamio: lo que tiene que valer justo antes de que arranque el agente.
andamio_cerrar() {
	andamio_verificar_que_no_sale
	andamio_asentar_memoria
	andamio_verificar_sin_fuga "$@"
}
