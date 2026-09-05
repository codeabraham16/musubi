package mcp

import (
	"strings"
	"testing"
)

// ═════════════════════════════════════════════════════════════════════════════════════════════
// LA VENTANA DE MANTENIMIENTO LA MIRABAN 18 DE 23 REGLAS, Y FALTABA EN LA QUE MÁS IMPORTA (A97)
//
// `musubi_fleet_maintenance` se describe a sí misma con ESTE caso como su razón de ser: «Sin esto,
// un reinicio planificado dispara servicio_caido». Un reinicio planificado es el motivo más común
// para declarar una ventana — y `MaquinaCaida` no miraba la serie, así que disparaba a los cinco
// minutos igual. La ventana no hacía lo que su propio texto promete, justo en su caso de portada.
//
// Medido el 2026-09-05 sobre `musubi-alerts-flota.yml`: 18 reglas llevaban
// `unless on(project, device) (musubi_fleet_device_maintenance == 1)` y 5 no. Y la vecina de la
// misma familia —`MaquinaLateSinMedir`— sí la lleva. O sea que la intención no estaba en duda: se
// había aplicado de a pedazos. Es la forma dominante de defecto de este repo: una guarda que existe
// en N-1 de N caminos.
//
// ESTA PRUEBA ES LA GUARDA DE LA GUARDA. Sin ella, la regla 26 se agrega sin la línea y nadie se
// entera hasta que una ventana de mantenimiento no calla algo — y eso se descubre de madrugada.
//
// LAS EXCEPCIONES SE ENUMERAN CON SU MOTIVO, NO SE PERMITEN POR OMISIÓN: agregar una alerta nueva
// obliga a poner la guarda o a escribir acá por qué no corresponde. Un allowlist sin razones se
// llena solo.
var alertasSinGuardaDeMantenimiento = map[string]string{
	// Es GLOBAL: `absent(musubi_fleet_device_up)` no tiene etiqueta `device`, así que un
	// `unless on(project, device)` no podría emparejar con nada. Y el sentido tampoco: que UNA
	// máquina esté en mantenimiento no explica que la flota entera deje de reportar.
	"FlotaSinTelemetria": "es global y sin etiqueta device: el unless no tendría con qué emparejar",

	// Las tres cuentan el MOTOR de políticas, no el estado de una máquina. Y no hace falta la
	// guarda porque la ventana ya corta la causa un nivel antes: mientras vale 1, las políticas de
	// auto-heal NO ACTÚAN sobre esa máquina, así que el contador no se incrementa y no hay qué
	// callar. Poner la guarda acá sugeriría que sin ella habría ruido, y no lo hay.
	"PoliticaQueNoCura":          "cuenta acciones de política, y la ventana ya impide que la política actúe",
	"PoliticaSinPermiso":         "cuenta rechazos del motor de políticas, no el estado de una máquina",
	"AllowlistDeFlotaRechazando": "cuenta rechazos de la allowlist de tools, del lado del cerebro",
	// Misma familia: cuenta rechazos por consentimiento del motor de políticas. Y la ventana ya
	// corta la causa DOS niveles antes — durante el mantenimiento la política no se evalúa
	// (`contarPolitica(pol, "mantenimiento")`), así que el contador de consentimiento no se mueve.
	"PoliticaFrenadaPorConsentimiento": "cuenta rechazos por consentimiento del motor de políticas, no el estado de una máquina",
	// Mira la cobertura del SLA por PROYECTO, no por máquina: una ventana de mantenimiento en un
	// equipo no explica que el TSDB haya perdido datos, y suprimirla por eso taparía justo el caso
	// que la alerta existe para ver.
	"CoberturaDelSlaSeCayo": "mira la cobertura del SLA por proyecto, no el estado de una máquina",

	// Las de custodia miran `prometheus_rule_group_rules`: hablan del DESPLIEGUE de los archivos de
	// reglas, no de ninguna máquina. Una ventana de mantenimiento sobre un equipo no tiene nada que
	// decir sobre si Prometheus cargó el archivo que el repo declara.
	"ReglasDelCerebroSinDesplegar": "mira el conteo de reglas cargadas, no una máquina",
	"ReglasDelSlaSinDesplegar":     "mira el conteo de reglas cargadas, no una máquina",
}

