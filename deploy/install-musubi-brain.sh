#!/usr/bin/env bash
#
# install-musubi-brain.sh — provisiona Musubi como CEREBRO CENTRAL (daemon MCP sobre
# HTTP) en un servidor Linux. Reproduce, idempotente, el montaje manual de la Fase 1:
# binario + workspace + bloque service + token + servicio systemd + contexto SELinux +
# firewall de la malla + verificación. Re-ejecutable sin romper nada (NO regenera el
# token si ya existe).
#
# Uso:
#   sudo ./install-musubi-brain.sh
#
# Variables de entorno opcionales (con defaults):
#   BRAIN_USER      usuario que corre el daemon; debe existir   (default: musubi)
#   BRAIN_HOME      workspace del cerebro = MUSUBI_HOME          (default: /home/$BRAIN_USER/musubi-brain)
#   BRAIN_ADDR      dirección de bind (tailnet-only vía fw)      (default: 0.0.0.0:7717)
#   MUSUBI_VERSION  tag de release o "latest"                   (default: latest)
#   MUSUBI_REPO     owner/repo de las releases                  (default: codeabraham16/musubi)
#   MUSUBI_BIN_SHA256  sha256 del binario que el operador espera, para cuando el .sha256 del
#                      release no se puede bajar o no se le quiere confiar. NO existe una
#                      variable para SALTEAR la verificación, y esto NO es una promesa escrita:
#                      lo sostienen dos pruebas en internal/mcp/despliegue_verificacion_*_test.go
#                      —una exige que ningún `install` sea alcanzable sin haber comparado el
#                      sha256, y otra corre el caso que tiene que frenar una vez por cada
#                      variable de entorno que el bloque lee—. El porqué está en el paso 1.
#
set -euo pipefail

BRAIN_USER="${BRAIN_USER:-musubi}"
BRAIN_HOME="${BRAIN_HOME:-/home/$BRAIN_USER/musubi-brain}"
BRAIN_ADDR="${BRAIN_ADDR:-0.0.0.0:7717}"
MUSUBI_VERSION="${MUSUBI_VERSION:-latest}"
MUSUBI_REPO="${MUSUBI_REPO:-codeabraham16/musubi}"
ENV_FILE="/etc/musubi/musubi.env"
BIN="/usr/local/bin/musubi"
UNIT="/etc/systemd/system/musubi-brain.service"
PORT="${BRAIN_ADDR##*:}"
# sha256 de deploy/musubi-backup.sh que ESTE instalador acepta instalar. Es el único origen honesto
# del checksum: un .sha256 publicado junto al script en `main` lo controla el mismo que controla el
# script (main no tiene branch protection), así que no verificaría nada. El pin vive acá, en el
# archivo que el operador ya confía porque lo corre con sudo desde su clone. Si cambiás
# deploy/musubi-backup.sh, actualizá este valor: sha256sum deploy/musubi-backup.sh
BACKUP_SHA256="e18add80c762b668c2220183afa3d50220eb75c6c2d22b9a5379c223a647dc72"
BACKUP_SCRIPT_URL="https://raw.githubusercontent.com/$MUSUBI_REPO/main/deploy/musubi-backup.sh"
BACKUP_BIN="/usr/local/bin/musubi-backup"
# Las unidades del timer, en variables y no escritas en el `cat >`: así el paso 5b se puede
# EJECUTAR entero en un arnés de prueba apuntando a un directorio temporal. Un bloque que sólo se
# puede leer se custodia con grep, y un grep lo satisface un comentario.
BACKUP_UNIT="/etc/systemd/system/musubi-backup.service"
BACKUP_TIMER="/etc/systemd/system/musubi-backup.timer"
# sha256 de deploy/redesplegar-cerebro.sh, mismo criterio y mismo motivo que el de arriba — con
# una razón MÁS fuerte: este guion reemplaza el binario del cerebro y se corre como root, así que
# es el peor archivo del despliegue para instalar sin verificar. Si lo cambiás, actualizá esto:
# sha256sum deploy/redesplegar-cerebro.sh
REDESPLIEGUE_SHA256="e988c1d8cc2c4759b558ae48b106d2289450834c6d9340320e51be0f35a92f3a"
REDESPLIEGUE_SCRIPT_URL="https://raw.githubusercontent.com/$MUSUBI_REPO/main/deploy/redesplegar-cerebro.sh"
# /usr/local/sbin y no el home de $BRAIN_USER: lo corre root, así que no puede vivir donde escribe
# un usuario sin privilegios. El porqué largo está en el paso 5c.
REDESPLIEGUE_BIN="/usr/local/sbin/redesplegar-cerebro.sh"
# Directorio del propio instalador: cuando se corre desde el clone (sudo ./install-musubi-brain.sh),
# musubi-backup.sh está al lado y no hace falta bajar nada de la red.
AQUI="$(cd "$(dirname "$0")" 2>/dev/null && pwd || echo /nonexistent)"

