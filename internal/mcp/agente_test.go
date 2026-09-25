package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory/memtest"
)

// handshakeYCatalogo devuelve el initialize y el tools/list TAL COMO SALEN POR EL CABLE: se
// serializan y se vuelven a leer, porque lo que el agente recibe es el JSON y no el mapa de Go. Un
// `_meta` con el tag mal escrito, o un campo que omitempty se come, sólo se ve de este lado.
func handshakeYCatalogo(t *testing.T, s *McpServer) (map[string]interface{}, []map[string]interface{}) {
	t.Helper()
	var init map[string]interface{}
	crudo, err := json.Marshal(s.handleInitialize())
	if err != nil {
		t.Fatalf("serializar initialize: %v", err)
	}
	if err := json.Unmarshal(crudo, &init); err != nil {
		t.Fatalf("releer initialize: %v", err)
	}
	var lista struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	crudo, err = json.Marshal(s.handleToolsList())
	if err != nil {
		t.Fatalf("serializar tools/list: %v", err)
	}
	if err := json.Unmarshal(crudo, &lista); err != nil {
		t.Fatalf("releer tools/list: %v", err)
	}
	return init, lista.Tools
}

// cargadas devuelve los nombres de las tools que el catálogo marca para llegar cargadas.
func cargadas(tools []map[string]interface{}) map[string]bool {
	out := map[string]bool{}
	for _, tl := range tools {
		meta, _ := tl["_meta"].(map[string]interface{})
		if v, _ := meta[metaAlwaysLoad].(bool); v {
			out[tl["name"].(string)] = true
		}
	}
	return out
}

func servidorQueLeHablaAlAgente(t *testing.T) *McpServer {
	t.Helper()
	return NewMcpServer(memtest.NuevoEngine(t, t.TempDir()), t.TempDir(), embedding.NoopProvider{}, WithInstruccionesParaElAgente())
}

// EL DAEMON LE HABLA AL AGENTE, Y EL CENTRAL NO.
//
// Las dos mitades importan igual. La primera es el cambio: sin instrucciones ni tools cargadas, el
// agente no sabe que Musubi existe (ver agente.go). La segunda es lo que el cambio NO puede romper:
// el central comparte este handshake y `musubi cerebro` se lo reenvía al agente tal cual, así que
// si el central hablara, una sesión con los dos servidores —Altura— recibiría todo dos veces.
//
// Sabotaje que la hace fallar: que el central también hable.
// arnes: archivo="internal/mcp/agente.go"
// arnes: de="\tif !s.hablaAlAgente {"
// arnes: a="\tif false {"
//
// Sabotaje que la hace fallar: mandar la clave vacía en vez de no mandarla.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="if texto := s.instruccionesParaElAgente(); texto != \"\" {"
// arnes: a="if texto := s.instruccionesParaElAgente(); true {"
//
// Sabotaje que la hace fallar: listar el núcleo sin la marca.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="\t\tif nucleo[t.Name] {"
// arnes: a="\t\tif nucleo[t.Name] && false {"
func TestElDaemonLeHablaAlAgenteYElCentralNo(t *testing.T) {
	init, tools := handshakeYCatalogo(t, servidorQueLeHablaAlAgente(t))
	texto, _ := init["instructions"].(string)
	if !strings.Contains(texto, "musubi_recall") {
		t.Fatalf("el daemon no le dio instrucciones al agente: instructions=%q", texto)
	}
	marcadas := cargadas(tools)
	if len(marcadas) == 0 {
		t.Fatal("el daemon no marcó ninguna tool para llegar cargada: el agente las recibe todas diferidas")
	}
	if len(marcadas) == len(tools) {
		t.Fatalf("el daemon marcó las %d tools: eso es el catálogo entero en cada sesión, no un núcleo", len(tools))
	}

	central := newTestServer(t, embedding.NoopProvider{})
	init, tools = handshakeYCatalogo(t, central)
	if _, habla := init["instructions"]; habla {
		t.Errorf("el central mandó instrucciones (%q): el relé se las reenvía al agente y le llegan dos veces", init["instructions"])
	}
	if m := cargadas(tools); len(m) != 0 {
		t.Errorf("el central marcó tools para llegar cargadas (%v): con el relé, el agente recibe dos juegos", nombresOrdenados(m))
	}
	for _, tl := range tools {
		if _, tiene := tl["_meta"]; tiene {
			t.Errorf("el central sirvió %s con `_meta`: su catálogo tiene que salir como antes", tl["name"])
		}
	}
}

