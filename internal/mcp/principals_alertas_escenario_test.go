package mcp

import (
	"fmt"
	"testing"
)

// principals_alertas_escenario_test.go — que las tres alertas de credenciales DISPAREN cuando pasa
// lo que avisan, y se callen el resto del tiempo.
//
// TestLasAlertasDeCredencialesLeenSeriesQueElCerebroEmite cruza los NOMBRES de las series, y
// TestElUmbralDeLaAlertaEsElDelListado el número de CredencialPorVencer. Ninguna de las dos mira lo
// que la `expr` HACE con el valor, y un cambio de una línea la deja muda:
//
//	delta(musubi_principals_expired[2h]) > 1     un solo vencimiento —el caso de todos los días— no dispara
//	musubi_principals_reload_failing == 2        un 0/1 comparado contra un número que no toma nunca
//
// Las dos compilan, pasan el `promtool check` y leen series que existen. La guarda de alcance
// (alertas_umbral_alcanzable_test.go) tampoco las ve: sólo mira alertas sobre series GRABADAS, y
// un rango no alcanzaría igual —`> 1` es alcanzable si vencen dos en la misma ventana—.
//
// Así que acá la pregunta es por el ESCENARIO: con el valor que toma la serie cuando pasa lo que
// la alerta avisa, ¿dispara?; y con el de un día normal, ¿se calla? La `expr` se parsea con el
// mismo parser de la guarda de alcance, y una forma que este evaluador no conoce es ROJO: «no pude
// evaluar» no puede pasar por «dispara bien».

// valorEnVentana es lo que vale una serie al principio de la ventana de la regla y ahora.
type valorEnVentana struct{ antes, ahora float64 }

// evaluarEnEscenario evalúa una expresión con valores fijos. Devuelve el valor y si la expresión
// DEVUELVE ALGO: en PromQL una comparación falsa no da 0, no da nada, y eso es lo que decide si la
// alerta dispara. Entiende sólo lo que usan estas alertas; el resto es error a propósito.
func evaluarEnEscenario(n *nodoProm, esc map[string]valorEnVentana) (float64, bool, error) {
	switch n.tipo {
	case "num":
		return n.num, true, nil
	case "serie":
		if n.ventana != "" {
			return 0, false, fmt.Errorf("%s[%s] suelto: un selector con ventana sólo tiene valor adentro de una función", n.nombre, n.ventana)
		}
		v, ok := esc[n.nombre]
		if !ok {
			return 0, false, fmt.Errorf("el escenario no dice cuánto vale %s", n.nombre)
		}
		return v.ahora, true, nil
	case "llamada":
		// delta e idelta, las dos de un GAUGE. increase/rate tratan una bajada como el reinicio de
		// un contador, y sobre un gauge eso es otra pregunta: si alguien las pone, que se vea acá.
		if n.nombre != "delta" && n.nombre != "idelta" {
			return 0, false, fmt.Errorf("no sé evaluar %s(...) en un escenario: enseñáselo a evaluarEnEscenario si el cambio es a propósito", n.nombre)
		}
		if len(n.hijos) != 1 || n.hijos[0].tipo != "serie" || n.hijos[0].ventana == "" {
			return 0, false, fmt.Errorf("%s espera un único selector con ventana", n.nombre)
		}
		v, ok := esc[n.hijos[0].nombre]
		if !ok {
			return 0, false, fmt.Errorf("el escenario no dice cuánto vale %s", n.hijos[0].nombre)
		}
		return v.ahora - v.antes, true, nil
	case "binario", "cmp":
		a, hayA, err := evaluarEnEscenario(n.hijos[0], esc)
		if err != nil {
			return 0, false, err
		}
		b, hayB, err := evaluarEnEscenario(n.hijos[1], esc)
		if err != nil {
			return 0, false, err
		}
		if !hayA || !hayB {
			return 0, false, nil
		}
		if n.tipo == "cmp" {
			var cumple bool
			switch n.op {
			case "<":
				cumple = a < b
			case "<=":
				cumple = a <= b
			case ">":
				cumple = a > b
			case ">=":
				cumple = a >= b
			case "==":
				cumple = a == b
			case "!=":
				cumple = a != b
			default:
				return 0, false, fmt.Errorf("comparación %q que no conozco", n.op)
			}
			return a, cumple, nil
		}
		switch n.op {
		case "+":
			return a + b, true, nil
		case "-":
			return a - b, true, nil
		case "*":
			return a * b, true, nil
		case "/":
			if b == 0 {
				return 0, false, nil
			}
			return a / b, true, nil
		}
		return 0, false, fmt.Errorf("operador %q que no conozco", n.op)
	}
	return 0, false, fmt.Errorf("nodo %q que no sé evaluar en un escenario", n.tipo)
}

