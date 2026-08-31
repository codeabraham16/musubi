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

// El instalador de Windows tiene que ser ASCII PURO, con BOM.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTO MERECE UNA PRUEBA
//
// PowerShell 5.1 —el que viene en Windows— lee un `.ps1` como ANSI si no encuentra BOM. Con
// caracteres UTF-8 adentro, los bytes se vuelven basura y el parser muere en la primera comilla
// que quedó partida. Pasó de verdad: un `✓` en un `Write-Host` tumbó el script entero con un
// «Missing closing '}'» que no tiene NADA que ver con la causa.
//
// El arreglo son DOS cosas a la vez, y por eso las dos se custodian acá:
//
//   - EL BOM, para que PowerShell lea UTF-8 sin ambigüedad.
//   - ASCII PURO, para que no pueda romperse ni aunque el BOM se pierda — y se pierde: este
//     archivo viaja por scp, por `cat >` sobre ssh y por el portapapeles de una consola remota.
//
// Un guion largo en un COMENTARIO ya se coló una vez después de establecer la regla. En un
// comentario no rompe nada, y por eso mismo la vigilancia humana no lo ve: la regla tiene que
// valer para todo el archivo o no vale.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestElInstaladorDeWindowsEsAsciiConBOM(t *testing.T) {
	crudo, err := os.ReadFile(filepath.Join("..", "..", "deploy", "agente-windows.ps1"))
	if err != nil {
		t.Fatalf("no se pudo leer el instalador de Windows: %v", err)
	}
	if len(crudo) < 3 || crudo[0] != 0xEF || crudo[1] != 0xBB || crudo[2] != 0xBF {
		t.Error("el .ps1 no empieza con BOM UTF-8: PowerShell 5.1 lo va a leer como ANSI")
	}
	for i, b := range crudo[3:] {
		if b > 0x7E || (b < 0x20 && b != '\n' && b != '\r' && b != '\t') {
			linea := 1 + strings.Count(string(crudo[3:i+3]), "\n")
			t.Fatalf("byte no-ASCII 0x%02X en la línea %d del instalador.\n"+
				"Un .ps1 que viaja por scp y portapapeles no puede depender de que un carácter sobreviva el viaje: usá ASCII (-- en vez de guion largo, OK/ERR en vez de simbolos).", b, linea)
		}
	}
}

