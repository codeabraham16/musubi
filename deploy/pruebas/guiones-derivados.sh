#!/usr/bin/env bash
#
# guiones-derivados.sh — ejercita `sha_alla` de `verificar-despliegue.sh` (A111).
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# POR QUÉ ES DE COMPORTAMIENTO Y NO UN grep
#
# `sha_alla` tiene que distinguir CUATRO respuestas —el sha, AUSENTE, ILEGIBLE y «no pude
# preguntar»— y las dos últimas son las que un verificador remoto confunde con un verde. La de
# «no pude preguntar» es la ausencia de salida: `corre_alla` se traga los errores con `|| true`,
# así que si `sha_alla` devolviera vacío por un error de comillas, el `case` caería en la rama
# `""`, diría «no se pudo preguntar» y la sección entera pasaría de largo sin comparar nada.
#
# Y una guarda de texto no puede ver eso: `sha_alla` menciona las tres cadenas y el `case` las
# lista, así que un grep queda satisfecho por un guion que no funciona. La función se EXTRAE del
# archivo de producción —no se copia acá— y se corre.
#
# La cita del escape que importa: adentro de la función el programa de awk lleva `\$1`, para que
# la comilla doble de bash lo deje pasar literal y lo lea el shell remoto. Un `$1` sin escapar se
# expandiría acá, a la ruta, y awk imprimiría la línea entera en vez del sha — o sea que la
# comparación fallaría SIEMPRE, con un rojo que dice «difiere» sobre archivos idénticos.
#
# Uso:  ./deploy/pruebas/guiones-derivados.sh [ruta/a/verificar-despliegue.sh]
# ════════════════════════════════════════════════════════════════════════════════════════════
set -uo pipefail

VERIF="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/verificar-despliegue.sh}"
[[ -r "$VERIF" ]] || { echo "✗ no puedo leer $VERIF"; exit 2; }

FALLOS=0
ok()    { printf '  \033[32m✔ %s\033[0m\n' "$1"; }
mal()   { printf '  \033[31m✘ %s\033[0m\n' "$1"; FALLOS=$((FALLOS+1)); }

# ── Se EXTRAEN las dos funciones del archivo de producción ─────────────────────────────────────
# Si alguna deja de existir o cambia de nombre, esto se corta acá y no pasa en verde sobre nada:
# una prueba que no encuentra qué ejercitar no es una prueba que pasa.
extraer() {  # extraer <nombre> — el cuerpo de `nombre() { ... }` hasta el `}` en la columna 0
  awk -v f="$1" '$0 ~ "^"f"\\(\\) \\{" {d=1} d {print} d && /^\}/ {exit}' "$VERIF"
}
CUERPO_CORRE="$(extraer corre_alla)"
CUERPO_SHA="$(extraer sha_alla)"
for par in "corre_alla:$CUERPO_CORRE" "sha_alla:$CUERPO_SHA"; do
  nombre="${par%%:*}"
  if [[ -z "${par#*:}" ]]; then
    echo "✗ no encontré la función \`$nombre\` en $VERIF."
    echo "  O se renombró, o cambió de forma, o la sección de guiones derivados de A111 ya no está."
    echo "  En cualquiera de los tres casos esta prueba no estaría ejercitando nada, así que corta acá."
    exit 2
  fi
done
SSH_HOST=""           # modo local: `corre_alla` evalúa acá mismo, que es lo que queremos ejercitar
eval "$CUERPO_CORRE"
eval "$CUERPO_SHA"

TMP="$(mktemp -d)"
trap 'chmod -R u+rwX "$TMP" 2>/dev/null; rm -rf "$TMP"' EXIT

printf '\033[1mguiones derivados — las cuatro respuestas de sha_alla\033[0m\n'

# 1 · UN ARCHIVO QUE ESTÁ: tiene que devolver el sha256 y NADA MÁS.
printf 'contenido de prueba\n' > "$TMP/presente"
ESPERADO="$(sha256sum "$TMP/presente" | awk '{print $1}')"
OBTENIDO="$(sha_alla "$TMP/presente")"
if [[ "$OBTENIDO" == "$ESPERADO" ]]; then
  ok "un archivo presente devuelve su sha256 y sólo eso"
