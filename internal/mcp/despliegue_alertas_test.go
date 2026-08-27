package mcp

// Guardas del DESPLIEGUE de la cadena de alertas.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ HAY PRUEBAS SOBRE ARCHIVOS DE DESPLIEGUE
//
// El cerebro estuvo exponiendo `/metrics` mientras NADIE lo scrapeaba. Las 9 reglas de S4b, las
// de políticas de S10 y el dead-man's switch entero estaban escritos, probados... e INERTES. El
// registro de abiertos no lo vio porque cubre código, y esto era despliegue.
//
// Las dos cosas que estas pruebas custodian son las que se rompen solas con el tiempo:
//
//  1. TRES ARCHIVOS TIENEN QUE COINCIDIR EN UN PUERTO. `prometheus.yml` se scrapea a sí mismo,
//     el instalador systemd elige el bind, y el compose pasa `--web.listen-address`. Si divergen,
//     Prometheus anda pero su propio target queda DOWN — y `up{job="prometheus"}` es justamente
//     lo que distingue «se cayó el cerebro» de «se cayó Prometheus». Falla en el instrumento que
//     mide el fallo: el peor lugar posible.
//
//  2. NINGUNO PUEDE VOLVER AL 9090. Es el puerto por defecto de Cockpit, que viene instalado en
//     muchos servidores Linux (y ocupa el 9090 de `musubi-server`). El fallo silencioso no es que
//     Prometheus no arranque: es que alguien abra el 9090, vea una UI y crea que anda.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func leerDeploy(t *testing.T, partes ...string) string {
	t.Helper()
	ruta := filepath.Join(append([]string{"..", "..", "deploy"}, partes...)...)
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", ruta, err)
	}
	return string(crudo)
}

func TestElPuertoDePrometheusEsElMismoEnTodosLados(t *testing.T) {
	puerto := regexp.MustCompile(`127\.0\.0\.1:(\d{4,5})`)

	// El self-scrape de prometheus.yml: el ÚLTIMO target 127.0.0.1:<puerto> del archivo, que es
	// el del job `prometheus` (el primero es el cerebro en 7717).
	cfg := leerDeploy(t, "prometheus", "prometheus.yml")
	var selfScrape string
	dentroDelJob := false
	for _, l := range strings.Split(cfg, "\n") {
		if strings.Contains(l, "job_name: prometheus") {
			dentroDelJob = true
			continue
		}
		if dentroDelJob && strings.Contains(l, "targets:") {
			if m := puerto.FindStringSubmatch(l); m != nil {
				selfScrape = m[1]
			}
			break
		}
	}
	if selfScrape == "" {
		t.Fatal("prometheus.yml no tiene un self-scrape con puerto reconocible: sin él, `up{job=\"prometheus\"}` no existe y no se puede distinguir «cayó el cerebro» de «cayó Prometheus»")
	}

	instalador := leerDeploy(t, "prometheus", "install-musubi-prometheus.sh")
	mInst := regexp.MustCompile(`PROM_ADDR="\$\{PROM_ADDR:-127\.0\.0\.1:(\d+)\}"`).FindStringSubmatch(instalador)
	if mInst == nil {
		t.Fatal("no se encontró el default de PROM_ADDR en el instalador systemd")
	}

	compose := leerDeploy(t, "docker", "compose.yml")
	mComp := regexp.MustCompile(`--web\.listen-address=127\.0\.0\.1:(\d+)`).FindStringSubmatch(compose)
	if mComp == nil {
		t.Fatal("no se encontró --web.listen-address de Prometheus en el compose")
	}

	if selfScrape != mInst[1] || selfScrape != mComp[1] {
		t.Errorf("el puerto de Prometheus NO coincide entre los tres archivos que tienen que acordarlo:\n"+
			"  prometheus.yml (self-scrape) : %s\n"+
			"  instalador systemd (PROM_ADDR): %s\n"+
			"  docker compose (listen-address): %s\n"+
			"Con esto Prometheus anda pero su propio target queda DOWN, y `up{job=\"prometheus\"}` deja de servir justo para lo que existe.",
			selfScrape, mInst[1], mComp[1])
	}
	if selfScrape == "9090" {
		t.Error("Prometheus volvió al 9090, que es el puerto por defecto de Cockpit. El fallo peligroso no es que no arranque: es que alguien abra el 9090, vea una UI y crea que Prometheus está andando.")
	}
}

// El compose tiene que llegar al cerebro y a Alertmanager por 127.0.0.1. Dentro de una red de
// Docker eso apuntaría al CONTENEDOR: los scrapes darían «connection refused» y el bloque
// `alerting:` no encontraría a nadie — con las reglas evaluándose y sin notificar, que es
// exactamente el estado que este despliegue viene a terminar.
func TestElComposeUsaLaRedDelHost(t *testing.T) {
	compose := leerDeploy(t, "docker", "compose.yml")
	// Sólo las DIRECTIVAS, no las menciones en comentarios: contar apariciones sueltas hacía que
	// el comentario que explica la decisión rompiera la prueba que la custodia.
	directiva := regexp.MustCompile(`(?m)^\s+network_mode:\s*host\s*$`)
	if n := len(directiva.FindAllString(compose, -1)); n != 2 {
		t.Errorf("el compose declara `network_mode: host` %d veces, esperaba 2 (Prometheus y Alertmanager): sin la red del host, `127.0.0.1:7717` y `127.0.0.1:9093` apuntan al contenedor", n)
	}
	// Los secretos entran por ARCHIVO, nunca escritos en el yaml. La comprobación mira valores,
	// no rutas: `- /etc/.../musubi.token:/etc/prometheus/musubi.token:ro` es un montaje, y
	// confundirlo con un secreto volvía la guarda inservible por ruidosa.
	for _, l := range strings.Split(compose, "\n") {
		limpia := strings.TrimSpace(l)
		if strings.HasPrefix(limpia, "#") || strings.HasPrefix(limpia, "- /") {
			continue
		}
		if strings.Contains(limpia, "msb_") {
			t.Errorf("hay un token de Musubi escrito en el compose: %s", limpia)
		}
		if m := regexp.MustCompile(`(?i)^(password|token|api_key|secret):\s*(\S+)`).FindStringSubmatch(limpia); m != nil {
			t.Errorf("hay un secreto en el yaml (%s); tienen que entrar por archivo (url_file / credentials_file) para poder rotarlos sin tocar configuración", m[1])
		}
	}
}
