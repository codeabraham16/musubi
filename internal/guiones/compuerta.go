// Package guiones es EL ÚNICO LUGAR donde una prueba de este repo lanza un proceso, y la
// compuerta de las que EJECUTAN un guion de shell. No tiene otra función y no debería tener otra.
//
// Las dos cosas en un solo paquete no son dos responsabilidades: son la misma. Mientras una
// prueba pudiera llamar a `exec.Command` por su cuenta, saber cuáles de esas llamadas eran shells
// obligaba a interpretar el primer argumento en el árbol sintáctico, y esa pregunta no converge
// (A127). Con `Herramienta` acá, la guarda pregunta una sola cosa —«¿alguien nombró
// `exec.Command`?»— y el «¿es una shell?» se contesta en runtime, sobre el string.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ EXISTE: EL ARNÉS ES DE UNIX Y LO QUE MIDE SON INSTALADORES DE SERVIDOR LINUX
//
// Las pruebas de `internal/mcp/despliegue_*_test.go` no grepean los guiones de `deploy/`: los
// CORREN, con un `curl` de mentira en el PATH, y miran si el archivo apareció en el destino. Eso
// las hace valiosas —un grep lo satisface un comentario— y también las ata al sistema operativo:
//
//   - el arnés escribe stubs ejecutables (`0o755` + shebang) y los pone primero en `$PATH`
//     separando con `:`;
//   - el prólogo es bash con `set -euo pipefail`, arrays y `[[ =~ ]]`;
//   - los guiones llaman `sha256sum`, `install`, `mktemp`, `awk`, `tr`, `unzip`, `tar`, `find`,
//     `uname -m` — coreutils GNU;
//   - y lo que instalan son unidades systemd de un servidor Linux, con `useradd` y `/etc`.
//
// MEDIDO EN CI (jobs test-cross), y por eso esto existe:
//
//   - Windows: el guion salió con `exit status 22` —el código de curl para «el servidor devolvió
//     un error HTTP»— después de imprimir la URL real del release. O sea: el shell de Windows NO
//     tomó el stub del PATH y el arnés SALIÓ A INTERNET. Un `PATH=C:\...\stubs:"$PATH"` se parte
//     en dos por el `:` del nombre de unidad, y un archivo `curl` sin `.exe` ni bit de ejecución
//     no es un ejecutable ahí.
//   - macOS: el runner es arm64, `uname -m` contesta `arm64`, y el guion del relay muere en
//     «arquitectura no soportada por los binarios oficiales» ANTES de llegar al checksum. El
//     arnés veía «no se instaló» y lo leía como «la verificación funcionó».
//
// EL SEGUNDO CASO ES EL PELIGROSO Y ES LA RAZÓN DE ESTA COMPUERTA: una prueba que se pone roja —o
// verde— por un motivo que NO es el que dice custodiar es un falso positivo, y un falso positivo
// termina apagado. Acotar el alcance a donde el guion corre de verdad no baja el listón: el job
// `test` de CI, el que gatea el merge, es linux y ahí corren TODAS, enteras.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LO QUE ESTA COMPUERTA NO PUEDE HACER, POR CONSTRUCCIÓN
//
//  1. NO PUEDE SALTEAR EN LINUX. La rama de `t.Skipf` es inalcanzable cuando GOOS es linux: no es
//     una promesa escrita, es el `if` de abajo. Y no se le cree al `if`: `TestLaCompuertaNuncaSaltea…`
//     la CORRE de verdad y comprueba que la línea siguiente se ejecutó.
//  2. NO PUEDE TAPAR UN «NO PUDE MEDIR». En linux, una herramienta que falta es `t.Fatal`, no
//     `t.Skip`. Ésa es la trampa que ya estaba puesta en cuatro pruebas de este repo
//     (`if _, err := exec.LookPath("bash"); err != nil { t.Skip(...) }`): sin bash, `go test`
//     contestaba `ok` y la guarda no existía.
//  3. NO PUEDE USARSE PARA OTRA COSA, y la equivalencia se exige en los DOS sentidos. Quien
//     ejecuta una shell pasa por acá: no hay dónde escribir lo contrario, porque el `*exec.Cmd`
//     de una shell sólo se consigue como método de lo que esta compuerta DEVUELVE. Y quien llama
//     a esta compuerta ejecuta una shell: si la prueba termina sin haber pedido un comando, la
//     propia compuerta lo dice en su `t.Cleanup`. Las dos mitades se contestan sobre valores
//     reales; hasta A127 se perseguían leyendo el árbol sintáctico, y así no convergían.
//
// Y EL ARCHIVO NO LLEVA SUFIJO DE PLATAFORMA NI `//go:build`: compila en todas y decide en
// runtime. Este repo ya se comió que `despliegue_agente_windows_test.go` no compilara en Linux
// mientras `go test` decía `ok` — la ausencia de una prueba se ve idéntica a su éxito.
package guiones

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ElSistemaDondeCorren es el GOOS en el que los guiones de `deploy/` se ejecutan de verdad, que es
// el único en el que estas pruebas miden lo que dicen medir.
const ElSistemaDondeCorren = "linux"

