package publico

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// lectorQueSeCorta entrega el texto y después falla: modela un archivo que el lector NO tiene
// entero en la mano.
type lectorQueSeCorta struct{ r io.Reader }

var errCorte = errors.New("se cortó el archivo")

func (l *lectorQueSeCorta) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	if err == io.EOF {
		return n, errCorte
	}
	return n, err
}

// El lector entrega cada pregunta ANTES de terminar de leer el archivo. Es la única propiedad que
// hace posible pasar longmemeval_s (278 MB) —y la M, 2,7 GB— sin tenerlos enteros en memoria: una
// lectura que primero junta todo y después decodifica no le entrega NADA a un archivo que se corta
// después de la primera pregunta.
//
// Sabotaje: el lector junta el archivo entero antes de decodificar.
// arnes: archivo="internal/recalleval/publico/longmemeval.go"
// arnes: de="\tdec := json.NewDecoder(r)\n"
// arnes: a="\tif _, err := io.ReadAll(r); err != nil {\n\t\treturn err\n\t}\n\tdec := json.NewDecoder(r)\n"
func TestLeerLongMemEvalEntregaAntesDeTerminarDeLeer(t *testing.T) {
	primera := `[{"question_id":"q1","question_type":"single-session-user","question":"¿Qué estudié?",` +
		`"answer":42,"question_date":"2023/05/30","haystack_dates":["d"],"haystack_session_ids":["answer_1"],` +
		`"haystack_sessions":[[{"role":"user","content":"hola","has_answer":true}]],"answer_session_ids":["answer_1"]},`
	var vistas []string
	err := LeerLongMemEval(&lectorQueSeCorta{r: strings.NewReader(primera)}, func(p Pregunta) error {
		vistas = append(vistas, p.QuestionID)
		return nil
	})
	if len(vistas) != 1 || vistas[0] != "q1" {
		t.Fatalf("el lector tenía la primera pregunta completa y no la entregó antes del corte: vistas=%v err=%v", vistas, err)
	}
	if !errors.Is(err, errCorte) {
		t.Fatalf("un archivo cortado tiene que terminar en error, y terminó en %v", err)
	}
}

