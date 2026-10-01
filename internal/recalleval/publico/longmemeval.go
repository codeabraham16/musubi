// Package publico pone a Musubi al lado de VARAS PÚBLICAS de recuperación: datasets que publicó
// otra gente, con sus propias métricas, para comparar contra números que no fabricamos nosotros.
//
// POR QUÉ UN ADAPTADOR PROPIO Y NO EL CÓDIGO DEL PAPER. El banco de recalleval mide a Musubi contra
// fixtures nuestros, y un número propio contra un corpus propio no dice dónde está Musubi en el
// mundo. Pero correr el código de terceros para conseguir ese número es otra cosa: dependencias,
// red y un proceso ajeno cerca de la memoria. Acá se LEEN sus datos y se REPLICAN sus reglas —la del
// oro, las exclusiones y las métricas— en Go, citando la línea de la que sale cada una, para que un
// desacuerdo con el paper se pueda rastrear a una regla y no a una caja negra.
//
// LongMemEval (Wu et al., ICLR 2025, arXiv 2410.10813): 500 preguntas sobre historiales de chat.
// Cada pregunta trae su propio «pajar» de sesiones (unas 50 en la variante S) y dice cuáles tienen
// la evidencia. Acá se mide SÓLO la recuperación: qué sesiones devuelve el recall para la pregunta,
// no si un modelo contesta bien.
package publico

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Turno es un turno de una sesión del pajar.
type Turno struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// HasAnswer marca el turno que tiene la evidencia. Es *bool porque el dataset no lo trae en
	// todos los turnos, y «no lo trae» no es lo mismo que «trae false»: el paper los distingue al
	// excluir preguntas (`('has_answer' in turn) and turn['has_answer']`, run_retrieval.py:400).
	HasAnswer *bool `json:"has_answer,omitempty"`
}

// Pregunta es una instancia de LongMemEval.
//
// NO LLEVA EL CAMPO `answer`, A PROPÓSITO. Esto mide recuperación, y la respuesta no le sirve a
// ninguna métrica de recuperación; tenerla a mano sólo abre la puerta a que se cuele en una
// consulta o en un doc. Además su tipo varía en el dataset (texto en casi todas, número en
// algunas), y un decodificador estricto se caería por un campo que no se usa.
type Pregunta struct {
	QuestionID         string    `json:"question_id"`
	QuestionType       string    `json:"question_type"`
	Question           string    `json:"question"`
	QuestionDate       string    `json:"question_date"`
	HaystackDates      []string  `json:"haystack_dates"`
	HaystackSessionIDs []string  `json:"haystack_session_ids"`
	HaystackSessions   [][]Turno `json:"haystack_sessions"`
	AnswerSessionIDs   []string  `json:"answer_session_ids"`
}

// LeerLongMemEval recorre el arreglo JSON del dataset DE A UNA PREGUNTA, llamando a cb con cada una
// apenas se decodifica.
//
// POR QUÉ EN STREAMING. longmemeval_s pesa 278 MB y la variante M 2,7 GB; un ReadAll + Unmarshal
// tiene en memoria el archivo entero y, encima, el árbol decodificado — varias veces el tamaño del
// archivo, en una máquina compartida de 7 GB. Con json.Decoder vive sólo la pregunta en curso.
//
// Un error de cb corta el recorrido y se devuelve tal cual; un error de decodificación dice en qué
// pregunta pasó.
func LeerLongMemEval(r io.Reader, cb func(Pregunta) error) error {
	dec := json.NewDecoder(r)
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("longmemeval: leyendo el comienzo del arreglo: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return fmt.Errorf("longmemeval: se esperaba un arreglo JSON y el archivo empieza con %v", tok)
	}
	n := 0
	for dec.More() {
		var p Pregunta
		if err := dec.Decode(&p); err != nil {
			return fmt.Errorf("longmemeval: pregunta #%d: %w", n+1, err)
		}
		if err := cb(p); err != nil {
			return err
		}
		n++
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("longmemeval: leyendo el cierre del arreglo tras %d preguntas: %w", n, err)
	}
	return nil
}

// EtiquetaDeSesion es el id de corpus que el paper le da a una sesión a granularidad de sesión
// (process_item_flat_index, run_retrieval.py:209): el id de la sesión, salvo que contenga "answer"
// y NINGÚN turno de usuario tenga has_answer=true — entonces se renombra "answer"→"noans" y deja de
// ser oro. Es lo que pasa con las sesiones cuya evidencia la dijo el asistente: el paper indexa
// sólo lo que dijo el usuario, así que ahí no hay nada que encontrar.
func EtiquetaDeSesion(sid string, turnos []Turno) string {
	if strings.Contains(sid, "answer") && !usuarioConEvidencia(turnos) {
		// str.replace de Python reemplaza TODAS las apariciones.
		return strings.ReplaceAll(sid, "answer", "noans")
	}
	return sid
}

// EsOro dice si una etiqueta de corpus es de una sesión relevante: la regla del paper es que el id
// contenga "answer" (`correct_docs = [... if "answer" in doc_id]`, run_retrieval.py:272).
func EsOro(etiqueta string) bool { return strings.Contains(etiqueta, "answer") }

