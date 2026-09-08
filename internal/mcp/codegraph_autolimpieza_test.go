package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// TestIncrementalLimpiaLaContaminacionYNoLaReplanta — el índice incremental se cura solo, PERO sólo
// con la guarda de contención puesta.
//
// El mecanismo tiene dos mitades que se pisaban entre sí:
//  1. Un nodo de otro árbol no está en el walk del proyecto, así que el incremental lo clasifica
//     FANTASMA y lo poda. Esa mitad ya funcionaba.
//  2. Pero acto seguido metía el directorio del fantasma en `dirtyDirs` —para soltar aristas
//     colgantes— y `dirExists` decía que sí, porque el directorio ajeno EXISTE de verdad. Lo
//     re-derivaba en el mismo tick y los nodos volvían. Neto: cero.
//
// Con `dentroDelProyecto` la segunda mitad se corta y la poda queda firme. Esto es lo que convierte
// la limpieza de un DELETE a mano contra la base en una corrida normal de la herramienta.
func TestIncrementalLimpiaLaContaminacionYNoLaReplanta(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	raiz := t.TempDir()
	if err := os.WriteFile(filepath.Join(raiz, "propio.go"), []byte("package p\n\nfunc Propia() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Un árbol AJENO que existe en disco: es la condición que hacía fracasar la poda.
	ajeno := t.TempDir()
	ajenoGo := filepath.Join(ajeno, "ajeno.go")
	if err := os.WriteFile(ajenoGo, []byte("package q\n\nfunc Ajena() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewMcpServer(engine, raiz, embedding.NoopProvider{})
	ctx := context.Background()

	// Se siembra el grafo como quedó en la base real: la clave del ajeno es ABSOLUTA, porque
	// NormalizeCodePath no puede relativizar lo que no cuelga de la raíz.
	claveAjena := memory.NormalizeCodePath(raiz, ajenoGo)
	if !filepath.IsAbs(claveAjena) {
		t.Fatalf("la clave del archivo ajeno tenía que salir absoluta y salió %q", claveAjena)
	}
	if err := engine.UpsertPackageGraph(
		[]string{claveAjena},
		[]memory.GraphNode{
			{Key: claveAjena, Kind: "file", Name: "ajeno.go", Path: claveAjena, SrcFingerprint: "x"},
			{Key: claveAjena + "#func:Ajena", Kind: "func", Name: "Ajena", Path: claveAjena, SrcFingerprint: "x"},
		}, nil); err != nil {
		t.Fatal(err)
	}

	// ⚠️ La contaminación se mide contra la CLAVE CONCRETA que se sembró, NO con
	// s.dentroDelProyecto(). Usar acá el mismo predicado que el código bajo prueba lo volvería su
	// propio oráculo: al sabotear la guarda, el test dejaría de VER la contaminación en vez de
	// fallar por ella, y un sabotaje que no se nota es un test que no prueba nada.
	if _, ok := fpsDe(t, s)[claveAjena]; !ok {
		t.Fatal("el escenario no quedó contaminado: no hay nada que limpiar y el test no probaría nada")
	}

	if _, err := s.indexIncremental(ctx); err != nil {
		t.Fatalf("indexIncremental falló: %v", err)
	}

	// MITAD 1 — el nodo ajeno no está, ni volvió.
	if _, ok := fpsDe(t, s)[claveAjena]; ok {
		t.Errorf("sobrevivió (o se replantó) el nodo de otro árbol: %s", claveAjena)
	}
	// MITAD 2 — y el propio SÍ se indexó. Sin esto, un índice que borrara todo pasaría por limpio.
	propio := false
	for p := range fpsDe(t, s) {
		if p == "propio.go" {
			propio = true
		}
	}
	if !propio {
		t.Error("el archivo propio no quedó en el grafo: el índice no está haciendo su trabajo")
	}
}