log(){ printf '\033[36m▶ %s\033[0m\n' "$*"; }
ok(){  printf '\033[32m✓ %s\033[0m\n' "$*"; }
die(){ printf '\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "Corré con sudo/root."
id "$BRAIN_USER" &>/dev/null || die "El usuario '$BRAIN_USER' no existe. Crealo primero (useradd -m $BRAIN_USER)."
command -v curl &>/dev/null || die "Falta 'curl'."
command -v openssl &>/dev/null || die "Falta 'openssl'."

case "$(uname -m)" in
  x86_64)        ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "Arquitectura no soportada: $(uname -m)" ;;
esac

# ── 1. Binario ──────────────────────────────────────────────────────────────
if [ "$MUSUBI_VERSION" = latest ]; then
  URL="https://github.com/$MUSUBI_REPO/releases/latest/download/musubi-linux-$ARCH"
else
  URL="https://github.com/$MUSUBI_REPO/releases/download/$MUSUBI_VERSION/musubi-linux-$ARCH"
fi
log "Descargando binario ($ARCH): $URL"
tmp="$(mktemp)"; tmpsha="$(mktemp)"
curl -fsSL "$URL" -o "$tmp"
# EL BINARIO DEL CEREBRO NO SE INSTALA SIN VERIFICAR.
#
# Acá había un `if` SIN `else`: si el `.sha256` no bajaba —o bajaba vacío— el bloque no entraba y
# el `install` de abajo se hacía IGUAL, en silencio. Fail-OPEN sobre el archivo más privilegiado
# del despliegue, que además corre como servicio. Y era el hermano sin la guarda de los tres que
# baja este instalador: el guion de backup (BACKUP_SHA256) y el de redespliegue
# (REDESPLIEGUE_SHA256) tienen pin duro, y el comentario de éste último dice, sobre un guion MENOS
# privilegiado que este binario, que es «el peor archivo del despliegue para instalar sin
# verificar».
#
# POR QUÉ NO LLEVA UN PIN DURO COMO SUS DOS HERMANOS. El pin de ellos funciona porque esos archivos
# viven en ESTE commit: el número es computable al escribirlo y una prueba de Go lo custodia contra
# pudrirse. El binario es el artefacto de un release FUTURO —con MUSUBI_VERSION=latest ni siquiera
# se sabe cuál—, así que un pin acá congelaría el instalador en una versión y convertiría cada
# release en una edición de este archivo.
#
# Y LA URL VERSIONADA NO REEMPLAZA AL CHECKSUM: nombra el asset, no atestigua sus bytes. Una
# descarga truncada, un proxy o portal cautivo que contesta 200 con HTML, y un asset RE-SUBIDO
# sobre el mismo tag (los assets de un release de GitHub son mutables para cualquiera con write —
# el tag no) pasan los tres por una URL versionada perfecta.
#
# QUÉ VERIFICA Y QUÉ NO. El `.sha256` lo publica `release.yml` al lado de cada asset, en la misma
# corrida que lo compila (`sha256sum "$asset" > "$asset.sha256"`). Sale del mismo origen que el
# binario, así que NO defiende contra quien controle el release entero; para eso está
# MUSUBI_BIN_SHA256, que trae el número por un camino que elige el operador. Sí defiende contra
# todo lo demás, y su ausencia significa «no pude medir», que no es lo mismo que «medí y está
# bien»: por eso frena.
#
# NO HAY VÍA DE ESCAPE FAIL-OPEN, Y NO ALCANZA CON ESCRIBIRLO ACÁ. Esto mismo estaba afirmado en
# tres lugares —esta línea, la cabecera del guion y el commit que lo cerró— sostenido por CERO
# código: se envolvió todo este bloque en un `if` guardado por una variable nueva, con un `else`
# que sólo loguea, y las ocho pruebas del paso 1 siguieron en verde porque ninguna exportaba esa
# variable. Un doc que miente es peor que uno que falta: el que lo lee deja de buscar.
#
# LO QUE LO SOSTIENE HOY, en internal/mcp:
#   despliegue_verificacion_forma_test.go   — ningún `install` de este repo es alcanzable sin que
#                                             una comparación del sha256 lo domine. Envolver esto
#                                             en un `if` nuevo rompe la dominancia aunque el `if`
#                                             venga apagado.
#   despliegue_verificacion_corrida_test.go — corre el caso que tiene que frenar una vez por cada
#                                             variable de entorno que este bloque LEE (la lista se
#                                             deriva del guion, no está escrita en la prueba).
#
# Un `MUSUBI_SIN_VERIFICAR=1` termina copiado en un
# runbook —y de ahí en el próximo, y en el de la máquina siguiente— y a partir de ese momento la
# verificación está muerta en todas partes mientras se ve viva. Es la forma exacta de A111: una
# comprobación vacuamente cierta que pasó en seis redespliegues sin comprobar nada. La salida para
# el operador es DAR el sha que espera, no saltear la comprobación — que es además la forma que ya
# tiene `redesplegar-cerebro.sh`, donde el sha esperado es un argumento obligatorio.
if [ -n "${MUSUBI_BIN_SHA256:-}" ]; then
  want="$(printf '%s' "$MUSUBI_BIN_SHA256" | tr 'A-Z' 'a-z')"
  origen_sha="MUSUBI_BIN_SHA256 (dado por el operador)"