// usuarioConEvidencia dice si algún turno de USUARIO de la sesión tiene has_answer=true.
func usuarioConEvidencia(turnos []Turno) bool {
	for _, t := range turnos {
		if t.Role == "user" && t.HasAnswer != nil && *t.HasAnswer {
			return true
		}
	}
	return false
}

// Exclusion dice por qué el paper deja una pregunta fuera del promedio (run_retrieval.py:396-402),
// o "" si entra. Son dos motivos, en este orden:
//   - "abstencion": el question_id contiene "_abs". Son preguntas cuya respuesta correcta es «no
//     lo sé»: no hay sesión que encontrar.
//   - "sin_oro_de_usuario": ningún turno de usuario de NINGUNA sesión del pajar tiene la
//     evidencia. Es sobre todo el tipo single-session-assistant.
func Exclusion(p Pregunta) string {
	if strings.Contains(p.QuestionID, "_abs") {
		return "abstencion"
	}
	for _, s := range p.HaystackSessions {
		if usuarioConEvidencia(s) {
			return ""
		}
	}
	return "sin_oro_de_usuario"
}

// Modo es qué texto de cada sesión va al corpus.
type Modo string

const (
	// ModoUsuario es el del paper: los turnos de USUARIO unidos con un espacio
	// (`' '.join([... if interact['role'] == 'user'])`, run_retrieval.py:206). Es el que hace
	// comparables los números con los suyos.
	ModoUsuario Modo = "user"
	// ModoCompleto suma los turnos del asistente. No es comparable con el paper; existe para medir
	// cuánto cambia el ranking de Musubi cuando la memoria guarda la sesión entera.
	ModoCompleto Modo = "full"
)

// TextoDeSesion arma el texto de una sesión según el modo.
func TextoDeSesion(turnos []Turno, modo Modo) string {
	partes := make([]string, 0, len(turnos))
	for _, t := range turnos {
		if modo == ModoUsuario && t.Role != "user" {
			continue
		}
		partes = append(partes, t.Content)
	}
	return strings.Join(partes, " ")
}

// Corpus es el pajar de UNA pregunta, alineado por posición: el doc i tiene el id IDs[i] en Musubi,
// la etiqueta del paper Etiquetas[i] y el texto Textos[i].
type Corpus struct {
	IDs       []string
	Etiquetas []string
	Textos    []string
	// Desparejas cuenta si las tres listas del pajar (ids, sesiones y fechas) venían de largos
	// distintos. El paper las recorre con zip(), que corta en la más corta sin avisar; acá se corta
	// igual —para medir lo mismo— pero se cuenta, para que no pase callado.
	Desparejas bool
}

// TopicoLongMemEval es el topic_key con el que se siembra TODO doc de LongMemEval. Es uno solo a
// propósito: un topic por sesión le daría al recall un texto indexable derivado del id.
const TopicoLongMemEval = "varas/longmemeval"

// CorpusDe arma el corpus de una pregunta.
//
// EL ID DEL DOC ES OPACO, Y ESO ES LO QUE EVITA LA FUGA. Los ids de sesión del dataset llevan la
// etiqueta adentro («answer_…» es oro), y un id que el recall pudiera ver le regalaría la respuesta a
// cualquier señal que mire el id. Por eso el doc se guarda con un hash de (pregunta, posición) y la
// etiqueta queda del lado del medidor, en Etiquetas. La posición —y no el id de sesión— entra al
// hash porque un pajar puede repetir una sesión, y el paper cuenta cada aparición como un doc.
func CorpusDe(p Pregunta, modo Modo) Corpus {
	n := len(p.HaystackSessionIDs)
	if len(p.HaystackSessions) < n {
		n = len(p.HaystackSessions)
	}
	if len(p.HaystackDates) < n {
		n = len(p.HaystackDates)
	}
	c := Corpus{
		IDs:       make([]string, n),
		Etiquetas: make([]string, n),
		Textos:    make([]string, n),
		Desparejas: len(p.HaystackSessionIDs) != len(p.HaystackSessions) ||
			len(p.HaystackSessionIDs) != len(p.HaystackDates),
	}
	for i := 0; i < n; i++ {
		c.IDs[i] = IDOpaco(p.QuestionID, i)
		c.Etiquetas[i] = EtiquetaDeSesion(p.HaystackSessionIDs[i], p.HaystackSessions[i])
		c.Textos[i] = TextoDeSesion(p.HaystackSessions[i], modo)
	}
	return c
}

// IDOpaco es el id en Musubi del doc en la posición pos del pajar de la pregunta qid.
func IDOpaco(qid string, pos int) string {
	h := sha256.Sum256([]byte(qid + "\x00" + strconv.Itoa(pos)))
	return "lme-" + hex.EncodeToString(h[:8])
}

// Posiciones traduce un ranking de ids de Musubi a posiciones del corpus. Un id que no es del
// corpus es un error, no un dato: querría decir que la base no estaba vacía, y entonces el número
// mediría otra cosa.
func (c Corpus) Posiciones(ids []string) ([]int, error) {
	pos := make(map[string]int, len(c.IDs))
	for i, id := range c.IDs {
		pos[id] = i
	}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		i, ok := pos[id]
		if !ok {
			return nil, fmt.Errorf("el recall devolvió %q, que no es de este corpus: la base no estaba vacía", id)
		}
		out = append(out, i)
	}
	return out, nil
}
