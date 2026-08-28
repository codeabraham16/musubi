#!/usr/bin/env bash
#
# preparar.sh — deja el relay de pantalla corriendo con podman/docker compose, sin root.
#
# Es el hermano de deploy/docker/preparar.sh: mismo estilo, mismas manías. Las dos que importan:
#
#   1. NO da por hecho que arrancó. Levanta, espera, y VERIFICA que los puertos contestan y que la
#      clave existe. Un `compose up` que devuelve 0 no prueba nada: la cadena de alertas ya enseñó
#      que un servicio puede reportar éxito con cero reglas cargadas.
#   2. Imprime la clave pública, que es el único dato que hace falta del otro lado. Sin ella los
#      clientes se registran contra el servidor PÚBLICO de RustDesk creyendo que usan el propio.
#
# Uso:  ./preparar.sh [--bind <ip>]
set -uo pipefail

log(){ printf '\033[36m▶ %s\033[0m\n' "$*"; }
ok(){  printf '\033[32m✓ %s\033[0m\n' "$*"; }
aviso(){ printf '\033[33m! %s\033[0m\n' "$*"; }
die(){ printf '\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

AQUI="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIND=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --bind) BIND="${2:-}"; shift 2 ;;
    *) die "argumento desconocido: $1" ;;
  esac
done

# ── El runtime ───────────────────────────────────────────────────────────────────────────────
#
# EL ORDEN IMPORTA, y costó un despliegue fallido: en un servidor con podman rootless y el binario
# `docker-compose` instalado, llamar a `docker-compose` a secas busca el socket de Docker en
# /var/run/docker.sock, no lo encuentra, y muere. `podman compose` usa EL MISMO docker-compose
# pero apuntándole el DOCKER_HOST al socket de podman del usuario. Así que si hay podman, se
# pregunta por podman primero — tener docker-compose en el PATH no significa que haya un Docker.
if command -v podman >/dev/null 2>&1 && podman compose version >/dev/null 2>&1; then
  COMPOSE=(podman compose)
elif command -v podman-compose >/dev/null 2>&1; then COMPOSE=(podman-compose)
elif docker compose version >/dev/null 2>&1;   then COMPOSE=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then COMPOSE=(docker-compose)
else die "no hay compose (podman compose, podman-compose, docker compose ni docker-compose)"; fi
ok "compose: ${COMPOSE[*]}"

# ── El bind: tailnet por defecto, igual que el instalador systemd ────────────────────────────
#
# Que el default sea el mismo en los dos caminos NO es cosmético: si uno atara al tailnet y el
# otro a 0.0.0.0, elegir el despliegue por comodidad cambiaría la exposición del relay sin que
# nadie lo decidiera.
if [[ -z "$BIND" ]]; then
  BIND="$(tailscale ip -4 2>/dev/null | head -1 || true)"
  [[ -z "$BIND" ]] && BIND="$(ip -4 -o addr show 2>/dev/null | awk '{print $4}' | cut -d/ -f1 \
      | awk -F. '$1==100 && $2>=64 && $2<=127' | head -1 || true)"
  if [[ -n "$BIND" ]]; then
    ok "atando al tailnet: $BIND"
  else
    die "no se encontró IP de tailnet. Pasá --bind <ip> a conciencia y leé el README: un relay en 0.0.0.0 queda a un firewall de distancia de internet."
  fi
fi
if [[ "$BIND" == "0.0.0.0" ]]; then
  aviso "--bind 0.0.0.0: el relay va a escuchar en TODAS las interfaces, incluidas las públicas."
  aviso "Eso es el «acceso híbrido» del README y es una decisión, no un default. Sigo."
fi

DIR="${MUSUBI_RUSTDESK_DIR:-$HOME/musubi-rustdesk}"
mkdir -p "$DIR/data" || die "no se pudo crear $DIR/data"
cp -f "$AQUI/compose.yml" "$DIR/compose.yml"

cat > "$DIR/.env" <<ENV
MUSUBI_RUSTDESK_DIR=$DIR
MUSUBI_RUSTDESK_BIND=$BIND
ENV
ok "configuración en $DIR"

# ── SELinux ──────────────────────────────────────────────────────────────────────────────────
# Sin la etiqueta, el contenedor no puede escribir su base ni su clave. El `:z` del compose la
# pone, pero sólo si el volumen se monta limpio; re-preparar con los contenedores arriba deja
# archivos con la etiqueta del home. Se fuerza acá, que es barato y evita una tarde de journal.
if command -v chcon >/dev/null 2>&1 && [[ "$(getenforce 2>/dev/null || echo Disabled)" != "Disabled" ]]; then
  chcon -R -t container_file_t "$DIR/data" 2>/dev/null && ok "SELinux: $DIR/data etiquetado" \
    || aviso "no se pudo etiquetar $DIR/data para SELinux; si el relay no guarda nada, empezá por acá"
fi

# ── Arriba ───────────────────────────────────────────────────────────────────────────────────
log "levantando hbbs y hbbr"
( cd "$DIR" && "${COMPOSE[@]}" up -d ) || die "compose up falló"

# ── VERIFICAR. Que `up -d` devuelva 0 no dice nada sobre si el servicio quedó útil. ───────────
log "verificando"
falla=0

for i in $(seq 1 30); do
  [[ -f "$DIR/data/id_ed25519.pub" ]] && break
  sleep 1
done
if [[ ! -f "$DIR/data/id_ed25519.pub" ]]; then
  aviso "hbbs no generó su clave en 30s. Log:"
  ( cd "$DIR" && "${COMPOSE[@]}" logs --tail 30 hbbs 2>&1 | sed 's/^/    /' )
  falla=1
fi

# Los puertos, uno por uno y por su nombre: «el relay anda» es justo el diagnóstico que no sirve.
for p in 21115 21116 21117; do
  if (exec 3<>"/dev/tcp/$BIND/$p") 2>/dev/null; then
    ok "TCP $BIND:$p contesta"
  else
    aviso "TCP $BIND:$p NO contesta"
    falla=1
  fi
done
# El UDP no se puede probar con /dev/tcp; se verifica que esté publicado, que es lo que falla.
if ss -lnu 2>/dev/null | grep -q ":21116"; then
  ok "UDP 21116 publicado"
else
  aviso "UDP 21116 no aparece escuchando: sin él los clientes NO se registran (el latido va por UDP)"
  falla=1
fi

echo
if [[ $falla -ne 0 ]]; then
  die "el relay quedó a medias — arreglá lo de arriba antes de configurar ningún cliente"
fi

CLAVE="$(cat "$DIR/data/id_ed25519.pub")"
ok "Relay listo. En CADA máquina de la flota, en el cliente RustDesk:"
echo "    ID Server:    ${BIND}:21116"
echo "    Relay Server: ${BIND}:21117"
echo "    Key:          ${CLAVE}"
echo
aviso "Y leé deploy/rustdesk/README.md §1 y §2 antes de darlo por hecho:"
aviso "  · el «confirmar antes de conectar» lo aplica RustDesk, NO Musubi;"
aviso "  · si ponés una contraseña permanente a mano, existe un acceso que no pasa por la compuerta"
aviso "    ni queda en la bitácora — que es exactamente lo que este diseño evita."
