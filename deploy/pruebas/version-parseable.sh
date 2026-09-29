#!/usr/bin/env bash
#
# version-parseable.sh — comprueba que TODA versión que `construir.sh` pueda emitir la parsee
# `fleet.NucleoDeVersion`, el código que decide si el cerebro puede comparar versiones. Y, desde
# A138, que `fleet.BuildDelAgenteDifiere` reconozca como el mismo código al mismo commit construido
# con otra etiqueta y otra huella (pasos 2b y 6).
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
git -C "$CLON" -c user.email=arnes@local -c user.name=arnes commit -q -am "el construir.sh a probar" >/dev/null 2>&1
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

# ── 2b · EL MISMO COMMIT, CON OTRA ETIQUETA Y OTRA HUELLA (A138) ──────────────────────────────
# La versión lleva dos cosas que NO salen del código: la ETIQUETA, que elige quien construye, y el
# LARGO de la huella, que `git rev-parse --short` decide según cuántos objetos tiene el clon. El
# 2026-09-29 el cerebro corría `0.141.0-main.eb2cdb7` y el actualizador de Windows sacó, del MISMO
# commit, `0.141.0-flota.eb2cdb72`. El paso 6 le pasa un par así al comparador de builds de verdad.
#
# EL LARGO SE FUERZA CON `core.abbrev` EN VEZ DE ESPERAR A QUE PASE: adentro de un mismo clon el
# largo automático sale igual en las dos corridas, y sin forzarlo el caso que marcó a la flota no
# se ejercitaría nunca. Va ANTES del paso 3 porque tiene que salir limpio, y el 3 ensucia el árbol.
# Y si el build del paso 2 dejó algo suelto en el clon, éste saldría sucio y el paso 6 acusaría al
# comparador por lo que fue una falla de la medición: se para acá, en 2.
[[ -z "$(git -C "$CLON" status --porcelain)" ]] || {
  echo "✗ el clon quedó sucio después del paso 2: el build con otra etiqueta no saldría limpio"
  git -C "$CLON" status --porcelain | sed 's/^/    /'; exit 2; }
