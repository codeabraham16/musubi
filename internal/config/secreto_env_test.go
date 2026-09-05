package config

import (
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