// Portable declara que una prueba EJECUTA un guion de shell y aun así corre en las tres
// plataformas A PROPÓSITO. No saltea nada y no apaga nada: existe para que la guarda de alcance
// distinga «este hermano se olvidó» de «este hermano es portable y alguien lo pensó».
//
// LA PUERTA DE ATRÁS QUE ESTO NO ES. Lo obvio sería dejar que la guarda ignore a quien no llame a
// Exigir, o llevar una lista central de excepciones. Las dos se pudren igual: la primera no
// distingue el olvido de la decisión, y la segunda envejece lejos del código que describe. Acá la
// declaración vive PEGADA a la prueba, exige una frase que diga por qué, y —esto es lo que la hace
// cara de usar mal— COMPRUEBA EN ESTA PLATAFORMA que la shell existe de verdad. Si alguien la usa
// para callar la guarda en una prueba que en Windows no puede correr, se entera ahí, en Windows.
//
// El caso que la trajo: TestElFirmadorCorreDePuntaAPunta corre `deploy/firmar-release.sh`, y dos
// de los tres defectos que ese test vino a cazar el 2026-09-10 SÓLO SE VEN EN WINDOWS (el
// `python3` que allá es el alias de la Microsoft Store y contesta 0 sin ejecutar nada, y el
// `chmod 600` que deja 644). Compuertarlo a linux habría borrado justo la cobertura que #435
// acababa de agregar — o sea, la guarda habría apagado a su propio hermano.
func Portable(t *testing.T, motivo string, herramientas ...string) Compuerta {
	t.Helper()

	if len(strings.Fields(motivo)) < 4 {
		t.Fatalf("guiones.Portable se llamó con el motivo %q: hace falta una frase que diga QUÉ "+
			"guion ejecuta y por qué vale la pena correrlo en las tres plataformas. Una excepción "+
			"sin motivo es una excepción que nadie revisa.", motivo)
	}
	if len(herramientas) == 0 {
		t.Fatal("guiones.Portable se llamó sin nombrar una sola herramienta. Declarar que una prueba " +
			"es portable sin decir QUÉ tiene que existir para que corra es declarar nada: nombrá al " +
			"menos `bash`.")
	}

	// Acá NO hay rama por GOOS a propósito: portable quiere decir portable, así que la exigencia
	// es la misma en las tres. Si falta algo, es un FALLO en la plataforma donde falta — nunca un
	// salteo —, porque «no pude medir» no puede contestar lo mismo que «medí y está bien».
	var faltan []string
	for _, h := range herramientas {
		if _, err := exec.LookPath(h); err != nil {
			faltan = append(faltan, h)
		}
	}
	if len(faltan) > 0 {
		t.Fatalf("en %s/%s falta(n) %s en el PATH, y esta prueba se declaró PORTABLE con "+
			"guiones.Portable.\n"+
			"  MOTIVO DECLARADO: %s\n"+
			"  O la declaración está mal y esto no es portable —entonces va guiones.Exigir—, o el "+
			"entorno le debe esa herramienta. Lo que no puede pasar es que quede verde sin medir.",
			runtime.GOOS, runtime.GOARCH, strings.Join(faltan, ", "), motivo)
	}

	return vigilarQueSeUse(t, "Portable", motivo, herramientas)
}

