package fleet

import "testing"

// UN DESTINO MAL ESCRITO SE DESCARTA AL CONFIGURAR, no se sondea para reportar `false`.
//
// Si se sondeara igual, un typo se vería IDÉNTICO a un relay caído — y son dos problemas de dos
// personas distintas: uno lo arregla quien escribió la configuración, el otro quien opera la red.
// Distinguirlos después, mirando una serie en 0, es imposible.
//
// Sabotaje: que DestinoDeAlcanceValido devuelva true a secas → falla acá.
func TestUnDestinoMalEscritoNoLlegaASondearse(t *testing.T) {
	malos := []string{"", "  ", "sinpuerto", "host:", ":21116", "host:0", "host:65536",
		"host:abc", "host con espacio:21116", "a,b:21116"}
	for _, m := range malos {
		if DestinoDeAlcanceValido(m) {
			t.Errorf("se aceptó el destino inválido %q: un typo se va a ver igual que un relay caído", m)
		}
	}
	// EL CONTROL POSITIVO, sin el cual una función que devuelve false siempre pasaría lo de arriba.
	for _, b := range []string{"100.79.126.62:21116", "relay.casa:21117", "[::1]:21115"} {
		if !DestinoDeAlcanceValido(b) {
			t.Errorf("se rechazó el destino válido %q", b)
		}
	}
}

// LA LISTA SE LIMPIA Y LO QUE QUEDA AFUERA SE CUENTA.
//
// El conteo no es cosmético: sin él, un destino descartado desaparece en silencio y quien lo
// configuró lo descubre media hora después preguntándose por qué «la serie no aparece».
//
// Sabotaje: devolver siempre 0 en `descartados` → falla la segunda mitad.
func TestLosDestinosSeLimpianYLoQueQuedaAfueraSeCuenta(t *testing.T) {
	in := []string{
		"a:1", "a:1", // repetido
		"roto",                            // inválido
		"b:2", "c:3", "d:4", "e:5", "f:6", // pasa el tope de 4
	}
	got, descartados := LimpiarDestinosDeAlcance(in)
	if len(got) != AlcanceMaxDestinos {
		t.Errorf("quedaron %d destinos, esperaba el tope de %d: %v", len(got), AlcanceMaxDestinos, got)
	}
	// 1 repetido + 1 inválido + 2 que pasan el tope.
	if descartados != 4 {
		t.Errorf("se descartaron %d y no se contaron los 4 reales: quien configuró no se entera", descartados)
	}
	vistos := map[string]bool{}
	for _, d := range got {
		if vistos[d] {
			t.Errorf("quedó un destino repetido (%q): se sondearía dos veces por latido", d)
		}
		vistos[d] = true
	}
}
