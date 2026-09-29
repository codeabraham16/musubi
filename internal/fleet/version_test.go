package fleet

import (
	"strings"
	"testing"
)

// LAS DOS FAMILIAS DE VERSIÓN QUE HAY ENROLADAS TIENEN QUE PARSEAR.
//
// No es un caso teórico: el 2026-09-01 el cerebro corría `0.130.0-flota.38a0a9f` y los dos Windows
// `v0.106.0-28-gdf2ec21`. Si el parser sólo entiende una de las dos formas, la máquina vieja —que
// es justo la que hay que detectar— cae en el «no sé» y la serie no se emite: el agujero de A68
// quedaría exactamente igual, pero ahora con código que parece cubrirlo.
func TestElNucleoSaleDeLasDosFamiliasDeVersionQueExisten(t *testing.T) {
	casos := []struct {
		entrada string
		nucleo  string
		ok      bool
	}{
		{"0.130.0-flota.38a0a9f", "0.130.0", true}, // la que deriva construir.sh
		{"v0.106.0-28-gdf2ec21", "0.106.0", true},  // la vieja de git describe
		{"0.130.0", "0.130.0", true},               // pelada
		{"  0.130.0-flota.x  ", "0.130.0", true},   // con espacios alrededor
		{"0.130.0-sucio", "0.130.0", true},         // árbol sucio: el núcleo sigue siendo el mismo
		{"0.130.0+build5", "0.130.0", true},        // metadata semver
		{"dev", "", false},                         // build local sin ldflags
		{"", "", false},                            // Tier B
		{"0.130", "", false},                       // incompleta
		{"0.130.0.1", "", false},                   // cuatro componentes
		{"0.x.0", "", false},                       // no numérica
		{"v", "", false},                           // sólo el prefijo
	}
	for _, c := range casos {
		nucleo, ok := NucleoDeVersion(c.entrada)
		if ok != c.ok || nucleo != c.nucleo {
			t.Errorf("NucleoDeVersion(%q) = (%q, %v); esperaba (%q, %v)", c.entrada, nucleo, ok, c.nucleo, c.ok)
		}
	}
}

// EL CASO QUE HACE ÚTIL A LA MÉTRICA Y EL QUE LA HARÍA RUIDO, UNO AL LADO DEL OTRO.
//
// Sabotaje: comparar las cadenas completas en vez del núcleo → la primera fila (mismo release,
// commits distintos) pasa a difiere=true, y con ella toda la flota en cada redespliegue.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="\tnucleoAgente, ok := NucleoDeVersion(agente)\n\tif !ok {\n\t\treturn true, true\n\t}\n\treturn nucleoAgente != nucleoCerebro, true"
// arnes: a="\tnucleoAgente, ok := NucleoDeVersion(agente)\n\tif !ok {\n\t\treturn true, true\n\t}\n\tnucleoAgente = strings.TrimSpace(agente)\n\tnucleoCerebro = strings.TrimSpace(cerebro)\n\treturn nucleoAgente != nucleoCerebro, true"
func TestDosCommitsDelMismoReleaseNoSonUnAgenteAtrasado(t *testing.T) {
	const cerebro = "0.130.0-flota.38a0a9f"

	casos := []struct {
		nombre     string
		agente     string
		difiere    bool
		comparable bool
	}{
		{"mismo release, otro commit", "0.130.0-flota.e140e0c", false, true},
		{"idéntica", cerebro, false, true},
		{"veinticuatro versiones atrás (el caso de A68)", "v0.106.0-28-gdf2ec21", true, true},
		{"adelantada: alguien desplegó un binario que el cerebro no tiene", "0.131.0-flota.aaaaaaa", true, true},
		{"Tier B: no corre nuestro binario", "", false, false},
		{"versión ilegible: no sabemos cuánto, sabemos que no es la nuestra", "vieja", true, true},
	}
	for _, c := range casos {
		difiere, comparable := VersionDelAgenteDifiere(c.agente, cerebro)
		if difiere != c.difiere || comparable != c.comparable {
			t.Errorf("%s: VersionDelAgenteDifiere(%q, %q) = (%v, %v); esperaba (%v, %v)",
				c.nombre, c.agente, cerebro, difiere, comparable, c.difiere, c.comparable)
		}
	}
}