// Exigir frena la prueba si esta máquina no es donde corren los guiones, y comprueba que las
// herramientas Unix que el arnés va a usar estén.
//
//   - En linux NUNCA saltea. Si falta una herramienta, MUERE: eso es «no pude medir», y un verde
//     ahí sería la mentira exacta que estas pruebas existen para cazar.
//   - Fuera de linux saltea diciendo el GOOS, el GOARCH y el motivo concreto de quien la llamó.
//   - Un uso sin motivo o sin herramientas es un error EN TODAS las plataformas: esta compuerta no
//     es un `t.Skip` de propósito general y no se la puede usar como tal ni por accidente.
func Exigir(t *testing.T, motivo string, herramientas ...string) Compuerta {
	t.Helper()

	// Los dos controles de USO van antes que nada y no miran el GOOS: si alguien llama a esto
	// para apagar una prueba que no corre ningún guion, tiene que romperse donde lo intentó y no
	// quedar en un `skip` cómodo en la plataforma que le molestaba.
	if len(strings.Fields(motivo)) < 4 {
		t.Fatalf("guiones.Exigir se llamó con el motivo %q: hace falta una frase que diga QUÉ guion "+
			"se ejecuta y por qué sólo aplica en %s. Un salteo sin motivo es un salteo que nadie "+
			"revisa.", motivo, ElSistemaDondeCorren)
	}
	if len(herramientas) == 0 {
		t.Fatal("guiones.Exigir se llamó sin nombrar una sola herramienta Unix. Esta compuerta NO es " +
			"un t.Skip de propósito general: existe para las pruebas que EJECUTAN un guion de shell, " +
			"y ésas siempre necesitan por lo menos `bash`. Si lo que querés es apagar otra cosa, esto " +
			"no es lo que buscás.")
	}

	if runtime.GOOS == ElSistemaDondeCorren {
		var faltan []string
		for _, h := range herramientas {
			if _, err := exec.LookPath(h); err != nil {
				faltan = append(faltan, h)
			}
		}
		if len(faltan) > 0 {
			t.Fatalf("en %s falta(n) %s en el PATH, así que este arnés NO PUEDE EJERCITAR el guion.\n"+
				"  Esto es un FALLO y no un salteo a propósito: %s es donde el guion corre de verdad, "+
				"y «no pude medir» no puede contestar lo mismo que «medí y está bien».\n"+
				"  Instalalas (en Debian/Ubuntu casi todas vienen en coreutils; `unzip` y `tar` son "+
				"paquetes propios) y volvé a correr.",
				runtime.GOOS, strings.Join(faltan, ", "), ElSistemaDondeCorren)
		}
		// Único camino de vuelta en linux. No hay `t.Skip` después de acá.
		return vigilarQueSeUse(t, "Exigir", motivo, herramientas)
	}

	t.Skipf("SALTEADA EN %s/%s — esto sólo se mide en %s.\n"+
		"  MOTIVO: %s\n"+
		"  EL ARNÉS ES DE UNIX: escribe stubs ejecutables (shebang + bit de ejecución), los antepone "+
		"al PATH separando con `:`, corre bash con `set -euo pipefail` y usa %s.\n"+
		"  EN WINDOWS eso no se sostiene —medido: el `:` de `C:\\` parte el PATH, el stub sin `.exe` "+
		"no se ejecuta, y el guion SALIÓ A LA URL REAL del release (exit 22 de curl)—; en macOS el "+
		"runner es arm64 y el guion del relay muere en la arquitectura antes de llegar al checksum.\n"+
		"  DÓNDE SÍ CORRE, ENTERA Y SIEMPRE: el job `test` de CI, que es ubuntu y es el que gatea el "+
		"merge. Acá no se pierde cobertura de la propiedad; se deja de medirla donde el resultado "+
		"habla del runner y no del guion.",
		runtime.GOOS, runtime.GOARCH, ElSistemaDondeCorren, motivo, strings.Join(herramientas, ", "))
	// Inalcanzable: `t.Skipf` sale por `runtime.Goexit`. Está acá porque Go no reconoce a Skipf
	// como sentencia terminal, y una Compuerta vacía que igual se devolviera no podría construir
	// ningún comando: sus campos son privados y su `t` sería nil.
	return Compuerta{}
}

