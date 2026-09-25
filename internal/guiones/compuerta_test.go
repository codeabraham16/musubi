package guiones

// compuerta_test.go — lo que impide que esta compuerta se convierta en un `t.Skip` de propósito
// general, que es la forma exacta en que este repo perdió antes.
//
// Un `t.Skip` apaga una guarda entera y deja `PASS`. Acá hay tres guardas y ninguna pregunta por
// un texto:
//
//  1. TestLaCompuertaNuncaSalteaEnLinux — la CORRE y comprueba que la línea de después se ejecutó.
//     Si alguien le agrega una condición que saltee en linux, esto se pone rojo en linux.
//  2. TestUnUsoIndebidoDeLaCompuertaEsUnFallo — corre los usos prohibidos en un PROCESO APARTE y
//     exige que ese proceso FALLE. Sin esto, los dos `t.Fatalf` de uso serían una promesa escrita.
//  3. TestElSalteoEstaAcotadoALasPruebasQueCorrenUnaShell — arma el grafo de llamadas de cada
//     paquete de prueba del repo y exige la equivalencia en los dos sentidos.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/arbol"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// (0) DÓNDE MIDE CADA MODO, CLAVADO A MANO. Es el único lugar del paquete donde se escribe.
// ─────────────────────────────────────────────────────────────────────────────────────────────

