package mcp

// fleet_prometheus_servicios.go exporta QUÉ CORRE ADENTRO de cada máquina (A43).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ NO ENTRA EN LA TABLA DE seriesDeFlota
//
// Las series de máquina cierran sobre `(Device, Muestra)`: una fila por máquina. Un servicio es
// otra cardinalidad —N por máquina— así que necesita su propia tabla y su propio bucle. Meterlo
// a la fuerza en la otra obligaría a que cada cierre supiera de servicios, que es justo el tipo
// de acoplamiento que después nadie deshace.
//
// LA COMPUERTA NO SE VUELVE A EVALUAR ACÁ. Se recibe la lista de máquinas que YA pasó por
// `devicesVisiblesParaMetricas`, y los servicios se buscan sólo para ésas. Un segundo recorrido
// de la flota sería un segundo lugar donde olvidarse la compuerta, y ese olvido no se ve —exporta
// de más, calladito— hasta que alguien audita.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA CARDINALIDAD, QUE ES LO QUE MATA A CUALQUIER SISTEMA DE MÉTRICAS
//
// Tres series por servicio. En el servidor real son 54 servicios = 162 series; a 40 máquinas con
// 50 servicios cada una, 6.000. Es mucho más que las ~900 de hoy y sigue siendo cómodo, pero
// SÓLO porque los nombres de servicio son ESTABLES: una unit se llama igual toda su vida. El día
// que alguien quiera exportar algo cuyo nombre rota —un pid, un id de contenedor, una ruta— esto
// se va a 80.000 y no vuelve, porque una serie que deja de recibir datos no se borra.

