package mcp

import (
	"strings"
	"testing"
)

// «NO LLEGA TELEMETRÍA DE FLOTA» TIENE DOS FORMAS, Y HABÍA UNA SOLA ALERTA.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// La telemetría de flota viaja por DOS transportes, y es una decisión escrita: `prometheus.yml`
// descarta `musubi_fleet_(device|service)_.*` del scrape para que no haya dos productores del
// mismo dato, así que esa familia llega SÓLO por el empuje OTLP. Las otras cinco series de flota
// —`export_truncated`, `approval_pending`, `approval_wait_seconds`, `net_up` y
// `policy_actions_total`— viajan SÓLO por el scrape.
//
// `FlotaSinTelemetria` es `absent(musubi_fleet_device_up)`: mira el empuje. Y su propia nota
// mandaba a «revisar la sección `fleet:` del principal del scrape» — que es EXACTAMENTE la causa
// que no puede detectar. Con esa concesión sacada, `/metrics` deja de traer las series de flota,
// el target sigue en `up == 1` (así que `MusubiDown` tampoco dice nada), y `device_up` sigue
// llegando por el empuje. Todo verde y la mitad del plano de flota apagada.
// ────────────────────────────────────────────────────────────────────────────────────────────

func TestLasDosMitadesDeLaTelemetriaDeFlotaTienenQuienLasVigile(t *testing.T) {
	reglas := leerDeploy(t, "musubi-alerts-flota.yml")

	// Una serie testigo por transporte. La del scrape NO puede ser de la familia
	// `device|service`: ésa se descarta del scrape a propósito, así que su ausencia no diría nada
	// sobre el scrape.
	transportes := []struct {
		testigo   string
		transp    string
		porQue    string
		esDeFlota bool
	}{
		{"musubi_fleet_device_up", "el empuje OTLP",
			"es de la familia que prometheus.yml descarta del scrape: sólo puede llegar empujada", true},
		{"musubi_fleet_export_truncated", "el scrape de /metrics",
			"el exportador la emite SIEMPRE (0 o 1) y no tiene copia por OTLP, así que su ausencia " +
				"sólo puede ser del scrape", true},
	}

	for _, tr := range transportes {
		aguja := "absent(" + tr.testigo + ")"
		if !strings.Contains(strings.Join(strings.Fields(reglas), " "), aguja) {
			t.Errorf("no hay ninguna alerta `%s`, así que nadie vigila %s.\n"+
				"  %s\n"+
				"  Sin ella, esa mitad de la telemetría de flota puede quedarse muda con TODO EN "+
				"VERDE: las reglas que dependen de sus series no fallan, enmudecen.", aguja, tr.transp, tr.porQue)
		}
	}

	// Y LAS DOS TIENEN QUE SER ALERTAS DISTINTAS. Una sola regla con un `or` entre las dos
	// ausencias avisaría igual, pero mandaría al lugar equivocado la mitad de las veces: el
	// empuje y el scrape se arreglan en sitios distintos, y un aviso que nombra el sitio
	// equivocado es peor que no avisar — quien lo lee toca una perilla, no ve ningún cambio, y
	// concluye que la alerta miente.
	nombres := 0
	for _, bloque := range strings.Split(reglas, "- alert:")[1:] {
		n := strings.Fields(bloque)[0]
		if strings.HasPrefix(n, "FlotaSinTelemetria") {
			nombres++
		}
	}
	if nombres < 2 {
		t.Errorf("hay %d alerta(s) `FlotaSinTelemetria*` y tienen que ser al menos 2, una por "+
			"transporte: el empuje y el scrape se arreglan en lugares distintos", nombres)
	}
}
