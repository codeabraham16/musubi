package mcp

// alertas_cobertura_sla_test.go — la hermana que faltaba, del lado de los SERVICIOS.
//
// ═════════════════════════════════════════════════════════════════════════════════════════════
// EL CABO, MEDIDO: LA ÚNICA ALERTA DE COBERTURA MIRABA LA FAMILIA DE MÁQUINAS
//
// `musubi-recording.yml` graba DOS resúmenes por proyecto de la cobertura del SLA:
//
//	musubi:project_up:cobertura30d            ← la leía `CoberturaDelSlaSeCayo`
//	musubi:project_service_up:cobertura30d    ← NO LA LEÍA NADIE
//
// O sea la forma dominante de defecto de este repo —una guarda presente en N-1 de N caminos—
// puesta justo encima de lo que se le factura a un cliente: el SLA de servicios podía perder su
// medición entera, en silencio, y el reporte seguía saliendo con un número plausible.
//
// `recording_cobertura_test.go` ya custodia que las coberturas EXISTAN y que sus `:sla30d` las
// USEN. Eso cierra el lado del dato. Esto cierra el lado del AVISO: que alguien se entere cuando
// la medición se pierde, que es lo único accionable (la cobertura baja no se arregla actuando —
// por eso no hay alerta de nivel, y sí de caída).
//
// QUÉ MIRA Y QUÉ NO: mira la EXPRESIÓN parseada del YAML, nunca el texto del archivo. Este repo
// produjo siete guardas satisfechas por un comentario, un mensaje de error o la línea vecina; acá
// una alerta que sólo NOMBRE la serie en su `nota:` no cuenta, porque las anotaciones no entran.
// ═════════════════════════════════════════════════════════════════════════════════════════════

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// archivosDeAlertas son los archivos que pueden contener una alerta, DESCUBIERTOS POR EL MISMO
// GLOB CON EL QUE PROMETHEUS LAS CARGA. No es una lista escrita a mano, y eso es el punto.
//
// Antes eran cuatro nombres a mano, con un «piso de alertas parseadas» de guardia. El piso no
// alcanzaba: sigue habiendo cientos de alertas en los cuatro archivos viejos, así que un QUINTO
// archivo —`deploy/musubi-alerts-sla.yml`, digamos— con una alerta inalcanzable adentro lo deja
// intacto y todo el barrido queda en verde. Medido: la alerta
// `delta(musubi:project_service_up:cobertura30d[6h]) < -5` + `for: 30d`, o sea los dos sabotajes
// juntos, puesta en un quinto archivo, pasaba VERDE por las tres guardas de alcanzabilidad.
//
// Y ese quinto archivo NO es hipotético: `deploy/prometheus/prometheus.yml` carga las reglas con
// `rule_files: [/etc/prometheus/rules/*.yml]` —un GLOB, y a propósito: con una ruta fija
// `preparar.sh` copiaba el archivo de flota y Prometheus lo ignoraba en silencio—. Y
// `verificar-despliegue.sh` también compara por `musubi-alerts*.yml`. O sea que el despliegue
// evalúa por glob y la guarda leía por lista: cualquier archivo nuevo nace desplegado y sin medir.
//
// La lección del repo aplicada tal cual: a una lista de nombres siempre le falta el próximo. Se
// dejó de mirar la lista y se mira la forma que decide, que es la misma que mira Prometheus.
var globDeArchivosDeAlertas = "musubi-alerts*.yml"

