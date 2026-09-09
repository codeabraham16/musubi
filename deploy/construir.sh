#!/usr/bin/env bash
#
# construir.sh — arma el binario del cerebro con una versión DERIVADA, nunca tipeada.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# POR QUÉ EXISTE: DOS SESIONES NUMERANDO DISTINTO EL MISMO CÓDIGO
#
# El 2026-08-30 el redespliegue imprimió `de: 0.115.0-reparto.cc2ae9c` → `a: 0.133.0-rename...`,
# y eso se lee como un salto de dieciocho versiones. No lo era: era el MISMO tronco. Una sesión
# leía el archivo VERSION (0.115.0) y la otra —la mía— venía inventando números a mano, seis
# despliegues seguidos.
#
# ESO NO ES COSMÉTICO. El `de:`/`a:` del script de redespliegue es el ÚNICO registro de qué
# estuvo corriendo, y hoy fue exactamente lo que permitió descubrir que dos sesiones se estaban
# pisando los binarios. Con números que no son comparables entre sí, ese aviso pasa a mentir: un
# rollback real se ve igual que dos esquemas de numeración conviviendo.
#
# La versión se DERIVA de dos cosas que no se pueden discutir: el archivo VERSION y el commit.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# EL SUFIJO `-sucio` NO ES DECORACIÓN
#
# Un binario construido con cambios sin commitear no se puede reconstruir después: el commit que
# anuncia NO es el código que corre. Hoy la etiqueta ajena se pudo auditar justamente porque su
# commit existía y se podía mirar. Un build sucio rompe eso, así que lo dice en su propio nombre —
# no se prohíbe (a veces hace falta probar algo rápido), se DECLARA.
#
# Uso:   ./deploy/construir.sh [etiqueta-del-track] [ruta-de-salida]
# Ej:    ./deploy/construir.sh rename /tmp/musubi-rename
set -euo pipefail

cd "$(dirname "$0")/.."

[[ -f VERSION ]] || { echo "no encuentro el archivo VERSION en la raíz del repo" >&2; exit 1; }
BASE="$(tr -d '[:space:]' < VERSION)"
COMMIT="$(git rev-parse --short HEAD)"
# EL CHEQUEO DE SUCIO MIRA `git status --porcelain`, Y NO `git diff` ────────────────────────────
#
# `git diff` NO VE LOS ARCHIVOS SIN TRACKEAR, y `go build` compila todo el `.go` del directorio,
# rastreado o no. Así que el caso PEOR —código que se despliega y no está en ningún commit, o sea
# el único que no se puede auditar después— era exactamente el que este guion daba por limpio.
#
# Medido el 2026-09-05 en un clon aislado, agregando `cmd/musubi/zz_chivato.go` sin trackear:
#   · `git status --short`            →  ?? cmd/musubi/zz_chivato.go
#   · el chequeo viejo (`git diff`)   →  LIMPIO
#   · versión estampada              →  0.131.0-flota.ac75fec, SIN `-sucio`
#   · sha256                         →  CAMBIÓ: el archivo sin trackear viajó adentro del binario
#   · `vcs.modified` que Go embute   →  true. Lo cazaba, y no lo mirábamos.
#
# O sea: la cabecera de arriba dice «un build sucio no se prohíbe, se DECLARA», y no lo declaraba.
# `--porcelain` es además la MISMA semántica que usa Go para su `vcs.modified`, así que las dos
# señales del binario dejan de poder contradecirse — y abajo se comprueba que no lo hagan.
#
# Los ignorados de `.gitignore` no cuentan (`--porcelain` los omite), así que el `./musubi` de una
# corrida anterior no ensucia nada.
SUCIO=""
ESTADO="$(git status --porcelain --untracked-files=normal 2>/dev/null)"
[[ -z "$ESTADO" ]] || SUCIO="-sucio"

