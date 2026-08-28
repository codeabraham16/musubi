package mcp

// fleet_prometheus.go exporta la telemetría de la flota en formato Prometheus, para que la
// HISTORIA la guarde quien sabe guardarla. Track «Control de flota».
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA TENSIÓN QUE ESTE ARCHIVO RESUELVE, Y POR QUÉ NO ERA UN `for` SOBRE LAS FILAS
//
// S4 dejó a Musubi guardando el PRESENTE de la flota (la última muestra de cada máquina) y la
// historia explícitamente afuera: una tabla de series con 40 máquinas latiendo cada 30 s son
// 115.000 filas diarias que nadie consulta salvo para graficar, y graficar series es para lo que
// existe Prometheus — que este repo ya despliega.
//
// Pero exportar chocaba con S3. Un scrape de Prometheus presenta UNA credencial, mientras la
// compuerta de la flota es POR MÁQUINA Y POR CAPACIDAD. La salida fácil habría sido decir «/metrics
// es infraestructura, que vea todo»: eso convierte al scraper en una puerta trasera que sortea el
// eje entero, y bastaría con darle su token a alguien para leer la telemetría de todos los tenants.
//
// LA RESOLUCIÓN ES QUE EL SCRAPER NO ES UN CASO ESPECIAL: es un principal más. Se exporta
// exactamente lo que ESA credencial puede ver, con la misma PuedeSobreDevice que usa la tool. Si
// querés que Prometheus vea toda la flota, se lo declarás en principals.yaml:
//
//	- name: prometheus
//	  role: reader
//	  read: all              # ve todos los proyectos
//	  fleet:
//	    metrics: ["*"]       # ...y la telemetría de todas las máquinas
//
// Consecuencia fail-closed que conviene tener presente: con el TOKEN LEGACY (admin sin sección
// `fleet:`) no se exporta ninguna máquina. Es coherente con C1 —el rol de memoria no otorga
// capacidades de flota— y la salida lo DICE en vez de quedarse muda, para que nadie pierda una
// tarde buscando por qué el dashboard está vacío.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// proyectosParaExportar es el tope de proyectos que se recorren en un scrape.
//
// Existe porque el scrape corre cada 15 s y no puede convertirse en un escaneo sin fin de la
// base. No es un límite de la flota: es un límite de cuántos TENANTS distintos se barren por
// scrape, y si alguna vez se alcanza, la salida lo dice (nunca se trunca en silencio).
const proyectosParaExportar = 64

// renderFlota agrega al exposition format las métricas de las máquinas que `p` puede ver.
//
// Recibe el principal ya resuelto por el handler: la autorización NO se decide acá, se APLICA.
// intervaloSonda entra por parámetro (y no por una constante) porque de él SE DERIVA el umbral
// de «caído» de las máquinas sin agente (S10 · I2): con el umbral fijo de 90 s, un Tier B
// sondeado cada 5 min exportaba up=0 el 97 % del tiempo y `MaquinaCaida` disparaba para siempre.
// Una alerta que grita sin parar se silencia, y con ella se silencian las que sí importaban.
//
// DESDE S11 ESTE YA NO ES EL ÚNICO CAMINO DE SALIDA: el empuje OTLP (fleet_otlp.go) exporta lo
// mismo por otra boca. Por eso la selección de máquinas, la tabla de series y el juego de labels
// viven en funciones compartidas y no adentro de este `for` — dos copias discrepan el día que
// alguien agrega un campo, y la discrepancia se descubre semanas después, cuando dos dashboards
// muestran cosas distintas.
func renderFlota(b *strings.Builder, engine memory.StorageBackend, p *Principal, ahora time.Time, intervaloSonda time.Duration) {
	vistos, truncado := devicesVisiblesParaMetricas(engine, p)

	if len(vistos) == 0 {
		// Un bloque vacío y mudo manda a alguien a depurar Prometheus cuando el problema está en
		// principals.yaml. Se dice, en un comentario que el parser ignora.
		b.WriteString("# musubi_fleet: ninguna máquina visible para esta credencial.\n")
		b.WriteString("# Las capacidades de flota NO se derivan del rol: declarálas en principals.yaml\n")
		b.WriteString("#   fleet:\n#     metrics: [\"*\"]\n")
		return
	}
	if truncado {
		b.WriteString(fmt.Sprintf("# musubi_fleet: se barrieron los primeros %d proyectos; hay más.\n", proyectosParaExportar))
	}

	for _, s := range seriesDeFlota(ahora, intervaloSonda) {
		escribirGauge(b, vistos, s.Nombre, s.Ayuda, s.Valor)
	}
	// QUÉ CORRE ADENTRO de esas máquinas (A43). Va DESPUÉS y con las mismas máquinas ya
	// compuertadas: la lista `vistos` es la que pasó por PuedeSobreDevice, y reusarla es lo que
	// evita un segundo lugar donde olvidarse la compuerta.
	renderServicios(b, engine, vistos, ahora)
}

