package mcp

import (
	"testing"

	"musubi/internal/censo"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// censo_test.go ES LA GUARDA DEL CENSO DE TOOLS VIVAS (A3). Calca a TestEveryReadOnlyToolClassified
// (read_surface_class_test.go): aquélla sella por contrato la clase «lectura aislada por proyecto»;
// ésta sella la clase de INVOCACIÓN. La lección es la misma de tres auditorías del censo manual
// (se rehízo a mano al menos tres veces repitiendo los mismos errores medidos): en vez de perseguir
// de a una las tools que el catálogo suma, se exige POR CONTRATO que TODA tool del registro vivo
// esté censada, y que el censo no nombre ninguna que ya no exista.
//
// La fuente de verdad es el registro EN PROCESO (s.tools), el mismo que sirve el MCP — no un grep,
// que es el método de respaldo para cuando no se puede correr código. El test corre código, así que
// usa la fuente viva (s.tools) directamente; NO lee el grep (ése es sólo para una inspección a mano).

// TestCadaToolVivaEstaCensada falla en AMBAS direcciones: (E1) una tool del registro vivo sin
// entrada en el censo, o (E2) una entrada del censo que ya no existe en el registro. Así una tool
// nueva NO puede colarse sin que un humano decida su clase de invocación, y un rename/borrado que
// deje el censo rancio tampoco pasa en silencio.
func TestCadaToolVivaEstaCensada(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// (E1) Toda tool viva tiene que estar censada.
	vivas := make(map[string]bool, len(s.tools))
	for i := range s.tools {
		name := s.tools[i].Name
		vivas[name] = true
		if _, ok := censo.ClaseDe(name); !ok {
			t.Errorf("tool viva %q SIN censar: agregала a internal/censo con su clase de invocación "+
				"(administrativa / cubierta-por-ciclo / sin-productor / camino-cli / debe-invocarse). "+
				"Ante la duda, debe-invocarse: que el doctor la surface como hipótesis, no que se excuse en silencio", name)
		}
	}

	// (E2) El censo no nombra ninguna tool que ya no exista en el registro.
	for _, name := range censo.Tools() {
		if !vivas[name] {
			t.Errorf("el censo nombra %q, que ya no existe en el registro vivo: sacала de internal/censo "+
				"(un rename o un borrado dejó la entrada huérfana)", name)
		}
	}
}