// LAS ALERTAS DE CREDENCIALES DISPARAN CUANDO PASA LO QUE AVISAN, Y SÓLO ENTONCES.
//
// Sabotaje que la pone roja: que un solo vencimiento no alcance para disparar.
// arnes: archivo="deploy/musubi-alerts.yml"
// arnes: de="        expr: delta(musubi_principals_expired[2h]) > 0\n"
// arnes: a="        expr: delta(musubi_principals_expired[2h]) > 1\n"
//
// Sabotaje que la pone roja: comparar el flag 0/1 contra un valor que no toma nunca.
// arnes: archivo="deploy/musubi-alerts.yml"
// arnes: de="        expr: musubi_principals_reload_failing == 1\n"
// arnes: a="        expr: musubi_principals_reload_failing == 2\n"
//
// Sabotaje que la pone roja: un `for:` más largo que la ventana del `delta` (la condición se apaga
// sola a las 2 h del salto, así que un plazo de 3 h no se cumple nunca).
// arnes: archivo="deploy/musubi-alerts.yml"
// arnes: de="          runbook: \"deploy/RUNBOOK.md#credencialrecienvencida\"\n"
// arnes: a="          runbook: \"deploy/RUNBOOK.md#credencialrecienvencida\"\n        for: 3h\n"
func TestLasAlertasDeCredencialesDisparanCuandoPasaLoQueAvisan(t *testing.T) {
	const dia = 24 * 3600
	type escenario struct {
		que     string
		series  map[string]valorEnVentana
		dispara bool
	}
	casos := []struct {
		alerta     string
		escenarios []escenario
	}{
		{"CredencialPorVencer", []escenario{
			{"a la próxima credencial le quedan 13 días", map[string]valorEnVentana{nombreProximoVencimiento: {ahora: 13 * dia}}, true},
			{"a la próxima credencial le quedan 15 días", map[string]valorEnVentana{nombreProximoVencimiento: {ahora: 15 * dia}}, false},
		}},
		{"CredencialRecienVencida", []escenario{
			{"vence UNA credencial", map[string]valorEnVentana{nombreCredencialesVencidas: {antes: 0, ahora: 1}}, true},
			{"vence otra con una vencida vieja en el archivo", map[string]valorEnVentana{nombreCredencialesVencidas: {antes: 1, ahora: 2}}, true},
			{"una vencida vieja sigue en el archivo y nada cambia", map[string]valorEnVentana{nombreCredencialesVencidas: {antes: 1, ahora: 1}}, false},
			{"se borra del archivo una credencial vencida", map[string]valorEnVentana{nombreCredencialesVencidas: {antes: 1, ahora: 0}}, false},
		}},
		{"RegistroDePrincipalsSinPoderRecargarse", []escenario{
			{"la relectura de principals.yaml se viene rechazando", map[string]valorEnVentana{nombreRegistroSinRecargar: {antes: 1, ahora: 1}}, true},
			{"el registro recarga bien", map[string]valorEnVentana{nombreRegistroSinRecargar: {antes: 0, ahora: 0}}, false},
		}},
	}

	alertas := map[string]alertaCruda{}
	for _, a := range alertasCrudasDelRepo(t) {
		if a.Archivo == "musubi-alerts.yml" {
			alertas[a.Nombre] = a
		}
	}
	for _, c := range casos {
		a, ok := alertas[c.alerta]
		if !ok {
			t.Errorf("no existe la alerta %s en musubi-alerts.yml", c.alerta)
			continue
		}
		arbol, err := parsearProm(a.Expr)
		if err != nil {
			t.Errorf("%s: no pude parsear %q: %v", c.alerta, a.Expr, err)
			continue
		}
		for _, e := range c.escenarios {
			_, dispara, err := evaluarEnEscenario(arbol, e.series)
			if err != nil {
				t.Errorf("%s: no pude evaluar %q cuando %s: %v", c.alerta, a.Expr, e.que, err)
				continue
			}
			if dispara != e.dispara {
				if e.dispara {
					t.Errorf("%s NO DISPARA cuando %s: `%s`. Es justo lo que avisa, y la alerta queda muda sin un solo error.",
						c.alerta, e.que, a.Expr)
				} else {
					t.Errorf("%s dispara cuando %s: `%s`. Una alerta que suena en un día normal entrena a ignorar el canal.",
						c.alerta, e.que, a.Expr)
				}
			}
		}
		// EL PLAZO TIENE QUE CABER EN LA VENTANA: `delta(x[2h]) > 0` es cierto durante las 2 h que
		// siguen al salto y después se apaga solo, así que un `for:` de 2 h o más no se cumple nunca.
		ventanas, err := ventanasDe(arbol)
		if err != nil {
			t.Errorf("%s: ventana ilegible en %q: %v", c.alerta, a.Expr, err)
			continue
		}
		if !a.TienePlazo || len(ventanas) == 0 {
			continue
		}
		plazo, err := duracionProm(a.Plazo)
		if err != nil {
			t.Errorf("%s: `for: %s` ilegible: %v", c.alerta, a.Plazo, err)
			continue
		}
		for _, w := range ventanas {
			if plazo >= w {
				t.Errorf("%s: `for: %s` no cabe en la ventana de %v de su `expr` (%q): la condición se apaga "+
					"sola antes de cumplir el plazo y la alerta no suena nunca", c.alerta, a.Plazo, w, a.Expr)
			}
		}
	}
}
