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
