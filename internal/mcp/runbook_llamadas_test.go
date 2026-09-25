package mcp

// runbook_llamadas_test.go custodia las RECETAS del runbook contra el contrato real de las tools.
//
// Una receta con un parámetro mal escrito no rompe ninguna compilación. Se descubre cuando alguien
// la copia en medio de una intervención y el cerebro contesta «argumentos inválidos» —o peor:
// `json.Unmarshal` descarta en silencio las claves que no conoce, así que un `"motivos"` en vez de
// `"motivo"` declara la ventana SIN motivo y nadie se entera—. Y el runbook es justo el texto que se
// copia sin releer, porque se abre cuando algo ya está sonando.
//
// La receta que trajo esta guarda es la de «antes de tocar un agente a mano, declarar la ventana»:
// el 2026-09-20 una intervención a mano sobre la tarea del agente de `gio` hizo sonar
// `AgenteCaidoConMaquinaViva` 50 minutos, y `musubi_fleet_maintenance` no se había llamado nunca.
// Escribir el paso en el runbook no alcanza si el paso escrito no funciona.

import (
	"encoding/json"
	"math"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// reLlamadaATool reconoce una llamada CON ARGUMENTOS en una sola línea, en las dos formas que usa
// el runbook: `./deploy/musubi-tool.sh <tool> '<json>'` y la abreviada `<tool> {json}`.
//
// Exige espacio entre el nombre y la llave a propósito: una serie de Prometheus con etiquetas
// (`musubi_fleet_net_up{device="…"}`) va pegada, y no es una llamada.
var reLlamadaATool = regexp.MustCompile("\\b(musubi_[a-z_]+)[ \\t]+'?(\\{[^'\\n`]*\\})")

// reGuionConTool reconoce un guion que el runbook manda a correr con una tool —`<ruta>.sh <tool>`—,
// lleve argumentos o no. Captura la RUTA tal como está escrita, no el nombre que se espera: un
// `musubi-tools.sh` mal tipeado tiene que verse, no pasar de largo por no coincidir.
var reGuionConTool = regexp.MustCompile("([^\\s`'\"(]+\\.sh)[ \\t]+(musubi_[a-z_]+)")

// guionDelRunbookExiste dice si la ruta de un guion que el runbook manda a correr existe en el repo.
//
// El runbook vive en deploy/ y usa dos formas: `./deploy/x.sh`, desde la raíz de un clon, y
// `./x.sh`, desde deploy/ o desde el home del server, adonde se copia el guion de deploy/. Una ruta
// absoluta es del server: se busca el archivo del mismo nombre en deploy/, que es de donde sale.
func guionDelRunbookExiste(ruta string) bool {
	raiz := filepath.Join("..", "..")
	var candidatos []string
	if strings.HasPrefix(ruta, "/") || strings.HasPrefix(ruta, "~") {
		candidatos = []string{filepath.Join(raiz, "deploy", path.Base(ruta))}
	} else {
		r := filepath.FromSlash(strings.TrimPrefix(ruta, "./"))
		candidatos = []string{filepath.Join(raiz, r), filepath.Join(raiz, "deploy", r)}
	}
	for _, c := range candidatos {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}

// rangosQueElHandlerExige son los límites que el InputSchema no declara y el handler aplica igual:
// un valor fuera de rango parsea, tiene el tipo correcto y vuelve con error en mitad de la
// intervención. `minutos` ≤ 0 contesta «falta `minutos`» y por encima del techo lo rechaza
// AbrirMantenimiento (fleet.MantenimientoMax).
var rangosQueElHandlerExige = map[string]map[string][2]float64{
	"musubi_fleet_maintenance": {"minutos": {1, fleet.MantenimientoMax.Minutes()}},
}

// llamadaDelRunbook es una llamada a una tool encontrada en el runbook, con su línea.
type llamadaDelRunbook struct {
	tool  string
	crudo string
	linea int
}

func llamadasDelRunbook(runbook string) []llamadaDelRunbook {
	var out []llamadaDelRunbook
	for n, l := range strings.Split(runbook, "\n") {
		for _, m := range reLlamadaATool.FindAllStringSubmatch(l, -1) {
			out = append(out, llamadaDelRunbook{tool: m[1], crudo: m[2], linea: n + 1})
		}
	}
	return out
}

// tipoCoincide dice si un valor decodificado de JSON tiene el tipo que el InputSchema declara. Un
// `"minutos":"30"` parsea como JSON perfectamente y el handler lo rechaza: por eso se mira el tipo
// además del nombre.
func tipoCoincide(tipo string, v interface{}) bool {
	switch tipo {
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "integer":
		f, ok := v.(float64)
		return ok && f == math.Trunc(f)
	case "number":
		_, ok := v.(float64)
		return ok
	case "array":
		_, ok := v.([]interface{})
		return ok
	case "object":
		_, ok := v.(map[string]interface{})
		return ok
	}
	return true // un tipo que esta guarda no conoce no se juzga
}

// TestLasLlamadasDelRunbookUsanLasToolsComoSon cruza cada llamada del runbook con el registro.
//
// Una llamada que nombra una tool registrada tiene que traer JSON válido, usar SÓLO parámetros que
// la tool declara, con el tipo que declara, dentro del rango que el handler acepta cuando hay uno
// conocido (rangosQueElHandlerExige), y traer los obligatorios. Y cada guion que el runbook manda a
// correr con una tool tiene que existir EN LA RUTA ESCRITA, y lo que se le pasa tiene que ser una
// tool registrada: ahí no hay ambigüedad posible con una serie.
//
// Lo que NO mira: la forma `tool clave=valor` (la de `musubi_fleet_approve` en
// `AprobacionDeCuatroOjosSinAtender`), que no es JSON y no hay cómo parsearla sin adivinar; ni los
// rangos que no están en el mapa.
//
// Sabotaje que la hace fallar: que la receta cierre la ventana con un parámetro que la tool no tiene.
// arnes: archivo="deploy/RUNBOOK.md"
// arnes: de="\"cancelar\":\"<id>\",\"project\":\"musubi\"}'"
// arnes: a="\"cancel\":\"<id>\",\"project\":\"musubi\"}'"
//
// Sabotaje que la hace fallar: que la receta mande a correr el guion con la ruta mal escrita.
// arnes: archivo="deploy/RUNBOOK.md"
// arnes: de="./deploy/musubi-tool.sh musubi_fleet_maintenance '{\"device\":\"gio\",\"minutos\""
// arnes: a="./deploy/musubi-tools.sh musubi_fleet_maintenance '{\"device\":\"gio\",\"minutos\""
//
// Sabotaje que la hace fallar: que la receta pida una ventana de cero minutos, que el handler rechaza.
// arnes: archivo="deploy/RUNBOOK.md"
// arnes: de="\"minutos\":30,"
// arnes: a="\"minutos\":0,"
func TestLasLlamadasDelRunbookUsanLasToolsComoSon(t *testing.T) {
	runbook := leerDeploy(t, "RUNBOOK.md")
	s := NewMcpServer(nil, "", nil)
	esquemas := map[string]InputSchema{}
	for i := range s.tools {
		esquemas[s.tools[i].Name] = s.tools[i].InputSchema
	}
	if len(esquemas) < 20 {
		t.Fatalf("el registro trae %d tools: el servidor de prueba no se armó y esta guarda no compararía contra nada", len(esquemas))
	}

	// EL GUION TIENE QUE EXISTIR EN LA RUTA QUE EL RUNBOOK ESCRIBE, no en alguna parte: la primera
	// versión miraba que `musubi-tool.sh` existiera si el nombre aparecía en cualquier lado, y una
	// receta con `./deploy/musubi-tools.sh` pasaba en verde y contestaba «No such file or directory».
	guiones := 0
	for n, l := range strings.Split(runbook, "\n") {
		for _, m := range reGuionConTool.FindAllStringSubmatch(l, -1) {
			guiones++
			if !guionDelRunbookExiste(m[1]) {
				t.Errorf("RUNBOOK.md:%d manda a correr %s y ese archivo no existe en el repo: quien copie la receta recibe «No such file or directory»", n+1, m[1])
			}
			if _, ok := esquemas[m[2]]; !ok {
				t.Errorf("RUNBOOK.md:%d le pasa %q a %s y esa tool no está registrada: la receta contesta «tool desconocida» en el peor momento", n+1, m[2], m[1])
			}
		}
	}
	if guiones < 3 {
		t.Fatalf("se reconocieron %d guiones llamados con una tool en el runbook y hay al menos tres: el patrón se rompió", guiones)
	}

	revisadas := 0
	for _, ll := range llamadasDelRunbook(runbook) {
		esquema, esTool := esquemas[ll.tool]
		if !esTool {
			continue // una serie o un nombre que no es de tool; si venía de musubi-tool.sh, ya se denunció arriba
		}
		revisadas++
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(ll.crudo), &args); err != nil {
			t.Errorf("RUNBOOK.md:%d llama a %s con argumentos que no son JSON (%v): %s", ll.linea, ll.tool, err, ll.crudo)
			continue
		}
		for clave, valor := range args {
			prop, declarada := esquema.Properties[clave]
			if !declarada {
				t.Errorf("RUNBOOK.md:%d le pasa `%s` a %s y la tool no declara ese parámetro: el handler lo descarta en silencio o contesta «argumentos inválidos». Parámetros que sí tiene: %v",
					ll.linea, clave, ll.tool, parametrosDe(esquema.Properties))
				continue
			}
			if !tipoCoincide(prop.Type, valor) {
				t.Errorf("RUNBOOK.md:%d le pasa `%s` a %s como %T y la tool lo declara %q", ll.linea, clave, ll.tool, valor, prop.Type)
				continue
			}
			if r, hay := rangosQueElHandlerExige[ll.tool][clave]; hay {
				if f, esNumero := valor.(float64); esNumero && (f < r[0] || f > r[1]) {
					t.Errorf("RUNBOOK.md:%d le pasa `%s`=%v a %s y el handler sólo acepta de %v a %v: la receta vuelve con error en mitad de la intervención",
						ll.linea, clave, valor, ll.tool, r[0], r[1])
				}
			}
		}
		for _, req := range esquema.Required {
			if _, ok := args[req]; !ok {
				t.Errorf("RUNBOOK.md:%d llama a %s sin `%s`, que es obligatorio", ll.linea, ll.tool, req)
			}
		}
	}
	// CONTROL DE QUE MIRÓ ALGO: si el patrón dejara de reconocer las llamadas, todo lo de arriba
	// pasaría en verde sin haber comprobado nada.
	if revisadas < 4 {
		t.Fatalf("se reconocieron %d llamadas con argumentos en el runbook y hay al menos cuatro: el patrón se rompió y la guarda dejó de mirar", revisadas)
	}
}

func parametrosDe(m map[string]Property) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var reNoAncla = regexp.MustCompile(`[^a-z0-9\s-]`)

// anclaDeTitulo devuelve el ancla de GitHub de un título, con la misma regla que usa
// TestCadaRunbookDeUnaAlertaApuntaAUnaSeccionQueExiste: minúsculas, sin lo que no sea
// alfanumérico, espacio o guion, y los espacios como guiones.
func anclaDeTitulo(titulo string) string {
	limpio := reNoAncla.ReplaceAllString(strings.ToLower(titulo), "")
	return strings.ReplaceAll(strings.TrimSpace(limpio), " ", "-")
}

// seccionDelRunbook devuelve el texto de la sección `## <titulo>` hasta el próximo `## `.
func seccionDelRunbook(runbook, titulo string) (string, bool) {
	cab := "\n## " + titulo + "\n"
	i := strings.Index(runbook, cab)
	if i < 0 {
		return "", false
	}
	resto := runbook[i+len(cab):]
	if j := strings.Index(resto, "\n## "); j >= 0 {
		resto = resto[:j]
	}
	return resto, true
}

// recetaDeLaVentana devuelve el título de la sección que ABRE una ventana con
// musubi_fleet_maintenance (`minutos`) y la CIERRA (`cancelar`), más lo que encontró de cada lado.
//
// La sección se reconoce por lo que HACE y no por su título: así renombrarla no deja a las guardas
// que la leen mirando a la nada, y la que tiene que denunciar el cambio es la del enlace.
func recetaDeLaVentana(runbook string) (titulo string, abre, cierra map[string]bool) {
	lineas := strings.Split(runbook, "\n")
	// La sección de cada línea es el último `## ` que la precede.
	tituloDeLinea := func(n int) string {
		for i := n - 1; i >= 0; i-- {
			if strings.HasPrefix(lineas[i], "## ") {
				return strings.TrimPrefix(lineas[i], "## ")
			}
		}
		return ""
	}
	abre, cierra = map[string]bool{}, map[string]bool{}
	for _, ll := range llamadasDelRunbook(runbook) {
		if ll.tool != "musubi_fleet_maintenance" {
			continue
		}
		var args map[string]interface{}
		if json.Unmarshal([]byte(ll.crudo), &args) != nil {
			continue // eso lo denuncia TestLasLlamadasDelRunbookUsanLasToolsComoSon
		}
		sec := tituloDeLinea(ll.linea - 1)
		if _, ok := args["minutos"]; ok {
			abre[sec] = true
		}
		if _, ok := args["cancelar"]; ok {
			cierra[sec] = true
		}
	}
	for sec := range abre {
		if cierra[sec] {
			titulo = sec
		}
	}
	return titulo, abre, cierra
}

// textoDeLaReceta devuelve el texto de la receta de la ventana, o corta la prueba: sin receta, la
// guarda que la pide es TestAntesDeTocarUnAgenteElRunbookDeclaraLaVentana, y las demás no tienen
// qué mirar.
func textoDeLaReceta(t *testing.T, runbook string) string {
	t.Helper()
	titulo, _, _ := recetaDeLaVentana(runbook)
	texto, ok := seccionDelRunbook(runbook, titulo)
	if titulo == "" || !ok {
		t.Fatal("no encontré la receta de la ventana (la sección que abre y cierra con musubi_fleet_maintenance): " +
			"eso lo denuncia TestAntesDeTocarUnAgenteElRunbookDeclaraLaVentana, y sin ella esta guarda no mira nada")
	}
	return texto
}

// TestAntesDeTocarUnAgenteElRunbookDeclaraLaVentana custodia la receta Y el camino hasta ella.
//
// La receta tiene que ABRIR la ventana (`minutos`) y CERRARLA (`cancelar`) en la misma sección:
// una ventana que se abre y no se cierra calla la máquina hasta que venza, y si la intervención
// salió mal ése es justo el rato en que el agente muerto no avisa. Y `AgenteCaidoConMaquinaViva`
// —la alerta que suena cuando esto se olvida— tiene que mandar a esa sección con un enlace que
// llegue: quien la lee es quien está por tocar el agente otra vez.
//
// Sabotaje que la hace fallar: renombrar la sección de la receta sin tocar el enlace que la cita.
// arnes: archivo="deploy/RUNBOOK.md"
// arnes: de="## Antes de tocar un agente a mano: declarar la ventana\n"
// arnes: a="## Antes de tocar el agente: declarar la ventana\n"
func TestAntesDeTocarUnAgenteElRunbookDeclaraLaVentana(t *testing.T) {
	runbook := leerDeploy(t, "RUNBOOK.md")
	receta, abre, cierra := recetaDeLaVentana(runbook)
	if receta == "" {
		t.Fatalf("el runbook no tiene una sección que abra una ventana con musubi_fleet_maintenance (`minutos`) Y la cierre (`cancelar`).\n"+
			"  Abren: %v · cierran: %v\n"+
			"  Sin la receta, tocar un agente a mano vuelve a sonar como una caída: pasó el 2026-09-20 en gio, 50 minutos.", abre, cierra)
	}

	caido, ok := seccionDelRunbook(runbook, "AgenteCaidoConMaquinaViva")
	if !ok {
		t.Fatal("no está la sección AgenteCaidoConMaquinaViva: la alerta la cita y esta guarda la necesita para mirar el enlace")
	}
	if want := "](#" + anclaDeTitulo(receta) + ")"; !strings.Contains(caido, want) {
		t.Errorf("AgenteCaidoConMaquinaViva no manda a la receta de la ventana (%q, esperaba un enlace %q).\n"+
			"  Es la alerta que suena cuando la ventana se olvidó: quien la lee es quien está por volver a tocar el agente.", receta, want)
	}

	// Y TODO enlace interno del runbook tiene que llegar a una sección. Un enlace roto no falla en
	// ningún lado: el navegador se queda arriba del archivo y el operador cree que la sección no existe.
	anclas := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^#{2,3}\s+(.+?)\s*$`).FindAllStringSubmatch(runbook, -1) {
		anclas[anclaDeTitulo(m[1])] = true
	}
	for _, m := range regexp.MustCompile(`\]\(#([^)\s]+)\)`).FindAllStringSubmatch(runbook, -1) {
		if !anclas[m[1]] {
			t.Errorf("el runbook enlaza a #%s y ninguna sección tiene esa ancla", m[1])
		}
	}
}