// SI EL CEREBRO NO SABE SU PROPIA VERSIÓN, LA CULPA NO ES DE LA FLOTA.
//
// Un binario construido sin `-ldflags -X main.version` queda en `dev`. Con la comparación ingenua,
// `dev != 0.130.0` para TODAS las máquinas: la flota entera se dibuja atrasada por un problema de
// nuestro propio build. La serie se apaga, y el binario mal construido se descubre por otro lado
// —`musubi version` lo dice— en vez de por una alarma que acusa a las máquinas equivocadas.
//
// Sabotaje: devolver (true, true) cuando el núcleo del cerebro no parsea → falla acá.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="func VersionDelAgenteDifiere(agente, cerebro string) (difiere bool, comparable bool) {\n\tif strings.TrimSpace(agente) == \"\" {\n\t\treturn false, false\n\t}\n"
// arnes: a="func VersionDelAgenteDifiere(agente, cerebro string) (difiere bool, comparable bool) {\n\tif strings.TrimSpace(agente) == \"\" {\n\t\treturn false, false\n\t}\n\tif _, ok := NucleoDeVersion(cerebro); !ok {\n\t\treturn true, true\n\t}\n"
func TestUnCerebroSinVersionNoMarcaAtrasadaALaFlotaEntera(t *testing.T) {
	for _, cerebro := range []string{"dev", "", "no-es-una-version"} {
		difiere, comparable := VersionDelAgenteDifiere("0.130.0-flota.e140e0c", cerebro)
		if comparable {
			t.Errorf("con el cerebro en %q la serie se emite igual (difiere=%v): un build propio sin "+
				"ldflags marcaría a toda la flota como atrasada", cerebro, difiere)
		}
	}
}

// EL BUILD ES OTRA PREGUNTA QUE EL RELEASE, Y HASTA A118 SÓLO UNA ESTABA CONTESTADA.
//
// `0.131.0-flota.b97a81c` y `0.131.0-flota.d6f623d` son 41 commits de diferencia y el MISMO
// núcleo: la serie del release contesta «no difiere» y una máquina puede quedarse meses atrás
// mientras el MINOR no cambie. Medido con las cadenas reales de la flota el 2026-09-08.
//
// LAS DOS CONVIVEN Y NINGUNA REEMPLAZA A LA OTRA: la del núcleo NO puede comparar el build —su
// propia guarda declara eso como sabotaje, porque el cerebro se redespliega varias veces por día
// y marcaría a la flota entera— y ésta no puede ordenar dos commits, porque de la cadena no sale
// ningún orden.
//
// Sabotaje: hacer que BuildDelAgenteDifiere compare el núcleo, que es volver a A118. Desde A138 el
// commit sale de `commitDelBuild`, así que basta con que lo tire: queda sólo el núcleo.
//
// arnes: archivo="internal/fleet/version.go"
// arnes: de="\treturn nucleo, commit, true\n}"
// arnes: a="\treturn nucleo, \"\", true\n}"
func TestDosBuildsDelMismoReleaseSonBinariosDistintos(t *testing.T) {
	const cerebro = "0.131.0-flota.d6f623d"

	casos := []struct {
		nombre     string
		agente     string
		difiere    bool
		comparable bool
	}{
		// EL CASO DE A118, medido con las cadenas reales de la flota: mismo release, 41 commits
		// de diferencia. La serie del núcleo dice «al día»; ésta dice la verdad.
		{"mismo release, otro commit", "0.131.0-flota.b97a81c", true, true},
		{"el mismo binario", "0.131.0-flota.d6f623d", false, true},
		{"con espacios y prefijo v", "  v0.131.0-flota.d6f623d ", false, true},
		{"otro release", "0.130.0-flota.38a0a9f", true, true},
		// Un agente viejo de la otra familia: difiere, y se puede decir.
		{"la familia de git describe", "v0.106.0-28-gdf2ec21", true, true},
		// Tier B: no corre nuestro binario, no hay nada que comparar.
		{"sin versión", "", false, false},
		// Un agente ilegible SÍ se responde: no sabemos cuánto se atrasó, pero no es el nuestro.
		{"ilegible", "dev", true, true},
	}
	for _, c := range casos {
		difiere, comparable := BuildDelAgenteDifiere(c.agente, cerebro)
		if difiere != c.difiere || comparable != c.comparable {
			t.Errorf("%s: BuildDelAgenteDifiere(%q) = (%v, %v); esperaba (%v, %v)",
				c.nombre, c.agente, difiere, comparable, c.difiere, c.comparable)
		}
	}
}

