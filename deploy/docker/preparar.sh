#!/usr/bin/env bash
#
# preparar.sh — deja el host listo para `docker compose up -d` de la cadena de alertas.
#
# Idempotente: re-ejecutarlo NO regenera el token si ya existe (regenerarlo dejaría a Prometheus
# scrapeando con una credencial que el cerebro ya no reconoce, y el síntoma sería «MusubiDown»
# sobre un cerebro perfectamente sano).
#
# Uso:   sudo ./preparar.sh
#
set -euo pipefail

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$AQUI/../.." && pwd)"

# ROOTLESS POR DEFECTO. En un host con SELinux en Enforcing, la variante rootless evita de raíz
# el mapeo de UIDs y los permisos de /etc — y no pide contraseña en ningún paso. Si alguien lo
# corre con sudo, se respeta y va a /etc.
if [ "$(id -u)" -eq 0 ]; then
	DEST="${DEST:-/etc/musubi-prometheus}"
else
	DEST="${DEST:-$HOME/musubi-prometheus}"
fi
SECRETOS="$DEST/secretos"

# ── poner ORIGEN DESTINO [MODO] — instalar sin romperle el archivo a un contenedor que lo tiene
# montado.
#
# DOS CAMINOS, PORQUE SON DOS SITUACIONES Y UNA SOLA RECETA ROMPE UNA DE LAS DOS.
#
#   · EL DESTINO NO EXISTE (primera instalación). No hay contenedor que lo tenga montado todavía,
#     e `install` es lo correcto: crea el archivo con su modo en un solo paso. `cat >` NO sirve acá
#     porque exige que el destino exista — y por eso «cambiá install por cat > y listo» dejaría la
#     primera corrida en una máquina limpia sin configuración.
#
#   · EL DESTINO YA EXISTE (redespliegue). Puede estar BIND-MONTADO en un contenedor CORRIENDO, y
#     un bind-mount de ARCHIVO se pega al INODO, no al nombre. `install`, `mv` y `sed -i` no
#     escriben el archivo: lo desenlazan y crean otro. El contenedor sigue leyendo el anterior —que
#     ya no tiene nombre— y la recarga contesta 200 sobre el archivo equivocado. Y en un host con
#     SELinux el inodo nuevo nace con otra etiqueta, así que ni lo puede leer: la recarga contesta
#     500 sobre un archivo de dueño y modo perfectos. Las dos cosas por la misma razón, porque las
#     dos viven en el inodo. `cat > destino` escribe DENTRO del que ya existe.
#
# La tabla medida de qué operación reemplaza el inodo está en deploy/README.md.
poner() {
	origen="$1"; destino="$2"; modo="${3:-0644}"
	# SE COMPRUEBA ANTES DE TRUNCAR: `> destino` lo vacía en el acto, así que un origen ausente
	# dejaría al contenedor con una configuración VACÍA en vez de la vieja, que es peor.
	[ -s "$origen" ] || { echo "poner: origen ausente o vacío: $origen" >&2; return 1; }
	if [ -f "$destino" ]; then
		chmod u+w "$destino"          # el token vive en 0400: ni su dueño puede escribirlo
		cat "$origen" > "$destino"    # MISMO INODO: no rompe el mount ni la etiqueta
		chmod "$modo" "$destino"
	else
		install -m "$modo" "$origen" "$destino"
	fi
}

echo "→ directorios ($DEST)"
install -d -m 0755 "$DEST" "$DEST/rules"
install -d -m 0700 "$SECRETOS"

echo "→ configuración"
# `poner` y no `install`: los dos son mounts de ARCHIVO (compose.yml). Los de `rules/` y
# `secretos/` son mounts de DIRECTORIO y ahí `install` está bien — un inodo nuevo adentro sí lo ve
# el contenedor, y la etiqueta la repara el `chcon -R` de más abajo en la misma corrida.
poner "$REPO/deploy/prometheus/prometheus.yml"   "$DEST/prometheus.yml"
poner "$REPO/deploy/prometheus/alertmanager.yml" "$DEST/alertmanager.yml"

