package motor

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// LOS FIXTURES SON JSON CRUDO, NO STRUCTS DE GO, y es deliberado: lo que llega por el cable
// es texto, y la mitad de los invariantes de este archivo viven en los UnmarshalJSON a mano
// (el `content` que es string o arreglo, el `sections` que conserva el orden, el `null` que
// borra). Armar el pedido con literales de Go saltearía justo esa capa, y el test quedaría
// midiendo la traducción de algo que el servidor nunca va a recibir así.
func desdeJSON(t *testing.T, cuerpo string) Pedido {
	t.Helper()
	var p PedidoProxy
	if err := json.Unmarshal([]byte(cuerpo), &p); err != nil {
		t.Fatalf("el fixture no es JSON válido: %v", err)
	}
	ped, err := PedidoDesdeProxy(p)
	if err != nil {
		t.Fatalf("PedidoDesdeProxy: %v", err)
	}
	return ped
}

func errorDesdeJSON(t *testing.T, cuerpo string) string {
	t.Helper()
	var p PedidoProxy
	if err := json.Unmarshal([]byte(cuerpo), &p); err != nil {
		t.Fatalf("el fixture no es JSON válido: %v", err)
	}
	_, err := PedidoDesdeProxy(p)
	if err == nil {
		t.Fatal("se esperaba un error y la traducción pasó")
	}
	return err.Error()
}

// mismoJSON compara por VALOR y no por texto. `encoding/json` serializa las claves de un
// mapa en orden alfabético, así que comparar strings ataría el test a ese detalle: agregar
// un campo opcional rompería aserciones de bloques que no cambiaron.
func mismoJSON(t *testing.T, tiene json.RawMessage, quiero string) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(tiene, &a); err != nil {
		t.Fatalf("el bloque traducido no es JSON: %v (%s)", err, tiene)
	}
	if err := json.Unmarshal([]byte(quiero), &b); err != nil {
		t.Fatalf("el bloque esperado no es JSON: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("bloque distinto\n tiene: %s\nquiero: %s", tiene, quiero)
	}
}

// ─────────────────────────────── validación del pedido ───────────────────────────────

// Sabotaje E1: el id del modelo no se valida (sale un request sin modelo)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tif modelo == \"\" {\n\t\treturn Pedido{}, errors.New(\"el pedido no trae model.id: sin modelo no hay turno que servir\")\n\t}"
// arnes: a=""
//
// Sabotaje E2: un transcript sin mensajes pasa (la API lo rechaza con un 400)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tif len(p.Context.Messages) == 0 {\n\t\treturn Pedido{}, errors.New(\"el pedido no trae context.messages\")\n\t}"
// arnes: a=""
//
// Sabotaje E3: un transcript de puro system pasa sin mensajes de conversacion
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tif len(mensajes) == 0 {"
// arnes: a="\tif false {"
//
// Sabotaje E23: un rol desconocido se saltea en vez de ser error
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\tdefault:\n\t\t\t// NO SE SALTEA, Y ES A PROPÓSITO. pi admite roles propios en los transcripts de"
// arnes: a="\t\tcase \"rol-que-no-existe-y-nunca-llega\":\n\t\t\t// NO SE SALTEA, Y ES A PROPÓSITO. pi admite roles propios en los transcripts de"
func TestElPedidoDelProxySeValidaAntesDeTraducir(t *testing.T) {
	casos := []struct {
		nombre string
		cuerpo string
		busca  string
	}{
		{
			// Sin id de modelo no hay turno que servir, y el error tiene que decirlo acá:
			// más adelante lo diría el proveedor, con un 400 que el cliente no puede leer.
			"sin model.id",
			`{"model":{},"context":{"messages":[{"role":"user","content":"hola"}]}}`,
			"model.id",
		},
		{
			"sin mensajes",
			`{"model":{"id":"claude-opus-5"},"context":{"messages":[]}}`,
			"context.messages",
		},
		{
			// Un transcript que sólo trae system no es un turno: no hay nada que responder.
			"sólo mensajes system",
			`{"model":{"id":"claude-opus-5"},"context":{"messages":[
				{"role":"system","content":"sos un asistente"}]}}`,
			"ningún turno",
		},
		{
			// Un user vacío no se puede mandar (la API pide `content` no vacío) y tampoco
			// aporta: se descarta, y si era el único mensaje el pedido queda sin turnos.
			"el único turno está vacío",
			`{"model":{"id":"claude-opus-5"},"context":{"messages":[
				{"role":"user","content":""}]}}`,
			"ningún turno",
		},
		{
			// 🔴 NO SE SALTEA EN SILENCIO. Descartar un mensaje de rol desconocido le
			// cambia el contexto al modelo sin dejar rastro: es la clase de falla que se
			// lee como «el modelo se olvidó de lo que le dije».
			"rol desconocido",
			`{"model":{"id":"claude-opus-5"},"context":{"messages":[
				{"role":"user","content":"hola"},
				{"role":"inventado","content":"qué"}]}}`,
			`rol desconocido "inventado"`,
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if msg := errorDesdeJSON(t, c.cuerpo); !strings.Contains(msg, c.busca) {
				t.Errorf("el error no nombra %q: %s", c.busca, msg)
			}
		})
	}
}

