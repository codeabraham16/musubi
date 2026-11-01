package specgraph

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// La sintaxis de ruta de Windows (drive relativo `C:nota.md`, stream alterno
// `nota:flujo.md`) es legal como nombre en POSIX, así que la compuerta de
// ResolveSpecPath que la rechaza vive bajo `runtime.GOOS=="windows"` (store.go).
// Este test fija que ResolveSpecPath CABLEA esa compuerta —y que es CONDICIONAL al
// SO— en vez de que sólo exista HasWindowsNamespaceSyntax suelto (su invariante
// unitario puro ya está en TestHasWindowsNamespaceSyntax, multiplataforma).
//
// Corre en CI (Linux): fija que la compuerta NO sobre-rechaza en POSIX —esas rutas
// resuelven como nombres legales—; sabotearla volviéndola incondicional (sacarle el
// `runtime.GOOS=="windows"`) pone ESTE brazo en rojo. En Windows (davantis-1) verifica
// la otra mitad: que SÍ frenan cerrado. El archivo NO lleva `//go:build windows` ni el
// sufijo `_windows_test.go` justamente para no quedar invisible en CI —esa trampa la
// persigue internal/mcp/pruebas_que_no_corren_test.go, que es lo que lo cazó—.
func TestResolveSpecPathWindowsNamespace(t *testing.T) {
	root := t.TempDir()
	vectores := []string{
		`C:nota.md`,     // drive-relative: no la ve filepath.IsAbs
		`nota:flujo.md`, // stream alterno (ADS)
	}
	for _, v := range vectores {
		t.Run(v, func(t *testing.T) {
			rel, _, err := ResolveSpecPath(root, v)
			frenadaPorSintaxisWindows := err != nil && strings.Contains(err.Error(), "Windows drive or stream")
			if runtime.GOOS == "windows" {
				// La compuerta DEBE cablear HasWindowsNamespaceSyntax: en Windows estas
				// rutas son drive-relative/ADS y frenan cerrado con ESE mensaje.
				if !frenadaPorSintaxisWindows {
					t.Fatalf("en Windows %q debía frenar en la compuerta de sintaxis Windows; dio rel=%q err=%v", v, rel, err)
				}
			} else {
				// En POSIX un `:` en un nombre es legal: la compuerta no existe en este SO,
				// así que la ruta resuelve como nombre relativo (ResolveSpecPath tolera el
				// archivo final inexistente para que create pueda resolver una spec nueva).
				// Si algún día se decide rechazar el `:` en TODOS los SO, este brazo es el
				// que cambia —a propósito, no por accidente—.
				if err != nil {
					t.Fatalf("en POSIX %q es un nombre legal y debía resolver; dio err=%v", v, err)
				}
			}
		})
	}
}

// TestResolveSpecPathRechazaJunction fija R11(b)/D4 en Windows: un JUNCTION (mklink
// /J) es un reparse point que NO es un symlink POSIX —os.Lstat lo reporta con
// os.ModeIrregular, no os.ModeSymlink—. La compuerta vieja miraba sólo ModeSymlink
// y dejaba pasar el junction, que es la vía nativa de Windows para apuntar fuera de
// la raíz. isReparsePoint mira ModeSymlink|ModeIrregular, así que lo frena.
//
// Este test NO lleva directiva `// arnes:` a propósito, y por eso evita el
// encabezado canónico de corte al principio de línea: el corte honesto (sacar
// ModeIrregular de isReparsePoint) sólo se pone en ROJO corriendo ESTE test en
// Windows, y el arnés corre en Linux, donde el test se saltea; un skip se leería
// como verde y acreditaría cobertura que no existe. Se verifica a mano en
// davantis-1. El symlink POSIX ya lo cubre TestResolveSpecPathRechazaSymlink.
func TestResolveSpecPathRechazaJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("el junction (mklink /J) es específico de Windows; en POSIX lo cubre el symlink")
	}
	root := t.TempDir()
	fuera := t.TempDir()

	jx := filepath.Join(root, "jx")
	// mklink /J es un builtin de cmd (no un .exe), así que crear el junction NTFS pasa por cmd;
	// guiones.Portable es la vía DECLARADA para lanzarlo sin romper la guarda A127
	// (TestNingunaPruebaLanzaUnProcesoPorFueraDeLaCompuerta), que prohíbe exec.Command directo.
	// Sólo se llega acá en Windows (el t.Skip de arriba), donde cmd siempre está en el PATH.
	c := guiones.Portable(t, "mklink /J es un builtin de cmd y crea el junction NTFS que este test necesita en Windows", "cmd")
	if out, err := c.Comando("cmd", "/c", "mklink", "/J", jx, fuera).CombinedOutput(); err != nil {
		t.Skipf("no se pudo crear el junction (%v): %s", err, out)
	}

	if _, _, err := ResolveSpecPath(root, "jx/sub.md"); err == nil {
		t.Error("una ruta que pasa por un junction debía rechazarse (R11b/D4) y resolvió")
	} else if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("el rechazo debía mencionar el symlink (reparse point); obtuve: %v", err)
	}
}
