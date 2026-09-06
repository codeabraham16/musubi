package mcp

import (
	"context"
	"strings"
	"testing"

	"musubi/internal/codeintel"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// Tocar un README no puede volver inalcanzable el panel más barato.
//
// ESTE TEST MIDE EL CABLEADO, NO EL HELPER. Que PuedeTenerRadio clasifique bien las extensiones
// se prueba en codeintel; acá lo que se verifica es que profundidadDe la use. La distinción no
// es formal: en este mismo track hubo un test que congelaba la tabla de escalones llamando a
// `escalon()` con números escritos a mano, y mover una constante calibrada lo dejaba verde —
// medía el proxy y no la cosa. Se descubrió saboteándolo.
func servidorConGrafoVacio(t *testing.T) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")
	return NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
}

func cegado(v codeintel.Veredicto) bool {
	return strings.Contains(strings.Join(v.Motivos, " "), "no se pudo medir")
}

func TestUnCambioSoloDeDocsNoDejaElRadioCiego(t *testing.T) {
	s := servidorConGrafoVacio(t)
	diffs := []codeintel.FileDiff{
		{Path: "CHANGELOG.md", ChangeType: codeintel.ChangeModified, Agregadas: 4},
		{Path: ".github/workflows/ci.yml", ChangeType: codeintel.ChangeModified, Agregadas: 2},
	}
	v := s.profundidadDe(context.Background(), diffs, 0, nil)

	if cegado(v) {
		t.Errorf("un CHANGELOG no es código cuyo radio quedó sin medir: es un archivo sin radio.\nmotivos=%v", v.Motivos)
	}
	// Y la consecuencia concreta: el panel más barato tiene que seguir siendo alcanzable.
	if v.Nivel != codeintel.NivelMinimo {
		t.Errorf("seis líneas de documentación deben dar «mínima», obtuve %q (puntos=%d)", v.Nivel, v.Puntos)
	}
}

// El contrapeso, y es la mitad que hace que el test anterior signifique algo: cuando SÍ es
// código que el grafo no cubre, la ceguera se declara. Sin esta mitad, apagar el piso de
// honestidad entero pasaría los dos tests.
func TestCodigoSinCoberturaSiDejaElRadioCiego(t *testing.T) {
	s := servidorConGrafoVacio(t)
	diffs := []codeintel.FileDiff{
		{Path: "internal/algo/motor.go", ChangeType: codeintel.ChangeModified, Agregadas: 4},
	}
	v := s.profundidadDe(context.Background(), diffs, 0, nil)

	if !cegado(v) {
		t.Errorf("un .go que el grafo no tiene indexado SÍ deja el radio ciego; motivos=%v", v.Motivos)
	}
	if v.Nivel == codeintel.NivelMinimo {
		t.Error("con el radio sin medir, «mínima» afirmaría lo que no se sabe")
	}
}

// Y la mezcla, que es el caso real: un PR que toca código Y su CHANGELOG. El .md no debe
// aportar ceguera, pero el .go sin cobertura sí.
func TestElDocNoAportaCegueraCuandoElCodigoSiLaAporta(t *testing.T) {
	s := servidorConGrafoVacio(t)
	soloCodigo := []codeintel.FileDiff{{Path: "internal/algo/motor.go", ChangeType: codeintel.ChangeModified, Agregadas: 4}}
	conDoc := append(append([]codeintel.FileDiff{}, soloCodigo...),
		codeintel.FileDiff{Path: "CHANGELOG.md", ChangeType: codeintel.ChangeModified, Agregadas: 2})

	a := s.profundidadDe(context.Background(), soloCodigo, 0, nil)
	b := s.profundidadDe(context.Background(), conDoc, 0, nil)

	motivoA := strings.Join(a.Motivos, " ")
	motivoB := strings.Join(b.Motivos, " ")
	if strings.Contains(motivoB, "no indexa") && !strings.Contains(motivoA, "no indexa") {
		t.Errorf("el CHANGELOG agregó un motivo de ceguera que no existía:\nsin doc: %s\ncon doc: %s", motivoA, motivoB)
	}
}
