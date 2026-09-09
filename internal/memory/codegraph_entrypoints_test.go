package memory

import (
	"context"
	"fmt"
	"testing"
)

// nodoFunc arma un nodo func mínimo con la clave canónica.
func nodoFunc(path, name string) GraphNode {
	return GraphNode{Key: path + "#func:" + name, Kind: "func", Name: name, Path: path}
}

// TestEntryPointsOrdenaPorGradoYNoPorAbecedario — LA REGRESIÓN QUE ESTE ORDEN EXISTE PARA CAZAR.
//
// Antes se recortaba a `limit` sobre la lista ordenada por node_key, así que entry_points devolvía
// una rebanada ALFABÉTICA. En el repo real eso daba 25 de 3.851 (el 57,4% de los funcs/métodos no
// tiene llamador porque el grafo no captura la mayoría de las llamadas), y las claves con ruta
// absoluta —'C' < 'c'— se comían los 25 lugares con tests de OTRO repo.
//
// El escenario está armado para que el abecedario y el grado den respuestas OPUESTAS: `zeta` es el
// último alfabéticamente y el que más llama; `alfa` es el primero y no llama a nadie. Con limit=1,
// un orden alfabético devuelve alfa y el correcto devuelve zeta.
func TestEntryPointsOrdenaPorGradoYNoPorAbecedario(t *testing.T) {
	e, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ctx := context.Background()

	nodos := []GraphNode{nodoFunc("a.go", "alfa"), nodoFunc("z.go", "zeta")}
	var aristas []GraphEdge
	// zeta llama a tres hojas; alfa no llama a nadie. Ninguna de las hojas llama a alfa ni a zeta,
	// así que los dos siguen siendo entry points.
	for i := 0; i < 3; i++ {
		hoja := fmt.Sprintf("hoja%d", i)
		nodos = append(nodos, nodoFunc("h.go", hoja))
		aristas = append(aristas, GraphEdge{
			FromKey: "z.go#func:zeta", ToKey: "h.go#func:" + hoja, Kind: "CALLS", SrcPath: "z.go",
		})
	}
	if err := e.UpsertPackageGraph([]string{"a.go", "z.go", "h.go"}, nodos, aristas); err != nil {
		t.Fatal(err)
	}

	ep, total, err := e.GraphEntryPointsCtx(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(ep) != 1 || ep[0] != "z.go#func:zeta" {
		t.Errorf("con limit=1 esperaba zeta (grado 3, último del abecedario) y obtuve %v — "+
			"si devolvió alfa, el recorte volvió a hacerse sobre el orden por clave", ep)
	}
	// LA OTRA MITAD: el recorte tiene que DECLARARSE. alfa y zeta son los dos entry points.
	if total != 2 {
		t.Errorf("total = %d, esperaba 2 (alfa y zeta): un recorte que no dice cuánto dejó afuera "+
			"se lee como una lista completa", total)
	}
}

// TestEntryPointsEsDeterministaConGradoEmpatado — sin desempate, dos nodos de igual grado se
// alternarían entre corridas y el informe parecería cambiar solo sin que cambie el código.
func TestEntryPointsEsDeterministaConGradoEmpatado(t *testing.T) {
	e, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ctx := context.Background()

	// Cuatro entry points, todos con grado 0: sólo puede ordenarlos el desempate por clave.
	var nodos []GraphNode
	for _, n := range []string{"delta", "beta", "gamma", "alfa"} {
		nodos = append(nodos, nodoFunc("x.go", n))
	}
	if err := e.UpsertPackageGraph([]string{"x.go"}, nodos, nil); err != nil {
		t.Fatal(err)
	}

	quiero := []string{"x.go#func:alfa", "x.go#func:beta"}
	for corrida := 0; corrida < 4; corrida++ {
		ep, total, err := e.GraphEntryPointsCtx(ctx, 2)
		if err != nil {
			t.Fatal(err)
		}
		if total != 4 {
			t.Fatalf("corrida %d: total = %d, esperaba 4", corrida, total)
		}
		for i := range quiero {
			if ep[i] != quiero[i] {
				t.Fatalf("corrida %d: ep = %v, esperaba %v — el empate de grado no está desempatando "+
					"por clave y el resultado no es estable", corrida, ep, quiero)
			}
		}
	}
}