CORTA="$(git -C "$CLON" rev-parse --short HEAD)"
git -C "$CLON" config core.abbrev $(( ${#CORTA} + 1 ))
LARGA="$(git -C "$CLON" rev-parse --short HEAD)"
( cd "$CLON" && ./deploy/construir.sh flota "$TMP/otra" ) >"$TMP/log-otra" 2>&1
RC=$?
git -C "$CLON" config --unset core.abbrev
[[ $RC -eq 0 ]] || { echo "✗ el build con otra etiqueta falló:"; sed 's/^/    /' "$TMP/log-otra"; exit 1; }
V_OTRA="$(version_de "$TMP/otra")"
[[ -n "$V_OTRA" ]] || { echo "✗ no pude leer la versión del binario con otra etiqueta"; exit 2; }
# LAS PREMISAS NO SUPONEN EL FORMATO, a propósito: si lo supusieran, un cambio de formato las
# rompería a ellas antes que al paso 6, y el rojo acusaría a la medición en vez de al guion.
[[ ${#LARGA} -gt ${#CORTA} && "$LARGA" == "$CORTA"* ]] || {
  echo "✗ core.abbrev no alargó la huella ($CORTA → $LARGA): el caso de otra huella no se pudo armar"; exit 2; }
[[ "$V_OTRA" != "$V_LIMPIO" ]] || {
  echo "✗ el build con otra etiqueta salió igual al limpio ($V_OTRA): el paso 6 no mediría nada"; exit 2; }
[[ "$V_LIMPIO" == *"$CORTA"* && "$V_OTRA" == *"$LARGA"* ]] || {
  echo "✗ las versiones no llevan las huellas que pidió git (limpio «$V_LIMPIO» con $CORTA, otra «$V_OTRA» con $LARGA):"
  echo "  el caso de otra huella no se ejercitó"; exit 2; }

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
	// `-build agente cerebro [agente cerebro …]`: el comparador de builds sobre cada par, para el
	// paso 6. Mismo separador que abajo, por lo mismo.
	if len(os.Args) > 1 && os.Args[1] == "-build" {
		pares := os.Args[2:]
		for i := 0; i+1 < len(pares); i += 2 {
			difiere, comparable := fleet.BuildDelAgenteDifiere(pares[i], pares[i+1])
			fmt.Printf("%s|%s|%v|%v\n", pares[i], pares[i+1], difiere, comparable)
		}
		return
	}
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

# ── 5 · LAS DOS IMPLEMENTACIONES DEL MISMO PARSER, CONTRA LA MISMA TABLA ──────────────────────
#
# `deploy/verificar-despliegue.sh` NO PUEDE LLAMAR AL PARSER DE VERDAD: corre sin Go, a veces
# contra un servidor que tampoco lo tiene. Así que lo reimplementa en shell, y eso no se puede
# evitar. Lo que sí se puede evitar es que las dos DIVERJAN sin que nadie se entere.
#
# Divergían en TRES cosas el 2026-09-09, y las tres daban FALSO ROJO —«producción diverge del
# repo»— sobre un binario del release CORRECTO: no validaba que quedaran tres componentes, no
# sacaba el prefijo `v` (la familia `git describe`, enrolada en producción) y cortaba sólo en `-`
# y no en `-` o `+`. Quedó tapado porque ese día el rojo era cierto por otro motivo: la causa
# buena escondida atrás de una verdadera.
#
# La función se EXTRAE del archivo de producción y se corre, como hace `guiones-derivados.sh`. Un
# grep quedaría satisfecho con que el nombre aparezca.
echo "  · comparando las dos implementaciones del parser"
# DEL ÁRBOL DE TRABAJO Y NO DEL CLON, por lo mismo que `construir.sh`: el clon está en HEAD,
# así que extraer de ahí verificaría la versión VIEJA de la función — verde sobre código que
# no es el que se está por commitear.
eval "$(sed -n '/^nucleo_de_version() {/,/^}/p' "$RAIZ/deploy/verificar-despliegue.sh")" 2>/dev/null || {
  echo "✗ no pude extraer nucleo_de_version de verificar-despliegue.sh"; exit 2; }
type nucleo_de_version >/dev/null 2>&1 || { echo "✗ nucleo_de_version no quedó definida tras extraerla"; exit 2; }

# LA TABLA ES LA DE `internal/fleet/version_test.go` más las formas que rompieron algo. Si el Go
# suma un caso y el shell no lo sigue, esto se pone rojo.
TABLA=(
  "0.130.0-flota.38a0a9f"   # la que deriva construir.sh
  "v0.106.0-28-gdf2ec21"    # la vieja de git describe
  "0.130.0"                 # pelada
  "  0.130.0-flota.x  "     # con espacios alrededor
  "0.130.0-sucio"           # árbol sucio
  "0.130.0+build5"          # metadata semver
  "0.130.0.1"               # cuatro componentes
  "dev"                     # sin ldflags
  "0..0"                    # componente vacío
  "0.139.6.7e2d211"         # LA QUE APAGÓ LA FLOTA
)
SAL_GO="$( cd "$CLON" && go run ./cmd/zz_arnes_version "${TABLA[@]}" 2>&1 )" || {
  echo "✗ el verificador Go falló sobre la tabla:"; sed 's/^/    /' <<<"$SAL_GO"; exit 2; }

difieren=0
comparados=0
while IFS="|" read -r v nucleo ok; do
  [[ -n "$v$nucleo$ok" ]] || continue
  if s_n="$(nucleo_de_version "$v")"; then s_ok=true; else s_ok=false; s_n=""; fi
  comparados=$((comparados+1))
  if [[ "$s_ok" != "$ok" || ( "$ok" == "true" && "$s_n" != "$nucleo" ) ]]; then
    echo "✗ los dos parsers NO coinciden sobre «$v»:"
    echo "    fleet.NucleoDeVersion  → (${nucleo:-<vacío>}, $ok)"
    echo "    nucleo_de_version (sh) → (${s_n:-<vacío>}, $s_ok)"
    echo "  verificar-despliegue.sh decide con el de shell, así que una divergencia acá es un"
    echo "  veredicto distinto sobre si producción coincide con el repo."
    difieren=1
  fi
done <<< "$SAL_GO"

# CONTROL: si la tabla no llegó entera, el verde de arriba no dice nada.
if [[ "$comparados" -ne "${#TABLA[@]}" ]]; then
  echo "✗ se compararon $comparados de ${#TABLA[@]} casos: la tabla no llegó entera y este verde no vale"
  exit 2
fi
[[ $difieren -eq 0 ]] || exit 1
echo "  ✓ los dos parsers coinciden en los ${#TABLA[@]} casos de la tabla"

# ── 6 · EL MISMO CÓDIGO CON OTRA ETIQUETA, CONTRA EL COMPARADOR DE BUILDS (A138) ───────────────
#
# `fleet.BuildDelAgenteDifiere` decide si una máquina corre OTRO CÓDIGO que el cerebro. Hasta A138
# comparaba el texto entero, y el par del paso 2b daba «difiere» para siempre: una máquina
# actualizada a exactamente el código del cerebro no se apagaba ni actualizando. Sus pruebas de
# internal/fleet usan cadenas escritas a mano; acá le llega LO QUE EMITE construir.sh, que es lo
# que dice si la forma que el comparador espera sigue siendo la que el guion produce. Es la misma
# costura que el paso 4 cubre para el núcleo: productor y consumidor, sin nadie en el medio.
#
# EL CONTROL ES EL SUCIO: el mismo commit con el árbol sucio TIENE que dar «difiere». Un comparador
# que contestara «igual» a todo pasaría los dos primeros pares y cae en el tercero; uno que
# contestara «distinto» a todo —el de antes de A138— cae en los dos primeros.
#
# EL COMPARADOR SALE DEL CLON, O SEA DEL CÓDIGO COMMITEADO: un cambio sin commitear a
# internal/fleet/version.go NO lo ve este paso. Lo que este paso cubre es el guion, que sí se copia
# del árbol de trabajo; al comparador lo cubren sus pruebas de internal/fleet.
SAL_BUILD="$( cd "$CLON" && go run ./cmd/zz_arnes_version -build \
  "$V_OTRA" "$V_LIMPIO" \
  "$V_LIMPIO" "$V_OTRA" \
  "$V_SUCIO" "$V_LIMPIO" 2>&1 )" || {
  echo "✗ no pude correr el comparador de builds:"; sed 's/^/    /' <<<"$SAL_BUILD"; exit 2; }

# Lo que tiene que contestar cada par, en el MISMO orden en que se le pasaron.
ESPERADO=(
  "$V_OTRA|$V_LIMPIO|false|true"
  "$V_LIMPIO|$V_OTRA|false|true"
  "$V_SUCIO|$V_LIMPIO|true|true"
)
mapfile -t OBTENIDO <<<"$SAL_BUILD"
if [[ ${#OBTENIDO[@]} -ne ${#ESPERADO[@]} ]]; then
  echo "✗ el comparador contestó ${#OBTENIDO[@]} líneas por ${#ESPERADO[@]} pares: este verde no valdría"
  sed 's/^/    /' <<<"$SAL_BUILD"; exit 2
fi
fallo=0
for i in "${!ESPERADO[@]}"; do
  IFS="|" read -r a c difiere comparable <<<"${OBTENIDO[$i]}"
  IFS="|" read -r ea ec edifiere ecomparable <<<"${ESPERADO[$i]}"
  if [[ "$a|$c" != "$ea|$ec" ]]; then
    echo "✗ el comparador contestó por «$a» contra «$c» cuando se le preguntó por «$ea» contra «$ec»"; exit 2
  fi
  [[ "$difiere|$comparable" == "$edifiere|$ecomparable" ]] && continue
  if [[ "$edifiere" == "false" ]]; then
    echo "✗ «$a» y «$c» son el MISMO commit con otra etiqueta y otra huella, y BuildDelAgenteDifiere contesta difiere=$difiere comparable=$comparable."
    echo "  Una máquina actualizada a exactamente el código del cerebro quedaría marcada para siempre"
    echo "  (A138): la huella ya no sale donde el comparador la busca."
  else
    echo "✗ «$a» es un build SUCIO del mismo commit que «$c», y BuildDelAgenteDifiere contesta difiere=$difiere comparable=$comparable."
    echo "  Un binario que no salió del commit que nombra pasaría por ese código."
  fi
  fallo=1
done
[[ $fallo -eq 0 ]] || exit 1
echo "  ✓ el mismo commit con otra etiqueta y otra huella es el mismo código, y el sucio no"
