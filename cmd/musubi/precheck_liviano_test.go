package main

// Tests del hook LIVIANO: `musubi precheck --hook-mode` decide ANTES de abrir la base, y cuando
// abre no hace el trabajo de arranque de un daemon. Ver precheckHook y
// internal/memory/sin_arranque.go.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// abridorQueDejaRastro abre con NewDbEngine, que CREA .musubi y la base. Así «el hook abrió» se ve
// desde afuera como una carpeta que existe, y no depende de que la prueba confíe en un contador.
func abridorQueDejaRastro(llamadas *int) abridorDelHook {
	return func(root string) (storeDelHook, error) {
		*llamadas++
		eng, err := memory.NewDbEngine(root)
		if err != nil {
			return nil, err
		}
		return eng, nil
	}
}

func existeMusubi(root string) bool {
	_, err := os.Stat(filepath.Join(root, config.DirName))
	return err == nil
}

// L1 — un evento que no le toca no abre nada. El caso nombrado es el de todos los días: un Bash.
func TestPrecheckNoAbreLaBaseSiNoLeToca(t *testing.T) {
	casos := []struct{ nombre, evento string }{
		{"Bash ls", `{"tool_name":"Bash","tool_input":{"command":"ls"},"session_id":"s"}`},
		{"Read sin file_path", `{"tool_name":"Read","tool_input":{},"session_id":"s"}`},
		{"Edit sin file_path", `{"tool_name":"Edit","tool_input":{"old_string":"a"},"session_id":"s"}`},
		{"Grep con path (no file_path)", `{"tool_name":"Grep","tool_input":{"pattern":"x","path":"a.go"},"session_id":"s"}`},
		{"tool ajena con file_path", `{"tool_name":"Glob","tool_input":{"file_path":"a.go"},"session_id":"s"}`},
		{"JSON roto", `{"tool_name":`},
		{"stdin vacío", ``},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			root := t.TempDir()
			llamadas := 0
			var stderr bytes.Buffer
			out := precheckHook(root, strings.NewReader(c.evento), abridorQueDejaRastro(&llamadas), &stderr)
			if out != "" {
				t.Errorf("no le tocaba y produjo salida: %q", out)
			}
			if existeMusubi(root) {
				t.Errorf("el hook abrió la base para un evento que no le tocaba: %s existe", config.DirName)
			}
			if llamadas != 0 {
				t.Errorf("el abridor se llamó %d vez/veces", llamadas)
			}
		})
	}

	// CONTROL: el mismo abridor, con un evento que SÍ le toca, deja la carpeta. Sin esto, «.musubi
	// no existe» podría ser verde porque el abridor no crea nada, y la prueba no mediría el orden.
	t.Run("control: un Read con file_path sí abre", func(t *testing.T) {
		root := t.TempDir()
		llamadas := 0
		in := `{"tool_name":"Read","tool_input":{"file_path":"a.go"},"session_id":"s"}`
		precheckHook(root, strings.NewReader(in), abridorQueDejaRastro(&llamadas), io.Discard)
		if llamadas != 1 || !existeMusubi(root) {
			t.Fatalf("el control no abrió (llamadas=%d, existe=%v): la prueba de arriba no mide nada", llamadas, existeMusubi(root))
		}
	})
}

// L2 — el abridor REAL no materializa una base donde no la hay, ni avisa por stderr: un proyecto
// sin Musubi no es un error.
func TestPrecheckSinBaseNoLaCreaNiAvisa(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.go", "package a\n")
	var stderr bytes.Buffer
	for _, tool := range []string{"Read", "Edit"} {
		in := `{"tool_name":"` + tool + `","tool_input":{"file_path":"a.go"},"session_id":"s"}`
		if out := precheckHook(root, strings.NewReader(in), abrirMemoriaDelHook, &stderr); out != "" {
			t.Errorf("%s sin base no debía producir salida: %q", tool, out)
		}
	}
	if existeMusubi(root) {
		t.Errorf("el hook creó %s en un proyecto que no lo tenía", config.DirName)
	}
	if stderr.Len() != 0 {
		t.Errorf("sin base el hook tiene que callar, escribió en stderr: %q", stderr.String())
	}
}

