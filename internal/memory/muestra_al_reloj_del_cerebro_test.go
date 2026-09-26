package memory

import (
	"testing"
	"time"

	"musubi/internal/fleet"
)

// A131 · T2 (pulido) — UNA MUESTRA FECHADA DESPUÉS DE LLEGAR SE GUARDA CON LA HORA EN QUE LLEGÓ.
//
// Es la decisión del usuario sobre la frescura de la muestra: la mide el reloj del CEREBRO, no el
// del agente. `tomada` la pone el agente y puede ir adelantada; creerle una fecha del futuro
// rejuvenece la muestra para las políticas, para `antiguedad_s` y para la alerta de muestra rancia,
// que miden todas `ahora − tomada`. El recorte vive en latirDeviceCon porque es el único lugar donde
// la muestra y su hora de llegada están juntas, así que esta guarda entra por las DOS puertas que lo
// usan: el latido en autocommit (la sonda de un Tier B) y el de la transacción (el agente).
//
// Lo que NO se toca también se mide: una `tomada` anterior a la llegada queda como vino, al
// nanosegundo, y el resto de la muestra sobrevive al recorte. La guarda de comportamiento —que una
// política no actúe sobre la muestra vieja fechada adelante— es
// TestUnaPoliticaDeHostActuaSoloConLatidoYMuestraDentroDelUmbralDeSuTier (internal/mcp).
//
// Sabotaje: creerle al agente los 2 s de adelanto que se midieron en la flota. Una muestra fechada
// 1 s después de llegar queda con su fecha. El arreglo escribe la misma comparación con Compare.
// arnes: archivo="internal/fleet/muestra.go"
// arnes: de="\tif !m.Tomada.After(llegada) {\n\t\treturn m, false\n\t}\n"
// arnes: a="\tif !m.Tomada.After(llegada.Add(2 * time.Second)) {\n\t\treturn m, false\n\t}\n"
// arnes: arreglo_de="\tif !m.Tomada.After(llegada) {\n\t\treturn m, false\n\t}\n"
// arnes: arreglo_a="\tif m.Tomada.Compare(llegada) <= 0 {\n\t\treturn m, false\n\t}\n"
func TestUnaMuestraFechadaDespuesDeLlegarSeGuardaConLaHoraDeLlegada(t *testing.T) {
	e := newTestEngine(t)
	llegada := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	puertas := []struct {
		nombre string
		latir  func(id, muestra string) error
	}{
		{"autocommit", func(id, muestra string) error {
			_, err := e.LatirDevice(id, llegada, muestra)
			return err
		}},
		{"transaccion", func(id, muestra string) error {
			_, _, err := e.LatirYTomarComandos(id, llegada, muestra, 10)
			return err
		}},
	}
	casos := []struct {
		nombre string
		tomada time.Time
		quedar time.Time
	}{
		{"adelantada-1s", llegada.Add(time.Second), llegada},
		{"adelantada-1h", llegada.Add(time.Hour), llegada},
		// Una fecha anterior a la llegada queda como vino, con sus nanosegundos: más vieja es el lado
		// prudente, y reescribirla sería perder el dato.
		{"atrasada-3s", llegada.Add(-3*time.Second - 250*time.Millisecond), llegada.Add(-3*time.Second - 250*time.Millisecond)},
		{"justo-al-llegar", llegada, llegada},
	}
	for _, p := range puertas {
		for _, c := range casos {
			alta, _ := altaDePrueba(t, e, "casa", p.nombre+"_"+c.nombre)
			texto, err := fleet.Muestra{Tomada: c.tomada, NumCPU: 8, MemTotal: 16000, MemUsada: 8000}.Serializar()
			if err != nil {
				t.Fatal(err)
			}
			if err := p.latir(alta.ID, texto); err != nil {
				t.Fatalf("%s/%s: latir: %v", p.nombre, c.nombre, err)
			}
			d, hay, err := e.DevicePorID(alta.ID)
			if err != nil || !hay || d.UltimaMuestra == nil {
				t.Fatalf("%s/%s: no quedó la muestra: hay=%v err=%v", p.nombre, c.nombre, hay, err)
			}
			if !d.UltimaMuestra.Tomada.Equal(c.quedar) {
				t.Errorf("%s/%s: el agente la fechó %s, llegó %s y quedó guardada con %s; tenía que quedar %s. "+
					"La frescura la mide el reloj del cerebro: una muestra no puede haber sido tomada después de llegar",
					p.nombre, c.nombre, c.tomada.Format(time.RFC3339Nano), llegada.Format(time.RFC3339Nano),
					d.UltimaMuestra.Tomada.Format(time.RFC3339Nano), c.quedar.Format(time.RFC3339Nano))
			}
			if d.UltimaMuestra.NumCPU != 8 || d.UltimaMuestra.MemUsada != 8000 {
				t.Errorf("%s/%s: el recorte se llevó el resto de la muestra: %+v", p.nombre, c.nombre, *d.UltimaMuestra)
			}
		}
	}
}
