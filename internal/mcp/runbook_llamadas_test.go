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
	"regexp"
	"sort"
	"strings"
	"testing"
)

// reLlamadaATool reconoce una llamada CON ARGUMENTOS en una sola línea, en las dos formas que usa
// el runbook: `./deploy/musubi-tool.sh <tool> '<json>'` y la abreviada `<tool> {json}`.
//
// Exige espacio entre el nombre y la llave a propósito: una serie de Prometheus con etiquetas
// (`musubi_fleet_net_up{device="…"}`) va pegada, y no es una llamada.
var reLlamadaATool = regexp.MustCompile("\\b(musubi_[a-z_]+)[ \\t]+'?(\\{[^'\\n`]*\\})")

// reToolDelGuion reconoce la tool que se le pasa a musubi-tool.sh, lleve argumentos o no.
var reToolDelGuion = regexp.MustCompile(`musubi-tool\.sh[ \t]+(musubi_[a-z_]+)`)

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
// la tool declara, con el tipo que declara, y traer los obligatorios. Y todo lo que se le pasa a
// `musubi-tool.sh` tiene que ser una tool registrada: ahí no hay ambigüedad posible con una serie.
//
// Lo que NO mira: la forma `tool clave=valor` (la de `musubi_fleet_approve` en
// `AprobacionDeCuatroOjosSinAtender`), que no es JSON y no hay cómo parsearla sin adivinar.
//
// Sabotaje que la hace fallar: que la receta cierre la ventana con un parámetro que la tool no tiene.
// arnes: archivo="deploy/RUNBOOK.md"
// arnes: de="\"cancelar\":\"<id>\",\"project\":\"musubi\"}'"
// arnes: a="\"cancel\":\"<id>\",\"project\":\"musubi\"}'"
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

	// EL GUION TIENE QUE EXISTIR si el runbook lo manda a correr. leerDeploy corta la prueba si no está.
	if strings.Contains(runbook, "musubi-tool.sh") {
		_ = leerDeploy(t, "musubi-tool.sh")
	}
	for n, l := range strings.Split(runbook, "\n") {
		for _, m := range reToolDelGuion.FindAllStringSubmatch(l, -1) {
			if _, ok := esquemas[m[1]]; !ok {
				t.Errorf("RUNBOOK.md:%d le pasa %q a musubi-tool.sh y esa tool no está registrada: la receta contesta «tool desconocida» en el peor momento", n+1, m[1])
			}
		}
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
	abre, cierra := map[string]bool{}, map[string]bool{}
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
	var receta string
	for sec := range abre {
		if cierra[sec] {
			receta = sec
		}
	}
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
