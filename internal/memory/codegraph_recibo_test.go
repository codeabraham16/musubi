package memory

import (
	"errors"
	"strings"
	"testing"
)

// codegraph_recibo_test.go custodia la mitad de la base del RECIBO del mapa (punto 47A): quién
// publicó el grafo vigente, con qué huella, y cuánto quedó guardado de verdad.

// La publicación dice QUIÉN la empujó y con qué huella, en una clave APARTE: el valor de
// 'codegraph_publicado:<p>' sigue siendo exactamente "head|fecha", que es lo único que un binario
// anterior sabe leer si el central vuelve atrás. Y la firma no se le atribuye a quien no publicó:
// ni cuando un binario anterior reescribió la publicación sin tocarla, ni cuando el mismo commit lo
// re-empuja alguien sin credencial.
//
// Sabotaje que la pone roja: no guardar quién publicó.
// arnes: archivo="internal/memory/codegraph_publicacion.go"
// arnes: de="metaFirmaDeLaPublicacion(projectID), nueva.Head+\"|\"+nueva.Por+\"|\"+nueva.Huella,"
// arnes: a="metaFirmaDeLaPublicacion(projectID), nueva.Head+\"|\"+nueva.Por[:0]+\"|\"+nueva.Huella,"
//
// Sabotaje que la pone roja: sumarle el por al valor de la publicación, que rompe el rollback.
// arnes: archivo="internal/memory/codegraph_publicacion.go"
// arnes: de="metaPublicacionDelGrafo(projectID), nueva.Head+\"|\"+nueva.En.UTC().Format(time.RFC3339),"
// arnes: a="metaPublicacionDelGrafo(projectID), nueva.Head+\"|\"+nueva.En.UTC().Format(time.RFC3339)+\"|\"+nueva.Por,"
//
// Sabotaje que la pone roja: leer la firma sin mirar si es del head vigente.
// arnes: archivo="internal/memory/codegraph_publicacion.go"
// arnes: de="if !ok || head != headVigente {"
// arnes: a="if !ok || head != headVigente && false {"
//
// Sabotaje que la pone roja: escribir la firma sólo cuando hay credencial.
// arnes: archivo="internal/memory/codegraph_publicacion.go"
// arnes: de="metaFirmaDeLaPublicacion(projectID), nueva.Head"
// arnes: a="metaFirmaDeLaPublicacion(projectID+strings.Repeat(\"#\", 1-min(1, len(nueva.Por)))), nueva.Head"
// arnes: colision_ok="TestLaPublicacionRecuerdaQuienLaEmpujo"
//
// Sabotaje que la pone roja: que el rechazo no diga quién publicó lo vigente.
// arnes: archivo="internal/memory/codegraph_publicacion.go"
// arnes: de="\t\tquien = \", lo publicó \" + vigente.Por\n"
// arnes: a="\t\tquien = \"\"\n"
func TestLaPublicacionRecuerdaQuienLaEmpujo(t *testing.T) {
	e := newTestEngine(t)
	pc := PublicacionDelGrafo{Head: "fc3c297aaaa", En: fechaPub(t, "2026-09-23T17:00:00Z"), Por: "davantis-mando-admin", Huella: "h-pc"}
	laptop := PublicacionDelGrafo{Head: "8de3b00bbbb", En: fechaPub(t, "2026-09-12T16:04:09Z"), Por: "davantis-2", Huella: "h-laptop"}

	if err := e.ReplaceProjectGraphPublicado("musubi", pc, nil, nil); err != nil {
		t.Fatalf("publicar: %v", err)
	}
	crudo, _, _ := e.GetMeta(metaPublicacionDelGrafo("musubi"))
	if crudo != "fc3c297aaaa|2026-09-23T17:00:00Z" {
		t.Errorf("el valor de la publicación tiene que seguir siendo head|fecha (lo lee un binario anterior), quedó %q", crudo)
	}
	if pub, _ := e.PublicacionDelGrafoDe("musubi"); pub.Por != "davantis-mando-admin" || pub.Huella != "h-pc" {
		t.Fatalf("la publicación no recuerda quién la empujó ni su huella: %+v", pub)
	}

	// El rechazado se entera de QUIÉN tiene el árbol que ganó, y la firma no cambia.
	err := e.ReplaceProjectGraphPublicado("musubi", laptop, nil, nil)
	if !errors.Is(err, ErrGrafoMasViejo) || !strings.Contains(err.Error(), "lo publicó davantis-mando-admin") {
		t.Fatalf("el rechazo tiene que decir quién publicó lo vigente, dio %v", err)
	}
	if pub, _ := e.PublicacionDelGrafoDe("musubi"); pub.Por != "davantis-mando-admin" {
		t.Fatalf("un push rechazado cambió el publicador: %+v", pub)
	}

	// El mismo commit re-empujado sin credencial (loopback): la firma no le queda al anterior.
	sinPor := PublicacionDelGrafo{Head: pc.Head, En: pc.En}
	if err := e.ReplaceProjectGraphPublicado("musubi", sinPor, nil, nil); err != nil {
		t.Fatal(err)
	}
	if pub, _ := e.PublicacionDelGrafoDe("musubi"); pub.Por != "" || pub.Huella != "" {
		t.Errorf("un push sin credencial quedó atribuido al publicador anterior: %+v", pub)
	}

	// Un binario anterior reescribe la publicación y no toca la firma: la firma de otro head no vale.
	if err := e.ReplaceProjectGraphPublicado("musubi", pc, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SetMeta(metaPublicacionDelGrafo("musubi"), "0dd0000eeee|2026-09-24T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if pub, _ := e.PublicacionDelGrafoDe("musubi"); pub.Head != "0dd0000eeee" || pub.Por != "" || pub.Huella != "" {
		t.Errorf("la firma de otro commit se le atribuyó a la publicación vigente: %+v", pub)
	}
}

// El conteo es el de SU proyecto y el de lo que quedó guardado: una clave repetida cuenta una vez
// (el reemplazo la colapsa) y un gist sin contenido no cuenta (el reemplazo lo saltea). Es
// exactamente la diferencia entre el eco de lo recibido y el recibo.
//
// Sabotaje que la pone roja: contar los nodos de todos los proyectos.
// arnes: archivo="internal/memory/codegraph_publicacion.go"
// arnes: de="(SELECT COUNT(*) FROM code_graph_nodes WHERE project_id=?)"
// arnes: a="(SELECT COUNT(*) FROM code_graph_nodes WHERE (project_id=? OR 1=1))"
func TestElConteoDelGrafoEsElDeSuProyecto(t *testing.T) {
	e := newTestEngine(t)
	a := GraphNode{Key: "a.go#func:A", Kind: "func", Name: "A", Path: "a.go"}
	b := GraphNode{Key: "b.go#func:B", Kind: "func", Name: "B", Path: "b.go"}
	llama := GraphEdge{FromKey: a.Key, ToKey: b.Key, Kind: "CALLS"}
	if err := e.ReplaceProjectGraphFrom("musubi", []GraphNode{a, a, b}, []GraphEdge{llama, llama}); err != nil {
		t.Fatal(err)
	}
	x := GraphNode{Key: "x.ts#func:X", Kind: "func", Name: "X", Path: "x.ts"}
	if err := e.ReplaceProjectGraphFrom("altura", []GraphNode{x}, []GraphEdge{{FromKey: x.Key, ToKey: x.Key, Kind: "CALLS"}}); err != nil {
		t.Fatal(err)
	}
	if err := e.ReplaceProjectCodeMemoryFrom("musubi", []CodeMemory{{Path: "a.go", Gist: "el gist de a"}, {Path: "b.go", Gist: ""}}); err != nil {
		t.Fatal(err)
	}
	if err := e.ReplaceProjectCodeMemoryFrom("altura", []CodeMemory{{Path: "x.ts", Gist: "el gist de x"}}); err != nil {
		t.Fatal(err)
	}

	c, err := e.ConteoDelGrafoDe("musubi")
	if err != nil {
		t.Fatal(err)
	}
	if want := (ConteoDelGrafo{Nodes: 2, Edges: 1, Gists: 1}); c != want {
		t.Errorf("ConteoDelGrafoDe(musubi) = %+v, esperaba %+v (lo guardado de SU proyecto)", c, want)
	}
}

// La huella es del CONTENIDO: no cambia con el orden en que se leyó ni con claves repetidas, y sí
// cambia si falta un nodo o una arista. Si dependiera del orden, dos máquinas con el mismo árbol
// darían huellas distintas y el aviso de «mismo commit, otro contenido» sería ruido.
//
// Sabotaje que la pone roja: no ordenar las claves.
// arnes: archivo="internal/memory/codegraph_publicacion.go"
// arnes: de="\tsort.Strings(claves)\n"
// arnes: a=""
//
// Sabotaje que la pone roja: no descartar las repetidas.
// arnes: archivo="internal/memory/codegraph_publicacion.go"
// arnes: de="if i > 0 && s == lista[i-1] {"
// arnes: a="if false && i > 0 && s == lista[i-1] {"
func TestLaHuellaEsDelContenidoYNoDelOrden(t *testing.T) {
	a := GraphNode{Key: "a.go#func:A"}
	b := GraphNode{Key: "b.go#func:B"}
	llama := GraphEdge{FromKey: a.Key, ToKey: b.Key, Kind: "CALLS"}

	h := HuellaDelGrafo([]GraphNode{a, b}, []GraphEdge{llama})
	if otra := HuellaDelGrafo([]GraphNode{b, a, a}, []GraphEdge{llama, llama}); otra != h {
		t.Errorf("el mismo contenido en otro orden y con repetidas dio otra huella: %s contra %s", otra, h)
	}
	if otra := HuellaDelGrafo([]GraphNode{a}, []GraphEdge{llama}); otra == h {
		t.Error("sin un nodo, la huella dio igual")
	}
	if otra := HuellaDelGrafo([]GraphNode{a, b}, nil); otra == h {
		t.Error("sin la arista, la huella dio igual")
	}
	// Nodos y aristas no se mezclan: la misma cadena de un lado o del otro es otro contenido.
	if HuellaDelGrafo([]GraphNode{{Key: "k"}}, nil) == HuellaDelGrafo(nil, []GraphEdge{{FromKey: "k"}}) {
		t.Error("un nodo y una arista con el mismo texto dieron la misma huella")
	}
}
