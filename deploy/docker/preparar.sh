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

DEST="${DEST:-/etc/musubi-prometheus}"
SECRETOS="${SECRETOS:-/etc/musubi}"
AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$AQUI/../.." && pwd)"
NOBODY=65534

[ "$(id -u)" -eq 0 ] || { echo "preparar.sh: corré con sudo (escribe en $DEST y $SECRETOS)" >&2; exit 1; }

echo "→ directorios"
install -d -m 0755 "$DEST" "$DEST/rules" "$SECRETOS"

echo "→ configuración"
install -m 0644 "$REPO/deploy/prometheus/prometheus.yml"   "$DEST/prometheus.yml"
install -m 0644 "$REPO/deploy/prometheus/alertmanager.yml" "$DEST/alertmanager.yml"
install -m 0644 "$REPO/deploy/musubi-alerts.yml"           "$DEST/rules/musubi-alerts.yml"

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
chown "$NOBODY:$NOBODY" "$TOKEN_FILE"
chmod 0400 "$TOKEN_FILE"

SHA="$(sha256sum "$TOKEN_FILE" >/dev/null 2>&1 && printf '%s' "$(cat "$TOKEN_FILE")" | sha256sum | cut -d' ' -f1)"

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
echo "Después:  cd $AQUI && docker compose up -d"