func archivosDeAlertasDelRepo(t *testing.T) []string {
	t.Helper()
	rutas, err := filepath.Glob(filepath.Join("..", "..", "deploy", globDeArchivosDeAlertas))
	if err != nil {
		t.Fatalf("no pude expandir %s: %v", globDeArchivosDeAlertas, err)
	}
	var out []string
	for _, r := range rutas {
		out = append(out, filepath.Base(r))
	}
	sort.Strings(out)
	// UN CERO ACÁ NO ES «NO HAY ALERTAS MAL». Es «no pude medir»: el glob no encontró nada porque
	// alguien movió deploy/, renombró los archivos o corrió la prueba desde otro directorio. Los
	// cuatro históricos son el piso; si un día se consolidan en menos, hay que bajarlo A MANO y
	// mirando, que es exactamente la revisión que este Fatalf existe para forzar.
	if len(out) < 4 {
		t.Fatalf("el glob deploy/%s encontró %d archivo(s) de alertas: %v.\n"+
			"  Eran cuatro por lo menos. Cero o pocos acá no significa «no hay alertas rotas», "+
			"significa QUE NO PUDE MEDIR — y un verde sobre eso no vale nada.",
			globDeArchivosDeAlertas, len(out), out)
	}
	return out
}

// grabacionesDelSla devuelve nombre -> expresión de cada `- record:` de musubi-recording.yml.
func grabacionesDelSla(t *testing.T) map[string]string {
	t.Helper()
	ruta := filepath.Join("..", "..", "deploy", "musubi-recording.yml")
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("falta %s: %v", ruta, err)
	}
	var a struct {
		Groups []struct {
			Rules []struct {
				Record string `yaml:"record"`
				Expr   string `yaml:"expr"`
			} `yaml:"rules"`
		} `yaml:"groups"`
	}
	if err := yaml.Unmarshal(crudo, &a); err != nil {
		t.Fatalf("%s no es YAML válido: %v", ruta, err)
	}
	out := map[string]string{}
	for _, g := range a.Groups {
		for _, r := range g.Rules {
			if r.Record != "" {
				out[r.Record] = r.Expr
			}
		}
	}
	return out
}

// alertasDeTodosLosArchivos devuelve nombre -> expresión, SIN anotaciones ni comentarios: el YAML
// ya los deja afuera, que es justamente por qué se parsea en vez de buscar texto.
func alertasDeTodosLosArchivos(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range archivosDeAlertasDelRepo(t) {
		a, _ := cargarReglas(t, f)
		for _, g := range a.Groups {
			for _, r := range g.Rules {
				if r.Alert != "" {
					out[r.Alert] = r.Expr
				}
			}
		}
	}
	return out
}

