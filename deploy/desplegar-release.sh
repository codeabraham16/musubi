#!/usr/bin/env bash
#
# desplegar-release.sh — preflight + runbook de un release de Musubi.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# QUÉ HACE, Y QUÉ NO
#
# HACE (todo lectura, sin efectos hacia afuera): verifica que el release esté listo para desplegar
# —tag publicado, binarios subidos, firmado, el árbol y la rama en orden, las guardas verdes, el
# cerebro vivo— y después IMPRIME el viaje de comandos que falta, con los valores reales ya puestos
# (el tag, el sha256 del binario del server, las rutas). El dolor que saca es doble: deja de haber
# que acordarse de la secuencia y su orden, y deja de haber que verificar a mano doce assets.
#
# NO HACE: firmar, subir, commitear, taguear, empujar, redesplegar el cerebro ni tocar una máquina.
# Nada hacia afuera ni privilegiado. Regla de oro de la sala de mando: cada uno de esos pasos se
# aprueba y se corre en el momento, con el visto bueno explícito del dueño, y una aprobación NO se
# extiende al próximo. Por eso este guion imprime esos comandos en vez de ejecutarlos.
#
# Uso:
#   ./deploy/desplegar-release.sh                 # usa el tag v$(cat VERSION)
#   ./deploy/desplegar-release.sh v0.145.0        # un tag explícito
#   ./deploy/desplegar-release.sh 0.145.0         # la «v» es opcional
#   ./deploy/desplegar-release.sh --sin-guardas   # no corre `go test` (más rápido)
# ════════════════════════════════════════════════════════════════════════════════════════════
set -uo pipefail

REPO="codeabraham16/musubi"
RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$RAIZ" || { echo "no pude entrar a la raíz del repo ($RAIZ)" >&2; exit 1; }

SIN_GUARDAS=0
TAG=""
for arg in "$@"; do
  case "$arg" in
    --sin-guardas) SIN_GUARDAS=1 ;;
    v[0-9]*.[0-9]*.[0-9]*) TAG="$arg" ;;
    [0-9]*.[0-9]*.[0-9]*)  TAG="v$arg" ;;
    *) echo "argumento no reconocido: $arg" >&2; exit 2 ;;
  esac
done

if [[ ! -f VERSION ]]; then echo "no encontré VERSION en $RAIZ" >&2; exit 1; fi
VERSION="$(tr -d '[:space:]' < VERSION)"
[[ -z "$TAG" ]] && TAG="v$VERSION"

NOGO=0
# Color sólo si stdout es un terminal: en un pipe o un log las secuencias ANSI ensucian.
if [[ -t 1 ]]; then C_OK=$'\033[32m'; C_NO=$'\033[31m'; C_WARN=$'\033[33m'; C_RST=$'\033[0m'
else C_OK=''; C_NO=''; C_WARN=''; C_RST=''; fi
ok()   { printf '  %s✓%s %s\n' "$C_OK" "$C_RST" "$1"; }
no()   { printf '  %s✗%s %s\n' "$C_NO" "$C_RST" "$1"; NOGO=1; }
warn() { printf '  %s⚠%s %s\n' "$C_WARN" "$C_RST" "$1"; }

echo "══ Preflight de deploy — $TAG (VERSION=$VERSION) ══"

# 1. El tag concuerda con VERSION.
echo "· Versión"
if [[ "$TAG" == "v$VERSION" ]]; then
  ok "el tag $TAG concuerda con VERSION ($VERSION)"
else
  warn "vas a desplegar $TAG pero VERSION dice $VERSION — asegurate de que es a propósito"
fi

