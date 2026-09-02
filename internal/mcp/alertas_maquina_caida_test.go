package mcp

// Guarda de UNA SOLA ALERTA POR EVENTO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// PROMETHEUS NO BORRA LA SERIE DE UNA MÁQUINA MUERTA: LA CONGELA
//
// Cuando un agente deja de latir, sus series no desaparecen — conservan el último valor hasta que
// caducan. Una comparación sobre un valor congelado se sostiene indefinidamente, así que el `for:`
// —que existe para filtrar los picos— la CONFIRMA en vez de descartarla.
//
// MEDIDO EN PRODUCCIÓN el 2026-09-02: `davantis-1` se cayó y el tablero mostró TRES alertas para un
// solo evento — `MaquinaCaida` (verdadera), más `CPUSostenida` y `MaquinaLateSinMedir` disparando
// sobre la última muestra que llegó antes de morir. Dos de las tres describían una máquina que no
// estaba, y `MaquinaLateSinMedir` afirmaba textualmente «late pero dejó de medir» de algo que no
// late.
//
// El patrón que lo arregla YA EXISTÍA en el repo desde S12 —`ServicioCaido` lo usa— y estaba
// aplicado en 3 de 15 reglas. Esta prueba es lo que evita que la número 16 vuelva a nacer sin él.
//
// FALLA CERRADA A PROPÓSITO: la lista de abajo es de EXCEPCIONES, no de reglas a vigilar. Una
// métrica de flota nueva entra vigilada por default y hay que declararla si no corresponde. Al
// revés —una lista de las que sí— una métrica nueva nacería sin guarda y en silencio, que es
// exactamente el modo de fallo que este archivo persigue.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"regexp"
	"strings"
	"testing"
)

// laGuarda es el sufijo que neutraliza una serie congelada. `unless` y no `and up == 1` porque
// conserva las etiquetas del lado izquierdo: con `and`, el mensaje perdería `{{ $labels.service }}`.
const laGuarda = `unless on(device) (musubi_fleet_device_up == 0)`

// sinGuardaPorDiseno son las alertas que NO deben llevarla, cada una por su razón.
var sinGuardaPorDiseno = map[string]string{
	"MaquinaCaida":         "ES la alerta de la máquina caída: agregarle la guarda la haría imposible de disparar",
	"FlotaSinTelemetria":   "es `absent(...)` sobre la propia serie de vida; no hay muestra congelada que leer",
	"MaquinaSinInventario": "se condiciona con `musubi_fleet_device_up{tier=\"A\"} == 1`, que ya exige que esté viva",
}

// metricasQueNoSeCongelan salen de la FILA del device y no de la muestra, así que no aplican.
var metricasQueNoSeCongelan = map[string]bool{
	"musubi_fleet_device_up":                true,
	"musubi_fleet_device_last_seen_seconds": true,
}

// TestNingunaAlertaDeMuestraDisparaSobreUnaMaquinaCaida.
//
// Sabotaje: quitarle el `unless on(device) (musubi_fleet_device_up == 0)` a cualquier regla de
// flota, o agregar una regla nueva sobre una métrica `musubi_fleet_*` sin él.
func TestNingunaAlertaDeMuestraDisparaSobreUnaMaquinaCaida(t *testing.T) {
	texto := leerDeploy(t, "musubi-alerts-flota.yml")

	// Se parte por `- alert:` y se mira cada bloque hasta el siguiente, sin las líneas de
	// comentario: un comentario que explica la guarda no puede hacer las veces de la guarda.
	bloques := strings.Split(texto, "- alert:")[1:]
	if len(bloques) < 10 {
		t.Fatalf("se detectaron %d alertas; el patrón se rompió y este test no probaría nada", len(bloques))
	}
	reMetrica := regexp.MustCompile(`\bmusubi_fleet_(?:device|service)_[a-z0-9_]+\b`)

	vistas, conGuarda := 0, 0
	for _, bloque := range bloques {
		nombre := strings.Fields(bloque)[0]
		// Sólo la expresión: la anotación `ausente_en` y los comentarios nombran métricas para
		// explicarse, y contarlas haría que una alerta pareciera vigilar lo que sólo menciona.
		expr := bloque
		if i := strings.Index(expr, "annotations:"); i > 0 {
			expr = expr[:i]
		}
		var utiles []string
		for _, l := range strings.Split(expr, "\n") {
			if s := strings.TrimSpace(l); s != "" && !strings.HasPrefix(s, "#") {
				utiles = append(utiles, s)
			}
		}
		expr = strings.Join(utiles, " ")

		tiene := strings.Contains(strings.Join(strings.Fields(expr), " "), laGuarda)

		// LAS EXCEPCIONES SE MIRAN PRIMERO, Y ESTE ORDEN ES UN ARREGLO, NO UN DETALLE.
		//
		// Estaba después del filtro de «¿lee una serie que se congela?», y `MaquinaCaida` lee
		// `musubi_fleet_device_up`, que está en la lista de las que NO se congelan. Así que la
		// rama de las excepciones era INALCANZABLE para las dos que más importan: se le podía
		// poner la guarda a `MaquinaCaida` —dejándola imposible de disparar, o sea la flota
		// entera sin aviso de máquina caída— y este test pasaba en verde. Lo cazó un sabotaje.
		if razon, exenta := sinGuardaPorDiseno[nombre]; exenta {
			if tiene {
				t.Errorf("%s NO debe llevar la guarda: %s.\n"+
					"Con ella queda imposible de disparar, y una alerta que no puede sonar se ve "+
					"exactamente igual que una que no tiene nada que decir.", nombre, razon)
			}
			continue
		}

		// ¿lee alguna métrica que se congela?
		congela := false
		for _, m := range reMetrica.FindAllString(expr, -1) {
			if !metricasQueNoSeCongelan[m] {
				congela = true
				break
			}
		}
		if !congela {
			continue
		}
		vistas++

		if !tiene {
			t.Errorf("%s lee una serie que se CONGELA cuando la máquina muere y no lleva `%s`.\n"+
				"Sin eso dispara sobre la última muestra que llegó antes de morir, y el tablero "+
				"muestra dos o tres alertas para un solo evento — con `MaquinaCaida` ya diciendo lo "+
				"que pasa. Si de verdad no corresponde, declarala en sinGuardaPorDiseno con el porqué.",
				nombre, laGuarda)
			continue
		}
		conGuarda++
	}

	if vistas < 8 {
		t.Fatalf("sólo %d alertas resultaron leer métricas de flota; el detector se rompió y el "+
			"test estaría pasando por mirar casi nada", vistas)
	}
	if conGuarda == 0 {
		t.Fatal("ninguna alerta lleva la guarda: o se borraron todas, o la constante `laGuarda` " +
			"dejó de coincidir con lo que dicen las reglas")
	}
}
