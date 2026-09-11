#!/usr/bin/env bash
# renovar-tls.sh — vuelve a pedirle a Tailscale el certificado del cerebro.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# QUÉ RESUELVE, Y POR QUÉ NO ALCANZABA CON ANOTARLO EN UN CALENDARIO
#
# `tailscale cert` emite un certificado de Let's Encrypt válido 90 DÍAS. Sin renovación
# automática, el cerebro se apaga solo a los tres meses: arranca fail-closed y no sirve. Eso
# estaba escrito en `deploy/README.md` y en `musubi-comparar.service` como «la decisión abierta que
# bloquea la migración a TLS», con la renovación delegada a «la fecha va en el calendario de quien
# opera». Un recordatorio en un calendario no es un mecanismo: no falla ruidosamente, falla
# calladamente, y el día 91 el síntoma es «el cerebro no levanta» sin nada que nombre la causa.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# ESTO SOLO NO ALCANZA, Y CONVIENE SABER POR QUÉ ANTES DE CONFIAR EN ÉL
#
# Renovar el par EN DISCO no cambia lo que el cerebro está sirviendo: `ListenAndServeTLS` lee los
# archivos UNA SOLA VEZ, al arrancar. La otra mitad vive en el binario —`internal/mcp/tls_recarga.go`,
# un `GetCertificate` que relee cuando el par cambia— y sin ella este guion sería teatro: dejaría
# el certificado nuevo quieto en el disco mientras los clientes siguen recibiendo el viejo.
#
# Peor todavía: un `openssl x509` sobre el ARCHIVO contestaría «faltan 89 días» con el proceso
# sirviendo uno vencido. Por eso la serie que vigila el vencimiento —`musubi_tls_certificate_expiry_seconds`—
# la emite el CEREBRO desde el certificado que tiene cargado, y no este guion desde el archivo.
# Quien renueva no es quien mide, a propósito.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# INSTALACIÓN (en el servidor del cerebro, como el usuario del cerebro)
#
#   mkdir -p ~/.config/systemd/user
#   cp deploy/renovar-tls.sh ~/renovar-tls.sh && chmod 755 ~/renovar-tls.sh
#   sed "s|@GUION@|$HOME/renovar-tls.sh|" deploy/systemd/musubi-tls-renovar.service \
#       > ~/.config/systemd/user/musubi-tls-renovar.service
#   cp deploy/systemd/musubi-tls-renovar.timer ~/.config/systemd/user/
#   systemctl --user daemon-reload
#   systemctl --user enable --now musubi-tls-renovar.timer
#   loginctl enable-linger "$(id -un)"     # si no, se muere al cerrar sesión
#
# CÓDIGOS DE SALIDA, y los tres significan cosas distintas:
#   0  el certificado está en su lugar y vigente
#   1  se pudo preguntar y la renovación FALLÓ — el certificado viejo sigue ahí hasta que venza
#   2  no se pudo ni intentar (falta `tailscale`, falta el nombre del nodo, no hay dónde escribir)
# ════════════════════════════════════════════════════════════════════════════════════════════
set -uo pipefail

# El nombre del nodo NO se adivina: es lo que el certificado va a decir, y un nombre equivocado
# produce un certificado que ningún cliente valida. Se puede pasar por entorno o por argumento.
NODO="${MUSUBI_TLS_NODO:-${1:-}}"
DESTINO="${MUSUBI_TLS_DIR:-$HOME/musubi-brain/.musubi}"
CERT="$DESTINO/tls.crt"
KEY="$DESTINO/tls.key"

fatal() { printf '\033[31m✘ %s\033[0m\n' "$1" >&2; exit "${2:-2}"; }

if [ -z "$NODO" ]; then
  fatal "no sé para qué nombre pedir el certificado. Pasalo como argumento o en MUSUBI_TLS_NODO (ej. cerebro.tailXXXX.ts.net). Adivinarlo produciría un certificado que ningún cliente valida, que es peor que no tenerlo."