// LosSistemasDondeElArnesSeSostiene son los GOOS en los que el arnés de stubs de estas pruebas
// funciona de verdad: escribe un ejecutable con shebang y lo antepone al PATH separando con `:`.
//
// Es una lista y no una constante porque la pregunta que contesta es distinta de la de
// `ElSistemaDondeCorren`: aquélla dice dónde corre EL PRODUCTO (un servidor Linux), ésta dice
// dónde se puede MEDIR con este arnés. Confundirlas es lo que hace que una compuerta apague
// cobertura que sí valía.
var LosSistemasDondeElArnesSeSostiene = []string{"linux", "darwin"}

// Unix declara que una prueba EJECUTA un guion de shell y se mide en linux Y en macOS, nunca en
// Windows. Es el tercer modo, y existe porque los otros dos no cubren este caso.
//
// POR QUÉ NO ALCANZABA CON LOS DOS QUE YA ESTABAN:
//
//   - `Exigir` saltea en darwin. Para una prueba cuyo defecto SÓLO SE VE en darwin, eso no acota
//     el alcance: lo apaga. El caso que trajo este modo es el `${VAR}` pegada a un carácter
//     no-ASCII que mata el guion en el bash 3.2 de macOS y en Linux es invisible — había cinco en
//     `verificar-despliegue.sh` y cuatro llevaban meses sin que nadie los viera. Compuertar a
//     linux la prueba que los caza habría cerrado el defecto y apagado al mismo tiempo la única
//     plataforma capaz de verlo.
//   - `Portable` exige las herramientas en LAS TRES. En Windows `bash` existe (git-bash), así que
//     no saltea: la prueba corre de verdad y muere por rutas POSIX y por el shim de `ssh` por
//     shebang. Declarar portable algo que en Windows no puede correr es declarar una mentira que
//     se cobra en el job de Windows.
//
// Y POR QUÉ WINDOWS SÍ SE SALTEA, que es lo único medido del asunto: `PATH=C:\...\stubs:"$PATH"`
// se parte en dos por el `:` del nombre de unidad, y un archivo `curl` sin `.exe` ni bit de
// ejecución no es un ejecutable ahí. Medido en CI: el guion salió con `exit status 22` —el código
// de curl para «el servidor devolvió un error HTTP»— después de imprimir la URL real del release.
// El arnés no tomó el stub y SALIÓ A INTERNET.
//
// EL OTRO CASO MEDIDO DE macOS NO APLICA ACÁ, y la distinción es la que justifica este modo: el
// runner de macOS es arm64 y el guion del RELAY (`install-rustdesk-relay.sh`) muere en
// «arquitectura no soportada» antes de llegar al checksum. Eso es cierto de las pruebas que bajan
// binarios por arquitectura, y ésas van con `Exigir`. Las que corren `verificar-despliegue.sh` o
// `comparar-y-latir.sh` no bajan nada: leen el repo, hablan por ssh contra un shim y comparan
// texto. El arnés se les sostiene en darwin, y es donde su defecto vive.
//
// LA FIRMA ES `testing.TB` Y NO `*testing.T`, y no es gusto: un `Benchmark` o un `Fuzz` que
// ejecute un guion no puede llamar a una compuerta que pide `*testing.T` —no compila—, así que la
// guarda de alcance le estaría reclamando algo imposible de cumplir. `TB` cubre los tres.
//
// EN LOS DOS SISTEMAS DONDE MIDE, NO SALTEA NUNCA: una herramienta que falta es `t.Fatal`, igual
// que en `Exigir`. «No pude medir» no puede contestar lo mismo que «medí y está bien».
func Unix(t testing.TB, motivo string, herramientas ...string) Compuerta {
	t.Helper()

	// Los dos controles de USO van antes que el GOOS, por lo mismo que en las otras dos: si
	// alguien llama a esto para apagar una prueba que no corre ningún guion, tiene que romperse
	// donde lo intentó y no quedar en un `skip` cómodo en la plataforma que le molestaba.
	if len(strings.Fields(motivo)) < 4 {
		t.Fatalf("guiones.Unix se llamó con el motivo %q: hace falta una frase que diga QUÉ guion "+
			"se ejecuta y por qué se mide en linux y en macOS pero no en Windows. Un salteo sin "+
			"motivo es un salteo que nadie revisa.", motivo)
	}
	if len(herramientas) == 0 {
		t.Fatal("guiones.Unix se llamó sin nombrar una sola herramienta. Esta compuerta NO es un " +
			"t.Skip de propósito general: existe para las pruebas que EJECUTAN un guion de shell, " +
			"y ésas siempre necesitan por lo menos `bash`.")
	}

	for _, sistema := range LosSistemasDondeElArnesSeSostiene {
		if runtime.GOOS != sistema {
			continue
		}
		var faltan []string
		for _, h := range herramientas {
			if _, err := exec.LookPath(h); err != nil {
				faltan = append(faltan, h)
			}
		}
		if len(faltan) > 0 {
			t.Fatalf("en %s/%s falta(n) %s en el PATH, así que este arnés NO PUEDE EJERCITAR el guion.\n"+
				"  Esto es un FALLO y no un salteo a propósito: %s es uno de los sistemas donde esta "+
				"prueba se declaró medible con guiones.Unix, y «no pude medir» no puede contestar lo "+
				"mismo que «medí y está bien».\n"+
				"  MOTIVO DECLARADO: %s",
				runtime.GOOS, runtime.GOARCH, strings.Join(faltan, ", "), runtime.GOOS, motivo)
		}
		// Único camino de vuelta en los sistemas donde mide. No hay `t.Skip` después de acá.
		return vigilarQueSeUse(t, "Unix", motivo, herramientas)
	}

	t.Skipf("SALTEADA EN %s/%s — esto se mide en %s.\n"+
		"  MOTIVO: %s\n"+
		"  EN WINDOWS EL ARNÉS NO SE SOSTIENE, y está medido: `PATH=C:\\...\\stubs:\"$PATH\"` se "+
		"parte en dos por el `:` del nombre de unidad, un stub sin `.exe` ni bit de ejecución no se "+
		"ejecuta, y el guion SALIÓ A LA URL REAL del release (exit 22 de curl). Además el banco "+
		"necesita rutas POSIX y un `ssh` ejecutable por shebang.\n"+
		"  DÓNDE SÍ CORRE, ENTERA Y SIEMPRE: el job `test` de CI, que es ubuntu y es el que gatea el "+
		"merge; y el job de macOS, que es donde vive el defecto que estas pruebas cazan.",
		runtime.GOOS, runtime.GOARCH, strings.Join(LosSistemasDondeElArnesSeSostiene, " y "),
		motivo)
	// Inalcanzable: `t.Skipf` sale por `runtime.Goexit`. Está acá porque Go no reconoce a Skipf
	// como sentencia terminal, y una Compuerta vacía que igual se devolviera no podría construir
	// ningún comando: sus campos son privados y su `t` sería nil.
	return Compuerta{}
}