else
  mal "un archivo presente devolvió «$OBTENIDO» y su sha es «$ESPERADO». Si trae la ruta pegada, el \`\$1\` del awk se expandió acá en vez de allá — y entonces TODA comparación diría «difiere» sobre archivos idénticos"
fi

# 2 · UN ARCHIVO QUE NO ESTÁ: AUSENTE, que NO es lo mismo que «no pude preguntar».
OBTENIDO="$(sha_alla "$TMP/no-existe")"
if [[ "$OBTENIDO" == "AUSENTE" ]]; then
  ok "un archivo que no está dice AUSENTE (y no vacío, que se leería como «no pude preguntar»)"
else
  mal "un archivo que no está devolvió «$OBTENIDO» y tenía que decir AUSENTE"
fi

# 3 · UN ARCHIVO SIN PERMISO DE LECTURA: ILEGIBLE. Root lee igual, así que ahí el caso no aplica
#     y se dice en vez de inventar un verde.
printf 'secreto\n' > "$TMP/ilegible"
chmod 000 "$TMP/ilegible"
if [[ "$(id -u)" -eq 0 ]]; then
  printf '  \033[90m· el caso ILEGIBLE no se puede probar como root: root lee igual\033[0m\n'
else
  OBTENIDO="$(sha_alla "$TMP/ilegible")"
  if [[ "$OBTENIDO" == "ILEGIBLE" ]]; then
    ok "un archivo sin permiso de lectura dice ILEGIBLE, distinto de AUSENTE y de vacío"
  else
    mal "un archivo sin permiso de lectura devolvió «$OBTENIDO» y tenía que decir ILEGIBLE"
  fi
fi
chmod u+rw "$TMP/ilegible"

# 4 · LA CUARTA RESPUESTA ES LA AUSENCIA DE RESPUESTA, y es la que importa: con un ssh que no
#     contesta, `corre_alla` devuelve vacío y el `case` tiene que ir por «no pude preguntar».
#     Se simula apuntando SSH_HOST a un host que no existe.
(
  SSH_HOST="host-que-no-existe.invalido"
  eval "$CUERPO_CORRE"; eval "$CUERPO_SHA"
  OBTENIDO="$(sha_alla /etc/hostname)"
  if [[ -z "$OBTENIDO" ]]; then
    printf '  \033[32m✔ %s\033[0m\n' "con un servidor que no contesta la respuesta es VACÍA — el case la manda a «no pude preguntar» (exit 2), no a verde"
    exit 0
  fi
  printf '  \033[31m✘ %s\033[0m\n' "con un servidor que no contesta devolvió «$OBTENIDO» en vez de vacío: el «no vi» se estaría confundiendo con un veredicto"
  exit 1
) || FALLOS=$((FALLOS+1))

# 5 · Y QUE LA TABLA NO ESTÉ VACÍA. Toda la sección cuelga de `GUIONES_DERIVADOS`: si quedara sin
#     filas, el bucle no daría una vuelta y la sección entera pasaría en verde sin comparar nada.
FILAS="$(awk '/^GUIONES_DERIVADOS="/{d=1} d{print} d && /"$/ && !/^GUIONES_DERIVADOS="$/{exit}' "$VERIF" | grep -c '|/')"
if [[ "${FILAS:-0}" -ge 2 ]]; then
  ok "la tabla GUIONES_DERIVADOS declara $FILAS guion(es): el bucle tiene sobre qué correr"
else
  mal "GUIONES_DERIVADOS quedó con $FILAS fila(s) útiles. Con la tabla vacía el bucle no da una vuelta y la sección pasa en VERDE sin comparar nada, que es el defecto que A111 vino a cerrar"
fi

echo
if [[ "$FALLOS" -ne 0 ]]; then
  printf '\033[31m%d comprobación(es) en rojo\033[0m\n' "$FALLOS"
  exit 1
fi
printf '\033[32mlas cuatro respuestas de sha_alla se distinguen, y la tabla tiene filas\033[0m\n'
