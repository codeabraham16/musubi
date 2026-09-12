package mcp

import (
	"context"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestElConstructorEstampaLaProcedenciaVengaLaLlamadaComoVenga — la red de PRIMER orden.
//
// POR QUÉ ESTA PRUEBA Y NO LA DE AST QUE YA HABÍA. `TestTodoServidorMcpEstampaLaProcedenciaDelVector`
// (cmd/musubi) es SINTÁCTICA: parsea el fuente y exige que toda función que construya un McpServer
// llame también a `cablearProcedenciaDelVector`. Una auditoría adversaria la pasó por arriba de tres
// formas, las tres compilando y las tres dejando el defecto puesto:
//
//	engine.SetVectorModelID("")            → contaba como cableado (mira el NOMBRE de la llamada)
//	if engine == nil { cablear(...) }      → contaba como cableado (ast.Inspect no mira alcanzabilidad)
//	nuevo := mcp.NewMcpServer; nuevo(...)  → el servidor se volvía INVISIBLE y seguía contando 3,
//	                                         o sea que su control positivo de «al menos 3» tampoco
//	                                         se enteraba de que había un cuarto sin cablear
//
// Enumerar formas de escribir una llamada no converge. Ahora el que estampa es el CONSTRUCTOR, así
// que el camino malo dejó de ser escribible, y esta prueba lo mide DONDE SE DECIDE: construye el
// servidor y pregunta si el motor quedó con la procedencia. No le importa cómo se escribió la
// llamada, porque no mira el fuente.
func TestElConstructorEstampaLaProcedenciaVengaLaLlamadaComoVenga(t *testing.T) {
	// EL CONTROL, PRIMERO: un embebedor Noop NO debe estampar nada. Sin esto, una implementación
	// que estampe SIEMPRE —incluso la cadena vacía del Noop— pasaría la prueba de abajo y estaría
	// escribiendo justo la procedencia que rompe el filtro de homogeneidad.
	espiaNoop := &motorQueAnotaLaProcedencia{}
	_ = NewMcpServer(espiaNoop, t.TempDir(), embedding.NoopProvider{})
	if espiaNoop.veces != 0 {
		t.Errorf("con un embebedor Noop se estampó %d vez/veces (%q) y no debe estamparse ninguna: "+
			"sin vectores no hay procedencia que declarar, y estampar la cadena vacía es exactamente "+
			"el defecto que el filtro de homogeneidad no perdona", espiaNoop.veces, espiaNoop.estampado)
	}

	// Y AHORA EL CASO. La llamada se escribe por el camino que la guarda de AST NO VE —el
	// constructor por variable— a propósito: es la demostración de que esta red no depende de la
	// forma de la llamada.
	espia := &motorQueAnotaLaProcedencia{}
	nuevo := NewMcpServer
	_ = nuevo(espia, t.TempDir(), proveedorDePrueba{})

	if espia.veces == 0 {
		t.Fatal("el constructor NO estampó la procedencia: el motor queda con `model_id` vacío, " +
			"`SearchObservations` filtra `AND e.model_id = ''`, y en una base poblada eso no matchea " +
			"NADA — el pool vectorial sale vacío SIN UN SOLO ERROR y el servidor contesta recall " +
			"léxico creyendo que hizo semántico")
	}
	if espia.estampado != (proveedorDePrueba{}).Name() {
		t.Errorf("estampó %q y el embebedor se llama %q: la procedencia tiene que ser la del "+
			"embebedor que ESTE servidor va a usar, o los vectores viejos se comparan por coseno "+
			"contra los nuevos como si fueran del mismo modelo",
			espia.estampado, (proveedorDePrueba{}).Name())
	}
}

// motorQueAnotaLaProcedencia embebe la interfaz para no tener que implementarla entera (el mismo
// truco que backend_seam_test.go) y sólo anota la llamada que interesa.
type motorQueAnotaLaProcedencia struct {
	memory.StorageBackend
	estampado string
	veces     int
}

func (m *motorQueAnotaLaProcedencia) SetVectorModelID(s string) { m.estampado = s; m.veces++ }

// proveedorDePrueba es un embebedor cualquiera que NO es Noop, que es lo único que decide si hay
// procedencia que estampar (embedding.Enabled).
type proveedorDePrueba struct{}

func (proveedorDePrueba) Name() string    { return "prueba:modelo@abc123" }
func (proveedorDePrueba) Dimensions() int { return 4 }
func (proveedorDePrueba) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0, 0, 0, 0}, nil
}