// Sabotaje E4: ModeloResuelto queda vacio (el CLI descarta el thinking firmado)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\tModeloResuelto: modelo,"
// arnes: a="\t\tModeloResuelto: \"\","
func TestElIdDelModeloViajaEnLosDosCamposDelPedido(t *testing.T) {
	p := desdeJSON(t, `{"model":{"id":"claude-opus-5"},"context":{"messages":[
		{"role":"user","content":"hola"}]}}`)

	// LOS DOS CAMPOS, Y NO ES REDUNDANCIA: `ModeloResuelto` es lo que el Lanzador escribe
	// en los turnos del asistente, y si queda vacío el CLI descarta los bloques de thinking
	// firmados sin decir nada. Los modelos de pi ya traen el id concreto, así que el
	// traductor es el único lugar donde esto se puede declarar.
	if p.Modelo != "claude-opus-5" {
		t.Errorf("Modelo = %q", p.Modelo)
	}
	if p.ModeloResuelto != "claude-opus-5" {
		t.Errorf("ModeloResuelto = %q, y sin él el thinking firmado se pierde", p.ModeloResuelto)
	}
}

// Sabotaje E5: options.maxTokens no le gana al del modelo
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\ttope := p.Options.MaxTokens\n\tif tope <= 0 {\n\t\ttope = p.Model.MaxTokens\n\t}"
// arnes: a="\ttope := p.Model.MaxTokens"
func TestElTopeDeTokensDelPedidoLeGanaAlDelModelo(t *testing.T) {
	// El `maxTokens` del modelo es su techo; el de las opciones es lo que pidió este turno.
	// Si gana el del modelo, un cliente que acota la respuesta a 500 tokens recibe 64000 y
	// paga la diferencia.
	conAmbos := desdeJSON(t, `{"model":{"id":"m","maxTokens":64000},
		"context":{"messages":[{"role":"user","content":"hola"}]},
		"options":{"maxTokens":500}}`)
	if conAmbos.MaxTokens != 500 {
		t.Errorf("con options.maxTokens: MaxTokens = %d, quiero 500", conAmbos.MaxTokens)
	}

	soloModelo := desdeJSON(t, `{"model":{"id":"m","maxTokens":8192},
		"context":{"messages":[{"role":"user","content":"hola"}]}}`)
	if soloModelo.MaxTokens != 8192 {
		t.Errorf("sin options.maxTokens: MaxTokens = %d, quiero el del modelo (8192)", soloModelo.MaxTokens)
	}
}

// ─────────────────────────────── el system prompt ───────────────────────────────

// Sabotaje E6: los bloques de UN system se juntan con el separador de ENTRE mensajes
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\tif t := m.Content.texto(\"\\n\"); t != \"\" {"
// arnes: a="\t\tif t := m.Content.texto(\"\\n\\n\"); t != \"\" {"
//
// Sabotaje E7: los mensajes system se juntan con el separador de DENTRO de un mensaje
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tprompt = strings.Join(limpias, \"\\n\\n\")"
// arnes: a="\tprompt = strings.Join(limpias, \"\\n\")"
func TestElSystemPromptSeReproduceComoLoHacePi(t *testing.T) {
	// DOS SEPARADORES DISTINTOS EN EL MISMO PROMPT, y confundirlos cambia el texto que ve
	// el modelo: los bloques de texto DENTRO de un mensaje se juntan con "\n"
	// (`contentText`, text.ts:6) y los mensajes ENTRE sí con "\n\n"
	// (`getCurrentSystemMessage`, transcript.ts:84).
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":[{"type":"text","text":"uno"},{"type":"text","text":"dos"}]},
		{"role":"system","content":"tres"},
		{"role":"user","content":"hola"}]}}`)

	quiero := "uno\ndos\n\ntres"
	if p.SystemPrompt != quiero {
		t.Errorf("SystemPrompt = %q, quiero %q", p.SystemPrompt, quiero)
	}
}

// Sabotaje E8: una seccion vacia deja su hueco en el prompt
// Se pisa con E9 porque el `de` de E9 es la línea `for` que ésta contiene, y son dos invariantes
// del mismo bucle: ésta mide el FILTRO del vacío, E9 mide el ORDEN.
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tfor _, p := range append(textos, secciones.valores()...) {\n\t\tif p != \"\" {\n\t\t\tlimpias = append(limpias, p)\n\t\t}\n\t}"
// arnes: a="\tlimpias = append(textos, secciones.valores()...)"
// arnes: colision_ok="TestLasSeccionesVanDespuesDelContenidoYRespetanElOrdenDeMap"
func TestUnSystemSinTextoNoDejaUnHuecoEnElPrompt(t *testing.T) {
	// Un mensaje system vacío —o uno que sólo trae tools— no aporta texto. Sin el filtro
	// dejaría un "\n\n" de más, y el prompt llegaría con una línea en blanco donde el
	// cliente no puso ninguna.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"arriba"},
		{"role":"system","content":"","toolsAdded":[{"name":"t","parameters":{}}]},
		{"role":"system","content":"abajo"},
		{"role":"user","content":"hola"}]}}`)

	if p.SystemPrompt != "arriba\n\nabajo" {
		t.Errorf("SystemPrompt = %q", p.SystemPrompt)
	}

	// 🔴 Y EL OTRO CAMINO AL MISMO HUECO, que el sabotaje descubrió sin test: lo de arriba lo
	// atrapa el filtro de la ENTRADA (`if t := m.Content.texto("\n"); t != ""`), así que el
	// filtro de la SALIDA quedaba sin medir — y es el único que cubre esto, porque una SECCIÓN
	// puede quedar en "" sin pasar por ahí. Son dos filtros, no uno repetido, y el de la salida
	// se podía borrar entero con la suite en verde.
	q := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"arriba","sections":{"a":"A"}},
		{"role":"system","content":"","sections":{"a":""}},
		{"role":"user","content":"hola"}]}}`)

	if q.SystemPrompt != "arriba" {
		t.Errorf("con una sección vaciada: SystemPrompt = %q, quiero \"arriba\" sin hueco", q.SystemPrompt)
	}
}

// Sabotaje E9: las secciones van ANTES del contenido (orden al reves de pi)
// Se pisa con E8, que sabotea el bloque entero: ver su ancla. Ésta mide sólo el ORDEN.
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tfor _, p := range append(textos, secciones.valores()...) {"
// arnes: a="\tfor _, p := range append(secciones.valores(), textos...) {"
// arnes: colision_ok="TestUnSystemSinTextoNoDejaUnHuecoEnElPrompt"
func TestLasSeccionesVanDespuesDelContenidoYRespetanElOrdenDeMap(t *testing.T) {
	// 🔴 ESTE ES EL INVARIANTE QUE UN map[string]string DE GO NO PUEDE CUMPLIR. pi parchea
	// las secciones sobre un `Map` de JavaScript (transcript.ts:75), donde re-asignar una
	// clave que YA EXISTE conserva su posición. Con un mapa de Go el orden sale distinto en
	// cada corrida, y eso se ve como un modelo que «responde distinto a veces».
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"base","sections":{"a":"A1","b":"B1","c":"C1"}},
		{"role":"system","content":"","sections":{"a":"A2"}},
		{"role":"user","content":"hola"}]}}`)

	// "a" se redefinió y SIGUE PRIMERA; las secciones van después del contenido.
	quiero := "base\n\nA2\n\nB1\n\nC1"
	if p.SystemPrompt != quiero {
		t.Errorf("SystemPrompt = %q, quiero %q", p.SystemPrompt, quiero)
	}
}

