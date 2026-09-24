package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func fechaPub(t *testing.T, s string) time.Time {
	t.Helper()
	en, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return en
}

// La regla de la guarda, caso por caso. El caso medido el 2026-09-24 es «más viejo»: la laptop
// publicó una rama del 12/09 encima del grafo del 23/09.
func TestDecidirPublicacionDelGrafo(t *testing.T) {
	nuevo := PublicacionDelGrafo{Head: "fc3c297aaaa", En: fechaPub(t, "2026-09-23T17:00:00Z")}
	viejo := PublicacionDelGrafo{Head: "8de3b00bbbb", En: fechaPub(t, "2026-09-12T16:04:09Z")}
	casos := []struct {
		nombre         string
		vigente, llega PublicacionDelGrafo
		decision       decisionPublicacion
	}{
		{"nada publicado, llega con commit", PublicacionDelGrafo{}, nuevo, aceptarPublicacion},
		{"nada publicado, llega sin commit (estado de antes de la guarda)", PublicacionDelGrafo{}, PublicacionDelGrafo{}, aceptarPublicacion},
		{"llega MÁS VIEJO que lo publicado", nuevo, viejo, rechazarPublicacion},
		{"llega sin commit y lo publicado lo tiene (binario viejo)", nuevo, PublicacionDelGrafo{}, ignorarPublicacion},
		{"mismo commit (re-empuje de higiene)", nuevo, nuevo, aceptarPublicacion},
		{"llega más nuevo", viejo, nuevo, aceptarPublicacion},
		{"otro commit en el mismo segundo", nuevo, PublicacionDelGrafo{Head: "otro", En: nuevo.En}, aceptarPublicacion},
	}
	for _, c := range casos {
		decision, motivo := decidirPublicacion(c.vigente, c.llega)
		if decision != c.decision {
			t.Errorf("%s: decisión=%v, esperaba %v (motivo %q)", c.nombre, decision, c.decision, motivo)
		}
		if decision != aceptarPublicacion && motivo == "" {
			t.Errorf("%s: un push no aplicado sin motivo es un federated:false mudo", c.nombre)
		}
	}
	// El remedio del push sin commit es otro y el mensaje tiene que decirlo: no es «indexá un árbol
	// al día» sino «actualizá ese binario».
	if _, motivo := decidirPublicacion(nuevo, PublicacionDelGrafo{}); !strings.Contains(motivo, "binario") {
		t.Errorf("el push sin commit ignorado tiene que decir que es un binario viejo, dijo %q", motivo)
	}
}