import (
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// serviciosPorExportar es el techo de servicios que salen a métricas POR PROYECTO y por scrape.
// No es el mismo techo que `fleet.ServiciosPorLatido` (que acota UN latido de UNA máquina): éste
// protege a Prometheus de una flota entera.
//
// Cuando se corta, SE DICE. Un recorte silencioso deja series que desaparecen sin que nadie sepa
// por qué, y eso se lee como «ese servicio ya no existe» — que es una afirmación, no un silencio.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// ERA UN TECHO TOTAL Y PASÓ A SER POR PROYECTO (Ola 0 del plan empresa, 2026-09-03)
//
// El total era lo primero que iba a romper, y no de a poco: al llegar a 2000 servicios entre
// TODA la flota, los que quedaban afuera dejaban de tener serie, y `ServicioCaido` no puede
// alertar sobre una serie que no existe. O sea: cobertura que desaparece en silencio, y
// justamente en el momento en que la flota crece. Medido el 2026-09-03: dos máquinas reales ya
// reportan 121 servicios entre las dos, así que a ~35 máquinas con 60 servicios se cruzaba.
//
// Por proyecto, un tenant grande no puede dejar ciego a otro — que con un techo compartido era
// exactamente lo que pasaba, y sin que ninguno de los dos se enterara.
const serviciosPorExportar = 2000

// servicioExportable ata un servicio a la máquina donde corre. Los dos hacen falta para las
// etiquetas: el nombre del servicio solo no identifica nada en una flota.
type servicioExportable struct {
	sv fleet.Servicio
	d  fleet.Device
}

// serviciosVisiblesParaMetricas devuelve los servicios de las máquinas YA compuertadas.
func serviciosVisiblesParaMetricas(engine memory.StorageBackend, vistos []fleet.Device) (out []servicioExportable, truncado bool) {
	contadoPorProyecto := map[string]int{}
	// Se agrupa por proyecto para no pedirle a la base una vez por máquina: `ListarServicios`
	// trae los del proyecto entero y acá se filtra por las máquinas que pasaron la compuerta.
	porProyecto := map[string][]fleet.Device{}
	for _, d := range vistos {
		porProyecto[d.ProjectID] = append(porProyecto[d.ProjectID], d)
	}
	for proy, devices := range porProyecto {
		porID := make(map[string]fleet.Device, len(devices))
		for _, d := range devices {
			porID[d.ID] = d
		}
		servicios, err := engine.ListarServicios(proy, "", false)
		if err != nil {
			continue // un proyecto ilegible no puede tumbar el scrape entero
		}
		for _, sv := range servicios {
			d, ok := porID[sv.DeviceID]
			if !ok {
				// El servicio es de una máquina que esta credencial NO ve. Se saltea en silencio
				// y a propósito: contarlo o nombrarlo sería decir cuántas máquinas hay del otro
				// lado de la compuerta.
				continue
			}
			// EL TECHO SE CUENTA POR PROYECTO, no sobre `out`: con un contador global el
			// primer tenant en ser barrido se comía el cupo y los demás quedaban sin series.
			if contadoPorProyecto[d.ProjectID] >= serviciosPorExportar {
				truncado = true
				continue
			}
			contadoPorProyecto[d.ProjectID]++
			out = append(out, servicioExportable{sv: sv, d: d})
		}
	}
	// `truncado` y no `false`: con el corte viejo la función salía por un `return out, true`
	// temprano, así que el final podía devolver false sin mentir. Ahora el recorte NO corta el
	// recorrido —sigue para contar los otros proyectos—, y un `false` acá borraría el hecho.
	return out, truncado
}

// serieDeServicio es la misma forma que serieDeFlota, para un servicio.
type serieDeServicio struct {
	Nombre string
	Ayuda  string
	Unidad string
	// Valor devuelve (valor, hay). `hay=false` OMITE la serie — no emite un 0. Es la misma regla
	// que rige toda la telemetría de este repo: lo que no se sabe no se inventa.
	Valor func(sv fleet.Servicio, ahora time.Time) (float64, bool)
}

// seriesDeServicio son las tres del ESTADO más las cuatro del RENDIMIENTO.
//
// `up` dice el ESTADO y `last_report_seconds` dice si esa afirmación es reciente. Combinarlas en
// una sola —«up sólo si además está fresco»— parece más simple y esconde el porqué: no se
// distinguiría un servicio caído de uno que dejó de reportar, que se arreglan distinto.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LAS DEL RENDIMIENTO (fase 4) SALEN JUNTO A LAS OTRAS, Y NO EN UN EXPORTADOR APARTE
//
// La tool `musubi_fleet_services` muestra el rendimiento; si el exportador no lo emitiera, las dos
// superficies que leen la MISMA fila dirían cosas distintas — que es exactamente el bug que A39
// cerró un nivel más arriba, con el gráfico mostrando un hueco y la tabla un número.
//
// EL DESGLOSE NO SE EXPORTA, y es la única decisión que hace falta explicar. Sus claves las elige
// quien reporta, con el vocabulario de SU dominio (`no_puedo`, `vacio`), y una etiqueta cuyos
// valores decide un tercero es cardinalidad sin techo: el dominio ya la acota a
// fleet.DesgloseMax por servicio, pero por FLOTA no hay tope. El desglose se mira en la tool y en
// el panel, donde una clave nueva cuesta una columna y no una serie más por máquina.
//
// `atendidas` y `fallidas` SÍ salen aunque valgan 0, al revés que casi todo el resto de este
// archivo: acá el cero es una MEDICIÓN —«miré y no pasó nada»— y omitirlo borraría el latido que
// distingue un bot callado de un colector muerto. Lo que se omite es el servicio que no reporta
// rendimiento en absoluto, que es la mayoría.
func seriesDeServicio() []serieDeServicio {
	return []serieDeServicio{
		{"musubi_fleet_service_up",
			"1 si el servicio está corriendo según su último reporte, 0 si no. NO dice si ese reporte es reciente: para eso está musubi_fleet_service_last_report_seconds.",
			"",
			func(sv fleet.Servicio, ahora time.Time) (float64, bool) {
				switch sv.EstadoActual() {
				case fleet.EstadoCorriendo:
					return 1, true
				case fleet.EstadoOcioso:
					// OCIOSO NO SE EXPORTA, ni como 0 ni como 1 (A70). No es «no sé» —eso es
					// `desconocido`— sino «la pregunta no aplica»: este servicio está apagado
					// PORQUE NADIE LO PIDIÓ, y un 0 acá dice «se cayó».
					//
					// Es la misma regla que gobierna el resto del exportador, con el mismo
					// motivo: la serie ausente deja a `ServicioCaido` sin nada que matchear, y
					// un 0 la habría hecho sonar dieciséis veces por máquina Windows. Una
					// alarma que suena sin que nada esté mal enseña a ignorar el canal.
					//
					// Cuando ese mismo servicio se muera de verdad, el agente lo reporta
					// `fallado`, la serie aparece en 0, y la alerta dispara.
					return 0, false
				default:
					return 0, true
				}
			}},
		{"musubi_fleet_service_last_report_seconds",
			"Segundos desde que la máquina reportó este servicio. AUSENTE si nunca lo reportó — un servicio declarado a mano y todavía sin muestras no tiene antigüedad, y un 0 diría «recién reportado».",
			"s",
			func(sv fleet.Servicio, ahora time.Time) (float64, bool) {
				if sv.UltimoReporte.IsZero() {
					return 0, false
				}
				return ahora.Sub(sv.UltimoReporte).Seconds(), true
			}},
		{"musubi_fleet_service_restarts_total",
			"Veces que el supervisor reinició este servicio. AUSENTE si la plataforma no lo sabe (el SCM de Windows no lo da). Es lo que distingue «anda» de «anda a los tumbos».",
			"",
			func(sv fleet.Servicio, ahora time.Time) (float64, bool) {
				if sv.Salud == nil || sv.Salud.Reinicios == nil {
					return 0, false
				}
				return float64(*sv.Salud.Reinicios), true
			}},

		// ── Rendimiento: qué HIZO el servicio, no en qué estado está (fase 4) ────────────────
		{"musubi_fleet_service_handled",
			"Unidades de trabajo que el servicio atendió en su última ventana. AUSENTE si el servicio no reporta rendimiento (la mayoría: un supervisor sabe si algo corre, no cuánto trabajo hizo). Un 0 SÍ se emite y es un dato: «se midió y no pasó nada», que es lo que distingue un servicio callado de un colector muerto. Se lee junto a musubi_fleet_service_window_seconds: un conteo sin su ventana no es una tasa.",
			"",
			func(sv fleet.Servicio, ahora time.Time) (float64, bool) {
				if r := rendimientoDe(sv); r != nil {
					return float64(r.Atendidas), true
				}
				return 0, false
			}},
		{"musubi_fleet_service_failed",
			"De las atendidas, cuántas salieron mal. Es un SUBCONJUNTO de musubi_fleet_service_handled, nunca un total aparte. AUSENTE con el mismo criterio que aquélla.",
			"",
			func(sv fleet.Servicio, ahora time.Time) (float64, bool) {
				if r := rendimientoDe(sv); r != nil {
					return float64(r.Fallidas), true
				}
				return 0, false
			}},
		{"musubi_fleet_service_window_seconds",
			"Cuánto tiempo cubren las dos series de arriba. Existe porque «47 atendidas» no significa nada sin saber en cuánto tiempo, y deducirlo del intervalo del colector ataría el gráfico a un número que vive en otra máquina.",
			"s",
			func(sv fleet.Servicio, ahora time.Time) (float64, bool) {
				if r := rendimientoDe(sv); r != nil && r.VentanaSeg > 0 {
					return float64(r.VentanaSeg), true
				}
				return 0, false
			}},
		{"musubi_fleet_service_latency_p95_ms",
			"Percentil 95 de latencia en la última ventana. AUSENTE si no se midió — y sobre cero unidades atendidas NO HAY percentil, así que ahí también está ausente: un 0 hundiría el promedio justo en los minutos tranquilos.",
			// LA UNIDAD VA VACÍA AUNQUE LA MÉTRICA SEA EN MILISEGUNDOS, Y NO ES UN DESCUIDO.
			//
			// El receptor OTLP de Prometheus AGREGA la unidad canónica al nombre cuando el nombre
			// no termina en ella. Con `Unit: "ms"`, `musubi_fleet_service_latency_p95_ms` entra a
			// Prometheus como `musubi_fleet_service_latency_p95_ms_milliseconds` — y la regla
			// `ServicioLento`, que consulta el nombre sin el sufijo, NO PUEDE DISPARARSE NUNCA.
			//
			// Medido en producción el 2026-08-30, al cablear el primer servicio que reporta
			// latencia: la serie estaba, con el nombre mutado, y la alerta llevaba un día
			// existiendo sin poder cumplirse. Dos omisiones se tapaban entre sí —las reglas de la
			// fase 4 no estaban desplegadas y ningún servicio reportaba latencia—, así que el
			// silencio se veía exactamente igual que «todo bien».
			//
			// `_seconds` no sufre esto porque `seconds` ES la forma canónica de `s` y Prometheus
			// deduplica. `ms` no lo es. La unidad vive en el NOMBRE, que es la convención de
			// Prometheus y la del repo; el campo `Unit` de OTLP sólo la duplicaría.
			//
			// TestNingunaSerieCambiaDeNombreAlEntrarPorOTLP custodia esto para todas.
			"",
			func(sv fleet.Servicio, ahora time.Time) (float64, bool) {
				if r := rendimientoDe(sv); r != nil && r.LatenciaP95Ms != nil {
					return float64(*r.LatenciaP95Ms), true
				}
				return 0, false
			}},
	}
}

// rendimientoDe saca el rendimiento de un servicio, o nil. Una sola definición de «este servicio
// mide trabajo», para que las cuatro series de arriba no puedan discrepar entre ellas.
func rendimientoDe(sv fleet.Servicio) *fleet.Rendimiento {
	if sv.Salud == nil {
		return nil
	}
	return sv.Salud.Rendimiento
}

// labelsDeServicio son los de la máquina más los dos del servicio.
//
// `class` entra porque es lo que permite preguntar «¿cómo están mis contenedores?» sin nombrarlos
// uno por uno. NO entra el pid: rota en cada reinicio, y una etiqueta que rota multiplica las
// series por cada vez que algo se reinicia — es exactamente la forma de matar un Prometheus.
func labelsDeServicio(sv fleet.Servicio, d fleet.Device) [][2]string {
	base := labelsDeFlota(d)
	out := make([][2]string, 0, len(base)+2)
	for _, kv := range base {
		out = append(out, kv)
	}
	return append(out, [2]string{"service", sv.Nombre}, [2]string{"class", sv.Clase})
}

// renderServicios escribe el bloque de servicios en el formato de exposición.
// serviciosTruncados responde si el techo POR PROYECTO recortó algo, sin escribir nada.
//
// Es una segunda pasada sobre la misma consulta y se acepta a propósito: la alternativa era que
// renderServicios devolviera el dato y que renderFlota lo llamara ANTES de emitir las series de
// máquina, lo que cambiaría el orden del exposition format que ya está probado. Un barrido más
// por scrape es barato al lado de reordenar la salida.
func serviciosTruncados(engine memory.StorageBackend, vistos []fleet.Device) bool {
	_, truncado := serviciosVisiblesParaMetricas(engine, vistos)
	return truncado
}

func renderServicios(b *strings.Builder, engine memory.StorageBackend, vistos []fleet.Device, ahora time.Time) {
	svs, truncado := serviciosVisiblesParaMetricas(engine, vistos)
	if len(svs) == 0 {
		return
	}
	if truncado {
		b.WriteString(fmt.Sprintf("# musubi_fleet_service: se exportaron los primeros %d servicios POR PROYECTO; hay más.\n", serviciosPorExportar))
	}
	for _, s := range seriesDeServicio() {
		escribirGaugeDeServicios(b, svs, s, ahora)
	}
}

func escribirGaugeDeServicios(b *strings.Builder, svs []servicioExportable, s serieDeServicio, ahora time.Time) {
	var cuerpo strings.Builder
	for _, e := range svs {
		v, hay := s.Valor(e.sv, ahora)
		if !hay {
			continue
		}
		fmt.Fprintf(&cuerpo, "%s{%s} %s\n", s.Nombre, etiquetasDeServicio(e.sv, e.d), formatearValor(v))
	}
	if cuerpo.Len() == 0 {
		// Ni una muestra: se omite el bloque ENTERO, con su HELP y su TYPE. Un HELP sin series
		// deja a Prometheus con una métrica declarada y vacía, que en un panel se dibuja igual
		// que una que vale cero.
		return
	}
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s gauge\n", s.Nombre, s.Ayuda, s.Nombre)
	b.WriteString(cuerpo.String())
}

func etiquetasDeServicio(sv fleet.Servicio, d fleet.Device) string {
	var b strings.Builder
	for i, kv := range labelsDeServicio(sv, d) {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(kv[0])
		b.WriteByte('=')
		// citarLabel y no %q: el nombre de un servicio lo produce la MÁQUINA, y una unit llamada
		// `a"b` partiría la línea en dos y corrompería todo el scrape, no sólo esa serie.
		b.WriteString(citarLabel(kv[1]))
	}
	return b.String()
}