func TestTodaAlertaDeUnaMaquinaRespetaLaVentanaDeMantenimiento(t *testing.T) {
	flota, _ := cargarReglas(t, "musubi-alerts-flota.yml")

	const guarda = "musubi_fleet_device_maintenance"
	vistas := 0
	conGuarda := 0
	for _, g := range flota.Groups {
		for _, r := range g.Rules {
			if r.Alert == "" {
				continue
			}
			vistas++
			tiene := strings.Contains(r.Expr, guarda)
			motivo, exenta := alertasSinGuardaDeMantenimiento[r.Alert]
			switch {
			case tiene && exenta:
				t.Errorf("%s tiene la guarda de mantenimiento Y está en la lista de exentas (%q).\n"+
					"Una de las dos cosas sobra: si la guarda corresponde, sacala de la lista; si no, "+
					"sacala de la expresión. Una excepción que ya no lo es enseña a no leer la lista.",
					r.Alert, motivo)
			case tiene:
				conGuarda++
			case exenta:
				// Correcto y explicado.
			default:
				t.Errorf("%s NO mira `%s` y no está en la lista de excepciones.\n"+
					"  Si habla del estado de UNA MÁQUINA, agregale:\n"+
					"      unless on(project, device) (%s == 1)\n"+
					"  Si no —porque es global, o porque cuenta algo del cerebro— sumala a\n"+
					"  `alertasSinGuardaDeMantenimiento` CON SU MOTIVO.\n"+
					"  Lo que no se puede es dejarla afuera en silencio: una ventana de mantenimiento\n"+
					"  que no calla lo que promete callar se descubre de madrugada, y hace que la\n"+
					"  próxima ventana no se declare.", r.Alert, guarda, guarda)
			}
		}
	}

	// CONTROL DE QUE MIRÓ ALGO. Si el YAML cambiara de forma y `cargarReglas` devolviera cero
	// reglas, todo lo de arriba pasaría en verde sin haber comprobado nada — que es el modo de falla
	// que este archivo entero persigue.
	if vistas < 20 {
		t.Fatalf("se reconocieron %d alertas en musubi-alerts-flota.yml y son más de veinte: cambió la "+
			"forma del archivo y esta guarda dejó de mirar", vistas)
	}
	if conGuarda < 15 {
		t.Fatalf("sólo %d alertas llevan la guarda de mantenimiento, y eran 18 cuando se escribió esto: "+
			"si de verdad bajaron, alguien la está sacando", conGuarda)
	}

	// Y al revés: una exención sobre una alerta que ya no existe queda diciendo que cuida algo, y la
	// próxima persona la va a leer como vigente.
	for nombre := range alertasSinGuardaDeMantenimiento {
		if _, ok := exprDeAlerta(flota, nombre); !ok {
			t.Errorf("`alertasSinGuardaDeMantenimiento` exime a %s y esa alerta ya no está en el "+
				"archivo: sacá la entrada", nombre)
		}
	}
}

// LA GUARDA TIENE QUE EMPAREJAR POR (project, device) Y NO A SECAS.
//
// `unless (musubi_fleet_device_maintenance == 1)` sin el `on(...)` no es una versión más floja: es
// otra cosa. Sin etiquetas comunes declaradas, un `unless` entre dos vectores empareja por el
// conjunto COMPLETO de etiquetas, y como los dos lados traen `tier`, `os` y demás, en la práctica
// deja de emparejar o empareja de más. Una máquina en mantenimiento podría callar a las OTRAS, o a
// ninguna, y en los dos casos el síntoma es una alerta que se comporta raro y nadie sabe por qué.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA PRIMERA VERSIÓN DE ESTA PRUEBA NO SERVÍA, Y VALE MÁS DEJARLO ESCRITO QUE BORRARLO
//
// Buscaba `on(project, device)` en TODO el texto anterior a la métrica. En `MaquinaCaida` ese texto
// ya contiene un `on(project, device)` —el de la cláusula de `musubi_fleet_net_up`—, así que la
// prueba pasaba en verde con la guarda escrita mal. Lo destapó correr el sabotaje: sacarle el `on`
// y ver `ok`. La prueba afirmaba cubrir algo que no cubría, que es exactamente el defecto que este
// archivo entero persigue — y esta vez lo escribí yo.
//
// Ahora se parte la expresión por sus operadores de conjunto y se exige que el operando que trae la
// métrica EMPIECE con `on(project, device)`, que es donde PromQL lo pone.
//
// Sabotaje verificado que la pone en rojo: sacarle el `on(project, device)` a cualquiera de las
// guardas (probado sobre MaquinaCaida, que es donde la primera versión fallaba).
func TestLaGuardaDeMantenimientoEmparejaPorMaquinaYNoAlAzar(t *testing.T) {
	flota, _ := cargarReglas(t, "musubi-alerts-flota.yml")

	const metrica = "musubi_fleet_device_maintenance"
	revisadas := 0
	for _, g := range flota.Groups {
		for _, r := range g.Rules {
			if r.Alert == "" || !strings.Contains(r.Expr, metrica) {
				continue
			}
			// El YAML parte las expresiones en varias líneas; se normaliza el espacio para poder
			// razonar sobre el texto que Prometheus va a ver.
			expr := strings.Join(strings.Fields(r.Expr), " ")

			// Se corta por los operadores de conjunto de PromQL. El operando que sigue a cada uno es
			// donde vive el `on(...)`, así que el fragmento que trae la métrica tiene que empezar ahí.
			frag := ""
			for _, op := range []string{" unless ", " and ", " or "} {
				for _, cand := range strings.Split(expr, op) {
					if strings.Contains(cand, metrica) && (frag == "" || len(cand) < len(frag)) {
						frag = cand
					}
				}
			}
			if frag == "" {
				t.Errorf("%s nombra %s y no pude aislar su operando: %s", r.Alert, metrica, expr)
				continue
			}
			revisadas++
			if !strings.HasPrefix(strings.TrimSpace(frag), "on(project, device)") {
				t.Errorf("%s usa la guarda de mantenimiento sin `on(project, device)` en su operando:\n"+
					"  operando: %s\n"+
					"  completa: %s\n"+
					"Sin las etiquetas declaradas, el `unless` empareja por el set completo —que incluye "+
					"tier y os— y una máquina en mantenimiento callaría a las otras o a ninguna.",
					r.Alert, strings.TrimSpace(frag), expr)
			}
		}
	}
	// CONTROL DE QUE MIRÓ ALGO: sin esto, un cambio de forma del YAML deja la prueba en verde sin
	// haber revisado una sola guarda — que es la falla que su primera versión tuvo de verdad.
	if revisadas < 15 {
		t.Fatalf("sólo se revisaron %d guardas de mantenimiento y son al menos quince: esta prueba "+
			"dejó de mirar", revisadas)
	}
}