# EL chat_id SE SUSTITUYE AL INSTALAR, no se versiona.
# Alertmanager no tiene `chat_id_file` —sólo `bot_token_file`— así que sin esto un identificador
# personal quedaría en el repo. Y sin un chat_id válido Alertmanager NO ARRANCA
# (`missing chat_id on telegram_config`), que es lo correcto: un canal a medio configurar que
# arranca y se traga las alertas es peor que uno que se niega y lo grita en el log.
CHAT_FILE="$SECRETOS/telegram_chat_id"
if [ -s "$CHAT_FILE" ]; then
	CHAT_ID="$(tr -dc "0-9-" < "$CHAT_FILE")"
	# NO `sed -i`: reemplaza el inodo igual que `install`, y acá es el peor sitio posible —el
	# archivo está bind-montado en Alertmanager y ESTA línea es la que configura el canal, así que
	# rompe justo lo que se está armando. Se filtra a un temporal y se vuelca DENTRO del inodo.
	TMP_AM="$(mktemp "$DEST/.alertmanager.yml.XXXXXX")"
	sed "s/^\( *chat_id: \)0$/\1$CHAT_ID/" "$DEST/alertmanager.yml" > "$TMP_AM"
	poner "$TMP_AM" "$DEST/alertmanager.yml"
	rm -f "$TMP_AM"
	echo "→ chat_id de Telegram: puesto desde $CHAT_FILE"
else
	echo "→ chat_id de Telegram: FALTA. Alertmanager no va a arrancar hasta que exista."
	echo "  Mandale /start a tu bot y después:"
	echo "    printf %s TU_CHAT_ID > $CHAT_FILE"
fi
install -m 0644 "$REPO/deploy/musubi-alerts.yml"           "$DEST/rules/musubi-alerts.yml"

# LAS REGLAS DE FLOTA SÓLO SE INSTALAN SI EL CEREBRO TIENE FLOTA.
#
# No es prolijidad: `FlotaSinTelemetria` es `absent(musubi_fleet_device_up)`, así que contra un
# cerebro sin el plano de flota DISPARA de inmediato y para siempre. Una alarma falsa desde el
# día uno es lo que enseña a ignorar las alarmas — y entonces la próxima, la de verdad, tampoco
# se mira. Se le pregunta al cerebro en vez de suponer.
# LA ALERTA DEL BACKUP OFF-HOST SÓLO SE INSTALA SI HAY DESTINO OFF-HOST.
#
# `musubi-backup` admite un modo local-only DECLARADO (BACKUP_ALLOW_LOCAL_ONLY=1). Contra esa
# configuración `musubi_backup_offhost_age_seconds` vale -1 para siempre, así que la regla
# dispararía todos los días sin que haya NADA que arreglar. Una alarma que no se apaga arreglando
# algo no es una alarma: es ruido que enseña a ignorar el canal, y se lleva puestas a las demás.
BACKUP_ENV="${BACKUP_ENV:-/etc/musubi/musubi.env}"
# El `|| true` NO es decorativo: con `set -e`, un grep que no encuentra nada sale con 1 y aborta
# el script en la asignación. Y "no encontrar nada" es justamente el caso normal acá.
DESTINO_OFFHOST="$(grep -hoP "^BACKUP_REMOTE=\K.*" "$BACKUP_ENV" 2>/dev/null | tr -d "\"' " | tail -1 || true)"
if [ -n "$DESTINO_OFFHOST" ]; then
	install -m 0644 "$REPO/deploy/musubi-alerts-backup-offhost.yml" "$DEST/rules/musubi-alerts-backup-offhost.yml"
	echo "→ alerta de backup off-host: INSTALADA (destino: $DESTINO_OFFHOST)"
else
	rm -f "$DEST/rules/musubi-alerts-backup-offhost.yml"
	install -m 0644 "$REPO/deploy/musubi-alerts-backup-offhost.yml" "$DEST/musubi-alerts-backup-offhost.yml.cuando-haya-destino"
	echo "→ alerta de backup off-host: NO instalada — no hay BACKUP_REMOTE en $BACKUP_ENV."
	echo "  MODO LOCAL-ONLY: el snapshot queda en el MISMO disco que la base. Protege contra"
	echo "  borrado accidental y corrupción; NO contra perder el host. Es una decisión válida"
	echo "  mientras sea DECLARADA — pero que quede escrito qué cubre y qué no."
fi