// ═════════════════════════════════════════════════════════════════════════════════════════════
// A127 · LA COMPUERTA CONSTRUYE EL COMANDO, Y POR ESO YA NO HAY QUE ADIVINAR QUÉ ES UNA SHELL
//
// Hasta acá la compuerta sólo GATEABA, y una guarda de AST se encargaba de comprobar la
// equivalencia en los dos sentidos: quien ejecuta una shell llama a la compuerta, y quien llama a
// la compuerta ejecuta una shell. Las dos mitades apoyaban en la misma pregunta —«¿ESTA llamada
// arranca una shell?»— contestada mirando el árbol sintáctico, y esa pregunta NO CONVERGE: hay
// infinitas maneras de escribir el nombre de un programa en Go. Una ronda cerró once formas y
// aparecieron catorce. No fue falta de cuidado; estaba mal planteada.
//
// LO QUE CAMBIA NO ES LA RESPUESTA SINO CUÁNDO SE PREGUNTA. En el momento de EJECUTAR, el nombre
// del programa ya no es sintaxis: es un string. Ahí «¿es una shell?» se contesta mirándolo, y da
// lo mismo que haya llegado de un literal, de un `const` de paquete, de un campo de struct, de
// una concatenación o del valor de retorno de otra función. Las catorce formas y las que nadie
// escribió todavía colapsan en una sola.
//
// Y LA OTRA MITAD SE VUELVE ESTRUCTURAL: el `*exec.Cmd` de una shell sólo se consigue como método
// de lo que la compuerta DEVUELVE. No hay forma de tener uno sin haberla llamado antes — no
// porque una guarda lo vigile, sino porque no hay dónde escribirlo. Un `if false { Exigir(…) }`
// deja de ser una fuga y pasa a ser un error de compilación: adentro del `if` no hay nada que
// nombre a la variable de afuera.
//
// LO QUE QUEDA PARA LA GUARDA ES UNA SOLA PREGUNTA, y es sintáctica de verdad: ¿alguna prueba
// nombra `exec.Command` o `exec.CommandContext`? Eso no pide interpretar ningún argumento. La
// lista de maneras de lanzar un proceso en Go SÍ es cerrada —es un hecho de la stdlib, no una
// forma de escribir un string— y está medida: en este repo no hay `os.StartProcess`, ni
// `syscall.Exec`, ni literales `exec.Cmd{}`.
// ═════════════════════════════════════════════════════════════════════════════════════════════

