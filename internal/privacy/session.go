// Package privacy es el portero que se para entre la memoria de Musubi y cualquier LLM externo:
// tapa los secretos ANTES de que el texto cruce la red y los repone DESPUÉS, sobre la respuesta.
//
// La diferencia con internal/redact —del que depende— es una sola, y es toda la razón de existir de
// este paquete: redact es de UNA VÍA (deja `[REDACTED:tipo]`, que ya no se puede deshacer), y para
// hablarle a un LLM hace falta poder volver atrás. El modelo tiene que poder razonar sobre el texto
// y devolvernos algo útil, sin haber visto nunca el valor real.
//
//	entrada:   "usá la clave sk-ant-api03-REAL para autenticar"
//	al LLM:    "usá la clave [[MSB:ai-provider-key:1]] para autenticar"
//	respuesta: "poné [[MSB:ai-provider-key:1]] en el header Authorization"
//	al caller: "poné sk-ant-api03-REAL en el header Authorization"
//
// LA DETECCIÓN NO SE REIMPLEMENTA. Qué cosa es un secreto lo decide redact.Redact, que ya está
// auditado (reglas por forma + entropía de Shannon + catch-all de hex, con allowlist de
// placeholders). Acá sólo se usan sus hallazgos para hacer una sustitución que sepa deshacerse. Una
// sola fuente de verdad: si redact aprende un patrón nuevo, este paquete lo hereda sin tocar nada.
//
// Determinista, sin red, sin estado en disco y sin dependencias fuera de la stdlib.
package privacy

import (
	"strconv"
	"strings"

	"musubi/internal/redact"
)

// tokenPrefix y tokenSuffix delimitan el marcador. ASCII puro a propósito: los modelos reformatean
// o "corrigen" los caracteres raros, y un marcador que vuelve mutado es un secreto que no se repone.
const (
	tokenPrefix = "[[MSB:"
	tokenSuffix = "]]"
)

// Session es el mapeo reversible de UNA llamada al motor. Vive lo que dura esa llamada y muere con
// ella: no hay estado global ni mapeo que sobreviva entre llamadas.
//
// NO ES SEGURA PARA USO CONCURRENTE, y es deliberado: una sesión pertenece a una sola llamada, así
// que compartirla entre goroutines sería el bug, no la falta de mutex. El decorador crea una por
// invocación.
type Session struct {
	byValue map[string]string // secreto → marcador. Da R3: el mismo secreto siempre el mismo marcador.
	byToken map[string]string // marcador → secreto. Da R2: Restore SÓLO repone lo que esta sesión acuñó.
	seen    []string          // textos ya scrubbeados. Da R5: contra ellos se chequea la colisión.
	n       int               // contador de marcadores acuñados.
	finds   []redact.Finding  // hallazgos acumulados, para poder auditar qué se tapó.
}

// NewSession crea una sesión vacía.
func NewSession() *Session {
	return &Session{
		byValue: make(map[string]string),
		byToken: make(map[string]string),
	}
}

// Scrub devuelve text con cada secreto reemplazado por un marcador reversible, y registra el mapeo
// para que Restore pueda deshacerlo. Llamarlo varias veces sobre la misma sesión acumula el mapeo:
// así el system prompt y el user prompt comparten marcadores para un mismo secreto.
//
// Si no hay secretos, devuelve el texto tal cual y no acuña nada.
func (s *Session) Scrub(text string) string {
	if text == "" {
		return text
	}
	// El texto entra en `seen` SIEMPRE, aunque no tenga secretos: lo que importa para la colisión de
	// marcadores (R5) es todo lo que el modelo va a ver, tenga o no hallazgos.
	s.seen = append(s.seen, text)

	_, finds := redact.Redact(text)
	if len(finds) == 0 {
		return text
	}
	s.finds = append(s.finds, finds...)

	// De ATRÁS PARA ADELANTE. redact devuelve los hallazgos ordenados por Start y sin solaparse; si
	// se sustituyera de izquierda a derecha, el primer reemplazo correría todos los offsets
	// siguientes (el marcador rara vez mide lo mismo que el secreto) y el segundo secreto de una
	// misma línea se cortaría mal. En reversa, los offsets pendientes siguen siendo válidos.
	var b strings.Builder
	b.Grow(len(text))
	out := text
	for i := len(finds) - 1; i >= 0; i-- {
		f := finds[i]
		if f.Start < 0 || f.End > len(out) || f.Start >= f.End {
			continue // defensa: un offset fuera de rango se ignora, nunca entra en pánico.
		}
		tok := s.mint(out[f.Start:f.End], f.Type)
		b.Reset()
		b.WriteString(out[:f.Start])
		b.WriteString(tok)
		b.WriteString(out[f.End:])
		out = b.String()
	}
	return out
}

