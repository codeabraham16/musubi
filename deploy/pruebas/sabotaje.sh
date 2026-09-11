#!/usr/bin/env bash
#
# sabotaje.sh — corre UN sabotaje contra UNA guarda y dice si de verdad la puso en rojo.
#
# ════════════════════════════════════════════════════════════════════════════════════════════
# POR QUÉ EXISTE: EL 2026-09-05 DECLARÉ SEIS SABOTAJES QUE NO FUNCIONABAN
#
# Este repo pide que toda guarda tenga un sabotaje que la ponga en rojo, y que el sabotaje COMPILE.
# La disciplina es buena y aplicarla a mano falla de OCHO formas distintas, todas medidas en este
# repo — las seis primeras ese día, las dos últimas el 2026-09-05 a la tarde:
#
#   1. LA GUARDA NO MIRA EL CAMINO QUE EL SABOTAJE TOCA. Deshacer el arreglo térmico entero dejaba
#      la prueba en verde, porque ejercitaba la función directo y nunca a su llamador.
#   2. LA GUARDA SE SATISFACE CON UN TEXTO. Un regex de `ConsentimientoEfectivo\(` lo contentaba un
#      COMENTARIO, así que borrar el `switch` entero no rompía nada. (Pasó tres veces en el día:
#      comentario, comentario, y un string de un mensaje de error.)
#   3. LA GUARDA MIRA UN VECINO. Buscar `on(project, device)` en todo el texto anterior a la métrica
#      lo satisfacía el `on(...)` de OTRA cláusula.
#   4. LA GUARDA DEJA DE MIRAR. Una línea en blanco le daba la tabla por terminada, y todo lo que
#      venía después quedaba sin revisar — en verde.
#   5. EL ROJO ERA UN BUILD ROTO. Con el paquete sin compilar, `go test` sale distinto de cero pase
#      lo que pase: «el sabotaje funcionó» y «el árbol no compila» se leen igual.
#   6. DOS SABOTAJES DISTINTOS FALLABAN CON EL MISMO MENSAJE, y se contaron como dos. Eran uno
#      mal medido, y lo único que lo mostró fue correr el control SIN sabotaje.
#   7. LA GUARDA VE AL REVÉS: premia el defecto y castiga el arreglo. Un sabotaje NO puede
#      detectarlo —contesta bien— y sólo lo muestra la pregunta simétrica: «¿sigue en VERDE cuando
#      hago lo correcto?». Es lo que mide el quinto argumento (sección 4, abajo).
#   8. LA GUARDA ES NEGATIVA Y EL SABOTAJE FUE EN LA DIRECCIÓN QUE NO EXISTE. Una aserción que dice
#      «esto NO puede aparecer» no se sabotea SACANDO lo prohibido: eso es un no-op, y su verde no
#      dice «la guarda no cubre», dice «no la toqué». Se sabotea AGREGÁNDOLO. Medido en
#      `TestElScriptDeCambioDeAgenteNoDependeDeQuienLoEjecuta`, que prohíbe `taskkill /IM
#      musubi.exe` en una línea ejecutable: neutralizar el patrón la dejaba en verde; agregar la
#      línea prohibida la puso en rojo en el acto. Le pasó también al agente que audita: dos de sus
#      veredictos «VERDE» del día eran esto y no un hueco.
#
# Las ocho tienen la misma forma que el resto de los defectos de este repo: una señal con la forma
# de la respuesta, contestando otra pregunta. Un verde nuevo no distingue «no hay defecto» de «no
# estoy mirando», y un rojo nuevo no distingue «la guarda funcionó» de «rompí el build».
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# LO QUE ESTE GUION EXIGE, Y POR QUÉ CADA COSA
#
#   · CONTROL SIN SABOTAJE: la prueba tiene que PASAR antes. Sin esto, una guarda ya rota se
#     confirma a sí misma — el rojo del sabotaje sería el rojo que ya había (falla 6).
#   · COMPILA ANTES Y DESPUÉS: si el sabotaje rompe la compilación, el rojo no dice nada (falla 5).
#   · EL NOMBRE DEL TEST EN EL FALLO: `--- FAIL: <nombre>` y no un exit code (falla 5).
#   · LA PRIMERA LÍNEA DEL FALLO, IMPRESA: para que dos sabotajes que fallan por el mismo motivo se
#     vean distintos de dos que fallan por motivos distintos (falla 6).
#   · RESTAURA SIEMPRE, con `trap`: un sabotaje que queda puesto es peor que no haberlo corrido.
#
# Uso:
#   ./deploy/pruebas/sabotaje.sh <paquete> <patrón-de-test> <archivo-a-sabotear> <comando-que-sabotea>
#
# El comando que sabotea recibe la ruta del archivo en $1 y lo edita en el lugar. Ej:
#   ./deploy/pruebas/sabotaje.sh ./internal/fleet 'TestElegirTemperatura' internal/fleet/procparse.go \
#     'sed -i "s/TempMinPlausibleC = 5/TempMinPlausibleC = 0/" "$1"'
set -uo pipefail

