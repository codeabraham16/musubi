package selfupdate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// Corre el firmador DE VERDAD. No lee su texto ni reimplementa su lógica: lo ejecuta contra un
// directorio con un asset y una clave, y mira qué manifiesto sale.
//
// Existe porque `deploy/firmar-release.sh` no tenía ningún test y acumuló TRES defectos que sólo
// se ven ejecutándolo, los tres encontrados el 2026-09-10 al ir a firmar v0.140.0:
//
//  1. invocaba `python3`, que en Windows es el alias de la Microsoft Store: `command -v` lo
//     encuentra y devuelve 0, pero no ejecuta nada;
//  2. su lista blanca no casaba con `Musubi.exe` ni `Musubi-arm64.exe` (eso lo cubre
//     TestElFirmadorCubreLosAssetsReales, que cruza la lista con AssetName);
//  3. comprobaba el permiso de la clave con un modo POSIX, y en Windows `chmod 600` deja 644,
//     así que abortaba SIEMPRE.
//
// Los tres dejaban el guion sin correr en la máquina de quien publica, con la clave montada. Un
// test que lea el archivo no encuentra ninguno de los tres.
func TestElFirmadorCorreDePuntaAPunta(t *testing.T) {
	guion := filepath.Join("..", "..", "deploy", "firmar-release.sh")
	if _, err := os.Stat(guion); err != nil {
		t.Fatalf("no encuentro %s: %v", guion, err)
	}
	// PORTABLE A PROPÓSITO, y declarado para que la guarda de alcance lo sepa: dos de los tres
	// defectos que este test vino a cazar SÓLO SE VEN EN WINDOWS, así que compuertarlo a linux
	// borraría justo la cobertura que lo justifica. Portable no saltea: si falta bash, MUERE —
	// el `t.Skip` que había acá contestaba «no pude medir» con el mismo verde que «medí y está
	// bien», que es el defecto que este repo viene pagando.
	guiones.Portable(t, "corre deploy/firmar-release.sh de punta a punta, y dos de sus tres defectos sólo aparecen en Windows", "bash")
	if !hayPythonQueEjecuta(t) {
		t.Skipf("no hay un python que ejecute (probé python3 y python): el guión no puede correr acá")
	}

	dir := t.TempDir()

	// Un asset con un nombre REAL de los que publica release.yml. Se elige por plataforma para que
	// el caso de Windows —el que la lista blanca dejaba afuera— se ejercite justamente en Windows.
	asset, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("plataforma sin asset de release (%s/%s): %v", runtime.GOOS, runtime.GOARCH, err)
	}
	if err := os.WriteFile(filepath.Join(dir, asset), []byte("binario de mentira"), 0o644); err != nil {
		t.Fatalf("escribir el asset: %v", err)
	}

	// La clave va FUERA del directorio que se firma: el guion aborta si encuentra algo con pinta de
	// secreto ahí adentro, y ese invariante no es el que este test mide.
	clave := filepath.Join(t.TempDir(), "prueba.hex")
	if err := os.WriteFile(clave, []byte(strings.Repeat("ab", 32)), 0o600); err != nil {
		t.Fatalf("escribir la clave: %v", err)
	}

	salida, err := exec.Command("bash", guion, "0.140.0", clave, dir).CombinedOutput()
	texto := string(salida)

	// El manifiesto se escribe ANTES de firmar, así que se puede exigir aunque falte `cryptography`.
	crudo, errLeer := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if errLeer != nil {
		t.Fatalf("el guión no dejó manifest.json (err=%v). Salida:\n%s", err, texto)
	}
	var man struct {
		Version string            `json:"version"`
		Assets  map[string]string `json:"assets"`
	}
	if err := json.Unmarshal(crudo, &man); err != nil {
		// La salida del guion va en el mensaje a propósito: si el intérprete murió, el manifiesto
		// queda VACÍO por el redirect y el error de JSON sólo cuenta el síntoma. La causa está en
		// lo que imprimió el guion, y sin ella esto se lee como «el JSON salió mal».
		t.Fatalf("manifest.json no es JSON válido (%v). Contenido: %q\nSalida del guión:\n%s", err, crudo, texto)
	}
	if man.Version != "0.140.0" {
		t.Errorf("version del manifiesto = %q, esperaba %q", man.Version, "0.140.0")
	}
	if _, ok := man.Assets[asset]; !ok {
		t.Errorf("el asset %q de %s/%s NO entró al manifiesto (entraron: %v). En esta plataforma "+
			"`musubi update` se negaría a instalar aunque el release esté bien firmado",
			asset, runtime.GOOS, runtime.GOARCH, clavesDe(man.Assets))
	}

	// La firma necesita el paquete `cryptography`, que no está en todas las máquinas. Si falta, el
	// guion lo DICE, y este test lo tolera nombrándolo — pero sólo ese motivo: cualquier otro
	// fallo del guion es una falla del test, no un salteo.
	if err != nil {
		if strings.Contains(texto, "cryptography") {
			t.Logf("manifiesto verificado; la firma no se pudo ejercitar acá: falta `cryptography`")
			return
		}
		t.Fatalf("el guión falló por algo que no es la falta de `cryptography`: %v\n%s", err, texto)
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json.sig")); err != nil {
		t.Errorf("el guión salió 0 y no dejó manifest.json.sig: %v", err)
	}
}

func hayPythonQueEjecuta(t *testing.T) bool {
	t.Helper()
	for _, c := range []string{"python3", "python"} {
		if _, err := exec.LookPath(c); err != nil {
			continue
		}
		// Se le PIDE QUE EJECUTE algo: en Windows `python3` existe como alias de la Store y
		// LookPath lo encuentra igual. Preguntar si el archivo existe es la comprobación que
		// dejó pasar el defecto original.
		if err := exec.Command(c, "-c", "import sys").Run(); err == nil {
			return true
		}
	}
	return false
}

func clavesDe(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
