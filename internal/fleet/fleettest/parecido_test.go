package fleettest

import "testing"

// A131 · T3 (revisión) — CADA FORMA DE PARECIDO RECONOCE SU PAR, Y LO QUE NO SE PARECE NO TIENE
// FORMA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ HACE FALTA
//
// Los PISOS de las tablas de alcance —las dos de máquinas y, desde la revisión 2 de T3, las dos de
// servicios, en internal/fleet y en internal/mcp— cuentan las formas que DeParecido les devuelve, y
// exigen cada una. Si una forma aceptara cualquier par, el piso la encontraría en pares que no se
// parecen en nada (`nas` frente a `davantis`) y quedaría satisfecho sin que la tabla trajera la fila
// que la ejercita: la guarda se volvería hueca desde el instrumento que la mide, que es el mismo
// agujero que dejó al glob afuera de la tabla de los consumidores. Y si una forma dejara de reconocer
// su par, los pisos pedirían una fila que no se puede escribir. Esta prueba fija las dos direcciones.
//
// No hay producción detrás: es el instrumento de los pisos, y lo que se mide es que no mienta.
//
// PISO: la tabla trae un par de cada forma de Formas(), y pares sin forma.
//
// Sabotaje: el glob acepta cualquier par. Es la última forma, así que se queda con todo lo que las
// anteriores no explican, incluidos los pares que no se parecen en nada.
// arnes: archivo="internal/fleet/fleettest/parecido.go"
// arnes: de="\t\treturn err == nil && calza\n"
// arnes: a="\t\treturn err == nil && calza || s != \"\"\n"
//
// Sabotaje: la subcadena deja de reconocer su par.
// arnes: archivo="internal/fleet/fleettest/parecido.go"
// arnes: de="{Subcadena, func(s, n string) bool { return strings.Contains(n, s) }},"
// arnes: a="{Subcadena, func(s, n string) bool { return false && strings.Contains(n, s) }},"
func TestCadaFormaDeParecidoReconoceSuParYNadaMas(t *testing.T) {
	casos := []struct {
		selector, nombre string
		forma            Forma
		porque           string
	}{
		{"davantis", "davantis-1", Prefijo, "el par real de la malla: la laptop Linux y la PC Windows"},
		{"davan", "davantis", Prefijo, "también es subcadena, y se lleva la PRIMERA forma que lo describe"},
		{"davantis-1", "davantis", NombrePrefijo, "la comparación al revés"},
		{"antis", "davantis", Sufijo, "sufijo"},
		{"antis", "davantis-1", Subcadena, "el mismo selector, adentro de otro nombre sin tocar sus bordes"},
		{"vant", "davantis", Subcadena, "subcadena pura"},
		{"DAVANTIS", "davantis", Mayusculas, "el mismo nombre sin distinguir mayúsculas"},
		{"davan*", "davantis", Glob, "un patrón que calza"},

		// Lo que NO es un parecido.
		{"nas", "davantis", "", "no comparten nada"},
		{"otra-pc", "davantis-1", "", "no comparten nada"},
		{"zz*", "davantis", "", "un patrón que no calza: la regla glob tampoco confundiría este par"},
		{" davantis ", "davantis", "", "es el nombre: la gramática recorta los bordes del selector"},
		{"davantis", "davantis", "", "es el nombre"},
		{"", "davantis", "", "un selector vacío no es un selector"},
		{"davantis", "", "", "una máquina sin nombre no existe"},
	}

	// PISO: una fila de cada forma, y filas sin forma.
	vistas := map[Forma]bool{}
	sinForma := 0
	for _, c := range casos {
		if c.forma == "" {
			sinForma++
			continue
		}
		vistas[c.forma] = true
	}
	for _, f := range Formas() {
		if !vistas[f] {
			t.Errorf("PISO: ningún caso de la tabla tiene la forma %q. Si DeParecido dejara de reconocerla, los "+
				"pisos de las dos tablas de alcance pedirían una fila imposible y nada lo diría acá", f)
		}
	}
	if sinForma == 0 {
		t.Errorf("PISO: la tabla no trae ningún par sin parecido, y es el que caza a una forma que acepta cualquier par")
	}

	for _, c := range casos {
		got := DeParecido(c.selector, c.nombre)
		switch {
		case got == c.forma:
		case c.forma == "":
			t.Errorf("DeParecido(%q, %q) = %q y el par no se parece en nada (%s): el PISO de cada tabla de alcance "+
				"contaría esta forma sobre un par que no pone rojo nada, y la tabla podría perder la fila que la "+
				"ejercita sin enterarse", c.selector, c.nombre, got, c.porque)
		default:
			t.Errorf("DeParecido(%q, %q) = %q y el caso dice %q (%s): los pisos de las tablas de alcance contarían "+
				"este par en otra forma, o en ninguna", c.selector, c.nombre, got, c.forma, c.porque)
		}
	}
}