// Sabotaje E10: el null de una seccion no la borra (queda el texto viejo)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\t\tif s.Borrar {\n\t\t\t\t\tsecciones.borrar(s.Nombre)\n\t\t\t\t\tcontinue\n\t\t\t\t}"
// arnes: a=""
func TestUnaSeccionEnNullSeBorraYAlVolverVaAlFinal(t *testing.T) {
	// El `null` del protocolo QUITA la sección (types.ts:564). Es un tercer estado: ausente,
	// con texto, y presente-en-null. Tratarlo como string vacío la dejaría en su lugar.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"base","sections":{"a":"A","b":"B"}},
		{"role":"system","content":"","sections":{"a":null}},
		{"role":"user","content":"hola"}]}}`)
	if p.SystemPrompt != "base\n\nB" {
		t.Errorf("con el borrado: SystemPrompt = %q, quiero \"base\\n\\nB\"", p.SystemPrompt)
	}

	// Y la otra mitad de la semántica de Map: borrarla y volver a ponerla la manda AL FINAL,
	// no a su posición original.
	q := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"base","sections":{"a":"A","b":"B"}},
		{"role":"system","content":"","sections":{"a":null}},
		{"role":"system","content":"","sections":{"a":"A2"}},
		{"role":"user","content":"hola"}]}}`)
	if q.SystemPrompt != "base\n\nB\n\nA2" {
		t.Errorf("al reponerla: SystemPrompt = %q, quiero \"base\\n\\nB\\n\\nA2\"", q.SystemPrompt)
	}
}

// ─────────────────────────────── las tools ───────────────────────────────

func TestLasToolsVigentesSalenDelReplayDeLosSystem(t *testing.T) {
	// `parameters` → `input_schema`. Es el renombre que, mal hecho, hace que la API rechace
	// el request entero hablando de un campo que el cliente nunca escribió.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"x","toolsAdded":[
			{"name":"leer","description":"lee un archivo","parameters":{"type":"object","properties":{"ruta":{"type":"string"}}}}]},
		{"role":"user","content":"hola"}]}}`)

	mismoJSON(t, p.Tools, `[{"name":"leer","description":"lee un archivo",
		"input_schema":{"type":"object","properties":{"ruta":{"type":"string"}}}}]`)
}

// Sabotaje E11: las tools AGREGADAS se procesan antes que las QUITADAS
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\tfor _, t := range m.ToolsRemoved {\n\t\t\t\tdeclaradas.borrar(t.Name)\n\t\t\t}\n\t\t\tfor _, t := range m.ToolsAdded {\n\t\t\t\tdeclaradas.poner(t.Name, t)\n\t\t\t}"
// arnes: a="\t\t\tfor _, t := range m.ToolsAdded {\n\t\t\t\tdeclaradas.poner(t.Name, t)\n\t\t\t}\n\t\t\tfor _, t := range m.ToolsRemoved {\n\t\t\t\tdeclaradas.borrar(t.Name)\n\t\t\t}"
func TestLasToolsQuitadasSeProcesanAntesQueLasAgregadas(t *testing.T) {
	// EL ORDEN ES EL DE pi (`getCurrentTools`, transcript.ts:62-63). Un mensaje que REDEFINE
	// una tool la nombra en las dos listas; al revés, el delete correría después del set y
	// la tool se perdería justo cuando se la estaba actualizando.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"x","toolsAdded":[{"name":"leer","description":"v1","parameters":{}}]},
		{"role":"system","content":"","toolsRemoved":[{"name":"leer"}],
			"toolsAdded":[{"name":"leer","description":"v2","parameters":{}}]},
		{"role":"user","content":"hola"}]}}`)

	mismoJSON(t, p.Tools, `[{"name":"leer","description":"v2","input_schema":{}}]`)
}