# 2. git: rama, árbol y qué commit tiene el tag.
echo "· git"
rama="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo '?')"
if [[ "$rama" == "main" ]]; then ok "estás en main"; else warn "estás en «$rama», no en main"; fi
if [[ -z "$(git status --porcelain 2>/dev/null)" ]]; then ok "árbol limpio"; else warn "hay cambios sin commitear en el árbol"; fi
if git rev-parse "$TAG" >/dev/null 2>&1; then
  if git merge-base --is-ancestor "$TAG" HEAD 2>/dev/null; then
    ok "el tag $TAG existe en local y es ancestro de HEAD"
  else
    warn "el tag $TAG existe en local pero NO es ancestro de HEAD (¿main sin traer?)"
  fi
else
  warn "el tag $TAG no existe en local (lo creás al publicar: git tag $TAG && git push origin $TAG)"
fi

# 3. Guardas que el CI exige verdes (las mismas que corre preparar-release).
if [[ "$SIN_GUARDAS" -eq 0 ]]; then
  echo "· Guardas (go test)"
  if go test ./cmd/musubi/ -run '^TestVersioninfoMatchesVERSION$' >/dev/null 2>&1 \
   && go test ./deploy/cmd/notas-release/ -run '^(TestChangelogRealTieneLaEstructuraEsperada|TestReadmeNovedadesAlDia|TestSinPublicarDeLosReadmeReales)$' >/dev/null 2>&1; then
    ok "versioninfo + CHANGELOG + README al día"
  else
    no "una guarda quedó en rojo — corré: go test ./cmd/musubi/ ./deploy/cmd/notas-release/"
  fi
else
  echo "· Guardas: salteadas (--sin-guardas)"
fi

# 4-6. El release en GitHub: existe, tiene los binarios, está firmado.
echo "· Release en GitHub ($REPO)"
assets=""
if assets="$(gh release view "$TAG" --repo "$REPO" --json assets --jq '.assets[].name' 2>/dev/null)"; then
  ok "el release $TAG existe"
else
  no "el release $TAG no existe en GitHub (lo crea release.yml al empujar el tag)"
fi

FIRMADO=0
if [[ -n "$assets" ]]; then
  # Los seis binarios que compila release.yml, cada uno con su .sha256.
  faltan=""
  for a in Musubi.exe Musubi-arm64.exe musubi-darwin-amd64 musubi-darwin-arm64 musubi-linux-amd64 musubi-linux-arm64; do
    grep -qxF "$a" <<<"$assets"        || faltan+=" $a"
    grep -qxF "$a.sha256" <<<"$assets" || faltan+=" $a.sha256"
  done
  if [[ -z "$faltan" ]]; then ok "los 6 binarios y sus .sha256 están subidos"; else no "faltan assets:$faltan"; fi

  if grep -qxF 'manifest.json' <<<"$assets" && grep -qxF 'manifest.json.sig' <<<"$assets"; then
    ok "firmado: manifest.json + manifest.json.sig presentes"
    FIRMADO=1
  else
    warn "SIN FIRMAR todavía: no están manifest.json / manifest.json.sig (es el próximo paso)"
  fi
fi

# 7. El cerebro central, vivo (informativo: es el estado ANTES de desplegar).
echo "· Cerebro central"
if command -v musubi >/dev/null 2>&1; then
  if musubi cerebro --alcance >/dev/null 2>&1; then ok "el cerebro contesta /readyz (200)"; else warn "musubi cerebro --alcance no dio 200 (¿binario viejo sin -alcance, o cerebro caído?)"; fi
else
  warn "no hay 'musubi' en el PATH de esta shell; verificá el cerebro a mano con: musubi cerebro --alcance"
fi

# ── Runbook ──────────────────────────────────────────────────────────────────────────────────
echo ""
if [[ "$NOGO" -eq 1 ]]; then
  echo "══ NO-GO: resolvé los ✗ de arriba antes de desplegar ══"
else
  echo "══ Preflight OK ══"
fi

echo ""
echo "── Viaje hacia afuera (revisá y corré a mano; NADA de esto lo ejecuta este guion) ──"

if [[ "$FIRMADO" -eq 0 ]]; then
  cat <<EOF