// TestLosSistemasDondeCadaModoMideSonLosQueSeDeclaran — la guarda que las dos sondas NO pueden ser.
//
// EL PROBLEMA, Y ES UNA TRAMPA FINA. Las sondas de más abajo preguntan «¿salteó donde no debía?»
// usando `ElSistemaDondeCorren` y `LosSistemasDondeElArnesSeSostiene` — LAS MISMAS constantes que
// la compuerta usa para decidir. O sea que derivan su expectativa de su propio sujeto: si alguien
// mueve la constante, la sonda mueve la pregunta con ella y sigue en verde.
//
// MEDIDO, y el agujero era mío: poniendo `LosSistemasDondeElArnesSeSostiene = []string{"windows"}`,
// `TestLaCompuertaUnixNoSalteaDondeElArnesSeSostiene` queda VERDE en linux —cree que acá tiene que
// saltear, y saltea— mientras las cuatro pruebas de `internal/mcp` que corren los guiones de
// deploy empiezan a SALTEARSE de verdad. La cobertura se apaga entera y nadie se entera.
//
// LA SALIDA NO ES DERIVARLO MEJOR: ES CLAVARLO. «Los guiones de deploy/ corren en un servidor
// Linux» y «el arnés de stubs se sostiene en linux y en macOS pero no en Windows» son HECHOS del
// mundo, no valores calculados. Un hecho se escribe una vez, a mano, en el lugar donde cambiarlo
// obliga a justificarlo — y todo lo demás se deriva de ahí. Es la otra cara de «un derivado
// escrito a mano es una copia»: el problema no era escribir a mano, era escribir a mano lo que se
// deduce. Esto no se deduce de nada.
//
// Sabotaje que la pone roja: mover cualquiera de las dos constantes.
// arnes: archivo="internal/guiones/compuerta.go"
// arnes: de="const ElSistemaDondeCorren = \"linux\""
// arnes: a="const ElSistemaDondeCorren = \"darwin\""
func TestLosSistemasDondeCadaModoMideSonLosQueSeDeclaran(t *testing.T) {
	if ElSistemaDondeCorren != "linux" {
		t.Errorf("ElSistemaDondeCorren es %q y tiene que ser \"linux\".\n"+
			"  Lo que `guiones.Exigir` custodia son pruebas que CORREN los instaladores de "+
			"deploy/, y lo que esos instaladores dejan son unidades systemd de un servidor Linux, "+
			"con useradd y /etc. Moviendo esta constante, las sondas de abajo mueven su expectativa "+
			"con ella y siguen en verde mientras la cobertura se apaga entera.", ElSistemaDondeCorren)
	}

	quiere := map[string]bool{"linux": true, "darwin": true}
	visto := map[string]bool{}
	for _, s := range LosSistemasDondeElArnesSeSostiene {
		visto[s] = true
	}
	if visto["windows"] {
		t.Error("LosSistemasDondeElArnesSeSostiene incluye \"windows\", y ahí el arnés NO se sostiene: " +
			"está medido en CI que el `:` de `C:\\` parte el PATH, que un stub sin `.exe` no se " +
			"ejecuta, y que por eso el guion SALIÓ A LA URL REAL del release (exit 22 de curl). " +
			"Declararlo medible ahí pone en rojo un job por algo que no es el producto.")
	}
	for s := range quiere {
		if !visto[s] {
			t.Errorf("LosSistemasDondeElArnesSeSostiene NO incluye %q, y tiene que incluirlo.\n"+
				"  linux es donde corren los guiones y donde gatea el merge; darwin es donde vive el "+
				"defecto que estas pruebas cazan —el `${VAR}` pegada a un carácter no-ASCII que mata "+
				"el guion en bash 3.2 y en Linux es INVISIBLE—. Sacar darwin de acá cierra el defecto "+
				"y apaga a su único testigo en el mismo commit.", s)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// (1) EN LINUX NO SALTEA. No se le cree al `if`: se corre.
// ─────────────────────────────────────────────────────────────────────────────────────────────

// TestLaCompuertaNuncaSalteaEnLinux — la sonda.
//
// `t.Skipf` hace `runtime.Goexit()`: si la compuerta saltea, la línea siguiente NO se ejecuta.
// Entonces «¿salteó?» no se pregunta por el texto del mensaje ni por `t.Skipped()` —que además
// cuenta como éxito en `go test`— sino por un DATO de la corrida: si el cuerpo siguió.
//
// SABOTAJE QUE LA PONE EN ROJO (verificado): cambiar el `if runtime.GOOS == ElSistemaDondeCorren`
// de compuerta.go por `if false`, o meterle un `t.Skip` antes del `if`. Las dos cosas dejan a
// `siguio` en false y esto muere en linux.
//
// Y ESTA PRUEBA TAMBIÉN EJECUTA UNA SHELL a propósito (`bash -c :`): así queda del lado correcto
// de la guarda de alcance de más abajo en vez de necesitar una excepción, y de paso comprueba que
// lo que la compuerta acaba de declarar presente se puede ejecutar de verdad.
// arnes: archivo="internal/guiones/compuerta.go"
// arnes: de="\tif runtime.GOOS == ElSistemaDondeCorren {"
// arnes: a="\tif runtime.GOOS == ElSistemaDondeCorren && false {"
func TestLaCompuertaNuncaSalteaEnLinux(t *testing.T) {
	siguio := false
	t.Run("sonda", func(t *testing.T) {
		c := Exigir(t, "la sonda de la propia compuerta ejecuta bash para comprobar que no saltea", "bash")
		siguio = true
		if err := c.Comando("bash", "-c", ":").Run(); err != nil {
			t.Fatalf("la compuerta dejó pasar y bash no se pudo ejecutar: %v", err)
		}
	})

	if runtime.GOOS == ElSistemaDondeCorren {
		if !siguio {
			t.Fatalf("guiones.Exigir SALTEÓ en %s. En %s no puede saltear NUNCA: acá es donde los "+
				"guiones de deploy/ corren de verdad, y un salteo silencioso apaga todas las pruebas "+
				"de despliegue dejando `go test` en `ok`. Si falta una herramienta, tiene que MORIR.",
				runtime.GOOS, ElSistemaDondeCorren)
		}
		return
	}
	// Fuera de linux el contrato es el opuesto, y también se comprueba: si un día la compuerta
	// dejara pasar en windows, el arnés saldría a internet como salió (exit 22 de curl) en vez de
	// saltear, y esta prueba lo dice acá y no en el log de siete pruebas ajenas.
	if siguio {
		t.Fatalf("guiones.Exigir DEJÓ PASAR en %s/%s y la compuerta existe justamente porque el arnés "+
			"no se sostiene ahí.", runtime.GOOS, runtime.GOARCH)
	}
}

// elArnesSeSostieneAca contesta si ESTE sistema es uno de los que `Unix` declara medibles.
//
// Se deriva de `LosSistemasDondeElArnesSeSostiene` y no se tipea la lista de vuelta: una copia
// escrita a mano acá diría «darwin mide» el día que la compuerta dejara de medir en darwin, y la
// prueba pasaría en verde sobre la contradicción.
func elArnesSeSostieneAca() bool {
	for _, sistema := range LosSistemasDondeElArnesSeSostiene {
		if runtime.GOOS == sistema {
			return true
		}
	}
	return false
}

// TestLaCompuertaUnixNoSalteaDondeElArnesSeSostiene — la sonda del tercer modo, en los dos sentidos.
//
// LO QUE ESTÁ EN JUEGO, y es la razón de que `Unix` exista: las pruebas que corren
// `verificar-despliegue.sh` cazan defectos que en Linux SON INVISIBLES —el `${VAR}` pegada a un
// carácter no-ASCII que mata el guion en el bash 3.2 de macOS: había cinco y cuatro llevaban meses
// sin verse—. Si `Unix` salteara en darwin, el defecto quedaría cerrado y su única plataforma
// testigo apagada en el mismo commit. Por eso la sonda mide darwin con el mismo rigor que linux.
//
// SE PREGUNTA POR UN DATO DE LA CORRIDA, igual que la sonda de `Exigir`: `t.Skipf` hace
// `runtime.Goexit()`, así que «¿salteó?» se contesta mirando si el cuerpo siguió — no el texto del
// mensaje, y no `t.Skipped()`, que en `go test` cuenta como éxito.
//
// SABOTAJE QUE LA PONE EN ROJO (verificado): en compuerta.go, cambiar el `for` sobre
// `LosSistemasDondeElArnesSeSostiene` por `if runtime.GOOS == ElSistemaDondeCorren`. En linux
// sigue verde y en macOS muere — que es exactamente el defecto que este modo vino a evitar.
// arnes: archivo="internal/guiones/compuerta.go"
// arnes: de="\t\tif runtime.GOOS != sistema {"
// arnes: a="\t\tif runtime.GOOS != sistema || true {"
func TestLaCompuertaUnixNoSalteaDondeElArnesSeSostiene(t *testing.T) {
	siguio := false
	t.Run("sonda", func(t *testing.T) {
		c := Unix(t, "la sonda del modo Unix ejecuta bash para comprobar que no saltea ni en linux ni en macOS", "bash")
		siguio = true
		if err := c.Comando("bash", "-c", ":").Run(); err != nil {
			t.Fatalf("la compuerta dejó pasar y bash no se pudo ejecutar: %v", err)
		}
	})

	if elArnesSeSostieneAca() {
		if !siguio {
			t.Fatalf("guiones.Unix SALTEÓ en %s. Los sistemas donde declara medir son %s, y en ésos "+
				"no puede saltear NUNCA: una herramienta que falta es un FALLO, porque «no pude "+
				"medir» no puede contestar lo mismo que «medí y está bien».",
				runtime.GOOS, strings.Join(LosSistemasDondeElArnesSeSostiene, " y "))
		}
		return
	}
	// En windows el contrato es el opuesto y también se comprueba: si un día dejara pasar ahí, el
	// arnés saldría a internet (exit 22 de curl) en vez de saltear.
	if siguio {
		t.Fatalf("guiones.Unix DEJÓ PASAR en %s/%s, donde el arnés no se sostiene: el `:` de `C:\\` "+
			"parte el PATH y el stub sin `.exe` no se ejecuta.", runtime.GOOS, runtime.GOARCH)
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// (2) LOS DOS USOS PROHIBIDOS ROMPEN DE VERDAD
// ─────────────────────────────────────────────────────────────────────────────────────────────

// La variable con la que el proceso hijo sabe qué uso indebido tiene que intentar.
const sobreUsoIndebido = "MUSUBI_PRUEBA_USO_INDEBIDO"

// TestAyudanteDeUsoIndebido no es una prueba: es el cuerpo que corre el proceso hijo de
// TestUnUsoIndebidoDeLaCompuertaEsUnFallo. Sin la variable de entorno no hace nada, así que en una
// corrida normal de `go test` pasa sin tocar la compuerta.
func TestAyudanteDeUsoIndebido(t *testing.T) {
	switch os.Getenv(sobreUsoIndebido) {
	case "sin-motivo":
		Exigir(t, "porque si", "bash")
	case "sin-herramientas":
		Exigir(t, "un motivo perfectamente redactado y de largo suficiente para pasar el filtro")
	case "unix-sin-motivo":
		Unix(t, "porque si", "bash")
	case "unix-sin-herramientas":
		Unix(t, "un motivo perfectamente redactado y de largo suficiente para pasar el filtro")
	// LAS DOS PUNTAS DE LA LISTA DE HERRAMIENTAS. Los llamadores reales declaran de tres a seis
	// (`"bash", "git", "python3", "od"`), y las sondas de arriba declaran UNA sola: con una lista
	// de largo 1, recorrerla entera y recorrer `herramientas[1:]` son indistinguibles. Estos dos
	// casos ponen la herramienta inexistente en cada extremo, así que saltear cualquiera de las
	// dos puntas se ve.
	case "falta-la-primera":
		Exigir(t, "declara una herramienta inexistente en primer lugar para que saltearse el principio de la lista se vea", "musubi-herramienta-inexistente-zz", "bash")
	case "falta-la-ultima":
		Exigir(t, "declara una herramienta inexistente en último lugar para que saltearse el final de la lista se vea", "bash", "musubi-herramienta-inexistente-zz")
	case "unix-falta-la-primera":
		Unix(t, "declara una herramienta inexistente en primer lugar para que saltearse el principio de la lista se vea", "musubi-herramienta-inexistente-zz", "bash")
	case "unix-falta-la-ultima":
		Unix(t, "declara una herramienta inexistente en último lugar para que saltearse el final de la lista se vea", "bash", "musubi-herramienta-inexistente-zz")
	// LOS CUATRO DE A127. La equivalencia que antes se perseguía por AST —quien ejecuta una shell
	// pasa por la compuerta, y quien pasa por la compuerta ejecuta una shell— hoy se contesta acá,
	// sobre el valor y al final de la prueba. Si estos `t.Fatalf` fueran decorativos, la mudanza
	// del AST al runtime habría cambiado una pregunta que no converge por ninguna pregunta.
	case "comando-no-es-shell":
		c := Exigir(t, "le pide a la compuerta un comando que no es una shell, que es para lo que existe Herramienta", "bash")
		c.Comando("git", "status")
	case "comando-no-declarada":
		c := Exigir(t, "ejecuta una shell que la compuerta nunca declaró, así que su promesa no cubre nada", "bash")
		c.Comando("sh", "-c", ":")
	case "herramienta-es-shell":
		Herramienta(t, "bash", "-c", ":")
	case "compuerta-sin-usar":
		Exigir(t, "llama a la compuerta y no ejecuta ningún guion, que es el t.Skip disfrazado", "bash")
	default:
		// En una corrida normal pasa por la compuerta y ejecuta una shell como cualquiera de las
		// pruebas que ésta custodia. No es adorno: así queda del lado correcto de la guarda de
		// alcance de más abajo SIN NECESITAR UNA EXCEPCIÓN, y una lista de excepciones es
		// exactamente por donde una guarda como ésta se vacía con el tiempo.
		c := Exigir(t, "el ayudante ejecuta bash igual que las pruebas que esta compuerta custodia", "bash")
		if err := c.Comando("bash", "-c", ":").Run(); err != nil {
			t.Fatalf("no se pudo ejecutar bash: %v", err)
		}
	}
}

// TestUnUsoIndebidoDeLaCompuertaEsUnFallo — que los dos `t.Fatalf` de uso no sean decorativos.
//
// POR QUÉ UN PROCESO APARTE: un `t.Fatalf` sobre el `*testing.T` de acá haría fallar a ESTA
// prueba, y no hay forma de fabricar un `*testing.T` de mentira. El patrón del proceso hijo es el
// que ya usa `cmd/musubi/cerebro_test.go`.
//
// SE MIDE EL CÓDIGO DE SALIDA, no el texto del mensaje: el texto se comprueba sólo DESPUÉS de
// saber que falló, para que un mensaje bien redactado sobre un `PASS` no pueda contestar que sí.
// Y CADA CASO EXIGE QUE FALLE EN CUALQUIER PLATAFORMA: el uso indebido no puede quedar en un
// `skip` cómodo en windows.
func TestUnUsoIndebidoDeLaCompuertaEsUnFallo(t *testing.T) {
	// `mide` dice EN QUÉ PLATAFORMAS este caso tiene que hacer morir al hijo.
	//
	// LOS DOS CONTROLES DE USO —motivo pobre, cero herramientas— MUEREN EN LAS TRES, a propósito:
	// corren antes de mirar el GOOS, para que nadie use la compuerta como un `t.Skip` cómodo en la
	// plataforma que le molesta. Los CUATRO de la lista de herramientas, no: la comprobación de
	// que las herramientas están sólo corre donde el modo mide, y donde no mide la compuerta
	// SALTEA — que es salir con código 0. Afirmar que mueren en las tres es afirmar como universal
	// algo que es condicional, y CI lo cobró: rojo en windows y en macOS.
	//
	// ACÁ SÍ SE DERIVA DE LAS CONSTANTES, y no contradice a
	// `TestLosSistemasDondeCadaModoMideSonLosQueSeDeclaran`: lo que esta prueba mide es el RECORRIDO
	// de la lista, no en qué plataformas mide cada modo. Esa segunda pregunta está clavada a mano
	// allá, una sola vez. Derivar de un hecho ya clavado es derivar; clavarlo de nuevo acá sería la
	// copia.
	siempre := func() bool { return true }
	dondeMideExigir := func() bool { return runtime.GOOS == ElSistemaDondeCorren }
	dondeMideUnix := elArnesSeSostieneAca

	casos := []struct {
		nombre, valor, esperado, porque string
		mide                            func() bool
	}{
		{"motivo vacío de contenido", "sin-motivo", "hace falta una frase",
			"un motivo de dos palabras pasó: el salteo puede quedar sin decir por qué, y un salteo " +
				"que nadie puede revisar se queda para siempre", siempre},
		{"sin nombrar ninguna herramienta", "sin-herramientas", "NO es",
			"la compuerta aceptó ser usada sin nombrar una sola herramienta Unix, o sea como un " +
				"t.Skip de propósito general: es exactamente lo que no puede ser", siempre},
		// EL HERMANO: los dos controles de uso valen para los TRES modos o no valen para ninguno.
		// Un modo nuevo con los `t.Fatalf` copiados y sin prueba que los corra es la forma exacta
		// que este repo persigue —la guarda puesta en N-1 de N caminos—, y acá el N acaba de subir.
		{"modo Unix, motivo vacío de contenido", "unix-sin-motivo", "hace falta una frase",
			"guiones.Unix aceptó un motivo de dos palabras: el salteo de windows puede quedar sin " +
				"decir por qué, y un salteo que nadie puede revisar se queda para siempre", siempre},
		{"modo Unix, sin nombrar ninguna herramienta", "unix-sin-herramientas", "NO es",
			"guiones.Unix aceptó ser usada sin nombrar una sola herramienta, o sea como un t.Skip " +
				"de propósito general para apagar windows: es exactamente lo que no puede ser", siempre},
		// EL CONTROL DE QUE LA LISTA SE MIRA ENTERA. Sin estos cuatro, un `herramientas[1:]` o un
		// `herramientas[:len(herramientas)-1]` pasan inadvertidos: las sondas declaran UNA sola
		// herramienta y con largo 1 las tres formas de recorrer se ven iguales. Los llamadores
		// reales declaran hasta seis, así que la ceguera sería sobre lo que de verdad se usa.
		{"Exigir saltea el principio de la lista", "falta-la-primera", "musubi-herramienta-inexistente-zz",
			"guiones.Exigir no miró la PRIMERA herramienta declarada. Una que falta tiene que MORIR en linux; " +
				"si se la saltea, la prueba corre sin lo que necesita y su verde no mide el guion", dondeMideExigir},
		{"Exigir saltea el final de la lista", "falta-la-ultima", "musubi-herramienta-inexistente-zz",
			"guiones.Exigir no miró la ÚLTIMA herramienta declarada, y las listas reales llegan a seis", dondeMideExigir},
		{"Unix saltea el principio de la lista", "unix-falta-la-primera", "musubi-herramienta-inexistente-zz",
			"guiones.Unix no miró la PRIMERA herramienta declarada", dondeMideUnix},
		{"Unix saltea el final de la lista", "unix-falta-la-ultima", "musubi-herramienta-inexistente-zz",
			"guiones.Unix no miró la ÚLTIMA herramienta declarada", dondeMideUnix},
		// A127 · LAS DOS MITADES DE LA EQUIVALENCIA, YA NO POR AST SINO SOBRE EL VALOR.
		{"la compuerta construye algo que no es una shell", "comando-no-es-shell", "NO es una shell",
			"la compuerta construyó un comando que no es una shell. Es la mitad que impide que se " +
				"la use para acotar a linux una prueba que no corre ningún guion", dondeMideExigir},
		{"la compuerta construye una shell que no declaró", "comando-no-declarada", "NO declaró",
			"la compuerta ejecutó una shell que no estaba en sus herramientas. Lo que promete es " +
				"que en esta plataforma existe lo que la prueba usa, y así no cubre nada", dondeMideExigir},
		{"el ayudante sin compuerta acepta una shell", "herramienta-es-shell", "ES una shell",
			"guiones.Herramienta dejó lanzar una shell sin gatear: sería la puerta de atrás para " +
				"saltarse la compuerta, y la dejaría existiendo de adorno", siempre},
		{"la compuerta se llama y no se usa", "compuerta-sin-usar", "NINGÚN guion se ejecutó",
			"se llamó a la compuerta sin ejecutar ningún guion y nadie dijo nada: es el t.Skip de " +
				"propósito general disfrazado, que apaga windows y macOS a cambio de nada", dondeMideExigir},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			cmd := Herramienta(t, os.Args[0], "-test.run=^TestAyudanteDeUsoIndebido$", "-test.v")
			cmd.Env = append(os.Environ(), sobreUsoIndebido+"="+c.valor)
			salida, err := cmd.CombinedOutput()

			// DONDE ESTE MODO NO MIDE, EL CONTRATO ES EL OPUESTO Y TAMBIÉN SE COMPRUEBA: la
			// compuerta tiene que SALTEAR diciéndolo, no dejar pasar. Un `err == nil` a secas acá
			// dejaría el caso vacío en windows —«pasó», sin saber si salteó o si corrió sin las
			// herramientas—, que es medio archivo de prueba sin medir nada.
			if !c.mide() {
				if err != nil {
					t.Fatalf("en %s/%s este modo NO mide, así que la compuerta tenía que SALTEAR, y el "+
						"hijo murió:\n%s", runtime.GOOS, runtime.GOARCH, salida)
				}
				if !strings.Contains(string(salida), "SALTEADA EN") {
					t.Errorf("en %s/%s el hijo pasó sin saltear: o la compuerta dejó pasar donde el "+
						"arnés no se sostiene, o corrió sin las herramientas que declaró.\n%s",
						runtime.GOOS, runtime.GOARCH, salida)
				}
				return
			}

			if err == nil {
				t.Fatalf("%s\n  el proceso hijo terminó con código 0\n  salida:\n%s", c.porque, salida)
			}
			if !strings.Contains(string(salida), c.esperado) {
				t.Errorf("el hijo falló, pero no por lo que tenía que fallar: no encontré %q en la "+
					"salida. Puede estar muriendo por otra cosa y esta prueba estaría verde por "+
					"casualidad.\n%s", c.esperado, salida)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// (3) EL ALCANCE: NINGUNA PRUEBA LANZA UN PROCESO POR FUERA DE ESTE PAQUETE
// ─────────────────────────────────────────────────────────────────────────────────────────────

// losLanzadores son las funciones de `os/exec` que CONSTRUYEN un proceso. La lista es corta
// porque la stdlib es la que es, y está MEDIDA sobre este repo: no hay `os.StartProcess`, ni
// `syscall.Exec`, ni literales `exec.Cmd{}` en ninguna prueba.
//
// ES UNA ENUMERACIÓN Y ACÁ SÍ CONVERGE, que es la distinción que costó A127. Enumerar «las formas
// de escribir el nombre de una shell en Go» no converge: son infinitas, y una ronda que cerró
// once destapó catorce. Enumerar «las funciones de la stdlib que lanzan un proceso» sí: es un
// hecho del mundo, no una sintaxis. Si Go agrega una, la agrega una vez y para siempre.
var losLanzadores = []string{"Command", "CommandContext"}

// TestNingunaPruebaLanzaUnProcesoPorFueraDeLaCompuerta — la guarda de alcance, y ahora pregunta
// UNA SOLA COSA.
//
// LO QUE HABÍA ANTES, Y POR QUÉ NO PODÍA FUNCIONAR. Esta guarda armaba el grafo de llamadas de
// cada paquete y exigía la equivalencia en los dos sentidos: quien ejecuta una shell llama a la
// compuerta, y quien llama a la compuerta ejecuta una shell. Las dos mitades apoyaban en la misma
// pregunta —«¿ESTA llamada arranca una shell?»— contestada sobre el árbol sintáctico, y esa
// pregunta no converge. Tres fugas quedaron medidas con control positivo: la compuerta adentro de
// un `if false`, la shell nombrada por un `const` de paquete, y la shell adentro de una tabla
// `map[string]func(*testing.T)`. La primera se cerró enseñándole DOMINANCIA; las otras dos no,
// porque la salida no era la forma número N+1.
//
// LO QUE CAMBIÓ NO ES LA RESPUESTA SINO DÓNDE VIVE LA PREGUNTA (ver el bloque A127 de
// compuerta.go). Hoy el `*exec.Cmd` de una shell sólo se consigue como método de lo que la
// compuerta DEVUELVE, así que «¿esto pasó por la compuerta?» ya no se contesta: no hay dónde
// escribir un no. Y «¿esto es una shell?» se contesta en runtime, con el string en la mano, donde
// da igual si vino de un literal, de un `const`, de un campo o de una concatenación.
//
// A ESTA GUARDA LE QUEDA LO QUE SÍ ES SINTÁCTICO: que nadie nombre `exec.Command` en una prueba.
// No hay que interpretar ni un argumento. Y no hay excepciones que enumerar —ni siquiera para
// este paquete—, porque el ayudante vive en `compuerta.go`, que no es un `_test.go`.
//
// EL CONTROL QUE NO PUEDE ENMUDECER NO DEPENDE DEL ÁRBOL. Una guarda que espera CERO tiene el
// problema de que «medí y no hay» y «no pude medir» se escriben igual. Acá el reconocedor se
// prueba a sí mismo contra dos fuentes de mentira que viven en esta misma función: una que lanza
// procesos —tiene que encontrar los dos— y una que usa el ayudante —no tiene que encontrar
// ninguno—. Si el reconocedor se rompe, esa comprobación se cae aunque el repo esté impecable.
//
// Sabotaje verificado que la pone roja: devolverle a una prueba su `exec.Command` directo. Se
// eligió `firmador_e2e_test.go` porque SIGUE IMPORTANDO `os/exec` para un `LookPath`, así que la
// mutación COMPILA — un sabotaje que no compila da rojo por el build y no por esta guarda.
// arnes: archivo="internal/selfupdate/firmador_e2e_test.go"
// arnes: de="salida, err := compuerta.Comando(\"bash\", guion, \"0.140.0\", clave, dir).CombinedOutput()"
// arnes: a="salida, err := exec.Command(\"bash\", guion, \"0.140.0\", clave, dir).CombinedOutput()"
func TestNingunaPruebaLanzaUnProcesoPorFueraDeLaCompuerta(t *testing.T) {
	// EL RECONOCEDOR SE PRUEBA ANTES DE CREERLE, y contra fuentes que no salen del disco: si esto
	// pasara por el árbol real, un árbol vacío dejaría el control tan mudo como a la guarda.
	const fuenteQueLanza = `package x
import "os/exec"
func A() { _ = exec.Command("bash", "-c", ":") }
func B() { _ = exec.CommandContext(nil, "sh", "-c", ":") }
`
	const fuenteSana = `package x
import "musubi/internal/guiones"
func A(c guiones.Compuerta, t T) { _ = c.Comando("bash", "-c", ":"); _ = guiones.Herramienta(t, "git", "status") }
`
	ctrl := token.NewFileSet()
	for _, caso := range []struct {
		nombre    string
		fuente    string
		esperados int
	}{
		{"una fuente que lanza procesos", fuenteQueLanza, 2},
		{"una fuente que usa el ayudante", fuenteSana, 0},
	} {
		f, err := parser.ParseFile(ctrl, "control.go", caso.fuente, 0)
		if err != nil {
			t.Fatalf("el control %q no parsea: %v", caso.nombre, err)
		}
		if n := len(lanzamientosDirectos(ctrl, f)); n != caso.esperados {
			t.Fatalf("EL RECONOCEDOR DE ESTA GUARDA ESTÁ ROTO: sobre %s encontró %d lanzamiento(s) y "+
				"tenían que ser %d.\n"+
				"  Sin esto, un repo impecable y un reconocedor ciego dan el mismo verde. Arreglá "+
				"`lanzamientosDirectos` antes de creerle a nada de lo que sigue.",
				caso.nombre, n, caso.esperados)
		}
	}

	raiz := filepath.Join("..", "..")
	fset := token.NewFileSet()

	// El árbol lo dice git y no el directorio (A128): `.claude/worktrees/` son copias enteras del
	// repo, y contarlas haría que esta guarda acusara el mismo archivo muchas veces.
	pruebas, err := arbol.ConSufijo(raiz, "_test.go")
	if err != nil {
		t.Fatal(err)
	}

	var culpables []string
	archivos, usosDelAyudante := 0, 0
	for _, rel := range pruebas {
		ruta := filepath.Join(raiz, filepath.FromSlash(rel))
		f, e := parser.ParseFile(fset, ruta, nil, 0)
		if e != nil {
			t.Errorf("%s no parsea (%v): esta guarda no puede mirar lo que no puede leer, y un archivo "+
				"invisible acá es una prueba que lanza procesos y nadie le va a reclamar", ruta, e)
			continue
		}
		archivos++
		culpables = append(culpables, lanzamientosDirectos(fset, f)...)
		usosDelAyudante += contarUsosDelAyudante(f)
	}

	// CONTROL DE QUE MIRÓ ALGO.
	if archivos < 100 {
		t.Fatalf("se parsearon %d archivos `_test.go` y el repo tiene cientos: el recorrido dejó de "+
			"mirar y esta guarda estaría en verde sin haber comprobado nada", archivos)
	}

	// CONTROL DE QUE EL REFACTOR SIGUE EN PIE. No es el número exacto de hoy —eso obligaría a tocar
	// esta guarda cada vez que se agrega una prueba, y una guarda que estorba se apaga—; es la
	// comprobación de que las pruebas siguen lanzando procesos POR ACÁ. Si alguien revirtiera el
	// refactor y a la vez rompiera el reconocedor, este piso lo dice.
	if usosDelAyudante < 25 {
		t.Fatalf("sólo %d prueba(s) lanzan procesos por el ayudante de este paquete, y son más de "+
			"treinta. O el refactor de A127 se revirtió, o este barrido dejó de ver el árbol: en los "+
			"dos casos el cero de arriba no significa «no hay», significa «no pude medir».",
			usosDelAyudante)
	}
	sort.Strings(culpables)
	for _, x := range culpables {
		t.Errorf("UNA PRUEBA LANZA UN PROCESO POR FUERA DE LA COMPUERTA: %s\n"+
			"  Este repo tiene UN solo lugar donde una prueba construye un proceso, y es este\n"+
			"  paquete. No es estilo: mientras `exec.Command` esté disponible, la pregunta «¿esto\n"+
			"  arranca una shell?» hay que contestarla mirando la sintaxis, y ésa es exactamente la\n"+
			"  pregunta que no converge (A127: once formas cerradas, catorce aparecidas).\n"+
			"  Arreglo, y son dos casos:\n"+
			"    · ejecuta un GUION DE SHELL -> gatéalo y pedile el comando a la compuerta:\n"+
			"        c := guiones.Exigir(t, \"<qué guion corre y por qué es de linux>\", \"bash\", ...)\n"+
			"        c.Comando(\"bash\", ruta).CombinedOutput()\n"+
			"      (o `guiones.Unix` si también se mide en macOS, o `guiones.Portable` en las tres.)\n"+
			"    · ejecuta OTRA COSA (`git`, `go`, el propio binario de prueba) -> no hay nada que\n"+
			"      gatear, pero pasa igual por acá para que esta guarda pueda preguntar una sola cosa:\n"+
			"        guiones.Herramienta(t, \"git\", \"-C\", dir, \"status\")\n"+
			"      Si resulta ser una shell, se entera ahí mismo y en runtime.", x)
	}
	// EL CENSO SE IMPRIME DESPUÉS DE LAS ACUSACIONES, y no antes. Sus números CAMBIAN con el
	// sabotaje (un lanzamiento directo más, un uso del ayudante menos), así que el juez del arnés no
	// lo puede restar contra el control; impreso primero, era la primera línea del rojo y el motivo
	// salía de un `t.Logf` en vez de la aserción: «rojo sospechoso» en la primera corrida nocturna
	// completa (#652). Acá abajo sigue estando para quien lea la salida, sin hacerse pasar por ella.
	t.Logf("%d archivos de prueba parseados, %d lanzamientos por el ayudante, %d directos",
		archivos, usosDelAyudante, len(culpables))
}

// lanzamientosDirectos devuelve las llamadas REALES a `os/exec` que construyen un proceso.
//
// Mira el árbol y no el texto: los `exec.Command` que hay en comentarios y en literales de este
// repo —hay nueve, casi todos en la guarda de PowerShell, que los cita para explicarse— no son
// llamadas y no tienen por qué serlo.
func lanzamientosDirectos(fset *token.FileSet, f *ast.File) []string {
	alias := aliasDeOsExec(f)
	if alias == "" {
		return nil
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, lanzador := range losLanzadores {
			if esSelector(c.Fun, alias, lanzador) {
				p := fset.Position(c.Pos())
				out = append(out, filepath.ToSlash(p.Filename)+":"+strconv.Itoa(p.Line)+
					" (`"+alias+"."+lanzador+"`)")
			}
		}
		return true
	})
	return out
}

// aliasDeOsExec devuelve con qué nombre este archivo llama a `os/exec`, o "" si no lo importa.
//
// Un import con punto rompería la pregunta —la llamada quedaría `Command(...)` a secas— así que
// no se lo interpreta: no hay ninguno, y si aparece uno, que se entere quien lo escriba.
func aliasDeOsExec(f *ast.File) string {
	for _, im := range f.Imports {
		if im.Path == nil || im.Path.Value != `"os/exec"` {
			continue
		}
		if im.Name == nil {
			return "exec"
		}
		return im.Name.Name
	}
	return ""
}

// contarUsosDelAyudante cuenta las llamadas que lanzan un proceso POR este paquete. Es el control
// de que el refactor sigue en pie, no una aserción sobre nadie en particular.
func contarUsosDelAyudante(f *ast.File) int {
	n := 0
	ast.Inspect(f, func(nd ast.Node) bool {
		c, ok := nd.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "Comando", "ComandoCtx", "Herramienta", "HerramientaCtx":
			n++
		}
		return true
	})
	return n
}

// TestSoloLasPruebasImportanLaCompuerta — que el paquete no se filtre al producto.
//
// `guiones` importa `testing`, que registra flags al init. Si un archivo que NO es de prueba lo
// importara, esas flags terminarían en el binario del agente que se instala en las máquinas de la
// flota. Y además sería la señal de que la compuerta dejó de ser una herramienta de pruebas.
func TestSoloLasPruebasImportanLaCompuerta(t *testing.T) {
	raiz := filepath.Join("..", "..")
	fset := token.NewFileSet()
	revisados := 0
	// El árbol lo dice git y no el directorio (A128), por lo mismo que arriba.
	gos, err := arbol.ConSufijo(raiz, ".go")
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range gos {
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		ruta := filepath.Join(raiz, filepath.FromSlash(rel))
		revisados++
		f, e := parser.ParseFile(fset, ruta, nil, parser.ImportsOnly)
		if e != nil {
			continue
		}
		for _, imp := range f.Imports {
			v, _ := strconv.Unquote(imp.Path.Value)
			if v == rutaDeLaCompuerta {
				t.Errorf("%s NO es un archivo de prueba y importa %s. Esa compuerta importa `testing`, "+
					"así que sus flags terminarían registradas en el binario que se instala en la "+
					"flota — y una herramienta de saltear pruebas no tiene nada que hacer en el "+
					"producto.", filepath.ToSlash(ruta), rutaDeLaCompuerta)
			}
		}
	}
	if revisados < 50 {
		t.Fatalf("se revisaron %d archivos .go que no son de prueba y el repo tiene muchos más: el "+
			"recorrido dejó de mirar", revisados)
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// El análisis
// ─────────────────────────────────────────────────────────────────────────────────────────────

// rutaDeLaCompuerta es el import path de este paquete.
const rutaDeLaCompuerta = "musubi/internal/guiones"

func esSelector(e ast.Expr, x, sel string) bool {
	s, ok := e.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != sel {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	return ok && id.Name == x
}