elif curl -fsSL "$URL.sha256" -o "$tmpsha" && [ -s "$tmpsha" ]; then
  want="$(awk '{print $1}' "$tmpsha" | tr 'A-Z' 'a-z')"
  origen_sha="$URL.sha256"
else
  rm -f "$tmp" "$tmpsha"
  die "NO se verificó el binario del cerebro y por eso NO se instaló: $URL.sha256 no bajó o vino vacío, y no hay MUSUBI_BIN_SHA256. Sin ese número no se distingue un binario íntegro de una descarga truncada o de un asset re-subido, y esto es lo que va a correr como servicio en el cerebro. Salidas: (a) reintentá, si fue la red; (b) si el release no publicó su .sha256, miralo en la corrida de release.yml del tag; (c) conseguí el sha del asset por un camino que confíes y pasalo:  MUSUBI_BIN_SHA256=<sha256> sudo $0"
fi
# Un `want` que no tiene forma de sha256 es «no pude medir» disfrazado de medición: pasa cuando el
# .sha256 vino con HTML de un portal cautivo, o cuando el operador pegó de más. Compararlo contra
# $got daría distinto y moriría igual, pero con el mensaje equivocado —«no coincide», que manda a
# buscar un binario adulterado— en vez del que corresponde.
if ! [[ "$want" =~ ^[0-9a-f]{64}$ ]]; then
  rm -f "$tmp" "$tmpsha"
  die "NO se verificó el binario del cerebro y por eso NO se instaló: lo que llegó de $origen_sha no tiene forma de sha256 (${want:0:80}). Si viene de la red, en el medio contestó algo que no era el archivo (un proxy, un portal cautivo). Revisalo a mano:  curl -fsSL $URL.sha256"