FIRMA (pendiente) — a mano, en la máquina del que publica, con la clave montada el rato que dura.

  Camino de un comando (gateado: firma y, SÓLO con tu OK tipeado, sube):
       ./deploy/firmar-y-subir.sh <clave.pem> $TAG --subir
     La clave ed25519 está en Vaultwarden (musubi-infra → «Musubi — clave de firma de releases»)
     como .pem CIFRADA. El guion te pide la contraseña, verifica que sea LA clave, baja los
     binarios EXACTOS del release, firma el manifiesto y te muestra qué va a subir antes de
     empujar. Sin --subir, firma y para. Authenticode es opcional y va PRIMERO (cambia el sha256
     del .exe que el manifest sella): agregá --authenticode <cert.crt> <clave.key> (pide osslsigncode).

  O paso a paso, si preferís:
  1. Bajá los binarios del release a un directorio:
       gh release download $TAG --repo $REPO -D /tmp/$TAG
  2. (Opcional) Authenticode PRIMERO — irreversible, cambia el sha256 del .exe:
       ./deploy/firmar-windows.sh /tmp/$TAG/Musubi.exe <cert.crt> <clave.key>
  3. Firmá el manifiesto ed25519 (lo ÚNICO que \`musubi update\` exige). OJO: toma el HEX de 64
     chars, no la .pem de Vaultwarden — hay que descifrarla+convertirla antes (lo hace el wrapper):
       ./deploy/firmar-release.sh $VERSION <clave-privada-ed25519-hex> /tmp/$TAG
  4. Subí el manifest + su firma (y, si hiciste Authenticode, los .exe y sus .sha256):
       gh release upload $TAG --repo $REPO --clobber /tmp/$TAG/manifest.json /tmp/$TAG/manifest.json.sig
  5. Volvé a correr este preflight: ya tiene que dar «firmado».

EOF
fi

# sha256 del binario del server (Rocky Linux x86_64 → musubi-linux-amd64), si se puede traer.
sha_server="<sha256 de musubi-linux-amd64 — está en su .sha256 del release>"
if [[ "$FIRMADO" -eq 1 ]]; then
  tmp_sha="$(mktemp -d 2>/dev/null || echo /tmp/musubi-sha-$$)"
  if gh release download "$TAG" --repo "$REPO" -p 'musubi-linux-amd64.sha256' -D "$tmp_sha" --clobber >/dev/null 2>&1; then
    leido="$(awk '{print $1; exit}' "$tmp_sha/musubi-linux-amd64.sha256" 2>/dev/null)"
    [[ -n "$leido" ]] && sha_server="$leido"
  fi
  rm -rf "$tmp_sha" 2>/dev/null
fi

cat <<EOF
DEPLOY DEL CEREBRO (con el visto bueno del dueño, en el momento — el server es Rocky Linux x86_64):
  1. Traé el binario del server y su sha256:
       gh release download $TAG --repo $REPO -p 'musubi-linux-amd64' -p 'musubi-linux-amd64.sha256' -D /tmp/$TAG
  2. Copialo al server y redesplegá (redesplegar-cerebro.sh saca su propio respaldo de binario+base
     y vuelve atrás solo si algo falla; necesita root, que pide contraseña):
       scp /tmp/$TAG/musubi-linux-amd64 server:/tmp/musubi-nuevo
       ssh server 'sudo ./deploy/redesplegar-cerebro.sh /tmp/musubi-nuevo $sha_server'
  3. Actualizá el resto de la flota según corresponda, y VERIFICÁ el alcance del cerebro:
       musubi cerebro --alcance    # tiene que dar 200

Recordá: el esquema de la base es una puerta de una sola dirección. Un binario viejo se niega a
abrir una base migrada por uno nuevo, así que volver atrás el binario NO alcanza — por eso el
redespliegue respalda las dos cosas. No mezcles versiones entre el cerebro y los que hidratan.
EOF

exit "$NOGO"
