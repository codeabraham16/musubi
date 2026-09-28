package mcp

import (
	"testing"

	"musubi/internal/config"
)

// EL LEASE DE LA BAJADA ES EL QUE SUPONE EL LATIDO DEL PANEL.
//
// El latido del riel (cmd/musubi/assets/src/latido.mjs) queda prendido mientras el último sondeo
// tenga 10 min o menos, y su guarda en cmd/musubi exige que el peor hueco SANO entre dos sondeos
// —tope + lease + tick— entre en esos 10 min. Pero el lease lo COPIA (cuatro ticks, piso de 120 s),
// porque leaseBajadaSegundos no se exporta: si el lease crece acá y la copia no, el panel dice «sin
// sondeo» con el sistema sano y su guarda sigue en verde. Medido en la revisión: con un lease de
// diez ticks el peor hueco pasa a 300 + 300 + 30 = 630 s y TestElLatidoAguantaLaBajadaEspaciadaYSeApagaConUnCorte
// queda verde. Si esto se pone rojo, actualizá peorHuecoSanoDeLaBajada y el umbral del panel juntos.
//
// Sabotaje: un lease de diez ticks.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="seg := 4 * s.syncCfg.DrainIntervalSeconds"
// arnes: a="seg := 10 * s.syncCfg.DrainIntervalSeconds"
func TestElLeaseDeLaBajadaEsElQueSuponeElLatidoDelPanel(t *testing.T) {
	for _, tick := range []int{10, 30, 60} {
		cfg := config.Default().Sync
		cfg.DrainIntervalSeconds = tick
		s := &McpServer{syncCfg: cfg}
		copia := 4 * tick // la fórmula que copia peorHuecoSanoDeLaBajada: cuatro ticks, piso de 120 s
		if copia < 120 {
			copia = 120
		}
		if got := s.leaseBajadaSegundos(); got != copia {
			t.Errorf("con un tick de %d s el lease de la bajada es %d s y el latido del panel supone %d s: el peor hueco "+
				"sano que custodia cmd/musubi/panel_latido_node_test.go ya no es el real", tick, got, copia)
		}
	}
}
