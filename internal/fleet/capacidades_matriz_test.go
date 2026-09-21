package fleet

import "testing"

// todasLasCapacidades es la lista CLAVADA de lo que el dominio conoce hoy.
//
// Va clavada y no derivada de `capsPorTier` ni del `switch` del parser: derivarla de cualquiera de
// los dos dejaría a las guardas de abajo midiéndose contra la misma fuente que custodian. El
// control de que no se quedó corta vive en cada guarda, contra el tier que las admite todas.
var todasLasCapacidades = []Cap{CapMetrics, CapExec, CapScreen, CapScreenView, CapShell}

// TestCadaCapacidadVaYVuelveSiendoEllaMisma — LA COLUMNA QUE NADIE LEÍA.
//
// `CapsDesdeTexto` es lo que convierte la columna `caps` de la base en la lista que decide quién
// puede qué. `TestCapsIdaYVueltaYBasuraSeDescarta` la prueba con `[metrics, exec]` y con
// `"metrics, root ,,screen"`: NUNCA le pasa `screen:view`. Y en todo el repo hay UN solo test que
// escribe ese literal —`internal/mcp/fleet_cronologia_test.go`— y sólo mira un mensaje de error.
//
// O sea que la capacidad cuya razón de existir es la ASIMETRÍA era la única que el parser podía
// leer mal sin que nada se enterara. Medido el 2026-09-21: con `case CapScreenView: set[CapScreen]
// = true`, una fila que concede sólo MIRAR se lee como CONTROLAR, y `./internal/fleet`,
// `./internal/mcp` y `./internal/memory` quedan los tres en verde.
//
// LA TABLA RECORRE EL ENUM ENTERO y no los dos casos de siempre: preguntar por una lista de
// ejemplos es cómo se llega a que falte justo la que importa.
//
// Sabotaje que la hace fallar: que el parser lea `screen:view` como `screen` — la fila que sólo
// concede mirar pasa a permitir controlar.
// arnes: archivo="internal/fleet/device.go"
// arnes: de="\t\tswitch c := Cap(strings.ToLower(strings.TrimSpace(p))); c {\n\t\tcase CapMetrics, CapExec, CapScreen, CapScreenView, CapShell:\n\t\t\tset[c] = true\n\t\t}"
// arnes: a="\t\tswitch c := Cap(strings.ToLower(strings.TrimSpace(p))); c {\n\t\tcase CapScreenView:\n\t\t\tset[CapScreen] = true\n\t\tcase CapMetrics, CapExec, CapScreen, CapShell:\n\t\t\tset[c] = true\n\t\t}"
func TestCadaCapacidadVaYVuelveSiendoEllaMisma(t *testing.T) {
	// EL CONTROL DE QUE LA LISTA NO SE QUEDÓ CORTA. `TierAgente` es el tier que admite todo, así
	// que su fila es el censo del enum. Si algún día nace una capacidad que ese tier NO admite,
	// este control deja de servir y hay que darle otra fuente — se dice acá para que no se
	// descubra con una guarda callada.
	if n := len(CapsDelTier(TierAgente)); n != len(todasLasCapacidades) {
		t.Fatalf("`TierAgente` admite %d capacidades y esta tabla enumera %d: la tabla se quedó corta "+
			"y todo lo que sigue estaría midiendo un subconjunto", n, len(todasLasCapacidades))
	}

	for _, c := range todasLasCapacidades {
		got := CapsDesdeTexto(string(c))
		if len(got) != 1 || got[0] != c {
			t.Errorf("la columna %q se leyó como %v: una capacidad que va y vuelve siendo OTRA es una "+
				"fila que concede algo que su dueño no escribió", string(c), got)
		}
	}

	// Y TODAS JUNTAS, que es la forma real de la columna.
	if got := CapsDesdeTexto(CapsComoTexto(todasLasCapacidades)); len(got) != len(todasLasCapacidades) {
		t.Errorf("la ida y vuelta de la fila entera devolvió %v (%d de %d)", got, len(got), len(todasLasCapacidades))
	}
}

