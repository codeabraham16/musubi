package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// fijarHome apunta el home del proceso a `dir` EN TODAS LAS PLATAFORMAS.
//
// `t.Setenv("HOME", ...)` a secas NO ALCANZA EN WINDOWS, y eso dejó dos guardas de A96 mudas ahí
// durante cinco días: `os.UserHomeDir()` lee `$HOME` en Unix y `%USERPROFILE%` en Windows, así que
// una prueba que sólo fija `HOME` corre contra el home REAL del runner —donde no hay ningún
// `.musubi/config.yaml`— y `ConfigSombra` devuelve "" con razón. La guarda no fallaba por un bug:
// fallaba porque el escenario nunca se armó.
//
// Lo destapó `test-cross (windows-latest)` al abrir el PR #426. No se había visto antes porque una
// rama de larga vida SIN PR es invisible para CI (`ci.yml:14-21`), así que las pruebas
// multiplataforma nunca habían corrido sobre esta rama.
//
// Se fijan LAS DOS variables en todas las plataformas y no un `if runtime.GOOS`: una condición acá
// significa que la mitad del código de la prueba sólo se ejercita en un sistema, que es la forma en
// que este defecto entró.
func fijarHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestSecretoDeEnvLeeLaVariableDirecta(t *testing.T) {
	t.Setenv("PRUEBA_TOKEN", "  valor-directo  ")
	got, err := SecretoDeEnv("PRUEBA_TOKEN")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got != "valor-directo" {
		t.Fatalf("se esperaba el valor recortado, se obtuvo %q", got)
	}
}