// Restore repone los secretos en text. SÓLO sustituye marcadores que ESTA sesión acuñó: cualquier
// otra cosa con forma de marcador —inventada por el modelo o traída en la entrada— se devuelve
// intacta. Es el invariante R2, y es lo que impide que un motor malicioso escriba un marcador para
// hacer aparecer un secreto que nunca vio.
func (s *Session) Restore(text string) string {
	if text == "" || len(s.byToken) == 0 {
		return text
	}
	// strings.Replacer hace una sola pasada y NO re-escanea lo que ya sustituyó, así que un secreto
	// cuyo valor contenga la forma de otro marcador no puede disparar una sustitución en cascada.
	pairs := make([]string, 0, len(s.byToken)*2)
	for tok, val := range s.byToken {
		pairs = append(pairs, tok, val)
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// Count es cuántos secretos DISTINTOS tapó la sesión. El decorador lo usa para decidir si el modo
// `refuse` tiene que cortar la llamada.
func (s *Session) Count() int { return len(s.byToken) }

// Findings son los hallazgos acumulados, para logs y auditoría. Devuelve una copia: nadie de afuera
// puede alterar el estado de la sesión.
//
// OJO CON LOS OFFSETS: Start/End son relativos al texto de SU llamada a Scrub. Si se scrubbearon
// varios textos en la misma sesión, los offsets de distintas llamadas conviven en la misma lista y
// NO son comparables entre sí. Para contar o clasificar usá Count() y Types(); los offsets sólo
// tienen sentido cuando se scrubbeó un único texto.
func (s *Session) Findings() []redact.Finding {
	if len(s.finds) == 0 {
		return nil
	}
	out := make([]redact.Finding, len(s.finds))
	copy(out, s.finds)
	return out
}

// Types son los tipos de secreto tapados, sin repetir y en orden de aparición. Sirve para poder
// loguear QUÉ clase de secreto se tapó sin loguear jamás el valor.
func (s *Session) Types() []string {
	var out []string
	seen := make(map[string]bool, len(s.finds))
	for _, f := range s.finds {
		if !seen[f.Type] {
			seen[f.Type] = true
			out = append(out, f.Type)
		}
	}
	return out
}

// mint devuelve el marcador de `value`, acuñando uno nuevo si es la primera vez que se lo ve.
//
// Dos propiedades, y las dos importan:
//   - R3: el mismo valor devuelve SIEMPRE el mismo marcador dentro de la sesión.
//   - R5: el marcador acuñado NO aparece en ninguno de los textos ya vistos. Si el índice candidato
//     produjera una colisión, se salta al siguiente. Termina siempre: los índices son infinitos y el
//     texto es finito.
func (s *Session) mint(value, typ string) string {
	if tok, ok := s.byValue[value]; ok {
		return tok
	}
	if typ == "" {
		typ = "secret"
	}
	for {
		s.n++
		tok := tokenPrefix + typ + ":" + strconv.Itoa(s.n) + tokenSuffix
		if s.collides(tok) {
			continue
		}
		s.byValue[value] = tok
		s.byToken[tok] = value
		return tok
	}
}

// collides indica si tok ya aparece en algún texto visto o si ya fue acuñado. Lo primero es R5 (no
// pisar algo que el usuario escribió); lo segundo es una defensa barata contra reusar un índice.
func (s *Session) collides(tok string) bool {
	if _, taken := s.byToken[tok]; taken {
		return true
	}
	for _, t := range s.seen {
		// Camino rápido: si el texto no contiene NI SIQUIERA el prefijo del marcador, no puede
		// contener el marcador entero. Es el caso normal (nadie escribe "[[MSB:" a mano), y evita
		// re-escanear el prompt completo una vez por cada secreto encontrado.
		if !strings.Contains(t, tokenPrefix) {
			continue
		}
		if strings.Contains(t, tok) {
			return true
		}
	}
	return false
}