// EL RECEPTOR OTLP SE HABILITA EN LOS DOS CAMINOS DE INSTALACIÓN, Y LA DUPLICACIÓN ESTÁ DECIDIDA.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// DOS COSAS QUE SE ROMPEN SOLAS, Y LAS DOS SIN SÍNTOMA
//
//  1. Prometheus NO ACEPTA OTLP POR DEFECTO. Sin `--web.enable-otlp-receiver` el POST del empuje
//     devuelve 404 y el empuje muere en silencio con la configuración del cerebro perfecta. El
//     flag tiene que estar en el instalador systemd Y en el compose — es el mismo patrón de
//     divergencia entre dos caminos de instalación que esta suite ya custodia para el puerto, y
//     acá se paga peor: el que se instaló con el otro camino no exporta y no se entera.
//
//  2. SI EL MISMO PROMETHEUS SCRAPEA /metrics Y RECIBE EL PUSH, las mismas series
//     `musubi_fleet_device_*` entran por dos caminos con `instance` distinto, y las 12 reglas de
//     rules/musubi-alerts-flota.yml disparan DOBLE. Eso es una decisión que hay que escribir, no
//     descubrir a las 4 de la mañana con dos avisos por incidente.
//
// Sabotaje que la hace fallar: agregar el flag sólo al unit systemd; o encender el push sin dejar
// la receta escrita en prometheus.yml.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestElReceptorOTLPSeHabilitaEnLosDosLugaresYLaDuplicacionEstaDeclarada(t *testing.T) {
	const flag = "--web.enable-otlp-receiver"

	instalador := leerDeploy(t, "prometheus", "install-musubi-prometheus.sh")
	// En el ExecStart de la unit, no sólo en un comentario: un flag comentado no habilita nada.
	if !regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(flag)).MatchString(instalador) {
		t.Errorf("el instalador systemd no le pasa %s a Prometheus: el POST del empuje va a devolver 404 y el empuje va a morir en silencio", flag)
	}
	compose := leerDeploy(t, "docker", "compose.yml")
	if !regexp.MustCompile(`(?m)^\s+- ` + regexp.QuoteMeta(flag)).MatchString(compose) {
		t.Errorf("el compose no le pasa %s a Prometheus: los dos caminos de instalación tienen que habilitarlo o el que use el otro no recibe el empuje", flag)
	}

	// La puerta de ESCRITURA anónima sigue cerrada en los dos: con remote-write habilitado,
	// cualquiera que llegue al puerto puede inyectar `musubi_fleet_device_up 1` y apagar
	// MaquinaCaida y FlotaSinTelemetria sin dejar rastro.
	//
	// Se miran las DIRECTIVAS y no las menciones: los dos archivos EXPLICAN en un comentario por
	// qué no se habilita, y contar apariciones sueltas haría que el comentario que documenta la
	// decisión rompa la prueba que la custodia. Es la misma corrección que ya lleva
	// TestElComposeUsaLaRedDelHost.
	for nombre, contenido := range map[string]string{"instalador": instalador, "compose": compose} {
		for _, l := range strings.Split(contenido, "\n") {
			limpia := strings.TrimSpace(l)
			if strings.HasPrefix(limpia, "#") {
				continue
			}
			if strings.Contains(limpia, "--web.enable-remote-write-receiver") {
				t.Errorf("el %s habilita remote-write: es una puerta de escritura anónima sobre los datos de los que viven las alertas", nombre)
			}
		}
	}

	// La duplicación, DECIDIDA Y ESCRITA donde la va a leer quien encienda el push.
	cfg := leerDeploy(t, "prometheus", "prometheus.yml")
	for _, quiero := range []string{
		flag,                               // dónde se habilita
		"musubi-otlp-push",                 // cómo se distingue lo empujado de lo scrapeado
		"metric_relabel_configs",           // la receta concreta
		"musubi_fleet_(device|service)_.*", // sobre qué series: las DOS familias empujadas, y sólo ésas
	} {
		if !strings.Contains(cfg, quiero) {
			t.Errorf("prometheus.yml no declara la duplicación de series del empuje: falta %q.\nSin esa receta, encender el push hace que las 12 reglas de flota disparen dos veces por incidente.", quiero)
		}
	}
}