// L4 — `.musubi/` con config.yaml y SIN memory.db: tampoco crea la base ni avisa.
//
// Es el estado real que deja ensureWorkspace (la carpeta y la config existen, la base todavía no),
// y L2 no lo cubre porque su root no tiene `.musubi`: una guarda que mirara la CARPETA en vez del
// archivo pasaba L2 verde. Con la carpeta presente esa guarda deja pasar a sql.Open, el driver crea
// una memory.db vacía y cada Read/Edit escribe «no such table: meta» en stderr.
// Sabotaje: que el Stat de NewDbEngineSinArranque mire filepath.Dir(dbPath) → rojo acá.
func TestPrecheckConCarpetaYSinBaseNoLaCreaNiAvisa(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.go", "package a\n")
	writeFile(t, root, filepath.Join(config.DirName, config.ConfigFile), "# proyecto con config y sin base todavía\n")
	var stderr bytes.Buffer
	for _, tool := range []string{"Read", "Edit"} {
		in := `{"tool_name":"` + tool + `","tool_input":{"file_path":"a.go"},"session_id":"s"}`
		if out := precheckHook(root, strings.NewReader(in), abrirMemoriaDelHook, &stderr); out != "" {
			t.Errorf("%s con .musubi y sin base no debía producir salida: %q", tool, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, config.DirName, config.DBFile)); !os.IsNotExist(err) {
		t.Errorf("el hook creó %s en un proyecto con %s pero sin base (stat: %v)", config.DBFile, config.DirName, err)
	}
	if stderr.Len() != 0 {
		t.Errorf("con .musubi y sin base el hook tiene que callar, escribió en stderr: %q", stderr.String())
	}
}

// L3 — con el abridor REAL, la contabilidad de las superficies precheck_* sigue sumando en la base.
// Es lo que se perdería en silencio si alguien «optimizara» el hook con NewDbEngineSoloLectura.
func TestPrecheckLivianoConservaElLedger(t *testing.T) {
	t.Setenv("MUSUBI_CODEGRAPH_HOOK", "")
	root := t.TempDir()
	const svc = "package svc\nfunc Bar(){}\n"
	writeFile(t, root, "svc.go", svc)
	writeFile(t, root, "alertas.sql", "select 1;\n")
	memtest.Sembrar(t, root)

	// La siembra va por NewDbEngine (el camino de siempre) y se cierra antes de correr el hook.
	eng, err := memory.NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	fp, _ := memory.FileFingerprint(root, "svc.go")
	if err := eng.SaveCodeMemory(memory.CodeMemory{Path: "svc.go", Gist: "Paquete svc con Bar().", Symbols: "Bar() L2", Fingerprint: fp}); err != nil {
		t.Fatalf("SaveCodeMemory: %v", err)
	}
	if err := eng.SaveTelemetryLog("svc.go", "nil pointer en Bar", "chequear nil"); err != nil {
		t.Fatalf("SaveTelemetryLog: %v", err)
	}
	nodos := []memory.GraphNode{{Key: "svc.go#func:Bar", Kind: "func", Name: "Bar", Path: "svc.go", StartLine: 2, EndLine: 2, SrcFingerprint: fp}}
	aristas := []memory.GraphEdge{{FromKey: "main.go#func:main", ToKey: "svc.go#func:Bar", Kind: "CALLS"}}
	if err := eng.UpsertPackageGraph([]string{"svc.go"}, nodos, aristas); err != nil {
		t.Fatalf("UpsertPackageGraph: %v", err)
	}
	eng.Close()

	eventos := []string{
		`{"tool_name":"Read","tool_input":{"file_path":"svc.go"},"session_id":"sesion-liviana"}`,
		`{"tool_name":"Edit","tool_input":{"file_path":"svc.go"},"session_id":"sesion-liviana"}`,
		`{"tool_name":"Write","tool_input":{"file_path":"alertas.sql"},"session_id":"sesion-liviana"}`,
	}
	var stderr bytes.Buffer
	for _, ev := range eventos {
		if out := precheckHook(root, strings.NewReader(ev), abrirMemoriaDelHook, &stderr); out == "" {
			t.Fatalf("el evento %s tenía que producir contexto; stderr=%q", ev, stderr.String())
		}
	}

	leer, err := memory.NewDbEngineSinArranque(root)
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	defer leer.Close()
	l, err := leer.LedgerStatus()
	if err != nil {
		t.Fatalf("LedgerStatus: %v", err)
	}
	if l.SessionID != "sesion-liviana" {
		t.Errorf("el ledger tiene que quedar en la sesión del hook, quedó %q", l.SessionID)
	}
	for _, sup := range []string{"precheck_code", "precheck_telemetry", "precheck_codegraph", "precheck_impacto", "precheck_sin_grafo"} {
		if l.Surfaces[sup] <= 0 {
			t.Errorf("la superficie %s no sumó en el ledger: %v", sup, l.Surfaces)
		}
	}
}