ETIQUETA="${1:-}"
SALIDA="${2:-./musubi}"
VERSION="${BASE}${ETIQUETA:+-$ETIQUETA}.${COMMIT}${SUCIO}"

echo "▶ versión: $VERSION"
# SE DICE QUÉ lo ensucia, y no sólo que está sucio: un sufijo sin la lista manda a adivinar, y con
# los sin trackear adentro del chequeo la causa más probable es un archivo que alguien olvidó.
if [[ -n "$SUCIO" ]]; then
  echo "! el árbol tiene cambios SIN COMMITEAR: este binario no se va a poder reconstruir" >&2
  echo "$ESTADO" | head -8 | sed 's/^/    /' >&2
  [[ "$(wc -l <<<"$ESTADO")" -gt 8 ]] && echo "    … y más ($(wc -l <<<"$ESTADO") en total)" >&2
fi

# LA CLAVE PÚBLICA DE RELEASE SE INYECTA ACÁ, y su ausencia es deliberada por default: un binario
# de desarrollo NO puede verificar la procedencia de una actualización, y `musubi update` se niega
# a instalar sin poder verificarla. Sólo el build que se publica la lleva.
#
#   MUSUBI_RELEASE_PUBKEY=<64 hex> ./deploy/construir.sh flota /tmp/musubi
#
# El par se genera UNA vez con `go run ./deploy/cmd/clave-release`, y la privada vive fuera de
# línea: si estuviera en el CI, quien comprometa el CI firma lo que quiera y la firma no compra
# nada — sería un sha256 más caro.
# Las `-ldflags` se arman adentro de compilar(), más abajo, para que el relink que corrige el
# sufijo use la MISMA receta. Acá sólo se informa, porque es lo que el operador tiene que saber
# antes de que salga el binario.
if [[ -n "${MUSUBI_RELEASE_PUBKEY:-}" ]]; then
  echo "▶ con clave pública de release embebida"
else
  echo "! sin clave pública de release: este binario NO va a poder auto-actualizarse (a propósito)" >&2
fi

# `-trimpath` NO ES COSMÉTICO: SIN ÉL, EL MISMO COMMIT DA BINARIOS DISTINTOS
#
# Go empotra la ruta de compilación en el binario, así que dos compilaciones del MISMO commit
# hechas en carpetas distintas salen con sha256 distinto. Medido el 2026-09-04, dos worktrees del
# commit 8bb1e98:
#
#   sin -trimpath   996ffe97f534143f…   y   ca660b1675d746fd…     ← distintos
#   con -trimpath   a2996572df2fc2aa…   y   a2996572df2fc2aa…     ← idénticos
#
# Lo que se pierde sin esto es la pregunta que uno quiere hacerle a un sha: «¿este binario es el
# commit que dice ser?». Sin reproducibilidad el sha sólo contesta «¿llegó entero el archivo que
# serví?» —integridad de transporte, útil pero mucho menos—, y dos personas que compilan el mismo
# commit no pueden compararse entre sí. Importa más todavía donde la autorización es POR HASH: A31
# describe reautorizar hash por hash en AppLocker/WDAC, y un hash que cambia por la carpeta donde
# se compiló hace ese trámite imposible de automatizar.
#
# El costo es cero: `-trimpath` sólo quita rutas absolutas del binario.
#
# PERO `-trimpath` NO ALCANZA PARA UN GIT WORKTREE, y eso conviene saberlo antes de acusar a nadie
# de adulterar un binario. Medido el 2026-09-05 con el MISMO commit (`ac75fec`) en dos lugares:
#
#   checkout normal   fd4bedb202ad7ffa…        clon fresco   fd4bedb202ad7ffa…   ← idénticos
#   git worktree      81d5e74a2ce0c6ef…                                          ← distinto
#
# La causa la dice `go version -m`: en un worktree despegado el módulo es `(devel)` y NO aparece
# ninguno de los sellos `vcs=git`, `vcs.revision`, `vcs.time`, `vcs.modified`; en un checkout de
# verdad están los cuatro. Van adentro del binario, así que el sha cambia. La regla precisa es:
# **el sha reproduce desde un clon o checkout completo en ese commit, y NO desde un worktree** — y
# un verificador honesto que use worktree ve un mismatch que se lee como binario adulterado.

