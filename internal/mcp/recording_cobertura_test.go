package mcp

// recording_cobertura_test.go es la hermana de alertas_maquina_caida_test.go, un plano más allá.
//
// Aquélla custodia que ninguna ALERTA dispare sobre una máquina caída. Ésta custodia que ninguna
// REGLA DE GRABACIÓN promedie una serie congelable sin decir sobre qué la promedió.

import (
	"regexp"
	"strings"
	"testing"
)

// laGuardaDeFrescura es la misma que llevan las 24 alertas de flota.
const laGuardaDeFrescura = "unless on(project, device) (musubi_fleet_device_up == 0)"

// seriesQueSeCongelan son las familias que SIGUEN PUBLICÁNDOSE con hora fresca cuando el agente
// que las alimenta se muere. No es una lista de gustos: es lo que el exportador hace, y su propio
// HELP lo dice («NO dice si ese reporte es reciente»).
//
// `musubi_fleet_device_up` NO está: ésa es justamente la que vale 0 cuando la máquina cae, así que
// es la señal y no la congelada. Ponerla acá haría imposible escribir la propia guarda.
var seriesQueSeCongelan = []string{
	"musubi_fleet_service_up",
	"musubi_fleet_device_disk_available_bytes",
	"musubi_fleet_device_memory_used_bytes",
	"musubi_fleet_device_temperature_celsius",
}

// UNA REGLA QUE PROMEDIA UNA SERIE CONGELABLE TIENE QUE LLEVAR LA GUARDA **Y** TENER COBERTURA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL DEFECTO QUE ESTO PERSIGUE, MEDIDO
//
// `musubi:service_up:avg30d` leía `musubi_fleet_service_up` sin la guarda de frescura. El
// 2026-09-04 `davantis-1` estuvo caída de 15:51 a 21:21 UTC —`musubi_fleet_device_up` = 0 las seis
// horas— y sus 64 servicios publicaron `1` durante TODA la ventana: el SLA contó seis horas de
// máquina inalcanzable como 100 % de disponibilidad de servicios.
//
// Evaluando la expresión corregida a las 18:00 de ese día: 186 series sin guarda, 122 con guarda.
// La diferencia son exactamente las 64 de la máquina muerta.
//
// Y LAS DOS MITADES SON UNA SOLA REPARACIÓN. Con la guarda sola, durante la caída la serie no vale
// 0: DESAPARECE. `avg_over_time` ignora los huecos, así que un `avg30d` de 1.0 sobre 62 minutos
// medidos se dibuja idéntico a uno sobre 43.200 — el defecto cambiado de lugar, no cerrado. Por
// eso este test exige LAS DOS: la guarda en la serie normalizada y una `:cobertura30d` declarada.
//
// Sabotaje que lo hace fallar: sacar el `unless` de musubi:service_up:norm, o borrar la regla
// musubi:service_up:cobertura30d.
func TestNingunaReglaPromediaUnaSerieCongelableSinDecirloDeQue(t *testing.T) {
	texto := leerDeploy(t, "musubi-recording.yml")

	// Se parte por `- record:` y se mira cada bloque, SIN las líneas de comentario: un comentario
	// que explica la guarda no puede hacer las veces de la guarda.
	bloques := strings.Split(texto, "- record:")[1:]
	if len(bloques) < 8 {
		t.Fatalf("se detectaron %d reglas; el patrón se rompió y este test no probaría nada", len(bloques))
	}

	reglas := map[string]string{} // nombre -> expresión sin comentarios
	for _, b := range bloques {
		nombre := strings.Fields(b)[0]
		var utiles []string
		for _, l := range strings.Split(b, "\n") {
			if s := strings.TrimSpace(l); s != "" && !strings.HasPrefix(s, "#") {
				utiles = append(utiles, s)
			}
		}
		reglas[nombre] = strings.Join(strings.Fields(strings.Join(utiles, " ")), " ")
	}

	// ── (a) toda serie normalizada que lee una congelable lleva la guarda ────────────────────
	guardaNormalizada := strings.Join(strings.Fields(laGuardaDeFrescura), " ")
	normalizadasConCongelable := 0
	for nombre, expr := range reglas {
		if !strings.HasSuffix(nombre, ":norm") {
			continue
		}
		var congelable string
		for _, s := range seriesQueSeCongelan {
			if regexp.MustCompile(`\b` + s + `\b`).MatchString(expr) {
				congelable = s
				break
			}
		}
		if congelable == "" {
			continue
		}
		normalizadasConCongelable++
		if !strings.Contains(expr, guardaNormalizada) {
			t.Errorf("%s lee %s —que se REPUBLICA con hora fresca cuando el agente muere— y NO lleva la guarda de frescura.\n"+
				"  Sin ella, las horas en que la máquina estuvo inalcanzable entran al promedio como disponibilidad MEDIDA.\n"+
				"  Falta: %s", nombre, congelable, laGuardaDeFrescura)
		}
	}
	if normalizadasConCongelable == 0 {
		t.Fatal("ninguna regla `:norm` lee una serie congelable: o se renombraron las reglas o se cambió la lista, y este test dejó de mirar lo que dice mirar")
	}

	// ── (b) y toda familia que promedia sobre una `:norm` tiene su cobertura ─────────────────
	//
	// La guarda sola deja un HUECO durante la caída, y `avg_over_time` ignora los huecos. Sin un
	// número que diga sobre cuánto se promedió, «100 % disponible» y «casi no medimos» se dibujan
	// igual — que es exactamente lo que la guarda vino a impedir.
	promedios := 0
	for nombre, expr := range reglas {
		if !strings.Contains(expr, "avg_over_time(") || !strings.Contains(expr, ":norm[") {
			continue
		}
		promedios++
		// La familia es el prefijo antes del último `:`. `musubi:service_up:avg30d` -> `musubi:service_up`.
		i := strings.LastIndex(nombre, ":")
		if i < 0 {
			continue
		}
		familia := nombre[:i]
		if _, hay := reglas[familia+":cobertura30d"]; !hay {
			t.Errorf("%s promedia sobre una serie normalizada y NO existe %s:cobertura30d.\n"+
				"  `avg_over_time` ignora los huecos que deja la guarda, así que un 1.0 sobre una hora medida\n"+
				"  se dibuja igual que uno sobre treinta días. Las dos mitades son una sola reparación.",
				nombre, familia)
		}
	}
	if promedios < 2 {
		t.Fatalf("sólo %d reglas promedian sobre una `:norm`; el patrón se rompió", promedios)
	}
}
