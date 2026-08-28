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

// serviciosPorExportar es el techo de servicios que salen a métricas, en toda la flota y por
// scrape. No es el mismo techo que `fleet.ServiciosPorLatido` (que acota UN latido de UNA
// máquina): éste protege a Prometheus de una flota entera.
//
// Cuando se corta, SE DICE. Un recorte silencioso deja series que desaparecen sin que nadie sepa
// por qué, y eso se lee como «ese servicio ya no existe» — que es una afirmación, no un silencio.
const serviciosPorExportar = 2000

// servicioExportable ata un servicio a la máquina donde corre. Los dos hacen falta para las
// etiquetas: el nombre del servicio solo no identifica nada en una flota.
type servicioExportable struct {
	sv fleet.Servicio
	d  fleet.Device
}

// serviciosVisiblesParaMetricas devuelve los servicios de las máquinas YA compuertadas.
func serviciosVisiblesParaMetricas(engine memory.StorageBackend, vistos []fleet.Device) (out []servicioExportable, truncado bool) {
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
			if len(out) >= serviciosPorExportar {
				return out, true
			}
			out = append(out, servicioExportable{sv: sv, d: d})
		}
	}
	return out, false
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

// seriesDeServicio son TRES, y son tres a propósito.
//
// `up` dice el ESTADO y `last_report_seconds` dice si esa afirmación es reciente. Combinarlas en
// una sola —«up sólo si además está fresco»— parece más simple y esconde el porqué: no se
// distinguiría un servicio caído de uno que dejó de reportar, que se arreglan distinto.
func seriesDeServicio() []serieDeServicio {
	return []serieDeServicio{
		{"musubi_fleet_service_up",
			"1 si el servicio está corriendo según su último reporte, 0 si no. NO dice si ese reporte es reciente: para eso está musubi_fleet_service_last_report_seconds.",
			"",
			func(sv fleet.Servicio, ahora time.Time) (float64, bool) {
				if sv.EstadoActual() == fleet.EstadoCorriendo {
					return 1, true
				}
				return 0, true
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
	}
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
func renderServicios(b *strings.Builder, engine memory.StorageBackend, vistos []fleet.Device, ahora time.Time) {
	svs, truncado := serviciosVisiblesParaMetricas(engine, vistos)
	if len(svs) == 0 {
		return
	}
	if truncado {
		b.WriteString(fmt.Sprintf("# musubi_fleet_service: se exportaron los primeros %d servicios; hay más.\n", serviciosPorExportar))
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
