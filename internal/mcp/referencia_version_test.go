package mcp

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// EL EJE DE «AGENTE ATRASADO» SE PUEDE APAGAR PARA LA FLOTA ENTERA, Y AHORA SE VE.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `musubi_fleet_device_agent_stale` se OMITE cuando la comparación no se puede hacer, y eso es
// correcto: marcar a toda la flota como atrasada porque el build propio no selló su versión sería
// culparla de un problema ajeno.
//
// Pero la omisión era MUDA. Con la versión del cerebro sin parsear —vacía, o de cuatro
// componentes— la serie desaparece de TODAS las máquinas y `AgenteDesactualizado` queda imposible
// de disparar: sin un solo error, sin un log, y con el tablero entero en verde.
//
// Es el incidente del 2026-09-09. Se arregló la CAUSA que se conoció —un argumento omitido al
// construir— y no la FORMA de falla, que sigue viva: cualquier versión que el parser no entienda
// vuelve a apagar el eje igual.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestElCerebroDeclaraSiPuedeCompararVersiones(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc")
	latirConVersion(t, ts.URL, tok, "0.140.0-flota.abc1234")

	dump := func(version string) string {
		var b strings.Builder
		renderFlota(&b, s.engine, nil, time.Now(), s.sondaIntervalo, version, nil, serviciosPorProyectoDefault)
		return b.String()
	}

	t.Run("con una versión sellada, la referencia sirve", func(t *testing.T) {
		out := dump("0.140.3")
		if !strings.Contains(out, nombreReferenciaVersion+" 1") {
			t.Errorf("con el cerebro en 0.140.3 la referencia tiene que declararse usable:\n%s", out)
		}
	})

	// EL CASO DEL INCIDENTE, y los dos que el parser no entiende.
	for _, mala := range []struct{ nombre, version string }{
		{"sin ldflags", "dev"},
		{"vacía", ""},
		{"cuatro componentes", "1.2.3.4"},
	} {
		t.Run("con una versión "+mala.nombre+", la referencia se declara inservible", func(t *testing.T) {
			out := dump(mala.version)
			if !strings.Contains(out, nombreReferenciaVersion+" 0") {
				t.Errorf("con el cerebro en %q la referencia tiene que declararse INSERVIBLE: "+
					"`agent_stale` se omite para la flota entera y nadie puede decir por qué\n%s",
					mala.version, out)
			}
			// Y LA MITAD QUE JUSTIFICA TODO: el eje efectivamente se apaga.
			for _, l := range strings.Split(out, "\n") {
				if strings.HasPrefix(l, "musubi_fleet_device_agent_stale{") {
					t.Errorf("con el cerebro en %q se exporta agent_stale igual: %s", mala.version, l)
				}
			}
		})
	}

	// LA SERIE EXISTE SIEMPRE, incluso cuando vale 1. Una serie que sólo aparece cuando hay
	// problema no se distingue de «el exportador no corrió», que es el mismo silencio con otra
	// forma.
	t.Run("la serie no desaparece cuando todo está bien", func(t *testing.T) {
		// SE BUSCA LA LÍNEA DE LA SERIE Y NO EL NOMBRE SUELTO. La primera versión hacía
		// `strings.Contains(dump, nombreReferenciaVersion)` y la satisfacía el `# HELP`, que
		// nombra la serie SIEMPRE — o sea que el sabotaje «emitir el valor sólo cuando vale 0»
		// salía en VERDE. Guarda hueca propia, cazada por su propio sabotaje.
		hayLinea := false
		for _, l := range strings.Split(dump("0.140.3"), "\n") {
			if strings.HasPrefix(l, nombreReferenciaVersion+" ") {
				hayLinea = true
				break
			}
		}
		if !hayLinea {
			t.Error("la serie sólo aparece cuando hay problema: así `absent()` no distingue «todo " +
				"bien» de «el exportador no corrió», que es el mismo silencio con otra forma")
		}
	})
}

// Y LA ALERTA QUE LA LEE. Sin lector, la serie es una medición que nadie mira — el defecto que
// este plan ya cerró tres veces.
func TestLaAlertaDeLaReferenciaDeVersionExiste(t *testing.T) {
	reglas := strings.Join(strings.Fields(leerDeploy(t, "musubi-alerts-flota.yml")), " ")
	if !strings.Contains(reglas, nombreReferenciaVersion+" == 0") {
		t.Errorf("ninguna alerta lee `%s == 0`: la serie diría que el eje de «agente atrasado» está "+
			"apagado para la flota entera, y nadie estaría escuchando", nombreReferenciaVersion)
	}
}
