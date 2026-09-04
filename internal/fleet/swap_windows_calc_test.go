package fleet

import "testing"

// UN SWAP QUE NO SE PUDO MEDIR NO SE REPORTA COMO LLENO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// El colector arrancaba `libre := 0` y sólo lo pisaba si `disponiblePagina > disponibleFisica`.
// Cuando esa resta no daba —máquina con mucha RAM libre y page file chico, o sea una máquina
// SANA— el cero quedaba puesto y salía `SwapUsada == SwapTotal`: swap al 100 %.
//
// Es el colector AFIRMANDO una emergencia porque no supo medir. La regla del cero mentiroso del
// lado más caro, y la misma forma que A70 (`ocioso` exportado como caído).
//
// VIVÍA DONDE NADIE PODÍA PROBARLO: cuatro líneas de resta adentro de `//go:build windows`, en un
// repo que se desarrolla en Linux. Por eso la aritmética se sacó del build tag — el mismo
// movimiento que ya se había hecho con los parsers de servicios.
//
// Sabotaje que lo hace fallar: devolver (total, total, true) cuando disponiblePagina <= disponibleFisica.
func TestUnSwapQueNoSePudoMedirNoViajaComoLleno(t *testing.T) {
	const gb = uint64(1) << 30

	casos := []struct {
		nombre                               string
		totalPag, totalFis, dispPag, dispFis uint64
		quiereOK                             bool
		quiereTotal, quiereUsada             uint64
	}{
		{"medible: 8 GB de page file, 6 GB libres",
			24 * gb, 16 * gb, 10 * gb, 4 * gb, true, 8 * gb, 2 * gb},
		{"NO medible: mucha RAM libre y page file chico — antes salía 100 % lleno",
			20 * gb, 16 * gb, 4 * gb, 6 * gb, false, 0, 0},
		{"NO medible: los dos disponibles iguales",
			20 * gb, 16 * gb, 5 * gb, 5 * gb, false, 0, 0},
		{"sin swap: el page file no supera la RAM",
			16 * gb, 16 * gb, 8 * gb, 4 * gb, false, 0, 0},
		{"incoherente: el libre supera al total, se descarta el par entero",
			17 * gb, 16 * gb, 20 * gb, 4 * gb, false, 0, 0},
	}

	for _, c := range casos {
		total, usada, ok := SwapDeWindows(c.totalPag, c.totalFis, c.dispPag, c.dispFis)
		if ok != c.quiereOK {
			t.Errorf("%s: ok=%v, esperaba %v", c.nombre, ok, c.quiereOK)
			continue
		}
		if total != c.quiereTotal || usada != c.quiereUsada {
			t.Errorf("%s: (total=%d usada=%d), esperaba (%d, %d)", c.nombre, total, usada, c.quiereTotal, c.quiereUsada)
		}
		// LO QUE NO PUEDE PASAR NUNCA, dicho aparte porque es el defecto exacto: si se reporta,
		// no puede ser «lleno» salvo que de verdad esté lleno.
		if ok && usada == total && c.quiereUsada != c.quiereTotal {
			t.Errorf("%s: se reportó el swap al 100 %% sin estarlo", c.nombre)
		}
	}

	// Y la regla de los pares: lo que sale de acá tiene que pasar Muestra.Valida.
	total, usada, ok := SwapDeWindows(24*gb, 16*gb, 10*gb, 4*gb)
	if !ok {
		t.Fatal("el caso medible dejó de serlo")
	}
	m := Muestra{SwapTotal: total, SwapUsada: usada}
	if err := m.Valida(); err != nil {
		t.Errorf("una muestra con el swap que produce este cálculo no valida: %v", err)
	}
}
