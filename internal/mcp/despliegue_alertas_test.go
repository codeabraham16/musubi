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

// leerDeploy devuelve el archivo SIN SUS COMENTARIOS, y ésa es la parte importante.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL CRUDO HAY QUE PEDIRLO POR NOMBRE
//
// La clase de defecto dominante de estas pruebas no es que falte una guarda: es que la guarda
// EXISTE, está en verde, y lo que la satisface es un comentario. Medido: de 46 guardas que leen
// archivos no-Go, SIETE quedaban verdes sobre su propio sabotaje — el texto que buscaban vivía en
// un comentario, en un mensaje de error, en un prefijo o en el vecino. Y pasó tres veces en un
// solo día con estos mismos archivos: el comentario que EXPLICA el arreglo satisfacía la guarda
// DEL arreglo, así que borrar el arreglo y dejar su documentación salía verde.
//
// Se intentó una vez con un helper opcional (`codigoDe`) y no alcanzó, porque un helper que hay
// que acordarse de llamar se olvida: en el archivo que lo inventó, CINCO de sus siete aserciones
// seguían leyendo el texto crudo. La única forma de cerrar la clase es invertir el default —que
// lo cómodo sea lo correcto— y que ver comentarios haya que pedirlo con otro nombre:
// `leerDeployCrudo`, que existe para las pocas guardas que de verdad miran la prosa (la pureza
// ASCII del cambiador mira TODOS los bytes, comentarios incluidos).
//
// LOS COMENTARIOS SE BLANQUEAN, NO SE BORRAN. Sacar la línea correría la numeración, y varias de
// estas guardas reportan «línea %d» para que alguien vaya a mirarla. Una guarda que manda al
// número equivocado gasta la confianza que la hace útil.
// ────────────────────────────────────────────────────────────────────────────────────────────
func leerDeploy(t *testing.T, partes ...string) string {
	t.Helper()
	return codigoSegunExtension(leerDeployCrudo(t, partes...), partes[len(partes)-1])
}

// leerDeployCrudo devuelve el archivo tal cual, comentarios incluidos. Usalo SÓLO cuando la
// propiedad que custodiás es del texto entero y no del código: la pureza ASCII, un encabezado
// obligatorio, una licencia.
func leerDeployCrudo(t *testing.T, partes ...string) string {
	t.Helper()
	ruta := filepath.Join(append([]string{"..", "..", "deploy"}, partes...)...)
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", ruta, err)
	}
	return string(crudo)
}

// codigoSegunExtension blanquea las líneas de comentario según el idioma del archivo.
//
// EL IDIOMA SALE DE LA EXTENSIÓN Y NO SE ADIVINA. `cambiar-agente.cmd` comenta con `REM` y `::`,
// no con `#`: aplicarle la regla de bash lo dejaría entero, que es exactamente cómo el candado de
// `device.token`, el matador de zombis y el `%~dp0` quedaron sostenidos por tres líneas `REM`.
// Una extensión que no conozco cae en `#`, que es la conservadora: si me equivoco, la guarda ve
// DE MENOS y falla pidiendo que alguien mire — nunca de más.
//
// El caso `#` lo resuelve `codigoDe`, que ya existía: tener dos funciones para la misma regla es
// la forma exacta en que una se arregla y la otra no.
func codigoSegunExtension(texto, nombre string) string {
	switch strings.ToLower(filepath.Ext(nombre)) {
	case ".cmd", ".bat":
		return codigoDeCmd(texto)
	case ".html", ".htm":
		return codigoDeHTML(texto)
	case ".md", ".markdown", ".txt", ".json":
		// NO TIENEN COMENTARIOS, y el `#` de un markdown es un TÍTULO. Blanquear esas líneas
		// destruiría justo la estructura que las guardas del RUNBOOK miran.
		return texto
	default:
		return codigoDe(texto)
	}
}

// codigoDeHTML blanquea los bloques `<!-- … -->`, que pueden abarcar varias líneas.
//
// Se blanquea DENTRO de cada línea en vez de borrarla entera: en `flota.html` hay comentarios al
// final de una línea con código, y comerse la línea completa haría que la guarda vea de menos.
func codigoDeHTML(texto string) string {
	var b strings.Builder
	dentro := false
	for i := 0; i < len(texto); {
		if !dentro && strings.HasPrefix(texto[i:], "<!--") {
			dentro = true
			i += 4
			continue
		}
		if dentro && strings.HasPrefix(texto[i:], "-->") {
			dentro = false
			i += 3
			continue
		}
		if !dentro {
			b.WriteByte(texto[i])
		} else if texto[i] == '\n' {
			// Los saltos se conservan para no correr la numeración de líneas.
			b.WriteByte('\n')
		}
		i++
	}
	return b.String()
}

