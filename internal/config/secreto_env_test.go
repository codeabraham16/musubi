package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestSecretoDeEnvLaVariableDirectaLeGanaAlArchivo(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "token")
	if err := os.WriteFile(ruta, []byte("del-archivo"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PRUEBA_TOKEN", "de-la-variable")
	t.Setenv("PRUEBA_TOKEN_FILE", ruta)

	got, _ := SecretoDeEnv("PRUEBA_TOKEN")
	if got != "de-la-variable" {
		t.Fatalf("la variable directa tiene que ganar; se obtuvo %q", got)
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
	t.Setenv("HOME", home)
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
	t.Setenv("HOME", t.TempDir())
	if got := ConfigSombra(t.TempDir()); got != "" {
		t.Fatalf("sin config en el home no hay nada que avisar; devolvió %q", got)
	}
}

// Si el proyecto ES el home, no hay dos configs: no se avisa de sí mismo.
func TestConfigSombraNoSeDelataASiMismo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
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
// existe —lista de tokens, el más nuevo primero, para que una rotación tenga fallback— es del token
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
				for _, aguja := range []string{"PRUEBA_SECRETO_FILE", ruta, "una sola línea"} {
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