// Sabotaje E12: re-poner una clave la manda al final (no es el Map de JS)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="func (m *mapaOrdenado[T]) poner(k string, v T) {\n\tif _, existe := m.porClave[k]; !existe {\n\t\tm.orden = append(m.orden, k)\n\t}\n\tm.porClave[k] = v\n}"
// arnes: a="func (m *mapaOrdenado[T]) poner(k string, v T) {\n\tm.borrar(k)\n\tm.orden = append(m.orden, k)\n\tm.porClave[k] = v\n}"
func TestUnaToolRedefinidaConservaSuPosicion(t *testing.T) {
	// Mismo `Map` de JS que las secciones, aplicado al set de tools (transcript.ts:59).
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"x","toolsAdded":[
			{"name":"a","description":"a1","parameters":{}},
			{"name":"b","description":"b1","parameters":{}}]},
		{"role":"system","content":"","toolsAdded":[{"name":"a","description":"a2","parameters":{}}]},
		{"role":"user","content":"hola"}]}}`)

	var ts []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(p.Tools, &ts); err != nil {
		t.Fatalf("Tools: %v", err)
	}
	if len(ts) != 2 || ts[0].Name != "a" || ts[0].Description != "a2" || ts[1].Name != "b" {
		t.Errorf("Tools = %+v, quiero [a/a2, b/b1] en ese orden", ts)
	}
}

// Sabotaje E13: una tool sin parametros omite input_schema (tira el turno entero)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\tesquema = json.RawMessage(`{\"type\":\"object\",\"properties\":{}}`)"
// arnes: a=""
//
// Sabotaje E14: input_schema sale como 'parameters' (la API no lo conoce)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\tInputSchema json.RawMessage `json:\"input_schema\"`"
// arnes: a="\t\tInputSchema json.RawMessage `json:\"parameters\"`"
func TestUnaToolSinParametrosRecibeUnEsquemaVacioYNoOmiteElCampo(t *testing.T) {
	// `input_schema` es OBLIGATORIO. Omitirlo tira el turno COMPLETO, así que una sola tool
	// sin parámetros —legítima, pi las declara así— se llevaría puestas a todas las demás.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"x","toolsAdded":[{"name":"hora"}]},
		{"role":"user","content":"hola"}]}}`)

	mismoJSON(t, p.Tools, `[{"name":"hora","input_schema":{"type":"object","properties":{}}}]`)
}

func TestSinToolsDeclaradasElPedidoNoLlevaElCampo(t *testing.T) {
	// Un `tools: []` no es lo mismo que no mandar tools: con un arreglo vacío algunos
	// proveedores igual habilitan el camino de tool use. Nil es «este turno no tiene tools».
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"system","content":"x"},{"role":"user","content":"hola"}]}}`)
	if p.Tools != nil {
		t.Errorf("Tools = %s, quiero nil", p.Tools)
	}
}

// ─────────────────────────────── los bloques ───────────────────────────────

// Sabotaje E15: un content string pelado no se lee como texto
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tif b[0] == '\"' {\n\t\tif err := json.Unmarshal(b, &c.texto1); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tc.esTexto = true\n\t\treturn nil\n\t}"
// arnes: a=""
func TestElContentComoStringPeladoEsUnBloqueDeTexto(t *testing.T) {
	// pi manda el string pelado en el camino corto (types.ts:574); el arreglo es la forma
	// larga. Las dos llegan por el MISMO campo, y un decodificador que sólo acepte una de
	// las dos falla en la mitad de los turnos reales.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"hola"}]}}`)

	if len(p.Mensajes) != 1 || p.Mensajes[0].Rol != "user" || len(p.Mensajes[0].Contenido) != 1 {
		t.Fatalf("Mensajes = %+v", p.Mensajes)
	}
	mismoJSON(t, p.Mensajes[0].Contenido[0], `{"type":"text","text":"hola"}`)
}