// LasShells son los programas que EJECUTAN UN GUION, y la lista es un hecho del mundo: se clava,
// no se deriva de nada de este repo. Que esté acá escrita a mano no la vuelve una enumeración de
// las que no convergen — aquéllas enumeran FORMAS DE ESCRIBIR un nombre en Go, que son infinitas;
// ésta enumera INTÉRPRETES DE GUIONES, que son los que son.
//
// Se compara contra el nombre base, sin `.exe` y en minúsculas, así que `/bin/bash`,
// `C:\Windows\System32\cmd.exe` y `BASH` caen en el mismo lugar.
//
// NO PASES ESTA LISTA POR `strings.Join` EN UN MENSAJE, por más que tiente. La guarda de
// PowerShell (`TestNingunPowerShellDeDeployEscapaComillasConBarra`) expande los slices de cadenas
// a sus elementos para poder leer un `exec.Command("powershell", args...)`, así que una llamada
// que reciba esta lista se le parece a una invocación con argv: encuentra el literal
// `"powershell"` seguido de más argumentos, no halla ningún `-Command` y acusa «NO PUDE LEER EL
// GUION». Medido acá el 2026-09-20. Su red es ancha A PROPÓSITO —prefiere un falso positivo
// ruidoso a un falso negativo mudo, y en eso tiene razón—, así que el que cede es este mensaje:
// nombra el identificador en vez de enumerar.
var LasShells = []string{
	"ash", "bash", "busybox", "cmd", "csh", "dash", "fish", "ksh",
	"powershell", "pwsh", "sh", "tcsh", "zsh",
}

