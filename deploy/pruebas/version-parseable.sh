#!/usr/bin/env bash
#
# version-parseable.sh — comprueba que TODA versión que `construir.sh` pueda emitir la parsee
# `fleet.NucleoDeVersion`, el código que decide si el cerebro puede comparar versiones.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# QUÉ ROMPIÓ ESTO, Y POR QUÉ NO LO CAZÓ LA PRUEBA QUE YA EXISTÍA
#
# `construir.sh` arma `<VERSION>[-<track>].<commit>[-sucio]`. Con el track vacío el guión
# desaparece y salen CUATRO componentes: `0.139.6.7e2d211`. `NucleoDeVersion` corta en el primer
# `-`, parte por `.` y exige tres, así que devuelve ok=false — y `VersionDelAgenteDifiere` le
# pregunta AL CEREBRO PRIMERO y contesta `comparable=false` para TODAS las máquinas cuando la suya
# no parsea. Un argumento omitido apagó `musubi_fleet_device_agent_stale` para la flota entera
# (medido el 2026-09-09: 3 series a las 20:30 UTC, 0 a las 21:00, redespliegue a las 20:39).
#
# `internal/fleet/version_test.go` YA fijaba el caso `{"0.130.0.1", "", false} // cuatro
# componentes`. Está verde y es correcta. Lo que no hacía —y es todo el punto de este arnés— es
# conectar esa forma con que es LA SALIDA DE NUESTRO PROPIO GUIÓN: la trataba como entrada basura
# de afuera. Productor y parser sin nadie en el medio, que es la forma que este repo persigue.
#
# Por eso acá no se agregan más casos basura a mano: se ENUMERA lo que construir.sh puede emitir
# —con track y sin track, árbol limpio y sucio— y se exige que el parser los acepte todos.
#
# ────────────────────────────────────────────────────────────────────────────────────────────
# EL CONTROL, QUE ES LA MITAD QUE HACE QUE EL VERDE SIGNIFIQUE ALGO
#
# Un verificador que dijera «parsea» a todo dejaría este arnés en verde sin medir nada. Así que
# además de las formas buenas se le pasa la forma MALA CONOCIDA (`0.139.6.7e2d211`, la que rompió
# la flota) y se exige que la RECHACE. Si la acepta, el arnés sale en 2: no falló el guión, falló
# la medición, y son cosas distintas.
#
# EN UN CLON Y NO EN UN WORKTREE, por lo mismo que `sufijo-sucio.sh` lo dice en su cabecera: en un
# worktree Go no estampa `vcs` y el sufijo `-sucio` queda decidido sin segunda opinión.
#
# Uso:  ./deploy/pruebas/version-parseable.sh [raíz-del-repo]
set -uo pipefail

RAIZ="$(cd "${1:-$(dirname "${BASH_SOURCE[0]}")/../..}" && pwd)"
[[ -f "$RAIZ/VERSION" && -x "$RAIZ/deploy/construir.sh" ]] || { echo "✗ $RAIZ no parece la raíz del repo"; exit 2; }
command -v go >/dev/null || { echo "✗ sin go en el PATH"; exit 2; }

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
CLON="$TMP/clon"

SHA="$(git -C "$RAIZ" rev-parse HEAD)"
git clone --quiet --local --no-hardlinks "$RAIZ" "$CLON" || { echo "✗ no se pudo clonar"; exit 2; }
git -C "$CLON" checkout --quiet "$SHA" || { echo "✗ no se pudo posicionar en $SHA"; exit 2; }

# El guión QUE SE PRUEBA es el del árbol de trabajo, commiteado adentro del clon para que el árbol
# quede limpio: si quedara sin commitear, el caso «limpio» arrancaría sucio y mediría otra cosa.
cp -- "$RAIZ/deploy/construir.sh" "$CLON/deploy/construir.sh"
git -C "$CLON" -c user.email=arnes@local -c user.name=arnes commit -q -am "el construir.sh a probar" 2>/dev/null
[[ -z "$(git -C "$CLON" status --porcelain)" ]] || { echo "✗ el clon no quedó limpio"; exit 2; }

version_de() { "$1" version 2>/dev/null | awk '{print $2}'; }

# ── 1 · SIN TRACK: el guión tiene que NEGARSE ────────────────────────────────────────────────
# Es la puerta por la que entró el incidente. No alcanza con que la versión salga parseable: el
# track dice de qué salió el binario, y un guión que no puede contestarlo tiene que parar.
( cd "$CLON" && ./deploy/construir.sh ) >"$TMP/log-sin-track" 2>&1
RC=$?
if [[ $RC -eq 0 ]]; then
  echo "✗ construir.sh ACEPTÓ que no le pasaran track y emitió: $(version_de "$CLON/musubi")"
  echo "  Sin track la versión sale con CUATRO componentes y NucleoDeVersion la rechaza, así que"
  echo "  el cerebro deja de poder comparar y agent_stale se apaga para TODA la flota."
  exit 1