// SIN REFERENCIA PROPIA, LA PREGUNTA NO SE CONTESTA — la misma regla que gobierna a la del
// núcleo: un cerebro construido sin ldflags marcaría a la flota entera por un defecto de acá.
func TestSinVersionDelCerebroElBuildNoSeCompara(t *testing.T) {
	for _, cerebro := range []string{"", "dev", "0.131", "v"} {
		if difiere, comparable := BuildDelAgenteDifiere("0.131.0-flota.abc", cerebro); comparable || difiere {
			t.Errorf("con el cerebro en %q la comparación se contestó (difiere=%v comparable=%v)", cerebro, difiere, comparable)
		}
	}
}

// EL MISMO COMMIT ES EL MISMO CÓDIGO, AUNQUE CAMBIEN LA ETIQUETA Y EL LARGO DE LA HUELLA (A138).
//
// Medido el 2026-09-29: el cerebro corría `0.141.0-main.eb2cdb7` y ese MISMO commit, construido por
// el actualizador de Windows en la laptop, salía `0.141.0-flota.eb2cdb72`. Otra etiqueta, porque la
// elige quien construye, y una huella un carácter más larga, porque `git rev-parse --short` la
// alarga cuando el clon tiene más objetos. Con la comparación textual, una máquina actualizada a
// exactamente el código del cerebro quedaba marcada para siempre: la alerta no se apagaba ni
// actualizando.
//
// Y la respuesta no puede depender de cuál de los dos es el cerebro: si A es el mismo código que B,
// B es el mismo código que A. La prueba pregunta en los dos sentidos.
//
// EL ORDEN DE LAS FILAS ES PARTE DE LA PRUEBA. Cada sabotaje de abajo cae PRIMERO en una fila
// distinta, así que el motivo del rojo dice cuál se rompió: con las filas en otro orden, tres de
// ellos caían en la misma y el arnés no los podía distinguir. Por eso la primera fila tiene la misma
// huella —sólo la tira el texto entero— y la segunda tiene la huella más larga del lado del cerebro,
// que sólo la tira la igualdad exacta, y con los papeles cambiados el no ordenar por largo.
//
// Sabotaje: volver a comparar el texto entero, que es exactamente lo que había en main.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="\treturn !mismoCodigo(a, c), true\n}"
// arnes: a="\treturn a != c, true\n}"
//
// Sabotaje: igualar las huellas sólo si son idénticas, que deja afuera el largo variable.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="\treturn strings.HasPrefix(y, x)\n}"
// arnes: a="\treturn x == y\n}"
//
// Sabotaje: no ordenar las huellas por largo, con lo que el prefijo se mira en un solo sentido.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="\tif len(x) > len(y) {\n\t\tx, y = y, x\n\t}\n"
// arnes: a=""
//
// Sabotaje: igualar el commit sin mirar el release.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="okA && okC && na == nc && mismoCommit(ca, cc)"
// arnes: a="okA && okC && na+nc != \"\" && mismoCommit(ca, cc)"
func TestElMismoCommitEsElMismoCodigoAunqueCambieLaEtiqueta(t *testing.T) {
	const cerebro = "0.141.0-main.eb2cdb7"

	casos := []struct {
		nombre  string
		agente  string
		cerebro string
		difiere bool
	}{
		{"otra etiqueta, la misma huella", "0.141.0-flota.eb2cdb7", cerebro, false},
		{"el cerebro con la huella más larga", "0.141.0-flota.eb2cdb7", "0.141.0-main.eb2cdb72", false},
		// EL CASO MEDIDO: el actualizador de Windows contra el redespliegue del cerebro.
		{"el agente con la huella más larga", "0.141.0-flota.eb2cdb72", cerebro, false},
		{"la huella completa", "0.141.0-flota.eb2cdb72ffa390b24bdc391f96df977e309a41df", cerebro, false},
		{"una etiqueta con guiones", "0.141.0-mi-rama.eb2cdb7", cerebro, false},
		// El agente de musubi-server el mismo día: mismo release, otro commit. Ése SÍ difiere.
		{"otro commit", "0.141.0-main.b14bff87", cerebro, true},
		// La misma huella con otro release no es el mismo código: o la huella choca, o la cadena la
		// armó alguien a mano. Ninguna de las dos es «al día».
		{"otro release con la misma huella", "0.140.3-flota.eb2cdb7", cerebro, true},
	}
	for _, c := range casos {
		difiere, comparable := BuildDelAgenteDifiere(c.agente, c.cerebro)
		if difiere != c.difiere || !comparable {
			t.Errorf("%s: BuildDelAgenteDifiere(%q, %q) = (%v, %v); esperaba (%v, true)",
				c.nombre, c.agente, c.cerebro, difiere, comparable, c.difiere)
		}
		difiere, comparable = BuildDelAgenteDifiere(c.cerebro, c.agente)
		if difiere != c.difiere || !comparable {
			t.Errorf("%s, con los papeles cambiados: BuildDelAgenteDifiere(%q, %q) = (%v, %v); esperaba (%v, true)",
				c.nombre, c.cerebro, c.agente, difiere, comparable, c.difiere)
		}
	}
}

