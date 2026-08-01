package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// engineConArchivos crea un engine cuya raíz de proyecto es un tempdir con los archivos dados.
func engineConArchivos(t *testing.T, archivos map[string]string) (*DbEngine, string) {
	t.Helper()
	root := t.TempDir()
	for rel, contenido := range archivos {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	engine, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	return engine, root
}

func contarAnclas(t *testing.T, e *DbEngine, obsID string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observation_origins WHERE observation_id=?`, obsID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// La raíz del proyecto se deriva bien de la ruta de la base.
func TestProjectRootSaleDeLaRutaDeLaBase(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{"a.go": "package a\n"})
	if got := engine.projectRoot(); got != root {
		t.Errorf("projectRoot() = %q, quería %q", got, root)
	}
}

// Camino feliz: se guarda la observación y su ancla con el fingerprint del contenido.
func TestAnclaSeGuardaConElFingerprintActual(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{"src/a.go": "package a\n"})
	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "la nota", 1.0, "", "local",
		[]string{"src/a.go"}, nil); err != nil {
		t.Fatalf("save con ancla: %v", err)
	}
	var path, fp string
	if err := engine.db.QueryRow(
		`SELECT path, fingerprint FROM observation_origins WHERE observation_id='O1'`).Scan(&path, &fp); err != nil {
		t.Fatal(err)
	}
	if path != "src/a.go" {
		t.Errorf("path guardado = %q", path)
	}
	esperado, _ := FileFingerprint(root, "src/a.go")
	if fp != esperado {
		t.Errorf("el fingerprint guardado debe ser el del contenido actual")
	}
}

// R3: anclar a lo inexistente RECHAZA el guardado entero — ni observación ni ancla.
func TestAnclaInexistenteRechazaElGuardado(t *testing.T) {
	engine, _ := engineConArchivos(t, map[string]string{"src/a.go": "package a\n"})
	err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "la nota", 1.0, "", "local",
		[]string{"src/a.go", "no/existe.go"}, nil)
	if err == nil {
		t.Fatal("anclar a un archivo inexistente debe fallar")
	}
	if !strings.Contains(err.Error(), "no/existe.go") {
		t.Errorf("el error debe nombrar la ruta ofensora, obtuve: %v", err)
	}
	var n int
	engine.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id='O1'`).Scan(&n)
	if n != 0 {
		t.Error("si el ancla falla no debe quedar la observación guardada a medias")
	}
	if contarAnclas(t, engine, "O1") != 0 {
		t.Error("no deben quedar anclas de un guardado que falló")
	}
}

// D6: pasarse del tope es error, no truncado silencioso.
func TestAnclasTopeExcedidoEsError(t *testing.T) {
	archivos := map[string]string{}
	rutas := []string{}
	for i := 0; i < maxOriginPaths+1; i++ {
		rel := filepath.ToSlash(filepath.Join("src", string(rune('a'+i))+".go"))
		archivos[rel] = "package x\n"
		rutas = append(rutas, rel)
	}
	engine, _ := engineConArchivos(t, archivos)
	err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "la nota", 1.0, "", "local", rutas, nil)
	if err == nil {
		t.Fatalf("anclar a %d archivos debe fallar (tope %d)", len(rutas), maxOriginPaths)
	}
	if contarAnclas(t, engine, "O1") != 0 {
		t.Error("no debe guardar un subconjunto: es error, no truncado")
	}
}

// Rutas absolutas dentro del proyecto y duplicados se normalizan.
func TestAnclasNormalizaAbsolutasYDeduplica(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{"src/a.go": "package a\n"})
	abs := filepath.Join(root, "src", "a.go")
	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "la nota", 1.0, "", "local",
		[]string{abs, "src/a.go", "./src/a.go"}, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := contarAnclas(t, engine, "O1"); got != 1 {
		t.Errorf("las tres formas son el mismo archivo: esperaba 1 ancla, obtuve %d", got)
	}
}

// Anclar fuera de la raíz del proyecto se rechaza.
func TestAnclaFueraDelProyectoSeRechaza(t *testing.T) {
	engine, _ := engineConArchivos(t, map[string]string{"src/a.go": "package a\n"})
	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "n", 1.0, "", "local",
		[]string{"../afuera.go"}, nil); err == nil {
		t.Error("una ruta fuera del proyecto debe rechazarse")
	}
}

// R5: guardar SIN anclas no crea filas ni cambia nada.
func TestGuardarSinAnclasNoCreaFilas(t *testing.T) {
	engine, _ := engineConArchivos(t, map[string]string{"src/a.go": "package a\n"})
	if err := engine.SaveObservationTyped("O1", "t/k", "la nota", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	if contarAnclas(t, engine, "O1") != 0 {
		t.Error("sin origin_paths no debe haber anclas")
	}
}

// R14: borrar la observación borra sus anclas por FK (verifica de paso que el pragma
// foreign_keys está activo en el pool, no sólo declarado en el DDL).
func TestBorrarObservacionCascadeaLasAnclas(t *testing.T) {
	engine, _ := engineConArchivos(t, map[string]string{"src/a.go": "package a\n"})
	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "la nota", 1.0, "", "local",
		[]string{"src/a.go"}, nil); err != nil {
		t.Fatal(err)
	}
	if contarAnclas(t, engine, "O1") != 1 {
		t.Fatal("pre-condición: debía haber un ancla")
	}
	if _, err := engine.db.Exec(`DELETE FROM observations WHERE id='O1'`); err != nil {
		t.Fatal(err)
	}
	if got := contarAnclas(t, engine, "O1"); got != 0 {
		t.Errorf("la FK ON DELETE CASCADE debe llevarse las anclas, quedaron %d", got)
	}
}

// R15: el doctor detecta y cura anclas huérfanas. La FK las previene, pero una base
// restaurada de un backup viejo o escrita por una herramienta que abrió sin
// foreign_keys(1) puede dejarlas — por eso el check existe igual.
func TestDoctorDetectaYCuraAnclasHuerfanas(t *testing.T) {
	engine, _ := engineConArchivos(t, map[string]string{"src/a.go": "package a\n"})

	if c, _ := engine.RunCheck("orphan_origins"); c.Status != "ok" {
		t.Fatalf("pre-condición: sin anclas huérfanas debe dar ok, dio %q", c.Status)
	}
	// Insertar un ancla huérfana esquivando la FK (como lo haría una herramienta externa).
	if _, err := engine.db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.db.Exec(
		`INSERT INTO observation_origins (observation_id, path, fingerprint) VALUES ('NO-EXISTE','src/a.go','deadbeef')`); err != nil {
		t.Fatal(err)
	}
	engine.db.Exec(`PRAGMA foreign_keys=ON`)

	if c, _ := engine.RunCheck("orphan_origins"); c.Status == "ok" {
		t.Fatal("con un ancla huérfana el check no puede dar ok")
	}
	if _, err := engine.Repair("orphan_origins", "apply"); err != nil {
		t.Fatalf("repair: %v", err)
	}
	if c, _ := engine.RunCheck("orphan_origins"); c.Status != "ok" {
		t.Errorf("tras la reparación debe quedar ok, quedó %q", c.Status)
	}
}