// Sabotaje E16: la imagen viaja con mimeType en vez de media_type
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\t\t\"source\": map[string]string{\"type\": \"base64\", \"media_type\": bl.MimeType, \"data\": bl.Data},"
// arnes: a="\t\t\t\t\"source\": map[string]string{\"type\": \"base64\", \"mimeType\": bl.MimeType, \"data\": bl.Data},"
func TestUnaImagenSeTraduceALaFuenteBase64DeLaAPI(t *testing.T) {
	// `{data, mimeType}` → `source{type:"base64", media_type, data}`. Dos renombres y un
	// nivel de anidamiento: nada de esto se ignora si sale mal, hace fallar el request.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":[
			{"type":"text","text":"mirá"},
			{"type":"image","data":"QUJD","mimeType":"image/png"}]}]}}`)

	bl := p.Mensajes[0].Contenido
	if len(bl) != 2 {
		t.Fatalf("quiero 2 bloques, tengo %d", len(bl))
	}
	mismoJSON(t, bl[1], `{"type":"image","source":{"type":"base64","media_type":"image/png","data":"QUJD"}}`)
}

// Sabotaje E17: el bloque de texto se pasa TAL CUAL (viajan los campos de pi)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\tout = append(out, bloqueDeTexto(bl.Text))\n\t\tcase \"image\":"
// arnes: a="\t\t\tout = append(out, b)\n\t\tcase \"image\":"
func TestLosCamposPropiosDePiNoViajanALaAPI(t *testing.T) {
	// 🔴 LA RAZÓN POR LA QUE LOS BLOQUES SE REESCRIBEN EN VEZ DE PASARSE TAL CUAL. El
	// `TextContent` de pi puede traer `textSignature` y el `ToolCall` trae
	// `thoughtSignature` y `namespace`: campos que la Messages API no conoce y que la hacen
	// rechazar el request. Un passthrough «eficiente» acá es un 400 que habla de un campo
	// que el cliente nunca escribió.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":[{"type":"text","text":"hola","textSignature":"firma-de-pi"}]},
		{"role":"assistant","content":[
			{"type":"toolCall","id":"t1","name":"leer","arguments":{"ruta":"a"},
			 "thoughtSignature":"otra","namespace":"local"}]}]}}`)

	mismoJSON(t, p.Mensajes[0].Contenido[0], `{"type":"text","text":"hola"}`)
	mismoJSON(t, p.Mensajes[1].Contenido[0], `{"type":"tool_use","id":"t1","name":"leer","input":{"ruta":"a"}}`)
}

// Sabotaje E18: el toolCall sale con 'arguments' en vez de 'input'
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\t\t\"input\": args,"
// arnes: a="\t\t\t\t\"arguments\": args,"
func TestUnaLlamadaATootUsaInputYNoArguments(t *testing.T) {
	// `arguments` → `input`. Es el mismo renombre que el protocolo de SALIDA hace al revés
	// en `ToolCall.Arguments` (eventos.go), y por el mismo motivo: son dos vocabularios
	// para el mismo dato. Acá se cruzan en un solo paquete, así que conviene que esté medido.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"a"},
		{"role":"assistant","content":[
			{"type":"toolCall","id":"t1","name":"leer","arguments":{"ruta":"/tmp/x"}}]}]}}`)

	mismoJSON(t, p.Mensajes[1].Contenido[0],
		`{"type":"tool_use","id":"t1","name":"leer","input":{"ruta":"/tmp/x"}}`)
}

// Sabotaje E19: un toolCall sin argumentos no recibe el objeto vacio
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\t\targs = json.RawMessage(`{}`)"
// arnes: a=""
func TestUnaLlamadaSinArgumentosLlevaUnObjetoVacio(t *testing.T) {
	// `input` es obligatorio y tiene que ser un objeto. Un `arguments` ausente pasa de
	// verdad: cuando el modelo mandó argumentos que no parseaban, pi deja `{}` y anota el
	// motivo en `argumentsError`. El objeto vacío es la réplica fiel de ese turno.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"a"},
		{"role":"assistant","content":[
			{"type":"toolCall","id":"t1","name":"leer","argumentsError":"JSON inválido"}]}]}}`)

	mismoJSON(t, p.Mensajes[1].Contenido[0], `{"type":"tool_use","id":"t1","name":"leer","input":{}}`)
}

// Sabotaje E20: la firma del thinking sale con el nombre de pi
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\tth, err := json.Marshal(map[string]string{\"type\": \"thinking\", \"thinking\": bl.Thinking, \"signature\": bl.Signature})"
// arnes: a="\t\t\tth, err := json.Marshal(map[string]string{\"type\": \"thinking\", \"thinking\": bl.Thinking, \"thinkingSignature\": bl.Signature})"
func TestElThinkingViajaConSuFirmaYConElNombreDeLaAPI(t *testing.T) {
	// `thinkingSignature` → `signature`. Es la firma criptográfica del bloque: sin ella la
	// API rechaza el turno en una conversación con thinking, y con el nombre de pi viaja
	// como un campo desconocido, que es el mismo rechazo por otra puerta.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"a"},
		{"role":"assistant","content":[
			{"type":"thinking","thinking":"pensando","thinkingSignature":"abc123"},
			{"type":"text","text":"listo"}]}]}}`)

	bl := p.Mensajes[1].Contenido
	if len(bl) != 2 {
		t.Fatalf("quiero 2 bloques, tengo %d", len(bl))
	}
	mismoJSON(t, bl[0], `{"type":"thinking","thinking":"pensando","signature":"abc123"}`)
	mismoJSON(t, bl[1], `{"type":"text","text":"listo"}`)
}

// Sabotaje E21: un thinking redactado viaja como thinking normal
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\tif bl.Redacted {\n\t\t\t\trd, err := json.Marshal(map[string]string{\"type\": \"redacted_thinking\", \"data\": bl.Signature})\n\t\t\t\tif err != nil {\n\t\t\t\t\treturn nil, fmt.Errorf(\"serializar el thinking redactado: %w\", err)\n\t\t\t\t}\n\t\t\t\tout = append(out, rd)\n\t\t\t\tcontinue\n\t\t\t}"
// arnes: a=""
func TestUnThinkingRedactadoEsOtroTipoDeBloque(t *testing.T) {
	// No es el mismo bloque con una bandera: la API lo devuelve como `redacted_thinking` con
	// el payload cifrado en `data`, y reenviarlo así es lo que deja seguir una conversación
	// que pasó por el filtro de seguridad. Mandarlo como `thinking` con texto vacío la corta.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"a"},
		{"role":"assistant","content":[
			{"type":"thinking","thinking":"","thinkingSignature":"cifrado","redacted":true}]}]}}`)

	mismoJSON(t, p.Mensajes[1].Contenido[0], `{"type":"redacted_thinking","data":"cifrado"}`)
}

