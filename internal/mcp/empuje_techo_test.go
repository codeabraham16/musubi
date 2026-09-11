package mcp

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
)

// EL INTERVALO DEL EMPUJE NO PUEDE PASAR LA VENTANA DE OBSOLESCENCIA DE PROMETHEUS.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// Prometheus marca una serie como OBSOLETA a los 5 minutos de su última muestra. Toda la
// telemetría de flota llega por este empuje —`prometheus.yml` descarta esa familia del scrape a
// propósito, para que no haya dos productores del mismo dato— así que un intervalo por encima de
// ese umbral deja las series obsoletas la mayor parte del tiempo.
//
// Y EL MODO DE FALLA ES EL PEOR QUE HAY: las reglas de flota no fallan, ENMUDECEN. Un
// `musubi_fleet_device_up == 0` sobre una serie obsoleta no matchea nada, así que `MaquinaCaida`
// y sus treinta hermanas quedan imposibles de disparar — con la configuración del cerebro
// perfecta, sin un error en ningún log, y el tablero entero en verde. Es exactamente la clase de
// falla que este plano existe para no tener.
//
// Se falla al ARRANCAR y no se recorta en silencio: recortar dejaría corriendo una configuración
// distinta de la que alguien escribió.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestElIntervaloDelEmpujeTieneTecho(t *testing.T) {
	base := func(seg float64) config.OTLPPushConfig {
		return config.OTLPPushConfig{
			Endpoint:        "http://127.0.0.1:9099/api/v1/otlp/v1/metrics",
			Principal:       "empujador",
			IntervalSeconds: seg,
			TimeoutSeconds:  5,
		}
	}

	t.Run("un intervalo sano arranca", func(t *testing.T) {
		s := newTestServer(t, embedding.NoopProvider{})
		if err := s.configurarEmpujeOTLP(base(30)); err != nil {
			t.Fatalf("30 s es la cadencia del scrape y tiene que ser válida: %v", err)
		}
	})

	// EL CASO QUE JUSTIFICA EL TECHO.
	t.Run("un intervalo por encima del techo NO arranca", func(t *testing.T) {
		s := newTestServer(t, embedding.NoopProvider{})
		err := s.configurarEmpujeOTLP(base(600))
		if err == nil {
			t.Fatal("un intervalo de 10 minutos se aceptó: con la serie obsoleta la mayor parte del " +
				"tiempo, las reglas de flota quedan imposibles de disparar y NADA lo dice")
		}
		// El mensaje tiene que decir la CONSECUENCIA y no sólo el número: un error que dice
		// «valor inválido» manda a subir el número hasta que entre, que es lo contrario.
		for _, palabra := range []string{"obsoleta", "ENMUDECEN"} {
			if !strings.Contains(err.Error(), palabra) {
				t.Errorf("el error no explica por qué el techo existe (falta %q): %v", palabra, err)
			}
		}
	})

	// JUSTO EN EL BORDE. Sin este caso, la guarda la satisface un techo puesto en cualquier lado.
	t.Run("el techo es la frontera y no una zona gris", func(t *testing.T) {
		s := newTestServer(t, embedding.NoopProvider{})
		if err := s.configurarEmpujeOTLP(base(TechoDelEmpujeOTLP.Seconds())); err != nil {
			t.Errorf("el valor del techo exacto tiene que ser válido: %v", err)
		}
		s2 := newTestServer(t, embedding.NoopProvider{})
		if err := s2.configurarEmpujeOTLP(base(TechoDelEmpujeOTLP.Seconds() + 1)); err == nil {
			t.Error("un segundo por encima del techo se aceptó")
		}
	})

	// EL TECHO TIENE QUE CABER EN LA VENTANA DE PROMETHEUS, y eso NO es una tautología: si alguien
	// subiera la constante a 10 minutos, esta prueba seguiría pasando en todo lo de arriba —los
	// casos se derivan del techo— y el defecto volvería entero. La ventana de Prometheus es un
	// hecho de afuera y se escribe como número.
	const ventanaDeObsolescenciaDePrometheus = 5 * time.Minute
	if TechoDelEmpujeOTLP >= ventanaDeObsolescenciaDePrometheus {
		t.Errorf("el techo del empuje (%s) llega o pasa la ventana de obsolescencia de Prometheus "+
			"(%s): con esa cadencia las series de flota están obsoletas parte del tiempo y las "+
			"reglas que las miran no disparan",
			TechoDelEmpujeOTLP, ventanaDeObsolescenciaDePrometheus)
	}

	// Y EL APAGADO EXPLÍCITO SIGUE SIENDO VÁLIDO: un intervalo negativo es «no exportar», no un
	// intervalo larguísimo. Sin este caso, la guarda la satisface un techo que también prohíba
	// apagar el empuje.
	t.Run("el apagado explícito no choca con el techo", func(t *testing.T) {
		s := newTestServer(t, embedding.NoopProvider{})
		cfg := base(-1)
		if err := s.configurarEmpujeOTLP(cfg); err != nil {
			t.Errorf("un interval_seconds negativo es el apagado explícito y tiene que seguir siendo "+
				"válido: %v", err)
		}
	})
}