PKG="${1:?falta el paquete, ej ./internal/fleet}"
PATRON="${2:?falta el patrón del test}"
ARCHIVO="${3:?falta el archivo a sabotear}"
SABOTAJE="${4:?falta el comando que aplica el sabotaje}"
ARREGLO="${5:-}"   # opcional: un cambio CORRECTO que la guarda tiene que seguir aceptando (falla 7)

[[ -r "$ARCHIVO" ]] || { echo "✗ no puedo leer $ARCHIVO"; exit 2; }

RESPALDO="$(mktemp)"
cp -- "$ARCHIVO" "$RESPALDO"
# EL MODO SE GUARDA APARTE, PORQUE `cp` SOBRE UN ARCHIVO QUE EXISTE NO LO TOCA.
#
# Medido el 2026-09-05 y costó una corrida de gio: un sabotaje que reescribe el archivo con
# `grep -v ... > tmp && mv tmp archivo` no lo edita, lo REEMPLAZA, y el archivo nuevo trae el modo
# del umask (644). El `cp` de restaurar devuelve el CONTENIDO —y por eso `git status` quedó
# limpio en contenido— pero conserva los permisos del destino, o sea los del sabotaje. El guion
# quedó sin bit de ejecución y la siguiente corrida murió con `Permission denied`, que manda a
# mirar cualquier cosa menos acá.
#
# Es la forma de siempre una vuelta más adentro: la herramienta que existe para no dejar el
# sabotaje puesto lo dejaba puesto en la dimensión que no estaba mirando.
MODO="$(stat -c %a -- "$ARCHIVO")"
# EL RESTORE VA EN UN TRAP Y NO AL FINAL: si algo de acá falla —o alguien corta con Ctrl-C— el
# sabotaje queda puesto en el árbol, y ése es el peor desenlace posible de esta herramienta.
restaurar() { cp -- "$RESPALDO" "$ARCHIVO"; chmod -- "$MODO" "$ARCHIVO"; rm -f -- "$RESPALDO"; }
trap restaurar EXIT INT TERM

# ── EL ENTORNO NO VIAJA ────────────────────────────────────────────────────────────────────────
#
# Este guion corre en la máquina del operador, y ahí `MUSUBI_TOKEN` y `MUSUBI_CENTRAL_URL` están
# puestas: son el estado normal de esta terminal. Una prueba que las lea como «autorización» sale
# a pegarle al cerebro de PRODUCCIÓN con la credencial viva, y el corredor de sabotajes —que llama
# a `go test` con un patrón ANCHO, a propósito— es justo quien más fácil la selecciona sin querer.
# Pasó: 209 llamadas al cerebro de producción desde una corrida de rutina.
#
# El arreglo de fondo va en la prueba (que ahora exige su propio permiso explícito). Esto es el
# cinturón: el corredor no se lo pasa. Se BORRAN y no se vacían — una variable vacía y una
# ausente son lo mismo para `os.Getenv`, pero no para todo el mundo, y acá lo que se quiere es
# que no exista.
sinCredenciales() {
  env -u MUSUBI_TOKEN -u MUSUBI_CENTRAL_URL -u MUSUBI_SONDA_CONTRA_PRODUCCION \
      -u ALTURA_MUSUBI_TOKEN -u CRM_MUSUBI_TOKEN -u MB_DESIGN_TOKEN \
      -u MUSUBI_TOKEN_FILE -u ALTURA_MUSUBI_TOKEN_FILE "$@"
}

corrida() { sinCredenciales go test "$PKG" -run "$PATRON" -count=1 2>&1; }
# EL CONTROL VA CON -v A PROPÓSITO: sin él `go test` no imprime los `--- PASS`, así que no se puede
# contar CUÁNTAS pruebas pasaron — y «pasó» sobre cero pruebas es el falso verde de A100.
controlVerboso() { sinCredenciales go test "$PKG" -run "$PATRON" -count=1 -v 2>&1; }
compila() { go vet "$PKG" >/dev/null 2>&1; }

