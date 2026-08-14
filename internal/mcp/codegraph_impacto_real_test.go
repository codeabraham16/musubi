package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Verificación de ACEPTACIÓN de F8-A contra el CÓDIGO REAL de Musubi, no contra un fixture.
//
// El defecto que motivó la feature se midió acá adentro: `musubi_impact` sobre
// `internal/codeintel/graph.go#func:DerivePackage` devolvía 9 callers y los 9 eran tests de su
// propio paquete. El único caller de producción vive en `internal/mcp/methods_codegraph.go` y no
// aparecía, porque ninguna arista CALLS cruzaba la frontera del paquete.
//
// Un fixture sintético no cierra ese lazo: prueba el mecanismo, no que el salto real exista. Este
// test copia los DOS paquetes de verdad a un módulo temporal, indexa y pregunta lo mismo que se
// preguntó al empezar. La derivación sólo PARSEA (go/ast), así que no hace falta que el módulo
// compile ni que estén sus dependencias — por eso alcanza con dos directorios.
//
// Escribe en una base temporal: no toca el .musubi del repo.

// copiarGo copia los .go de un directorio del repo al módulo temporal. Sin recursión: la unidad del
// grafo de Go es el paquete, o sea el directorio.
func copiarGo(t *testing.T, origen, destino string) int {
	t.Helper()
	if err := os.MkdirAll(destino, 0o755); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(origen)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", origen, err)
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(origen, e.Name()))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(destino, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
		n++
	}
	return n
}

func TestImpactoReal_ElSaltoEntrePaquetesSeVe(t *testing.T) {
	repo := filepath.Join("..", "..") // este test corre en internal/mcp
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module musubi\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := copiarGo(t, filepath.Join(repo, "internal", "codeintel"), filepath.Join(tmp, "internal", "codeintel")); n == 0 {
		t.Fatal("no se copió ningún .go de internal/codeintel: el test no estaría midiendo nada")
	}
	if n := copiarGo(t, filepath.Join(repo, "internal", "mcp"), filepath.Join(tmp, "internal", "mcp")); n == 0 {
		t.Fatal("no se copió ningún .go de internal/mcp")
	}

	s := newTestServerWithPath(t, tmp)
	ctx := context.Background()
	if _, err := s.indexAllPackages(ctx); err != nil {
		t.Fatalf("indexAllPackages: %v", err)
	}

	const derive = "internal/codeintel/graph.go#func:DerivePackage"
	callers, err := s.engine.GraphImpactCtx(ctx, derive, 5, 200)
	if err != nil {
		t.Fatalf("GraphImpactCtx: %v", err)
	}

	// EL CALLER DE PRODUCCIÓN, que es exactamente el que faltaba cuando se midió el defecto.
	// Ojo con el kind: lleva receiver, así que su clave es `#method:`, no `#func:`.
	const prod = "internal/mcp/methods_codegraph.go#method:McpServer.refreshCodeGraphPkg"

	tiene := func(k string) bool {
		for _, c := range callers {
			if c == k {
				return true
			}
		}
		return false
	}

	if !tiene(prod) {
		t.Errorf("impact sobre DerivePackage NO incluye a su caller de producción %s.\ncallers = %v", prod, callers)
	}

	// EL SALTO POR ENVOLTORIO, que hasta F8-C se cortaba. `refreshCodeGraphForPackage` no llama a
	// `DerivePackage`: sólo delega en `s.refreshCodeGraphPkg(...)`, un método sobre su propio
	// receptor. Mientras ningún método fue DESTINO de una llamada (medido: 4.115 aristas CALLS y
	// CERO llegando a un método), el cierre transitivo moría en el primer envoltorio — y en un
	// paquete como éste, donde casi todo es `McpServer.*`, eso dejaba a `impact` casi ciego.
	//
	// Este test fue escrito en F8-A como aserción INVERTIDA («si algún día aparece, avisá»), y en
	// F8-C disparó con ese mensaje. Ahora afirma lo contrario, que es el punto: un límite declarado
	// en un test se entera solo de cuándo deja de existir.
	const envoltorio = "internal/mcp/methods_codegraph.go#method:McpServer.refreshCodeGraphForPackage"
	if !tiene(envoltorio) {
		t.Errorf("impact no llegó a %s: el cierre transitivo se cortó en el envoltorio.\ncallers = %v", envoltorio, callers)
	}

	// CONTROL: que el impact no se haya vuelto un "todo con todo". Un símbolo del mismo paquete que
	// NADIE llama tiene que seguir dando cero — si diera callers, las aristas nuevas serían ruido.
	solo, err := s.engine.GraphImpactCtx(ctx, "internal/codeintel/crosspkg.go#func:NewModuleIndex", 5, 200)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range solo {
		if strings.Contains(c, "#func:DerivePackage") {
			t.Errorf("NewModuleIndex reporta a DerivePackage como caller: el grafo está conectando de más (%v)", solo)
		}
	}
}
