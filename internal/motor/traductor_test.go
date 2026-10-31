package motor

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// traducirArchivo corre el traductor sobre un fixture y devuelve los eventos y los errores
// de línea. Los fixtures salieron de `claude -p --output-format stream-json
// --include-partial-messages` ejecutado de verdad (2026-10-28) y después se curaron para
// sacarles los identificadores de sesión y las rutas de la máquina.
func traducirArchivo(t *testing.T, nombre string) ([]Evento, []error) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", nombre))
	if err != nil {
		t.Fatalf("abrir fixture: %v", err)
	}
	defer f.Close()

	tr := NuevoTraductor()
	var out []Evento
	var errs []error
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		linea := strings.TrimSpace(sc.Text())
		if linea == "" {
			continue
		}
		evs, err := tr.Linea([]byte(linea))
		if err != nil {
			errs = append(errs, err)
		}
		out = append(out, evs...)
		if tr.Cerrado() {
			// El consumidor real corta acá y mata el proceso. El test imita eso para
			// probar que lo de después se descarta: sigue leyendo, y si el traductor
			// emitiera algo, aparecería en out.
			continue
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("leer fixture: %v", err)
	}
	return out, errs
}

// traducirDesdeTurno es traducirArchivo entrando por el n-ésimo `message_start` del
// fixture. Hace falta porque el traductor corta en el primer turno a propósito, y algunos
// hechos del stream real sólo aparecen en los turnos siguientes (el thinking con su firma
// recién aparece cuando el modelo ya ejecutó una tool). Entrar por ahí no es inventar
// datos: son las líneas tal como las emitió el binario, y para el traductor ese
// `message_start` es el primero que ve — igual que cuando el modelo piensa desde el arranque.
func traducirDesdeTurno(t *testing.T, nombre string, n int) []Evento {
	t.Helper()
	crudo, err := os.ReadFile(filepath.Join("testdata", nombre))
	if err != nil {
		t.Fatalf("leer fixture: %v", err)
	}
	tr := NuevoTraductor()
	var out []Evento
	var visto int
	for _, linea := range strings.Split(string(crudo), "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" {
			continue
		}
		if visto < n {
			// Se parsea en vez de buscar el substring: el curador reescribió los fixtures
			// con json.dumps, que separa la clave del valor con un espacio, así que
			// `"type":"message_start"` no aparece literal en el archivo.
			var s sobreClaude
			if json.Unmarshal([]byte(linea), &s) == nil && s.Type == "stream_event" {
				var e eventoCrudo
				if json.Unmarshal(s.Event, &e) == nil && e.Type == "message_start" {
					visto++
				}
			}
			if visto < n {
				continue
			}
		}
		evs, err := tr.Linea([]byte(linea))
		if err != nil {
			t.Fatalf("Linea: %v", err)
		}
		out = append(out, evs...)
	}
	if visto < n {
		t.Fatalf("el fixture %s no tiene %d message_start", nombre, n)
	}
	return out
}

func tipos(evs []Evento) []string {
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		out = append(out, e.Type)
	}
	return out
}

// forma colapsa las corridas de un mismo tipo de evento a "tipo xN", para poder afirmar la
// secuencia sin atarla a la cantidad exacta de deltas que trajo la captura.
func forma(evs []Evento) []string {
	var out []string
	for i := 0; i < len(evs); {
		j := i
		for j < len(evs) && evs[j].Type == evs[i].Type {
			j++
		}
		if j-i > 1 {
			out = append(out, evs[i].Type+" xN")
		} else {
			out = append(out, evs[i].Type)
		}
		i = j
	}
	return out
}

// I3 — EL CORTE. El turno cierra en el primer `message_delta` y todo lo que sigue se
// descarta. Sin esto, `claude -p` —que es un agente con loop propio— emite un segundo
// `message_start` y el cliente queda reconstruyendo dos mensajes sobre el mismo parcial.
// El fixture tiene DOS turnos a propósito.
func TestCortaEnElPrimerTurno(t *testing.T) {
	evs, errs := traducirArchivo(t, "texto-y-tool-dos-turnos.jsonl")
	if len(errs) != 0 {
		t.Fatalf("el fixture no debería dar errores de línea: %v", errs)
	}

	var starts, dones int
	for _, e := range evs {
		switch e.Type {
		case EvStart:
			starts++
		case EvDone, EvError:
			dones++
		}
	}
	if starts != 1 {
		t.Errorf("start: got %d, want 1 (el fixture tiene 2 message_start)", starts)
	}
	if dones != 1 {
		t.Errorf("cierres: got %d, want 1", dones)
	}
	// El cierre tiene que ser el ÚLTIMO evento: nada se cuela después.
	n := len(evs)
	if n == 0 || (evs[n-1].Type != EvDone && evs[n-1].Type != EvError) {
		t.Fatalf("el último evento debe ser el cierre, got %v", tipos(evs))
	}
	// El primer turno del fixture termina en un tool call, así que la razón es toolUse y
	// NO stop: es el caso que le dice al cliente que tiene que ejecutar la tool y volver.
	fin := evs[n-1]
	if fin.Type != EvDone || fin.Reason != ReasonToolUse {
		t.Errorf("cierre: got %s/%s, want done/%s", fin.Type, fin.Reason, ReasonToolUse)
	}
	if fin.Usage == nil || fin.Usage.Output == 0 {
		t.Errorf("el done debe llevar el usage del turno, got %+v", fin.Usage)
	}
}

