package fleet

// version.go — comparar la versión de un agente con la del cerebro (A68).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SE COMPARA EL NÚCLEO SEMVER, NO LA CADENA ENTERA, Y ESA ES LA DECISIÓN DEL ARCHIVO
//
// La versión que construye `deploy/construir.sh` es `<VERSION>-<track>.<commit>`: el cerebro
// corre `0.130.0-flota.38a0a9f` y los dos Windows corren `0.130.0-flota.e140e0c`. Son el MISMO
// release, construidos de commits distintos — que es lo normal, porque el binario de Windows se
// cruza a mano y el del cerebro se redespliega varias veces por día.
//
// Comparar la cadena completa marcaría a la flota entera como atrasada en cada redespliegue del
// cerebro, y se quedaría así hasta que alguien cruzara el binario a cada máquina Windows. Una
// alarma que está encendida siempre es una alarma apagada: es exactamente cómo se le enseña a
// alguien a ignorar el canal, y este track ya pagó esa lección dos veces (el umbral de los Tier B
// en I2, y el `reach_up` ausente de A67).
//
// LO QUE ESTO NO PUEDE VER, DICHO ACÁ Y NO DESCUBIERTO DESPUÉS: si una capacidad entra al cerebro
// SIN tocar el archivo VERSION, un agente que no la tiene se ve al día. La comparación mide lo que
// VERSION declara, y VERSION lo bumpea una persona. El caso que abrió A68 sí cae adentro —los
// agentes estaban en 0.106.0 contra un cerebro en 0.130.0— pero un agente atrasado dentro del
// mismo release no.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"strings"
	"unicode/utf8"
)

// NucleoDeVersion extrae el MAJOR.MINOR.PATCH de una versión de Musubi.
//
// Tolera las DOS familias que existen en producción hoy, porque las dos están enroladas: la que
// deriva construir.sh (`0.130.0-flota.e140e0c`) y la vieja de `git describe` con prefijo
// (`v0.106.0-28-gdf2ec21`). Devuelve ok=false para todo lo demás —`dev` incluido, que es lo que
// queda en un binario construido sin ldflags—, y ese false es el que apaga la serie en vez de
// inventar una comparación.
func NucleoDeVersion(v string) (string, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	partes := strings.Split(v, ".")
	if len(partes) != 3 {
		return "", false
	}
	for _, p := range partes {
		if p == "" {
			return "", false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return "", false
			}
		}
	}
	return v, true
}

