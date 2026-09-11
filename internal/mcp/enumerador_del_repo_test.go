package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// EL ENUMERADOR DEFINE QUÉ ES «EL REPO» PARA TODA GUARDA QUE LEA ARCHIVOS.
//
// `archivosDelRepo` (y sus dos filtros, `archivosGo` y `archivosDeGuiones`) es de dónde salen los
// archivos que varias guardas parsean para decidir. Si ese alcance incluye lo que cada uno tenga
// tirado en su árbol de trabajo, el veredicto pasa a depender de la máquina: la guarda **falla
// sólo en local y pasa SIEMPRE en CI**, que es el peor de los dos mundos —el CI no la puede ver y
// la persona no le puede creer.
//
// MEDIDO EL 2026-09-11: `TestElTopePorConteoDelOutboxNoTieneQuienLoLea` acusaba a tres lectores de
// `Sync.MaxAttempts`, y los tres vivían en `.musubi/backups/rescate-worktrees-20260908/…` — un
// respaldo, sin trackear e ignorado por `.gitignore`, que el barrido de disco leía como código del
// repo.

// intrusoSinTrackear deja un archivo en el árbol que git NO conoce, y lo saca al terminar.
func intrusoSinTrackear(t *testing.T, raiz, carpeta, nombre, contenido string) string {
	t.Helper()
	base := filepath.Join(raiz, filepath.FromSlash(carpeta))
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("crear %s: %v", base, err)
	}
	dir, err := os.MkdirTemp(base, "zz-prueba-enumerador-")
	if err != nil {
		t.Fatalf("crear el directorio del intruso en %s: %v", base, err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, nombre), []byte(contenido), 0o644); err != nil {
		t.Fatalf("escribir el intruso: %v", err)
	}
	rel, err := filepath.Rel(raiz, filepath.Join(dir, nombre))
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

// N1 — LO QUE GIT NO TRACKEA NO ES DEL REPO, EN LAS DOS FORMAS EN QUE ESO PASA.
//
// Se prueban las DOS a propósito, y no una: un archivo sin trackear que `.gitignore` **no**
// menciona, y otro que **sí**. La segunda es la que se midió de verdad —el respaldo bajo
// `.musubi/backups/`—, así que una guarda que sólo cubriera la primera no cubriría el defecto que
// la motivó. Son dos flags distintas de `git ls-files` y se puede fallar una sin fallar la otra.
func TestElEnumeradorMiraElRepoYNoElDisco(t *testing.T) {
	raiz := filepath.Join("..", "..")

	noIgnorado := intrusoSinTrackear(t, raiz, "deploy", "intruso.go", "package zz\n")
	ignorado := intrusoSinTrackear(t, raiz, ".musubi/backups", "intruso.go", "package zz\n")

	gos := archivosGo(t, raiz)

	// CONTROL POSITIVO PRIMERO. Sin esto, un enumerador que devolviera la lista vacía pasaría las
	// dos aserciones de abajo con las mejores notas: «no está el intruso» y «no medí nada» se
	// escriben igual.
	if !contiene(gos, "internal/mcp/methods.go") {
		t.Fatalf("el enumerador no devolvió `internal/mcp/methods.go`, que está trackeado: "+
			"no estoy midiendo el repo (devolvió %d archivos)", len(gos))
	}

	if contiene(gos, noIgnorado) {
		t.Errorf("%s no está trackeado y el enumerador lo devolvió: el alcance sale del DISCO y no del repo", noIgnorado)
	}
	if contiene(gos, ignorado) {
		t.Errorf("%s está IGNORADO por .gitignore y el enumerador lo devolvió. Es el caso medido el "+
			"2026-09-11: un respaldo leído como si fuera código del repo", ignorado)
	}
}

// N2 — EL HERMANO. El enumerador de guiones comparte la fuente, así que comparte el arreglo.
//
// Va aparte y no como una aserción más de G1 porque el defecto dominante de este repo es la
// lección aprendida de un lado y no del hermano: `archivosDeGuiones` barría `deploy/` y
// `scripts/` con la misma técnica, y un `.sh` sin trackear ahí entraba igual. Con el alcance
// derivado de git, los dos se arreglan de una sola vez — y esto lo fija.
func TestElEnumeradorDeGuionesTampocoMiraElDisco(t *testing.T) {
	raiz := filepath.Join("..", "..")

	intruso := intrusoSinTrackear(t, raiz, "deploy", "intruso.sh", "#!/usr/bin/env bash\necho hola\n")

	shs, _, _ := archivosDeGuiones(t, raiz)

	if !contiene(shs, "deploy/construir.sh") {
		t.Fatalf("el enumerador de guiones no devolvió `deploy/construir.sh`, que está trackeado: "+
			"no estoy midiendo el repo (devolvió %d .sh)", len(shs))
	}
	if contiene(shs, intruso) {
		t.Errorf("%s no está trackeado y el enumerador de guiones lo devolvió", intruso)
	}
}

// N3 — EL ALCANCE POR CARPETA SE CONSERVA.
//
// El arreglo cambió de dónde sale la lista, y eso NO puede cambiar qué carpetas mira. `.` en
// `carpetasDeGuiones` significa «los archivos SUELTOS de la raíz» —ahí viven los `.bat` de doble
// clic— y no «el repo entero colgando de la raíz». Si `enCarpetaDeGuiones` tratara `.` como un
// prefijo cualquiera, el inventario de guiones se tragaría todo el árbol y las guardas que lo
// usan pasarían a exigir cosas de archivos que no les tocan.
func TestElAlcancePorCarpetaSobreviveAlCambioDeFuente(t *testing.T) {
	casos := []struct {
		ruta    string
		adentro bool
		porQue  string
	}{
		{"deploy/construir.sh", true, "deploy/ se mira entera"},
		{"deploy/pruebas/sabotaje.sh", true, "y en profundidad"},
		{"scripts/algo.sh", true, "scripts/ también"},
		{"install.bat", true, "un archivo SUELTO de la raíz: es el de doble clic"},
		{"internal/mcp/algo.sh", false, "un .sh de otra carpeta NO es un guion de despliegue"},
		{"deployment/otro.sh", false, "`deploy` no puede matchear a `deployment` por prefijo pelado"},
	}
	for _, c := range casos {
		if got := enCarpetaDeGuiones(c.ruta); got != c.adentro {
			t.Errorf("enCarpetaDeGuiones(%q) = %v, esperaba %v — %s", c.ruta, got, c.adentro, c.porQue)
		}
	}
}