// I2 — EL TIPO DEL BLOQUE. Un `content_block_stop` viene pelado: no dice de qué bloque es.
// Si el traductor pierde ese estado y emite `text_end` sobre un tool_use, el cliente tira
// `Received text_end for non-text content` (proxy.ts:294) y se pierde la respuesta entera.
func TestElCierreDeBloqueRespetaSuTipo(t *testing.T) {
	evs, _ := traducirArchivo(t, "texto-y-tool-dos-turnos.jsonl")

	// El fixture abre texto en el índice 0 y un tool_use en el 1, y cierra con tool_use.
	quiero := []string{
		EvStart,
		EvTextStart, EvTextDelta + " xN", EvTextEnd,
		EvToolCallStart, EvToolCallDelta + " xN", EvToolCallEnd,
		EvDone,
	}
	if got := forma(evs); strings.Join(got, " · ") != strings.Join(quiero, " · ") {
		t.Errorf("secuencia:\n  got  %v\n  want %v", got, quiero)
	}
	// Y el cierre del tool_use es toolcall_end, no text_end.
	var vioToolEnd bool
	for _, e := range evs {
		if e.Type == EvToolCallEnd {
			vioToolEnd = true
			if e.ToolCall == nil {
				t.Fatal("toolcall_end sin toolCall: el cliente le hace Object.assign")
			}
			if e.ToolCall.Name == "" || e.ToolCall.ID == "" {
				t.Errorf("toolcall_end sin id/name: %+v", e.ToolCall)
			}
			if !json.Valid(e.ToolCall.Arguments) {
				t.Errorf("arguments no es JSON válido: %s", e.ToolCall.Arguments)
			}
		}
	}
	if !vioToolEnd {
		t.Error("no hubo toolcall_end")
	}
}

// La firma de un bloque de thinking NO es un delta de contenido: llega en su propio evento
// y el protocolo la espera en el `thinking_end`. Medido: 740 caracteres de una sola vez.
func TestLaFirmaDelThinkingSaleEnElEnd(t *testing.T) {
	// El thinking aparece en el segundo turno del fixture, porque el modelo piensa recién
	// después de haber visto el resultado de una tool.
	evs := traducirDesdeTurno(t, "thinking-con-firma.jsonl", 2)

	var fines int
	for _, e := range evs {
		if e.Type == EvThinkingDelta && e.Delta == "" {
			t.Error("thinking_delta con delta vacío: no debería emitirse")
		}
		if e.Type != EvThinkingEnd {
			continue
		}
		fines++
		if e.ContentSignature == "" {
			t.Fatal("thinking_end sin contentSignature: el fixture trae una firma de 740 caracteres")
		}
		if n := len(e.ContentSignature); n != 740 {
			t.Errorf("contentSignature: got %d caracteres, want 740 (se perdió un pedazo)", n)
		}
		// La firma va en el END, nunca en un delta: el protocolo no tiene dónde ponerla antes.
		bruto, _ := json.Marshal(e)
		if !strings.Contains(string(bruto), `"contentSignature"`) {
			t.Errorf("la firma no llegó al cable: %s", bruto)
		}
	}
	if fines != 1 {
		t.Errorf("thinking_end: got %d, want 1 (forma: %v)", fines, forma(evs))
	}

	// Y la firma NO viaja como delta de contenido.
	for _, e := range evs {
		if e.Type == EvThinkingDelta && len(e.Delta) == 740 {
			t.Error("la firma se emitió como thinking_delta: el cliente la mostraría como texto pensado")
		}
	}
}

// El primer `input_json_delta` de cada tool call llega con cadena vacía — medido en los
// tres bloques de los dos fixtures. Reenviarlo sería un evento que no mueve nada del otro
// lado, así que se descarta: los deltas emitidos son uno menos que los del stream.
func TestElDeltaVacioNoSeReenvia(t *testing.T) {
	evs, errs := traducirArchivo(t, "texto-y-tool-dos-turnos.jsonl")
	if len(errs) != 0 {
		t.Fatalf("errores de línea: %v", errs)
	}
	var deltas int
	for _, e := range evs {
		if e.Type == EvToolCallDelta {
			if e.Delta == "" {
				t.Error("se emitió un toolcall_delta vacío")
			}
			deltas++
		}
	}

	// El número se cuenta del fixture, no se escribe a mano: el curado re-partió los
	// argumentos y la cantidad de trozos vacíos es un detalle de ese curado, no del
	// protocolo. Lo que el test fija es la relación — se emiten todos menos los vacíos.
	conTexto, vacios := contarDeltasJSON(t, "texto-y-tool-dos-turnos.jsonl", 1)
	if vacios == 0 {
		t.Fatal("el fixture no tiene ningún input_json_delta vacío: el test no probaría nada")
	}
	if deltas != conTexto {
		t.Errorf("toolcall_delta: got %d, want %d (%d en el stream, %d vacíos)",
			deltas, conTexto, conTexto+vacios, vacios)
	}
	// Y los deltas concatenados tienen que dar el mismo JSON que el toolcall_end: es lo
	// que el cliente va reconstruyendo mientras llegan.
	var junto strings.Builder
	var fin *ToolCall
	for _, e := range evs {
		switch e.Type {
		case EvToolCallDelta:
			junto.WriteString(e.Delta)
		case EvToolCallEnd:
			fin = e.ToolCall
		}
	}
	if fin == nil {
		t.Fatal("sin toolcall_end")
	}
	if junto.String() != string(fin.Arguments) {
		t.Errorf("los deltas no reconstruyen los arguments:\n  deltas %s\n  end    %s", junto.String(), fin.Arguments)
	}
}

