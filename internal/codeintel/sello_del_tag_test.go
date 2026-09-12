package codeintel

import (
	"strings"
	"testing"
)

// TestElSelloAcusaSiElBinarioDerivaPolyglot — el eje que NO necesita ningún bump para abrirse.
//
// LO QUE SE MIDIÓ. Dos compilaciones del MISMO commit, una con `-tags treesitter` y otra sin, sobre
// el MISMO archivo `.py`:
//
//	sin tags : 0 nodos, 0 aristas, sello "4-crosspkg+ts-v0.52.0"
//	con tags : 3 nodos, 3 aristas, sello "4-crosspkg+ts-v0.52.0"
//
// El mismo sello para dos derivaciones que no se parecen en nada. `GraphDeriverVersion` es lo único
// que puede forzar al índice incremental a re-derivar un archivo cuyo contenido no cambió, así que
// un binario SIN el tag marcaba cada `.py`/`.ts` como «ya derivado» con CERO símbolos y ninguno se
// volvía a mirar NUNCA. Y no hace falta ningún bump de dependencia para llegar acá: alcanza con
// desplegar un binario compilado distinto, que es exactamente lo que separa a `deploy/construir.sh`
// de un `go build` a secas.
//
// ESTA GUARDA NO SE DEJA ENGAÑAR EDITANDO LA CONSTANTE, y es el punto. No pregunta «¿el sello dice
// poly-on?» —eso sería un espejo— sino si el sello COINCIDE CON LO QUE EL BINARIO HACE: deriva un
// archivo de verdad y cruza el resultado contra la marca. Un `motorPolyglot` mal puesto en
// cualquiera de las dos variantes la pone roja en ESA compilación.
//
// Por eso el archivo NO lleva build tag: tiene que compilar y correr en las dos, y en cada una mide
// la mitad que le toca. El job `test` corre la de sin tags; el paso polyglot, la otra.
func TestElSelloAcusaSiElBinarioDerivaPolyglot(t *testing.T) {
	// Un .py con dos funciones y una llamada: si el binario deriva polyglot, esto da nodos.
	nodos, _ := derivePolyglotFile("sonda.py", "def a():\n    pass\n\ndef b():\n    a()\n")
	deriva := len(nodos) > 0

	// EL CONTROL DE QUE LAS DOS MARCAS SON DISTINTAS. Si alguien las igualara, el sello dejaría de
	// distinguir las dos compilaciones y esta prueba pasaría en las dos sin custodiar nada.
	const marcaPrendida, marcaApagada = "poly-on", "poly-off"
	if marcaPrendida == marcaApagada {
		t.Fatal("las dos marcas del build tag son iguales: el sello no puede distinguir las dos compilaciones")
	}
	if motorPolyglot != marcaPrendida && motorPolyglot != marcaApagada {
		t.Fatalf("motorPolyglot vale %q y no es ninguna de las dos marcas conocidas (%q / %q): "+
			"si cambiaron los nombres, esta guarda dejó de saber qué está mirando",
			motorPolyglot, marcaPrendida, marcaApagada)
	}

	quiero := marcaApagada
	if deriva {
		quiero = marcaPrendida
	}
	if motorPolyglot != quiero {
		t.Errorf(`EL SELLO MIENTE SOBRE QUÉ DERIVA ESTE BINARIO.
  derivePolyglotFile(".py") devolvió %d nodos ⇒ este binario %s deriva polyglot
  motorPolyglot dice %q y tendría que decir %q

Dos binarios que derivan grafos distintos NO pueden escribir el mismo sello: el índice incremental
usa GraphDeriverVersion como la única puerta para re-derivar un archivo que no cambió, así que el
grafo queda mezclado entre las dos compilaciones y nada lo declara.`,
			len(nodos), map[bool]string{true: "SÍ", false: "NO"}[deriva], motorPolyglot, quiero)
	}

	// Y que la marca VIAJE en el sello: si no está adentro, cambiarla no re-deriva nada.
	if !strings.Contains(GraphDeriverVersion, motorPolyglot) {
		t.Errorf("GraphDeriverVersion (%q) no contiene motorPolyglot (%q): la marca del build tag no "+
			"viaja en el sello, así que compilar distinto no dispara ninguna re-derivación",
			GraphDeriverVersion, motorPolyglot)
	}
	t.Logf("deriva=%v marca=%q sello=%q", deriva, motorPolyglot, GraphDeriverVersion)
}