// nombraLaSerie dice si una expresión USA esa serie, y no una cuyo nombre la contenga.
func nombraLaSerie(expr, serie string) bool {
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_:])` + regexp.QuoteMeta(serie) + `($|[^A-Za-z0-9_:])`)
	return re.MatchString(expr)
}

// TODO RESUMEN POR PROYECTO DE LA COBERTURA DEL SLA TIENE UNA ALERTA QUE LO LEE.
//
// El resumen por proyecto es el número del reporte: `min by(project)`, el dato más flojo que lo
// sostiene. Si su medición se pierde y nadie avisa, lo que se factura se calcula sobre un hueco.
//
// Sabotaje que la hace fallar (verificado): borrar `CoberturaDelSlaDeServiciosSeCayo` de
// deploy/musubi-alerts-flota.yml. También falla si alguien la deja nombrando la serie sólo en la
// `nota:` —las anotaciones no entran acá— o si le cambia la expresión por una que mire el NIVEL
// en vez de la CAÍDA.
func TestCadaCoberturaDeSlaPorProyectoTieneUnaAlertaQueLaLee(t *testing.T) {
	grabadas := grabacionesDelSla(t)
	alertas := alertasDeTodosLosArchivos(t)

	// El universo se descubre del archivo, no se copia acá: una tercera familia por proyecto
	// —discos, backups— nace vigilada y hay que darle su alerta.
	reResumen := regexp.MustCompile(`^musubi:project[A-Za-z0-9_]*:cobertura30d$`)
	var resumenes []string
	for nombre := range grabadas {
		if reResumen.MatchString(nombre) {
			resumenes = append(resumenes, nombre)
		}
	}
	sort.Strings(resumenes)

	// CONTROL DE QUE MIRÓ ALGO. Un cero acá no es «todo bien»: es «no pude medir», y este archivo
	// entero existe por no distinguir esas dos cosas. Son dos desde A93 y el día que sean menos,
	// o el archivo cambió de forma o alguien borró una familia.
	if len(resumenes) < 2 {
		t.Fatalf("sólo encontré %d serie(s) `musubi:project*:cobertura30d` en musubi-recording.yml (%v).\n"+
			"  Eran dos —máquinas y servicios—: o se renombraron, o el parseo dejó de mirar. Un verde "+
			"acá no significaría nada.", len(resumenes), resumenes)
	}
	if len(alertas) < 10 {
		t.Fatalf("sólo se parsearon %d alertas de %v; el barrido dejó de mirar", len(alertas), archivosDeAlertasDelRepo(t))
	}

	for _, serie := range resumenes {
		var lectoras []string
		for nombre, expr := range alertas {
			if nombraLaSerie(expr, serie) {
				lectoras = append(lectoras, nombre)
			}
		}
		sort.Strings(lectoras)
		if len(lectoras) == 0 {
			t.Errorf("NADIE LEE %s.\n"+
				"  Es el resumen por proyecto que se le factura a un cliente y su medición puede "+
				"perderse entera en silencio.\n"+
				"  La hermana es `CoberturaDelSlaSeCayo` en deploy/musubi-alerts-flota.yml: calcá su "+
				"forma —`delta(<serie>[6h]) < -0.05`, `for: 30m`— y no inventes otra.\n"+
				"  Y OJO CON LAS UNIDADES al calcarla: la cobertura es un RATIO en [0,1] («5 puntos» "+
				"son 0,05) y el `for` tiene que caber en la ventana; las dos cosas las mide "+
				"TestElUmbralDeCadaAlertaSobreUnaSerieGrabadaEsAlcanzable.", serie)
			continue
		}
		for _, nombre := range lectoras {
			verificarFormaDeLaAlertaDeCobertura(t, nombre, alertas[nombre], serie)
		}
	}
}

// verificarFormaDeLaAlertaDeCobertura exige que la alerta mire LA CAÍDA y que lo haga sobre la
// serie grabada. Las tres comprobaciones son trampas medidas de este repo, no gusto.
func verificarFormaDeLaAlertaDeCobertura(t *testing.T, alerta, expr, serie string) {
	t.Helper()
	plano := strings.Join(strings.Fields(expr), " ")

	// (a) LA CAÍDA, NO EL NIVEL. Una alerta de «cobertura baja» no se puede apagar arreglando algo
	// —la única salida es esperar— y una alarma así enseña a ignorar el canal (A79, trece
	// `MaquinaCaida`). Además el `min` de servicios lo domina un servicio ocioso por diseño (A70),
	// así que su nivel es bajo y no informa; su caída sí.
	// OJO CON ESTA LISTA: acepta `deriv` A PROPÓSITO, y no es un descuido — un `deriv` con el
	// umbral bien escalado (por segundo) es una alerta legítima. Lo que NO puede hacer esta lista
	// es decir si el umbral está en las unidades de la función: `deriv(...) < -0.05` pasó por acá
	// en verde y no podía disparar nunca. Esa pregunta —la de las MAGNITUDES— no se contesta con
	// una lista de formas y por eso no se contesta acá: la contesta
	// `TestElUmbralDeCadaAlertaSobreUnaSerieGrabadaEsAlcanzable`
	// (internal/mcp/alertas_umbral_alcanzable_test.go), que deriva el rango de la serie del propio
	// musubi-recording.yml. Esto de acá sólo distingue VARIACIÓN de NIVEL.
	reCaida := regexp.MustCompile(`(?:delta|idelta|deriv|rate|irate|increase)\(\s*` + regexp.QuoteMeta(serie) + `\s*\[`)
	if !reCaida.MatchString(plano) {
		t.Errorf("%s nombra %s pero no mira su VARIACIÓN en el tiempo: %q\n"+
			"  Mientras el TSDB acumula historia la cobertura sólo sube, así que lo accionable es "+
			"la caída. El nivel bajo no se arregla actuando.", alerta, serie, plano)
	}
	if !regexp.MustCompile(`<\s*-\s*0?\.?\d`).MatchString(plano) {
		t.Errorf("%s no compara contra un umbral NEGATIVO: %q\n"+
			"  Sin eso no está detectando una pérdida de cobertura.", alerta, plano)
	}

	// (b) LA SERIE GRABADA, NUNCA LA CRUDA. `musubi_fleet_*` llega DUPLICADA por `instance` —el
	// scrape y el push— y un `min` sobre eso se queda con el camino MUERTO: así el «peor equipo»
	// del reporte marcó 10,2 % siendo 84,7 %. Y se CONGELA: un servicio que dejó de reportar
	// conserva su último estado, así que sobre la tabla cruda no se distingue «está fallando» de
	// «dejó de reportar». Las dos cosas ya las resuelve `:norm` con `max by(...)` y la guarda de
	// frescura, y estas series descienden de ella.
	if cruda := regexp.MustCompile(`\bmusubi_fleet_[a-z0-9_]+\b`).FindString(plano); cruda != "" {
		t.Errorf("%s lee la métrica cruda %s en vez de quedarse con la serie grabada: %q\n"+
			"  La cruda llega dos veces por máquina (scrape + push) y se congela cuando el agente "+
			"muere. `musubi:*:norm` colapsa `instance` con `max by(...)` y lleva la guarda de "+
			"frescura; saltearla trae de vuelta el fantasma.", alerta, cruda, plano)
	}

	// (c) Y NO CON `timestamp(last_over_time(...))`, que fue el intento que no funciona: devuelve
	// la hora de EVALUACIÓN, no la de la última muestra, así que una serie MUERTA contesta «hace
	// 0 min» y la guarda queda verde para siempre.
	if strings.Contains(strings.ReplaceAll(plano, " ", ""), "timestamp(last_over_time(") {
		t.Errorf("%s pregunta frescura con `timestamp(last_over_time(...))`: %q\n"+
			"  Eso devuelve la hora de evaluación, no la de la última muestra: una serie muerta "+
			"contesta «hace 0 min» y esta alerta no puede disparar nunca.", alerta, plano)
	}
}

// Y LA CONTRACARA: LA SERIE QUE LA ALERTA LEE TIENE QUE EXISTIR EN EL ARCHIVO DE GRABACIÓN.
//
// Una alerta sobre una serie que nadie graba no falla, no se queja y no dispara jamás: es una
// alarma apagada que se ve idéntica a una que no tiene nada que decir. Pasó en este repo con
// `CadenaDeAlertasFallando` sobre un job que no estaba desplegado (A73).
//
// Sabotaje que la hace fallar: renombrar `musubi:project_service_up:cobertura30d` en
// musubi-recording.yml y no tocar la alerta.
func TestNingunaAlertaLeeUnaSerieDeSlaQueNadieGraba(t *testing.T) {
	grabadas := grabacionesDelSla(t)
	alertas := alertasDeTodosLosArchivos(t)

	reSerie := regexp.MustCompile(`\bmusubi:[A-Za-z0-9_:]+\b`)
	vistas := 0
	for nombre, expr := range alertas {
		for _, s := range reSerie.FindAllString(expr, -1) {
			vistas++
			if _, ok := grabadas[s]; !ok {
				t.Errorf("%s lee %s y musubi-recording.yml no la graba.\n"+
					"  Una alerta sobre una serie inexistente no dispara nunca y se ve igual que "+
					"una que no tiene nada que decir.", nombre, s)
			}
		}
	}
	// Un cero acá significaría que el detector dejó de reconocer las series `musubi:` —o que
	// nadie las lee, que es exactamente el defecto que este archivo cierra—. En los dos casos es
	// «no pude medir», no «está bien».
	if vistas < 2 {
		t.Fatalf("sólo %d referencia(s) a una serie `musubi:*` en todas las alertas.\n"+
			"  Eran dos desde que existe `CoberturaDelSlaDeServiciosSeCayo`: o alguien borró una "+
			"alerta de cobertura, o este detector dejó de mirar.", vistas)
	}
}