// devicesVisiblesParaMetricas resuelve QUÉ máquinas ve `p`, ya ordenadas por (proyecto, nombre).
//
// Es el ÚNICO lugar donde se combinan proyectosVisibles y PuedeSobreDevice, y por eso lo comparten
// el scrape y el empuje: un segundo recorrido de la flota es un segundo lugar donde olvidarse la
// compuerta, y ese olvido no se ve —exporta de más, calladito— hasta que alguien audita.
func devicesVisiblesParaMetricas(engine memory.StorageBackend, p *Principal) (vistos []fleet.Device, truncado bool) {
	proyectos, truncado := proyectosVisibles(engine, p)
	for _, proy := range proyectos {
		devices, err := engine.ListarDevices(proy, false)
		if err != nil {
			continue // un proyecto ilegible no puede tumbar el scrape entero
		}
		for _, d := range devices {
			if PuedeSobreDevice(p, d, fleet.CapMetrics) {
				vistos = append(vistos, d)
			}
		}
	}
	sort.Slice(vistos, func(i, j int) bool {
		if vistos[i].ProjectID != vistos[j].ProjectID {
			return vistos[i].ProjectID < vistos[j].ProjectID
		}
		return vistos[i].Name < vistos[j].Name
	})
	return vistos, truncado
}

// serieDeFlota es UNA métrica exportable de una máquina.
//
// La lista la produce seriesDeFlota() y la consumen LOS DOS caminos de salida —el scrape de
// /metrics y el empuje OTLP—, para que no puedan discrepar. El día que alguien agregue un campo a
// fleet.Muestra lo agrega acá y aparece en los dos lados; con dos copias aparece en uno.
type serieDeFlota struct {
	Nombre string
	Ayuda  string
	// Unidad es la unidad OTLP (UCUM). El exposition format la ignora: los nombres ya la llevan
	// en el sufijo, que es la convención de Prometheus.
	//
	// OJO CON EL "1" DE LO ADIMENSIONAL, que es la trampa de este campo: el receptor OTLP de
	// Prometheus NORMALIZA el nombre con la unidad, y a un gauge con unidad "1" le agrega el
	// sufijo `_ratio` — `musubi_fleet_device_up` llegaría como `musubi_fleet_device_up_ratio` y
	// las 12 reglas de deploy/musubi-alerts-flota.yml seguirían evaluándose sin disparar NUNCA.
	// Por eso lo adimensional viaja con unidad VACÍA, y las demás sólo declaran una unidad que el
	// nombre YA lleva (bytes, seconds, celsius, percent), que es el caso en el que la
	// normalización no agrega nada. Lo custodia TestNingunaUnidadRenombraLaSerieEnPrometheus.
	Unidad string
	// Entera decide la codificación OTLP del punto: asInt (string) o asDouble (número). En el
	// exposition format no cambia nada —formatearValor ya imprime los enteros sin decimales—,
	// así que las dos salidas siguen coincidiendo valor por valor.
	Entera bool
	Valor  func(d fleet.Device, m *fleet.Muestra) (float64, bool)
}