// I7 — DOS BLOQUES EN EL MISMO ÍNDICE NO MEZCLAN. Un stream de varios turnos reusa el
// índice de bloque, así que un acumulador por índice que no se resetee al cerrar pega los
// argumentos de dos tool calls y produce JSON inválido. Lo encontró el curador de los
// fixtures sobre `thinking-con-firma.jsonl`, que trae dos ToolSearch los dos en el índice 1.
func TestBloquesConsecutivosEnElMismoIndiceNoMezclan(t *testing.T) {
	// Se arma a mano para poder pasar los DOS turnos (el traductor corta en el primero,
	// así que el fixture real no alcanza para ver el segundo bloque).
	lineas := []string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"usage":{}}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_A","name":"Uno"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"a\":1}"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":1}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_B","name":"Dos"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"b\":2}"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":1}}`,
	}
	tr := NuevoTraductor()
	var fines []Evento
	for _, l := range lineas {
		evs, err := tr.Linea([]byte(l))
		if err != nil {
			t.Fatalf("línea %q: %v", l, err)
		}
		for _, e := range evs {
			if e.Type == EvToolCallEnd {
				fines = append(fines, e)
			}
		}
	}
	if len(fines) != 2 {
		t.Fatalf("toolcall_end: got %d, want 2", len(fines))
	}
	if got := string(fines[0].ToolCall.Arguments); got != `{"a":1}` {
		t.Errorf("primer bloque: got %s, want {\"a\":1}", got)
	}
	if got := string(fines[1].ToolCall.Arguments); got != `{"b":2}` {
		t.Errorf("segundo bloque: got %s, want {\"b\":2} (se mezclaron los argumentos)", got)
	}
	if fines[0].ToolCall.ID == fines[1].ToolCall.ID {
		t.Error("los dos toolcall_end comparten id: se reusó el bloque viejo")
	}
}

// I1 — LAS CLAVES DEL CABLE. El cliente discrimina por `type` en un switch y su `default`
// sólo hace console.warn y devuelve undefined (proxy.ts:387): una clave mal escrita NO
// falla, se pierde en silencio. Este test fija el contrato de nombres.
func TestLasClavesSonLasDelProtocolo(t *testing.T) {
	casos := []struct {
		nombre string
		ev     Evento
		claves []string
	}{
		{"start", Evento{Type: EvStart}, []string{"type"}},
		{"text_start indice 0", Evento{Type: EvTextStart, ContentIndex: 0}, []string{"contentIndex", "type"}},
		{"text_delta", Evento{Type: EvTextDelta, ContentIndex: 2, Delta: "hola"}, []string{"contentIndex", "delta", "type"}},
		{"text_end", Evento{Type: EvTextEnd, ContentIndex: 0}, []string{"contentIndex", "type"}},
		{"thinking_end con firma", Evento{Type: EvThinkingEnd, ContentIndex: 0, ContentSignature: "f"}, []string{"contentIndex", "contentSignature", "type"}},
		{"toolcall_start", Evento{Type: EvToolCallStart, ContentIndex: 1, ID: "toolu_x", ToolName: "Read"}, []string{"contentIndex", "id", "toolName", "type"}},
		{"toolcall_end", Evento{Type: EvToolCallEnd, ContentIndex: 1, ToolCall: &ToolCall{Type: "toolCall", ID: "i", Name: "n", Arguments: json.RawMessage("{}")}}, []string{"contentIndex", "toolCall", "type"}},
		{"done", Evento{Type: EvDone, Reason: ReasonStop, Usage: &Usage{}}, []string{"reason", "type", "usage"}},
		{"error", Evento{Type: EvError, Reason: ReasonError, ErrorMessage: "x", Usage: &Usage{}}, []string{"errorMessage", "reason", "type", "usage"}},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			bruto, err := json.Marshal(c.ev)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(bruto, &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got := make([]string, 0, len(m))
			for k := range m {
				got = append(got, k)
			}
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(c.claves, ",") {
				t.Errorf("claves: got %v, want %v\n  json: %s", got, c.claves, bruto)
			}
		})
	}

	// El caso que omitempty rompería solo: contentIndex 0 es el bloque más común.
	bruto, _ := json.Marshal(Evento{Type: EvTextDelta, ContentIndex: 0, Delta: "x"})
	if !strings.Contains(string(bruto), `"contentIndex":0`) {
		t.Errorf("contentIndex 0 desapareció del cable: %s", bruto)
	}
}