# compilar() existe para poder RELINKEAR con otra versión sin repetir la receta. Cuesta ~3 s (la
# compilación está en caché; lo que se paga es el link) y sólo se paga cuando hace falta.
# TAGS es lo que hace que el binario ENTIENDA algo más que Go, y faltaba acá.
#
# ─────────────────────────────────────────────────────────────────────────────────────────────
# EL BINARIO QUE SE CONSTRUÍA ACÁ NO ERA EL QUE SE PUBLICA
#
# `.github/workflows/release.yml` compila con estos tags desde siempre; este guion no los ponía.
# O sea que el binario del release entiende TS/JS/TSX/Python y el que sale de acá —el que se usa
# para desarrollar y el que va al cerebro por `redesplegar-cerebro.sh`— sólo entiende Go.
#
# Y la diferencia era INVISIBLE: los dos imprimen la misma versión. Medido sobre el binario del
# árbol: `strings musubi | grep -c gotreesitter` daba 0. Cuando el grafo de un repo de TS salía
# vacío, no había forma de distinguir «este repo no tiene código» de «este binario no lo entiende».
# Por eso, junto con este cambio, `musubi version --lenguajes` responde esa pregunta.
#
# La lista está DOS VECES —acá y en release.yml— porque son dos sistemas distintos (bash y un
# workflow de Actions) y no hay un lugar que los dos lean. Que estén duplicadas es exactamente la
# forma de defecto que causó el resto de los arreglos de hoy, así que no queda librada a que
# alguien se acuerde: `TestLosTagsDeConstruirIgualanAlosDelRelease` cruza las dos listas y falla
# si divergen.
TAGS='treesitter grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_javascript grammar_subset_python'

compilar() {
  local ld="-X main.version=$1"
  [[ -n "${MUSUBI_RELEASE_PUBKEY:-}" ]] && ld="$ld -X main.clavePublicaDeReleaseHex=$MUSUBI_RELEASE_PUBKEY"
  go build -trimpath -tags "$TAGS" -ldflags "$ld" -o "$SALIDA" ./cmd/musubi
}
compilar "$VERSION"