// TestPermiteNoInventaUnaImplicacionQueImplicaNiega — LA MATRIZ ENTERA, Y NO LA PAREJA DE SIEMPRE.
//
// `TestMirarNoEsControlarYControlarSiEsMirar` verifica los 25 pares de `Implica`.
// `TestElAparatoQueAdmiteControlarAdmiteMirar` dice custodiar que «la implicación vale también en
// la matriz del aparato», y prueba UNA pareja: la de pantalla. Nunca comprueba que no se cuele
// NINGUNA OTRA implicación en `Permite`.
//
// Medido el 2026-09-21: agregarle a `Permite` un `shell` ⇒ `exec` —una implicación que `Implica`
// niega— deja las dos guardas en verde, y `./internal/fleet`, `./internal/mcp` y
// `./internal/memory` también. El hermano sano acota el hallazgo: una de las dos guardas recorre
// la matriz entera y la otra no, y el agujero está exactamente en la que no.
//
// LA ESPERANZA VA CLAVADA Y NO DERIVADA DE `Implica`. Escribir `esperado := Implica(tiene, pedida)`
// sería cómodo y dejaría a esta guarda inútil: una mutación de `Implica` movería los dos lados a
// la vez y la guarda la certificaría sana. Acá se escribe la REGLA —uno mismo, o controlar para
// mirar— que es un hecho del dominio y por eso se clava.
//
// Sabotaje que la hace fallar: colarle a `Permite` la implicación `shell` ⇒ `exec`, que `Implica`
// niega.
// arnes: archivo="internal/fleet/device.go"
// arnes: de="\treturn false\n}\n\n// EnLinea deriva el estado de conexión"
// arnes: a="\tif c == CapExec {\n\t\tfor _, ct := range d.Caps {\n\t\t\tif ct == CapShell {\n\t\t\t\treturn TierAdmite(d.Tier, CapExec)\n\t\t\t}\n\t\t}\n\t}\n\treturn false\n}\n\n// EnLinea deriva el estado de conexión"
func TestPermiteNoInventaUnaImplicacionQueImplicaNiega(t *testing.T) {
	if n := len(CapsDelTier(TierAgente)); n != len(todasLasCapacidades) {
		t.Fatalf("`TierAgente` admite %d capacidades y esta tabla enumera %d: la matriz se quedó corta", n, len(todasLasCapacidades))
	}

	for _, tiene := range todasLasCapacidades {
		for _, pedida := range todasLasCapacidades {
			// UN APARATO DE UNA SOLA CAPACIDAD por celda: con varias, `Permite` devuelve en la
			// PRIMERA que implica y la celda dejaría de decir cuál de ellas decidió.
			d := Device{Tier: TierAgente, Caps: []Cap{tiene}}
			esperado := tiene == pedida || (tiene == CapScreen && pedida == CapScreenView)
			if got := d.Permite(pedida); got != esperado {
				t.Errorf("un aparato con %q %s %q, y la regla dice que %s.\n"+
					"  `Permite` no puede conceder nada que `Implica` niegue: la asimetría entre mirar y "+
					"controlar es toda la razón de haber partido la capacidad, y una implicación de más "+
					"acá la borra sin tocar `Implica`.",
					string(tiene), map[bool]string{true: "SÍ permite", false: "NO permite"}[got], string(pedida),
					map[bool]string{true: "sí", false: "no"}[esperado])
			}
		}
	}

	// EL CONTROL DEL CONTROL: el par que SÍ tiene que implicar. Sin esta línea, un `Permite` que
	// devolviera `false` para todo pasaría la matriz de arriba salvo la diagonal — y la diagonal
	// sola no distingue «la implicación está» de «la implicación se borró».
	soloControla := Device{Tier: TierAgente, Caps: []Cap{CapScreen}}
	if !soloControla.Permite(CapScreenView) {
		t.Error("quien puede CONTROLAR la pantalla dejó de poder MIRARLA: la implicación se perdió, " +
			"y con ella la bitácora de sesiones de todas las máquinas que declaran sólo `screen`")
	}
}