// I4 — EL CONJUNTO CERRADO DE RAZONES. `done` sólo admite stop|length|toolUse
// (proxy.ts:36). Un valor fuera de eso no es un dato raro: es un tipo roto del otro lado,
// así que se reporta como error con su nombre en vez de disfrazarse de "stop".
//
// El mapeo copia al de pi (anthropic-messages.ts:1664-1690) PORQUE un cliente no puede ver
// una respuesta distinta según de qué lado le hablen. De ahí que `pause_turn` sea `stop`:
// allá está anotado «stop is good enough -> resubmit», y reportarlo como error dejaba al
// cliente terminando en falla donde su propio proveedor termina bien.
func TestRazonDeCorte(t *testing.T) {
	casos := map[string]struct {
		quiero string
		ok     bool
	}{
		"end_turn":      {ReasonStop, true},
		"stop_sequence": {ReasonStop, true},
		"tool_use":      {ReasonToolUse, true},
		"max_tokens":    {ReasonLength, true},
		"pause_turn":    {ReasonStop, true},
		"refusal":       {"", false},
		"sensitive":     {"", false},
		"":              {"", false},
	}
	for bruto, c := range casos {
		got, ok := razonDeCorte(bruto)
		if ok != c.ok || got != c.quiero {
			t.Errorf("razonDeCorte(%q) = (%q,%v), want (%q,%v)", bruto, got, ok, c.quiero, c.ok)
		}
	}

	// Y el camino completo: un stop_reason no soportado sale como error, no como done.
	tr := NuevoTraductor()
	evs, err := tr.Linea([]byte(`{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"refusal"},"usage":{"input_tokens":5}}}`))
	if err != nil {
		t.Fatalf("Linea: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != EvError {
		t.Fatalf("un refusal debe salir como error, got %v", tipos(evs))
	}
	if !strings.Contains(evs[0].ErrorMessage, "refusal") {
		t.Errorf("el error debe nombrar el stop_reason, got %q", evs[0].ErrorMessage)
	}
	if evs[0].Usage == nil || evs[0].Usage.Input != 5 {
		t.Errorf("el error debe llevar el usage que sí llegó, got %+v", evs[0].Usage)
	}
}

// I6 — LA TRADUCCIÓN DEL USAGE. El stream habla `input_tokens`/`cache_read_input_tokens`
// y el cliente espera `input`/`cacheRead`. Un campo mal mapeado llega como cero y el
// contador del cliente miente sin avisar.
func TestUsoSeTraduceALosNombresDePi(t *testing.T) {
	u := traducirUso(&usoCrudo{
		InputTokens: 2, OutputTokens: 140,
		CacheReadInputTokens: 15842, CacheCreationInputTokens: 8286,
	})
	if u.Input != 2 || u.Output != 140 || u.CacheRead != 15842 || u.CacheWrite != 8286 {
		t.Errorf("mapeo: %+v", u)
	}
	if want := 2 + 140 + 15842 + 8286; u.TotalTokens != want {
		t.Errorf("totalTokens: got %d, want %d (el stream no lo trae, se calcula)", u.TotalTokens, want)
	}

	// Las claves se comparan como SET del nivel superior y no con strings.Contains: el
	// `cost` anidado repite los nombres `cacheRead`/`cacheWrite`, así que un substring los
	// encuentra ahí adentro y el test pasa aunque el tag de Usage esté roto. Pasó de
	// verdad: el sabotaje de este invariante no se atrapaba por eso.
	bruto, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(bruto, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	quiero := []string{"cacheRead", "cacheWrite", "cost", "input", "output", "totalTokens"}
	if strings.Join(got, ",") != strings.Join(quiero, ",") {
		t.Errorf("claves del usage: got %v, want %v\n  json: %s", got, quiero, bruto)
	}
	// El usage nil no es cero-con-datos: es cero declarado.
	if got := traducirUso(nil); got != (Usage{}) {
		t.Errorf("traducirUso(nil) = %+v, want cero", got)
	}
}

// I5 — ARGUMENTOS TRUNCADOS. Un tool call cuyo JSON llegó cortado no se descarta: el
// cliente ya lo fue reconstruyendo delta por delta, y lo que necesita del `toolcall_end`
// es el id y el nombre. Lo que NO puede recibir es JSON inválido, que lo rompería.
//
// Y tiene que recibir el MOTIVO: un `{}` a secas pisa el parcial que el cliente venía
// armando (el Object.assign de proxy.ts:358) y su única guarda —`if
// (toolCall.argumentsError) throw`, validation.ts:318-319— mira un campo que, sin esto,
// nunca llega. El resultado era ejecutar la tool con un objeto vacío.
func TestArgumentosTruncadosNoEmitenJSONInvalido(t *testing.T) {
	casos := []struct {
		entrada  string
		quiero   string
		conError bool
	}{
		{`{"a":1}`, `{"a":1}`, false},
		{`{"a":`, `{}`, true},
		{``, `{}`, false}, // sin argumentos no es un error: la tool no recibe nada y punto
		{`no json`, `{}`, true},
		{`{"a":"ñ"}`, `{"a":"ñ"}`, false},
	}
	for _, c := range casos {
		got, errArgs := argumentos([]byte(c.entrada))
		if string(got) != c.quiero {
			t.Errorf("argumentos(%q) = %s, want %s", c.entrada, got, c.quiero)
		}
		if !json.Valid(got) {
			t.Errorf("argumentos(%q) devolvió JSON inválido: %s", c.entrada, got)
		}
		if (errArgs != "") != c.conError {
			t.Errorf("argumentos(%q): error = %q, want conError=%v", c.entrada, errArgs, c.conError)
		}
		// El texto le llega al MODELO, así que tiene que incluir lo que mandó: sin eso
		// no puede corregirse en el intento siguiente.
		if c.conError && !strings.Contains(errArgs, c.entrada) {
			t.Errorf("argumentos(%q): el error no incluye el texto recibido: %q", c.entrada, errArgs)
		}
	}
}

// El `argumentsError` tiene que llegar hasta el cable, no quedarse en la función: el
// toolcall_end de un tool call truncado lo lleva, y el de uno sano NO (un
// `argumentsError:""` también dispara la guarda del cliente, de ahí el omitempty).
func TestElToolCallEndLlevaElErrorDeArgumentos(t *testing.T) {
	stream := func(partial string) Evento {
		tr := NuevoTraductor()
		lineas := []string{
			`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"Read"}}}`,
			`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":` + mustJSON(partial) + `}}}`,
			`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
		}
		var fin Evento
		for _, l := range lineas {
			evs, err := tr.Linea([]byte(l))
			if err != nil {
				t.Fatalf("línea %q: %v", l, err)
			}
			for _, e := range evs {
				if e.Type == EvToolCallEnd {
					fin = e
				}
			}
		}
		if fin.ToolCall == nil {
			t.Fatalf("no llegó el toolcall_end con partial %q", partial)
		}
		return fin
	}

	sano := stream(`{"path":"x"}`)
	if sano.ToolCall.ArgumentsError != "" {
		t.Errorf("tool call sano con argumentsError: %q", sano.ToolCall.ArgumentsError)
	}
	bruto, err := json.Marshal(sano)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bruto), "argumentsError") {
		t.Errorf("el toolcall_end sano emite argumentsError en el cable: %s", bruto)
	}

	roto := stream(`{"path":"x`)
	if roto.ToolCall.ArgumentsError == "" {
		t.Error("tool call truncado SIN argumentsError: el cliente lo ejecutaría con {}")
	}
	if string(roto.ToolCall.Arguments) != "{}" {
		t.Errorf("arguments del truncado = %s, want {}", roto.ToolCall.Arguments)
	}
	bruto, err = json.Marshal(roto)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bruto), `"argumentsError"`) {
		t.Errorf("el argumentsError no llegó al cable: %s", bruto)
	}
}

func mustJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// Un bloque que se cierra sin haberse abierto no se puede traducir sin adivinar el tipo, y
// adivinar mal rompe el stream del cliente. Tiene que dar error y NO emitir nada.
func TestCerrarUnBloqueQueNoSeAbrioEsError(t *testing.T) {
	tr := NuevoTraductor()
	evs, err := tr.Linea([]byte(`{"type":"stream_event","event":{"type":"content_block_stop","index":7}}`))
	if err == nil {
		t.Error("se esperaba error")
	}
	if len(evs) != 0 {
		t.Errorf("no debe emitir nada, got %v", tipos(evs))
	}

	// Lo mismo para un delta huérfano.
	evs, err = tr.Linea([]byte(`{"type":"stream_event","event":{"type":"content_block_delta","index":7,"delta":{"type":"text_delta","text":"x"}}}`))
	if err == nil {
		t.Error("un delta sin start debe dar error")
	}
	if len(evs) != 0 {
		t.Errorf("no debe emitir nada, got %v", tipos(evs))
	}
}

// Los eventos del harness (`system`, `assistant`, `user`, `rate_limit_event`) no son del
// protocolo y se ignoran. Están en los fixtures a propósito.
func TestLosEventosDelHarnessSeIgnoran(t *testing.T) {
	tr := NuevoTraductor()
	for _, l := range []string{
		`{"type":"system","subtype":"init","session_id":"x"}`,
		`{"type":"rate_limit_event"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"x"}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"x"}]}}`,
		`{"type":"stream_event","event":{"type":"message_stop"}}`,
	} {
		evs, err := tr.Linea([]byte(l))
		if err != nil {
			t.Errorf("%q: %v", l, err)
		}
		if len(evs) != 0 {
			t.Errorf("%q emitió %v", l, tipos(evs))
		}
	}
}

// Una línea ilegible no aborta el turno: se devuelve el error para registrarlo y se sigue.
// El harness intercala líneas que no son del protocolo y una sola mal formada no debería
// tirar una respuesta entera.
func TestUnaLineaIlegibleNoCierraElTurno(t *testing.T) {
	tr := NuevoTraductor()
	if _, err := tr.Linea([]byte(`{roto`)); err == nil {
		t.Error("se esperaba error")
	}
	if tr.Cerrado() {
		t.Fatal("una línea ilegible NO debe cerrar el turno")
	}
	evs, err := tr.Linea([]byte(`{"type":"stream_event","event":{"type":"message_start","message":{"usage":{}}}}`))
	if err != nil || len(evs) != 1 || evs[0].Type != EvStart {
		t.Errorf("el turno debe seguir vivo, got %v err=%v", tipos(evs), err)
	}
}

// Después del cierre no se emite NADA, ni siquiera ante eventos válidos. Es la otra mitad
// del invariante del corte: el traductor no depende de que el llamador lo frene a tiempo.
func TestDespuesDelCierreNoEmiteNada(t *testing.T) {
	tr := NuevoTraductor()
	mustLinea(t, tr, `{"type":"stream_event","event":{"type":"message_start","message":{"usage":{}}}}`)
	evs := mustLinea(t, tr, `{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{}}}`)
	if len(evs) != 1 || evs[0].Type != EvDone {
		t.Fatalf("esperaba done, got %v", tipos(evs))
	}
	if !tr.Cerrado() {
		t.Fatal("Cerrado() debe ser true")
	}
	for _, l := range []string{
		`{"type":"stream_event","event":{"type":"message_start","message":{"usage":{}}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text"}}}`,
	} {
		if evs := mustLinea(t, tr, l); len(evs) != 0 {
			t.Errorf("después del cierre emitió %v", tipos(evs))
		}
	}
}

// contarDeltasJSON cuenta los input_json_delta del turno n del fixture, separando los que
// traen texto de los vacíos. Existe para que el test no hornee un número que en realidad
// depende de cómo quedó el curado del fixture.
func contarDeltasJSON(t *testing.T, nombre string, n int) (conTexto, vacios int) {
	t.Helper()
	crudo, err := os.ReadFile(filepath.Join("testdata", nombre))
	if err != nil {
		t.Fatalf("leer fixture: %v", err)
	}
	var turno int
	for _, linea := range strings.Split(string(crudo), "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" {
			continue
		}
		var s sobreClaude
		if json.Unmarshal([]byte(linea), &s) != nil || s.Type != "stream_event" {
			continue
		}
		var e eventoCrudo
		if json.Unmarshal(s.Event, &e) != nil {
			continue
		}
		if e.Type == "message_start" {
			turno++
		}
		if turno != n || e.Type != "content_block_delta" || e.Delta == nil {
			continue
		}
		if e.Delta.Type != "input_json_delta" {
			continue
		}
		if e.Delta.PartialJSON == "" {
			vacios++
		} else {
			conTexto++
		}
	}
	return conTexto, vacios
}

func mustLinea(t *testing.T, tr *Traductor, l string) []Evento {
	t.Helper()
	evs, err := tr.Linea([]byte(l))
	if err != nil {
		t.Fatalf("Linea(%q): %v", l, err)
	}
	return evs
}

// I14 — EL ERROR DEL PROVEEDOR CIERRA EL TURNO. Cuando la API se cae a mitad del stream
// manda un evento `error` y NO manda ningún `message_delta`. Sin traducirlo, el turno se
// quedaba esperando un cierre que no iba a llegar: el request colgaba hasta el tope de
// tiempo —diez minutos por defecto— y el cliente terminaba mostrando su fallback genérico
// de desconexión en vez del motivo que la API había dicho.
func TestElErrorDelProveedorCierraElTurno(t *testing.T) {
	tr := NuevoTraductor()
	mustLinea(t, tr, `{"type":"stream_event","event":{"type":"message_start","message":{"usage":{"input_tokens":120,"cache_read_input_tokens":4000}}}}`)
	evs := mustLinea(t, tr, `{"type":"stream_event","event":{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}}`)

	if len(evs) != 1 || evs[0].Type != EvError {
		t.Fatalf("esperaba un error, got %v", tipos(evs))
	}
	if !tr.Cerrado() {
		t.Error("el turno tiene que quedar cerrado: después del error no llega el message_delta")
	}
	// El motivo tiene que viajar: es lo único que distingue «el proveedor está saturado»
	// de «se cortó la red», y el cliente lo muestra tal cual.
	for _, quiero := range []string{"overloaded_error", "Overloaded"} {
		if !strings.Contains(evs[0].ErrorMessage, quiero) {
			t.Errorf("el mensaje no menciona %q: %q", quiero, evs[0].ErrorMessage)
		}
	}
	// Los tokens de entrada YA se consumieron aunque la respuesta no llegara.
	if evs[0].Usage == nil || evs[0].Usage.Input != 120 || evs[0].Usage.CacheRead != 4000 {
		t.Errorf("el usage parcial no viajó: %+v", evs[0].Usage)
	}
	// Y después del error se descarta todo, igual que después de un done.
	if evs := mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text"}}}`); len(evs) != 0 {
		t.Errorf("emitió %v después del error", tipos(evs))
	}
}

// I15 — LOS ÍNDICES VIAJAN DENSOS. El cliente hace `partial.content[contentIndex] = bloque`
// sobre un array (proxy.ts:266), así que un índice salteado queda como `null`. Si la API
// abre en el 0 un bloque que este traductor no anuncia y el texto llega en el 1, el cliente
// arma `content = [null, {...}]` y quien consuma ese array se encuentra un agujero que nadie
// puso ahí. Los índices que salen son propios y consecutivos.
func TestUnBloqueDesconocidoNoDejaUnHuecoEnElContenido(t *testing.T) {
	tr := NuevoTraductor()
	mustLinea(t, tr, `{"type":"stream_event","event":{"type":"message_start","message":{"usage":{}}}}`)

	// Índice 0: un tipo que el protocolo no tiene. No se anuncia y NO consume índice.
	if evs := mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"web_search_tool_result"}}}`); len(evs) != 0 {
		t.Fatalf("un bloque desconocido no se anuncia, got %v", tipos(evs))
	}
	// Índice 1 de la API: para el cliente tiene que ser el 0.
	evs := mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"text"}}}`)
	if len(evs) != 1 || evs[0].Type != EvTextStart {
		t.Fatalf("esperaba text_start, got %v", tipos(evs))
	}
	if evs[0].ContentIndex != 0 {
		t.Errorf("text_start en contentIndex %d: el cliente deja content[0]=null", evs[0].ContentIndex)
	}
	// Y el delta y el end del MISMO bloque tienen que usar el mismo índice denso, o el
	// cliente los aplica sobre un bloque que no existe y tira.
	evs = mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"hola"}}}`)
	if len(evs) != 1 || evs[0].ContentIndex != 0 {
		t.Errorf("el delta salió en %v con índice distinto del start: %+v", tipos(evs), evs)
	}
	evs = mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_stop","index":1}}`)
	if len(evs) != 1 || evs[0].Type != EvTextEnd || evs[0].ContentIndex != 0 {
		t.Errorf("el end salió con índice distinto del start: %v %+v", tipos(evs), evs)
	}
}