// reURLDelCerebro reconoce una URL del cerebro que el runbook manda a exportar.
var reURLDelCerebro = regexp.MustCompile("MUSUBI_CENTRAL_URL=([^\\s`'\"]+)")

// esURLConNombre dice si una URL es https contra un NOMBRE, que es la única forma en que el curl
// de musubi-tool.sh valida el certificado del tailnet.
func esURLConNombre(crudo string) bool {
	u, err := url.Parse(crudo)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && net.ParseIP(u.Hostname()) == nil
}

// TestLaRecetaDeLaVentanaDiceDondeSeCorre custodia que la receta no mande a un camino que no anda.
//
// La primera versión decía que el guion anda en cualquier lado con `MUSUBI_CENTRAL_URL` puesta, y
// en davantis-1 —la sala de mando, desde donde se opera— no arranca. Medido el 2026-09-25: el guion
// arma el cuerpo con `python3`, que ahí es el acceso directo de la Microsoft Store, y sale con 49
// antes de mandar nada; y aunque arrancara, le habla al cerebro con el `curl` que resuelva el PATH,
// que es el de MinGW y no ve la malla. Mientras el guion dependa de cualquiera de las dos cosas, la
// receta tiene que dar la otra vía, el MCP del cerebro, en el mismo párrafo que nombra davantis-1.
//
// Y ninguna URL del cerebro que el runbook mande a exportar puede ser https contra la IP pelada: el
// certificado del tailnet lleva el NOMBRE del nodo como único SAN y el guion no tiene `--resolve`,
// así que contra la IP el handshake falla y un cerebro vivo contesta 000 (medido desde la laptop:
// `curl: (35)`). La receta, además, tiene que dar la URL buena: la que trae el entorno de
// davantis-1 es justamente la de la IP.
//
// Sabotaje que la hace fallar: que la receta dé la URL del cerebro con la IP en vez del nombre.
// arnes: archivo="deploy/RUNBOOK.md"
// arnes: de="`MUSUBI_CENTRAL_URL=https://musubi-server.tail89e295.ts.net:10000`"
// arnes: a="`MUSUBI_CENTRAL_URL=https://100.79.126.62:10000`"
func TestLaRecetaDeLaVentanaDiceDondeSeCorre(t *testing.T) {
	runbook := leerDeploy(t, "RUNBOOK.md")
	receta := textoDeLaReceta(t, runbook)

	for n, l := range strings.Split(runbook, "\n") {
		for _, m := range reURLDelCerebro.FindAllStringSubmatch(l, -1) {
			u, err := url.Parse(m[1])
			if err != nil || u.Hostname() == "" {
				t.Errorf("RUNBOOK.md:%d exporta MUSUBI_CENTRAL_URL=%s y eso no es una URL", n+1, m[1])
				continue
			}
			if u.Scheme == "https" && !esURLConNombre(m[1]) {
				t.Errorf("RUNBOOK.md:%d exporta MUSUBI_CENTRAL_URL=%s: https contra la IP pelada. El certificado del "+
					"tailnet lleva el NOMBRE del nodo como único SAN y musubi-tool.sh no tiene --resolve: el "+
					"handshake falla y un cerebro vivo contesta 000", n+1, m[1])
			}
		}
	}
	conNombre := 0
	for _, m := range reURLDelCerebro.FindAllStringSubmatch(receta, -1) {
		if esURLConNombre(m[1]) {
			conNombre++
		}
	}
	if conNombre == 0 {
		t.Errorf("la receta de la ventana no dice a qué URL apuntar el guion (MUSUBI_CENTRAL_URL=https://<nombre>:10000).\n" +
			"  Sin eso se usa la del entorno, y la de davantis-1 es la IP pelada, contra la que el certificado no valida")
	}

	guion := leerDeploy(t, "musubi-tool.sh")
	var noAnda []string
	if strings.Contains(guion, "python3") {
		noAnda = append(noAnda, "arma el cuerpo con python3, que en davantis-1 es el acceso directo de la Store")
	}
	if !strings.Contains(guion, "--resolve") {
		noAnda = append(noAnda, "le habla al cerebro con curl sin --resolve")
	}
	if len(noAnda) == 0 {
		return // el guion ya anda en davantis-1: la otra vía deja de ser obligatoria
	}
	// Un párrafo o un ítem de lista: la vía tiene que estar AL LADO del nombre de la máquina, no
	// perdida en otra parte de la sección.
	var trozos []string
	for _, p := range strings.Split(receta, "\n\n") {
		trozos = append(trozos, strings.Split(p, "\n- ")...)
	}
	for _, tr := range trozos {
		if strings.Contains(tr, "davantis-1") && strings.Contains(tr, "MCP") {
			return
		}
	}
	t.Errorf("la receta de la ventana no dice que en davantis-1 se usa el MCP del cerebro, y el guion ahí no arranca: %s.\n"+
		"  Quien opera desde la sala de mando copia el paso 1 y la ventana no se declara: se repite el 2026-09-20",
		strings.Join(noAnda, "; "))
}