fi
echo "  ✓ sin track → el guión se niega (exit $RC), como debe"

# ── 2 · CON TRACK, ÁRBOL LIMPIO ──────────────────────────────────────────────────────────────
( cd "$CLON" && ./deploy/construir.sh arnes "$TMP/limpio" ) >"$TMP/log-limpio" 2>&1 || {
  echo "✗ el build con track falló:"; sed 's/^/    /' "$TMP/log-limpio"; exit 1; }
V_LIMPIO="$(version_de "$TMP/limpio")"
[[ -n "$V_LIMPIO" ]] || { echo "✗ no pude leer la versión del binario limpio"; exit 2; }

# ── 3 · CON TRACK, ÁRBOL SUCIO ───────────────────────────────────────────────────────────────
# La cuarta combinación: `-sucio` se pega AL FINAL, así que hay que comprobar que el parser
# tampoco se atore con ella. Un build sucio se despliega poco, pero cuando se despliega es
# justamente cuando más importa poder comparar.
printf 'package main\n\n// lo pone deploy/pruebas/version-parseable.sh\nvar chivatoDelArnesDeVersion = "presente"\n' \
  > "$CLON/cmd/musubi/zz_chivato_del_arnes_version.go"
( cd "$CLON" && ./deploy/construir.sh arnes "$TMP/sucio" ) >"$TMP/log-sucio" 2>&1 || {
  echo "✗ el build sucio falló:"; sed 's/^/    /' "$TMP/log-sucio"; exit 1; }
V_SUCIO="$(version_de "$TMP/sucio")"
[[ "$V_SUCIO" == *-sucio ]] || { echo "✗ el árbol sucio no salió marcado ($V_SUCIO): lo cubre sufijo-sucio.sh, pero acá la premisa falla"; exit 2; }

# ── 4 · EL PARSER, CONTRA LAS FORMAS BUENAS Y CONTRA LA MALA CONOCIDA ─────────────────────────
# El verificador se escribe DESPUÉS de los builds a propósito: es un archivo sin trackear y
# ensuciaría el árbol de los casos 2 y 3.
mkdir -p "$CLON/cmd/zz_arnes_version"
cat > "$CLON/cmd/zz_arnes_version/main.go" <<'GO'
package main

import (
	"fmt"
	"os"

	"musubi/internal/fleet"
)

func main() {
	for _, v := range os.Args[1:] {
		n, ok := fleet.NucleoDeVersion(v)
		// EL SEPARADOR NO ES TAB, Y NO ES ESTÉTICO: bash COLAPSA las corridas de
		// caracteres IFS que son espacio en blanco —tab incluido, aunque IFS sea sólo
		// tab—, así que con el núcleo VACÍO (que es justo el caso que falla) los campos
		// se corren y el lector termina mirando `ok` en la variable del núcleo. Con `|`
		// no hay colapso y un campo vacío sigue siendo un campo.
		fmt.Printf("%s|%s|%v\n", v, n, ok)
	}
}
GO

# LA FORMA MALA ES UN LITERAL Y NO SE DERIVA: es la que de verdad se desplegó el 2026-09-09. Si un
# día `construir.sh` deja de poder emitirla, este caso sigue valiendo — comprueba que el
# verificador DISCRIMINA, no que el guión la produzca.
readonly MALA="0.139.6.7e2d211"
SALIDA="$( cd "$CLON" && go run ./cmd/zz_arnes_version "$V_LIMPIO" "$V_SUCIO" "$MALA" 2>&1 )" || {
  echo "✗ no pude correr el verificador:"; sed 's/^/    /' <<<"$SALIDA"; exit 2; }

fallo=0
while IFS="|" read -r v nucleo ok; do
  [[ -n "$v" ]] || continue
  if [[ "$v" == "$MALA" ]]; then
    if [[ "$ok" == "true" ]]; then
      echo "✗ EL CONTROL FALLÓ: NucleoDeVersion($v) = ($nucleo, $ok), o sea que ACEPTA la forma que"
      echo "  apagó la flota el 2026-09-09. Este arnés no está discriminando nada y su verde no vale."
      exit 2
    fi
    echo "  ✓ control: la forma mala conocida ($v) sigue rechazada"
    continue
  fi
  if [[ "$ok" != "true" ]]; then
    echo "✗ construir.sh emite «$v» y NucleoDeVersion NO la parsea (núcleo=${nucleo:-<vacío>}, ok=$ok)."
    echo "  El cerebro con esa versión contesta comparable=false para TODAS las máquinas y"
    echo "  musubi_fleet_device_agent_stale deja de emitirse para la flota entera."
    fallo=1
    continue
  fi
  echo "  ✓ $v → núcleo $nucleo (parseable)"
done <<< "$SALIDA"

[[ $fallo -eq 0 ]] || exit 1
echo "✓ las formas que construir.sh puede emitir son TODAS parseables, y la mala sigue rechazada"