fi
command -v tailscale >/dev/null 2>&1 || fatal "no está \`tailscale\` en el PATH: este guion corre en el servidor del cerebro, no en la máquina del operador"
mkdir -p "$DESTINO" || fatal "no se pudo crear $DESTINO"

# EL VENCIMIENTO DE ANTES, para poder decir si la renovación movió algo. Si no hay certificado
# todavía, queda vacío — que es «no había», y es distinto de «no se pudo leer».
ANTES=""
[ -r "$CERT" ] && ANTES="$(openssl x509 -enddate -noout -in "$CERT" 2>/dev/null | cut -d= -f2)"

# ── LA RENOVACIÓN ───────────────────────────────────────────────────────────────────────────
#
# SE ESCRIBE A UN TEMPORAL Y RECIÉN DESPUÉS SE MUEVE, y no es prolijidad: `tailscale cert`
# escribiendo directo sobre los destinos deja una ventana en que el `.crt` ya es el nuevo y el
# `.key` todavía es el viejo. El cerebro tolera esa ventana a propósito —sigue sirviendo el par
# anterior y reintenta—, pero no hay razón para producirla si se puede evitar: dos `mv` sobre el
# mismo filesystem son atómicos cada uno, y así la ventana dura lo que tarda el segundo `mv`.
TMP="$(mktemp -d)" || fatal "no se pudo crear un directorio temporal"
trap 'rm -rf "$TMP"' EXIT

if ! SALIDA="$(tailscale cert --cert-file "$TMP/tls.crt" --key-file "$TMP/tls.key" "$NODO" 2>&1)"; then
  printf '\033[31m✘ `tailscale cert` falló para %s\033[0m\n' "$NODO" >&2
  printf '  %s\n' "$SALIDA" >&2
  printf '  El certificado que ya estaba SIGUE en su lugar y vigente hasta %s.\n' "${ANTES:-(no había)}" >&2
  printf '  La serie musubi_tls_certificate_expiry_seconds sigue contando ese vencimiento, así que\n' >&2
  printf '  esto se va a ver venir en vez de sorprender.\n' >&2
  exit 1
fi

chmod 600 "$TMP/tls.key" || fatal "no se pudo restringir la clave nueva" 1
mv -f "$TMP/tls.crt" "$CERT" || fatal "no se pudo instalar el certificado nuevo" 1
mv -f "$TMP/tls.key" "$KEY"  || fatal "no se pudo instalar la clave nueva (el certificado ya se movió: el cerebro va a seguir con el par viejo y reintentar)" 1

DESPUES="$(openssl x509 -enddate -noout -in "$CERT" 2>/dev/null | cut -d= -f2)"
if [ -z "$DESPUES" ]; then
  # El archivo está pero no se puede leer su fecha. NO es «salió bien»: puede ser un PEM truncado,
  # y el cerebro lo va a rechazar al releerlo.
  fatal "el certificado quedó escrito en $CERT pero no se le pudo leer la fecha de vencimiento: puede estar truncado, y el cerebro lo va a rechazar al releerlo" 1
fi

printf '\033[32m✔ certificado de %s en su lugar\033[0m — vence %s' "$NODO" "$DESPUES"
if [ -n "$ANTES" ] && [ "$ANTES" != "$DESPUES" ]; then
  printf ' (antes vencía %s)' "$ANTES"
elif [ -n "$ANTES" ]; then
  printf ' (sin cambios: Tailscale reusó el que ya tenía, que todavía está lejos de vencer)'
fi
printf '\n'
printf '  El cerebro lo toma SOLO: relee el par cuando cambia, no hace falta reiniciarlo.\n'
printf '  Que de verdad lo haya tomado se ve en musubi_tls_certificate_expiry_seconds, que sale\n'
printf '  del certificado que el proceso tiene CARGADO y no de este archivo.\n'