fi
got="$(sha256sum "$tmp" | awk '{print $1}')"
if [ "$want" != "$got" ]; then
  rm -f "$tmp" "$tmpsha"
  die "El binario del cerebro NO coincide con su checksum (origen=$origen_sha want=$want got=$got). NO se instaló. O la descarga se truncó, o el asset del release no es el que produjo esa corrida: NO lo instales a mano hasta saber cuál de las dos es."
fi
ok "Checksum del binario verificado ($origen_sha)"
# 'install' (no 'mv') aplica el contexto correcto del destino; igual forzamos restorecon.
install -m 0755 "$tmp" "$BIN"
rm -f "$tmp" "$tmpsha"
if command -v restorecon &>/dev/null; then restorecon -v "$BIN" || true; fi   # SELinux (gotcha Fase 1)
ok "Binario instalado: $("$BIN" version)"

# ── 2. Workspace ────────────────────────────────────────────────────────────
if [ ! -f "$BRAIN_HOME/.musubi/config.yaml" ]; then
  log "Inicializando workspace en $BRAIN_HOME"
  install -d -o "$BRAIN_USER" -g "$BRAIN_USER" "$BRAIN_HOME"
  sudo -u "$BRAIN_USER" env MUSUBI_HOME="$BRAIN_HOME" "$BIN" init
else
  ok "Workspace ya existe: $BRAIN_HOME"
fi
CFG="$BRAIN_HOME/.musubi/config.yaml"