// BuildDelAgenteDifiere responde si el agente corre un binario construido de OTRO CÓDIGO que el del
// cerebro, y no sólo de otro release. Es la hermana de VersionDelAgenteDifiere y existe porque las
// dos preguntas son distintas y sólo una estaba contestada. «Otro código» y no «otro binario»: un
// agente de Windows nunca corre los mismos bytes que el cerebro de Linux, y no es eso lo que importa.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ NO ALCANZA CON LA DEL NÚCLEO, Y POR QUÉ NO SE LA CAMBIA
//
// `NucleoDeVersion` recorta en el primer `-`, así que `0.131.0-flota.b97a81c` y
// `0.131.0-flota.d6f623d` —41 commits de diferencia— dan el MISMO núcleo y la serie contesta «no
// difiere». Una máquina puede quedarse meses atrás sin que nada lo diga, mientras el MINOR no
// cambie (A118).
//
// Y la del núcleo NO se toca, a propósito: su prueba
// `TestDosCommitsDelMismoReleaseNoSonUnAgenteAtrasado` declara como sabotaje exactamente eso
// —comparar las cadenas completas— porque el binario del cerebro se redespliega varias veces por
// día y marcaría a la flota entera después de cada despliegue. Las dos coexisten: aquélla dice
// «esta máquina se quedó en otro RELEASE» y ésta «esta máquina corre OTRO CÓDIGO». La segunda es
// ruidosa por naturaleza, y por eso su alerta la lee con un plazo largo en vez de al instante.
//
// SE COMPARAN EL NÚCLEO Y EL COMMIT, NO LA CADENA ENTERA (A138)
//
// Hasta A138 la comparación era textual, con el argumento de que «un sufijo distinto ES un binario
// distinto». Era falso, y se midió: la versión que arma construir.sh lleva dos cosas que no salen
// del código. La ETIQUETA del track la elige quien construye —`flota` el actualizador de Windows,
// `main` el redespliegue del cerebro— y el LARGO de la huella lo decide `git rev-parse --short`,
// que la alarga cuando el clon tiene más objetos. El 2026-09-29 el cerebro corría
// `0.141.0-main.eb2cdb7`, y ese mismo commit, construido por el actualizador en la laptop, salía
// `0.141.0-flota.eb2cdb72`. Una máquina actualizada a EXACTAMENTE el código del cerebro quedaba
// marcada para siempre: la alerta no se apagaba ni actualizando, y así se enseña a ignorarla.
//
// Así que se igualan el núcleo y el commit, y el commit por PREFIJO: la huella más corta tiene que
// ser el principio de la otra. Con tres límites que son la mitad de la decisión, porque lo que
// esta serie no puede hacer es declarar «al día» a una máquina que no lo está:
//
//   - la huella tiene al menos 7 caracteres hexadecimales en minúscula, que es lo que emite git:
//     por debajo, un prefijo compartido deja de nombrar UN commit, y cualquier cosa pegada detrás
//     es una forma que no sabemos qué código nombra;
//   - un build SUCIO no se iguala por commit: no salió del commit que nombra, y dos sucios del
//     mismo commit pueden ser código distinto;
//   - lo que no tiene esa forma —la familia de `git describe`, un `dev`— vuelve a la comparación
//     textual: no hay commit que leer, y adivinarlo sería inventar una igualdad.
//
// Sigue sin haber ORDEN entre dos commits: la serie dice «otro código», no «más viejo». Sacar un
// orden de la cadena inventaría una precisión que el dato no tiene.
// ════════════════════════════════════════════════════════════════════════════════════════════
func BuildDelAgenteDifiere(agente, cerebro string) (difiere bool, comparable bool) {
	a := strings.TrimPrefix(strings.TrimSpace(agente), "v")
	c := strings.TrimPrefix(strings.TrimSpace(cerebro), "v")
	// Sin versión del agente no hay nada que comparar: es un Tier B, que no corre nuestro binario.
	if a == "" {
		return false, false
	}
	// Sin referencia propia, callarse. Marcar a la flota entera por un build nuestro sin ldflags
	// sería culparla de un problema de acá — la misma regla que gobierna a la del núcleo.
	if _, ok := NucleoDeVersion(c); !ok {
		return false, false
	}
	return !mismoCodigo(a, c), true
}

// mismoCodigo dice si dos versiones nombran builds del mismo código: la cadena idéntica, o el mismo
// núcleo y el mismo commit, sin importar la etiqueta ni el largo de la huella.
//
// La cadena idéntica se iguala aunque sea sucia: es lo que hacía la comparación textual, y el dato
// no da para más.
func mismoCodigo(a, c string) bool {
	if a == c {
		return true
	}
	na, ca, okA := commitDelBuild(a)
	nc, cc, okC := commitDelBuild(c)
	return okA && okC && na == nc && mismoCommit(ca, cc)
}

// commitDelBuild separa una versión de construir.sh —`<núcleo>-<etiqueta>.<commit>`— en su núcleo
// y su commit. ok=false para cualquier otra forma: sin etiqueta, o sin una huella de git después
// del último punto.
//
// UN BUILD SUCIO CAE ACÁ SIN QUE HAGA FALTA NOMBRARLO, Y ES A PROPÓSITO. construir.sh le pega
// `-sucio` detrás de la huella, y `eb2cdb72-sucio` ya no es una huella: el binario no salió del
// commit que nombra, así que no hay commit que igualar. Una lista de sufijos prohibidos se queda
// corta el día que aparece otro; exigir que después del punto haya SÓLO una huella los deja afuera
// a todos. Que construir.sh siga poniendo la marca DETRÁS de la huella lo mide
// deploy/pruebas/version-parseable.sh contra el guion de verdad.
func commitDelBuild(v string) (nucleo, commit string, ok bool) {
	nucleo, resto, _ := strings.Cut(v, "-")
	i := strings.LastIndex(resto, ".")
	if i <= 0 {
		return "", "", false
	}
	commit = resto[i+1:]
	if len(commit) < 7 || strings.Trim(commit, "0123456789abcdef") != "" {
		return "", "", false
	}
	return nucleo, commit, true
}