# ─────────────────────────────────────────────────────────────────────────────────────────────
# EL GUION COMPRUEBA SU PROPIA AFIRMACIÓN CONTRA UNA SEÑAL QUE NO ES SUYA
#
# Go embute `vcs.modified` en el binario, con la misma semántica que `git status --porcelain`. O
# sea que hay DOS respuestas a «¿estaba limpio el árbol?» y hasta hoy no se comparaban: el sufijo
# de la versión lo decidía este guion y el sello lo pone Go. Cuando el chequeo de acá miraba
# `git diff`, las dos se contradecían en silencio justo en el caso peor (un `.go` sin trackear).
#
# Comparar cuesta una línea y convierte «confiá en mi sufijo» en «dos fuentes independientes
# coinciden». Y cuando NO coinciden, lo que hay es un defecto de este guion, no del árbol — así
# que se dice fuerte y con qué hacer.
#
# `go version -m` LEE el archivo, no lo ejecuta, así que esto también vale para una compilación
# cruzada.
SELLO="$(go version -m "$SALIDA" 2>/dev/null | sed -n 's/.*vcs\.modified=//p' | head -1)"
case "$SELLO" in
  "")
    # LA AUSENCIA NO ES UN «NO», y confundirlas es la falla 5 de deploy/pruebas/sabotaje.sh con
    # otro traje: un verificador que pregunte «¿modified es false?» leería el vacío como limpio.
    echo "! SIN SELLOS DE VCS en el binario: se compiló fuera de un checkout de git —típicamente un"
    echo "  git worktree—, así que Go no dice si el árbol estaba limpio: no dice NADA. El sufijo de"
    echo "  la versión quedó decidido por el chequeo de este guion, sin segunda opinión."
    echo "  Y el sha de abajo NO va a coincidir con el de un clon en el mismo commit. Para un sha"
    echo "  que otro pueda verificar, compilá desde un clon o el checkout principal." ;;
  true|false)
    # EL SUFIJO SE CORRIGE CONTRA EL SELLO, no se discute con él.
    #
    # La versión se inyecta por `-ldflags` ANTES de compilar, así que el sufijo arranca siendo una
    # CONJETURA de este guion sobre el árbol. El sello lo pone Go al compilar y es lo que de verdad
    # viajó adentro. Dos fuentes de verdad sobre la misma pregunta es cómo se podre una herramienta
    # —el 2026-09-05 se midió justo eso: `git diff` no ve los sin trackear, y el binario salía sin
    # `-sucio` con un `.go` que no estaba en ningún commit—, así que acá no hay dos: hay una lectura
    # y, si la conjetura no coincidía, un relink que la deja coincidiendo.
    #
    # Con esto el sufijo deja de ser una afirmación y pasa a ser una lectura, y el sabotaje se
    # escribe sin que el guion tenga que saber qué es «sin trackear»: agregás un `.go` que no está
    # commiteado y el binario TIENE que salir marcado.
    ESPERADO="$SUCIO"
    [[ "$SELLO" == "true" ]] && ESPERADO="-sucio" || ESPERADO=""
    if [[ "$ESPERADO" != "$SUCIO" ]]; then
      echo "! el chequeo de este guion dijo \"${SUCIO:-limpio}\" y el sello de Go dice"
      echo "  vcs.modified=$SELLO. MANDA EL SELLO: es lo que viajó adentro del binario. Se relinkea."
      echo "  (esto es un defecto del chequeo de arriba, no del árbol: vale mirarlo)"
      SUCIO="$ESPERADO"
      VERSION="${BASE}${ETIQUETA:+-$ETIQUETA}.${COMMIT}${SUCIO}"
      compilar "$VERSION"
      echo "▶ versión corregida: $VERSION"
    fi ;;
esac

echo "✓ $SALIDA"

# ─────────────────────────────────────────────────────────────────────────────────────────────
# EL BINARIO SE INTERROGA SÓLO SI ESTA MÁQUINA PUEDE EJECUTARLO
#
# `"$SALIDA" version` era incondicional, así que una compilación cruzada
# —`GOOS=windows ./deploy/construir.sh flota musubi-nuevo.exe`, que es como se arma el agente de
# Windows desde acá— moría en «cannot execute binary file» con `set -e` DESPUÉS de haber
# escrito el binario perfectamente. El guion terminaba en rojo sobre un trabajo que salió bien, y
# ni siquiera llegaba a imprimir el sha256, que es lo único que el otro lado necesita para
# comprobar que le llegó lo que se compiló.
#
# Un guion que reporta fracaso cuando tuvo éxito es peor que uno que no reporta nada: enseña a
# ignorar su código de salida, y ése es el que decide si un despliegue sigue.
DESTINO_OS="${GOOS:-$(go env GOHOSTOS)}"
DESTINO_ARCH="${GOARCH:-$(go env GOHOSTARCH)}"
if [[ "$DESTINO_OS" == "$(go env GOHOSTOS)" && "$DESTINO_ARCH" == "$(go env GOHOSTARCH)" ]]; then
  "$SALIDA" version
else
  echo "▶ compilación cruzada para $DESTINO_OS/$DESTINO_ARCH: no se puede correr acá, así que la"
  echo "  versión no se comprueba. Se llama $VERSION y hay que verificarla EN LA MÁQUINA DESTINO."
fi
sha256sum "$SALIDA"