// UN BUILD SUCIO NO SE IGUALA POR COMMIT: NO SALIÓ DEL COMMIT QUE NOMBRA.
//
// construir.sh le pega `-sucio` a la versión cuando el árbol tenía cambios sin commitear. Ese
// binario lleva código que el commit no tiene, y dos sucios del mismo commit pueden llevar cambios
// distintos: igualarlos por la huella sería declarar «al día» a una máquina que corre otra cosa, que
// es lo único que esta serie no puede hacer. La cadena idéntica sí se iguala —es lo que hacía la
// comparación textual, y el dato no da para más—.
//
// Sabotaje: leer el commit de un build sucio como si fuera limpio.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="\tnucleo, resto, _ := strings.Cut(v, \"-\")\n"
// arnes: a="\tnucleo, resto, _ := strings.Cut(strings.TrimSuffix(v, \"-sucio\"), \"-\")\n"
func TestUnBuildSucioNoSeIgualaPorCommit(t *testing.T) {
	casos := []struct {
		nombre  string
		agente  string
		cerebro string
		difiere bool
	}{
		{"el agente sucio", "0.141.0-flota.eb2cdb72-sucio", "0.141.0-main.eb2cdb7", true},
		{"el cerebro sucio", "0.141.0-flota.eb2cdb72", "0.141.0-main.eb2cdb7-sucio", true},
		{"los dos sucios del mismo commit", "0.141.0-flota.eb2cdb7-sucio", "0.141.0-main.eb2cdb7-sucio", true},
		{"la misma cadena sucia", "0.141.0-main.eb2cdb7-sucio", "0.141.0-main.eb2cdb7-sucio", false},
	}
	for _, c := range casos {
		difiere, comparable := BuildDelAgenteDifiere(c.agente, c.cerebro)
		if difiere != c.difiere || !comparable {
			t.Errorf("%s: BuildDelAgenteDifiere(%q, %q) = (%v, %v); esperaba (%v, true)",
				c.nombre, c.agente, c.cerebro, difiere, comparable, c.difiere)
		}
	}
}

// UNA VERSIÓN RECORTADA NO PUEDE PASAR POR UN BUILD LIMPIO (A138).
//
// El cerebro guarda a lo sumo VersionReportadaMax bytes de la versión que declara el agente, y
// `-sucio` va DETRÁS de la huella: es lo primero que se pierde al cortar. Lo encontró la revisión de
// A138, midiendo el camino del latido: con un recorte mudo, un build sucio de etiqueta larga quedaba
// guardado como el build limpio de ese commit, y la comparación por commit lo igualaba con el
// cerebro. La textual de antes no lo igualaba: el hueco lo abría A138.
//
// La etiqueta se calcula para que el corte caiga JUSTO detrás de la huella, que es el peor caso: lo
// que queda es indistinguible de un build limpio. El control lo afirma antes que nada. Sin él, un
// cambio en el techo dejaría a la prueba construyendo otro caso y pasando sin medir nada.
//
// Sabotaje: recortar sin la marca, que es lo que hacía el latido.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="\treturn v[:corte] + marcaDeVersionRecortada\n"
// arnes: a="\treturn v[:VersionReportadaMax]\n"
func TestUnaVersionRecortadaNoPasaPorUnBuildLimpio(t *testing.T) {
	const cerebro = "0.141.0-main.eb2cdb7"
	cabeza, huella := "0.141.0-", ".eb2cdb72"
	sucia := cabeza + strings.Repeat("e", VersionReportadaMax-len(cabeza)-len(huella)) + huella + "-sucio"

	if difiere, _ := BuildDelAgenteDifiere(sucia[:VersionReportadaMax], cerebro); difiere {
		t.Fatalf("control: %q cortada a secas ya no se iguala con %q, así que la prueba dejó de "+
			"construir el caso que la motivó", sucia, cerebro)
	}

	guardada := VersionReportada(sucia)
	if difiere, comparable := BuildDelAgenteDifiere(guardada, cerebro); !difiere || !comparable {
		t.Errorf("la versión sucia %q se guardó como %q y se declaró el mismo código que %q "+
			"(difiere=%v comparable=%v): el recorte le sacó el -sucio, y la serie diría «al día» de "+
			"un binario con código que el commit no tiene", sucia, guardada, cerebro, difiere, comparable)
	}
	if len(guardada) > VersionReportadaMax {
		t.Errorf("VersionReportada(%q) = %q mide %d bytes, más que el techo de %d",
			sucia, guardada, len(guardada), VersionReportadaMax)
	}
	// El núcleo sobrevive a la marca: un recorte no puede dejar ciega a la serie del release.
	if n, ok := NucleoDeVersion(guardada); !ok || n != "0.141.0" {
		t.Errorf("NucleoDeVersion(%q) = (%q, %v); esperaba (\"0.141.0\", true)", guardada, n, ok)
	}
	// Lo que entra no se toca: sólo pierde los espacios de alrededor.
	for _, v := range []string{"0.141.0-flota.eb2cdb72", " 0.141.0-flota.eb2cdb72\n", ""} {
		if got := VersionReportada(v); got != strings.TrimSpace(v) {
			t.Errorf("VersionReportada(%q) = %q; una versión que entra se guarda entera", v, got)
		}
	}
}