// TestElScrapeYElEmpujeNoTraenLoMismo — la guarda de «un solo productor por dato».
//
// `/metrics` sirve las métricas propias del cerebro Y la telemetría de flota. Con `fleet.otlp`
// encendido, esa segunda mitad llega ADEMÁS por el empuje, con otro `instance`. No se pisan, así
// que el daño no se ve como un error — se ve como que todo anda. Lo que pasa es que cada regla de
// flota matchea DOS series y cada alerta sale DUPLICADA. Medido en producción al encenderlo:
// 5 alertas se convirtieron en 10 avisos.
//
// Las dos mitades van juntas y esta prueba las ata: el `drop` en el scrape sólo es correcto
// mientras exista el empuje, y el empuje sólo es no-duplicación mientras exista el `drop`.
//
// Sabotaje que la hace fallar: sacar el bloque metric_relabel_configs de prometheus.yml,
// borrar las tres alertas del empujador de musubi-alerts.yml, o —el que costó descubrir—
// ENSANCHAR el drop de vuelta a `musubi_fleet_.*`.
func TestElScrapeYElEmpujeNoTraenLoMismo(t *testing.T) {
	prom, err := os.ReadFile("../../deploy/prometheus/prometheus.yml")
	if err != nil {
		t.Fatal(err)
	}
	texto := string(prom)

	tieneDrop := strings.Contains(texto, "metric_relabel_configs") &&
		strings.Contains(texto, `regex: "musubi_fleet_(device|service)_.*"`) &&
		strings.Contains(texto, "action: drop")
	if !tieneDrop {
		t.Error("el scrape ya no descarta las dos familias empujadas (`device_` y `service_`): con el empuje encendido, cada máquina tendría dos series y cada alerta de flota saldría duplicada")
	}

	// Y LA MITAD OPUESTA, que es la que faltaba: el drop no puede ser TAN ancho como para tirar
	// la única serie de la familia que el empuje NO lleva.
	//
	// `musubi_fleet_policy_actions_total` sale sólo del scrape. Con un drop sobre
	// `musubi_fleet_.*` no llegaba nunca a Prometheus, así que `PoliticaQueNoCura` y
	// `PoliticaSinPermiso` no podían dispararse ni con una política configurada Y disparando.
	// Es el modo de fallo que este repo persigue: la ausencia de la serie se explicaba con una
	// razón verdadera —«no hay políticas configuradas»— que tapaba la real.
	if strings.Contains(texto, `regex: "musubi_fleet_.*"`) {
		t.Error("el drop volvió a ser `musubi_fleet_.*`: eso también tira `musubi_fleet_policy_actions_total`, que el empuje NO lleva, y deja a las alertas de políticas sin serie para siempre")
	}

	// Y la contraparte: si el scrape no trae flota, la única fuente es el empuje. Que el empuje
	// muera no puede ser invisible — las reglas de flota no fallarían, ENMUDECERÍAN.
	reglas := reglasDeAlerta(t)
	for _, alerta := range []string{"MusubiPushOTLPFallando", "MusubiPushOTLPMudo", "MusubiPushOTLPNuncaLlego"} {
		if !strings.Contains(reglas, alerta) {
			t.Errorf("falta la alerta %s: sin ella, un empuje muerto deja a las 12 reglas de flota sin datos y nadie se entera", alerta)
		}
	}
}

// EL AGENTE DE WINDOWS CORRE AL INICIAR SESIÓN, Y ESO TIENE QUE SEGUIR SIENDO EL DEFAULT.
//
// La tarea programada por defecto arranca `-AtLogOn`: el agente vive mientras haya alguien
// logueado. La consecuencia costó dos días de lectura equivocada — `gio` figuraba apagada
// mientras respondía al ping por el tailnet en 145 ms; la máquina andaba y el agente no estaba
// corriendo, porque nadie había iniciado sesión desde el reinicio.
//
// El arreglo existe (`-AlArranque`, que registra al arranque y como SYSTEM) y es OPT-IN a
// propósito: correr el agente como SYSTEM cambia lo que la flota puede hacer en esa máquina —
// `musubi_fleet_exec` pasaría a ejecutarse con privilegios de SYSTEM. Es una decisión de
// seguridad, no una comodidad, y no puede tomarse sola por conveniencia de despliegue.
//
// Esta prueba custodia las dos mitades: que el default NO escale, y que la consecuencia del
// default esté DICHA. Un default seguro cuyo costo nadie ve es cómo se pierden dos días.
//
// Sabotaje que la hace fallar: hacer que -AlArranque sea el camino por defecto, o borrar el
// aviso que explica qué pasa si la máquina se reinicia.
func TestElAgenteDeWindowsNoCorreComoSystemSinQueAlguienLoPida(t *testing.T) {
	b, err := os.ReadFile("../../deploy/agente-windows.ps1")
	if err != nil {
		t.Fatalf("falta el instalador de Windows: %v", err)
	}
	ps := string(b)

	if !strings.Contains(ps, "$AlArranque") {
		t.Fatal("no existe la opción -AlArranque: una máquina que se reinicia y nadie loguea " +
			"figura caída estando viva, y no hay forma declarada de evitarlo")
	}
	// SYSTEM sólo puede aparecer DENTRO de la rama opt-in. Si el bloque `if ($AlArranque)`
	// desaparece, el registro como SYSTEM pasa a ser incondicional.
	iRama := strings.Index(ps, "if ($AlArranque)")
	iSystem := strings.Index(ps, `-UserId "SYSTEM"`)
	if iRama < 0 || iSystem < 0 || iSystem < iRama {
		t.Error("el registro como SYSTEM no está dentro de la rama opt-in: el default escalaría privilegios")
	}
	// Y exige elevación explícitamente: registrar una tarea como SYSTEM sin admin falla con un
	// error de PowerShell que no dice nada útil.
	if !strings.Contains(ps, "-AlArranque exige administrador") {
		t.Error("-AlArranque no comprueba admin antes de intentarlo")
	}
	// La consecuencia del default, dicha al usuario y no sólo en un comentario del código.
	if !strings.Contains(ps, "va a figurar CAIDA en la flota estando viva") {
		t.Error("el instalador no avisa qué pasa si la máquina se reinicia sin que nadie inicie sesión")
	}
}

