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
func renderFlota(b *strings.Builder, engine memory.StorageBackend, p *Principal, ahora time.Time, intervaloSonda time.Duration) {
	proyectos, truncado := proyectosVisibles(engine, p)

	var vistos []fleet.Device
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

	escribirGauge(b, vistos, ahora, "musubi_fleet_device_up",
		"1 si la máquina dio señal de vida dentro de SU umbral, 0 si no. El umbral es por tier: 90s (3 latidos) con agente, 3x el intervalo de sondeo sin agente.",
		func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
			if d.EnLinea(ahora, umbralEnLineaPara(d, intervaloSonda)) {
				return 1, true
			}
			return 0, true
		})

	escribirGauge(b, vistos, ahora, "musubi_fleet_device_last_seen_seconds",
		"Segundos desde el último latido. Ausente si la máquina nunca latió.",
		func(d fleet.Device, m *fleet.Muestra) (float64, bool) {
			if d.LastSeen.IsZero() {
				return 0, false
			}
			return ahora.Sub(d.LastSeen).Seconds(), true
		})

	// De acá abajo, todo sale de la MUESTRA. Una máquina que no reportó no aporta ninguna de
	// estas series — y ésa es la regla central del export, ver escribirGauge.
	series := []struct {
		nombre, ayuda string
		valor         func(*fleet.Muestra) (float64, bool)
	}{
		{"musubi_fleet_device_cpu_percent", "Uso de CPU (0-100), promedio del intervalo entre latidos. AUSENTE en el primer latido de un agente: el porcentaje es una derivada y hace falta una lectura anterior.",
			func(m *fleet.Muestra) (float64, bool) { return valorDe(m.CPUPct) }},
		{"musubi_fleet_device_cpus", "Cantidad de CPUs.", func(m *fleet.Muestra) (float64, bool) { return float64(m.NumCPU), m.NumCPU > 0 }},
		{"musubi_fleet_device_memory_total_bytes", "RAM total.", func(m *fleet.Muestra) (float64, bool) { return float64(m.MemTotal), m.MemTotal > 0 }},
		{"musubi_fleet_device_memory_used_bytes", "RAM usada (total menos MemAvailable, no menos MemFree: el page cache no cuenta como ocupado).", func(m *fleet.Muestra) (float64, bool) { return float64(m.MemUsada), m.MemTotal > 0 }},
		{"musubi_fleet_device_swap_total_bytes", "Swap total.", func(m *fleet.Muestra) (float64, bool) { return float64(m.SwapTotal), m.SwapTotal > 0 }},
		{"musubi_fleet_device_swap_used_bytes", "Swap usada.", func(m *fleet.Muestra) (float64, bool) { return float64(m.SwapUsada), m.SwapTotal > 0 }},
		{"musubi_fleet_device_disk_total_bytes", "Tamaño del filesystem raíz.", func(m *fleet.Muestra) (float64, bool) { return float64(m.DiscoTotal), m.DiscoTotal > 0 }},
		{"musubi_fleet_device_disk_used_bytes", "Ocupado por archivos (como la columna Used de df).", func(m *fleet.Muestra) (float64, bool) { return float64(m.DiscoUsado), m.DiscoTotal > 0 }},
		{"musubi_fleet_device_disk_available_bytes", "Lo que una aplicación todavía puede escribir (columna Avail de df). NO es total menos usado: entre medio está la reserva de root (~5%). Ésta es la serie para alertar.",
			func(m *fleet.Muestra) (float64, bool) { return float64(m.DiscoDisponible), m.DiscoTotal > 0 }},
		// AUSENTE en Windows: la carga es un concepto de UNIX y ahí no existe, así que la serie
		// no se emite en vez de emitir un 0 que se leería como «máquina ociosa».
		{"musubi_fleet_device_load1", "Carga a 1 minuto. AUSENTE en sistemas sin load average (Windows).", func(m *fleet.Muestra) (float64, bool) { return valorDe(m.Load1) }},
		{"musubi_fleet_device_load5", "Carga a 5 minutos. AUSENTE en sistemas sin load average (Windows).", func(m *fleet.Muestra) (float64, bool) { return valorDe(m.Load5) }},
		{"musubi_fleet_device_load15", "Carga a 15 minutos. AUSENTE en sistemas sin load average (Windows).", func(m *fleet.Muestra) (float64, bool) { return valorDe(m.Load15) }},
		{"musubi_fleet_device_uptime_seconds", "Segundos desde el arranque de la máquina.", func(m *fleet.Muestra) (float64, bool) { return float64(m.UptimeSeg), m.UptimeSeg > 0 }},
		{"musubi_fleet_device_temperature_celsius", "Primera zona térmica. AUSENTE si la máquina no expone sensor.",
			func(m *fleet.Muestra) (float64, bool) { return valorDe(m.TempC) }},
		{"musubi_fleet_device_sample_age_seconds", "Antigüedad de la muestra. Si crece sin parar, el agente late pero dejó de medir.",
			func(m *fleet.Muestra) (float64, bool) { return ahora.Sub(m.Tomada).Seconds(), !m.Tomada.IsZero() }},
	}
	for _, s := range series {
		valor := s.valor
		escribirGauge(b, vistos, ahora, s.nombre, s.ayuda, func(_ fleet.Device, m *fleet.Muestra) (float64, bool) {
			if m == nil {
				return 0, false
			}
			return valor(m)
		})
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
func escribirGauge(b *strings.Builder, devices []fleet.Device, ahora time.Time, nombre, ayuda string,
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

// etiquetasDe arma el juego de labels de una máquina. Cardinalidad acotada a propósito: nombre,
// proyecto, tier y OS. Las TAGS quedan afuera —son texto libre del administrador y meterlas
// haría explotar la cardinalidad de la serie, que es la forma clásica de voltear un Prometheus.
func etiquetasDe(d fleet.Device) string {
	return fmt.Sprintf(`device=%s,project=%s,tier=%s,os=%s`,
		citarLabel(d.Name), citarLabel(d.ProjectID), citarLabel(string(d.Tier)), citarLabel(d.OS))
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