// seriesDeFlota devuelve las 19 series en orden estable: las DOS que salen de la fila del device
// (up, last_seen) y las 17 que salen de la MUESTRA.
//
// `ahora` e `intervaloSonda` entran por parámetro porque tres series son relativas al reloj (up,
// last_seen, sample_age): con un reloj por serie, `up` podría decir «viva» y `sample_age` medirse
// contra otro instante. Un solo reloj por export, y el empuje además lo usa para sellar los puntos.
func seriesDeFlota(ahora time.Time, intervaloSonda time.Duration) []serieDeFlota {
	return []serieDeFlota{
		{"musubi_fleet_device_up",
			"1 si la máquina dio señal de vida dentro de SU umbral, 0 si no. El umbral es por tier: 90s (3 latidos) con agente, 3x el intervalo de sondeo sin agente.",
			"", false,
			func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
				if d.EnLinea(ahora, umbralEnLineaPara(d, intervaloSonda)) {
					return 1, true
				}
				return 0, true
			}},
		{"musubi_fleet_device_last_seen_seconds",
			"Segundos desde el último latido. Ausente si la máquina nunca latió.",
			"s", false,
			func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
				if d.LastSeen.IsZero() {
					return 0, false
				}
				return ahora.Sub(d.LastSeen).Seconds(), true
			}},

		// De acá abajo, todo sale de la MUESTRA. Una máquina que no reportó no aporta ninguna de
		// estas series — y ésa es la regla central del export, ver escribirGauge.
		{"musubi_fleet_device_cpu_percent", "Uso de CPU (0-100), promedio del intervalo entre latidos. AUSENTE en el primer latido de un agente: el porcentaje es una derivada y hace falta una lectura anterior.",
			"%", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.CPUPct) })},
		{"musubi_fleet_device_cpus", "Cantidad de CPUs.",
			"", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.NumCPU), m.NumCPU > 0 })},
		{"musubi_fleet_device_memory_total_bytes", "RAM total.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.MemTotal), m.MemTotal > 0 })},
		{"musubi_fleet_device_memory_used_bytes", "RAM usada (total menos MemAvailable, no menos MemFree: el page cache no cuenta como ocupado).",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.MemUsada), m.MemTotal > 0 })},
		// AUSENTE en Windows y macOS: ninguno de los dos expone el equivalente de MemFree sin
		// mentir (ullAvailPhys es el análogo de MemAvailable, no de MemFree).
		{"musubi_fleet_device_memory_free_bytes", "RAM que el kernel no tiene asignada a nada (MemFree). NO es total menos usada: la usada sale de MemAvailable, y el page cache vive en el medio. AUSENTE en Windows y macOS, que no la exponen sin mentir.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDeBytes(m.MemLibre) })},
		{"musubi_fleet_device_swap_total_bytes", "Swap total.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.SwapTotal), m.SwapTotal > 0 })},
		{"musubi_fleet_device_swap_used_bytes", "Swap usada.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.SwapUsada), m.SwapTotal > 0 })},
		{"musubi_fleet_device_disk_total_bytes", "Tamaño del filesystem raíz.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.DiscoTotal), m.DiscoTotal > 0 })},
		{"musubi_fleet_device_disk_used_bytes", "Ocupado por archivos (como la columna Used de df).",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.DiscoUsado), m.DiscoTotal > 0 })},
		{"musubi_fleet_device_disk_available_bytes", "Lo que una aplicación todavía puede escribir (columna Avail de df). NO es total menos usado: entre medio está la reserva de root (~5%). Ésta es la serie para alertar.",
			"By", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.DiscoDisponible), m.DiscoTotal > 0 })},
		// AUSENTE en Windows: la carga es un concepto de UNIX y ahí no existe, así que la serie
		// no se emite en vez de emitir un 0 que se leería como «máquina ociosa».
		{"musubi_fleet_device_load1", "Carga a 1 minuto. AUSENTE en sistemas sin load average (Windows).",
			"", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.Load1) })},
		{"musubi_fleet_device_load5", "Carga a 5 minutos. AUSENTE en sistemas sin load average (Windows).",
			"", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.Load5) })},
		{"musubi_fleet_device_load15", "Carga a 15 minutos. AUSENTE en sistemas sin load average (Windows).",
			"", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.Load15) })},
		{"musubi_fleet_device_uptime_seconds", "Segundos desde el arranque de la máquina.",
			"s", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.UptimeSeg), m.UptimeSeg > 0 })},
		{"musubi_fleet_device_temperature_celsius", "Primera zona térmica. AUSENTE si la máquina no expone sensor.",
			"Cel", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return valorDe(m.TempC) })},
		// El nombre viaja en INGLÉS aunque el campo de la muestra sea `num_procesos`: adentro el
		// JSON está en castellano, y en Prometheus la convención del ecosistema es inglesa. El
		// punto de traducción es éste y ninguno más.
		{"musubi_fleet_device_processes", "Procesos, no hilos (el 4º campo de /proc/loadavg cuenta hilos y da 3 a 5 veces más). AUSENTE en macOS: contarlos ahí exigiría un fork+exec por latido en el proceso que corre en todas las máquinas.",
			"", true, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return float64(m.NumProcesos), m.NumProcesos > 0 })},
		{"musubi_fleet_device_sample_age_seconds", "Antigüedad de la muestra. Si crece sin parar, el agente late pero dejó de medir.",
			"s", false, deLaMuestra(func(m *fleet.Muestra) (float64, bool) { return ahora.Sub(m.Tomada).Seconds(), !m.Tomada.IsZero() })},
	}
}

// deLaMuestra adapta una serie que sólo mira la MUESTRA a la firma que lleva también el device.
// Una máquina sin muestra no aporta la serie: la comprobación de nil está escrita UNA vez acá y
// no diecisiete veces, que es como se olvida en la número dieciocho.
func deLaMuestra(f func(*fleet.Muestra) (float64, bool)) func(fleet.Device, *fleet.Muestra) (float64, bool) {
	return func(_ fleet.Device, m *fleet.Muestra) (float64, bool) {
		if m == nil {
			return 0, false
		}
		return f(m)
	}
}