// Sabotaje E22: un bloque de tipo desconocido se saltea en silencio
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\tdefault:\n\t\t\treturn nil, fmt.Errorf(\"bloque de asistente de tipo %q\", cab.Type)\n\t\t}"
// arnes: a="\t\tdefault:\n\t\t\tcontinue\n\t\t}"
func TestUnBloqueDeTipoDesconocidoEsUnError(t *testing.T) {
	// Igual que con los roles: descartarlo le cambia el contexto al modelo sin dejar rastro.
	msg := errorDesdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"a"},
		{"role":"assistant","content":[{"type":"inventado"}]}]}}`)
	if !strings.Contains(msg, "inventado") {
		t.Errorf("el error no nombra el tipo: %s", msg)
	}
}

// ─────────────────────────── los resultados de tools ───────────────────────────

// Sabotaje E24: el tool_result va como su propio rol en vez de user
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\t\t\tmensajes = agregarMensaje(mensajes, rolUsuario, []json.RawMessage{bloque})"
// arnes: a="\t\t\tmensajes = agregarMensaje(mensajes, rolToolResult, []json.RawMessage{bloque})"
func TestUnResultadoDeToolEsUnMensajeDeUsuario(t *testing.T) {
	// La Messages API no tiene rol propio para esto: el resultado de una tool viaja como un
	// bloque `tool_result` dentro de un mensaje de USUARIO.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"leé"},
		{"role":"assistant","content":[{"type":"toolCall","id":"t1","name":"leer","arguments":{}}]},
		{"role":"toolResult","toolCallId":"t1","toolName":"leer",
			"content":[{"type":"text","text":"el contenido"}],"isError":false}]}}`)

	if len(p.Mensajes) != 3 {
		t.Fatalf("quiero 3 mensajes, tengo %d", len(p.Mensajes))
	}
	if p.Mensajes[2].Rol != "user" {
		t.Errorf("el rol del resultado = %q, quiero \"user\"", p.Mensajes[2].Rol)
	}
	mismoJSON(t, p.Mensajes[2].Contenido[0],
		`{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"el contenido"}]}`)
}

// Sabotaje E25: los mensajes consecutivos del mismo rol NO se fusionan
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tif n := len(ms); n > 0 && ms[n-1].Rol == rol {\n\t\tms[n-1].Contenido = append(ms[n-1].Contenido, bloques...)\n\t\treturn ms\n\t}"
// arnes: a=""
func TestVariosResultadosConsecutivosVanEnUnSoloMensaje(t *testing.T) {
	// 🔴 LA API LO EXIGE: los N resultados de un turno con N `tool_use` van en UN mensaje de
	// usuario. pi los trae como N mensajes `toolResult`, y mandarlos separados es un 400.
	// Sin la fusión, cualquier turno que llame a dos tools a la vez se cae.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"leé los dos"},
		{"role":"assistant","content":[
			{"type":"toolCall","id":"t1","name":"leer","arguments":{}},
			{"type":"toolCall","id":"t2","name":"leer","arguments":{}}]},
		{"role":"toolResult","toolCallId":"t1","toolName":"leer","content":[{"type":"text","text":"uno"}]},
		{"role":"toolResult","toolCallId":"t2","toolName":"leer","content":[{"type":"text","text":"dos"}]}]}}`)

	if len(p.Mensajes) != 3 {
		t.Fatalf("quiero 3 mensajes (user, assistant, user), tengo %d: %+v", len(p.Mensajes), p.Mensajes)
	}
	if n := len(p.Mensajes[2].Contenido); n != 2 {
		t.Fatalf("quiero 2 tool_result en el mismo mensaje, tengo %d", n)
	}
	mismoJSON(t, p.Mensajes[2].Contenido[0],
		`{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"uno"}]}`)
	mismoJSON(t, p.Mensajes[2].Contenido[1],
		`{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"dos"}]}`)
}

func TestUnResultadoQueSigueAUnUsuarioSeFusionaConEl(t *testing.T) {
	// El mismo `agregarMensaje` que fusiona los resultados entre sí los fusiona con un user
	// adyacente, y eso también es lo correcto: dos mensajes de usuario seguidos los rechaza
	// la API.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"a"},
		{"role":"assistant","content":[{"type":"toolCall","id":"t1","name":"x","arguments":{}}]},
		{"role":"toolResult","toolCallId":"t1","toolName":"x","content":[{"type":"text","text":"r"}]},
		{"role":"user","content":"y ahora esto"}]}}`)

	if len(p.Mensajes) != 3 {
		t.Fatalf("quiero 3 mensajes, tengo %d", len(p.Mensajes))
	}
	if n := len(p.Mensajes[2].Contenido); n != 2 {
		t.Errorf("el último mensaje tiene %d bloques, quiero 2 (el resultado y el texto)", n)
	}
}