// LAS INSTRUCCIONES NOMBRAN SÓLO TOOLS QUE EL AGENTE PUEDE LLAMAR, Y ENTRAN ENTERAS.
//
// Nombrar una tool que no está en tools/list es un callejón: medido sobre los transcripts, los
// avisos que nombraban tools dormidas se siguieron CERO veces, y el modelo ni siquiera intentó
// cargarlas. Y lo que pase de topeInstrucciones Claude Code lo corta sin que el servidor se entere.
//
// Sabotaje que la hace fallar: nombrar una tool dormida.
// arnes: archivo="internal/mcp/agente.go"
// arnes: de="dalo con musubi_judge."
// arnes: a="dalo con musubi_detect_stack."
//
// Sabotaje que la hace fallar: un texto que no entra (equivale a bajar el tope por debajo de él).
// arnes: archivo="internal/mcp/agente.go"
// arnes: de="const topeInstrucciones = 2048"
// arnes: a="const topeInstrucciones = 1000"
func TestLasInstruccionesNombranToolsVisiblesYEntranEnteras(t *testing.T) {
	s := servidorQueLeHablaAlAgente(t)
	texto := s.instruccionesParaElAgente()
	if n := largoParaElCliente(texto); n > topeInstrucciones {
		t.Errorf("las instrucciones miden %d unidades UTF-16 y Claude Code corta en %d: el final no le llega al agente", n, topeInstrucciones)
	}

	_, tools := handshakeYCatalogo(t, s)
	visibles := map[string]bool{}
	for _, tl := range tools {
		visibles[tl["name"].(string)] = true
	}
	nombradas := nombreDeTool.FindAllString(texto, -1)
	if len(nombradas) == 0 {
		t.Fatal("las instrucciones no nombran ninguna tool: un texto así no mueve al agente")
	}
	for _, n := range nombradas {
		if !visibles[n] {
			t.Errorf("las instrucciones nombran %s, que no está en tools/list: el agente no la puede llamar", n)
		}
	}
}

// techoDelNucleo es cuánto puede pesar el núcleo, en caracteres del JSON compacto que viaja: unos
// 2.000 tokens por sesión. NO ES UN DERIVADO: es un presupuesto, y subirlo es una decisión — cada
// tool que las instrucciones nombran se paga en todas las sesiones de todos los repos.
const techoDelNucleo = 8000

// EL NÚCLEO ES LO QUE LAS INSTRUCCIONES NOMBRAN, Y CABE EN SU TECHO.
//
// Se mide contra el catálogo servido y no contra nucleoDelAgente: lo que importa es qué recibe el
// agente. Las dos direcciones: una tool nombrada que llega diferida se sigue 5 a 6 veces menos, y
// una tool cargada que nada nombra cuesta tokens en cada sesión sin que nadie la señale.
//
// Sabotaje que la hace fallar: nombrar una tool pesada más.
// arnes: archivo="internal/mcp/agente.go"
// arnes: de="se abre con musubi_memory_expand."
// arnes: a="se abre con musubi_memory_expand; una cola de pendientes, con musubi_conflicts."
func TestElNucleoEsLoQueLasInstruccionesNombranYCabe(t *testing.T) {
	s := servidorQueLeHablaAlAgente(t)
	_, tools := handshakeYCatalogo(t, s)
	marcadas := cargadas(tools)

	nombradas := map[string]bool{}
	for _, n := range nombreDeTool.FindAllString(s.instruccionesParaElAgente(), -1) {
		nombradas[n] = true
	}
	for n := range nombradas {
		if !marcadas[n] {
			t.Errorf("las instrucciones nombran %s y el catálogo la sirve diferida", n)
		}
	}
	for n := range marcadas {
		if !nombradas[n] {
			t.Errorf("%s llega cargada y las instrucciones no la nombran: cuesta tokens sin que nada la señale", n)
		}
	}

	peso := 0
	for _, tl := range tools {
		if marcadas[tl["name"].(string)] {
			crudo, err := json.Marshal(tl)
			if err != nil {
				t.Fatal(err)
			}
			peso += len([]rune(string(crudo)))
		}
	}
	if peso > techoDelNucleo {
		t.Errorf("el núcleo pesa %d caracteres y el techo es %d: son tokens en cada sesión (núcleo: %v)", peso, techoDelNucleo, nombresOrdenados(marcadas))
	}
	t.Logf("núcleo: %d tools, %d caracteres de %d", len(marcadas), peso, techoDelNucleo)
}

// EL TOPE ES EL DEL CLIENTE, Y SE MIDE COMO LO MIDE EL CLIENTE.
//
// 2048 es un hecho del binario de Claude Code (lo corta y avisa «Server instructions truncated»),
// así que se clava acá en vez de derivarse: derivarlo de la constante que esta prueba custodia la
// dejaría midiéndose a sí misma. Y el largo va en UTF-16 porque el cliente es JavaScript: una letra
// fuera del plano básico son dos unidades, aunque sea una runa.
//
// Sabotaje que la hace fallar: medir runas en vez de unidades UTF-16.
// arnes: archivo="internal/mcp/agente.go"
// arnes: de="\treturn len(utf16.Encode([]rune(texto)))"
// arnes: a="\treturn len([]rune(texto)) + 0*len(utf16.Encode(nil))"
func TestElTopeEsElDelCliente(t *testing.T) {
	if topeInstrucciones != 2048 {
		t.Errorf("topeInstrucciones = %d; Claude Code corta las instrucciones en 2048", topeInstrucciones)
	}
	if n := largoParaElCliente("a😀"); n != 3 {
		t.Errorf("«a😀» mide %d para el cliente y son 3 unidades UTF-16", n)
	}
}