// escribirGauge emite un gauge con una línea por máquina.
//
// LA REGLA CENTRAL DEL EXPORT: si el valor es DESCONOCIDO, LA LÍNEA NO SE EMITE. No se emite 0.
//
// Es el mismo principio que el `null` de la tool (D1/D3), y en Prometheus importa todavía más:
// una serie ausente se dibuja como un hueco y `absent()` la puede alertar, mientras que un 0
// entra al gráfico como una medición real. Un `cpu_percent 0` durante el primer latido de cada
// agente pintaría una caída a cero en cada reinicio, y esas caídas fantasma son exactamente lo
// que hace que alguien deje de mirar un dashboard.
//
// Si ninguna máquina tiene el valor, tampoco se emiten HELP y TYPE: un bloque de cabeceras sin
// series es ruido.
func escribirGauge(b *strings.Builder, devices []fleet.Device, nombre, ayuda string,
	valor func(fleet.Device, *fleet.Muestra) (float64, bool)) {

	var cuerpo strings.Builder
	for _, d := range devices {
		v, ok := valor(d, d.UltimaMuestra)
		if !ok {
			continue
		}
		fmt.Fprintf(&cuerpo, "%s{%s} %s\n", nombre, etiquetasDe(d), formatearValor(v))
	}
	if cuerpo.Len() == 0 {
		return
	}
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s gauge\n", nombre, ayuda, nombre)
	b.WriteString(cuerpo.String())
}

// labelsDeFlota es EL juego de labels de una máquina, en orden canónico y en UN solo lugar.
//
// Cardinalidad acotada a propósito: nombre, proyecto, tier y OS. Las TAGS quedan afuera —son texto
// libre del administrador y meterlas haría explotar la cardinalidad de la serie, que es la forma
// clásica de voltear un Prometheus. Y NADA de lo que la máquina reporta de sí misma entra acá
// (versión del agente, dirección, id de RustDesk): eso la dejaría re-etiquetándose sola.
//
// etiquetasDe lo formatea para el exposition format y atributosOTLP para el empuje. Armarlos dos
// veces es cómo un renombre de `device` a `hostname` deja las 12 reglas de
// deploy/musubi-alerts-flota.yml evaluándose para siempre sin disparar nunca.
func labelsDeFlota(d fleet.Device) [4][2]string {
	return [4][2]string{
		{"device", d.Name},
		{"project", d.ProjectID},
		{"tier", string(d.Tier)},
		{"os", d.OS},
	}
}

// etiquetasDe formatea los labels para el exposition format.
func etiquetasDe(d fleet.Device) string {
	var b strings.Builder
	for i, kv := range labelsDeFlota(d) {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(kv[0])
		b.WriteByte('=')
		b.WriteString(citarLabel(kv[1]))
	}
	return b.String()
}

// citarLabel escapa un valor de label según el exposition format: backslash, comilla y salto de
// línea. No es teórico: el nombre de una máquina lo escribe un administrador, y un device llamado
// `a"b` partiría la línea en dos y corrompería TODO el scrape, no sólo esa serie.
func citarLabel(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}

// formatearValor evita la notación científica de %v para los enteros grandes (los bytes de un
// disco de 500 GB) y no arrastra decimales inútiles.
func formatearValor(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.4g", v)
}

// proyectosVisibles resuelve QUÉ proyectos barre este scrape.
//
// Un principal acotado barre el suyo y nada más. Uno read=all (la cabina, la sala de mando, el
// scraper declarado como corresponde) barre todos los que tengan máquinas. El stdio local —que
// no llega acá por HTTP, pero el seam lo admite— también.
func proyectosVisibles(engine memory.StorageBackend, p *Principal) (proyectos []string, truncado bool) {
	federado := p == nil
	if p != nil {
		if read, _ := p.caps(); read == ReadAll {
			federado = true
		}
	}
	if !federado {
		if p.ProjectID == "" {
			return nil, false
		}
		return []string{p.ProjectID}, false
	}
	todos, err := engine.ProyectosConDevices(proyectosParaExportar + 1)
	if err != nil {
		return nil, false
	}
	if len(todos) > proyectosParaExportar {
		return todos[:proyectosParaExportar], true
	}
	return todos, false
}

// valorDe traduce el vocabulario del «no sé» del dominio (un puntero nil) al del exportador (el
// bool que decide si la línea se emite). Una sola traducción, para que ningún campo opcional
// nuevo se olvide de respetarla.
func valorDe(p *float64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return *p, true
}

// valorDeBytes es el gemelo de valorDe para los contadores de BYTES opcionales (*uint64). Existe
// por lo mismo: que la traducción del «no sé» esté escrita una vez y no se le olvide a nadie.
func valorDeBytes(p *uint64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return float64(*p), true
}
