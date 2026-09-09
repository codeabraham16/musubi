package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// servidorConArbol arma un server con un árbol real y un .go adentro.
func servidorConArbol(t *testing.T) (*McpServer, string) {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })

	raiz := t.TempDir()
	if err := os.WriteFile(filepath.Join(raiz, "propio.go"), []byte("package p\n\nfunc X() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewMcpServer(engine, raiz, embedding.NoopProvider{}), raiz
}

// TestContencionRechazaOtroArbol — LA REGRESIÓN. Medido el 2026-09-05: 264 nodos con ruta absoluta
// de C:/Proyectos/musubi-body vivían en el grafo del proyecto Musubi, con project_id='musubi'.
//
// El caso que importa es el del directorio AJENO QUE EXISTE: el `os.ReadDir` de más abajo lo lee sin
// problema, así que ningún error de disco lo iba a frenar. La única defensa posible es la guarda.
func TestContencionRechazaOtroArbol(t *testing.T) {
	s, raiz := servidorConArbol(t)

	ajeno := t.TempDir() // existe de verdad, y es otro árbol
	if err := os.WriteFile(filepath.Join(ajeno, "ajeno.go"), []byte("package q\n\nfunc Y() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.refreshCodeGraphPkg(context.Background(), ajeno); !errors.Is(err, errFueraDelProyecto) {
		t.Fatalf("indexar un directorio de otro árbol devolvió %v, esperaba errFueraDelProyecto", err)
	}

	// LA OTRA MITAD: no se escribió NADA. Un error que igual deja el nodo plantado no sirve de nada.
	fps, err := s.engine.GraphFileFingerprintsCtx(s.scopedCtx(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	for path := range fps {
		if !s.dentroDelProyecto(path) {
			t.Errorf("quedó un nodo de otro árbol en el grafo: %s", path)
		}
	}

	// CONTROL POSITIVO: sin él, un test que sólo comprueba el rechazo pasaría igual con una guarda
	// rota que rechace TODO, y el índice quedaría sin indexar nada.
	if _, err := s.refreshCodeGraphPkg(context.Background(), raiz); err != nil {
		t.Fatalf("indexar la raíz del PROPIO proyecto falló: %v", err)
	}
	if len(fpsDe(t, s)) == 0 {
		t.Error("indexar el propio árbol no dejó ningún nodo: la guarda está rechazando de más")
	}
}

func fpsDe(t *testing.T, s *McpServer) map[string]string {
	t.Helper()
	fps, err := s.engine.GraphFileFingerprintsCtx(s.scopedCtx(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	return fps
}

// TestContencionNoSeEnganaConElPrefijoDeTexto — un hermano cuyo nombre EMPIEZA igual que la raíz.
//
// Es la trampa clásica de comparar con strings.HasPrefix: "/tmp/proy-otro" empieza con "/tmp/proy",
// así que un prefijo textual lo daría por adentro. Por eso la comparación va con filepath.Rel.
func TestContencionNoSeEnganaConElPrefijoDeTexto(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	padre := t.TempDir()
	raiz := filepath.Join(padre, "proy")
	hermano := filepath.Join(padre, "proy-otro") // comparte prefijo textual, NO es un subdirectorio
	for _, d := range []string{raiz, hermano} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := NewMcpServer(engine, raiz, embedding.NoopProvider{})

	if s.dentroDelProyecto(hermano) {
		t.Errorf("%s se dio por dentro de %s: la contención está comparando prefijos de texto", hermano, raiz)
	}
	if !s.dentroDelProyecto(filepath.Join(raiz, "sub", "a.go")) {
		t.Error("un subdirectorio propio se dio por fuera")
	}
	// Un directorio cuyo nombre EMPIEZA con ".." es legítimo: rechazarlo sería mirar `..` como
	// prefijo de texto en vez de como segmento de ruta.
	if !s.dentroDelProyecto(filepath.Join(raiz, "..datos", "a.go")) {
		t.Error("un directorio llamado '..datos' se dio por fuera: se está mirando '..' como texto")
	}
	// Y el escape explícito sí tiene que caer.
	if s.dentroDelProyecto(filepath.Join(raiz, "..", "afuera.go")) {
		t.Error("una ruta con '..' que sale del árbol se dio por dentro")
	}
}

// TestSaveCodeRechazaOtroArbol — la otra puerta. El contrato PUBLICADO decía «relativa a la raíz del
// proyecto o absoluta», así que un agente que guardara un archivo de otro repo hacía exactamente lo
// que la descripción permitía. NormalizeCodePath conserva la absoluta en silencio.
func TestSaveCodeRechazaOtroArbol(t *testing.T) {
	s, raiz := servidorConArbol(t)
	ajeno := filepath.Join(t.TempDir(), "otro.go")
	if err := os.WriteFile(ajeno, []byte("package q\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pedido := func(p string) json.RawMessage {
		b, _ := json.Marshal(map[string]string{"path": p, "gist": "algo"})
		return b
	}

	if _, rpcErr := s.toolSaveCode(context.Background(), pedido(ajeno)); rpcErr == nil {
		t.Error("save_code aceptó un archivo de otro árbol")
	}
	// CONTROL POSITIVO en las dos formas que SÍ son legítimas: relativa, y absoluta pero adentro.
	if _, rpcErr := s.toolSaveCode(context.Background(), pedido("propio.go")); rpcErr != nil {
		t.Errorf("save_code rechazó una ruta relativa propia: %v", rpcErr)
	}
	if _, rpcErr := s.toolSaveCode(context.Background(), pedido(filepath.Join(raiz, "propio.go"))); rpcErr != nil {
		t.Errorf("save_code rechazó una ruta ABSOLUTA que cae adentro del proyecto: %v", rpcErr)
	}
}

// TestContencionNoJuzgaSinArbol — varias instancias se construyen con projectPath vacío (tests, y el
// bind compartido). Sin raíz, toda ruta saldría "fuera" y la guarda rompería lo que hoy anda.
func TestContencionNoJuzgaSinArbol(t *testing.T) {
	s := NewMcpServer(nil, "", embedding.NoopProvider{})
	if !s.dentroDelProyecto("cualquier/cosa.go") {
		t.Error("sin projectPath no se puede juzgar contención, y se está juzgando")
	}
}
