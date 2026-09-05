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

	// ── (c) y toda familia que promedia tiene su `:sla30d` ACOTADO POR SU COBERTURA ───────────
	//
	// A93, segunda mitad. (b) exige que la cobertura EXISTA; esto exige que alguien la USE. Medido
	// el 2026-09-05: las cuatro `:cobertura30d` existían y no las leía nada —ni una alerta, ni un
	// panel, ni un guion— mientras los `avg30d` se publicaban rotulados «a 30 días» sobre el 1,65 %
	// de la ventana. Una cobertura que nadie consume es la mitad de la reparación otra vez.
	//
	// EL MECANISMO TIENE QUE SOBREVIVIR A QUIEN CONSTRUYA EL REPORTE, que todavía no existe: va a
	// escribir `avg30d`, porque es el nombre que suena a lo que busca. Un número al que hay que
	// acordarse de pedirle su cobertura al lado se publica solo. Por eso la defensibilidad viaja EN
	// LA SERIE: `:sla30d` desaparece cuando la ventana no alcanza, y un panel dibuja un hueco.
	for nombre, expr := range reglas {
		if !strings.Contains(expr, "avg_over_time(") || !strings.Contains(expr, ":norm[") {
			continue
		}
		familia := nombre[:strings.LastIndex(nombre, ":")]
		sla, hay := reglas[familia+":sla30d"]
		if !hay {
			t.Errorf("%s promedia sobre una serie normalizada y NO existe %s:sla30d.\n"+
				"  Su `:cobertura30d` existe y nadie la lee, que es la mitad de la reparación otra vez.\n"+
				"  Quien construya el reporte va a escribir `avg30d` porque es el nombre que suena a lo\n"+
				"  que busca; la defensibilidad tiene que viajar en la serie, no en la memoria de nadie.",
				nombre, familia)
			continue
		}
		// Y que de verdad se ACOTE con la cobertura de SU familia: un `:sla30d` que sea una copia
		// del promedio es peor que no tenerlo, porque su nombre promete lo que no cumple.
		if !strings.Contains(sla, familia+":cobertura30d") {
			t.Errorf("%s:sla30d no nombra %s:cobertura30d, así que no está acotado por nada.\n"+
				"  Su nombre promete «el número que se puede defender» y sería una copia de %s.",
				familia, familia, nombre)
		}
	}
}

// LOS TRES UMBRALES DE COBERTURA SON EL MISMO NÚMERO, Y EN YAML NO HAY CONSTANTES.
//
// `:sla30d` se define una vez por familia y cada una lleva el `>= 0.95` escrito a mano. Tres
// números que deberían ser el mismo es exactamente cómo uno se queda viejo: alguien afloja el del
// SLA de servicios «para que el reporte salga» y los otros dos siguen diciendo otra cosa, así que
// el mismo mes tiene dos definiciones de defendible. Esta guarda es el sustituto de la constante
// que el formato no tiene.
//
// Sabotaje: cambiar el umbral de UNA de las tres reglas.
func TestTodasLasSeriesDeSlaUsanElMismoUmbralDeCobertura(t *testing.T) {
	texto := leerDeploy(t, "musubi-recording.yml")
	reUmbral := regexp.MustCompile(`:cobertura30d\s*>=\s*([0-9.]+)`)

	umbrales := map[string][]string{} // valor -> qué reglas lo usan
	for _, b := range strings.Split(texto, "- record:")[1:] {
		nombre := strings.Fields(b)[0]
		if !strings.HasSuffix(nombre, ":sla30d") {
			continue
		}
		// Sin comentarios: un umbral citado en prosa no es el que se evalúa.
		var utiles []string
		for _, l := range strings.Split(b, "\n") {
			if x := strings.TrimSpace(l); x != "" && !strings.HasPrefix(x, "#") {
				utiles = append(utiles, x)
			}
		}
		expr := strings.Join(utiles, " ")
		m := reUmbral.FindStringSubmatch(expr)
		if m == nil {
			t.Errorf("%s no compara su cobertura contra ningún umbral: %s", nombre, expr)
			continue
		}
		umbrales[m[1]] = append(umbrales[m[1]], nombre)
	}

	// CONTROL DE QUE MIRÓ ALGO: si las reglas se renombraran, el mapa quedaría vacío y esto pasaría
	// en verde sin haber comparado un solo umbral.
	total := 0
	for _, rs := range umbrales {
		total += len(rs)
	}
	if total < 3 {
		t.Fatalf("se encontraron %d reglas `:sla30d` con umbral y son al menos tres: cambió la forma "+
			"del archivo y esta guarda dejó de mirar", total)
	}
	if len(umbrales) > 1 {
		t.Errorf("hay %d umbrales de cobertura distintos y tiene que ser UNO: %v\n"+
			"  Dos definiciones de «defendible» en el mismo mes es lo que esta guarda existe para "+
			"impedir, porque en YAML no hay constantes.", len(umbrales), umbrales)
	}
}
