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

echo "→ directorios ($DEST)"
install -d -m 0755 "$DEST" "$DEST/rules"
install -d -m 0700 "$SECRETOS"

echo "→ configuración"
install -m 0644 "$REPO/deploy/prometheus/prometheus.yml"   "$DEST/prometheus.yml"
install -m 0644 "$REPO/deploy/prometheus/alertmanager.yml" "$DEST/alertmanager.yml"

# EL chat_id SE SUSTITUYE AL INSTALAR, no se versiona.
# Alertmanager no tiene `chat_id_file` —sólo `bot_token_file`— así que sin esto un identificador
# personal quedaría en el repo. Y sin un chat_id válido Alertmanager NO ARRANCA
# (`missing chat_id on telegram_config`), que es lo correcto: un canal a medio configurar que
# arranca y se traga las alertas es peor que uno que se niega y lo grita en el log.
CHAT_FILE="$SECRETOS/telegram_chat_id"
if [ -s "$CHAT_FILE" ]; then
	CHAT_ID="$(tr -dc "0-9-" < "$CHAT_FILE")"
	sed -i "s/^\( *chat_id: \)0$/\1$CHAT_ID/" "$DEST/alertmanager.yml"
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
if [ -n "$(tail -c1 "$TOKEN_FILE" | tr -d '\0')" ] || [ -z "$(tail -c1 "$TOKEN_FILE")" ]; then
	printf '%s' "$(cat "$TOKEN_FILE")" > "$TOKEN_FILE.tmp" && mv "$TOKEN_FILE.tmp" "$TOKEN_FILE"
fi
chmod 0400 "$TOKEN_FILE"

SHA="$(sha256sum "$TOKEN_FILE" >/dev/null 2>&1 && printf '%s' "$(cat "$TOKEN_FILE")" | sha256sum | cut -d' ' -f1)"

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