BRAIN_URL="${BRAIN_URL:-http://127.0.0.1:7717}"
if [ -s "$DEST/musubi.token" ] &&
   curl -fsS -m 10 -H "Authorization: Bearer $(cat "$DEST/musubi.token")" "$BRAIN_URL/metrics" 2>/dev/null | grep -q "^musubi_fleet_"; then
	install -m 0644 "$REPO/deploy/musubi-alerts-flota.yml" "$DEST/rules/musubi-alerts-flota.yml"
	echo "→ reglas de flota: INSTALADAS (el cerebro expone musubi_fleet_*)"
	HAY_FLOTA=1
else
	rm -f "$DEST/rules/musubi-alerts-flota.yml"
	install -m 0644 "$REPO/deploy/musubi-alerts-flota.yml" "$DEST/musubi-alerts-flota.yml.cuando-haya-flota"
	echo "→ reglas de flota: NO instaladas — el cerebro desplegado no expone musubi_fleet_*."
	echo "  Quedan en $DEST/musubi-alerts-flota.yml.cuando-haya-flota. Volvé a correr este script"
	echo "  cuando despliegues un binario con el plano de flota."
	HAY_FLOTA=0
fi

TOKEN_FILE="$DEST/musubi.token"
if [ -s "$TOKEN_FILE" ]; then
	echo "→ token: ya existe, NO se toca"
	NUEVO=0
else
	echo "→ token: generando uno nuevo para el principal 'prometheus'"
	# 32 bytes de /dev/urandom en base64 url-safe. Sin dependencias: openssl está en cualquier
	# servidor, y si no, el fallback de od+tr.
	if command -v openssl >/dev/null 2>&1; then
		TOKEN="msb_$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n')"
	else
		TOKEN="msb_$(od -An -tx1 -N32 /dev/urandom | tr -d ' \n')"
	fi
	printf '%s' "$TOKEN" > "$TOKEN_FILE"
	NUEVO=1
fi
# Sin salto de línea final: Prometheus manda el archivo TAL CUAL como bearer, y un "\n" al final
# convierte el token en otro que el cerebro no reconoce. Es un fallo silencioso y desconcertante.
# LA CONDICIÓN QUE ESTABA ACÁ ERA UNA TAUTOLOGÍA, medida: verdadera con salto, sin salto y con el
# archivo vacío. No se puede preguntar por un salto final con `$( )`, porque la sustitución se los
# come — así que las dos mitades daban verdadero por caminos distintos y el `if` no protegía nada.
#
# Consecuencia: reescribía el token en CADA corrida, y lo hacía con `mv`, que le cambia el inodo a
# un archivo bind-montado como ARCHIVO (compose.yml). O sea que cada `preparar.sh` dejaba a
# Prometheus leyendo un token sin nombre, con un 401 y un archivo de permisos perfectos. Y en la
# corrida donde el token se acababa de crear, el comentario de arriba promete no tocarlo.
#
# Ahora se pregunta comparando el último byte con y sin saltos, y se escribe DENTRO del inodo.
if [ "$(tail -c1 "$TOKEN_FILE" | wc -c)" -eq 1 ] && [ "$(tail -c1 "$TOKEN_FILE" | tr -d '\n' | wc -c)" -eq 0 ]; then
	TOK="$(cat "$TOKEN_FILE")"          # `$( )` se come el salto: es justo lo que hay que sacar
	chmod u+w "$TOKEN_FILE"
	printf '%s' "$TOK" > "$TOKEN_FILE"  # MISMO INODO, sin `mv`
fi
chmod 0400 "$TOKEN_FILE"

SHA="$(sha256sum "$TOKEN_FILE" >/dev/null 2>&1 && printf '%s' "$(cat "$TOKEN_FILE")" | sha256sum | cut -d' ' -f1)"

# ETIQUETA DE SELINUX SOBRE TODO LO QUE ACABAMOS DE ESCRIBIR.
#
# `:z` en el compose reetiqueta los volúmenes AL ARRANCAR EL CONTENEDOR. Cualquier archivo que
# se escriba DESPUÉS —o sea, cada vez que se re-corre este script con los contenedores ya
# arriba— nace con el contexto del home (`user_home_t`) y el contenedor deja de poder leerlo.
#
# Y falla en silencio: Prometheus informa el reload como EXITOSO y se queda con CERO reglas
# cargadas. Un sistema de alertas que se apaga sin decirlo es peor que uno que nunca se montó,
# porque el silencio se lee como calma. Por eso también está la verificación del final.
if command -v chcon >/dev/null 2>&1 && [ "$(getenforce 2>/dev/null || echo Disabled)" != "Disabled" ]; then
	chcon -R -t container_file_t "$DEST" 2>/dev/null && echo "→ SELinux: etiquetado container_file_t" \
		|| echo "→ SELinux: no se pudo etiquetar $DEST (revisá si el contenedor puede leer su config)"
