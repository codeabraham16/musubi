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

import "strings"

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

// BuildDelAgenteDifiere responde si el agente corre un BINARIO distinto del cerebro, y no sólo un
// release distinto. Es la hermana de VersionDelAgenteDifiere y existe porque las dos preguntas son
// distintas y sólo una estaba contestada.
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
// «esta máquina se quedó en otro RELEASE» y ésta «esta máquina corre OTRO BINARIO». La segunda es
// ruidosa por naturaleza, y por eso su alerta la lee con un plazo largo en vez de al instante.
//
// LA COMPARACIÓN ES TEXTUAL Y NO SEMÁNTICA, y es lo correcto acá: un sufijo distinto ES un binario
// distinto, no hay orden entre dos commits que se pueda derivar de la cadena, y pretender uno
// inventaría una precisión que el dato no tiene.
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
	return a != c, true
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
