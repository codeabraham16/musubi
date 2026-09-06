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

// escribirGo deja un .go mínimo en disco y devuelve su fingerprint.
func escribirGo(t *testing.T, raiz, rel, cuerpo string) string {
	t.Helper()
	full := filepath.Join(raiz, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(cuerpo), 0o644); err != nil {
		t.Fatal(err)
	}
	fp, err := memory.FileFingerprint(raiz, rel)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

// TestGraphFreshnessCuentaLosQueNuncaSeIndexaron — LA REGRESIÓN QUE ESTE CONTADOR EXISTE PARA CAZAR.
//
// stale y ghosts se derivan recorriendo los archivos que YA ESTÁN en el grafo, así que su dominio es
// el grafo y no el repo: un archivo que nunca se indexó no puede salir ni stale ni fantasma POR
// CONSTRUCCIÓN. Medido el 2026-09-05 sobre el repo real: 213 de 798 `.go` sin un solo nodo
// (internal/fleet entero entre ellos) y musubi_map informaba `stale:113, ghosts:0` sin nombrarlos.
//
// El test afirma LAS DOS MITADES a propósito, y cada una cae por su lado:
//   - missing==1 se rompe si el contador deja de mirar el disco, o si se pasa de largo y cuenta
//     también el archivo que sí está indexado.
//   - stale==0 && ghosts==0 congela POR QUÉ hacía falta un contador nuevo. Si alguien "arregla" la
//     ceguera haciendo que ghosts cuente los ausentes, cambia la semántica de un campo que ya se
//     publica, y esto lo caza.
func TestGraphFreshnessCuentaLosQueNuncaSeIndexaron(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	raiz := t.TempDir()
	fp := escribirGo(t, raiz, "indexado.go", "package p\n\nfunc X() {}\n")
	escribirGo(t, raiz, "nunca.go", "package p\n\nfunc Y() {}\n")

	// Sólo el primero entra al grafo, y con el fingerprint correcto: así no puede salir stale.
	if err := engine.UpsertPackageGraph(
		[]string{"indexado.go"},
		[]memory.GraphNode{{
			Key: "indexado.go#func:X", Kind: "func", Name: "X",
			Path: "indexado.go", SrcFingerprint: fp,
		}}, nil); err != nil {
		t.Fatal(err)
	}

	s := NewMcpServer(engine, raiz, embedding.NoopProvider{})
	stale, ghosts, missing := s.graphFreshness(s.scopedCtx(context.Background()))

	if missing != 1 {
		t.Errorf("missing = %d, esperaba 1 (nunca.go está en disco y no tiene un solo nodo)", missing)
	}
	if stale != 0 || ghosts != 0 {
		t.Errorf("stale=%d ghosts=%d, esperaba 0 y 0: son ciegos a un archivo no indexado, y "+
			"ésa es justamente la razón por la que existe missing", stale, ghosts)
	}
}

// TestGraphFreshnessNoCuentaLoQueElIndexadorNiMira — el denominador tiene que ser el del INDEXADOR.
//
// Si `missing` contara contra una enumeración propia, mediría el desacuerdo entre dos listas en vez
// de la ceguera del grafo: cada archivo bajo testdata/, vendor/ o node_modules/ —que el indexador
// salta a propósito— aparecería como ausente para siempre, y un contador que nunca puede llegar a
// cero se aprende a ignorar.
func TestGraphFreshnessNoCuentaLoQueElIndexadorNiMira(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	raiz := t.TempDir()
	// Ninguno de éstos debe contar: el indexador los excluye por directorio.
	for _, rel := range []string{
		"testdata/x.go", "vendor/v.go", "node_modules/n.go", "dist/d.go", ".oculto/h.go",
	} {
		escribirGo(t, raiz, rel, "package p\n")
	}
	// Éste sí, y es el control positivo: sin él, un missing==0 no distinguiría "las exclusiones
	// funcionan" de "el contador está muerto".
	escribirGo(t, raiz, "visible.go", "package p\n")

	s := NewMcpServer(engine, raiz, embedding.NoopProvider{})
	_, _, missing := s.graphFreshness(s.scopedCtx(context.Background()))

	if missing != 1 {
		t.Errorf("missing = %d, esperaba exactamente 1 (sólo visible.go): las rutas excluidas por el "+
			"indexador no pueden contarse como ausentes", missing)
	}
}

// TestGraphFreshnessNoJuzgaEnElCentralCompartido — mismo motivo que #3 en cgStale: en el central el
// grafo es FEDERADO y su árbol no está en disco, así que TODO saldría ausente. Un `missing` enorme
// e inevitable ahí sería peor que no informarlo.
func TestGraphFreshnessNoJuzgaEnElCentralCompartido(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	raiz := t.TempDir()
	escribirGo(t, raiz, "suelto.go", "package p\n")

	s := NewMcpServer(engine, raiz, embedding.NoopProvider{})
	s.forceRedact = true
	stale, ghosts, missing := s.graphFreshness(s.scopedCtx(context.Background()))
	if stale != 0 || ghosts != 0 || missing != 0 {
		t.Errorf("en el central compartido no se juzga frescura: stale=%d ghosts=%d missing=%d",
			stale, ghosts, missing)
	}
}
