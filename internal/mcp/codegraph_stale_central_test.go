package mcp

import (
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// REGRESIÓN (auditoría 2026-07-26, #3): cgStale/graphFreshness calculaban frescura leyendo el disco
// del server (s.projectPath). En el central COMPARTIDO el grafo es FEDERADO (nodos de otros proyectos,
// cuyos archivos no existen en el central) ⇒ TODO salía fantasma/stale y la cabina mostraba el código
// federado como podrido. Ahora, con forceRedact (el bind compartido), no se juzga frescura.
func TestCgStaleNotMarkedOnSharedCentral(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Nodo cuyo archivo NO existe en el disco del server (típico de un nodo federado en el central).
	n := memory.GraphNode{Key: "internal/foo/bar.go#func:X", Kind: "func", Name: "X", Path: "internal/foo/bar.go", SrcFingerprint: "deadbeef"}

	// Instancia LOCAL (sin forceRedact): el archivo no está en disco ⇒ es un fantasma legítimo (stale).
	local := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	if !local.cgStale(n) {
		t.Error("en una instancia local, un nodo cuyo archivo no existe debe reportarse stale (fantasma)")
	}

	// Instancia CENTRAL compartida (forceRedact): el grafo es federado, no hay árbol en disco ⇒ NO stale.
	central := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	central.forceRedact = true
	if central.cgStale(n) {
		t.Error("en el central compartido, un nodo federado NO debe reportarse stale (#3)")
	}
}