// Sabotaje E26: is_error se manda siempre (ruido en cada turno del loop)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tif m.IsError {\n\t\t// `is_error` sólo cuando es true: la API acepta el campo ausente, y mandar\n\t\t// `is_error:false` en cada resultado es ruido en cada turno de un agent loop.\n\t\tcampos[\"is_error\"] = true\n\t}"
// arnes: a="\tcampos[\"is_error\"] = m.IsError"
func TestUnResultadoConErrorLoDeclaraYLosDemasNo(t *testing.T) {
	// `isError` → `is_error`, y SÓLO cuando es true: la API acepta el campo ausente, y
	// mandar `is_error:false` en cada resultado es ruido en cada turno de un agent loop.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"a"},
		{"role":"assistant","content":[{"type":"toolCall","id":"t1","name":"x","arguments":{}}]},
		{"role":"toolResult","toolCallId":"t1","toolName":"x",
			"content":[{"type":"text","text":"no existe"}],"isError":true}]}}`)

	mismoJSON(t, p.Mensajes[2].Contenido[0],
		`{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"no existe"}],"is_error":true}`)

	// 🔴 Y «LOS DEMÁS NO» — la mitad que el nombre prometía y el fixture no medía. Con un solo
	// resultado EN ERROR, mandar `is_error` siempre con su valor pasa igual, porque acá el valor
	// es `true` por los dos caminos: el sabotaje que cambia el `if` por una asignación directa
	// quedaba VERDE. Lo que distingue es el caso SIN error, donde la clave tiene que estar
	// AUSENTE y no en `false`.
	q := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"a"},
		{"role":"assistant","content":[{"type":"toolCall","id":"t2","name":"x","arguments":{}}]},
		{"role":"toolResult","toolCallId":"t2","toolName":"x",
			"content":[{"type":"text","text":"listo"}]}]}}`)

	mismoJSON(t, q.Mensajes[2].Contenido[0],
		`{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"listo"}]}`)
}

// Sabotaje E27: un tool_result vacio deja su llamada sin cerrar
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tif len(contenido) == 0 {"
// arnes: a="\tif false {"
func TestUnResultadoVacioIgualCierraSuLlamada(t *testing.T) {
	// Una tool que no devolvió nada TIENE que cerrar su `tool_use`, o el turno queda con una
	// llamada sin respuesta y la API lo rechaza. Se manda un texto vacío explícito, porque
	// un `content` vacío también es inválido.
	p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
		{"role":"user","content":"a"},
		{"role":"assistant","content":[{"type":"toolCall","id":"t1","name":"x","arguments":{}}]},
		{"role":"toolResult","toolCallId":"t1","toolName":"x","content":[]}]}}`)

	mismoJSON(t, p.Mensajes[2].Contenido[0],
		`{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":""}]}`)
}

// ─────────────────────────────── el thinking ───────────────────────────────

// Sabotaje E28: el clamp de xhigh/max no se aplica (nivel desconocido -> 0)
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tif nivel == \"xhigh\" || nivel == \"max\" {\n\t\tnivel = \"high\"\n\t}"
// arnes: a=""
//
// Sabotaje E29: los budgets propios del cliente se ignoran
// arnes: archivo="internal/motor/entrada.go"
// arnes: de="\tif o.ThinkingBudgets != nil {\n\t\tb = *o.ThinkingBudgets\n\t}"
// arnes: a=""
func TestElBudgetDeThinkingSaleDelNivelYDeLaTabla(t *testing.T) {
	casos := []struct {
		nombre  string
		options string
		quiero  int
	}{
		// Sin `reasoning` el thinking está APAGADO. Prenderlo por default le cambiaría la
		// factura y la latencia a todo cliente que no lo pidió.
		{"sin reasoning", `{}`, 0},
		{"minimal", `{"reasoning":"minimal"}`, 1024},
		{"low", `{"reasoning":"low"}`, 2048},
		{"medium", `{"reasoning":"medium"}`, 8192},
		{"high", `{"reasoning":"high"}`, 16384},

		// 🔴 EL CLAMP ES DE pi, NO UNA DECISIÓN DE ACÁ (`clampReasoning`,
		// simple-options.ts:78): su tabla sólo tiene cuatro niveles. Sin el clamp, un
		// cliente que pide "max" cae en el cero del struct y se queda SIN thinking justo
		// cuando pidió el máximo.
		{"xhigh cae en high", `{"reasoning":"xhigh"}`, 16384},
		{"max cae en high", `{"reasoning":"max"}`, 16384},

		// Los presupuestos del cliente le ganan a los de pi: son suyos.
		{"budgets propios", `{"reasoning":"medium","thinkingBudgets":{"medium":3000}}`, 3000},

		// Un nivel que esta versión no conoce NO es un error del pedido. Apagar el thinking
		// es la caída segura; pedirle a la API un budget inventado tiraría el turno entero.
		{"nivel desconocido", `{"reasoning":"turbo"}`, 0},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p := desdeJSON(t, `{"model":{"id":"m"},"context":{"messages":[
				{"role":"user","content":"hola"}]},"options":`+c.options+`}`)
			if p.Thinking != c.quiero {
				t.Errorf("Thinking = %d, quiero %d", p.Thinking, c.quiero)
			}
		})
	}
}