func nodosDe(t *testing.T, e *DbEngine, proyecto string) []GraphNode {
	t.Helper()
	n, err := e.AllGraphNodesCtx(WithProjectScope(context.Background(), ProjectScope{ProjectID: proyecto}))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Un grafo de un árbol más viejo NO pisa el publicado: error ErrGrafoMasViejo, y el grafo y la
// publicación quedan como estaban.
func TestReplaceProjectGraphPublicadoNoDejaRetroceder(t *testing.T) {
	e := newTestEngine(t)
	nuevo := PublicacionDelGrafo{Head: "fc3c297aaaa", En: fechaPub(t, "2026-09-23T17:00:00Z")}
	viejo := PublicacionDelGrafo{Head: "8de3b00bbbb", En: fechaPub(t, "2026-09-12T16:04:09Z")}
	alDia := []GraphNode{{Key: "internal/arbol/arbol.go#func:Nuevo", Kind: "func", Name: "Nuevo", Path: "internal/arbol/arbol.go"}}
	rancio := []GraphNode{{Key: "cmd/x.go#func:Viejo", Kind: "func", Name: "Viejo", Path: "cmd/x.go"}}

	if err := e.ReplaceProjectGraphPublicado("musubi", nuevo, alDia, nil); err != nil {
		t.Fatalf("publicar el al día: %v", err)
	}
	err := e.ReplaceProjectGraphPublicado("musubi", viejo, rancio, nil)
	if !errors.Is(err, ErrGrafoMasViejo) {
		t.Fatalf("un árbol más viejo tenía que rechazarse con ErrGrafoMasViejo, dio %v", err)
	}
	if n := nodosDe(t, e, "musubi"); len(n) != 1 || n[0].Name != "Nuevo" {
		t.Fatalf("el rechazo tocó el grafo publicado: %+v", n)
	}
	if pub, _ := e.PublicacionDelGrafoDe("musubi"); pub.Head != nuevo.Head || !pub.En.Equal(nuevo.En) {
		t.Fatalf("el rechazo cambió la publicación vigente: %+v", pub)
	}

	// Un binario viejo (sin commit) tampoco lo pisa: se ignora.
	if err := e.ReplaceProjectGraphPublicado("musubi", PublicacionDelGrafo{}, rancio, nil); !errors.Is(err, ErrGrafoIgnorado) {
		t.Fatalf("un push sin commit sobre uno publicado con commit tenía que ignorarse, dio %v", err)
	}
	if n := nodosDe(t, e, "musubi"); len(n) != 1 || n[0].Name != "Nuevo" {
		t.Fatalf("el push sin commit tocó el grafo publicado: %+v", n)
	}
}

// Uno más nuevo SÍ reemplaza, y queda registrado como el vigente.
func TestReplaceProjectGraphPublicadoAvanza(t *testing.T) {
	e := newTestEngine(t)
	viejo := PublicacionDelGrafo{Head: "8de3b00bbbb", En: fechaPub(t, "2026-09-12T16:04:09Z")}
	nuevo := PublicacionDelGrafo{Head: "fc3c297aaaa", En: fechaPub(t, "2026-09-23T17:00:00Z")}
	if err := e.ReplaceProjectGraphPublicado("musubi", viejo, []GraphNode{{Key: "a.go#func:A", Kind: "func", Name: "A", Path: "a.go"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.ReplaceProjectGraphPublicado("musubi", nuevo, []GraphNode{{Key: "b.go#func:B", Kind: "func", Name: "B", Path: "b.go"}}, nil); err != nil {
		t.Fatalf("uno más nuevo tenía que aceptarse: %v", err)
	}
	if n := nodosDe(t, e, "musubi"); len(n) != 1 || n[0].Name != "B" {
		t.Fatalf("el más nuevo no reemplazó: %+v", n)
	}
	if pub, _ := e.PublicacionDelGrafoDe("musubi"); pub.Head != nuevo.Head {
		t.Fatalf("la publicación vigente no avanzó: %+v", pub)
	}
}

// La publicación es POR PROYECTO: la base del central guarda el grafo de todos, y la de uno no
// puede frenar a otro.
func TestPublicacionDelGrafoEsPorProyecto(t *testing.T) {
	e := newTestEngine(t)
	nuevo := PublicacionDelGrafo{Head: "fc3c297aaaa", En: fechaPub(t, "2026-09-23T17:00:00Z")}
	viejo := PublicacionDelGrafo{Head: "1234567cccc", En: fechaPub(t, "2026-09-01T00:00:00Z")}
	if err := e.ReplaceProjectGraphPublicado("musubi", nuevo, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.ReplaceProjectGraphPublicado("altura", viejo, []GraphNode{{Key: "x.ts#func:X", Kind: "func", Name: "X", Path: "x.ts"}}, nil); err != nil {
		t.Fatalf("la publicación de musubi frenó a altura: %v", err)
	}
	if n := nodosDe(t, e, "altura"); len(n) != 1 {
		t.Fatalf("altura no quedó publicado: %+v", n)
	}
}

// Una fila de publicación ilegible cuenta como «nada publicado»: un proyecto no puede quedar
// trabado para siempre por un valor corrupto.
func TestPublicacionIlegibleNoTrabaElProyecto(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SetMeta(metaPublicacionDelGrafo("musubi"), "basura-sin-separador"); err != nil {
		t.Fatal(err)
	}
	if err := e.ReplaceProjectGraphPublicado("musubi", PublicacionDelGrafo{}, []GraphNode{{Key: "a.go#func:A", Kind: "func", Name: "A", Path: "a.go"}}, nil); err != nil {
		t.Fatalf("una publicación ilegible trabó el proyecto: %v", err)
	}
}
