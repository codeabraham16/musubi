package memory

import (
	"context"
	"testing"
	"time"
)

// codegraph_generacion_test.go custodia la generación durable del grafo: que TODO escritor de la
// foto la suba, que la foto la lea en su instantánea y que la empujada nunca retroceda.

// CADA ESCRITOR DE LA FOTO SUBE LA GENERACIÓN.
//
// La generación sube adentro de las funciones que escriben, no en los llamadores, justamente para
// que un escritor no pueda olvidarse. Esta tabla es la lista: si mañana aparece un escritor nuevo de
// nodos, aristas o gists, va acá. Sabotaje que la pone roja: sacar avanzarGeneracionDelGrafo de
// cualquiera de los cinco (medido con PruneGraphFilesFrom).
func TestCadaEscritorDeLaFotoSubeLaGeneracion(t *testing.T) {
	e, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	nodo := []GraphNode{{Key: "a.go#func:A", Kind: "func", Name: "A", Path: "a.go", SrcFingerprint: "x"}}
	arista := []GraphEdge{{FromKey: "a.go#func:A", ToKey: "a.go#func:A", Kind: "calls", SrcPath: "a.go", SrcFingerprint: "x"}}
	escritores := []struct {
		nombre   string
		escribir func() error
	}{
		{"UpsertPackageGraphFrom", func() error { return e.UpsertPackageGraphFrom("", []string{"a.go"}, nodo, arista) }},
		{"PruneGraphFilesFrom", func() error { _, err := e.PruneGraphFilesFrom("", []string{"a.go"}); return err }},
		{"ReplaceProjectGraphFrom", func() error { return e.ReplaceProjectGraphFrom("", nodo, arista) }},
		{"SaveCodeMemoryFrom", func() error { return e.SaveCodeMemoryFrom("", CodeMemory{Path: "a.go", Gist: "g"}) }},
		{"ReplaceProjectCodeMemoryFrom", func() error {
			return e.ReplaceProjectCodeMemoryFrom("", []CodeMemory{{Path: "a.go", Gist: "g2"}})
		}},
	}
	previa := int64(0)
	for _, w := range escritores {
		if err := w.escribir(); err != nil {
			t.Fatalf("%s: %v", w.nombre, err)
		}
		est, err := e.EstadoDelPushDelGrafo()
		if err != nil {
			t.Fatal(err)
		}
		if est.Generacion <= previa {
			t.Errorf("%s escribió la foto y la generación no subió (%d → %d): su cambio no llegaría nunca al central", w.nombre, previa, est.Generacion)
		}
		previa = est.Generacion
	}
}

// LA FOTO TRAE LA GENERACIÓN DE SU PROPIA INSTANTÁNEA, y la empujada NUNCA RETROCEDE.
//
// Un escritor que cae en medio de la foto sube la generación en la base, pero la foto tiene que
// seguir diciendo la vieja: es la que se marca como empujada. Y dos push que terminan en desorden no
// pueden bajar la marca.
func TestLaFotoTraeSuGeneracionYLaEmpujadaNoRetrocede(t *testing.T) {
	e, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	if est, err := e.EstadoDelPushDelGrafo(); err != nil || est.Generacion != 0 || est.Empujada != 0 || !est.EmpujadoEn.IsZero() {
		t.Fatalf("una base sin las claves tiene que leer todo en cero: %+v err=%v", est, err)
	}
	nodo := []GraphNode{{Key: "a.go#func:A", Kind: "func", Name: "A", Path: "a.go"}}
	if err := e.ReplaceProjectGraphFrom("", nodo, nil); err != nil {
		t.Fatal(err)
	}
	foto, err := e.fotoDelGrafo(context.Background(), func() {
		if werr := e.ReplaceProjectGraphFrom("", nodo, nil); werr != nil {
			t.Errorf("escritor concurrente: %v", werr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if foto.Generacion != 1 {
		t.Fatalf("la foto se leyó con la generación 1 y dice %d: se leyó fuera de su instantánea", foto.Generacion)
	}

	ahora := time.Now()
	if err := e.MarcarGrafoEmpujado(2, ahora); err != nil {
		t.Fatal(err)
	}
	if err := e.MarcarGrafoEmpujado(foto.Generacion, ahora); err != nil { // un push más viejo que termina después
		t.Fatal(err)
	}
	est, err := e.EstadoDelPushDelGrafo()
	if err != nil {
		t.Fatal(err)
	}
	if est.Empujada != 2 {
		t.Errorf("un push con la generación 1 terminó después de uno con la 2 y bajó la marca a %d", est.Empujada)
	}
	if est.EmpujadoEn.Unix() != ahora.Unix() {
		t.Errorf("el instante del push exitoso no quedó registrado: %v", est.EmpujadoEn)
	}
}
