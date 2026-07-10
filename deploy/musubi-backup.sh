#!/usr/bin/env bash
# musubi-backup.sh — backup PROGRAMADO y OFF-HOST del cerebro central.
#
# Toma un snapshot CONSISTENTE de la base (via `musubi backup`, que usa VACUUM INTO —
# puro-Go, no necesita el CLI sqlite3), lo copia a un destino FUERA del host, y purga
# los snapshots locales viejos. Lo dispara el timer systemd musubi-backup.timer (ver
# install-musubi-brain.sh). El runbook de RESTORE está en docs/Server_Brain_Onboarding.md.
#
# Por qué off-host: la memory.db central es el único punto donde converge la memoria
# compartida de todos los proyectos. Un backup en el MISMO disco no protege contra la
# pérdida del disco/host — por eso este script exige (o advierte fuerte por) un destino
# remoto.
#
# Configuración (por variables de entorno, típicamente desde el EnvironmentFile del timer):
#   MUSUBI_HOME            workspace del cerebro (REQUERIDO; ej. /home/musubi/musubi-brain)
#   MUSUBI_BIN             binario musubi (default: /usr/local/bin/musubi)
#   BACKUP_LOCAL_DIR       staging local (default: $MUSUBI_HOME/.musubi/backups)
#   BACKUP_REMOTE          destino OFF-HOST (ej. "user@host:/srv/backups/musubi",
#                          "rclone-remote:musubi", o "/mnt/backup/musubi"). Si está vacío,
#                          el script FALLA (exit≠0) — DR off-host segura por default: un backup
#                          en el mismo disco no es DR. Para aceptar EXPLÍCITAMENTE el modo
#                          local-only, seteá BACKUP_ALLOW_LOCAL_ONLY=1 (queda como advertencia).
#   BACKUP_ALLOW_LOCAL_ONLY  "1" ⇒ permite correr sin BACKUP_REMOTE (snapshot solo local). Es el
#                          escape hatch consciente al fallo-cerrado de arriba.
#   BACKUP_METHOD          rsync | rclone | cp   (default: rsync)
#   BACKUP_RETENTION_DAYS  purga snapshots locales más viejos que N días (default: 14)
set -euo pipefail

MUSUBI_BIN="${MUSUBI_BIN:-/usr/local/bin/musubi}"
: "${MUSUBI_HOME:?MUSUBI_HOME es requerido (workspace del cerebro)}"
export MUSUBI_HOME
BACKUP_LOCAL_DIR="${BACKUP_LOCAL_DIR:-$MUSUBI_HOME/.musubi/backups}"
BACKUP_REMOTE="${BACKUP_REMOTE:-}"
BACKUP_ALLOW_LOCAL_ONLY="${BACKUP_ALLOW_LOCAL_ONLY:-}"
BACKUP_METHOD="${BACKUP_METHOD:-rsync}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"

log() { printf '[musubi-backup] %s\n' "$*"; }
# mark_offhost_error deja la marca .last_offhost_error (timestamp + motivo) para que `musubi
# doctor` distinga "off-host configurado pero fallando / que NUNCA funcionó" de "instancia local"
# (Track 18). Best-effort: nunca hace fallar al script por sí misma.
mark_offhost_error() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" > "$BACKUP_LOCAL_DIR/.last_offhost_error" 2>/dev/null || true; }
# die_offhost registra el fallo off-host (para el check de doctor) y aborta.
die_offhost() { mark_offhost_error "$*"; die "$*"; }
die() { printf '[musubi-backup] ERROR: %s\n' "$*" >&2; exit 1; }

# 1. Snapshot consistente. `musubi backup` imprime SOLO la ruta del snapshot en stdout.
log "Tomando snapshot en $BACKUP_LOCAL_DIR ..."
SNAPSHOT="$("$MUSUBI_BIN" backup --out "$BACKUP_LOCAL_DIR")" || die "falló 'musubi backup'"
[ -f "$SNAPSHOT" ] || die "el snapshot no existe: $SNAPSHOT"
log "Snapshot OK: $SNAPSHOT ($(du -h "$SNAPSHOT" | cut -f1))"

# 2. Copia OFF-HOST. DR segura por default: sin destino remoto el backup NO es DR (queda en el
#    mismo disco), así que se FALLA-CERRADO salvo que se acepte local-only EXPLÍCITAMENTE. Un exit≠0
#    hace que systemd marque la unidad como failed → el operador lo ve en `systemctl status`, en vez
#    de un "éxito" silencioso que en realidad no protege nada.
if [ -z "$BACKUP_REMOTE" ]; then
  if [ "$BACKUP_ALLOW_LOCAL_ONLY" = "1" ]; then
    log "ADVERTENCIA: BACKUP_REMOTE vacío y BACKUP_ALLOW_LOCAL_ONLY=1 — el snapshot queda SOLO en el disco local (NO es DR)."
  else
    die_offhost "BACKUP_REMOTE vacío: un backup en el mismo disco NO protege contra la pérdida del host. Configurá BACKUP_REMOTE (rsync/rclone/cp a otra máquina o nube), o seteá BACKUP_ALLOW_LOCAL_ONLY=1 para aceptar el modo local-only a conciencia."
  fi
else
  log "Enviando off-host ($BACKUP_METHOD) → $BACKUP_REMOTE ..."
  case "$BACKUP_METHOD" in
    rsync)  rsync -a --mkpath "$SNAPSHOT" "$BACKUP_REMOTE/" || die_offhost "rsync falló" ;;
    rclone) rclone copy "$SNAPSHOT" "$BACKUP_REMOTE" || die_offhost "rclone falló" ;;
    cp)     mkdir -p "$BACKUP_REMOTE" && cp "$SNAPSHOT" "$BACKUP_REMOTE/" || die_offhost "cp falló" ;;
    *)      die_offhost "BACKUP_METHOD inválido: $BACKUP_METHOD (usá rsync|rclone|cp)" ;;
  esac
  log "Copia off-host OK."
  # Marca del dead-man's-switch: registra el MOMENTO del último envío off-host EXITOSO. `musubi
  # doctor` (check offhost_backup) avisa si esta marca envejece — el timer dejó de shipear.
  date -u +%Y-%m-%dT%H:%M:%SZ > "$BACKUP_LOCAL_DIR/.last_offhost" || log "no se pudo escribir la marca .last_offhost"
  # Envío OK ⇒ limpiar la marca de error (si venía de un fallo previo), así doctor vuelve a 'ok'.
  rm -f "$BACKUP_LOCAL_DIR/.last_offhost_error" 2>/dev/null || true
fi

# 3. Retención local (los snapshots off-host se retienen en el destino remoto).
if [ "$BACKUP_RETENTION_DAYS" -gt 0 ]; then
  PRUNED="$(find "$BACKUP_LOCAL_DIR" -maxdepth 1 -name 'memory.db.*' -type f -mtime "+$BACKUP_RETENTION_DAYS" -print -delete | wc -l)"
  log "Retención local: purgados $PRUNED snapshot(s) > $BACKUP_RETENTION_DAYS días."
fi

log "Backup completo."