// I16 — EL USAGE DEL ARRANQUE NO SE PIERDE. Los tokens de entrada y de caché llegan SÓLO en
// el `message_start`; el `message_delta` final puede traer nada más que `output_tokens`.
// Reportando el del delta pelado, un turno que leyó 15.842 tokens de caché se declaraba con
// cacheRead en cero y el contador del cliente subdeclaraba el consumo real.
func TestElUsageDelDeltaNoBorraElDelArranque(t *testing.T) {
	tr := NuevoTraductor()
	mustLinea(t, tr, `{"type":"stream_event","event":{"type":"message_start","message":{"usage":{"input_tokens":100,"output_tokens":1,"cache_read_input_tokens":5000,"cache_creation_input_tokens":200}}}}`)
	evs := mustLinea(t, tr, `{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":42}}}`)

	if len(evs) != 1 || evs[0].Type != EvDone {
		t.Fatalf("esperaba done, got %v", tipos(evs))
	}
	quiero := Usage{Input: 100, Output: 42, CacheRead: 5000, CacheWrite: 200, TotalTokens: 5342}
	if got := *evs[0].Usage; got != quiero {
		t.Errorf("usage = %+v, want %+v", got, quiero)
	}
}

// I17 — UN BLOQUE QUE NO SE ANUNCIÓ NO EMITE DELTAS. Mandar un `text_delta` sin su
// `text_start` no degrada: el cliente tira `Received text_delta for non-text content`
// (proxy.ts:352) y su catch lo deja con stopReason=error y el contenido vacío. O sea que un
// solo bloque de un tipo que todavía no conocemos se llevaría puesto el turno entero.
func TestElDeltaDeUnBloqueNoAnunciadoNoSeEmite(t *testing.T) {
	tr := NuevoTraductor()
	mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tipo_del_futuro"}}}`)

	// El bloque EXISTE (está en el mapa), así que esto no es un error de protocolo: es un
	// delta que simplemente no se reenvía.
	evs, err := tr.Linea([]byte(`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"fuga"}}}`))
	if err != nil {
		t.Fatalf("no tiene que dar error, el bloque se abrió: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("emitió %v: un delta sin su start rompe el stream del cliente", tipos(evs))
	}
	// Su cierre tampoco emite nada.
	if evs := mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`); len(evs) != 0 {
		t.Errorf("el cierre emitió %v", tipos(evs))
	}
}

// I18 — EL RAZONAMIENTO CIFRADO LLEGA ENTERO. Un `redacted_thinking` no trae texto, pero su
// `data` opaco tiene que viajar igual: el cliente lo guarda como la firma del bloque
// (anthropic-messages.ts:750) y lo devuelve como `redacted_thinking.data` en el request
// siguiente (:1446-1447). Sin él, el cliente no puede reconstruir el turno y la API le
// rechaza el intento que sigue.
func TestElRedactedThinkingLlevaSuPayloadEnLaFirma(t *testing.T) {
	tr := NuevoTraductor()
	evs := mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"redacted_thinking","data":"EroBCkYIBxgCKkDopaco"}}}`)

	if got := tipos(evs); strings.Join(got, ",") != EvThinkingStart+","+EvThinkingDelta {
		t.Fatalf("got %v, want thinking_start + thinking_delta", got)
	}
	// El texto es el placeholder de pi, para que las dos vías muestren lo mismo.
	if evs[1].Delta != razonamientoOculto {
		t.Errorf("delta = %q, want %q", evs[1].Delta, razonamientoOculto)
	}
	evs = mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`)
	if len(evs) != 1 || evs[0].Type != EvThinkingEnd {
		t.Fatalf("esperaba thinking_end, got %v", tipos(evs))
	}
	if evs[0].ContentSignature != "EroBCkYIBxgCKkDopaco" {
		t.Errorf("el payload opaco no llegó en la firma: %q", evs[0].ContentSignature)
	}
}

// I19 — UN TOOL_USE INVÁLIDO NO SE LLEVA EL BLOQUE QUE ESTABA ABIERTO. La validación corre
// ANTES de tocar el mapa. Si se registrara primero, el bloque inválido reemplazaría al
// legítimo del mismo índice, el `content_block_stop` siguiente cerraría el inválido —que no
// emite nada— y el bloque de texto se quedaría abierto para siempre del lado del cliente.
func TestUnToolUseInvalidoNoSeLlevaElBloqueAbiertoDelMismoIndice(t *testing.T) {
	tr := NuevoTraductor()
	mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text"}}}`)

	// Un tool_use sin id en el MISMO índice: tiene que dar error y no dejar rastro.
	if _, err := tr.Linea([]byte(`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","name":"Read"}}}`)); err == nil {
		t.Fatal("un tool_use sin id tiene que dar error")
	}
	// El bloque de texto sigue vivo: su cierre emite el text_end que el cliente espera.
	evs := mustLinea(t, tr, `{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`)
	if len(evs) != 1 || evs[0].Type != EvTextEnd {
		t.Fatalf("el text_end se perdió: got %v", tipos(evs))
	}
}

