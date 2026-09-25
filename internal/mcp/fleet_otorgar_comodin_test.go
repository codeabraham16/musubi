package mcp

import (
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// A131 · T3 (revisión) — PUEDE OTORGAR QUIEN TIENE EL COMODÍN, Y EL COMODÍN SE LEE COMO LO LEE LA
// GRAMÁTICA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJE CLAVABA LA GUARDA DE ANTES
//
// TestNoSePuedeOtorgarLoQueNoSeTiene (C7) arma dos concesiones: una con nombres (`pc-gio`, `nas`)
// y otra con `*` pelado. A puedeOtorgar nunca le llegaba un comodín con bordes, y la compuerta
// comparaba `selector == comodinFlota` sin recortar, mientras fleet.SelectorAlcanza —la que usa
// tieneGrant— recorta. Con ` * `, la misma concesión daba la capacidad sobre toda máquina y negaba
// otorgarla en una que nace: dos lecturas del mismo selector, y ninguna prueba que las enfrentara.
//
// Exposición medida: 0. parsearFleet recorta cada selector de `principals.yaml` antes de que llegue
// a una compuerta, y es el único constructor de Principal.Fleet en producción (principals.go).
// Medido en la revisión con una sonda sobre la punta de T3: puedeOtorgar con ` * ` y con `\t*\n`
// daba false, y SelectorAlcanza con los mismos, true. Lo que esta guarda evita es que la lectura
// propia vuelva: el día que otro camino arme concesiones, la compuerta ya no puede discrepar.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Por cada selector —el comodín pelado y con bordes, y lo que se le parece sin serlo: dos
// asteriscos, un asterisco con otro adentro o pegado a un nombre, un glob, un nombre, vacío—, una
// concesión con ESE selector CRUDO, sin pasar por el parser: puede otorgar exactamente cuando la
// fila dice que es el comodín. La columna está ESCRITA, no derivada de la función que se mide; y la
// gramática la confirma aparte (EsComodin, y alcanzar a una máquina que el selector no nombra), para
// que una fila mal escrita no se lea como una compuerta rota.
//
// PISO: la tabla trae al menos un comodín con bordes que puede, y un parecido con asterisco que no.
// Sin el primero, una comparación sin recorte pasa; sin el segundo, una compuerta que acepte
// cualquier asterisco pasa.
//
// Sabotaje: la compuerta vuelve a leer el comodín por su cuenta, sin recortar. (Con
// fleet.ComodinMaquinas: el alias local `comodinFlota` que usaba este sabotaje se borró en la
// revisión 2 de T3, justamente por ser la puerta de estas lecturas propias.)
// arnes: archivo="internal/mcp/fleet_authz.go"
// arnes: de="\t\tif fleet.EsComodin(selector) {\n"
// arnes: a="\t\tif selector == fleet.ComodinMaquinas {\n"
func TestPuedeOtorgarLeeElComodinComoLaGramatica(t *testing.T) {
	filas := []struct {
		selector string
		comodin  bool // HECHO: es el comodín, y con él se puede otorgar en una máquina que nace
		porque   string
	}{
		{"*", true, "el comodín"},
		{" * ", true, "el comodín con bordes: parsearFleet y SelectorAlcanza los recortan"},
		{"\t*\n", true, "bordes que no son espacios"},
		{"**", false, "dos asteriscos no son el comodín"},
		{"* *", false, "un asterisco con otro adentro"},
		{"*davantis", false, "un asterisco pegado a un nombre"},
		{"davan*", false, "un glob: el asterisco sólo es comodín solo"},
		{"davantis", false, "un nombre: tenerla en una máquina no es tenerla en la que nace"},
		{"", false, "vacío"},
		{"  ", false, "sólo espacios"},
	}

	// PISO: un comodín con bordes que puede, y un parecido con asterisco que no.
	conBordes, parecidos := 0, 0
	for _, f := range filas {
		switch {
		case f.comodin && f.selector != fleet.ComodinMaquinas:
			conBordes++
		case !f.comodin && strings.Contains(f.selector, fleet.ComodinMaquinas):
			parecidos++
		}
	}
	if conBordes == 0 || parecidos == 0 {
		t.Fatalf("PISO: la tabla trae %d comodín(es) con bordes y %d parecido(s) con asterisco, y necesita al menos uno "+
			"de cada uno: sin ellos, una lectura del comodín sin recorte, o una que acepte cualquier asterisco, pasa en verde",
			conBordes, parecidos)
	}

	const nueva = "la-que-nace"
	for _, f := range filas {
		// La gramática confirma la fila. Si esto falla, la que está mal es la fila (o cambió la
		// gramática, que mide TestUnSelectorAlcanzaSoloAlComodinOAlNombreExacto), no la compuerta.
		gramatica := fleet.EsComodin(f.selector) && fleet.SelectorAlcanza(f.selector, nueva) && !fleet.SelectorNombra(f.selector, nueva)
		if gramatica != f.comodin {
			t.Errorf("LA FILA Y LA GRAMÁTICA DISCREPAN sobre %q: la fila dice comodín=%v y la gramática, %v (%s)",
				f.selector, f.comodin, gramatica, f.porque)
			continue
		}
		p := &Principal{Name: "op", Role: RoleAdmin, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
			Fleet: map[fleet.Cap][]string{fleet.CapExec: {f.selector}}}
		if got := puedeOtorgar(p, fleet.CapExec); got != f.comodin {
			t.Errorf("puedeOtorgar con `exec: [%q]` = %v y la fila dice %v (%s): la compuerta que deja conceder una "+
				"capacidad a una máquina nueva lee el comodín distinto que la que la aplica, así que la misma concesión "+
				"alcanza a todas y no deja otorgar, o deja otorgar sin alcanzar a todas", f.selector, got, f.comodin, f.porque)
		}
	}
}