echo "▶ $PATRON  en  $PKG"
echo "  archivo: $ARCHIVO"

# ── 1 · CONTROL: sin sabotaje, compila y PASA ──────────────────────────────────────────────────
if ! compila; then
  echo "✗ el árbol NO COMPILA sin ningún sabotaje. Nada de lo que siga significa algo."
  go vet "$PKG" 2>&1 | head -5 | sed 's/^/    /'
  exit 1
fi
BASE="$(controlVerboso)"
if ! grep -q '^ok\|^--- PASS\|^PASS' <<<"$BASE"; then
  echo "✗ CONTROL EN ROJO: la prueba ya falla SIN el sabotaje, así que su rojo no prueba nada."
  grep -m3 -E '^\s*---? FAIL|^\s+\S+_test\.go:' <<<"$BASE" | sed 's/^/    /'
  exit 1
fi
if grep -q 'no tests to run\|no test files' <<<"$BASE"; then
  echo "✗ EL PATRÓN NO SELECCIONA NINGUNA PRUEBA: «$PATRON» no existe, o su archivo no compila en"
  echo "  esta plataforma (¿el nombre termina en _windows_test.go?). Un verde de cero pruebas es"
  echo "  indistinguible de un verde de mil."
  exit 1
fi
N="$(grep -c '^\s*--- PASS' <<<"$BASE")"
# EL NÚMERO SE MIRA, Y ANTES NO SE MIRABA. Se contaba, se imprimía, y se seguía igual con CERO —
# que es el falso verde que esta herramienta entera existe para cazar, adentro de la herramienta.
#
# Con cero, `grep '^ok'` igual matchea: un `--- SKIP` sale con `ok` y código 0. Así que un control
# que se saltea entero pasaba por «compila y pasa», y después el sabotaje —que también se saltea—
# se reportaba como «EL SABOTAJE NO LA PONE EN ROJO». O sea que la herramienta acusaba a la guarda
# de estar hueca cuando la verdad era que NADIE MIDIÓ NADA. Un diagnóstico equivocado manda a
# reescribir una guarda que estaba bien.
if [[ "${N:-0}" -eq 0 ]]; then
  echo "✗ NO SE MIDIÓ NADA: el control salió en verde sin ejecutar UNA SOLA prueba."
  echo "  Casi siempre es un \`t.Skip\`: un test salteado imprime \`--- SKIP\` y termina con \`ok\`,"
  echo "  así que el código de salida no lo distingue de haber corrido mil. Lo que venga después"
  echo "  no significaría nada — y el rojo ausente se leería como una guarda hueca."
  grep -m5 -E '^\s*--- SKIP|^\s+\S+_test\.go:[0-9]+: ' <<<"$BASE" | sed 's/^/    /'
  exit 1
fi
echo "  ✓ control: compila y pasa ($N subtest/s REALMENTE ejecutados)"

# ── 2 · SABOTAJE ───────────────────────────────────────────────────────────────────────────────
if ! bash -c "$SABOTAJE" -- "$ARCHIVO"; then
  echo "✗ el comando de sabotaje falló: no se aplicó nada."
  exit 1
fi
if cmp -s -- "$ARCHIVO" "$RESPALDO"; then
  echo "✗ EL SABOTAJE NO CAMBIÓ EL ARCHIVO. Casi siempre es un patrón que ya no matchea —el código"
  echo "  se movió— y el rojo que viniera después sería de otra cosa."
  exit 1
fi

if ! compila; then
  # LOS BACKTICKS VAN ESCAPADOS Y NO ES UN DETALLE: adentro de comillas dobles, bash los EJECUTA.
  # Sin esto el mensaje salía con la salida de `go test` incrustada en el medio —«no Go files in
  # ...»— justo en el momento en que hay que leerlo con atención. Medido escribiendo este guion.
  echo "✗ EL SABOTAJE ROMPE LA COMPILACIÓN, así que no prueba nada: con el paquete sin compilar,"
  echo "  \`go test\` sale distinto de cero pase lo que pase. Este repo pide que el sabotaje COMPILE,"
  echo "  justamente para que su rojo signifique algo."
  go vet "$PKG" 2>&1 | head -5 | sed 's/^/    /'
  exit 1
fi
echo "  ✓ el sabotaje se aplicó y COMPILA"

