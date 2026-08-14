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

	// ⚠️ LÍMITE CONOCIDO, y este test lo FIJA en vez de dejarlo como sorpresa.
	// `refreshCodeGraphForPackage` envuelve a `refreshCodeGraphPkg`, pero la llamada es
	// `s.refreshCodeGraphPkg(...)`: un método sobre un receptor. Ni F1 ni F8-A resuelven eso —
	// F1 sólo resuelve contra las funcs TOP-LEVEL del paquete, y F8-A contra las funcs top-level de
	// otro paquete. Medido sobre el código real de Musubi (3 paquetes, 3.062 aristas CALLS):
	// 2.537 aristas salen de funcs top-level y CERO apuntan a un método.
	//
	// O sea: hoy ningún método es destino de una llamada, así que el cierre transitivo se corta en
	// el primer envoltorio. NO es un defecto de este cambio; es la superficie que sigue abierta.
	// Si algún día se resuelven las llamadas a métodos, este test va a fallar acá y hay que
	// convertirlo en una aserción positiva.
	const envoltorio = "internal/mcp/methods_codegraph.go#method:McpServer.refreshCodeGraphForPackage"
	if tiene(envoltorio) {
		t.Errorf("¡%s ahora SÍ aparece! Se resolvieron las llamadas a métodos: actualizá este test y la nota de arriba", envoltorio)
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
