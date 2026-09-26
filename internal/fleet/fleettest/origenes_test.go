package fleettest_test

import (
	"sort"
	"strings"
	"testing"

	"musubi/internal/fleet"
	"musubi/internal/fleet/fleettest"
)

// EL CORPUS DE ORÍGENES RAROS TRAE, POR CADA VALOR DEL ENUM, CADA FORMA DE PARECÉRSELE.
//
// Lo consumen tres guardas: la lista blanca del dominio (internal/fleet) y las puertas de escritura y
// de lectura de la tabla (internal/memory). Hasta la revisión de A131 (tema T9) cada una lo derivaba
// por su lado; ahora sale de fleettest.OrigenesParecidos y esto cuida lo que sale.
//
// POR QUÉ HACE FALTA ADEMÁS DE LOS PISOS DE LOS CONSUMIDORES: los dos piden «al menos diez» raros, y
// con el enum de hoy (persona, politica y el vacío) el corpus da doce. Perder las mayúsculas deja
// diez: los pisos quedan verdes y ninguna guarda vuelve a preguntar si `PERSONA` se guarda como
// persona. Acá se pide cada variante de cada valor, no una cantidad.
//
// El enum es el de verdad (fleet.OrigenesDeComando, que el dominio cierra contra su bloque const): un
// origen nuevo recibe sus variantes sin que nadie lo escriba acá.
//
// Sabotaje: que el corpus pierda las mayúsculas (pasarlas a minúsculas las vuelve el valor mismo, que
// se descarta) → `PERSONA` y `POLITICA` dejan de recorrerse y los pisos de los consumidores no lo notan.
// arnes: archivo="internal/fleet/fleettest/origenes.go"
// arnes: de="\t\tsumar(strings.ToUpper(s))\n"
// arnes: a="\t\tsumar(strings.ToLower(s))\n"
func TestLosOrigenesParecidosSeDerivanDeCadaValorDelEnum(t *testing.T) {
	var enum []string
	for _, o := range fleet.OrigenesDeComando {
		enum = append(enum, string(o))
	}
	// EL PISO del enum: con menos, el recorrido de abajo mide menos de lo que dice.
	if len(enum) < 3 {
		t.Fatalf("fleet.OrigenesDeComando trae %d orígenes (%q) y cuando se escribió esta prueba eran 3", len(enum), enum)
	}
	delEnum := map[string]bool{}
	for _, o := range enum {
		delEnum[o] = true
	}

	raros := fleettest.OrigenesParecidos(enum)
	hay := map[string]bool{}
	for _, r := range raros {
		if delEnum[r] {
			t.Errorf("el corpus trae %q y es un valor del enum: una guarda que lo espere desconocido se pondría "+
				"roja con un origen legítimo", r)
		}
		if hay[r] {
			t.Errorf("el corpus repite %q", r)
		}
		hay[r] = true
	}
	if !sort.StringsAreSorted(raros) {
		t.Errorf("el corpus no viene ordenado (%q): los mensajes de los consumidores cambiarían de corrida en corrida", raros)
	}

	// Cada forma de parecerse, para cada valor. Una variante que coincide con otro valor del enum
	// (el vacío en mayúsculas es el vacío) no es rara y no se pide.
	formas := []struct {
		nombre  string
		derivar func(string) string
	}{
		{"en mayúsculas", strings.ToUpper},
		{"con un blanco atrás", func(s string) string { return s + " " }},
		{"con un blanco adelante", func(s string) string { return " " + s }},
		{"con una letra de más", func(s string) string { return s + "x" }},
	}
	for _, o := range enum {
		for _, f := range formas {
			v := f.derivar(o)
			if delEnum[v] {
				continue
			}
			if !hay[v] {
				t.Errorf("el corpus no trae %q (%q %s): las guardas que lo consumen no preguntan qué pasa con "+
					"un origen que se le parece así", v, o, f.nombre)
			}
		}
	}
	for _, s := range fleettest.OrigenesSinParecido {
		if !hay[s] {
			t.Errorf("el corpus no trae %q, que no se parece a ningún valor del enum", s)
		}
	}
	if len(fleettest.OrigenesSinParecido) < 2 {
		t.Errorf("OrigenesSinParecido trae %d valores y cuando se escribió esta prueba eran 2", len(fleettest.OrigenesSinParecido))
	}
}