fi

# El compose lee MUSUBI_PROM_DIR de este .env. Es lo que permite que UN solo compose sirva para
# la variante rootless y la de root, en vez de dos archivos casi iguales que se separan en la
# primera corrección que alguien aplica a uno solo.
printf 'MUSUBI_PROM_DIR=%s\n' "$DEST" > "$AQUI/.env"
echo "→ .env: MUSUBI_PROM_DIR=$DEST"

echo
echo "═══════════════════════════════════════════════════════════════════════════════════"
if [ "$NUEVO" -eq 1 ]; then
cat <<TXT
FALTA UN PASO, Y SIN ÉL PROMETHEUS NO VE NADA DE LA FLOTA.

Agregá este principal al principals.yaml del CEREBRO y esperá la recarga en caliente (≤10 s):

  principals:
    - name: prometheus
      token_sha256: "$SHA"
      role: reader
      read: all
      fleet:
        metrics: ["*"]

NADA de \`exec\` ni de \`screen\`. Un scraper sólo necesita MIRAR, y esta credencial vive en un
archivo de configuración de otro servicio: es exactamente la que no querés que pueda ejecutar.

Y ojo con el techo que documenta prometheus.yml: las capacidades de flota NO se derivan del rol.
Un token admin SIN concesiones explícitas no ve ni una máquina — verías las métricas del servidor
y ninguna de la flota, y las alertas de flota quedarían inertes sin decirlo.
TXT
else
	echo "El token ya estaba. Si Prometheus da 401, es que el principal 'prometheus' no está"
	echo "en el principals.yaml del cerebro, o su token_sha256 no coincide con este:"
	echo "  $SHA"
fi
echo "═══════════════════════════════════════════════════════════════════════════════════"
echo
echo "Después:  cd $AQUI && podman compose up -d   (o docker compose up -d)"

# VERIFICACIÓN FINAL: qué cargó Prometheus DE VERDAD.
#
# No alcanza con que el reload diga OK. Un archivo de reglas ilegible da reload exitoso y cero
# reglas — el fallo que este bloque existe para atrapar. Se pregunta por el resultado, no por el
# intento; es la misma disciplina que el resto del proyecto.
PROM_URL="${PROM_URL:-http://127.0.0.1:9099}"
if curl -fsS -m 5 "$PROM_URL/-/ready" >/dev/null 2>&1; then
	curl -fsS -m 5 -XPOST "$PROM_URL/-/reload" >/dev/null 2>&1 || true
	sleep 2
	# SE COMPARA CONTRA LO QUE ACABAMOS DE INSTALAR, no contra cero.
	#
	# La versión anterior preguntaba "¿hay alguna regla?" y daba por bueno un 9 que eran las del
	# cerebro — mientras las 12 de flota estaban en disco y sin cargar, porque `rule_files`
	# apuntaba a UN archivo en vez de a un glob. Una verificación que pasa por el motivo
	# equivocado es peor que ninguna: deja el problema puesto y con sello de aprobado.
	ESPERADAS=0
	for f in "$DEST"/rules/*.yml; do
		[ -e "$f" ] || continue
		ESPERADAS=$((ESPERADAS + $(grep -c "^\s*- alert:" "$f" || true)))
	done
	N="$(curl -fsS -m 5 "$PROM_URL/api/v1/rules" 2>/dev/null | grep -o "\"type\":\"alerting\"" | wc -l || echo 0)"
	if [ "$N" -lt "$ESPERADAS" ]; then
		echo
		echo "⚠  INSTALÉ $ESPERADAS ALERTAS Y PROMETHEUS TIENE $N."
		echo "   Casi siempre es una de dos: la etiqueta de SELinux de los archivos recién"
		echo "   escritos, o que rule_files apunte a un archivo fijo en vez de a un glob."
		echo "   Comprobalo:  podman exec musubi-prometheus ls -la /etc/prometheus/rules/"
		echo "     cd $AQUI && podman compose restart prometheus"
	elif [ "$N" -eq 0 ]; then
		echo
		echo "⚠  PROMETHEUS ESTÁ CORRIENDO Y NO CARGÓ NINGUNA REGLA."
		echo "   Casi siempre es la etiqueta de SELinux de los archivos recién escritos."
		echo "   Comprobalo:  podman exec musubi-prometheus cat /etc/prometheus/rules/musubi-alerts.yml"
		echo "   Si da 'Permission denied', reiniciá el contenedor para que :z reetiquete:"
		echo "     cd $AQUI && podman compose restart prometheus"
	else
		echo "→ Prometheus evalúa las $N alertas instaladas."
	fi
fi

# ── QUE LA CADENA VUELVA DE UN REBOOT, Y QUE ESO VIVA EN UN ARCHIVO ─────────────────────────
#
# Esto YA estaba puesto a mano en el servidor y funcionaba —medido: en el reboot del 2026-08-31
# Alertmanager volvió 55 segundos después—, pero no vivía en ningún archivo del repo. Un servidor
# reconstruido lo perdía en silencio, y el síntoma no aparece hasta el primer corte de luz.
#
# NO hace falta tocar `restart: unless-stopped` en el compose: `podman-restart.service` filtra por
# `should-start-on-boot=true`, no por `restart-policy=always`, y ese filtro YA incluye
# `unless-stopped`. Cambiarlo a `always` sería peor: levanta hasta lo que alguien paró a propósito.
#
# Todo idempotente y NINGUNO puede abortar el script: se corre con `set -euo pipefail` y esto va
# al final, después de haber instalado la configuración. Un fallo acá se avisa y no tira abajo un
# despliegue que ya salió bien.
echo "→ arranque tras reboot"
if loginctl show-user "$(id -un)" --property=Linger 2>/dev/null | grep -q 'Linger=yes'; then
	echo "   linger: ya estaba activo"
elif loginctl enable-linger "$(id -un)" 2>/dev/null; then
	echo "   linger: activado"
else
	echo "   ⚠ linger: NO se pudo activar (¿sin sesión de usuario?). Corré: loginctl enable-linger $(id -un)"
fi
# `systemctl --user` necesita el bus de la sesión; por ssh o con sudo -u puede no existir.
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
if systemctl --user is-enabled podman-restart.service >/dev/null 2>&1; then
	echo "   podman-restart: ya estaba habilitado"
elif systemctl --user enable podman-restart.service >/dev/null 2>&1; then
	echo "   podman-restart: habilitado"
else
	echo "   ⚠ podman-restart: NO se pudo habilitar (¿sin XDG_RUNTIME_DIR?). Corré, logueado como este usuario:"
	echo "     systemctl --user enable podman-restart.service"
fi

# Y ALERTMANAGER TAMBIÉN, porque conservar el inodo no alcanza.
#
# El `chat_id` ya está adentro del inodo que Alertmanager tiene montado —eso lo arregló `poner`—
# pero Alertmanager relee su configuración sólo cuando se lo piden. Este guion recargaba únicamente
# a Prometheus, así que el canal seguía con la configuración vieja y nadie decía nada: el archivo
# en disco perfecto, el contenedor con lo de antes, y el guion terminando en verde.
#
# No aborta si no contesta: `preparar.sh` corre también antes de que los contenedores existan, y
# fracasar ahí convertiría la primera instalación en un error. Se dice qué hacer, que es lo que
# falta cuando alguien vuelve mañana a preguntarse por qué el chat_id no tomó.
ALERT_URL="${ALERT_URL:-http://127.0.0.1:9093}"
if curl -fsS -m 5 "$ALERT_URL/-/ready" >/dev/null 2>&1; then
	if curl -fsS -m 5 -XPOST "$ALERT_URL/-/reload" >/dev/null 2>&1; then
		echo "→ Alertmanager: releyó su configuración"
	else
		echo "⚠ Alertmanager está arriba y no aceptó el reload. El chat_id está en el archivo y NO"
		echo "  en memoria: reiniciá el contenedor (podman restart musubi-alertmanager) o mandale"
		echo "  un HUP para que lo tome."
	fi
fi
