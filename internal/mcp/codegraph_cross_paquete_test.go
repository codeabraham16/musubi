package mcp

import (
	"context"
	"path/filepath"
	"testing"
)

// Tests del indexado de llamadas CROSS-PAQUETE (Track 20 · F8-A).
//
// El de codeintel prueba la derivación y la resolución en memoria. Éstos prueban lo que sólo se ve
// atravesando la persistencia, y que es el motivo por el que la feature se escribió con este cuidado:
// la arista cross-paquete la escribe el paquete del CALLER, y `UpsertPackageGraphFrom` BORRA por
// src_path antes de re-insertar. Si el refresco de un paquete no volviera a resolver sus llamadas
// cross, la arista se borraría sola en el próximo `save_code` — el grafo se degradaría con el uso, en
// silencio y en verde.

// proyectoDosPaquetes arma un módulo con `caller` llamando a `util.Ayuda()` y, de control, a stdlib.
func proyectoDosPaquetes(t *testing.T) (dir string, s *McpServer) {
	t.Helper()
	dir = t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n\ngo 1.26\n")
	writeFile(t, filepath.Join(dir, "caller", "caller.go"), `package caller

import (
	"fmt"
	"example.com/proj/util"
)

func Llama() {
	util.Ayuda()
	fmt.Println("control: stdlib no genera arista")
}
`)
	writeFile(t, filepath.Join(dir, "util", "util.go"), "package util\n\nfunc Ayuda() {}\n")
	return dir, newTestServerWithPath(t, dir)
}

func tieneArista(t *testing.T, s *McpServer, from, to string) bool {
	t.Helper()
	edges, err := s.engine.GraphOutEdgesCtx(context.Background(), from)
	if err != nil {
		t.Fatalf("GraphOutEdgesCtx(%s): %v", from, err)
	}
	for _, e := range edges {
		if e.ToKey == to && e.Kind == "CALLS" {
			return true
		}
	}
	return false
}

const (
	keyLlama = "caller/caller.go#func:Llama"
	keyAyuda = "util/util.go#func:Ayuda"
)

// El índice COMPLETO crea la arista cross-paquete, sin importar en qué orden recorra los directorios.
// Esta es la razón de que indexAllPackages tenga dos pasadas: con una sola, si `caller` se deriva
// antes que `util`, el símbolo destino todavía no está en el grafo y la arista no sale — y el índice
// terminaría "bien" con la arista faltante.
func TestIndiceCompleto_CreaLaAristaCrossPaquete(t *testing.T) {
	_, s := proyectoDosPaquetes(t)
	ctx := context.Background()

	if _, err := s.indexAllPackages(ctx); err != nil {
		t.Fatalf("indexAllPackages: %v", err)
	}
	if !tieneArista(t, s, keyLlama, keyAyuda) {
		t.Fatal("falta la arista CALLS cross-paquete Llama → Ayuda tras el índice completo")
	}
}

// R7 — EL TEST QUE JUSTIFICA LA FEATURE. Tras el índice completo, refrescar SÓLO el paquete del
// caller (que es lo que hace `save_code`) NO puede llevarse la arista puesta.
//
// Hacen falta DOS pasadas para probarlo: con una sola, el test pasaría igual aunque el refresco
// borrara la arista, porque nunca habría llegado a existir un estado previo que perder.
func TestRefrescoIncremental_NoSeLlevaLaAristaCrossPaquete(t *testing.T) {
	_, s := proyectoDosPaquetes(t)
	ctx := context.Background()

	if _, err := s.indexAllPackages(ctx); err != nil {
		t.Fatalf("indexAllPackages: %v", err)
	}
	if !tieneArista(t, s, keyLlama, keyAyuda) {
		t.Fatal("precondición rota: la arista tiene que existir ANTES del refresco, o el test no prueba nada")
	}

	// Sólo el paquete del caller, como haría un save_code sobre caller/caller.go.
	if err := s.refreshCodeGraphForPackage(ctx, "caller"); err != nil {
		t.Fatalf("refresco de caller: %v", err)
	}
	if !tieneArista(t, s, keyLlama, keyAyuda) {
		t.Error("el refresco incremental BORRÓ la arista cross-paquete: el grafo se degrada con el uso")
	}
}

// El control del test anterior. Si el refresco nunca retirara aristas, «sigue estando» no probaría
// que se re-resolvió — probaría que nadie la toca. Acá el caller deja de llamar y la arista TIENE
// que desaparecer.
func TestRefrescoIncremental_RetiraLaAristaCuandoYaNoSeLlama(t *testing.T) {
	dir, s := proyectoDosPaquetes(t)
	ctx := context.Background()

	if _, err := s.indexAllPackages(ctx); err != nil {
		t.Fatalf("indexAllPackages: %v", err)
	}
	if !tieneArista(t, s, keyLlama, keyAyuda) {
		t.Fatal("precondición rota: la arista tiene que existir antes de quitar la llamada")
	}

	writeFile(t, filepath.Join(dir, "caller", "caller.go"), `package caller

func Llama() {}
`)
	if err := s.refreshCodeGraphForPackage(ctx, "caller"); err != nil {
		t.Fatalf("refresco de caller: %v", err)
	}
	if tieneArista(t, s, keyLlama, keyAyuda) {
		t.Error("la arista sobrevivió a que el caller dejara de llamar: el refresco no la posee de verdad")
	}
}

// CONTROL NEGATIVO del indexado entero: una llamada a stdlib no produce arista CALLS. Sin esto,
// «el cross-paquete anda» no distingue entre resolver imports del módulo y tragarse cualquier
// selector — que sería llenar el grafo de ruido y hacer que `musubi_impact` mande a revisar código
// que no tiene nada que ver.
func TestIndiceCompleto_StdlibNoGeneraArista(t *testing.T) {
	_, s := proyectoDosPaquetes(t)
	ctx := context.Background()

	if _, err := s.indexAllPackages(ctx); err != nil {
		t.Fatalf("indexAllPackages: %v", err)
	}
	edges, err := s.engine.GraphOutEdgesCtx(ctx, keyLlama)
	if err != nil {
		t.Fatal(err)
	}
	llamadas := 0
	for _, e := range edges {
		if e.Kind != "CALLS" {
			continue
		}
		llamadas++
		if e.ToKey != keyAyuda {
			t.Errorf("arista CALLS inesperada: %s → %s", e.FromKey, e.ToKey)
		}
	}
	if llamadas != 1 {
		t.Errorf("esperaba exactamente 1 arista CALLS (la in-module), obtuve %d", llamadas)
	}
}