// reSesionConCuatroOjos reconoce, en el código del paquete, cada sesión que pasa por la puerta de
// cuatro ojos: la llamada que gasta la aprobación, con la capacidad que esa sesión pide.
var reSesionConCuatroOjos = regexp.MustCompile(`s\.gastarAprobacion\([^,]+,[^,]+,\s*fleet\.(Cap[A-Za-z]+)`)

// TestLaRecetaNombraCadaSesionQueFrenaCuatroOjos custodia que la receta no deje afuera una sesión
// que cuatro ojos frena.
//
// La primera versión hablaba sólo de `musubi_fleet_screen`, y cuatro ojos frena también
// `musubi_fleet_shell`. No es un detalle de redacción: quien aprueba necesita LA MISMA capacidad
// que la sesión pide, y medido el 2026-09-25 en `gio` el principal de meir tiene `screen` y no
// `shell`, así que una shell ahí no la puede aprobar nadie. El operador que la pide siguiendo la
// receta le avisa a meir, meir no puede aprobar, la solicitud vence a los 30 minutos y la ventana
// ya declarada se consume sin trabajo.
//
// LA LISTA SALE DEL CÓDIGO, no se escribe a mano: cada `gastarAprobacion` del paquete es una sesión
// que pasa por la puerta. Si mañana otra pasa, esta guarda pide que la receta la nombre. Lo que NO
// puede mirar es quién tiene cada capacidad en producción: eso vive en el `principals.yaml` del
// central, no en el repo, y la receta lo da con fecha de medición. Tampoco custodia la redacción
// del encabezado («En `gio` rige cuatro ojos…»): lo que tiene dientes es la enumeración.
//
// Sabotaje que la hace fallar: que la receta deje de nombrar la shell entre las sesiones frenadas.
// arnes: archivo="deploy/RUNBOOK.md"
// arnes: de="Cuatro ojos frena las dos sesiones, `musubi_fleet_screen` y `musubi_fleet_shell`, y quien"
// arnes: a="Cuatro ojos frena las sesiones de pantalla, `musubi_fleet_screen`, y quien"
func TestLaRecetaNombraCadaSesionQueFrenaCuatroOjos(t *testing.T) {
	receta := textoDeLaReceta(t, leerDeploy(t, "RUNBOOK.md"))

	s := NewMcpServer(nil, "", nil)
	registradas := map[string]bool{}
	for i := range s.tools {
		registradas[s.tools[i].Name] = true
	}
	capacidades := map[string]fleet.Cap{
		"CapMetrics": fleet.CapMetrics, "CapExec": fleet.CapExec,
		"CapScreen": fleet.CapScreen, "CapShell": fleet.CapShell,
	}

	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no pude listar el paquete: %v", err)
	}
	frenadas := map[string]bool{}
	for _, e := range entradas {
		nombre := e.Name()
		if e.IsDir() || !strings.HasSuffix(nombre, ".go") || strings.HasSuffix(nombre, "_test.go") {
			continue
		}
		fuente, err := os.ReadFile(nombre)
		if err != nil {
			t.Fatalf("no pude leer %s: %v", nombre, err)
		}
		for _, l := range strings.Split(string(fuente), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "//") {
				continue // un comentario que cita la llamada no es una sesión
			}
			for _, m := range reSesionConCuatroOjos.FindAllStringSubmatch(l, -1) {
				c, conocida := capacidades[m[1]]
				if !conocida {
					t.Errorf("%s pasa fleet.%s por cuatro ojos y esta guarda no sabe qué capacidad es: agregala al mapa", nombre, m[1])
					continue
				}
				tool := "musubi_fleet_" + string(c)
				if !registradas[tool] {
					t.Errorf("%s pasa `%s` por cuatro ojos y no hay una tool %s: la guarda no sabe qué sesión nombrar", nombre, c, tool)
					continue
				}
				frenadas[tool] = true
			}
		}
	}
	// CONTROL DE QUE MIRÓ ALGO: hoy son dos (pantalla y shell). Menos es un patrón que dejó de
	// reconocer la llamada, no una puerta que dejó de existir.
	if len(frenadas) < 2 {
		t.Fatalf("encontré %d sesiones que pasan por cuatro ojos y hay al menos dos (%v): el patrón dejó de reconocer gastarAprobacion", len(frenadas), frenadas)
	}

	// SE MIRA LA ORACIÓN QUE LAS ENUMERA, no la sección entera: la receta nombra también
	// `musubi_fleet_exec`, justamente para decir que cuatro ojos NO la cubre. Buscando el nombre en
	// cualquier lado, una `exec` que pasara a ir por la puerta seguiría en verde. Y se mira en los
	// dos sentidos: una sesión que la oración diera por frenada y el código no, también miente.
	oracion := reOracionDeCuatroOjos.FindString(receta)
	if oracion == "" {
		t.Fatalf("la receta de la ventana no tiene la oración que enumera las sesiones que frena cuatro ojos " +
			"(«Cuatro ojos frena …»), y en gio rige desde el 2026-09-24")
	}
	enLaOracion := map[string]bool{}
	for _, m := range reToolEntreComillas.FindAllStringSubmatch(oracion, -1) {
		enLaOracion[m[1]] = true
	}
	for tool := range frenadas {
		if !enLaOracion[tool] {
			t.Errorf("la receta no nombra `%s` entre las sesiones que frena cuatro ojos, y el código la frena: quien "+
				"aprueba necesita LA MISMA capacidad, así que la receta tiene que decir si esa sesión hoy tiene quién la apruebe", tool)
		}
	}
	for tool := range enLaOracion {
		if !frenadas[tool] {
			t.Errorf("la receta da `%s` como frenada por cuatro ojos y el código no la pasa por la puerta", tool)
		}
	}
}

// reOracionDeCuatroOjos es la oración de la receta que enumera las sesiones frenadas, hasta el punto.
var reOracionDeCuatroOjos = regexp.MustCompile(`(?i)cuatro ojos frena[^.]*`)

// reToolEntreComillas es una tool de flota citada como código en el texto del runbook.
var reToolEntreComillas = regexp.MustCompile("`(musubi_fleet_[a-z_]+)`")
