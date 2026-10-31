package motor

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// pedir arma un POST JSON-RPC contra el espejo y devuelve la respuesta cruda. Se usa un
// httptest.NewRecorder contra el ServeHTTP y no abrirEspejo porque lo que se juzga es el
// protocolo, no el listener: un socket de verdad agrega un puerto y una carrera a cada
// aserción sin cambiar una sola respuesta.
func pedir(t *testing.T, e *espejo, metodo, ruta, cuerpo string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Result()
}

// espejoDePrueba devuelve un espejo armado a mano, sin listener.
func espejoDePrueba(tools string) *espejo {
	return &espejo{url: "http://127.0.0.1:1/t", ruta: "/t", tools: json.RawMessage(tools)}
}

// 🔴 EL RENOMBRE DEL ESQUEMA. `Pedido.Tools` ya viene en el formato del REQUEST
// (`input_schema`, lo que consume el ClienteAPI) y MCP lo quiere en camelCase
// (`inputSchema`). Si esto se rompe, el hijo no rechaza nada: toma la tool con un
// `inputSchema` ausente, el modelo la ve sin parámetros y la llama vacía.
//
// Sabotaje M1: el esquema sale con el nombre del formato del request
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\tInputSchema json.RawMessage `json:\"inputSchema\"`"
// arnes: a="\t\tInputSchema json.RawMessage `json:\"input_schema\"`"
//
// 🔑 POR QUÉ ESTE SABOTAJE NO LLEVA `colision_ok`, Y LOS DE ABAJO TAMPOCO. Es cierto y está
// medido que M1 también pone rojo a TestUnaToolSinParametrosLlevaUnEsquemaVacioYNoSeOmite —las
// dos pruebas leen la misma clave renombrada—, pero ÉSA NO ES LA COLISIÓN QUE EL ARNÉS MODELA.
// Para `arnes.Colisiones` una colisión es que el reemplazo de un sabotaje borre el TEXTO al que
// apunta el `de` de OTRA directiva: daño al corpus, no dos pruebas cayendo juntas. No corre un
// solo test — simula el reemplazo y pregunta a quién le rompe el ancla. Declarar acá un
// `colision_ok` es contestarle a algo que la herramienta nunca preguntó, y lo denuncia como
// respuesta RANCIA, con razón: un «esto ya se miró» sobre algo que nadie miró es peor que el
// aviso. Que un invariante esté custodiado por varias pruebas se escribe en prosa, como acá.
func TestLasToolsDelPedidoSeTraducenAlFormatoMCP(t *testing.T) {
	entrada := `[{"name":"leer","description":"lee un archivo","input_schema":{"type":"object","properties":{"ruta":{"type":"string"}},"required":["ruta"]}}]`

	salida, err := toolsParaElEspejo(json.RawMessage(entrada))
	if err != nil {
		t.Fatalf("toolsParaElEspejo: %v", err)
	}

	var got []struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema"`
	}
	if err := json.Unmarshal(salida, &got); err != nil {
		t.Fatalf("la salida no es una lista de tools MCP: %v — %s", err, salida)
	}
	if len(got) != 1 {
		t.Fatalf("se esperaba 1 tool, hay %d: %s", len(got), salida)
	}
	if got[0].Name != "leer" || got[0].Description != "lee un archivo" {
		t.Errorf("nombre o descripción perdidos: %+v", got[0])
	}
	// El esquema viaja ENTERO, no re-serializado campo por campo: el `required` es lo que
	// hace que el modelo no llame la tool a medias.
	if !strings.Contains(string(got[0].InputSchema), `"required":["ruta"]`) {
		t.Errorf("el esquema llegó recortado: %s", got[0].InputSchema)
	}
	// Y el nombre en snake_case NO tiene que quedar en la salida: si quedara, serían dos
	// claves y el hijo elegiría una de las dos sin avisar cuál.
	if strings.Contains(string(salida), "input_schema") {
		t.Errorf("quedó la clave del formato del request: %s", salida)
	}
}