// leerArchivoDeDespliegue tiene LA MISMA FIRMA que `os.ReadFile` y blanquea los comentarios.
//
// Existe porque el helper cómodo no alcanzó: 31 guardas de este paquete leían `deploy/` con
// `os.ReadFile` directo, salteándose el filtro sin querer y sin que nada lo dijera. Una de ellas
// era el candado de `%~dp0` del cambiador, que se satisfacía con su propia línea `REM`. Con la
// firma idéntica la conversión es un cambio de nombre y no una reescritura, así que no hay
// excusa para no hacerla — y `TestNingunaGuardaLeeUnArchivoDeDespliegueSinFiltrarComentarios`
// impide que aparezca la número 32.
func leerArchivoDeDespliegue(ruta string) ([]byte, error) {
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		return nil, err
	}
	return []byte(codigoSegunExtension(string(crudo), ruta)), nil
}

// codigoDeCmd blanquea los comentarios de cmd.exe: `REM` (insensible a mayúsculas, y también solo
// en su línea) y `::`, que es la etiqueta que cmd usa de comentario.
func codigoDeCmd(texto string) string {
	lineas := strings.Split(texto, "\n")
	for i, linea := range lineas {
		desnuda := strings.ToLower(strings.TrimSpace(linea))
		if desnuda == "rem" || strings.HasPrefix(desnuda, "rem ") ||
			strings.HasPrefix(desnuda, "rem\t") || strings.HasPrefix(desnuda, "::") {
			lineas[i] = ""
		}
	}
	return strings.Join(lineas, "\n")
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
	crudo, err := leerArchivoDeDespliegue(filepath.Join("..", "..", "deploy", "agente-windows.ps1"))
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

	// LA RECETA, EXIGIDA DONDE CADA MITAD DECIDE DE VERDAD.
	//
	// ESTE BLOQUE PEDÍA LAS CUATRO COSAS EN `prometheus.yml`, Y DOS NO PODÍAN ESTAR AHÍ NUNCA.
	// Quedó al descubierto cuando `leerDeploy` pasó a blanquear comentarios: dos de los cuatro
	// literales vivían SÓLO en la prosa del archivo, así que la mitad de esta guarda no podía
	// ponerse roja por ningún cambio de código. La receta existía únicamente como comentario.
	cfg := leerDeploy(t, "prometheus", "prometheus.yml")
	for _, quiero := range []string{
		"metric_relabel_configs",           // la receta concreta
		"musubi_fleet_(device|service)_.*", // sobre qué series: las DOS familias empujadas, y sólo ésas
	} {
		if !strings.Contains(cfg, quiero) {
			t.Errorf("prometheus.yml no declara la duplicación de series del empuje: falta %q.\nSin esa receta, encender el push hace que las 12 reglas de flota disparen dos veces por incidente.", quiero)
		}
	}

	// `--web.enable-otlp-receiver` NO SE PIDE ACÁ, y no es un olvido: es una bandera de línea de
	// comandos y no entra en un yaml de configuración, así que pedírsela a `prometheus.yml` era
	// pedirle a un archivo algo que no puede tener. Donde sí decide —el ExecStart del instalador y
	// la lista de argumentos del compose— ya la exigen las dos aserciones ancladas del principio de
	// esta prueba, con un regex de posición que es más fuerte que buscar el texto suelto.

	// `musubi-otlp-push` NO ES UN TEXTO DE DESPLIEGUE: es una constante de Go que viaja adentro
	// del sobre OTLP (`service.instance.id`). Prometheus la recibe y la convierte en el label
	// `instance`, que es lo ÚNICO que distingue lo empujado de lo scrapeado en los tableros y en
	// el comentario que explica el descarte. Nadie estaba en el medio: el productor podía
	// renombrarse y la documentación del despliegue seguía afirmando el nombre viejo.
	if instanciaDelEmpuje != "musubi-otlp-push" {
		t.Errorf("el empuje se identifica como %q y el despliegue está escrito para %q: quien mire "+
			"un tablero o el descarte de prometheus.yml va a buscar una etiqueta que ya no llega.",
			instanciaDelEmpuje, "musubi-otlp-push")
	}
	// Se lee el CRUDO a propósito: acá lo que se custodia es que el documento no mienta, y el
	// documento es justamente el comentario.
	if !strings.Contains(leerDeployCrudo(t, "prometheus", "prometheus.yml"), instanciaDelEmpuje) {
		t.Errorf("prometheus.yml ya no nombra %q, que es como el empuje se identifica hoy: el archivo "+
			"que explica por qué se descarta la mitad del scrape dejó de decir contra qué.", instanciaDelEmpuje)
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
	prom, err := leerArchivoDeDespliegue("../../deploy/prometheus/prometheus.yml")
	if err != nil {
		t.Fatal(err)
	}
	texto := string(prom)

	// LAS TRES PIEZAS TIENEN QUE ESTAR EN EL MISMO BLOQUE, Y EN CÓDIGO.
	//
	// La versión anterior buscaba las tres SUELTAS en todo el archivo, y eso falla de dos maneras
	// que se midieron el 2026-09-05 con el sabotaje declarado:
	//
	//   · `metric_relabel_configs` aparece en un COMENTARIO que explica la receta (línea 62), así
	//     que renombrar los dos bloques REALES dejaba la guarda en verde — con el empuje
	//     encendido, cada máquina mandaría dos series y cada alerta de flota saldría duplicada.
	//   · aunque no hubiera comentario, las tres cadenas podrían vivir en scrapes DISTINTOS: el
	//     `action: drop` de un job y el `regex` de otro darían por buena una receta que no existe.
	//
	// Se mira el bloque: desde cada `metric_relabel_configs:` de código hasta el próximo
	// `- job_name:`, que es donde empieza otro scrape.
	codigo := codigoDe(texto)
	tieneDrop := false
	for _, tramo := range strings.Split(codigo, "metric_relabel_configs:")[1:] {
		if i := strings.Index(tramo, "- job_name:"); i > 0 {
			tramo = tramo[:i]
		}
		if strings.Contains(tramo, `regex: "musubi_fleet_(device|service)_.*"`) && strings.Contains(tramo, "action: drop") {
			tieneDrop = true
		}
	}
	if !tieneDrop {
		t.Error("ningún `metric_relabel_configs` del scrape descarta las dos familias empujadas\n" +
			"(`device_` y `service_`) con `action: drop` EN EL MISMO BLOQUE — nombrarlo en un\n" +
			"comentario, o tener el regex en un scrape y el drop en otro, no arma la receta.\n" +
			"Con el empuje encendido, cada máquina tendría dos series y cada alerta de flota saldría duplicada")
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
// bloqueQueEncierra devuelve el índice de la línea que ABRE el bloque `{ ... }` donde vive
// `lineas[idx]`, o -1 si no está adentro de ninguno.
//
// EXISTE PORQUE `strings.Index` ANCLA EN LA PRIMERA OCURRENCIA, Y ESO NO ES UNA ESTRUCTURA.
//
// La guarda de abajo preguntaba «¿aparece `-UserId "SYSTEM"` DESPUÉS del primer
// `if ($AlArranque)`?». Mientras hubo un solo `if ($AlArranque)` en el archivo la pregunta
// coincidía con la propiedad. El 2026-09-01 `5183c26` insertó otro 162 líneas más arriba —para
// el endurecimiento del token— y desde entonces el ancla apunta a la línea 143 mientras el `if`
// que de verdad protege está en la 305. Medido el 2026-09-09 sobre el archivo real: offset del
// ancla 8394 (línea 143), offset de SYSTEM 17604 (línea 312).
//
// La consecuencia: comentar el `if ($AlArranque) {` de la 305 —o sea, hacer que el registro como
// SYSTEM sea INCONDICIONAL, que es exactamente lo que esta guarda existe para prohibir— la dejaba
// en VERDE, porque 312 sigue siendo mayor que 143. Ocho días así, y es la ÚNICA guarda del repo
// sobre el registro como SYSTEM.
//
// Caminar las llaves contesta la pregunta que importa —«¿está ADENTRO de ese bloque?»— y no se
// rompe si mañana alguien agrega un tercer `if ($AlArranque)` en cualquier lado.
//
// LÍMITE DECLARADO: cuenta llaves sin entender literales de PowerShell, así que una `{` dentro de
// un string la confundiría. Hoy no pasa (se comprueba abajo con un piso), y si pasara la guarda
// FALLA en vez de pasar: la ambigüedad no puede resolverse en verde.
func bloqueQueEncierra(lineas []string, idx int) int {
	profundidad := 0
	for i := idx - 1; i >= 0; i-- {
		l := lineas[i]
		for j := len(l) - 1; j >= 0; j-- {
			switch l[j] {
			case '}':
				profundidad++
			case '{':
				if profundidad == 0 {
					return i
				}
				profundidad--
			}
		}
	}
	return -1
}

func TestElAgenteDeWindowsNoCorreComoSystemSinQueAlguienLoPida(t *testing.T) {
	b, err := leerArchivoDeDespliegue("../../deploy/agente-windows.ps1")
	if err != nil {
		t.Fatalf("falta el instalador de Windows: %v", err)
	}
	// EL CÓDIGO, NO EL TEXTO. `#` es el comentario de PowerShell, y sin sacarlo estas tres
	// comprobaciones se satisfacen con la línea COMENTADA — el defecto que A107 midió en 7 de 46
	// guardas de este repo. Verificado el 2026-09-09: comentar el `Mal "-AlArranque exige
	// administrador"` de la línea 307 y el `Write-Host "...va a figurar CAIDA..."` de la 323
	// dejaba las dos aserciones en VERDE con el aviso ya invisible para quien instala.
	ps := codigoDe(string(b))
	lineas := strings.Split(ps, "\n")

	if !strings.Contains(ps, "$AlArranque") {
		t.Fatal("no existe la opción -AlArranque: una máquina que se reinicia y nadie loguea " +
			"figura caída estando viva, y no hay forma declarada de evitarlo")
	}

	// SYSTEM sólo puede aparecer DENTRO de la rama opt-in. Se comprueba por ESTRUCTURA —qué
	// bloque lo encierra— y no por orden de aparición: ver `bloqueQueEncierra`.
	iSystem := -1
	for i, l := range lineas {
		if strings.Contains(l, `-UserId "SYSTEM"`) {
			iSystem = i
			break
		}
	}
	if iSystem < 0 {
		t.Fatal(`no se encontró -UserId "SYSTEM" en el código del instalador: o se renombró la ` +
			`opción y esta guarda quedó vieja, o el registro al arranque desapareció`)
	}
	iAbre := bloqueQueEncierra(lineas, iSystem)
	if iAbre < 0 {
		t.Errorf(`-UserId "SYSTEM" (línea ~%d del código) no está adentro de NINGÚN bloque: `+
			`el registro como SYSTEM sería incondicional y el default escalaría privilegios`, iSystem+1)
	} else if !strings.Contains(lineas[iAbre], "$AlArranque") {
		t.Errorf("el registro como SYSTEM está adentro de `%s` y no de un `if ($AlArranque)`: "+
			"el default escalaría privilegios sin que nadie lo pida",
			strings.TrimSpace(lineas[iAbre]))
	}

	// Y exige elevación explícitamente: registrar una tarea como SYSTEM sin admin falla con un
	// error de PowerShell que no dice nada útil.
	if !strings.Contains(ps, "-AlArranque exige administrador") {
		t.Error("-AlArranque no comprueba admin antes de intentarlo")
	}
	// La consecuencia del default, dicha al usuario y no sólo en un comentario del código —
	// literalmente: por eso se busca en `codigoDe` y no en el texto crudo.
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
	cfg, err := leerArchivoDeDespliegue("../../deploy/prometheus/prometheus.yml")
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

	reglas, err := leerArchivoDeDespliegue("../../deploy/musubi-alerts.yml")
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

// SI LA TAREA CORRE COMO SYSTEM, SYSTEM TIENE QUE PODER LEER EL TOKEN.
//
// El instalador endurece el `device.token` con `SetAccessRuleProtection($true, $false)`: corta la
// herencia y NO copia las reglas heredadas, así que después queda UNA sola regla, la del usuario
// que instaló. Con `-AlArranque` el agente no es ese usuario: es SYSTEM. No puede leer su propio
// token y muere al arrancar.
//
// MEDIDO EN PRODUCCIÓN el 2026-09-02, en `gio`: la tarea decía «Attempted to run», volvía a
// `Ready` al instante, y la máquina quedaba fuera de la flota SIN UN SOLO MENSAJE DE ERROR.
//
// Y ESTA ES LA PARTE QUE IMPORTA: el instalador no lo veía. Su prueba de latido pasa el token por
// VARIABLE DE ENTORNO y corriendo como el usuario, así que nunca abre el archivo — probaba un
// camino distinto del que la tarea iba a usar, y daba verde sobre una instalación muerta. Es el
// mismo modo de fallo que este repo persigue en todos lados: verde por el motivo equivocado.
//
// Negarle el archivo a SYSTEM tampoco era una defensa: SYSTEM puede tomar posesión de cualquier
// archivo de la máquina. Lo único que lograba era romper el arranque.
//
// Sabotaje: borrar la regla de SYSTEM del bloque `if ($AlArranque)` → falla acá.
func TestSiLaTareaCorreComoSystemEntoncesSystemPuedeLeerElToken(t *testing.T) {
	b, err := leerArchivoDeDespliegue("../../deploy/agente-windows.ps1")
	if err != nil {
		t.Fatalf("falta el instalador de Windows: %v", err)
	}
	// El CÓDIGO, no el texto: mismo motivo que su hermana de arriba.
	ps := codigoDe(string(b))
	lineas := strings.Split(ps, "\n")

	// ACÁ HABÍA UN `t.Skip` Y SE SACÓ: UNA PRUEBA OMITIDA SE LEE IGUAL QUE UNA QUE NO EXISTE.
	//
	// Decía «el instalador ya no corta la herencia del token; esta guarda dejó de aplicar» y se
	// disparaba con `strings.Contains(ps, "SetAccessRuleProtection($true, $false)")`. O sea que
	// **sacarle el espacio** —`($true,$false)`, que PowerShell interpreta idéntico— desarmaba la
	// guarda ENTERA en verde, con la regla de SYSTEM todavía en el archivo y sin que nadie viera
	// un skip en la salida normal de `go test`. Un cambio NO-OP no puede apagar una guarda.
	//
	// Si de verdad se deja de cortar la herencia, esto tiene que ponerse ROJO y que alguien
	// borre la guarda a mano: esa decisión se toma mirando, no por ausencia de un literal. Por eso
	// el chequeo es laxo en la FORMA (se normalizan los espacios) y duro en el HECHO.
	sinEspacios := strings.NewReplacer(" ", "", "\t", "").Replace(ps)
	if !strings.Contains(sinEspacios, "SetAccessRuleProtection($true,$false)") {
		t.Fatal("el instalador ya no corta la herencia del token (`SetAccessRuleProtection`). " +
			"Si eso fue a propósito, borrá esta guarda explicando por qué; si no, volvé a ponerlo: " +
			"sin cortar la herencia, el endurecimiento del token no existe")
	}

	regla := `"NT AUTHORITY\SYSTEM", "FullControl", "Allow"`
	if !strings.Contains(ps, regla) {
		t.Error("con -AlArranque la tarea corre como SYSTEM y el token queda ilegible para él: " +
			"el agente muere al arrancar y la máquina figura caída sin ningún error")
	}
	// Y LA REGLA VA DENTRO DEL OPT-IN. Darle el token a SYSTEM en una instalación normal —donde la
	// tarea corre como la persona— sería aflojar el endurecimiento sin que nadie lo pidiera.
	//
	// SE PREGUNTA POR EL BLOQUE Y NO POR LOS 400 CARACTERES DE ANTES. La ventana fija tenía el
	// mismo defecto que el `strings.Index` de la guarda de arriba, con otra cara: no mide
	// pertenencia, mide cercanía. Mover la regla veinte líneas más abajo —fuera del `if`, pero
	// cerca— la dejaba en verde.
	iRegla := -1
	for i, l := range lineas {
		if strings.Contains(l, regla) {
			iRegla = i
			break
		}
	}
	if iRegla >= 0 {
		iAbre := bloqueQueEncierra(lineas, iRegla)
		if iAbre < 0 || !strings.Contains(lineas[iAbre], "$AlArranque") {
			abre := "(ningún bloque)"
			if iAbre >= 0 {
				abre = strings.TrimSpace(lineas[iAbre])
			}
			t.Errorf("la regla de SYSTEM sobre el token está adentro de `%s` y no de un "+
				"`if ($AlArranque)`: una instalación normal no debería aflojar los permisos del token", abre)
		}
	}
}

// EL SCRIPT QUE REEMPLAZA EL BINARIO DEL AGENTE VIVE EN EL REPO Y TIENE DOS INVARIANTES.
//
// Hasta el 2026-09-02 existía SÓLO en las dos máquinas Windows, escrito a mano y sin versionar.
// Nadie lo revisaba y no tenía pruebas, así que sus dos fallas sobrevivieron meses — no porque
// fueran sutiles, sino porque no había dónde verlas.
//
// LAS DOS, Y LAS DOS SE MIDIERON EN PRODUCCIÓN:
//
//  1. Tomaba su carpeta de `%LOCALAPPDATA%`. Con el agente corriendo como SYSTEM eso apunta al
//     perfil de sistema, no a la instalación: el script no encontraba el binario nuevo, escribía
//     «NO HAY BINARIO NUEVO» y salía sin tocar nada. Fallaba EN SILENCIO, que es justo lo que
//     existe para no hacer. `%~dp0` es lo único que no depende de quién lo ejecuta.
//
//  2. No mataba al proceso viejo. `schtasks /end` termina la tarea, no al hijo que el envoltorio
//     oculto dejó corriendo; y como el paso [2] RENOMBRA el binario en uso, ese proceso sigue
//     vivo desde `musubi.exe.viejo`, latiendo con la versión anterior. En `davantis-1` un agente
//     v0.106.0 zombi llevaba HORAS latiendo después de una actualización «exitosa», y su archivo
//     no se podía borrar — lo que disparaba el rollback y dejaba la máquina en la versión vieja.
//
// Y LA MATANZA VA POR RUTA EXACTA. En estas máquinas también corre la app de escritorio en
// `AppData\Local\Programs\musubi\musubi.exe`: un `taskkill /IM musubi.exe` la cerraría de un
// saque. Es el mismo nombre de imagen y una cosa completamente distinta.
//
// Sabotajes: volver a `%LOCALAPPDATA%`, sacar el Stop-Process, o cambiarlo por `/IM`.
func TestElScriptDeCambioDeAgenteNoDependeDeQuienLoEjecuta(t *testing.T) {
	b, err := leerArchivoDeDespliegue("../../deploy/cambiar-agente.cmd")
	if err != nil {
		t.Fatalf("falta deploy/cambiar-agente.cmd: vive sólo en las máquinas otra vez: %v", err)
	}
	cmd := string(b)

	if !strings.Contains(cmd, "set DIR=%~dp0") {
		t.Error("el script no toma su carpeta de `%~dp0`: con el agente como SYSTEM, `%LOCALAPPDATA%` " +
			"apunta al perfil de sistema y el script no encuentra el binario nuevo — falla en silencio")
	}
	if strings.Contains(cmd, "set DIR=%LOCALAPPDATA%") {
		t.Error("el script volvió a armar su carpeta con `%LOCALAPPDATA%`: eso lo rompe en cualquier " +
			"máquina instalada con -AlArranque")
	}
	if !strings.Contains(cmd, "Stop-Process") {
		t.Error("el script no mata al proceso viejo: sobrevive corriendo desde musubi.exe.viejo, late " +
			"con la versión anterior, y toma el archivo — lo que dispara el rollback")
	}
	// POR RUTA, NO POR NOMBRE DE IMAGEN — y se miran SÓLO LAS LÍNEAS EJECUTABLES.
	//
	// La primera versión de esta prueba fallaba contra el script CORRECTO, porque su comentario
	// nombra `taskkill /IM musubi.exe` justamente para explicar por qué no se usa. Una aserción
	// que no distingue el código del comentario que lo explica prohíbe documentar la decisión.
	for _, l := range strings.Split(cmd, "\n") {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(l)), "REM") {
			continue
		}
		if strings.Contains(strings.ToLower(l), "/im musubi.exe") {
			t.Errorf("el script mata por nombre de imagen (%q): en estas máquinas eso también cierra "+
				"la app de escritorio, que corre desde otra carpeta con el mismo nombre de ejecutable",
				strings.TrimSpace(l))
		}
	}
	if !strings.Contains(cmd, `$_.Path -eq '%DIR%\musubi.exe.viejo'`) {
		t.Error("el script no mata al proceso que corre desde `musubi.exe.viejo`: ése es EL zombi")
	}
	// ASCII PURO, por lo mismo que el instalador: PowerShell 5.1 y cmd.exe con UTF-8 sin BOM ya
	// rompieron una vez en este repo.
	for i, r := range cmd {
		if r > 127 {
			t.Fatalf("byte no-ASCII en la posición %d (%q): cmd.exe lo va a leer mal", i, r)
		}
	}
}