// LA CADENA DE ALERTAS SE VIGILA A SÍ MISMA, Y ESO NO ESTABA.
//
// Medido el 2026-08-28, no supuesto: el receptor del dead-man's switch llevaba **32 horas
// fallando cada 5 minutos** —387 errores, un `url_file` a un archivo que no existía— mientras el
// MISMO Alertmanager entregaba por Telegram sin un solo fallo. 310 intentos por webhook con 279
// fallos; 31 por Telegram con 0. Un canal roto al lado de uno sano.
//
// Nada lo contaba: Prometheus scrapeaba dos targets —el cerebro y a sí mismo— y Alertmanager no
// era ninguno. El error vivía en el log de un contenedor, que es donde las cosas van a no ser
// vistas.
//
// Las dos mitades tienen que estar juntas o no sirve ninguna: el job sin la regla junta series
// que nadie mira, y la regla sin el job es una alerta sin datos — que no falla, enmudece, y se
// ve puesta.
//
// Sabotaje que la hace fallar: sacar el job `alertmanager` de prometheus.yml, o la regla de
// musubi-alerts.yml.
func TestLaCadenaDeAlertasSeVigilaASiMisma(t *testing.T) {
	cfg, err := os.ReadFile("../../deploy/prometheus/prometheus.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "job_name: alertmanager") {
		t.Error("Prometheus no scrapea al Alertmanager: que la cadena de alertas deje de entregar " +
			"queda sólo en el log de un contenedor")
	}
	if !strings.Contains(string(cfg), `targets: ["127.0.0.1:9093"]`) {
		t.Error("el job del alertmanager no apunta al 9093, que es donde escucha y adonde lo manda `alerting:`")
	}

	reglas, err := os.ReadFile("../../deploy/musubi-alerts.yml")
	if err != nil {
		t.Fatal(err)
	}
	// Se mira la regla y no el archivo: los comentarios de arriba nombran la métrica para
	// explicar el caso, y un Contains sobre el texto entero se conformaría con la explicación.
	var utiles []string
	for _, l := range strings.Split(string(reglas), "\n") {
		if s := strings.TrimSpace(l); s != "" && !strings.HasPrefix(s, "#") {
			utiles = append(utiles, s)
		}
	}
	cuerpo := strings.Join(utiles, "\n")
	if !strings.Contains(cuerpo, "alertmanager_notifications_failed_total") {
		t.Error("no hay regla sobre los envíos fallidos: el job junta las series y nadie las mira")
	}
	// Agrupado por `integration` y no por `reason`: un canal roto que rota entre `clientError` y
	// `other` produciría dos alertas de la misma cosa.
	if !strings.Contains(cuerpo, "by(integration)") {
		t.Error("la regla no agrupa por integración: un mismo canal roto daría una alerta por cada motivo")
	}
}