// mismoCommit dice si dos huellas nombran el mismo commit: la más corta tiene que ser el principio
// de la otra, que es lo que absorbe el largo variable de `git rev-parse --short`.
func mismoCommit(x, y string) bool {
	if len(x) > len(y) {
		x, y = y, x
	}
	return strings.HasPrefix(y, x)
}

// VersionReportadaMax es cuánto guarda el cerebro de la versión que un agente declara en su
// latido. Una versión de construir.sh mide unos veinte bytes: el techo está para que un agente roto
// no llene la fila de basura, no para recortar versiones de verdad.
const VersionReportadaMax = 64

// marcaDeVersionRecortada cierra una versión que no entró. No es hexadecimal a propósito: es lo que
// hace que lo que queda detrás del último punto deje de ser una huella.
const marcaDeVersionRecortada = "…"

// VersionReportada es la versión que declaró un agente tal como la guarda el cerebro: sin espacios
// alrededor y, si no entra en VersionReportadaMax, recortada CON UNA MARCA al final.
//
// EL RECORTE SE MARCA PORQUE UN RECORTE MUDO FABRICA UNA VERSIÓN QUE NO EXISTE (A138). construir.sh
// pone `-sucio` DETRÁS de la huella, que es justo lo primero que se pierde al cortar: una versión
// sucia de etiqueta larga quedaba guardada como el build LIMPIO de ese commit, y
// BuildDelAgenteDifiere la igualaba con el cerebro. «Al día», de un binario con código que el
// commit no tiene. Con la marca, detrás del último punto ya no hay una huella y la comparación
// vuelve al texto, que es el lado seguro. Lo encontró la revisión de A138: la comparación textual
// de antes no igualaba esas dos cadenas, así que el hueco lo abría la comparación por commit.
func VersionReportada(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= VersionReportadaMax {
		return v
	}
	corte := VersionReportadaMax - len(marcaDeVersionRecortada)
	// Sin partir un carácter en dos: una versión ilegible puede traer cualquier cosa.
	for corte > 0 && !utf8.RuneStart(v[corte]) {
		corte--
	}
	return v[:corte] + marcaDeVersionRecortada
}

// VersionDelAgenteDifiere responde si el agente de una máquina corre una versión distinta de la
// del cerebro. El segundo valor es si la pregunta se puede contestar; false ⇒ la serie NO se
// emite, que es la regla que gobierna el exportador entero (un dato ausente no es un cero).
//
// LOS TRES «NO SÉ» SON DISTINTOS Y CONVIENE TENERLOS SEPARADOS:
//
//   - El agente no reportó versión: es un Tier B sondeado por SSH, que no corre nuestro binario y
//     nunca va a tener una. No hay nada atrasado ahí.
//   - EL CEREBRO no sabe la suya (un binario sin ldflags, o una construcción nueva que se olvidó
//     de inyectarla): sin referencia, marcar a la flota entera sería culparla de un bug nuestro.
//   - El agente reporta algo ilegible: eso SÍ se responde, y con 1. No sabemos cuánto se atrasó,
//     pero sabemos que no es la nuestra, y ésa es la pregunta que la serie hace.
func VersionDelAgenteDifiere(agente, cerebro string) (difiere bool, comparable bool) {
	if strings.TrimSpace(agente) == "" {
		return false, false
	}
	nucleoCerebro, ok := NucleoDeVersion(cerebro)
	if !ok {
		return false, false
	}
	nucleoAgente, ok := NucleoDeVersion(agente)
	if !ok {
		return true, true
	}
	return nucleoAgente != nucleoCerebro, true
}
