package memory

import (
	"context"
	"testing"
)

// LA FOTO DEL PUSH ES DE UN SOLO INSTANTE.
//
// El push al central es de REEMPLAZO, así que una foto mezclada no se corrige sola: el central se
// queda con nodos de una generación y aristas de otra hasta el próximo push. Esta prueba mete un
// escritor EXACTAMENTE entre la lectura de los nodos y la de las aristas —la ventana que la
// transacción de lectura cierra— y exige que las tres listas sigan siendo de la generación vieja.
//
// Sin la transacción (leer las aristas contra la base suelta), las aristas ya salen de la
// generación nueva y apuntan a un nodo que la foto no trae.
func TestLaFotoDelGrafoEsDeUnSoloInstante(t *testing.T) {
	e, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	generacion := func(nombre, path string) ([]GraphNode, []GraphEdge) {
		key := path + "#func:" + nombre
		return []GraphNode{{Key: key, Kind: "func", Name: nombre, Path: path, SrcFingerprint: nombre}},
			[]GraphEdge{{FromKey: key, ToKey: key, Kind: "calls", SrcPath: path, SrcFingerprint: nombre}}
	}
	n1, e1 := generacion("Uno", "uno.go")
	if err := e.ReplaceProjectGraphFrom("", n1, e1); err != nil {
		t.Fatal(err)
	}
	if err := e.ReplaceProjectCodeMemoryFrom("", []CodeMemory{{Path: "uno.go", Gist: "gist uno", Fingerprint: "Uno"}}); err != nil {
		t.Fatal(err)
	}

	escribio := false
	foto, err := e.fotoDelGrafo(context.Background(), func() {
		n2, e2 := generacion("Dos", "dos.go")
		if werr := e.ReplaceProjectGraphFrom("", n2, e2); werr != nil {
			t.Errorf("el escritor concurrente no pudo escribir (la lectura no debe frenarlo): %v", werr)
			return
		}
		if werr := e.ReplaceProjectCodeMemoryFrom("", []CodeMemory{{Path: "dos.go", Gist: "gist dos", Fingerprint: "Dos"}}); werr != nil {
			t.Errorf("el escritor concurrente no pudo escribir los gists: %v", werr)
			return
		}
		escribio = true
	})
	if err != nil {
		t.Fatalf("FotoDelGrafo: %v", err)
	}
	if !escribio {
		t.Fatal("la costura no corrió el escritor: la prueba no ejercitó la ventana")
	}

	if len(foto.Nodes) != 1 || foto.Nodes[0].Name != "Uno" {
		t.Fatalf("los nodos tenían que ser de la generación vieja, obtuve %+v", foto.Nodes)
	}
	if len(foto.Edges) != 1 || foto.Edges[0].FromKey != foto.Nodes[0].Key {
		t.Errorf("las aristas salieron de OTRA generación que los nodos (foto mezclada): nodos %+v, aristas %+v", foto.Nodes, foto.Edges)
	}
	if len(foto.Gists) != 1 || foto.Gists[0].Path != "uno.go" {
		t.Errorf("los gists salieron de OTRA generación que los nodos (foto mezclada): %+v", foto.Gists)
	}

	// Y la escritura SÍ quedó: la foto vieja no es porque el escritor falló, es porque la lectura
	// estaba aislada de él.
	despues, err := e.FotoDelGrafoCtx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(despues.Nodes) != 1 || despues.Nodes[0].Name != "Dos" {
		t.Errorf("después de la foto, la base tenía que tener la generación nueva, obtuve %+v", despues.Nodes)
	}
}