# --- 3. Bloque service (idempotente: siempre lo deja en el estado deseado) ---
#
# SE BORRA SOLO EL BLOQUE service, NO DE AHI HASTA EL FINAL.
#
# Esto decia `sed -i '/^service:/,$d'` y se justificaba con "'service:' es el ultimo bloque del
# config generado por 'musubi init'". Fue verdad y dejo de serlo: hoy `Default().Marshal()` pone
# `service:` en la linea 119 y `sync:` en la 123 (medido el 2026-09-05). O sea que re-correr este
# instalador BORRA la configuracion de sync que hubiera, sin decir nada.
#
# Hoy el dano es cero porque el cerebro no tiene bloque sync -- pero eso es una foto, no una
# garantia, y es exactamente la forma que este repo persigue: un comentario que describe un estado
# anterior y se lee como el actual. El `.bak` de al lado es la unica red, y se pisa en cada corrida.
#
# Se borra hasta la proxima clave de primer nivel (una linea que empieza sin espacio y no es
# comentario), que es lo que "el bloque service" significa en YAML.
log "Configurando bloque service (addr=$BRAIN_ADDR)"
cp -f "$CFG" "$CFG.bak"
awk '
  /^service:/ { dentro=1; next }
  dentro && /^[^[:space:]#]/ { dentro=0 }
  !dentro
' "$CFG" > "$CFG.sin-service"
# `cat >` y no `mv`: conserva el inodo y la etiqueta del archivo. Es la leccion de A82 y la de
# SELinux -- un `mv` crea una entrada nueva y le cambia el contexto al destino.
cat "$CFG.sin-service" > "$CFG"
rm -f "$CFG.sin-service"
cat >> "$CFG" <<EOF
service:
    enabled: true
    addr: "$BRAIN_ADDR"
    # OLA 0 DEL PLAN EMPRESA (2026-09-03): fail-closed contra el modo legacy en bind remoto.
    # Sin registro de principals (o con solo el bearer legacy), un bind no-loopback es un unico
    # token con acceso TOTAL a todos los proyectos. Apagado, el cerebro solo lo AVISA en el log
    # al arrancar —y un aviso en un log que nadie lee es lo mismo que nada—. Encendido, se niega
    # a servir hasta que exista principals.yaml con al menos un miembro. Hoy es un cinturon que
    # no aprieta: el registro existe. El dia que alguien lo borre, esto lo va a decir en la cara.
    strict_tenancy: true
    auth_token_env: "MUSUBI_TOKEN"
    allow_insecure_token: true
    request_timeout_seconds: 60
EOF
chown "$BRAIN_USER:$BRAIN_USER" "$CFG"
ok "Bloque service configurado"

# ── 4. Token (NO se regenera si ya existe: romperia a los clientes) ─────────
install -d -m 0755 /etc/musubi
if [ ! -f "$ENV_FILE" ]; then
  log "Generando token"
  umask 077
  echo "MUSUBI_TOKEN=$(openssl rand -hex 32)" > "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  ok "Token generado en $ENV_FILE"
else
  ok "Token ya existe (no se regenera): $ENV_FILE"
fi

# ── 5. Servicio systemd ─────────────────────────────────────────────────────
log "Escribiendo unit systemd"
cat > "$UNIT" <<EOF
[Unit]
Description=Musubi cerebro central (MCP HTTP daemon)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$BRAIN_USER
Group=$BRAIN_USER
Environment=MUSUBI_HOME=$BRAIN_HOME
EnvironmentFile=$ENV_FILE
ExecStart=$BIN serve
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=$BRAIN_HOME
ProtectControlGroups=true
ProtectKernelTunables=true
ProtectKernelModules=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now musubi-brain.service
ok "Servicio systemd habilitado y arrancado"

# ── 5b. Backup programado OFF-HOST (musubi-backup.timer) ─────────────────────
# El cerebro central es el único punto donde converge la memoria compartida: sin backup
# off-host, perder el disco = perder toda la memoria. El timer toma un snapshot consistente
# (musubi backup = VACUUM INTO) y lo shipa a BACKUP_REMOTE. Runbook de restore en
# docs/Server_Brain_Onboarding.md.
log "Instalando backup programado (musubi-backup.timer)"
# El script lo va a ejecutar un timer como $BRAIN_USER, con el EnvironmentFile del cerebro cargado
# (token incluido). Por eso NUNCA se instala sin verificar el sha256 contra $BACKUP_SHA256 — mismo
# criterio que el binario en el paso 1. Fuente preferida: el archivo hermano del clone (sin red).
# Fallback: bajarlo de main — y verificarlo igual, porque main no está protegido.
tmpbk="$(mktemp)"
if [ -f "$AQUI/musubi-backup.sh" ]; then
  cp "$AQUI/musubi-backup.sh" "$tmpbk"
  origen_bk="$AQUI/musubi-backup.sh"
elif curl -fsSL "$BACKUP_SCRIPT_URL" -o "$tmpbk"; then
  origen_bk="$BACKUP_SCRIPT_URL"
else
  rm -f "$tmpbk"; origen_bk=""
fi
if [ -n "$origen_bk" ]; then
  got_bk="$(sha256sum "$tmpbk" | awk '{print $1}')"
  if [ "$got_bk" != "$BACKUP_SHA256" ]; then
    rm -f "$tmpbk"
    die "musubi-backup.sh NO coincide con el sha256 que espera este instalador (origen=$origen_bk want=$BACKUP_SHA256 got=$got_bk). NO se instaló. Si vos cambiaste deploy/musubi-backup.sh, actualizá BACKUP_SHA256 en este instalador (sha256sum deploy/musubi-backup.sh). Si NO lo tocaste, alguien cambió el script: revisá 'git log -p deploy/musubi-backup.sh' antes de seguir."
  fi
  ok "Checksum de musubi-backup.sh verificado ($origen_bk)"
  # 'install' (no 'mv') aplica el contexto SELinux del destino; igual forzamos restorecon (gotcha Fase 1).
  install -m 0755 "$tmpbk" "$BACKUP_BIN"
  rm -f "$tmpbk"
  command -v restorecon &>/dev/null && restorecon -v "$BACKUP_BIN" || true
  # Config de backup en el EnvironmentFile (idempotente: no pisa valores ya presentes).
  if ! grep -q '^BACKUP_' "$ENV_FILE" 2>/dev/null; then
    cat >> "$ENV_FILE" <<EOF

# Backup off-host (musubi-backup.timer). DR segura por default: con BACKUP_REMOTE vacío el
# backup FALLA-CERRADO (la unidad systemd queda 'failed' y se ve en 'systemctl status
# musubi-backup') en vez de dejar un snapshot solo-local que no protege contra la pérdida del
# disco. Configurá BACKUP_REMOTE (rsync/rclone/cp a otra máquina de la malla o a la nube), o
# seteá BACKUP_ALLOW_LOCAL_ONLY=1 para aceptar el modo local-only a conciencia.
BACKUP_REMOTE=
BACKUP_ALLOW_LOCAL_ONLY=
BACKUP_METHOD=rsync
BACKUP_RETENTION_DAYS=14
EOF
  fi
  cat > "$BACKUP_UNIT" <<EOF
[Unit]
Description=Musubi backup del cerebro central (snapshot off-host)
After=musubi-brain.service

[Service]
Type=oneshot
User=$BRAIN_USER
Group=$BRAIN_USER
Environment=MUSUBI_HOME=$BRAIN_HOME
Environment=MUSUBI_BIN=$BIN
EnvironmentFile=$ENV_FILE
ExecStart=$BACKUP_BIN
EOF
  cat > "$BACKUP_TIMER" <<EOF
[Unit]
Description=Musubi backup diario del cerebro central

[Timer]
OnCalendar=*-*-* 03:30:00
Persistent=true

[Install]
WantedBy=timers.target
EOF
  systemctl daemon-reload
  systemctl enable --now musubi-backup.timer
  ok "Backup diario habilitado (03:30). CONFIGURÁ BACKUP_REMOTE en $ENV_FILE (si no, la unidad falla-cerrado; o seteá BACKUP_ALLOW_LOCAL_ONLY=1)."
else
  log "No hay musubi-backup.sh junto al instalador y no se pudo bajar de $BACKUP_SCRIPT_URL; instalá el timer a mano (ver docs/Server_Brain_Onboarding.md)."
fi

# ── 5c. El guion de REDESPLIEGUE (A111) ──────────────────────────────────────
# POR QUÉ ESTE PASO EXISTE, que es la lección y no el trámite.
#
# Hasta el 2026-09-05 este instalador ponía UN guion derivado —musubi-backup— y `redesplegar-
# cerebro.sh` llegaba a mano. Medido ese día: el que tenía instalador coincidía BYTE A BYTE con el
# repo, y el que llegaba a mano estaba a 9690 bytes contra 19469, con su verificación de la
# migración muerta (`[[ "$ESQUEMA" -ge 37 ]]` con la base ya en 46: vacuamente cierta, pasaba sin
# comprobar nada, y pasó así en seis redespliegues). La diferencia entre los dos no fue el cuidado
# de nadie: fue que uno tenía compuerta de sha256 y el otro no tenía nada.
#
# Y VA A /usr/local/sbin, NO AL HOME DE $BRAIN_USER, donde estaba. Este guion se corre con sudo, y
# un archivo que ejecuta root desde un directorio que escribe un usuario sin privilegios es un
# camino de escalada. Acá no es teórico: `musubi_fleet_exec` corre en este servidor como ese mismo
# usuario, así que quien alcance el canal del agente podía dejar código escrito ahí y esperar al
# próximo redespliegue. La copia vieja NO se borra sola —es un archivo que puso una persona— pero
# se avisa fuerte, y `verificar-despliegue.sh` la marca en rojo hasta que no esté.
log "Instalando el guion de redespliegue del cerebro"
tmprd="$(mktemp)"
if [ -f "$AQUI/redesplegar-cerebro.sh" ]; then
  cp "$AQUI/redesplegar-cerebro.sh" "$tmprd"
  origen_rd="$AQUI/redesplegar-cerebro.sh"
elif curl -fsSL "$REDESPLIEGUE_SCRIPT_URL" -o "$tmprd"; then
  origen_rd="$REDESPLIEGUE_SCRIPT_URL"
else
  rm -f "$tmprd"; origen_rd=""
fi
if [ -n "$origen_rd" ]; then
  got_rd="$(sha256sum "$tmprd" | awk '{print $1}')"
  if [ "$got_rd" != "$REDESPLIEGUE_SHA256" ]; then
    rm -f "$tmprd"
    die "redesplegar-cerebro.sh NO coincide con el sha256 que espera este instalador (origen=$origen_rd want=$REDESPLIEGUE_SHA256 got=$got_rd). NO se instaló. Si vos cambiaste deploy/redesplegar-cerebro.sh, actualizá REDESPLIEGUE_SHA256 en este instalador (sha256sum deploy/redesplegar-cerebro.sh). Si NO lo tocaste, alguien cambió el guion que reemplaza el binario del cerebro corriendo como root: revisá 'git log -p deploy/redesplegar-cerebro.sh' antes de seguir."
  fi
  ok "Checksum de redesplegar-cerebro.sh verificado ($origen_rd)"
  # 0755 root:root a propósito: lo LEE y lo corre root, y no lo escribe nadie más.
  install -m 0755 -o root -g root "$tmprd" "$REDESPLIEGUE_BIN"
  rm -f "$tmprd"
  command -v restorecon &>/dev/null && restorecon -v "$REDESPLIEGUE_BIN" || true
  ok "Redespliegue disponible:  sudo $REDESPLIEGUE_BIN /ruta/al/binario-nuevo <sha256>"
  # La copia vieja en el home del usuario del cerebro. No se borra —la puso una persona— pero
  # dejarla callada sería peor que no haberla movido: son dos guiones que hacen lo mismo y sólo
  # uno se actualiza.
  LEGADO_RD="/home/$BRAIN_USER/redesplegar-cerebro.sh"
  if [ -e "$LEGADO_RD" ]; then
    printf '\033[33m! Quedó la copia vieja en %s. Son dos cosas: alguien puede correr ÉSA sin notarlo (y no la actualiza nadie), y está en un directorio que escribe el usuario %s —el mismo con el que corre musubi_fleet_exec— para un guion que se invoca con sudo. Sacala:  sudo rm %s\033[0m\n' "$LEGADO_RD" "$BRAIN_USER" "$LEGADO_RD"
  fi
else
  log "No hay redesplegar-cerebro.sh junto al instalador y no se pudo bajar de $REDESPLIEGUE_SCRIPT_URL; el cerebro queda instalado pero SIN guion de redespliegue (ver deploy/RUNBOOK.md)."
fi

# ── 6. Firewall de la malla (best-effort) ───────────────────────────────────
if command -v firewall-cmd &>/dev/null && systemctl is-active --quiet firewalld; then
  if ip link show tailscale0 &>/dev/null; then
    firewall-cmd --zone=trusted --add-interface=tailscale0 --permanent &>/dev/null || true
    firewall-cmd --reload &>/dev/null || true
    ok "firewalld: tailscale0 en zona 'trusted' (puerto $PORT solo alcanzable por la malla)"
  else
    log "tailscale0 aún no existe. Tras 'tailscale up', corré:"
    printf '    firewall-cmd --zone=trusted --add-interface=tailscale0 --permanent && firewall-cmd --reload\n'
  fi
fi

# ── 7. Verificación ─────────────────────────────────────────────────────────
sleep 1
if curl -fsS "http://127.0.0.1:$PORT/readyz" >/dev/null; then
  ok "Cerebro respondiendo: http://127.0.0.1:$PORT/readyz"
else
  die "El daemon no responde. Revisá:  journalctl -u musubi-brain -n 30 --no-pager"
fi

echo
# El token NO se imprime: quedaría en el scrollback de la terminal y en el log de cualquier sesión
# ssh grabada. Mismo criterio que deploy/docker/preparar.sh: se muestra SOLO su sha256 (que es
# además el token_sha256 que va en principals.yaml) y la ruta, y el operador lo lee cuando lo
# necesita. Se hashea el VALOR del token, no el archivo entero (que también lleva las BACKUP_*).
TOKEN_SHA="$(sed -n 's/^MUSUBI_TOKEN=//p' "$ENV_FILE" | head -n1 | tr -d '\n' | sha256sum | cut -d' ' -f1)"
ok "CEREBRO LISTO. El token está en $ENV_FILE (modo 600, NO se imprime acá)."
echo "  token_sha256: $TOKEN_SHA"
echo "  Para leerlo cuando lo necesités:  sudo cat $ENV_FILE"
echo
echo "Siguiente: en cada dispositivo, connect-brain-linux.sh / connect-brain-windows.ps1"
