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
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
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
func TestLaCompuertaNuncaSalteaEnLinux(t *testing.T) {
	siguio := false
	t.Run("sonda", func(t *testing.T) {
		Exigir(t, "la sonda de la propia compuerta ejecuta bash para comprobar que no saltea", "bash")
		siguio = true
		if err := exec.Command("bash", "-c", ":").Run(); err != nil {
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
func TestLaCompuertaUnixNoSalteaDondeElArnesSeSostiene(t *testing.T) {
	siguio := false
	t.Run("sonda", func(t *testing.T) {
		Unix(t, "la sonda del modo Unix ejecuta bash para comprobar que no saltea ni en linux ni en macOS", "bash")
		siguio = true
		if err := exec.Command("bash", "-c", ":").Run(); err != nil {
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
	default:
		// En una corrida normal pasa por la compuerta y ejecuta una shell como cualquiera de las
		// pruebas que ésta custodia. No es adorno: así queda del lado correcto de la guarda de
		// alcance de más abajo SIN NECESITAR UNA EXCEPCIÓN, y una lista de excepciones es
		// exactamente por donde una guarda como ésta se vacía con el tiempo.
		Exigir(t, "el ayudante ejecuta bash igual que las pruebas que esta compuerta custodia", "bash")
		if err := exec.Command("bash", "-c", ":").Run(); err != nil {
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
	casos := []struct {
		nombre, valor, esperado, porque string
	}{
		{"motivo vacío de contenido", "sin-motivo", "hace falta una frase",
			"un motivo de dos palabras pasó: el salteo puede quedar sin decir por qué, y un salteo " +
				"que nadie puede revisar se queda para siempre"},
		{"sin nombrar ninguna herramienta", "sin-herramientas", "NO es",
			"la compuerta aceptó ser usada sin nombrar una sola herramienta Unix, o sea como un " +
				"t.Skip de propósito general: es exactamente lo que no puede ser"},
		// EL HERMANO: los dos controles de uso valen para los TRES modos o no valen para ninguno.
		// Un modo nuevo con los `t.Fatalf` copiados y sin prueba que los corra es la forma exacta
		// que este repo persigue —la guarda puesta en N-1 de N caminos—, y acá el N acaba de subir.
		{"modo Unix, motivo vacío de contenido", "unix-sin-motivo", "hace falta una frase",
			"guiones.Unix aceptó un motivo de dos palabras: el salteo de windows puede quedar sin " +
				"decir por qué, y un salteo que nadie puede revisar se queda para siempre"},
		{"modo Unix, sin nombrar ninguna herramienta", "unix-sin-herramientas", "NO es",
			"guiones.Unix aceptó ser usada sin nombrar una sola herramienta, o sea como un t.Skip " +
				"de propósito general para apagar windows: es exactamente lo que no puede ser"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestAyudanteDeUsoIndebido$", "-test.v")
			cmd.Env = append(os.Environ(), sobreUsoIndebido+"="+c.valor)
			salida, err := cmd.CombinedOutput()
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
// (3) EL ALCANCE: quien saltea corre una shell, y quien corre una shell saltea
// ─────────────────────────────────────────────────────────────────────────────────────────────

// shells — los nombres de programa que SON una shell. Lo que decide es el nombre base del
// ejecutable que se le pasa a exec.Command, no un comentario ni el nombre de la prueba.
var shells = map[string]bool{
	"sh": true, "bash": true, "dash": true, "zsh": true, "ksh": true, "ash": true,
}

const rutaDeLaCompuerta = "musubi/internal/guiones"

// losModosDeLaCompuerta son los nombres que cuentan como «esta prueba pasó por la compuerta».
//
// ESTÁ ESCRITO UNA SOLA VEZ a propósito. Cuando eran dos, la lista vivía en cuatro lados —dos
// condiciones del detector, el comentario del campo y el mensaje de error— y agregar el tercero
// habría dejado tres de esos cuatro mintiendo: el detector no lo reconocería y seguiría pidiendo
// compuerta a pruebas que ya la tienen. Es la forma exacta que este repo persigue con nombre
// propio: la regla escrita en N lugares envejece en N-1.
var losModosDeLaCompuerta = []string{"Exigir", "Portable", "Unix"}

// funcDePrueba — lo que la guarda sabe de una función de un paquete de prueba.
type funcDePrueba struct {
	pkg      string // directorio + paquete: el ámbito donde se resuelven las llamadas
	nombre   string
	pos      string
	esTest   bool
	shell    bool // ejecuta una shell EN SU PROPIO cuerpo
	compuert bool // llama a alguno de losModosDeLaCompuerta en su propio cuerpo
	llama    []string
}

// TestElSalteoEstaAcotadoALasPruebasQueCorrenUnaShell — la guarda de alcance, en los dos sentidos.
//
// EL PROBLEMA QUE RESUELVE: una compuerta de salteo es una herramienta afilada. Mañana alguien
// tiene una prueba que falla en macOS por un motivo REAL —un bug del producto en darwin— y la
// forma más corta de ponerla en verde es llamar a `guiones.Exigir`. Eso taparía un defecto de
// verdad, y el mensaje del salteo hablaría con toda seguridad de guiones de shell que esa prueba
// nunca ejecutó.
//
// NO SE PREGUNTA POR UN TEXTO. Este repo ya se comió siete guardas de grep satisfechas por un
// comentario, un mensaje de error o la línea vecina. Acá se PARSEA el paquete, se arma el grafo de
// llamadas y se pregunta por lo que DECIDE: ¿hay, alcanzable desde esta prueba, un
// `exec.Command` cuyo programa es una shell?
//
// Y SE EXIGE LA EQUIVALENCIA, no la implicación. El sentido «quien corre una shell llama a la
// compuerta» es el que caza al HERMANO —el defecto dominante de este repo: la guarda puesta en N-1
// de N caminos—. Cuando mañana alguien agregue la prueba número 15 que ejecuta un guion, esta
// guarda se la va a pedir sin que nadie se acuerde.
func TestElSalteoEstaAcotadoALasPruebasQueCorrenUnaShell(t *testing.T) {
	raiz := filepath.Join("..", "..")
	fset := token.NewFileSet()

	porNombre := map[string]map[string]*funcDePrueba{} // pkg -> nombre -> func
	archivos := 0

	err := filepath.WalkDir(raiz, func(ruta string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", "vendor", "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		f, e := parser.ParseFile(fset, ruta, nil, 0)
		if e != nil {
			t.Errorf("%s no parsea (%v): esta guarda no puede mirar lo que no puede leer, y un archivo "+
				"invisible acá es una prueba sin compuerta que nadie va a reclamar", ruta, e)
			return nil
		}
		archivos++
		pkg := filepath.ToSlash(filepath.Dir(ruta)) + "|" + f.Name.Name
		if porNombre[pkg] == nil {
			porNombre[pkg] = map[string]*funcDePrueba{}
		}
		alias := aliasDeLaCompuerta(f)
		esLaCompuerta := f.Name.Name == "guiones"

		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil || fd.Recv != nil {
				continue
			}
			info := &funcDePrueba{
				pkg:    pkg,
				nombre: fd.Name.Name,
				pos:    filepath.ToSlash(ruta) + ":" + strconv.Itoa(fset.Position(fd.Pos()).Line),
				esTest: strings.HasPrefix(fd.Name.Name, "Test"),
			}
			analizarCuerpo(fd, alias, esLaCompuerta, info)
			porNombre[pkg][info.nombre] = info
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo recorrer el repo: %v", err)
	}

	// CONTROL DE QUE MIRÓ ALGO. Si el recorrido o el parseo se rompieran, las dos listas quedarían
	// vacías y esta guarda pasaría en verde sin haber comprobado un solo archivo — que es la forma
	// exacta de fallo que persigue. Un cero acá significa «no pude medir».
	if archivos < 100 {
		t.Fatalf("se parsearon %d archivos `_test.go` y el repo tiene cientos: el recorrido dejó de "+
			"mirar y esta guarda estaría en verde sin haber comprobado nada", archivos)
	}

	// Cierre transitivo dentro de cada paquete.
	for _, fs := range porNombre {
		for cambio := true; cambio; {
			cambio = false
			for _, f := range fs {
				for _, n := range f.llama {
					g, ok := fs[n]
					if !ok {
						continue
					}
					if g.shell && !f.shell {
						f.shell, cambio = true, true
					}
					if g.compuert && !f.compuert {
						f.compuert, cambio = true, true
					}
				}
			}
		}
	}

	var sinCompuerta, fueraDeAlcance []string
	conShell := 0
	for _, fs := range porNombre {
		for _, f := range fs {
			if !f.esTest {
				continue
			}
			switch {
			case f.shell && f.compuert:
				conShell++
			case f.shell && !f.compuert:
				conShell++
				sinCompuerta = append(sinCompuerta, f.pos+" "+f.nombre)
			case !f.shell && f.compuert:
				fueraDeAlcance = append(fueraDeAlcance, f.pos+" "+f.nombre)
			}
		}
	}
	sort.Strings(sinCompuerta)
	sort.Strings(fueraDeAlcance)

	// El piso NO es el número exacto de pruebas de hoy —eso obligaría a tocar esta guarda cada vez
	// que se agrega una, y una guarda que estorba se apaga—; es la comprobación de que el análisis
	// SIGUE VIENDO el grafo. Si el detector de `exec.Command` se rompiera, este número se cae a 0.
	if conShell < 10 {
		t.Fatalf("el análisis encontró sólo %d prueba(s) que ejecutan una shell, y en este repo hay "+
			"más de una docena (los arneses de deploy/ en internal/mcp y los dos de internal/fleet). "+
			"El detector del grafo de llamadas se rompió, y con él los dos sentidos de esta guarda: "+
			"estaría en verde sin haber mirado nada", conShell)
	}
	t.Logf("pruebas que ejecutan una shell, detectadas por el grafo de llamadas: %d", conShell)

	for _, x := range sinCompuerta {
		t.Errorf("EL HERMANO SIN LA COMPUERTA: %s ejecuta una shell y NO pasa por la compuerta.\n"+
			"  En Windows esa prueba no mide el guion: mide el runner. El arnés escribe stubs\n"+
			"  ejecutables y los antepone al PATH con `:` — medido, en windows el stub no se toma y el\n"+
			"  guion sale a la URL real del release.\n"+
			"  Arreglo, y son TRES casos distintos. Elegí por lo que el guion NECESITA, no por dónde\n"+
			"  te molesta que falle:\n"+
			"    · sólo corre en linux    -> `guiones.Exigir(t, \"<qué guion corre y por qué es de linux>\", \"bash\", ...)`\n"+
			"    · linux y macOS, no Win  -> `guiones.Unix(t, \"<qué guion corre y por qué se mide en los dos>\", \"bash\", ...)`\n"+
			"    · corre en las tres      -> `guiones.Portable(t, \"<qué guion corre y por qué vale en las tres>\", \"bash\", ...)`\n"+
			"      (Portable NO saltea: exige que las herramientas estén en TODAS las plataformas.)\n"+
			"  como primera línea. En linux NINGUNO saltea, así que no perdés nada donde importa.\n"+
			"  Y OJO CON ELEGIR `Exigir` POR COMODIDAD: el defecto de `${VAR}` pegada a un carácter\n"+
			"  no-ASCII que mata el guion en el bash 3.2 de macOS es INVISIBLE en Linux. Para una\n"+
			"  prueba que caza eso, `Exigir` no acota el alcance: lo apaga. Ésa es `Unix`.", x)
	}
	for _, x := range fueraDeAlcance {
		t.Errorf("COMPUERTA FUERA DE ALCANCE: %s llama a guiones.Exigir y NO ejecuta ninguna shell.\n"+
			"  Eso es un t.Skip de propósito general disfrazado, y el mensaje que imprime habla de\n"+
			"  guiones de shell que esta prueba nunca corre: taparía un fallo REAL del producto en esa\n"+
			"  plataforma mientras dice que salteó por el arnés.\n"+
			"  Si la prueba falla en darwin/windows por un motivo del producto, ése es el defecto y hay\n"+
			"  que arreglarlo o declararlo donde corresponda; no acá.", x)
	}
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
	err := filepath.WalkDir(raiz, func(ruta string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", "vendor", "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		revisados++
		f, e := parser.ParseFile(fset, ruta, nil, parser.ImportsOnly)
		if e != nil {
			return nil
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
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo recorrer el repo: %v", err)
	}
	if revisados < 50 {
		t.Fatalf("se revisaron %d archivos .go que no son de prueba y el repo tiene muchos más: el "+
			"recorrido dejó de mirar", revisados)
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// El análisis
// ─────────────────────────────────────────────────────────────────────────────────────────────

// aliasDeLaCompuerta devuelve con qué nombre este archivo se refiere al paquete de la compuerta,
// o "" si no lo importa.
func aliasDeLaCompuerta(f *ast.File) string {
	for _, imp := range f.Imports {
		v, _ := strconv.Unquote(imp.Path.Value)
		if v != rutaDeLaCompuerta {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "guiones"
	}
	return ""
}

// analizarCuerpo llena `info` mirando el cuerpo de fd (incluidas las funciones literales de
// adentro, que es donde viven los `t.Run`).
func analizarCuerpo(fd *ast.FuncDecl, alias string, esLaCompuerta bool, info *funcDePrueba) {
	// Nombres de variables que salieron de un exec.LookPath("bash") y compañía: es la forma
	// `bash, err := exec.LookPath("bash"); exec.Command(bash, ...)` que usan cuatro arneses.
	deShell := map[string]bool{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, r := range as.Rhs {
			c, ok := r.(*ast.CallExpr)
			if !ok || !esSelector(c.Fun, "exec", "LookPath") || len(c.Args) == 0 {
				continue
			}
			if !esLiteralDeShell(c.Args[0]) {
				continue
			}
			if len(as.Lhs) > 0 {
				if id, ok := as.Lhs[0].(*ast.Ident); ok {
					deShell[id.Name] = true
				}
			}
		}
		return true
	})

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// ¿ejecuta una shell?
		prog := -1
		switch {
		case esSelector(c.Fun, "exec", "Command"):
			prog = 0
		case esSelector(c.Fun, "exec", "CommandContext"):
			prog = 1
		}
		if prog >= 0 && len(c.Args) > prog {
			a := c.Args[prog]
			if esLiteralDeShell(a) {
				info.shell = true
			}
			if id, ok := a.(*ast.Ident); ok && deShell[id.Name] {
				info.shell = true
			}
		}
		// ¿llama a la compuerta?
		for _, modo := range losModosDeLaCompuerta {
			if alias != "" && esSelector(c.Fun, alias, modo) {
				info.compuert = true
			}
			if esLaCompuerta {
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == modo {
					info.compuert = true
				}
			}
		}
		// ¿llama a otra función del mismo paquete?
		if id, ok := c.Fun.(*ast.Ident); ok {
			info.llama = append(info.llama, id.Name)
		}
		return true
	})
}

func esSelector(e ast.Expr, x, sel string) bool {
	s, ok := e.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != sel {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	return ok && id.Name == x
}

// esLiteralDeShell mira el NOMBRE BASE del programa: `"bash"`, `"/bin/sh"` y `"/usr/bin/env"` no
// son lo mismo, y lo que decide es qué se ejecuta.
func esLiteralDeShell(e ast.Expr) bool {
	l, ok := e.(*ast.BasicLit)
	if !ok || l.Kind != token.STRING {
		return false
	}
	v, err := strconv.Unquote(l.Value)
	if err != nil {
		return false
	}
	return shells[filepath.Base(filepath.ToSlash(v))]
}
