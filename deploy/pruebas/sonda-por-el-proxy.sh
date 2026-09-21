#!/usr/bin/env bash
#
# sonda-por-el-proxy.sh — ejercita `veredicto_sonda_mcp` de `verificar-despliegue.sh`.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# POR QUE EXISTE: EL VERIFICADOR NUNCA TOCABA EL PROXY
#
# Hasta el 2026-09-21 la postura de TLS se decidia leyendo el CONFIG. El 2026-09-20 se ato el
# cerebro a loopback en produccion y `/mcp` por el proxy empezo a contestar 403 —atar el bind
# ENCIENDE la defensa anti DNS-rebinding, que exige un `Host` loopback, y `tailscale serve`
# preserva el del tailnet—. El informe lo habria cantado VERDE: el config decia exactamente lo que
# el queria ver. Leer configuracion no es medir alcance.
#
# SE EXTRAE LA FUNCION DEL ARCHIVO DE PRODUCCION, no se la copia: una replica de la logica prueba
# la replica. Si la funcion deja de existir o cambia de nombre, esto CORTA en vez de pasar en verde
# sobre nada — una prueba que no encuentra que ejercitar no es una prueba que pasa.
#
# Uso:  ./deploy/pruebas/sonda-por-el-proxy.sh [ruta/a/verificar-despliegue.sh]
set -uo pipefail

VERIF="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/verificar-despliegue.sh}"
[[ -r "$VERIF" ]] || { echo "✗ no puedo leer $VERIF"; exit 2; }

FALLOS=0
ok()  { printf '  \033[32m✔ %s\033[0m\n' "$1"; }
mal() { printf '  \033[31m✘ %s\033[0m\n' "$1"; FALLOS=$((FALLOS+1)); }

extraer() {  # extraer <nombre> — el cuerpo de `nombre() { ... }` hasta el `}` en la columna 0
  awk -v f="$1" '$0 ~ "^"f"\\(\\) \\{" {d=1} d {print} d && /^\}/ {exit}' "$VERIF"
}
CUERPO="$(extraer veredicto_sonda_mcp)"
if [[ -z "$CUERPO" ]]; then
  echo "✗ no encontre la funcion \`veredicto_sonda_mcp\` en $VERIF."
  echo "  O se renombro, o cambio de forma, o la sonda por el proxy ya no esta. En los tres casos"
  echo "  esta prueba no estaria ejercitando nada, asi que corta aca."
  exit 2
fi

# Los tres veredictos se reemplazan por grabadoras: lo que se ejercita es el DESPACHO —que codigo
# manda a que color—, no como se imprime.
VEREDICTO=""
verde()  { VEREDICTO="verde";  MENSAJE="$1"; }
rojo()   { VEREDICTO="rojo";   MENSAJE="$1"; }
dudoso() { VEREDICTO="dudoso"; MENSAJE="$1"; }
eval "$CUERPO"

probar() {  # probar <codigo> <esperado> <por que importa>
  VEREDICTO=""; MENSAJE=""
  veredicto_sonda_mcp "$1" "musubi-server.ts.net:10000" "hay-proxy"
  if [[ "$VEREDICTO" == "$2" ]]; then ok "HTTP ${1:-(vacio)} → $2"
  else mal "HTTP ${1:-(vacio)} dio «${VEREDICTO:-nada}» y tiene que dar «$2»: $3"; fi
}

printf '\033[1mla sonda por el proxy — los cinco caminos\033[0m\n'

probar 401 verde "sin credencial, un 401 es la respuesta BUENA: la puerta vive y la custodia el bearer"
probar 429 verde "un 429 tambien dice que la puerta vive; el candado por IP viene de fallos previos, no de esta sonda"
probar 403 rojo  "ES EL FALLO DEL 2026-09-20: la compuerta anti-rebinding rechaza al proxy por su Host mientras el config se ve perfecto"
probar 000 rojo  "no se llego al cerebro por el proxy: el camino que usan los clientes esta cortado"
probar 500 rojo  "cualquier otro codigo sin credencial significa que la puerta no esta donde se cree"
probar ""  dudoso "no se pudo sondear: eso es «no se», y no puede leerse como «esta bien»"

# ── EL CONTROL QUE IMPIDE QUE TODO ROJO PASE POR VERDE ──────────────────────────────────────
# Si la funcion devolviera `rojo` siempre, cuatro de las seis filas pasarian. Se exige que los tres
# colores aparezcan: sin eso, un despacho degenerado se lee como cobertura.
VISTOS=""
for c in 401 403 ""; do
  VEREDICTO=""; veredicto_sonda_mcp "$c" "f" "hay-proxy"; VISTOS="$VISTOS $VEREDICTO"
done
for color in verde rojo dudoso; do
  if [[ " $VISTOS " == *" $color "* ]]; then ok "el despacho sabe decir «$color»"
  else mal "el despacho NUNCA dice «$color»: esta degenerado y las filas de arriba no miden nada"; fi
done

# Y que el 403 nombre su causa: un rojo que no dice QUE mirar manda a diagnosticar la red.
VEREDICTO=""; MENSAJE=""; veredicto_sonda_mcp 403 "f" "hay-proxy"
if [[ "$MENSAJE" == *"rebinding"* && "$MENSAJE" == *"Host"* ]]; then
  ok "el 403 nombra la causa (rebinding + Host) en vez de mandar a mirar la red"
else
  mal "el mensaje del 403 no nombra ni «rebinding» ni «Host»: quien lo lea va a diagnosticar la red, que es donde NO esta el problema"
fi

# Y QUE NOMBRE EL ARREGLO, que es la parte que se perdia sola. La primera version escribia
# `puertaDePersona` entre BACKTICKS adentro de comillas dobles, asi que bash lo ejecutaba como
# comando: la sustitucion daba vacio y el mensaje llegaba SIN la palabra que dice que hacer,
# ensuciando stderr en cada 403. Lo revelo correr la prueba, no leerla.
if [[ "$MENSAJE" == *"puertaDePersona"* ]]; then
  ok "el 403 nombra el arreglo (puertaDePersona)"
else
  mal "el mensaje del 403 no nombra el arreglo: si el nombre se lo comio una sustitucion de shell, el rojo dice QUE pasa y no QUE hacer"
fi

echo
if (( FALLOS == 0 )); then printf '\033[32mTODO OK\033[0m\n'; exit 0
else printf '\033[31m%d FALLO(S)\033[0m\n' "$FALLOS"; exit 1; fi