// I20 — EL RESULT ANÓMALO SALE COMO ERROR, CON SU CONSUMO. A este camino se llega sólo
// cuando el proceso murió sin cerrar un turno, y es el ÚNICO lugar donde hay un usage para
// reportar: un intento que falló a mitad ya gastó los tokens de entrada, y declararlos en
// cero le hace creer al cliente que el fallo salió gratis.
func TestElResultAnomaloSaleComoErrorConSuUsage(t *testing.T) {
	tr := NuevoTraductor()
	evs := mustLinea(t, tr, `{"type":"result","subtype":"error_during_execution","is_error":true,"result":"se acabó la cuota","usage":{"input_tokens":77,"cache_read_input_tokens":300}}`)

	if len(evs) != 1 || evs[0].Type != EvError || evs[0].Reason != ReasonError {
		t.Fatalf("esperaba un error, got %v", tipos(evs))
	}
	if !tr.Cerrado() {
		t.Error("el result cierra el turno")
	}
	for _, quiero := range []string{"error_during_execution", "se acabó la cuota"} {
		if !strings.Contains(evs[0].ErrorMessage, quiero) {
			t.Errorf("el mensaje no menciona %q: %q", quiero, evs[0].ErrorMessage)
		}
	}
	quiero := Usage{Input: 77, CacheRead: 300, TotalTokens: 377}
	if got := *evs[0].Usage; got != quiero {
		t.Errorf("usage = %+v, want %+v", got, quiero)
	}

	// Y el caso que se lee como un crash sin serlo: el harness dice que salió BIEN y aun
	// así no cerró un turno. El mensaje tiene que decirlo, o manda a buscar la falla al
	// lado equivocado.
	tr2 := NuevoTraductor()
	evs = mustLinea(t, tr2, `{"type":"result","subtype":"success","is_error":false}`)
	if len(evs) != 1 || evs[0].Type != EvError {
		t.Fatalf("esperaba un error, got %v", tipos(evs))
	}
	if !strings.Contains(evs[0].ErrorMessage, "no reportó error") {
		t.Errorf("un result exitoso sin turno no se distingue de un crash: %q", evs[0].ErrorMessage)
	}
}

