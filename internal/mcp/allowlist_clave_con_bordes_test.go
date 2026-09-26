package mcp

import (
	"slices"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// A131 · T3 (revisión 2) — UNA CLAVE DE `fleet_exec_allow` CON BORDES SE LEE COMO LA LIMPIA, EN LOS
// TRES LUGARES QUE LA LEEN: LA COMPUERTA, EL INVENTARIO Y EL INFORME DEL RENAME.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJE CLAVABAN LAS GUARDAS DE ANTES
//
// Las pruebas de la allowlist (fleet_s10_test.go) arman el principal con claves limpias —`nas`,
// `pc-gio`, `produccion`, `*`— y la tabla de alcance (TestUnaPoliticaActuaYFiguraSoloSobreLasMaquinasQueNombra)
// la arma con parsearExecAllow, que recorta. A argvPermitido y a comandosPermitidos no les llegaba
// nunca una clave con bordes, y buscaban `p.ExecAllow[d.Name]` y `p.ExecAllow[comodinFlota]` sin
// recortar, mientras el informe del rename (impactoDeNombre) ya leía la MISMA clave con
// SelectorNombra. Medido en la revisión 2 de T3 con una sonda sobre 5b19840: con la clave
// ` davantis `, impactoDeNombre("davantis").Allowlists daba ["op"] y argvPermitido(uptime) daba false;
// con ` * `, EsComodin daba true, argvPermitido false y comandosPermitidos una lista vacía. Tres
// lectores del mismo selector y dos respuestas. Medido al escribirla, con esta misma prueba: contra
// la producción de 5b19840 cae en cuatro filas —las dos claves con bordes que nombran a la máquina,
// el comodín con bordes y la vecina que cae en él—, en la compuerta y en el inventario; contra la de
// la base (2a1601d), además en el informe del rename de tres de ellas, que entonces también buscaba
// `p.ExecAllow[device]`.
//
// Exposición: 0. parsearExecAllow recorta cada clave y es el único constructor de
// Principal.ExecAllow en producción. Lo que esta guarda evita es la segunda lectura: el día que otro
// camino arme allowlists, la compuerta, lo que muestra el inventario y lo que avisa el rename no
// pueden discrepar sobre qué entrada manda. La lectura que comparten es fleet.EntradaDeAllowlist, y
// que nadie más lea la allowlist por clave lo mide, sobre el código,
// TestLaAllowlistYElComodinSeLeenSoloConLaGramatica.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Por fila, una allowlist CRUDA —sin pasar por el parser— y una máquina, con tres HECHOS escritos:
// si `uptime` pasa (argvPermitido), qué lista muestra el inventario (comandosPermitidos) y si el
// informe del rename la lista como «se rompe» (impactoDeNombre). Las máquinas son el par real de la
// malla, `davantis` y `davantis-1`. Y cada fila contra su gemela LIMPIA —las mismas claves,
// recortadas—: las tres lecturas tienen que dar lo mismo con y sin bordes, que es la frase que la
// gramática promete.
//
// PISO: la tabla trae una clave con bordes que nombra a la máquina y deja pasar; el comodín con bordes
// que deja pasar donde ninguna clave nombra a la máquina; y una entrada con bordes VACÍA que manda
// sobre un comodín con bordes, que es la precedencia leída con bordes de los dos lados. Se clasifica
// con strings.TrimSpace y no con la gramática, que es lo que se mide.
//
// Sabotaje: argvPermitido vuelve a buscar la entrada por su cuenta, por clave y sin recortar (el
// código de antes de la revisión 2, puesto al principio de la función). El arreglo es otra forma
// correcta de escribirla: preguntarle a comandosPermitidos, que busca con la misma gramática.
// arnes: archivo="internal/mcp/fleet_authz.go"
// arnes: de="func argvPermitido(p *Principal, d fleet.Device, argv []string) bool {\n"
// arnes: a="func argvPermitido(p *Principal, d fleet.Device, argv []string) bool {\n\tif p != nil && p.ExecAllow != nil {\n\t\tif lista, hay := p.ExecAllow[d.Name]; hay {\n\t\t\treturn fleet.PermiteArgv(lista, argv)\n\t\t}\n\t\tif lista, hay := p.ExecAllow[fleet.ComodinMaquinas]; hay {\n\t\t\treturn fleet.PermiteArgv(lista, argv)\n\t\t}\n\t\treturn false\n\t}\n"
// arnes: arreglo_de="func argvPermitido(p *Principal, d fleet.Device, argv []string) bool {\n"
// arnes: arreglo_a="func argvPermitido(p *Principal, d fleet.Device, argv []string) bool {\n\tif comandos, acotada := comandosPermitidos(p, d); p != nil {\n\t\treturn !acotada || fleet.PermiteArgv(comandos, argv)\n\t}\n"
//
// Sabotaje: comandosPermitidos vuelve a buscar la entrada por su cuenta, por clave y sin recortar. El
// arreglo es la misma búsqueda con la lista vacía como valor por defecto en vez de un `return` aparte.
// arnes: archivo="internal/mcp/fleet_authz.go"
// arnes: de="func comandosPermitidos(p *Principal, d fleet.Device) ([]string, bool) {\n"
// arnes: a="func comandosPermitidos(p *Principal, d fleet.Device) ([]string, bool) {\n\tif p != nil && p.ExecAllow != nil {\n\t\tif lista, hay := p.ExecAllow[d.Name]; hay {\n\t\t\treturn lista, true\n\t\t}\n\t\tif lista, hay := p.ExecAllow[fleet.ComodinMaquinas]; hay {\n\t\t\treturn lista, true\n\t\t}\n\t\treturn []string{}, true\n\t}\n"
// arnes: arreglo_de="\tif comandos, _, hay := fleet.EntradaDeAllowlist(p.ExecAllow, d.Name); hay {\n\t\treturn comandos, true\n\t}\n\treturn []string{}, true\n"
// arnes: arreglo_a="\tcomandos, _, hay := fleet.EntradaDeAllowlist(p.ExecAllow, d.Name)\n\tif !hay {\n\t\tcomandos = []string{}\n\t}\n\treturn comandos, true\n"
func TestUnaClaveDeLaAllowlistConBordesSeLeeComoLaLimpia(t *testing.T) {
	type fila struct {
		caso    string
		allow   map[string][]string // CRUDA: sin parsearExecAllow
		maquina string
		// HECHOS
		pasa     bool     // `uptime` pasa por la compuerta
		comandos []string // la allowlist efectiva que muestra el inventario
		nombra   bool     // el informe del rename la lista como «se rompe»
	}
	filas := []fila{
		{"la clave limpia (control)", map[string][]string{"davantis": {"uptime"}}, "davantis",
			true, []string{"uptime"}, true},
		{"la clave con espacios", map[string][]string{" davantis ": {"uptime"}}, "davantis",
			true, []string{"uptime"}, true},
		{"la clave con tabulador y salto de línea", map[string][]string{"\tdavantis\n": {"uptime"}}, "davantis",
			true, []string{"uptime"}, true},
		{"la clave con espacios no nombra a la vecina que la extiende", map[string][]string{" davantis ": {"uptime"}}, "davantis-1",
			false, []string{}, false},
		{"el comodín limpio (control)", map[string][]string{"*": {"uptime"}}, "davantis",
			true, []string{"uptime"}, false},
		{"el comodín con espacios", map[string][]string{" * ": {"uptime"}}, "davantis",
			true, []string{"uptime"}, false},
		{"una entrada vacía con bordes manda sobre el comodín con bordes", map[string][]string{" davantis ": {}, " * ": {"uptime"}}, "davantis",
			false, []string{}, true},
		{"la vecina que nadie nombra cae en el comodín con bordes", map[string][]string{" davantis ": {}, " * ": {"uptime"}}, "davantis-1",
			true, []string{"uptime"}, false},
	}

	// PISO, clasificado con TrimSpace y no con la gramática que se mide.
	conBordesQueNombra, comodinConBordes, vaciaSobreComodin := false, false, false
	for _, f := range filas {
		nombrada, vaciaNombrada, comodinBordes := false, false, false
		for clave, lista := range f.allow {
			limpia := strings.TrimSpace(clave)
			switch {
			case clave != limpia && limpia == f.maquina:
				nombrada = true
				vaciaNombrada = len(lista) == 0
			case clave != limpia && limpia == fleet.ComodinMaquinas:
				comodinBordes = true
			}
		}
		conBordesQueNombra = conBordesQueNombra || (nombrada && f.pasa)
		comodinConBordes = comodinConBordes || (comodinBordes && !nombrada && f.pasa)
		vaciaSobreComodin = vaciaSobreComodin || (vaciaNombrada && comodinBordes && !f.pasa)
	}
	if !conBordesQueNombra || !comodinConBordes || !vaciaSobreComodin {
		t.Fatalf("PISO: la tabla trae una clave con bordes que nombra y deja pasar = %v, el comodín con bordes que deja "+
			"pasar = %v y una entrada vacía con bordes sobre un comodín con bordes = %v, y necesita las tres: sin ellas, una "+
			"búsqueda por clave sin recortar pasa en verde", conBordesQueNombra, comodinConBordes, vaciaSobreComodin)
	}

	for _, f := range filas {
		limpia := map[string][]string{}
		for clave, lista := range f.allow {
			limpia[strings.TrimSpace(clave)] = lista
		}
		for _, v := range []struct {
			version string
			allow   map[string][]string
		}{{"cruda", f.allow}, {"limpia", limpia}} {
			p := conAllowlist("casa", v.allow)
			d := fleet.Device{Name: f.maquina, ProjectID: "casa"}
			if got := argvPermitido(p, d, []string{"uptime"}); got != f.pasa {
				t.Errorf("%s (allowlist %s %q): argvPermitido(uptime, %s) = %v y la fila dice %v. La compuerta lee la clave "+
					"de `fleet_exec_allow` distinto que la gramática de las máquinas: abre o cierra un comando según cómo se "+
					"escribió la clave, no según qué máquina nombra", f.caso, v.version, f.allow, f.maquina, got, f.pasa)
			}
			comandos, acotada := comandosPermitidos(p, d)
			if !acotada || !slices.Equal(comandos, f.comandos) {
				t.Errorf("%s (allowlist %s %q): el inventario de %s muestra %q (acotada=%v) y la fila dice %q: lo que se "+
					"muestra como permitido no es lo que la compuerta aplica", f.caso, v.version, f.allow, f.maquina, comandos,
					acotada, f.comandos)
			}
			registro := registroDePrueba(*p)
			if got := slices.Contains(registro.impactoDeNombre(f.maquina).Allowlists, p.Name); got != f.nombra {
				t.Errorf("%s (allowlist %s %q): el informe del rename de %s lista la allowlist = %v y la fila dice %v: el "+
					"informe avisa que se rompe una entrada que la compuerta no lee así, o calla una que sí",
					f.caso, v.version, f.allow, f.maquina, got, f.nombra)
			}
		}
	}
}