# ── 3 · ¿PUSO EN ROJO A ESTA PRUEBA, Y POR QUÉ? ────────────────────────────────────────────────
CON="$(corrida)"
FALLOS="$(grep -E '^\s*--- FAIL: ' <<<"$CON" | sed -E 's/^\s*--- FAIL: ([^ ]+).*/\1/')"
if [[ -z "$FALLOS" ]]; then
  echo "✗ EL SABOTAJE NO LA PONE EN ROJO."
  echo "  Es el desenlace más silencioso de todos, porque el verde se lee como «no hay defecto»."
  echo
  echo "  ANTES DE CULPAR A LA GUARDA, DESCARTÁ QUE EL SABOTAJE SEA EL EQUIVOCADO:"
  echo "   · ¿LA GUARDA ES NEGATIVA? (falla 8) Si la aserción dice «esto NO puede aparecer», sacar"
  echo "     lo prohibido es un NO-OP y este verde no significa nada. Hay que AGREGARLO."
  echo "   · ¿LE APUNTASTE A LA PRUEBA HERMANA? (falla 1) La que ejercita el MÉTODO queda en verde"
  echo "     cuando el sabotaje toca a su LLAMADOR. Buscá quién más se pone rojo con este cambio:"
  echo "         grep -rn '<el símbolo sabateado>' --include='*_test.go' ."
  echo
  echo "  Si el sabotaje era el correcto: arreglá la guarda, o dejá ESCRITO adentro que este"
  echo "  sabotaje NO funciona y por qué — un doc que nombra un sabotaje inerte enseña a confiar"
  echo "  en una red que no está."
  exit 1
fi
echo "  ✓ ROJO, y falla en:"
printf '      %s\n' $FALLOS
# LA PRIMERA LÍNEA DEL FALLO, que es lo que distingue dos sabotajes de uno mal medido. Dos
# sabotajes distintos que fallan con el MISMO mensaje son un sabotaje solo, contado dos veces.
echo "  ── motivo (la primera línea de cada fallo, para poder compararlos) ──"
awk '/^\s*--- FAIL: /{t=$3; got=0; next}
     t!="" && got==0 && /_test\.go:[0-9]+:/{sub(/^[ \t]+/,""); print "      " t " · " substr($0,1,150); got=1}' <<<"$CON"
echo
echo "  ✓ el sabotaje funciona"

# ── 4 · LA OTRA DIRECCIÓN: ¿LA GUARDA CASTIGA EL ARREGLO? (falla 7) ────────────────────────────
if [[ -z "$ARREGLO" ]]; then
  echo
  echo "✓ listo. Se restaura el archivo."
  echo "! NO SE PROBÓ LA OTRA DIRECCIÓN: sin un quinto argumento no se sabe si esta guarda castiga"
  echo "  un cambio CORRECTO. Una guarda invertida contesta bien al sabotaje y manda a sacar la línea"
  echo "  buena — ver la falla 7 en la cabecera. Vale pasarle uno cuando el valor tenga formas"
  echo "  legítimas alternativas (comillas simples en YAML, un nombre con caracteres raros, un alias)."
  exit 0
fi

cp -- "$RESPALDO" "$ARCHIVO"; chmod -- "$MODO" "$ARCHIVO"
if ! bash -c "$ARREGLO" -- "$ARCHIVO"; then
  echo "✗ el comando del ARREGLO falló: no se aplicó nada."
  exit 1
fi
if cmp -s -- "$ARCHIVO" "$RESPALDO"; then
  echo "✗ EL ARREGLO NO CAMBIÓ EL ARCHIVO: sin cambio no se prueba la otra dirección."
  exit 1
fi
if ! compila; then
  echo "✗ el ARREGLO no compila: tiene que ser un cambio LEGÍTIMO, no otro sabotaje."
  go vet "$PKG" 2>&1 | head -5 | sed 's/^/    /'
  exit 1
fi
ARR="$(corrida)"
if grep -qE '^[[:space:]]*--- FAIL: ' <<<"$ARR"; then
  echo "✗ LA GUARDA CASTIGA EL ARREGLO (falla 7): se pone en ROJO con un cambio correcto."
  echo "  Es PEOR que no mirar: premia el defecto y manda a sacar la línea buena. Típicamente la"
  echo "  guarda describe el valor de más cerca de lo que el formato permite — una clase de"
  echo "  caracteres, o unas comillas concretas donde el formato acepta varias formas."
  grep -E '^[[:space:]]*--- FAIL: |_test\.go:[0-9]+:' <<<"$ARR" | head -6 | sed 's/^/      /'
  exit 1
fi
echo "  ✓ y NO castiga el arreglo: con el cambio correcto sigue en verde"
echo
echo "✓ las dos direcciones. Se restaura el archivo."