// El caso que motivó todo esto: la variable NO está y el archivo SÍ. Antes del arreglo, esto
// devolvía vacío y el daemon salía a la red sin credencial, fallando en silencio cada 30 s.
func TestSecretoDeEnvCaeAlArchivoCuandoLaVariableNoEsta(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "token")
	// Con salto de línea al final, que es como lo deja cualquier `echo`.
	if err := os.WriteFile(ruta, []byte("msb_del-archivo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv("PRUEBA_TOKEN")
	t.Setenv("PRUEBA_TOKEN_FILE", ruta)

	got, err := SecretoDeEnv("PRUEBA_TOKEN")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got != "msb_del-archivo" {
		t.Fatalf("se esperaba el contenido del archivo sin el salto de línea, se obtuvo %q", got)
	}
}

// EL ARCHIVO LE GANA A LA VARIABLE — decidido por gio el 2026-09-05 (A101).
//
// El argumento no es de gusto sino de mecanismo: el archivo es el ÚNICO de los dos que puede rotar.
// El cerebro ofrece un token nuevo en la respuesta del latido y el agente lo apenda al archivo; una
// variable ya está en el entorno de un proceso que arrancó, y ahí no llega nadie. El mismo día se
// pagó: un `MUSUBI_TOKEN` REVOCADO en la terminal le ganó en silencio a un `MUSUBI_TOKEN_FILE`
// correcto.
//
// Sabotaje que la hace fallar: volver a mirar la variable antes que el archivo.
func TestSecretoDeEnvElArchivoLeGanaALaVariable(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "token")
	if err := os.WriteFile(ruta, []byte("del-archivo"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PRUEBA_TOKEN", "de-la-variable")
	t.Setenv("PRUEBA_TOKEN_FILE", ruta)

	got, err := SecretoDeEnv("PRUEBA_TOKEN")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got != "del-archivo" {
		t.Fatalf("el archivo tiene que ganar —es el único que puede rotar—; se obtuvo %q", got)
	}
}

// LA MITAD QUE DE VERDAD PUEDE MORDER: archivo ROTO más variable BUENA tiene que dar ERROR.
//
// Es la parte peligrosa de invertir la precedencia. Si `SecretoDeEnv` cayera a la variable cuando
// el archivo nombrado no se puede leer, una configuración rota se convertiría en una credencial
// silenciosa DISTINTA de la que se pidió — y con la variable buena puesta ni siquiera fallaría,
// así que nadie se enteraría de que el archivo está roto hasta la próxima rotación, cuando ya no
// hay a qué volver. Es el defecto de A89 entrando por la puerta de al lado.
//
// Es el hermano de `cmd/musubi/agent_token_fuente_test.go`, que ya fija esto del lado del agente.
//
// Sabotaje que la hace fallar: que el error del archivo caiga a la variable en vez de propagarse.
func TestSecretoDeEnvConArchivoRotoNoCaeALaVariable(t *testing.T) {
	t.Setenv("PRUEBA_TOKEN", "de-la-variable")
	t.Setenv("PRUEBA_TOKEN_FILE", filepath.Join(t.TempDir(), "no-existe"))

	got, err := SecretoDeEnv("PRUEBA_TOKEN")
	if err == nil {
		t.Fatalf("con el archivo roto tiene que fallar y NO usar la variable; devolvió %q sin error", got)
	}
	if got != "" {
		t.Fatalf("además del error devolvió %q: un secreto y un error a la vez invita a usar el secreto", got)
	}
	if !strings.Contains(err.Error(), "PRUEBA_TOKEN_FILE") {
		t.Fatalf("el error tiene que nombrar la variable culpable, dijo: %v", err)
	}
}

// Y lo mismo con el archivo de VARIAS LÍNEAS: la variable buena no lo rescata. Decidido junto con
// la precedencia (A101, dimensión 2): un archivo de secreto es UNA línea, y el formato de lista
// existe SÓLO para `MUSUBI_DEVICE_TOKEN_FILE`, que es el que tiene máquina para usarlo.
func TestSecretoDeEnvConArchivoDeVariasLineasTampocoCaeALaVariable(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(ruta, []byte("uno\ndos\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PRUEBA_TOKEN", "de-la-variable")
	t.Setenv("PRUEBA_TOKEN_FILE", ruta)

	got, err := SecretoDeEnv("PRUEBA_TOKEN")
	if err == nil {
		t.Fatalf("un archivo de dos líneas tiene que fallar; devolvió %q", got)
	}
	if got != "" {
		t.Fatalf("devolvió %q junto con el error", got)
	}
}

// Un archivo nombrado y no legible es una configuración rota, y tiene que hacer ruido. Devolver ""
// acá es exactamente el fallo que este cabo persigue: un secreto vacío que después aparece como un
// 401 sin causa visible.
func TestSecretoDeEnvFallaSiElArchivoNoSePuedeLeer(t *testing.T) {
	os.Unsetenv("PRUEBA_TOKEN")
	t.Setenv("PRUEBA_TOKEN_FILE", filepath.Join(t.TempDir(), "no-existe"))

	got, err := SecretoDeEnv("PRUEBA_TOKEN")
	if err == nil {
		t.Fatalf("se esperaba error por archivo ilegible, se obtuvo %q sin error", got)
	}
	if !strings.Contains(err.Error(), "PRUEBA_TOKEN_FILE") {
		t.Fatalf("el error tiene que nombrar la variable culpable, dijo: %v", err)
	}
}

func TestSecretoDeEnvSinNadaConfiguradoNoEsError(t *testing.T) {
	os.Unsetenv("PRUEBA_TOKEN")
	os.Unsetenv("PRUEBA_TOKEN_FILE")
	got, err := SecretoDeEnv("PRUEBA_TOKEN")
	if err != nil || got != "" {
		t.Fatalf("se esperaba vacío sin error, se obtuvo %q / %v", got, err)
	}
}

// ── A96: decir cuál config gobierna ───────────────────────────────────────────────────────

func TestConfigPathCuelgaDelProyectoYNoDelHome(t *testing.T) {
	got := ConfigPath("/un/proyecto")
	quiero := filepath.Join("/un/proyecto", DirName, ConfigFile)
	if got != quiero {
		t.Fatalf("ConfigPath tiene que colgar del projectPath: %q", got)
	}
}

// El caso exacto del cabo: existe un config en el home Y otro en el proyecto, y manda el del
// proyecto. Sin este aviso, quien diagnostica abre el del home —porque es el que se conoce— lee
// un valor terminante y descarta la causa correcta.
func TestConfigSombraDelataAlConfigDelHomeQueNoGobierna(t *testing.T) {
	home := t.TempDir()
	fijarHome(t, home)
	if err := os.MkdirAll(filepath.Join(home, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, DirName, ConfigFile), []byte("sync:\n  enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	proyecto := t.TempDir()
	got := ConfigSombra(proyecto)
	if got != filepath.Join(home, DirName, ConfigFile) {
		t.Fatalf("tenía que delatar el config del home; devolvió %q", got)
	}
}

func TestConfigSombraCallaSiNoHayOtro(t *testing.T) {
	fijarHome(t, t.TempDir())
	if got := ConfigSombra(t.TempDir()); got != "" {
		t.Fatalf("sin config en el home no hay nada que avisar; devolvió %q", got)
	}
}

// Si el proyecto ES el home, no hay dos configs: no se avisa de sí mismo.
func TestConfigSombraNoSeDelataASiMismo(t *testing.T) {
	home := t.TempDir()
	fijarHome(t, home)
	if err := os.MkdirAll(filepath.Join(home, DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, DirName, ConfigFile), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ConfigSombra(home); got != "" {
		t.Fatalf("el proyecto es el home: no hay sombra. Devolvió %q", got)
	}
}

// ── A101: que nadie más vuelva a leer un token nombrado sin el respaldo del archivo ──────────
//
// El cabo A89 arregló DOS lugares (NewSyncClient y nuevoEmpujadorOTLP) y la pregunta que rinde no
// era «¿esto está mal?» sino «¿quién más hace esto?». Eran seis: `resolveServiceAuth` en el mismo
// http.go que se había tocado, provision, el canal `musubi cerebro`, el dashboard y la shell.
//
// Esta guarda es esa pregunta hecha permanente: un campo que NOMBRA una variable de entorno con un
// token —`AuthTokenEnv`, `TokenEnv`— no se puede leer con `os.Getenv` a secas, porque así el
// archivo `<VAR>_FILE` no existe para ese camino y el operador que siguió la recomendación queda
// sin credencial y sin aviso.
//
// Sabotaje que la hace fallar: volver a poner `os.Getenv(cfg.AuthTokenEnv)` en cualquier lado.
func TestNadieLeeUnTokenNombradoSinElRespaldoDelArchivo(t *testing.T) {
	var revisados int
	var culpables []string

	raiz := filepath.Join("..", "..")
	err := filepath.WalkDir(raiz, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Los directorios ocultos se saltean ENTEROS: `.claude/worktrees/` guarda copias
			// completas del repo de sesiones viejas, y revisarlas daba 129 «culpables» que no
			// existen en el árbol de verdad. Una guarda que grita sobre código que nadie despliega
			// se desactiva sola.
			// `ruta != raiz` importa: la raíz se pasa como "../.." y su Name() es "..", que empieza
			// con punto — sin esta condición el recorrido se saltea el repo ENTERO y revisa cero
			// archivos. Lo cazó el control de «miró algo» de abajo, que es exactamente para esto.
			if ruta != raiz && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			switch d.Name() {
			case "node_modules", "vendor", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		b, err := os.ReadFile(ruta)
		if err != nil {
			return err
		}
		revisados++
		for i, linea := range strings.Split(string(b), "\n") {
			if !strings.Contains(linea, "os.Getenv(") {
				continue
			}
			if !strings.Contains(linea, "AuthTokenEnv") && !strings.Contains(linea, "TokenEnv") {
				continue
			}
			culpables = append(culpables, fmt.Sprintf("%s:%d — %s", ruta, i+1, strings.TrimSpace(linea)))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no pude recorrer el repo: %v", err)
	}

	// CONTROL DE «MIRÓ ALGO». El modo de falla más común de una guarda como ésta no es una
	// aserción equivocada: es un recorrido que no llega y deja el verde sobre la nada.
	if revisados < 100 {
		t.Fatalf("sólo revisé %d archivos .go: el recorrido no llegó, así que este verde no dice nada", revisados)
	}
	if len(culpables) > 0 {
		t.Fatalf("hay %d lugar(es) leyendo un token NOMBRADO con os.Getenv a secas, sin el respaldo\n"+
			"`<VAR>_FILE`. Usá config.SecretoDeEnv (cabos A89/A101):\n  %s",
			len(culpables), strings.Join(culpables, "\n  "))
	}
}

// ═════════════════════════════════════════════════════════════════════════════════════════════
// UN ARCHIVO DE VARIAS LÍNEAS FALLA ACÁ Y CON SU MOTIVO, NO LEJOS Y MUDO
//
// El Trim saca el `\n` del final y nada más. Con dos líneas no vacías lo que vuelve es un secreto
// con un salto de línea ADENTRO, y eso NO da 401: `net/http` se niega a mandar el pedido. Medido el
// 2026-09-05 con un archivo de dos tokens:
//
//	SecretoDeEnv -> "tokenNUEVO\ntokenVIEJO"
//	net/http     -> invalid header field value for "Authorization"
//
// Ese error apunta a la biblioteca HTTP, o sea al único lugar donde la causa NO está. Y este mismo
// par de variables ya costó cuatro intentos de diagnóstico el 2026-08-31 con el YAML, el hash, la
// ruta, el proceso y la recarga TODOS verificados correctos (A89): la causa estaba en el shell, que
// era el único lugar donde nadie miró porque nada apuntaba ahí.
//
// SE RECHAZA Y NO SE ADIVINA LA PRIMERA LÍNEA a propósito: acá no hay formato multi-token. El que sí
// existe —lista de tokens, el más VIEJO primero (se apendea), para que una rotación tenga fallback— es del token
// de DISPOSITIVO (`MUSUBI_DEVICE_TOKEN_FILE`, en cmd/musubi/agent_token.go). Quedarse con la primera
// línea inventaría ese formato para un camino que no lo tiene, y elegiría en silencio entre dos
// credenciales cuando lo honesto es decir que no se sabe cuál se quiso poner.
//
// Sabotaje verificado: quitar la guarda → el caso de dos líneas devuelve el valor con `\n` adentro.
func TestUnArchivoDeSecretoConVariasLineasSeRechazaConSuMotivo(t *testing.T) {
	dir := t.TempDir()
	escribir := func(nombre, contenido string) string {
		t.Helper()
		ruta := filepath.Join(dir, nombre)
		if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
			t.Fatal(err)
		}
		return ruta
	}

	casos := []struct {
		nombre    string
		contenido string
		quiere    string // "" = se espera error
		porque    string
	}{
		{"una línea, el caso de todos los días", "elsecreto\n", "elsecreto",
			"un archivo escrito con `echo` termina en \\n y el secreto no lo incluye"},
		{"una línea sin salto final", "elsecreto", "elsecreto", ""},
		{"una línea con espacios alrededor", "  elsecreto  \n", "elsecreto", ""},
		{"varias líneas en blanco alrededor de una sola", "\n\n  elsecreto  \n\n\n", "elsecreto",
			"los blancos no son un segundo secreto"},
		{"DOS tokens: se rechaza", "tokenNUEVO\ntokenVIEJO\n", "",
			"es el formato del token de DISPOSITIVO, no de éste; devolver el valor entero mata el " +
				"pedido en net/http con un error que no nombra ni el archivo ni la variable"},
		{"dos líneas con CRLF, como lo escribiría Windows", "tokenA\r\ntokenB\r\n", "",
			"el CRLF no lo hace menos ambiguo"},
		{"una línea y un comentario debajo", "elsecreto\n# rotado el 5/9\n", "",
			"un comentario es una segunda línea no vacía, y adivinar cuál es el secreto sería inventar"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			ruta := escribir("s.txt", c.contenido)
			t.Setenv("PRUEBA_SECRETO", "")
			t.Setenv("PRUEBA_SECRETO_FILE", ruta)

			got, err := SecretoDeEnv("PRUEBA_SECRETO")
			if c.quiere == "" {
				if err == nil {
					t.Fatalf("devolvió %q sin error, y con un salto de línea adentro el pedido muere en "+
						"net/http con «invalid header field value», que apunta al único lugar donde la "+
						"causa NO está — %s", got, c.porque)
				}
				// El mensaje tiene que servirle a quien lo lee: la variable, el archivo, y qué hacer.
				//
				// LA RUTA SE BUSCA COMO EL MENSAJE LA ESCRIBE, o sea CITADA. El error usa `%q` a
				// propósito —una ruta con un espacio al final, o vacía, es un bug clásico de
				// variable de entorno y sin comillas es invisible—, y `%q` ESCAPA las barras
				// invertidas: en Windows la ruta sale `"C:\\Users\\..."` y buscar la cruda no la
				// encuentra. Eso puso en rojo a `test-cross (windows-latest)` el 2026-09-09 sobre
				// un mensaje que estaba bien: lo naive era la comparación, no el error.
				for _, aguja := range []string{"PRUEBA_SECRETO_FILE", strconv.Quote(ruta), "una sola línea"} {
					if !strings.Contains(err.Error(), aguja) {
						t.Errorf("el error no menciona %q, así que no alcanza para arreglarlo: %v", aguja, err)
					}
				}
				if strings.Contains(err.Error(), "tokenNUEVO") || strings.Contains(err.Error(), "tokenA") {
					t.Errorf("el error FILTRA el secreto: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado (%s): %v", c.porque, err)
			}
			if got != c.quiere {
				t.Errorf("devolvió %q y se esperaba %q — %s", got, c.quiere, c.porque)
			}
			if strings.ContainsAny(got, "\n\r") {
				t.Errorf("el valor tiene un salto de línea adentro: %q", got)
			}
		})
	}
}

// NINGÚN LUGAR DEL REPO PUEDE VOLVER A DECIR QUE EL ARCHIVO DE TOKENS VIENE EN ORDEN INVERSO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// ERA FALSO, ESTABA EN CINCO LUGARES, Y ES LA CLASE DE FALSEDAD QUE HACE DAÑO
// (la frase exacta que se prohíbe se arma abajo por partes; acá no se escribe)
//
// `cmd/musubi/agent_token.go` APENDEA el token nuevo al final del archivo (`apendarToken`, y su
// comentario explica por qué: un reemplazo atómico crea una entrada de directorio nueva que no es
// durable sin fsync del directorio). O sea que la PRIMERA línea es el token MÁS VIEJO. Lo fija
// `TestElTokenNuevoSePersisteAntesDeEstrenarse`, que exige `[viejo nuevo]`, y `Usar()` devuelve
// `viejo` primero.
//
// Cinco comentarios y filas del registro decían lo contrario, y no es un detalle de redacción:
// quien lea que el nuevo va primero y decida «entonces me quedo con la primera línea» —que es una
// conclusión razonable— toma justamente el token que la rotación está retirando, y ninguno de
// esos caminos tiene reintento: el resultado es un 401 permanente en vez de un error legible.
// Encontrado el 2026-09-05 auditando A101.
//
// Sabotaje que la hace fallar: volver a escribir esa frase en cualquiera de ellos.
func TestNadieDiceQueElArchivoDeTokensTraeElMasNuevoPrimero(t *testing.T) {
	raiz := filepath.Join("..", "..")
	// LA FRASE SE ARMA POR PARTES A PROPÓSITO. Escrita entera acá, este archivo sería su
	// propio culpable: la guarda se disparó con su propia documentación en la primera
	// corrida. Es la trampa del 2026-09-05 al revés — allá el comentario SATISFACÍA la
	// guarda, acá la ROMPÍA— y la salida es la misma: que el literal no viva en el texto.
	frase := "el más " + "nuevo primero"
	revisados := 0
	var culpables []string

	err := filepath.WalkDir(raiz, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Los ocultos se saltean, PERO nunca la raíz: se pasa como `../..`, cuyo `Name()` es
		// `..` y empieza con punto — el filtro ingenuo se saltaba el repo entero y la guarda
		// revisaba CERO archivos en verde. Ya pasó el 2026-09-05 en la prueba de al lado.
		if d.IsDir() {
			if ruta != raiz && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		switch filepath.Ext(ruta) {
		case ".go", ".md", ".sh", ".yml", ".ps1", ".cmd":
		default:
			return nil
		}
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			return nil
		}
		revisados++
		if strings.Contains(string(crudo), frase) {
			culpables = append(culpables, ruta)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo recorrer el repo: %v", err)
	}
	// CONTROL DE «MIRÓ ALGO»: sin esto, un recorrido que no llega da verde sobre la nada.
	if revisados < 100 {
		t.Fatalf("la guarda revisó %d archivos: el recorrido no llegó a ningún lado y su verde no vale", revisados)
	}
	if len(culpables) > 0 {
		t.Fatalf("estos archivos afirman %q del archivo de tokens, y es al revés:\n  %s\n"+
			"`apendarToken` agrega AL FINAL, así que la primera línea es el token MÁS VIEJO —el que\n"+
			"la rotación está retirando—. Quien lea esto y decida «me quedo con la primera línea»\n"+
			"elige la credencial equivocada, y ninguno de esos caminos reintenta: 401 permanente.",
			frase, strings.Join(culpables, "\n  "))
	}
}

// LA PRUEBA DE COMPORTAMIENTO DE LA PRECEDENCIA TIENE QUE SEGUIR EXISTIENDO Y SER EJECUTABLE.
//
// `deploy/pruebas/precedencia-del-token.sh` levanta un oidor, corre `musubi-tool.sh` de verdad y
// MIRA QUÉ BEARER LLEGÓ. Es la única forma de custodiar la mitad de A101 que vive en bash, y es de
// comportamiento a propósito: el 2026-09-05 se auditaron las 46 guardas de grep sobre archivos de
// despliegue y SIETE no se ponían rojas con su propio sabotaje, todas porque el texto que buscaban
// vivía donde no decide nada. La palabra «archivo» aparece decenas de veces en ese guion y ninguna
// elige la credencial que sale por el socket.
//
// El bit de ejecución importa tanto como el archivo: una comprobación que hay que invocar con
// `bash` de por medio se corre menos, y la que no se corre no existe.
//
// Sabotaje que la hace fallar: borrar el guion, o quitarle el bit de ejecución.
func TestLaPruebaDeComportamientoDeLaPrecedenciaSigueEnPie(t *testing.T) {
	ruta := filepath.Join("..", "..", "deploy", "pruebas", "precedencia-del-token.sh")
	fi, err := os.Stat(ruta)
	if err != nil {
		t.Fatalf("falta %s: %v\nSin ella, la precedencia de A101 queda custodiada sólo del lado de Go,\n"+
			"y la mitad que decidió mal dos veces vive en bash.", ruta, err)
	}
	// NTFS no tiene el bit y git en Windows no lo preserva: ahí la aserción sería siempre falsa,
	// dijera lo que dijera el repo. La que importa —que el guion ESTÉ— corre en las tres.
	if runtime.GOOS != "windows" && fi.Mode()&0o111 == 0 {
		t.Errorf("%s no es ejecutable", ruta)
	}
}