// El archivo entero, con la respuesta de tipo NÚMERO que trae el dataset en algunas preguntas: el
// lector no la decodifica (Pregunta no tiene `answer`), así que no puede caerse por ella.
func TestLeerLongMemEvalRecorreElArregloEntero(t *testing.T) {
	js := `[{"question_id":"a","answer":3},{"question_id":"b","answer":"texto"},{"question_id":"c"}]`
	var ids []string
	if err := LeerLongMemEval(strings.NewReader(js), func(p Pregunta) error {
		ids = append(ids, p.QuestionID)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "a,b,c" {
		t.Fatalf("ids = %v", ids)
	}
	if err := LeerLongMemEval(strings.NewReader(`{"question_id":"a"}`), func(Pregunta) error { return nil }); err == nil {
		t.Fatal("un objeto suelto no es el formato del dataset y pasó como si lo fuera")
	}
}

func si() *bool { v := true; return &v }
func no() *bool { v := false; return &v }

// La regla del oro del paper (run_retrieval.py:209): una sesión «answer_…» es oro sólo si algún
// turno de USUARIO tiene la evidencia. La evidencia que dijo el asistente no cuenta, porque el
// paper indexa sólo lo que dijo el usuario.
//
// Sabotaje: el oro deja de exigir que la evidencia sea del usuario.
// arnes: archivo="internal/recalleval/publico/longmemeval.go"
// arnes: de="if t.Role == \"user\" && t.HasAnswer != nil && *t.HasAnswer {"
// arnes: a="if t.HasAnswer != nil && *t.HasAnswer {"
func TestEtiquetaDeSesionSigueLaReglaDelPaper(t *testing.T) {
	casos := []struct {
		nombre string
		sid    string
		turnos []Turno
		quiere string
	}{
		{"evidencia del usuario", "answer_4be1b6b4_2", []Turno{{Role: "user", HasAnswer: si()}, {Role: "assistant", HasAnswer: no()}}, "answer_4be1b6b4_2"},
		{"evidencia sólo del asistente", "answer_4be1b6b4_2", []Turno{{Role: "user", HasAnswer: no()}, {Role: "assistant", HasAnswer: si()}}, "noans_4be1b6b4_2"},
		{"sin marca en ningún turno", "answer_x", []Turno{{Role: "user"}}, "noans_x"},
		{"sesión que no es de respuesta", "sharegpt_x_1", []Turno{{Role: "user", HasAnswer: si()}}, "sharegpt_x_1"},
		{"el reemplazo es de TODAS las apariciones", "answer_answer", []Turno{{Role: "user", HasAnswer: no()}}, "noans_noans"},
	}
	for _, c := range casos {
		if got := EtiquetaDeSesion(c.sid, c.turnos); got != c.quiere {
			t.Errorf("%s: EtiquetaDeSesion(%q) = %q, quiero %q", c.nombre, c.sid, got, c.quiere)
		}
	}
	if !EsOro("answer_4be1b6b4_2") || EsOro("noans_4be1b6b4_2") || EsOro("sharegpt_x_1") {
		t.Error("EsOro no sigue la regla del paper: oro es la etiqueta que contiene \"answer\"")
	}
}

// Las dos exclusiones del promedio del paper (run_retrieval.py:396-402), en su orden.
//
// Sabotaje: la exclusión deja de mirar las abstenciones.
// arnes: archivo="internal/recalleval/publico/longmemeval.go"
// arnes: de="if strings.Contains(p.QuestionID, \"_abs\") {"
// arnes: a="if strings.Contains(p.QuestionID, \"_abstencion_que_no_existe\") {"
func TestExclusionSigueAlPaper(t *testing.T) {
	conOro := [][]Turno{{{Role: "assistant", HasAnswer: si()}}, {{Role: "user", HasAnswer: si()}}}
	sinOroDeUsuario := [][]Turno{{{Role: "assistant", HasAnswer: si()}, {Role: "user", HasAnswer: no()}}}
	casos := []struct {
		p      Pregunta
		quiere string
	}{
		{Pregunta{QuestionID: "e47becba", HaystackSessions: conOro}, ""},
		{Pregunta{QuestionID: "e47becba_abs", HaystackSessions: conOro}, "abstencion"},
		{Pregunta{QuestionID: "7161e7e2", HaystackSessions: sinOroDeUsuario}, "sin_oro_de_usuario"},
	}
	for _, c := range casos {
		if got := Exclusion(c.p); got != c.quiere {
			t.Errorf("Exclusion(%s) = %q, quiero %q", c.p.QuestionID, got, c.quiere)
		}
	}
}

// EL RECALL NO PUEDE VER LA ETIQUETA. Los ids de sesión del dataset dicen «answer» en las de oro;
// si el id o el topic del doc los arrastraran, cualquier señal que los mire sacaría el oro gratis.
// Y un pajar que repite una sesión tiene que dar dos docs, como en el paper, no uno pisado.
//
// Sabotaje: el doc se guarda con el id de sesión del dataset.
// arnes: archivo="internal/recalleval/publico/longmemeval.go"
// arnes: de="\t\tc.IDs[i] = IDOpaco(p.QuestionID, i)\n"
// arnes: a="\t\tc.IDs[i] = p.HaystackSessionIDs[i]\n"
func TestCorpusDeNoFiltraLaEtiquetaAlRecall(t *testing.T) {
	p := Pregunta{
		QuestionID:         "e47becba",
		HaystackSessionIDs: []string{"answer_280352e9", "sharegpt_yywfCRo_0", "answer_280352e9"},
		HaystackDates:      []string{"d1", "d2", "d3"},
		HaystackSessions: [][]Turno{
			{{Role: "user", Content: "me recibí de administración", HasAnswer: si()}, {Role: "assistant", Content: "felicitaciones"}},
			{{Role: "user", Content: "receta de pan"}, {Role: "assistant", Content: "harina y agua"}},
			{{Role: "user", Content: "me recibí de administración", HasAnswer: si()}},
		},
	}
	c := CorpusDe(p, ModoUsuario)
	if len(c.IDs) != 3 || c.Desparejas {
		t.Fatalf("corpus de %d docs (desparejas=%v), quiero 3 alineados", len(c.IDs), c.Desparejas)
	}
	vistos := map[string]bool{}
	for i, id := range c.IDs {
		if strings.Contains(id, "answer") || strings.Contains(id, p.HaystackSessionIDs[i]) {
			t.Errorf("el id del doc %d (%q) deja ver la etiqueta de la sesión %q", i, id, p.HaystackSessionIDs[i])
		}
		if vistos[id] {
			t.Errorf("dos posiciones del pajar comparten el id %q: la segunda pisaría a la primera", id)
		}
		vistos[id] = true
	}
	if strings.Contains(TopicoLongMemEval, "answer") {
		t.Errorf("el topic %q deja ver la etiqueta", TopicoLongMemEval)
	}
	if got := strings.Join(c.Etiquetas, ","); got != "answer_280352e9,sharegpt_yywfCRo_0,answer_280352e9" {
		t.Errorf("etiquetas = %s", got)
	}
	if c.Textos[0] != "me recibí de administración" {
		t.Errorf("en el modo del paper el texto es sólo lo del usuario, y quedó %q", c.Textos[0])
	}
	if got := CorpusDe(p, ModoCompleto).Textos[0]; got != "me recibí de administración felicitaciones" {
		t.Errorf("el modo completo une todos los turnos con un espacio, y quedó %q", got)
	}
}

// El paper recorre el pajar con zip(), que corta en la lista más corta sin avisar. Acá se corta
// igual, pero queda contado.
func TestCorpusDeCuentaLasListasDesparejas(t *testing.T) {
	p := Pregunta{
		QuestionID:         "q",
		HaystackSessionIDs: []string{"a", "b"},
		HaystackDates:      []string{"d1"},
		HaystackSessions:   [][]Turno{{{Role: "user", Content: "x"}}, {{Role: "user", Content: "y"}}},
	}
	c := CorpusDe(p, ModoUsuario)
	if len(c.IDs) != 1 || !c.Desparejas {
		t.Fatalf("con fechas de menos: %d docs, desparejas=%v; quiero 1 y true", len(c.IDs), c.Desparejas)
	}
}

// Un id que no es del corpus es un error: querría decir que la base no nació vacía.
func TestPosicionesRechazaUnIDAjeno(t *testing.T) {
	c := CorpusDe(Pregunta{QuestionID: "q", HaystackSessionIDs: []string{"a", "b"}, HaystackDates: []string{"1", "2"},
		HaystackSessions: [][]Turno{{{Role: "user"}}, {{Role: "user"}}}}, ModoUsuario)
	pos, err := c.Posiciones([]string{c.IDs[1], c.IDs[0]})
	if err != nil || len(pos) != 2 || pos[0] != 1 || pos[1] != 0 {
		t.Fatalf("pos=%v err=%v", pos, err)
	}
	if _, err := c.Posiciones([]string{c.IDs[0], "obs-de-otra-base"}); err == nil {
		t.Fatal("un id ajeno al corpus pasó como si fuera del corpus")
	}
}
