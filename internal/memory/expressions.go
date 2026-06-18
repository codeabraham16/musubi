package memory

import (
	"fmt"
	"strings"
)

// expressions.go implementa un evaluador de expresiones MODEL-FREE y SEGURO para
// las condiciones `when` de los steps de workflow. NO es eval arbitrario: solo
// soporta una gramática booleana acotada sobre el contexto del run (estados y
// resultados de steps). Inspirado en expressions.py de spec-kit, en Go puro.
//
// Gramática:
//   expr   := or
//   or     := and ( "or" and )*
//   and    := not ( "and" not )*
//   not    := "not" not | cmp
//   cmp    := atom ( ("=="|"!="|"contains") atom )?
//   atom   := "(" expr ")" | string | path
//
// `not` envuelve la comparación (precedencia menor que ==), así `not a == b` es
// `not (a == b)`.
//
// Una path como `step.build.status` se resuelve del contexto; una palabra suelta
// (ej. `done`) o un string entre comillas es literal. Una comparación da bool;
// un valor suelto es "truthy" si no está vacío.

// EvalCondition evalúa expr contra ctx (claves tipo "step.<id>.status" /
// "step.<id>.result"). Una expresión vacía es true (sin condición = corre).
func EvalCondition(expr string, ctx map[string]string) (bool, error) {
	if strings.TrimSpace(expr) == "" {
		return true, nil
	}
	toks, err := tokenizeExpr(expr)
	if err != nil {
		return false, err
	}
	p := &exprParser{toks: toks, ctx: ctx}
	v, err := p.parseOr()
	if err != nil {
		return false, err
	}
	if p.pos != len(p.toks) {
		return false, fmt.Errorf("tokens sobrantes en la expresión: %q", expr)
	}
	return v.truthy(), nil
}

// exprValue es el resultado de un nodo: string o bool.
type exprValue struct {
	s      string
	b      bool
	isBool bool
}

func (v exprValue) truthy() bool {
	if v.isBool {
		return v.b
	}
	return strings.TrimSpace(v.s) != ""
}

type exprParser struct {
	toks []exprTok
	pos  int
	ctx  map[string]string
}

func (p *exprParser) peek() (exprTok, bool) {
	if p.pos < len(p.toks) {
		return p.toks[p.pos], true
	}
	return exprTok{}, false
}

func (p *exprParser) parseOr() (exprValue, error) {
	left, err := p.parseAnd()
	if err != nil {
		return left, err
	}
	for {
		t, ok := p.peek()
		if !ok || t.kind != tkOp || t.val != "or" {
			break
		}
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return left, err
		}
		left = exprValue{b: left.truthy() || right.truthy(), isBool: true}
	}
	return left, nil
}

func (p *exprParser) parseAnd() (exprValue, error) {
	left, err := p.parseNot()
	if err != nil {
		return left, err
	}
	for {
		t, ok := p.peek()
		if !ok || t.kind != tkOp || t.val != "and" {
			break
		}
		p.pos++
		right, err := p.parseNot()
		if err != nil {
			return left, err
		}
		left = exprValue{b: left.truthy() && right.truthy(), isBool: true}
	}
	return left, nil
}

func (p *exprParser) parseNot() (exprValue, error) {
	t, ok := p.peek()
	if ok && t.kind == tkOp && t.val == "not" {
		p.pos++
		v, err := p.parseNot()
		if err != nil {
			return v, err
		}
		return exprValue{b: !v.truthy(), isBool: true}, nil
	}
	return p.parseCmp()
}

func (p *exprParser) parseCmp() (exprValue, error) {
	left, err := p.parseAtom()
	if err != nil {
		return left, err
	}
	t, ok := p.peek()
	if ok && t.kind == tkOp && (t.val == "==" || t.val == "!=" || t.val == "contains") {
		p.pos++
		right, err := p.parseAtom()
		if err != nil {
			return left, err
		}
		switch t.val {
		case "==":
			return exprValue{b: left.s == right.s, isBool: true}, nil
		case "!=":
			return exprValue{b: left.s != right.s, isBool: true}, nil
		case "contains":
			return exprValue{b: strings.Contains(left.s, right.s), isBool: true}, nil
		}
	}
	return left, nil
}

func (p *exprParser) parseAtom() (exprValue, error) {
	t, ok := p.peek()
	if !ok {
		return exprValue{}, fmt.Errorf("expresión incompleta")
	}
	switch t.kind {
	case tkLParen:
		p.pos++
		v, err := p.parseOr()
		if err != nil {
			return v, err
		}
		end, ok := p.peek()
		if !ok || end.kind != tkRParen {
			return v, fmt.Errorf("falta ')' en la expresión")
		}
		p.pos++
		return v, nil
	case tkString:
		p.pos++
		return exprValue{s: t.val}, nil
	case tkIdent:
		p.pos++
		// Resolver como path del contexto; si no existe, es literal (palabra suelta).
		if val, found := p.ctx[t.val]; found {
			return exprValue{s: val}, nil
		}
		return exprValue{s: t.val}, nil
	default:
		return exprValue{}, fmt.Errorf("token inesperado %q", t.val)
	}
}

// --- tokenizer ---

type tokKind int

const (
	tkIdent tokKind = iota
	tkString
	tkOp
	tkLParen
	tkRParen
)

type exprTok struct {
	kind tokKind
	val  string
}

func tokenizeExpr(s string) ([]exprTok, error) {
	var toks []exprTok
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
			i++
		case c == '(':
			toks = append(toks, exprTok{tkLParen, "("})
			i++
		case c == ')':
			toks = append(toks, exprTok{tkRParen, ")"})
			i++
		case c == '"' || c == '\'':
			quote := c
			i++
			start := i
			for i < len(s) && s[i] != quote {
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("string sin cerrar en la expresión")
			}
			toks = append(toks, exprTok{tkString, s[start:i]})
			i++ // saltar comilla de cierre
		case c == '=' && i+1 < len(s) && s[i+1] == '=':
			toks = append(toks, exprTok{tkOp, "=="})
			i += 2
		case c == '!' && i+1 < len(s) && s[i+1] == '=':
			toks = append(toks, exprTok{tkOp, "!="})
			i += 2
		case isIdentChar(c):
			start := i
			for i < len(s) && isIdentChar(s[i]) {
				i++
			}
			word := s[start:i]
			if word == "and" || word == "or" || word == "not" || word == "contains" {
				toks = append(toks, exprTok{tkOp, word})
			} else {
				toks = append(toks, exprTok{tkIdent, word})
			}
		default:
			return nil, fmt.Errorf("carácter inesperado %q en la expresión", string(c))
		}
	}
	return toks, nil
}

func isIdentChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '_' || c == '.' || c == '-' || c == '/'
}