// La puerta de la Messages API es la MISMA traducción, sin el sobre del CLI. Es lo que hace
// que cambiar de motor no cueste el traductor, así que conviene que un test lo pinee: el
// día que `Linea` y `EventoAPI` se separen, lo que se rompe es la vía 2 entera.
func TestLaPuertaDeLaAPIYLaDelCLIDanLoMismo(t *testing.T) {
	eventos := []string{
		`{"type":"message_start","message":{"usage":{"input_tokens":10,"cache_read_input_tokens":90}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"París"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
	}

	porAPI := NuevoTraductor()
	porCLI := NuevoTraductor()
	for _, ev := range eventos {
		a, err := porAPI.EventoAPI([]byte(ev))
		if err != nil {
			t.Fatalf("EventoAPI(%s): %v", ev, err)
		}
		c := mustLinea(t, porCLI, `{"type":"stream_event","event":`+ev+`}`)

		brutoA, errA := json.Marshal(a)
		brutoC, errC := json.Marshal(c)
		if errA != nil || errC != nil {
			t.Fatalf("serializar: %v / %v", errA, errC)
		}
		if string(brutoA) != string(brutoC) {
			t.Errorf("las dos puertas difieren en %s:\n  api: %s\n  cli: %s", ev, brutoA, brutoC)
		}
	}
	if !porAPI.Cerrado() || !porCLI.Cerrado() {
		t.Errorf("los dos tienen que cerrar: api=%v cli=%v", porAPI.Cerrado(), porCLI.Cerrado())
	}
}