// EsShell contesta si `nombre` es un intérprete de guiones, mirando el VALOR y no cómo se escribió.
func EsShell(nombre string) bool {
	base := strings.ToLower(filepath.Base(filepath.ToSlash(nombre)))
	base = strings.TrimSuffix(base, ".exe")
	for _, s := range LasShells {
		if base == s {
			return true
		}
	}
	return false
}

// Compuerta es lo que devuelven Exigir, Portable y Unix: el permiso ya cobrado de ejecutar un
// guion, y —esto es lo que cierra A127— lo ÚNICO que sabe construir el comando que lo ejecuta.
//
// Se pasa por valor a propósito: una prueba que gatea arriba y corre el guion adentro de un
// helper, de un `t.Run` o de una tabla le pasa esta Compuerta como cualquier otro argumento. Eso
// vuelve innecesario el análisis de dominancia que hacía la guarda: no hay forma de ejecutar la
// shell «afuera» del gateo, porque la Compuerta ES el gateo y viaja con el comando.
type Compuerta struct {
	t          testing.TB
	modo       string
	motivo     string
	declaradas []string
	usada      *bool
}

// Comando construye el `*exec.Cmd` que ejecuta el guion, después de comprobar sobre el valor real
// dos cosas que antes se adivinaban en el árbol sintáctico:
//
//   - que `nombre` ES una shell — si no lo es, esto no es lo que buscabas y va `guiones.Herramienta`;
//   - que esa shell FUE DECLARADA en la compuerta, que es la equivalencia que la guarda de alcance
//     perseguía por AST. Acá se contesta con el string en la mano.
//
// No corre nada: devuelve el `*exec.Cmd` sin arrancar, así que quien lo llama sigue poniéndole
// `Env`, `Dir` o `Stdin` y eligiendo entre `Run`, `Output` y `CombinedOutput` como siempre.
func (c Compuerta) Comando(nombre string, args ...string) *exec.Cmd {
	c.t.Helper()
	c.exigirDeclarada(nombre)
	return exec.Command(nombre, args...)
}

// ComandoCtx es Comando con un contexto, para los guiones que se cancelan por timeout.
func (c Compuerta) ComandoCtx(ctx context.Context, nombre string, args ...string) *exec.Cmd {
	c.t.Helper()
	c.exigirDeclarada(nombre)
	return exec.CommandContext(ctx, nombre, args...)
}

func (c Compuerta) exigirDeclarada(nombre string) {
	c.t.Helper()

	if !EsShell(nombre) {
		c.t.Fatalf("guiones.%s(…).Comando(%q, …): %q NO es una shell, y esta compuerta existe sólo "+
			"para las pruebas que EJECUTAN un guion.\n"+
			"  Si lo que querés es correr una herramienta cualquiera (`git`, `go`, el propio binario "+
			"de prueba), usá `guiones.Herramienta(t, %q, …)`, que no gatea nada y no le miente a "+
			"nadie sobre el alcance de esta prueba.\n"+
			"  Las que cuentan como shell están en `guiones.LasShells`.",
			c.modo, nombre, nombre, nombre)
	}

	for _, d := range c.declaradas {
		if strings.EqualFold(d, nombre) {
			*c.usada = true
			return
		}
	}
	c.t.Fatalf("guiones.%s(…).Comando(%q, …): la compuerta NO declaró %q entre sus herramientas "+
		"(declaró: %s).\n"+
		"  ÉSTA ES LA EQUIVALENCIA QUE LA COMPUERTA CUSTODIA, y acá se comprueba sobre el valor y no "+
		"sobre cómo se escribió: lo que la compuerta promete es que en esta plataforma existe lo que "+
		"la prueba va a usar, y si ejecutás algo que no declaró, esa promesa no cubre nada.\n"+
		"  MOTIVO DECLARADO: %s",
		c.modo, nombre, nombre, strings.Join(c.declaradas, ", "), c.motivo)
}