// 🔴 EL `_meta` ES LO QUE HACE QUE EL MODELO VEA LA TOOL SIN TENER QUE BUSCARLA. Sin
// `_meta["anthropic/alwaysLoad"]` el CLI se la entrega DIFERIDA: no está en el contexto del
// modelo, y el primer `tool_use` del turno sale siendo un `ToolSearch` —una tool del CLI que
// el cliente remoto no declaró y no sabe ejecutar—. Medido en vivo el 2026-10-29 contra el
// 2.1.272, con un servidor MCP de juguete de una sola tool, mismo prompt y mismo host:
//
//	sin _meta -> el modelo emite [ToolSearch, mcp__juguete__sumar_dos_numeros]
//	con _meta -> el modelo emite [mcp__juguete__sumar_dos_numeros]
//
// Sabotaje M20: las tools del cliente le llegan diferidas al modelo y el turno arranca con
// un ToolSearch que el cliente no puede ejecutar
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\t\tMeta:        map[string]any{\"anthropic/alwaysLoad\": true},\n"
// arnes: a=""
func TestLasToolsDelEspejoSeDeclaranAlwaysLoad(t *testing.T) {
	entrada := `[{"name":"leer","input_schema":{"type":"object"}},` +
		`{"name":"escribir","input_schema":{"type":"object"}}]`

	salida, err := toolsParaElEspejo(json.RawMessage(entrada))
	if err != nil {
		t.Fatalf("toolsParaElEspejo: %v", err)
	}

	var got []struct {
		Name string         `json:"name"`
		Meta map[string]any `json:"_meta"`
	}
	if err := json.Unmarshal(salida, &got); err != nil {
		t.Fatalf("la salida no es una lista de tools MCP: %v — %s", err, salida)
	}
	if len(got) != 2 {
		t.Fatalf("se esperaban 2 tools, hay %d: %s", len(got), salida)
	}

	// TODAS y no sólo la primera: una tool diferida en el medio de la lista rompe igual, y
	// un bug que marque sólo la primera pasaría un test de una sola tool.
	for _, tl := range got {
		if v, ok := tl.Meta["anthropic/alwaysLoad"]; !ok || v != true {
			t.Errorf("la tool %q no se declara alwaysLoad y le llegaría diferida al modelo: "+
				"_meta=%v", tl.Name, tl.Meta)
		}
	}

	// La clave EXACTA importa, porque el CLI busca ese nombre y nada más. Con la clave mal
	// escrita el `_meta` viaja igual y no hace nada: el peor de los dos mundos, porque se
	// lee como arreglado.
	if !strings.Contains(string(salida), `"anthropic/alwaysLoad":true`) {
		t.Errorf("la clave del _meta no es la que mira el CLI: %s", salida)
	}
}

// Una tool sin parámetros es legítima, y `inputSchema` es obligatorio en MCP. Omitirlo haría
// que el hijo rechace el servidor ENTERO: una sola tool sin argumentos dejaría al turno sin
// ninguna.
//
// Sabotaje M2: una tool sin parámetros viaja con el esquema en null
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\tif len(bytes.TrimSpace(esquema)) == 0 || bytes.Equal(bytes.TrimSpace(esquema), []byte(\"null\")) {"
// arnes: a="\t\tif false {"
func TestUnaToolSinParametrosLlevaUnEsquemaVacioYNoSeOmite(t *testing.T) {
	for _, caso := range []struct{ nombre, entrada string }{
		{"ausente", `[{"name":"ping"}]`},
		{"null", `[{"name":"ping","input_schema":null}]`},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			salida, err := toolsParaElEspejo(json.RawMessage(caso.entrada))
			if err != nil {
				t.Fatalf("toolsParaElEspejo: %v", err)
			}
			if !strings.Contains(string(salida), `"inputSchema":{"type":"object","properties":{}}`) {
				t.Errorf("falta el esquema vacío explícito: %s", salida)
			}
		})
	}
}

// Un `Pedido.Tools` que no es una lista de tools tiene que fallar ACÁ, al abrir. Si pasara,
// el espejo serviría un `tools/list` con un cuerpo que el hijo rechaza, y el síntoma sería un
// turno sin tools sin un error que lo explique.
func TestUnToolsIlegibleNoAbreElEspejo(t *testing.T) {
	for _, crudo := range []string{`{esto no es json`, `{"name":"no es una lista"}`, `"texto"`} {
		if _, err := abrirEspejo(Pedido{Tools: json.RawMessage(crudo)}, DemoraPorDefecto); err == nil {
			t.Errorf("abrirEspejo aceptó %q", crudo)
		}
	}
}

// EL TOPE SE MIDE CON EL PREFIJO PUESTO, y es el único lugar donde se puede: quien conoce el
// prefijo es espejo.go. Una tool de 55 caracteres es válida para el cliente y para la API, y
// deja de serlo cuando el CLI le pone `mcp__pi__` adelante — el error del otro lado hablaría
// de un nombre que el cliente nunca escribió.
//
// Sabotaje M3: el tope se mide sobre el nombre del cliente y no sobre el que expone el CLI
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\tif n := len(prefijoDelEspejo) + len(t.Name); n > topeDelNombreDeTool {"
// arnes: a="\t\tif n := len(t.Name); n > topeDelNombreDeTool {"
func TestElTopeDelNombreCuentaElPrefijoQuePoneElCLI(t *testing.T) {
	justo := strings.Repeat("a", topeDelNombreDeTool-len(prefijoDelEspejo))
	unaDeMas := justo + "a"

	if _, err := toolsParaElEspejo(json.RawMessage(`[{"name":"` + justo + `"}]`)); err != nil {
		t.Errorf("un nombre que queda EXACTO en el tope tiene que pasar (%d+%d=%d): %v",
			len(prefijoDelEspejo), len(justo), len(prefijoDelEspejo)+len(justo), err)
	}

	_, err := toolsParaElEspejo(json.RawMessage(`[{"name":"` + unaDeMas + `"}]`))
	if err == nil {
		t.Fatalf("un nombre de %d + el prefijo de %d pasa el tope de %d y tiene que fallar",
			len(unaDeMas), len(prefijoDelEspejo), topeDelNombreDeTool)
	}
	// El error nombra el prefijo: sin eso, quien lo lea va a medir el nombre del cliente,
	// ver 56 caracteres contra un tope de 64 y buscar el defecto en otra parte.
	if !strings.Contains(err.Error(), prefijoDelEspejo) {
		t.Errorf("el error no explica de dónde salen los caracteres de más: %v", err)
	}
}

// Una tool sin nombre no se puede exponer, y dejarla pasar la convertiría en el prefijo
// pelado: una tool llamada `mcp__pi__` que el modelo podría llamar.
func TestUnaToolSinNombreNoPasa(t *testing.T) {
	if _, err := toolsParaElEspejo(json.RawMessage(`[{"description":"sin nombre"}]`)); err == nil {
		t.Fatal("una tool sin nombre tiene que fallar")
	}
}

// 🔴 UN NOMBRE FUERA DEL CHARSET NO FALLA: EL CLI LO RENOMBRA. Medido el 2026-10-29 contra el
// 2.1.272 leyendo la tabla del evento `system/init`: cada carácter que no entra en
// `[a-zA-Z0-9_-]` se reemplaza por `_`, la tool queda en la tabla con OTRO nombre y el
// servidor sigue `connected`. El cliente que declaró `read.file` recibe un `tool_use` para
// `read_file`, que no está en su tabla, y su agent loop se corta sin una explicación.
//
// Y medido también: `tool mala` y `tool/mala` normalizan las dos a `tool_mala` y el CLI
// DEDUPLICA, así que de tres tools el modelo vio dos. Dos entradas del cliente colapsadas en
// una, en silencio.
//
// Sabotaje M24: los nombres ilegales pasan y el CLI los renombra a espaldas del cliente
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\tif i := strings.IndexFunc(t.Name, fueraDelCharsetDeTool); i >= 0 {"
// arnes: a="\t\tif i := strings.IndexFunc(t.Name, func(rune) bool { return false }); i >= 0 {"
func TestUnNombreFueraDelCharsetNoPasa(t *testing.T) {
	// Los que el CLI SÍ respeta, medidos uno por uno: tienen que pasar. Sin esta mitad, la
	// validación podría rechazar todo y el test no lo vería.
	for _, bueno := range []string{"tool_ok", "tool-ok", "ToolOk", "leer", "a1", "A-_9"} {
		if _, err := toolsParaElEspejo(json.RawMessage(`[{"name":"` + bueno + `"}]`)); err != nil {
			t.Errorf("%q es un nombre legal para la API y lo rechazó: %v", bueno, err)
		}
	}

	for _, caso := range []struct{ nombre, culpable string }{
		{"tool.punto", "."},
		{"tool mala", " "},
		{"tool/mala", "/"},
		{"tool@arroba", "@"},
		{"tool(par)", "("},
		{"tool:dos", ":"},
		{"tool+mas", "+"},
		{"toolñ", "ñ"},
	} {
		_, err := toolsParaElEspejo(json.RawMessage(`[{"name":"` + caso.nombre + `"}]`))
		if err == nil {
			t.Errorf("%q pasó, y el CLI lo renombraría sin avisarle al cliente", caso.nombre)
			continue
		}
		// El error tiene que nombrar la tool: el que lo lee es el cliente remoto, que puede
		// tener veinte tools declaradas y ninguna forma de saber cuál es la del problema.
		if !strings.Contains(err.Error(), caso.nombre) {
			t.Errorf("el error de %q no nombra la tool: %v", caso.nombre, err)
		}
	}

	// Una tool buena NO salva al pedido si viene con una mala: el renombre afecta al conjunto
	// —dos nombres pueden colapsar en uno— así que el turno entero se rechaza.
	if _, err := toolsParaElEspejo(json.RawMessage(
		`[{"name":"buena"},{"name":"tool mala"},{"name":"tool/mala"}]`)); err == nil {
		t.Error("un pedido con una tool ilegal entre las buenas tiene que fallar entero: " +
			"`tool mala` y `tool/mala` colapsan en el mismo `tool_mala`")
	}
}

// Un turno sin tools no necesita espejo: abrir uno igual sería un listener y un
// `--mcp-config` de más en cada turno pelado.
//
// Sabotaje M4: un turno sin tools abre un espejo igual
//
// 🔴 ESTE SABOTAJE SE LLEVA DOCE TESTS PUESTOS —trece rojos contando a su custodio—, y la lista
// larga de abajo es el hallazgo, no burocracia: sacar este early-return no degrada un caso raro,
// rompe TODOS los turnos del Lanzador. Un pedido sin tools es el caso normal de los tests —y de
// cualquier turno de texto—, y sin el corte `abrirEspejo` le pasa un `nil` al parser de tools y
// devuelve error antes de lanzar el proceso. O sea que este invariante no lo custodia sólo el test
// de acá: lo custodia el paquete entero, y eso es lo que lo hace difícil de romper sin darse cuenta.
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tif len(bytes.TrimSpace(p.Tools)) == 0 {\n\t\treturn nil, nil\n\t}"
// arnes: a=""
// Los DOCE colaterales van en PROSA y no en `colision_ok`, por el motivo que explica M1
// arriba: caen por COMPORTAMIENTO —el early-return ya no está y `abrirEspejo` devuelve error
// en todo turno sin tools—, no porque este reemplazo les rompa el ancla. Medidos uno por uno
// (2026-10-29, `go test -overlay`: 13 `--- FAIL` de nivel superior; el decimotercero es
// `TestSinToolsNoHayEspejo`, que NO es colateral sino el custodio de este mismo sabotaje —el
// que tiene que morder—, así que no entra en la cuenta):
// TestBinarioInexistente, TestElCancelDelLlamadorCortaElTurno, TestElCorteMataElProceso,
// TestElLectorAvisaCuandoElHijoNuncaVioLasTools, TestElTurnoSeCortaPorTiempoYLoDice,
// TestSiElConsumidorFallaElTurnoVuelve, TestUnErrorEnElUltimoEventoNoSeReportaComoTurnoServido,
// TestUnPanicDelConsumidorNoDejaLaCosechaSinHacer, TestUnProcesoMudoEsError,
// TestUnProcesoQueMuereLlevaSuStderrAlError, TestUnTopeNegativoNoMataElTurnoEnElActo,
// TestUnaLineaLargaDelHarnessNoTiraElTurno.
//
// ⚠️ ESTE NÚMERO SE VENCE SOLO: cualquier test nuevo que corra un turno sin tools entra a la
// lista. Al re-medirlo faltaban DOS —`ElLectorAvisa…` desde antes y `UnTopeNegativo…` recién
// agregado—, y el «diez» que esta lista traía escrito ya era falso. Se re-mide con el overlay y se
// escribe CON su aritmética —doce colaterales, trece rojos—, que es lo que deja ver el desfase sin
// volver a correrlo; no se hereda del comentario anterior.
func TestSinToolsNoHayEspejo(t *testing.T) {
	for _, caso := range []struct {
		nombre string
		tools  json.RawMessage
	}{
		{"nil", nil},
		{"vacío", json.RawMessage("")},
		{"espacios", json.RawMessage("   \n")},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			esp, err := abrirEspejo(Pedido{Tools: caso.tools}, DemoraPorDefecto)
			if err != nil {
				t.Fatalf("abrirEspejo: %v", err)
			}
			if esp != nil {
				esp.cerrar()
				t.Fatal("un pedido sin tools no debe abrir un espejo")
			}
		})
	}
}

// Un espejo nil tiene que tolerar el cierre y la lectura del contador: el llamador hace
// `defer esp.cerrar()` sin preguntar si el pedido traía tools.
func TestUnEspejoNilNoExplota(t *testing.T) {
	var e *espejo
	e.cerrar()
	if n := e.EjecucionesIntentadas(); n != 0 {
		t.Errorf("un espejo que no existe no ejecutó nada, y dio %d", n)
	}
}

// El espejo que SÍ se abre escucha en loopback y nada más: un listener en todas las
// interfaces publicaría las definiciones de tools del cliente en la red.
//
// Sabotaje M5: el espejo escucha en todas las interfaces
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tln, err := net.Listen(\"tcp\", \"127.0.0.1:0\")"
// arnes: a="\tln, err := net.Listen(\"tcp\", \":0\")"
func TestElEspejoEscuchaSoloEnLoopbackYConUnaRutaSecreta(t *testing.T) {
	esp, err := abrirEspejo(Pedido{Tools: json.RawMessage(`[{"name":"leer"}]`)}, DemoraPorDefecto)
	if err != nil {
		t.Fatalf("abrirEspejo: %v", err)
	}
	defer esp.cerrar()

	if !strings.HasPrefix(esp.url, "http://127.0.0.1:") {
		t.Errorf("el espejo no está atado a loopback: %s", esp.url)
	}
	// 16 bytes en hex. Una ruta corta —o peor, fija— la adivina cualquier proceso local, y
	// detrás de ella están las tools del cliente.
	if tok := strings.TrimPrefix(esp.ruta, "/"); len(tok) != 32 {
		t.Errorf("la ruta no parece un token de 16 bytes: %q", esp.ruta)
	}
	if !strings.HasSuffix(esp.url, esp.ruta) {
		t.Errorf("la URL que se le pasa al hijo no incluye la ruta secreta: %s vs %s", esp.url, esp.ruta)
	}
}

// Dos turnos concurrentes no pueden compartir ni el puerto ni el token.
//
// Sabotaje M6: el token es fijo (un token predecible es lo mismo que no tener token)
//
// El `hex.EncodeToString(b[:0])` del reemplazo devuelve "" y está ahí sólo para que el import
// y el buffer sigan usados: sin eso el sabotaje no compila y el veredicto sería «no compila»
// en vez de «el test lo vio».
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\treturn hex.EncodeToString(b[:]), nil"
// arnes: a="\treturn \"00000000000000000000000000000000\" + hex.EncodeToString(b[:0]), nil"
func TestDosEspejosNoCompartenPuertoNiToken(t *testing.T) {
	p := Pedido{Tools: json.RawMessage(`[{"name":"leer"}]`)}
	a, err := abrirEspejo(p, DemoraPorDefecto)
	if err != nil {
		t.Fatalf("abrirEspejo a: %v", err)
	}
	defer a.cerrar()
	b, err := abrirEspejo(p, DemoraPorDefecto)
	if err != nil {
		t.Fatalf("abrirEspejo b: %v", err)
	}
	defer b.cerrar()

	if a.url == b.url {
		t.Errorf("dos espejos con la misma URL: %s", a.url)
	}
	if a.ruta == b.ruta {
		t.Errorf("dos espejos con el mismo token: %s", a.ruta)
	}
}

// 🔴 LA RUTA ES LA CREDENCIAL. Un listener en loopback lo alcanza cualquier proceso de la
// máquina, y lo que hay detrás son las definiciones de tools del cliente. El 404 va ANTES de
// mirar el método para que un sondeo no pueda distinguir «ruta equivocada» de «método
// equivocado» y usar la diferencia para encontrar la ruta buena.
//
// Las dos anclas de acá se ACTUALIZARON cuando la comparación pasó a ConstantTimeCompare: una
// directiva `arnes:` que apunta a un texto que ya no existe NO falla, el sabotaje no se aplica
// y el arnés informa verde. Mover la línea que un sabotaje ataca es, en silencio, apagarlo.
//
// Sabotaje M7: la ruta no se verifica (cualquier proceso local lee las tools del cliente). El
// reemplazo deja un `subtle.ConstantTimeCompare` inerte en vez de borrar el bloque: `subtle`
// no se usa en ninguna otra línea del archivo, y sin eso el sabotaje no compila y el veredicto
// sería «no compila» en vez de «el test lo vio» (mismo truco que M6 con `hex`).
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tif subtle.ConstantTimeCompare([]byte(r.URL.Path), []byte(e.ruta)) != 1 {\n\t\thttp.NotFound(w, r)\n\t\treturn\n\t}"
// arnes: a="\t_ = subtle.ConstantTimeCompare(nil, nil)"
// arnes: colision_ok="TestLaRutaEquivocadaDa404SeaCualSeaElMetodo"
//
// Sabotaje M8: el método se mira ANTES de la ruta, y la diferencia delata la ruta buena
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tif subtle.ConstantTimeCompare([]byte(r.URL.Path), []byte(e.ruta)) != 1 {"
// arnes: a="\tif r.Method == http.MethodPost && subtle.ConstantTimeCompare([]byte(r.URL.Path), []byte(e.ruta)) != 1 {"
// arnes: colision_ok="TestLaRutaEquivocadaDa404SeaCualSeaElMetodo"
//
// ⚠️ ESTAS DOS SÍ SE PISAN DE VERDAD, Y POR ESO SON LAS ÚNICAS DEL ARCHIVO CON `colision_ok`.
// La diferencia con M1/M4/M10 es la que importa: el `de` de M8 es un PREFIJO del `de` de M7 —la
// misma línea del `if`, una con su cuerpo y la otra sin él—, así que aplicar cualquiera de los
// dos BORRA el texto al que apunta el otro. Eso es daño al corpus, que es lo único que
// `arnes.Colisiones` modela, y sin contestarlo el segundo sabotaje correría contra un archivo ya
// mutado y su rojo no probaría nada.
//
// Y la respuesta es honesta porque los dos motivos son DISTINTOS: M7 pregunta si la ruta se
// verifica; M8, si se verifica ANTES del método. El test cubre los dos a propósito — el caso
// {GET, "/otra"} es el único que distingue a M8, porque es el que tiene las dos cosas mal. Son
// dos guardas sobre la misma línea, no una contada dos veces. Las dos nombran a este test porque
// las dos anclas viven en él: `colision_ok` nombra LA PRUEBA de la otra directiva, nunca un
// `archivo:línea`, que se corre solo en cuanto alguien agrega un comentario arriba.
func TestLaRutaEquivocadaDa404SeaCualSeaElMetodo(t *testing.T) {
	e := espejoDePrueba(`[{"name":"leer","inputSchema":{}}]`)

	for _, caso := range []struct{ metodo, ruta string }{
		{http.MethodPost, "/"},
		{http.MethodPost, "/otra"},
		{http.MethodGet, "/otra"},
		{http.MethodPost, e.ruta + "x"},
	} {
		resp := pedir(t, e, caso.metodo, caso.ruta, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s %s dio %d, se esperaba 404", caso.metodo, caso.ruta, resp.StatusCode)
		}
	}

	// Y la ruta buena con el método equivocado da 405 CON el Allow: el hijo no tiene que
	// quedar adivinando si el servidor habla o no.
	resp := pedir(t, e, http.MethodGet, e.ruta, "")
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("un GET a la ruta buena dio %d, se esperaba 405", resp.StatusCode)
	}
	if got := resp.Header.Get("Allow"); got != "POST" {
		t.Errorf("el 405 no dice qué método sirve: Allow=%q", got)
	}
}

// Una NOTIFICACIÓN no se contesta. `notifications/initialized` llega sin `id`, y mandarle una
// respuesta JSON-RPC es un error de protocolo que algunos clientes reportan como servidor
// roto — o sea, un turno sin tools y sin un motivo visible.
//
// Sabotaje M9: una notificación recibe respuesta
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tif len(req.ID) == 0 {\n\t\tw.WriteHeader(http.StatusAccepted)\n\t\treturn\n\t}"
// arnes: a=""
func TestUnaNotificacionNoSeContesta(t *testing.T) {
	e := espejoDePrueba(`[{"name":"leer","inputSchema":{}}]`)
	resp := pedir(t, e, http.MethodPost, e.ruta, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("una notificación dio %d, se esperaba 202", resp.StatusCode)
	}
	var cuerpo [1]byte
	if n, _ := resp.Body.Read(cuerpo[:]); n != 0 {
		t.Errorf("una notificación no lleva cuerpo, y volvió con %d byte(s)", n)
	}
}

// 🔴 EL `id` LO PONE EL QUE INTENTA, ASÍ QUE NO PUEDE SER ÉL EL QUE DECIDA SI QUEDA REGISTRADO.
// El contador de ejecuciones alimenta el aviso del lanzador, y durante todo un tramo se sumaba
// DENTRO del `case "tools/call"` — que está detrás de la puerta de las notificaciones de acá
// arriba. Un `tools/call` sin `id` salía por ese 202 y dejaba el contador en CERO: el único
// camino en que el hijo podía ejecutar sin dejar rastro era, justamente, el que menos se parece
// a un cliente bien portado. Ahora el conteo va antes de la puerta.
//
// El 202 y el 1 son dos invariantes distintos sobre el mismo request, y se miden juntos a
// propósito: «no contestar» es protocolo —ver el test de arriba— y «contar igual» es
// instrumentación. Arreglar uno moviendo el otro es exactamente el error que esto custodia.
//
// Sabotaje M28: el conteo vuelve atrás de la puerta y un tools/call sin id no deja rastro
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tif req.Method == \"tools/call\" {\n\t\te.llamadas.Add(1)\n\t}"
// arnes: a=""
func TestUnToolsCallSinIdSeCuentaIgual(t *testing.T) {
	e := espejoDePrueba(`[{"name":"leer","inputSchema":{}}]`)

	resp := pedir(t, e, http.MethodPost, e.ruta,
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"leer","arguments":{}}}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("un tools/call sin id dio %d, se esperaba 202", resp.StatusCode)
	}
	if n := e.EjecucionesIntentadas(); n != 1 {
		t.Errorf("el intento sin id no se contó: EjecucionesIntentadas()=%d", n)
	}

	// Y lo que NO es un tools/call no mueve el contador, o el número no diría nada: la
	// notificación `initialized` es lo primero que manda cualquier cliente MCP, y si contara
	// como ejecución el aviso saldría en todos los turnos.
	pedir(t, e, http.MethodPost, e.ruta, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if n := e.EjecucionesIntentadas(); n != 1 {
		t.Errorf("una notificación cualquiera movió el contador de ejecuciones: %d", n)
	}
}

// 🔴 EL FALLO QUE NO SE VE SOLO: EL HIJO QUE NUNCA ENCONTRÓ EL ESPEJO. El contador de
// ejecuciones de arriba mide que el hijo llegó DEMASIADO lejos; éste mide lo contrario, y es el
// peligroso: si el `--mcp-config` no llegó, si el CLI descartó el servidor o si el handshake se
// cayó, el modelo contesta el turno entero SIN SABER que las tools del cliente existen. Y lo que
// vuelve es una respuesta perfectamente formada: el cliente no puede distinguirla de una en la
// que el modelo miró sus tools y eligió no usar ninguna. Sin este número, un `--mcp-config` que
// dejó de funcionar se lee como un modelo que se volvió vago.
//
// Sabotaje M29: el listado servido no se cuenta y el hijo que nunca vio las tools queda invisible
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\te.listados.Add(1)\n"
// arnes: a=""
func TestElContadorDeListadosSoloSubeConUnToolsList(t *testing.T) {
	e := espejoDePrueba(`[{"name":"leer","inputSchema":{}}]`)

	// ARRANCA EN CERO, y esta mitad importa tanto como la otra: el aviso del lanzador se
	// dispara CON el cero, así que un contador que naciera en uno lo apagaría para siempre y
	// nadie lo notaría — un aviso que no sale nunca se ve igual que un sistema sano.
	if n := e.ListadosServidos(); n != 0 {
		t.Fatalf("un espejo recién armado ya dice haber servido %d listado(s)", n)
	}

	// El handshake solo NO alcanza: lo que prueba que el modelo vio el roster es el tools/list,
	// y un `initialize` contestado sin que después pidan la lista es justo el síntoma del
	// servidor que el CLI descartó.
	pedir(t, e, http.MethodPost, e.ruta, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	if n := e.ListadosServidos(); n != 0 {
		t.Errorf("el initialize contó como listado servido: %d", n)
	}

	pedir(t, e, http.MethodPost, e.ruta, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if n := e.ListadosServidos(); n != 1 {
		t.Errorf("el tools/list no se contó: %d", n)
	}

	// Y el nil contesta cero sin explotar: el lanzador consulta esto en un defer que corre
	// también en los turnos sin tools, donde no hay espejo.
	var nulo *espejo
	if n := nulo.ListadosServidos(); n != 0 {
		t.Errorf("un espejo nil devolvió %d", n)
	}
}

// El handshake. Si el `protocolVersion` no es el que el hijo espera, no hay tools: el
// servidor queda descartado antes del primer `tools/list`.
func TestElHandshakeDeclaraTools(t *testing.T) {
	e := espejoDePrueba(`[{"name":"leer","inputSchema":{}}]`)
	resp := pedir(t, e, http.MethodPost, e.ruta, `{"jsonrpc":"2.0","id":7,"method":"initialize"}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize dio %d", resp.StatusCode)
	}
	var r struct {
		JsonRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  struct {
			ProtocolVersion string                     `json:"protocolVersion"`
			Capabilities    map[string]json.RawMessage `json:"capabilities"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("la respuesta no es JSON-RPC: %v", err)
	}
	if r.JsonRPC != "2.0" {
		t.Errorf("jsonrpc=%q", r.JsonRPC)
	}
	if string(r.ID) != "7" {
		t.Errorf("el id no volvió como vino: %s", r.ID)
	}
	if r.Result.ProtocolVersion != "2024-11-05" {
		t.Errorf("protocolVersion=%q, y tiene que ser la misma que declara internal/mcp", r.Result.ProtocolVersion)
	}
	if _, ok := r.Result.Capabilities["tools"]; !ok {
		t.Errorf("un servidor que no declara la capability `tools` no le va a servir ninguna: %v", r.Result.Capabilities)
	}
}

// EL ID VIAJA CRUDO. El spec deja que sea un número o un string, y re-tipearlo acá es la
// forma de devolverle al hijo un id que no es el que mandó: la respuesta queda huérfana y el
// hijo espera para siempre.
//
// Sabotaje M10: el id se normaliza y la respuesta queda huérfana
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tif len(id) == 0 {\n\t\tid = json.RawMessage(\"null\")\n\t}"
// arnes: a="\tid = json.RawMessage(\"1\")"
//
// También caen TestElHandshakeDeclaraTools y TestUnCuerpoRotoContestaParseError, y va en prosa
// por lo que explica M1: los tres piden al espejo y comparan el id que vuelve, así que es
// comportamiento, no daño al ancla del otro.
func TestElIdVuelveConSuTipoOriginal(t *testing.T) {
	e := espejoDePrueba(`[]`)
	for _, id := range []string{`1`, `"abc-1"`, `0`} {
		resp := pedir(t, e, http.MethodPost, e.ruta, `{"jsonrpc":"2.0","id":`+id+`,"method":"initialize"}`)
		var r struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			t.Fatalf("decodificar con id=%s: %v", id, err)
		}
		if string(r.ID) != id {
			t.Errorf("mandé id=%s y volvió %s", id, r.ID)
		}
	}
}

// Las tools que se armaron al abrir son las que sirve `tools/list`, tal cual.
func TestToolsListSirveLoQueSeArmoAlAbrir(t *testing.T) {
	e := espejoDePrueba(`[{"name":"leer","inputSchema":{"type":"object","properties":{}}}]`)
	resp := pedir(t, e, http.MethodPost, e.ruta, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	var r struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("tools/list no devolvió JSON-RPC válido: %v", err)
	}
	if len(r.Result.Tools) != 1 || r.Result.Tools[0].Name != "leer" {
		t.Fatalf("tools/list no sirvió la tool del pedido: %+v", r.Result.Tools)
	}
	if string(r.Result.Tools[0].InputSchema) == "" {
		t.Errorf("la tool llegó sin inputSchema: el modelo la llamaría sin argumentos")
	}
}

// 🔴 ESTE CAMINO NO DEBERÍA EXISTIR, Y POR ESO CONTESTA UN ERROR. El turno se corta en el
// `tool_use` y el que ejecuta es el cliente remoto. Un resultado vacío sería lo peor
// posible: el modelo seguiría razonando sobre una tool que nadie corrió.
//
// ⚠️ M11 ATACABA EL CONTADOR Y HOY ATACA EL ERROR, y el cambio no es cosmético. Su `de` era
// `e.llamadas.Add(1)`, que vivía acá adentro del `case`; cuando el conteo se movió ANTES de la
// puerta de las notificaciones —para que un `tools/call` sin `id` no quedara sin registrar—, el
// ancla se quedó apuntando al mismo texto EN SU NUEVO SITIO. O sea que M11 y M28 pasaron a
// borrar la misma línea: el censo lo denunció como un pisón, y leyendo los dos motivos no eran
// dos guardas sino UNA CONTADA DOS VECES. El conteo queda custodiado por M28, donde se mide el
// caso que de verdad lo distingue, y M11 se muda al otro invariante de este test —que el camino
// imposible conteste un ERROR y no un resultado— que hasta ahora no tenía ninguno.
//
// Sabotaje M11: el camino imposible contesta con otro código y el cliente no lo distingue
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\t\tCode: -32603,"
// arnes: a="\t\t\tCode: 0,"
func TestUnToolsCallContestaErrorYQuedaContado(t *testing.T) {
	e := espejoDePrueba(`[{"name":"leer","inputSchema":{}}]`)
	if n := e.EjecucionesIntentadas(); n != 0 {
		t.Fatalf("el contador arranca en %d", n)
	}

	resp := pedir(t, e, http.MethodPost, e.ruta,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"leer","arguments":{}}}`)

	var r struct {
		Result json.RawMessage `json:"result"`
		Error  *errorRPC       `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if r.Error == nil {
		t.Fatalf("tools/call tiene que contestar un error, y devolvió result=%s", r.Result)
	}
	if r.Error.Code != -32603 {
		t.Errorf("code=%d, se esperaba el -32603 de error interno", r.Error.Code)
	}
	if len(r.Result) != 0 {
		t.Errorf("un error no puede venir con result: %s", r.Result)
	}
	if n := e.EjecucionesIntentadas(); n != 1 {
		t.Errorf("el intento no quedó contado: %d", n)
	}

	// Dos veces son dos.
	pedir(t, e, http.MethodPost, e.ruta, `{"jsonrpc":"2.0","id":4,"method":"tools/call"}`)
	if n := e.EjecucionesIntentadas(); n != 2 {
		t.Errorf("el contador no acumula: %d", n)
	}
}

// Un método que no existe contesta -32601 y no 200 vacío: el hijo tiene que poder distinguir
// «no lo sé hacer» de «lo hice y no devolví nada».
func TestUnMetodoDesconocidoContestaMethodNotFound(t *testing.T) {
	e := espejoDePrueba(`[]`)
	resp := pedir(t, e, http.MethodPost, e.ruta, `{"jsonrpc":"2.0","id":9,"method":"resources/list"}`)

	var r struct {
		Error *errorRPC `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("decodificar: %v", err)
	}
	if r.Error == nil || r.Error.Code != -32601 {
		t.Fatalf("se esperaba -32601, vino %+v", r.Error)
	}
	// El nombre del método va en el mensaje: sin eso, un cambio de protocolo del CLI se ve
	// como un servidor mudo.
	if !strings.Contains(r.Error.Message, "resources/list") {
		t.Errorf("el error no dice qué método se pidió: %q", r.Error.Message)
	}
}

// Un cuerpo que no es JSON contesta el -32700 del spec, no un 500 de Go.
func TestUnCuerpoRotoContestaParseError(t *testing.T) {
	e := espejoDePrueba(`[]`)
	resp := pedir(t, e, http.MethodPost, e.ruta, `{esto no es json`)

	var r struct {
		ID    json.RawMessage `json:"id"`
		Error *errorRPC       `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("ni la respuesta de error es JSON: %v", err)
	}
	if r.Error == nil || r.Error.Code != -32700 {
		t.Fatalf("se esperaba -32700, vino %+v", r.Error)
	}
	// Sin id que devolver, el spec pide `null` EXPLÍCITO. Omitirlo deja una respuesta que no
	// es JSON-RPC válido.
	if string(r.ID) != "null" {
		t.Errorf("un parse error sin id tiene que volver con id:null, y volvió %s", r.ID)
	}
}

// 🔴 EL TOPE DE 1 MiB DEL CUERPO, Y POR QUÉ ESTE TEST MANDA JSON VÁLIDO. El cuerpo lo escribe
// un proceso de afuera, así que sin tope un POST de un giga lo aloja este servidor en la
// memoria del turno. Pero un cuerpo de BASURA de un giga no mide nada: el parser lo rechaza
// con el mismo -32700 esté el tope o no esté, y el verde sería correcto y vacuo. Lo que
// discrimina es un cuerpo que SERÍA válido si el tope no existiera: con tope da -32700, sin
// tope se parsea y contesta la lista de tools.
//
// Sabotaje M25: el cuerpo del hijo entra sin tope
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="http.MaxBytesReader(w, r.Body, 1<<20)"
// arnes: a="r.Body"
func TestUnCuerpoGiganteNoSeParsea(t *testing.T) {
	e := espejoDePrueba(`[{"name":"leer","inputSchema":{}}]`)
	// JSON válido y bien formado, de 2 MiB: un `tools/list` legítimo con un campo de relleno.
	// Si el tope desaparece, esto se parsea sin problema y contesta un result.
	cuerpo := `{"jsonrpc":"2.0","id":1,"method":"tools/list","relleno":"` +
		strings.Repeat("a", 2<<20) + `"}`
	resp := pedir(t, e, http.MethodPost, e.ruta, cuerpo)

	var r struct {
		Result json.RawMessage `json:"result"`
		Error  *errorRPC       `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("ni la respuesta de error es JSON: %v", err)
	}
	if r.Result != nil {
		t.Errorf("un cuerpo de %d bytes se parseó y contestó un result: el tope no está "+
			"cortando nada. result=%s", len(cuerpo), r.Result)
	}
	if r.Error == nil || r.Error.Code != -32700 {
		t.Fatalf("se esperaba el -32700 del cuerpo cortado, vino %+v", r.Error)
	}
}

// El JSON del `--mcp-config` es lo único que conecta al hijo con el espejo. Tiene que
// declarar el transporte HTTP —es lo que permite que el espejo viva dentro de este proceso—
// y apuntar a la URL con el token.
//
// Sabotaje M12: el espejo se declara por stdio (tendría que ser un segundo binario)
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\t\t\t\"type\": \"http\","
// arnes: a="\t\t\t\t\"type\": \"stdio\","
func TestElConfigMCPDeclaraElEspejoPorHTTP(t *testing.T) {
	e := espejoDePrueba(`[]`)

	var cfg struct {
		MCPServers map[string]struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(e.configMCP()), &cfg); err != nil {
		t.Fatalf("el config no es JSON válido: %v — %s", err, e.configMCP())
	}
	srv, ok := cfg.MCPServers[nombreDelEspejo]
	if !ok {
		t.Fatalf("el config no declara el servidor %q: %v", nombreDelEspejo, cfg.MCPServers)
	}
	if srv.Type != "http" {
		t.Errorf("type=%q: con stdio el espejo tendría que ser un segundo binario", srv.Type)
	}
	if srv.URL != e.url {
		t.Errorf("url=%q, se esperaba %q", srv.URL, e.url)
	}
}

// 🔴 LA CREDENCIAL DEL TURNO NO VIAJA EN EL ARGV. `e.url` lleva el token de la ruta secreta, y
// una línea de comandos no es privada: en Linux `/proc/<pid>/cmdline` es modo 444 y lo lee
// cualquier usuario de la máquina. Así que el JSON del `--mcp-config` se escribe en un archivo
// propio y lo que viaja en el argv es su ruta. Ver escribirConfig en espejo.go.
//
// Sabotaje M22: el archivo con la credencial sobrevive al turno
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tif e.dirConfig != \"\" {\n\t\t_ = os.RemoveAll(e.dirConfig)\n\t}\n"
// arnes: a=""
//
// Sabotaje M23: el archivo con el token queda legible por todo el mundo.
//
// ⚠️ EN WINDOWS ESTE SABOTAJE INFORMA VERDE, y no es un hueco del test: el sistema no tiene
// bits POSIX y Go sólo puede traducir el de escritura, así que `os.Stat` devuelve 0666 pase lo
// que pase y ninguna aserción puede distinguir 0600 de 0644. La comprobación se saltea ahí a
// propósito (el t.Skip implícito del `if` de abajo) y el que la mide es el CI en Linux.
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tif err := os.WriteFile(ruta, []byte(e.configMCP()), 0o600); err != nil {"
// arnes: a="\tif err := os.WriteFile(ruta, []byte(e.configMCP()), 0o644); err != nil {"
func TestLaConfigDelEspejoViveEnUnArchivoPropioYSeBorraAlCerrar(t *testing.T) {
	esp, err := abrirEspejo(Pedido{Tools: json.RawMessage(`[{"name":"leer"}]`)}, DemoraPorDefecto)
	if err != nil {
		t.Fatalf("abrirEspejo: %v", err)
	}
	// Idempotente: abajo se cierra a mano para medir el borrado, y este defer es el que cubre
	// los caminos de t.Fatal.
	defer esp.cerrar()

	if esp.rutaConfig == "" {
		t.Fatal("el espejo abierto no dejó la ruta de su config: `--mcp-config` se quedaría " +
			"con el mapa vacío y el turno sin una sola tool del cliente")
	}
	cuerpo, err := os.ReadFile(esp.rutaConfig)
	if err != nil {
		t.Fatalf("leer la config del espejo: %v", err)
	}
	// El archivo tiene que ser exactamente lo que antes iba inline: si no, el hijo no encuentra
	// el espejo y el turno vuelve en prosa, sin haber pedido ninguna tool.
	if string(cuerpo) != esp.configMCP() {
		t.Errorf("el archivo no es el config del espejo:\n  archivo:  %s\n  esperado: %s",
			cuerpo, esp.configMCP())
	}
	// Y tiene que llevar el token adentro, que es el punto: lo que se movió al archivo es la
	// credencial, no un JSON cualquiera.
	if !strings.Contains(string(cuerpo), esp.ruta) {
		t.Errorf("el archivo no contiene la ruta secreta del turno (%s): %s", esp.ruta, cuerpo)
	}

	if runtime.GOOS != "windows" {
		fi, err := os.Stat(esp.rutaConfig)
		if err != nil {
			t.Fatalf("stat de la config: %v", err)
		}
		if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Errorf("la config está en %04o y tiene el token del turno adentro: tiene que "+
				"ser 0600", perm)
		}
	}

	esp.cerrar()
	if _, err := os.Stat(esp.rutaConfig); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("la config con la credencial del turno sobrevivió al cierre (%s): %v",
			esp.rutaConfig, err)
	}
}

// 🔴 EL ÚNICO RENOMBRE DE LA VUELTA (los cuatro de la ida los enumera entrada.go:23-25; éste va
// al revés y lo pone el CLI), y el más traicionero porque NO FALLA: el CLI re-expone lo
// que viene por MCP como `mcp__pi__<tool>`, el cliente busca por el nombre que declaró y
// recibe uno que no tiene en su tabla. Los DOS eventos que llevan nombre de tool se tocan:
// olvidar uno es peor que no hacer nada, porque el cliente armaría el bloque con un nombre y
// lo cerraría con otro.
//
// Sabotaje M13: sólo se des-prefija el start, y el bloque se cierra con otro nombre
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\tif ev.ToolCall != nil && ev.ToolCall.Name != \"\" {"
// arnes: a="\t\tif false && ev.ToolCall != nil && ev.ToolCall.Name != \"\" {"
func TestElPrefijoDelCLISeDeshaceEnLosDosEventos(t *testing.T) {
	var vistos []Evento
	emitir := sinElPrefijoDelEspejo(func(ev Evento) error {
		vistos = append(vistos, ev)
		return nil
	})

	if err := emitir(Evento{Type: EvToolCallStart, ID: "t1", ToolName: prefijoDelEspejo + "leer"}); err != nil {
		t.Fatalf("emitir start: %v", err)
	}
	if err := emitir(Evento{Type: EvToolCallEnd, ToolCall: &ToolCall{
		Type: "tool_use", ID: "t1", Name: prefijoDelEspejo + "leer",
	}}); err != nil {
		t.Fatalf("emitir end: %v", err)
	}

	if len(vistos) != 2 {
		t.Fatalf("se esperaban 2 eventos, hay %d", len(vistos))
	}
	if vistos[0].ToolName != "leer" {
		t.Errorf("el toolcall_start quedó con el prefijo: %q", vistos[0].ToolName)
	}
	if vistos[1].ToolCall == nil || vistos[1].ToolCall.Name != "leer" {
		t.Errorf("el toolcall_end quedó con el prefijo: %+v", vistos[1].ToolCall)
	}
}

// Una tool que NO viene del espejo se deja tal cual. Por el mismo servidor pueden venir
// nombres que ya tienen forma `mcp__algo__tool` —pi declara sus propias tools MCP así—, y
// renombrar uno de ésos sería inventarle al cliente una tool que no declaró.
func TestUnNombreAjenoAlEspejoNoSeToca(t *testing.T) {
	for _, nombre := range []string{
		"leer",
		"mcp__otro__leer",
		"mcp__pi_largo__leer", // parecido al prefijo, pero no es
		"Read",
	} {
		if got := nombreSinElPrefijoDelEspejo(nombre); got != nombre {
			t.Errorf("%q volvió como %q", nombre, got)
		}
	}
}

// El envoltorio COPIA el ToolCall en vez de escribir adentro del que le prestaron. Hoy el
// traductor lo construye nuevo en cada cierre y mutarlo sería inocuo; es la clase de atajo
// que funciona hasta el día que alguien cachea el evento del otro lado, y entonces el
// defecto no se ve en este archivo.
//
// Sabotaje M14: el envoltorio escribe adentro de la estructura del llamador
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\t\ttc := *ev.ToolCall\n\t\t\ttc.Name = nombreSinElPrefijoDelEspejo(tc.Name)\n\t\t\tev.ToolCall = &tc"
// arnes: a="\t\t\tev.ToolCall.Name = nombreSinElPrefijoDelEspejo(ev.ToolCall.Name)"
func TestElEnvoltorioNoMutaElToolCallQueLePrestaron(t *testing.T) {
	original := &ToolCall{Type: "tool_use", ID: "t1", Name: prefijoDelEspejo + "leer"}
	emitir := sinElPrefijoDelEspejo(func(Evento) error { return nil })

	if err := emitir(Evento{Type: EvToolCallEnd, ToolCall: original}); err != nil {
		t.Fatalf("emitir: %v", err)
	}
	if original.Name != prefijoDelEspejo+"leer" {
		t.Errorf("el envoltorio escribió adentro de la estructura del llamador: %q", original.Name)
	}
}

// Un error del consumidor tiene que volver tal cual: el envoltorio no puede comerse la señal
// con la que el Lanzador distingue «el cliente se fue» de «el turno se sirvió».
func TestElEnvoltorioDevuelveElErrorDelConsumidor(t *testing.T) {
	emitir := sinElPrefijoDelEspejo(func(Evento) error { return ErrDelConsumidor })
	if err := emitir(Evento{Type: EvStart}); !errors.Is(err, ErrDelConsumidor) {
		t.Errorf("el error del consumidor no volvió: %v", err)
	}
}

// 🔴 LOS CUATRO TIMEOUTS DEL LISTENER, Y LA RELACIÓN ENTRE DOS DE ELLOS. Acá había un
// `ReadHeaderTimeout` solo, con un comentario que lo declaraba «la única forma de colgar esto»:
// era falso, dejaba sin plazo el CUERPO del request y la espera entre requests keep-alive.
//
// ⚠️ LA ASERCIÓN QUE PARECE DE MÁS ES LA QUE SALVA EL ARREGLO. Poner sólo `ReadTimeout` habría
// sido PEOR que no tocar nada: con `IdleTimeout` en cero, net/http usa el valor de `ReadTimeout`
// para la espera del próximo request en una conexión viva, así que un `ReadTimeout` corto le
// corta al hijo la conexión keep-alive mientras el modelo piensa — y pensar es lo que el hijo
// hace casi todo el turno. Por eso se mide que el idle le GANE al plazo del turno: el que tiene
// que cortar un turno eterno es el reloj del lanzador, que devuelve un error que el cliente
// entiende, no un timeout de este listener que el hijo ve como un socket muerto.
//
// 🔴 Y FALTABA LA ASERCIÓN QUE MIDE EL MARGEN — el defecto que encontró la revisión. La única
// que había pedía `IdleTimeout > DemoraPorDefecto`, o sea que cualquier epsilon la contentaba:
// bajar el minuto de más a un NANOSEGUNDO dejaba la suite ENTERA en verde. No es una inferencia,
// está medido (2026-10-29, `go test -overlay` sobre el árbol quieto: exit 0, cero `--- FAIL`).
// El piso de abajo va con un literal PROPIO y NO con `margenDelIdle`: una aserción que nombrara
// la constante se movería con ella, y el margen seguiría sin tener quien lo mida — que es la
// misma trampa, escrita de nuevo.
//
// Sabotaje M32: un timeout de menos y una conexión ociosa se queda con su goroutine
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\tIdleTimeout: espera + margenDelIdle,\n"
// arnes: a=""
// arnes: colision_ok="TestElEspejoTieneLosCuatroTimeoutsYElIdleLeGanaAlTurno TestElIdleDelEspejoCuelgaDelPlazoRealDelTurno"
//
// Sabotaje M33: el idle baja a la escala del read y le corta el keep-alive al hijo que piensa
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\tIdleTimeout: espera + margenDelIdle,"
// arnes: a="\t\tIdleTimeout: 15 * time.Second,"
// arnes: colision_ok="TestElEspejoTieneLosCuatroTimeoutsYElIdleLeGanaAlTurno TestElIdleDelEspejoCuelgaDelPlazoRealDelTurno"
//
// Sabotaje M37: el margen se vuelve un epsilon y el techo del turno deja de ganar SIEMPRE
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="const margenDelIdle = time.Minute"
// arnes: a="const margenDelIdle = time.Nanosecond"
//
// ⚠️ M32, M33 Y M38 SE PISAN (M38 vive sobre el test de al lado): el `de` de M33 y el de M38 son
// la misma línea sin el `\n`, o sea un PREFIJO del de M32, así que aplicar cualquiera borra el
// ancla de los otros. Y la respuesta va en TODAS, no una vez por grupo — eso lo probó el censo
// acá mismo: con la clave sólo en M33, el aviso de que M32 pisa a M33 seguía saliendo. Cada
// directiva contesta por el daño que HACE ELLA, así que la que no declara nada sigue sin
// respuesta. Los tres motivos son distintos: M32 pregunta si el idle está puesto; M33, si está
// en la escala correcta; M38, si cuelga del plazo REAL o de una constante.
// M37 NO entra en ese grupo: ancla sobre la constante, que es otra línea, y por eso cae sólo
// sobre el piso del margen — es el sabotaje más quirúrgico de los cuatro.
func TestElEspejoTieneLosCuatroTimeoutsYElIdleLeGanaAlTurno(t *testing.T) {
	e, err := abrirEspejo(Pedido{Tools: json.RawMessage(`[{"name":"leer","input_schema":{"type":"object"}}]`)}, DemoraPorDefecto)
	if err != nil {
		t.Fatalf("abrirEspejo: %v", err)
	}
	if e == nil {
		t.Fatal("el pedido traía tools: tenía que abrirse un espejo")
	}
	defer e.cerrar()

	for _, c := range []struct {
		nombre string
		v      time.Duration
	}{
		{"ReadHeaderTimeout", e.srv.ReadHeaderTimeout},
		{"ReadTimeout", e.srv.ReadTimeout},
		{"IdleTimeout", e.srv.IdleTimeout},
		{"WriteTimeout", e.srv.WriteTimeout},
	} {
		if c.v <= 0 {
			// Lo que estos cuatro plazos compran es ACOTAR, no eliminar, y decir otra cosa sería
			// sobreafirmar el arreglo: con ellos puestos, la misma conexión igual se queda hasta
			// el idle y se puede repetir tantas veces como quiera el que la abre, porque un 404
			// conserva el keep-alive y entonces no se paga ni el ReadHeaderTimeout ni el
			// ReadTimeout. Acotar es lo que hace falta acá; el tope de CUÁNTAS es otro eje.
			t.Errorf("%s=%v: sin plazo, un proceso local que abre una conexión y no la cierra se "+
				"queda con una goroutine y un descriptor hasta que muera el proceso —que acá es "+
				"un `musubi serve` que no se muere nunca—. Con los cuatro puestos, esa misma "+
				"conexión queda acotada al idle (%v), repetible pero no eterna",
				c.nombre, c.v, e.srv.IdleTimeout)
		}
	}

	// El idle tiene que pasar el plazo del turno, o el listener corta antes que el lanzador.
	if e.srv.IdleTimeout <= DemoraPorDefecto {
		t.Errorf("IdleTimeout=%v no le gana al plazo del turno (%v): el keep-alive del hijo se "+
			"corta mientras el modelo piensa, y el hijo lo ve como un servidor MCP caído",
			e.srv.IdleTimeout, DemoraPorDefecto)
	}
	// Y las dos escalas tienen que seguir separadas: igualarlas es volver al bug por otro lado.
	if e.srv.ReadTimeout >= e.srv.IdleTimeout {
		t.Errorf("ReadTimeout=%v no es más corto que IdleTimeout=%v: son plazos de cosas distintas "+
			"—un request a medio mandar y una conexión en silencio— y con la misma escala uno de los "+
			"dos está mal", e.srv.ReadTimeout, e.srv.IdleTimeout)
	}

	// 🔑 EL PISO DEL MARGEN, con un literal PROPIO (ver el bloque de arriba: nombrar
	// `margenDelIdle` acá sería escribir la misma trampa otra vez). Lo que se pide no es que el
	// margen coincida con la constante del día, sino que sea una duración DE VERDAD: entre que
	// el reloj del lanzador decide cortar y que este listener lo haría pasan el mata-proceso y
	// la cosecha del hijo, así que un epsilon deja la carrera abierta.
	const pisoDelMargen = 30 * time.Second
	if margen := e.srv.IdleTimeout - DemoraPorDefecto; margen < pisoDelMargen {
		t.Errorf("el margen del idle sobre el plazo del turno es %v, por debajo del piso de %v: "+
			"con un margen así el listener puede cortar antes que el reloj del lanzador, que es "+
			"el único de los dos que le devuelve al cliente un error que entiende",
			margen, pisoDelMargen)
	}
}

// 🔴 EL IDLE CUELGA DEL PLAZO REAL DEL TURNO, NO DE UNA CONSTANTE — y acá vivía el defecto.
// `IdleTimeout` era `DemoraPorDefecto + <margen>`, o sea 11 minutos FIJOS, mientras el plazo del
// turno sale de `Lanzador.Espera`, que [factory.go](factory.go) llena con `turn_timeout_seconds`
// del yaml y `internal/config` NO acota. Con `turn_timeout_seconds: 700` el listener cortaba a
// los 660 s: el hijo veía su servidor MCP caído a mitad de un turno que todavía tenía permiso
// para pensar — exactamente lo que el comentario de `espejo.go` promete que no pasa. **No era
// hipotético: es un valor del yaml.**
//
// SE ARREGLA EN EL ESPEJO Y NO ACOTANDO EL YAML. Una cota en `internal/config` también sirve, y
// queda anotada aparte como endurecimiento, pero dejaría la promesa dependiendo de que nadie
// vuelva a correr el tope; colgando del plazo real se cumple con cualquier valor.
//
// El CERO se mide en el mismo test porque es la otra mitad del acuerdo nuevo: un cero que cayera
// al `espera + margenDelIdle` daría un idle de UN MINUTO pelado, que es PEOR que el defecto
// original — le cortaría el keep-alive al hijo al minuto de estar pensando.
//
// ⚠️ LO QUE ESTE TEST NO MIDE: el TAMAÑO del margen. Sus dos aserciones piden `IdleTimeout >`
// (el plazo largo en una, `DemoraPorDefecto` en la del cero), y eso lo contenta un NANOSEGUNDO
// de margen igual que un minuto — la misma debilidad que el bloque de M32/M33 describe arriba.
// El piso lo mide `TestElEspejoTieneLosCuatroTimeoutsYElIdleLeGanaAlTurno`, con su
// `pisoDelMargen` literal, y el sabotaje que cae ahí es M37. Acá se mide DE QUÉ cuelga el
// idle; cuánto mide el margen se verifica allá.
//
// Sabotaje M38: el idle vuelve a colgar de la constante y el yaml puede invertir los relojes
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\tIdleTimeout: espera + margenDelIdle,"
// arnes: a="\t\tIdleTimeout: DemoraPorDefecto + margenDelIdle,"
// arnes: colision_ok="TestElEspejoTieneLosCuatroTimeoutsYElIdleLeGanaAlTurno"
func TestElIdleDelEspejoCuelgaDelPlazoRealDelTurno(t *testing.T) {
	const tools = `[{"name":"leer","input_schema":{"type":"object"}}]`

	// Muy por encima del default a propósito: con el idle colgado de la constante son 11 min
	// contra 30, y el listener corta primero.
	const plazoLargo = 30 * time.Minute
	e, err := abrirEspejo(Pedido{Tools: json.RawMessage(tools)}, plazoLargo)
	if err != nil {
		t.Fatalf("abrirEspejo: %v", err)
	}
	if e == nil {
		t.Fatal("el pedido traía tools: tenía que abrirse un espejo")
	}
	defer e.cerrar()

	if e.srv.IdleTimeout <= plazoLargo {
		t.Errorf("IdleTimeout=%v no le gana al plazo del turno (%v): el idle quedó colgado de una "+
			"constante en vez del plazo real, así que un turn_timeout_seconds grande hace que el "+
			"listener corte antes que el reloj del lanzador y el hijo lo vea como un servidor MCP "+
			"caído", e.srv.IdleTimeout, plazoLargo)
	}

	cero, err := abrirEspejo(Pedido{Tools: json.RawMessage(tools)}, 0)
	if err != nil {
		t.Fatalf("abrirEspejo con espera cero: %v", err)
	}
	if cero == nil {
		t.Fatal("el pedido traía tools: tenía que abrirse un espejo")
	}
	defer cero.cerrar()

	if cero.srv.IdleTimeout <= DemoraPorDefecto {
		t.Errorf("con espera=0 el IdleTimeout quedó en %v: un cero tiene que resolverse a "+
			"DemoraPorDefecto (%v), no sumarle el margen a cero — un idle de un minuto le corta "+
			"el keep-alive al hijo al minuto de pensar, que es el caso normal del turno largo",
			cero.srv.IdleTimeout, DemoraPorDefecto)
	}
}

// 🔴 UN SHUTDOWN QUE EXPIRA NO CERRÓ NADA, y creerle es cómo se fuga una goroutine por turno.
// Shutdown cierra los listeners y las conexiones OCIOSAS, pero a las que están a mitad de un
// request sólo las ESPERA: si el contexto vence, devuelve su error y las deja vivas. El proceso
// que las hospeda es `musubi serve`, que no se muere nunca, así que la fuga se acumula de a una.
//
// Y NO HACE FALTA LA CREDENCIAL PARA PROVOCARLO, que es lo que lo vuelve un asunto de seguridad
// y no de prolijidad: el plazo de lectura corre ANTES de que el handler mire la ruta, así que la
// ruta-secreta no defiende de esto. Cualquier proceso local puede abrir la conexión y no
// terminar de mandar el request.
//
// Sabotaje M34: el Shutdown vencido se da por bueno y la conexión sobrevive al turno
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\t_ = e.srv.Close()\n"
// arnes: a=""
//
// Y DE PASO ESTE ES EL ÚNICO TEST DE ESTE ARCHIVO QUE PASA POR EL LISTENER, así que de él cuelgan
// los dos sabotajes que nadie medía: el `Serve` que atiende y el handler que se le cuelga. Los
// demás tests de acá llaman `ServeHTTP` a mano con un httptest.Recorder —que es lo correcto para
// probar el protocolo— y por eso ninguno se enteraría de que el servidor real no está sirviendo
// nada.
//
// ⚠️ DECÍA «EL ÚNICO DEL PAQUETE» Y ERA FALSO, y era justo el argumento con el que se justificó
// colgar M35 y M36 de este test: `llamarAlEspejo` (`lanzador_test.go:97`) hace DOS `http.Post`
// REALES contra la URL del espejo desde el proceso hijo, así que los tests del lanzador que pasan
// tools también discan este socket. La corrección no es cosmética: es lo que explica el dato ya
// medido de que M35 y M36 ponen rojo TAMBIÉN al aviso de M30 —un hijo real no puede pedirle
// `tools/list` a un espejo que no sirve—, que con la frase vieja parecía una casualidad.
//
// Sabotaje M35: nadie acepta las conexiones del espejo (el listener queda abierto y mudo)
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\tgo func() { _ = e.srv.Serve(ln) }()\n"
// arnes: a=""
//
// Sabotaje M36: el listener sirve el mux global en vez del espejo
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\tHandler: e,"
// arnes: a="\t\tHandler: nil,"
func TestUnShutdownQueExpiraIgualCortaLaConexion(t *testing.T) {
	e, err := abrirEspejo(Pedido{Tools: json.RawMessage(`[{"name":"leer","input_schema":{"type":"object"}}]`)}, DemoraPorDefecto)
	if err != nil {
		t.Fatalf("abrirEspejo: %v", err)
	}
	if e == nil {
		t.Fatal("el pedido traía tools: tenía que abrirse un espejo")
	}

	dir, _, ok := strings.Cut(strings.TrimPrefix(e.url, "http://"), "/")
	if !ok {
		t.Fatalf("la URL del espejo no tiene la forma esperada: %q", e.url)
	}
	conn, err := net.Dial("tcp", dir)
	if err != nil {
		t.Fatalf("discar el espejo: %v", err)
	}
	defer conn.Close()

	// PRIMERO UN REQUEST COMPLETO, y su respuesta se lee entera. Eso hace dos cosas que el test
	// necesita: prueba que el servidor está atendiendo ESTE socket —no uno que quedó en el
	// backlog— y lo deja limpio, así que lo que se lea después del cierre no puede ser un resto
	// de esta respuesta haciéndose pasar por una conexión viva.
	cuerpo := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	primero := "POST " + e.ruta + " HTTP/1.1\r\nHost: 127.0.0.1\r\n" +
		"Content-Type: application/json\r\nContent-Length: " + strconv.Itoa(len(cuerpo)) + "\r\n\r\n" + cuerpo
	if _, err := conn.Write([]byte(primero)); err != nil {
		t.Fatalf("mandar el primer request: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	lector := bufio.NewReader(conn)
	resp, err := http.ReadResponse(lector, nil)
	if err != nil {
		t.Fatalf("el espejo no contestó el primer request: %v", err)
	}
	// Y QUE HAYA CONTESTADO BIEN, que no es celo de más: es la única aserción de todo el paquete
	// que toca el listener DE VERDAD —el `Serve` en su goroutine y el `Handler: e`— en vez del
	// ServeHTTP a mano. Sin esto, cambiar el handler por nil dejaba un 404 que este test se
	// tragaba sin chistar, y borrar el `Serve` quedaba igual de verde en los demás: el `pedir` de
	// arriba llama al método, no al socket, así que ninguno de los dos pasa por acá.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("el initialize por el socket real dio %d: el listener no está sirviendo este espejo",
			resp.StatusCode)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("drenar la respuesta: %v", err)
	}
	resp.Body.Close()

	// Y AHORA EL REQUEST QUE DEJA AL HANDLER COLGADO: headers COMPLETOS con un Content-Length de
	// 200 y un cuerpo de diecisiete bytes que nunca termina. El servidor lee los headers, entra
	// al handler, y el `Decode` se queda esperando el resto. Esa conexión es ACTIVA, y es el
	// único estado que Shutdown no cierra: a ésas sólo las ESPERA.
	//
	// ⚠️ Y NO SIRVE CORTAR EN LOS HEADERS, que es lo que este test intentaba primero: medido acá
	// mismo, el Shutdown volvió en 704 µs y la guarda del plazo lo denunció. El motivo está en
	// net/http: el `setState(StateActive)` se emite DESPUÉS de que `readRequest` devuelve, así
	// que una conexión bloqueada a mitad de los headers sigue contando como OCIOSA y Shutdown la
	// cierra sin dudar. Lo que hay que colgar es el handler, no el parser.
	parcial := `{"jsonrpc":"2.0",`
	segundo := "POST " + e.ruta + " HTTP/1.1\r\nHost: 127.0.0.1\r\n" +
		"Content-Type: application/json\r\nContent-Length: 200\r\n\r\n" + parcial
	if _, err := conn.Write([]byte(segundo)); err != nil {
		t.Fatalf("mandar el request colgado: %v", err)
	}
	// Lo que sigue no es un sleep decorativo: le da al servidor la chance de leer esos bytes y
	// marcar la conexión como activa. Si no alcanzara, la medición de abajo lo detecta y el test
	// lo dice en vez de aprobar — ver el Fatalf del plazo.
	time.Sleep(50 * time.Millisecond)

	arranque := time.Now()
	e.cerrar()
	tardo := time.Since(arranque)

	// QUE EL SHUTDOWN HAYA EXPIRADO DE VERDAD, antes de creerle nada al resultado. Si volvió
	// enseguida es que no tenía a nadie a quien esperar: la conexión estaba ociosa, la cerró el
	// Shutdown y el camino del Close NO SE EJERCIÓ. Un verde ahí sería el verde vacuo de siempre.
	if tardo < esperaDelCierreDelEspejo {
		t.Fatalf("cerrar() volvió en %v, antes del plazo de %v: el Shutdown no esperó a nadie, así "+
			"que este test no llegó a medir el Close", tardo, esperaDelCierreDelEspejo)
	}

	// LA MEDICIÓN: la conexión tiene que estar muerta YA. Se afirma por lo que NO puede pasar
	// —que la lectura se quede esperando—, y no por un error puntual: cortar un socket con bytes
	// sin leer da EOF en Unix y un reset en Windows, y pinear cualquiera de los dos haría fallar
	// el test en el otro sistema por algo que no es el defecto.
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var buf [1]byte
	_, errLectura := conn.Read(buf[:])
	var nerr net.Error
	if errors.As(errLectura, &nerr) && nerr.Timeout() {
		t.Error("la conexión sobrevivió al cierre del espejo: la lectura expiró en vez de cortarse. " +
			"El Shutdown vencido dejó viva una conexión a mitad de request, con su goroutine y su " +
			"descriptor, y el proceso que la hospeda no se muere nunca")
	}
}