func TestSinThinkingBudgetsSeUsanLosDePiYNoCeros(t *testing.T) {
	// LA DIFERENCIA ENTRE nil Y UN STRUCT DE CEROS, que es por qué `ThinkingBudgets` es un
	// puntero. Un struct de ceros se leería como «presupuesto 0» y apagaría el thinking que
	// el cliente acaba de pedir, en vez de caer en la tabla de pi.
	var p PedidoProxy
	if err := json.Unmarshal([]byte(`{"model":{"id":"m"},
		"context":{"messages":[{"role":"user","content":"hola"}]},
		"options":{"reasoning":"high"}}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.Options.ThinkingBudgets != nil {
		t.Fatal("el fixture no debería traer thinkingBudgets")
	}
	ped, err := PedidoDesdeProxy(p)
	if err != nil {
		t.Fatal(err)
	}
	if ped.Thinking != 16384 {
		t.Errorf("Thinking = %d, quiero el default de pi (16384)", ped.Thinking)
	}
}

// ─────────────────────────────── el viaje completo ───────────────────────────────

func TestUnTranscriptDeDosTurnosConToolsSobreviveElViaje(t *testing.T) {
	// El caso real completo: system con prompt, secciones y tools; un turno de usuario; una
	// respuesta con thinking y una llamada a tool; el resultado; y la pregunta siguiente.
	// Es la forma que tiene un agent loop a la segunda vuelta, y la que más renombres cruza
	// de una sola vez.
	p := desdeJSON(t, `{
		"model":{"id":"claude-opus-5","maxTokens":64000},
		"context":{"messages":[
			{"role":"system","content":"sos Musubi","sections":{"entorno":"Windows"},
			 "toolsAdded":[{"name":"leer","description":"lee","parameters":{"type":"object"}}]},
			{"role":"user","content":"qué dice el archivo"},
			{"role":"assistant","content":[
				{"type":"thinking","thinking":"hay que leerlo","thinkingSignature":"sig"},
				{"type":"toolCall","id":"t1","name":"leer","arguments":{"ruta":"a.txt"}}]},
			{"role":"toolResult","toolCallId":"t1","toolName":"leer",
			 "content":[{"type":"text","text":"hola mundo"}]},
			{"role":"user","content":"y el otro"}]},
		"options":{"maxTokens":2000,"reasoning":"low"}}`)

	if p.SystemPrompt != "sos Musubi\n\nWindows" {
		t.Errorf("SystemPrompt = %q", p.SystemPrompt)
	}
	mismoJSON(t, p.Tools, `[{"name":"leer","description":"lee","input_schema":{"type":"object"}}]`)
	if p.MaxTokens != 2000 || p.Thinking != 2048 {
		t.Errorf("MaxTokens = %d, Thinking = %d; quiero 2000 y 2048", p.MaxTokens, p.Thinking)
	}

	// Tres mensajes: el user, el assistant, y el user que fusiona el resultado con la
	// pregunta siguiente. Los roles tienen que ALTERNAR, o la API rechaza el turno.
	quieroRoles := []string{"user", "assistant", "user"}
	if len(p.Mensajes) != len(quieroRoles) {
		t.Fatalf("quiero %d mensajes, tengo %d: %+v", len(quieroRoles), len(p.Mensajes), p.Mensajes)
	}
	for i, r := range quieroRoles {
		if p.Mensajes[i].Rol != r {
			t.Errorf("Mensajes[%d].Rol = %q, quiero %q", i, p.Mensajes[i].Rol, r)
		}
	}
	mismoJSON(t, p.Mensajes[1].Contenido[0], `{"type":"thinking","thinking":"hay que leerlo","signature":"sig"}`)
	mismoJSON(t, p.Mensajes[1].Contenido[1], `{"type":"tool_use","id":"t1","name":"leer","input":{"ruta":"a.txt"}}`)
	if n := len(p.Mensajes[2].Contenido); n != 2 {
		t.Errorf("el último mensaje tiene %d bloques, quiero 2", n)
	}
}

// TestElPedidoTraducidoLoAceptanLosDosMotores cierra el círculo del paquete: lo que sale de
// acá tiene que poder entrar en el mismo `Pedido` que consumen el Lanzador y el ClienteAPI.
// Sin esta prueba, el traductor de entrada podría estar produciendo un Pedido perfecto para
// un motor que no existe.
func TestElPedidoTraducidoLoAceptaElCuerpoDelRequest(t *testing.T) {
	p := desdeJSON(t, `{"model":{"id":"claude-opus-5","maxTokens":64000},
		"context":{"messages":[
			{"role":"system","content":"sé breve","toolsAdded":[{"name":"x","parameters":{}}]},
			{"role":"user","content":"hola"}]},
		"options":{"reasoning":"medium"}}`)

	cuerpo, err := cuerpoDelRequest(p)
	if err != nil {
		t.Fatalf("cuerpoDelRequest rechazó un pedido traducido: %v", err)
	}
	var r struct {
		Model     string            `json:"model"`
		System    string            `json:"system"`
		MaxTokens int               `json:"max_tokens"`
		Tools     json.RawMessage   `json:"tools"`
		Messages  []json.RawMessage `json:"messages"`
		Thinking  *struct {
			Type         string `json:"type"`
			BudgetTokens int    `json:"budget_tokens"`
		} `json:"thinking"`
	}
	if err := json.Unmarshal(cuerpo, &r); err != nil {
		t.Fatalf("el request no es JSON: %v", err)
	}
	if r.Model != "claude-opus-5" || r.System != "sé breve" || r.MaxTokens != 64000 {
		t.Errorf("request = %+v", r)
	}
	if r.Thinking == nil || r.Thinking.BudgetTokens != 8192 {
		t.Errorf("thinking del request = %+v, quiero budget 8192", r.Thinking)
	}
	if len(r.Messages) != 1 {
		t.Errorf("quiero 1 mensaje, tengo %d", len(r.Messages))
	}
	mismoJSON(t, r.Tools, `[{"name":"x","input_schema":{}}]`)
}