// vigilarQueSeUse es la otra mitad de la equivalencia —quien llama a la compuerta tiene que
// ejecutar un guion— y también se mudó del AST al runtime, por el mismo motivo: «¿este cuerpo
// ejecuta una shell?» no se puede contestar mirando la sintaxis, y al final de la prueba se
// contesta sola.
//
// SÓLO SE REGISTRA EN LOS CAMINOS QUE DEVUELVEN. Si la compuerta saltea, `t.Skipf` sale por
// `runtime.Goexit` y los cleanups corren igual: registrarla antes acusaría a toda prueba salteada
// de no haber usado la compuerta, que es exactamente lo que el salteo significa.
func vigilarQueSeUse(t testing.TB, modo, motivo string, herramientas []string) Compuerta {
	usada := false
	t.Cleanup(func() {
		// Una prueba que ya falló o que se salteó por su cuenta no necesita este reproche encima:
		// sería ruido sobre un rojo que alguien ya está leyendo.
		if usada || t.Failed() || t.Skipped() {
			return
		}
		t.Errorf("guiones.%s se llamó y NINGÚN guion se ejecutó por esta compuerta.\n"+
			"  MOTIVO DECLARADO: %s\n"+
			"  Esta compuerta NO es un t.Skip de propósito general: declara que la prueba corre un "+
			"guion de shell, y eso acota su alcance a las plataformas donde el arnés se sostiene. "+
			"Usada sin ejecutar nada, apaga cobertura en Windows y en macOS a cambio de nada.\n"+
			"  Si la prueba sí corre un guion, hacelo con el `*exec.Cmd` que devuelve esta compuerta "+
			"(`c := guiones.%s(…)` y después `c.Comando(\"bash\", …)`). Si no corre ninguno, sacá la "+
			"compuerta.", modo, motivo, modo)
	})
	return Compuerta{t: t, modo: modo, motivo: motivo, declaradas: herramientas, usada: &usada}
}

// Herramienta construye el `*exec.Cmd` de un programa que NO es una shell —`git`, `go`, `df`, el
// propio binario de prueba— sin gatear nada, porque no hay nada que gatear: esos programas no
// traen el problema de plataforma que trajo esta compuerta.
//
// EXISTE PARA QUE LA GUARDA PUEDA PREGUNTAR UNA SOLA COSA. Si las pruebas que corren `git`
// siguieran llamando a `exec.Command` directo, la guarda tendría que distinguir cuáles de esas
// llamadas son shells —y volveríamos a la pregunta que no converge—. Con esto, TODA prueba que
// lanza un proceso pasa por este paquete, y la guarda sólo mira si alguien nombró `exec.Command`.
//
// Y COMPRUEBA LO CONTRARIO QUE Comando: si el programa resulta SER una shell, muere. Esa es la
// mitad que impide que esto se use como puerta de atrás para saltarse la compuerta, y se contesta
// sobre el valor, así que no la burla ningún `const` de paquete.
func Herramienta(t testing.TB, nombre string, args ...string) *exec.Cmd {
	t.Helper()
	exigirQueNoSeaShell(t, nombre)
	return exec.Command(nombre, args...)
}

// HerramientaCtx es Herramienta con un contexto.
func HerramientaCtx(ctx context.Context, t testing.TB, nombre string, args ...string) *exec.Cmd {
	t.Helper()
	exigirQueNoSeaShell(t, nombre)
	return exec.CommandContext(ctx, nombre, args...)
}

func exigirQueNoSeaShell(t testing.TB, nombre string) {
	t.Helper()
	if EsShell(nombre) {
		t.Fatalf("guiones.Herramienta(t, %q, …): %q ES una shell, y ejecutar un guion tiene que "+
			"pasar por la compuerta.\n"+
			"  Esto no es un detalle de estilo: el arnés de estas pruebas es de Unix —escribe stubs "+
			"con shebang y los antepone al PATH separando con `:`— y en Windows NO se sostiene. Una "+
			"shell lanzada por acá correría ahí y mediría el runner en vez del guion.\n"+
			"  Declaralo: `c := guiones.Exigir(t, motivo, %q, …)` y después `c.Comando(%q, …)`.",
			nombre, nombre, nombre, nombre)
	}
}