// SÓLO UNA HUELLA DE GIT SE IGUALA POR PREFIJO; LO DEMÁS SE COMPARA COMO TEXTO.
//
// Igualar por prefijo es seguro mientras el prefijo nombre UN commit y sea todo lo que hay después
// del punto. Cada fila es una forma que se parece a la del cerebro y que NO se puede declarar el
// mismo código: una huella tan corta que ya no nombra un commit, algo pegado detrás de la huella,
// una versión sin etiqueta —construir.sh la exige, así que ésta no la armó él— y la familia vieja de
// `git describe`, cuyo núcleo es el último tag y no el archivo VERSION.
//
// Sabotaje: aceptar huellas de 6 caracteres.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="len(commit) < 7"
// arnes: a="len(commit) < 6"
//
// Sabotaje: no mirar que después del punto haya sólo hexadecimal.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="strings.Trim(commit, \"0123456789abcdef\") != \"\""
// arnes: a="false"
//
// Sabotaje: aceptar una versión sin etiqueta, o con la etiqueta vacía.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="\tif i <= 0 {\n\t\treturn \"\", \"\", false\n\t}\n"
// arnes: a=""
//
// Sabotaje: pasar la huella a minúsculas antes de mirarla, que acepta las mayúsculas.
// arnes: archivo="internal/fleet/version.go"
// arnes: de="\tcommit = resto[i+1:]\n"
// arnes: a="\tcommit = strings.ToLower(resto[i+1:])\n"
func TestSoloUnaHuellaDeGitSeIgualaPorPrefijo(t *testing.T) {
	const cerebro = "0.141.0-main.eb2cdb7"

	for _, agente := range []string{
		"0.141.0-flota.eb2c",           // cuatro caracteres: ya no nombra un commit
		"0.141.0-flota.eb2cdb",         // seis: uno menos que lo que emite git
		"0.141.0-flota.eb2cdb7-parche", // algo pegado detrás de la huella
		"0.141.0-eb2cdb7",              // sin etiqueta
		"0.141.0-.eb2cdb7",             // con la etiqueta vacía
		"0.141.0-28-geb2cdb7",          // la familia de git describe
	} {
		difiere, comparable := BuildDelAgenteDifiere(agente, cerebro)
		if !difiere || !comparable {
			t.Errorf("BuildDelAgenteDifiere(%q, %q) = (%v, %v); esperaba (true, true): esa forma no "+
				"es una huella de git y se declaró el mismo código", agente, cerebro, difiere, comparable)
		}
	}

	// UNA HUELLA EN MAYÚSCULAS NO LA EMITE GIT, ASÍ QUE NO SE IGUALA COMO SI LO FUERA. Va con las DOS
	// cadenas en mayúsculas: con el cerebro en minúsculas el prefijo ya no coincide, la fila da
	// «difiere» por la razón equivocada y no distingue nada. Lo encontró la revisión de A138.
	agente, cerebroEnMayusculas := "0.141.0-flota.EB2CDB72", "0.141.0-main.EB2CDB7"
	if difiere, comparable := BuildDelAgenteDifiere(agente, cerebroEnMayusculas); !difiere || !comparable {
		t.Errorf("BuildDelAgenteDifiere(%q, %q) = (%v, %v); esperaba (true, true): una huella en "+
			"mayúsculas no es de git y se declaró el mismo código", agente, cerebroEnMayusculas, difiere, comparable)
	}
}
