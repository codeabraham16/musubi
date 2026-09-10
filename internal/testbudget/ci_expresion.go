package testbudget

// UN `if:` QUE NUNCA ES CIERTO NO ES EL LITERAL `false`.
//
// La versión anterior comprobaba `if: false` y `if: ${{ false }}`, o sea DOS FORMAS de escribir
// «apagado». Pero el paso se apaga igual —y más discretamente— con un `if:` que simplemente no
// se cumple nunca en las corridas de este workflow:
//
//	if: github.event_name == 'schedule'      # y el workflow dispara en push/pull_request
//
// Nadie lee eso como un apagado. Enumerar formas de apagar no cierra nada: falta la próxima
// (`github.ref == 'refs/heads/inexistente'`, `!success()`, …). Lo que sí cierra es DAR VUELTA LA
// PREGUNTA: en vez de preguntar si el `if:` está apagado, se exige poder DEMOSTRAR que es cierto
// para TODOS los eventos que declara el `on:` del workflow. Se evalúa la expresión, evento por
// evento, con tres valores —cierto, falso y NO SÉ— y sólo el cierto en todos pasa.
//
// El «no sé» es la mitad que importa: una expresión que este evaluador no entiende NO puede ser
// verde. Es la misma regla que el resto del archivo — «no pude mirar» no sale por la puerta de
// «miré y está bien».

import (
	"fmt"
	"sort"
	"strings"
)

// tri es cierto / falso / no sé.
type tri int

const (
	triFalso tri = iota
	triCierto
	triNoSe
)

func (t tri) String() string {
	switch t {
	case triFalso:
		return "falso"
	case triCierto:
		return "cierto"
	default:
		return "no sé"
	}
}

// AnalizarSi dice si un `if:` está GARANTIZADO cierto en todos los eventos del workflow.
//
// eventos son los disparadores del `on:`. Si viene vacío (un workflow sintético, o un `on:` que
// no se pudo leer) las condiciones que dependen del evento dan «no sé», que es rojo: no se puede
// afirmar que un paso corre si no se sabe cuándo corre el workflow.
func AnalizarSi(expr string, eventos []string) (bool, string) {
	e := strings.TrimSpace(expr)
	if e == "" {
		return true, ""
	}
	e = desenvolverExpresionGH(e)

	evs := eventos
	if len(evs) == 0 {
		evs = []string{""} // evento desconocido
	}
	var falsos, dudosos []string
	for _, ev := range evs {
		p := &parserSi{src: []rune(e), evento: ev}
		v := p.orExpr()
		if !p.fin() {
			v = triNoSe
		}
		switch v {
		case triFalso:
			falsos = append(falsos, etiquetaEvento(ev))
		case triNoSe:
			dudosos = append(dudosos, etiquetaEvento(ev))
		}
	}
	sort.Strings(falsos)
	sort.Strings(dudosos)
	switch {
	case len(falsos) == len(evs):
		return false, fmt.Sprintf("`if: %s` es FALSO en todos los eventos que dispara este "+
			"workflow (%s): el paso está apagado", expr, listaEventos(eventos))
	case len(falsos) > 0:
		return false, fmt.Sprintf("`if: %s` es falso en %s, y este workflow dispara en %s: el paso "+
			"no corre en todas las corridas que tendría que cubrir", expr, strings.Join(falsos, ", "),
			listaEventos(eventos))
	case len(dudosos) > 0:
		return false, fmt.Sprintf("no pude derivar si `if: %s` es cierto en %s. Un `if:` que esta "+
			"guarda no entiende no puede pasar en verde: usá `success()`, `always()` o una "+
			"comparación de github.event_name, o enseñale la construcción a AnalizarSi",
			expr, strings.Join(dudosos, ", "))
	}
	return true, ""
}

func etiquetaEvento(ev string) string {
	if ev == "" {
		return "un evento que no pude leer del `on:`"
	}
	return ev
}

func listaEventos(evs []string) string {
	if len(evs) == 0 {
		return "eventos que no pude leer del `on:`"
	}
	return strings.Join(evs, ", ")
}

// desenvolverExpresionGH saca el `${{ … }}` de afuera cuando envuelve a TODA la expresión.
func desenvolverExpresionGH(e string) string {
	for {
		if !strings.HasPrefix(e, "${{") || !strings.HasSuffix(e, "}}") {
			return e
		}
		adentro := strings.TrimSpace(e[3 : len(e)-2])
		if strings.Contains(adentro, "}}") { // eran dos expresiones concatenadas
			return e
		}
		e = adentro
	}
}

// ---------------------------------------------------------------------------------------------
// El evaluador: `||`, `&&`, `!`, paréntesis y átomos.

type parserSi struct {
	src    []rune
	i      int
	evento string
}

func (p *parserSi) espacios() {
	for p.i < len(p.src) && (p.src[p.i] == ' ' || p.src[p.i] == '\t' || p.src[p.i] == '\n') {
		p.i++
	}
}

func (p *parserSi) fin() bool {
	p.espacios()
	return p.i >= len(p.src)
}

func (p *parserSi) mira(s string) bool {
	p.espacios()
	return strings.HasPrefix(string(p.src[p.i:]), s)
}

func (p *parserSi) come(s string) bool {
	if p.mira(s) {
		p.i += len([]rune(s))
		return true
	}
	return false
}

func (p *parserSi) orExpr() tri {
	v := p.andExpr()
	for p.come("||") {
		w := p.andExpr()
		v = oTri(v, w)
	}
	return v
}

func (p *parserSi) andExpr() tri {
	v := p.notExpr()
	for p.come("&&") {
		w := p.notExpr()
		v = yTri(v, w)
	}
	return v
}

func (p *parserSi) notExpr() tri {
	p.espacios()
	// `!` de negación, pero NO el `!=` de una comparación.
	if p.i < len(p.src) && p.src[p.i] == '!' && !(p.i+1 < len(p.src) && p.src[p.i+1] == '=') {
		p.i++
		return noTri(p.notExpr())
	}
	if p.come("(") {
		v := p.orExpr()
		if !p.come(")") {
			return triNoSe
		}
		return v
	}
	return p.atomo()
}

// atomo lee hasta el próximo operador de tope y clasifica lo leído.
func (p *parserSi) atomo() tri {
	p.espacios()
	inicio := p.i
	for p.i < len(p.src) {
		c := p.src[p.i]
		switch {
		case c == '\'' || c == '"':
			cierre := c
			p.i++
			for p.i < len(p.src) && p.src[p.i] != cierre {
				p.i++
			}
			if p.i < len(p.src) {
				p.i++
			}
			continue
		case c == '(':
			// Llamada a función (`success()`, `contains(a, b)`): los paréntesis son del átomo.
			prof := 0
			for p.i < len(p.src) {
				if p.src[p.i] == '(' {
					prof++
				} else if p.src[p.i] == ')' {
					prof--
					if prof == 0 {
						p.i++
						break
					}
				}
				p.i++
			}
			continue
		case c == ')':
			return clasificarAtomo(string(p.src[inicio:p.i]), p.evento)
		case c == '|' && p.i+1 < len(p.src) && p.src[p.i+1] == '|':
			return clasificarAtomo(string(p.src[inicio:p.i]), p.evento)
		case c == '&' && p.i+1 < len(p.src) && p.src[p.i+1] == '&':
			return clasificarAtomo(string(p.src[inicio:p.i]), p.evento)
		}
		p.i++
	}
	return clasificarAtomo(string(p.src[inicio:p.i]), p.evento)
}

// clasificarAtomo es la ÚNICA lista del evaluador, y es una lista de formas ENTENDIDAS —no de
// formas malas—: todo lo que no esté acá da «no sé», que es rojo.
func clasificarAtomo(a, evento string) tri {
	a = strings.TrimSpace(a)
	sinEspacios := strings.Join(strings.Fields(a), " ")
	switch strings.ToLower(sinEspacios) {
	case "":
		return triNoSe
	case "true", "success()":
		// success() es el default de un paso: cierto mientras los anteriores hayan ido bien, que
		// es exactamente la corrida en la que el guard tiene que hablar.
		return triCierto
	case "always()", "!cancelled()", "!failure()":
		return triCierto
	case "false", "cancelled()", "failure()":
		return triFalso
	}
	// github.event_name == 'x' / != 'x'
	for _, op := range []string{"==", "!="} {
		izq, der, ok := strings.Cut(sinEspacios, op)
		if !ok {
			continue
		}
		izq, der = strings.TrimSpace(izq), strings.TrimSpace(der)
		if izq != "github.event_name" {
			// También al revés: 'push' == github.event_name
			if der != "github.event_name" {
				return triNoSe
			}
			izq, der = der, izq
		}
		valor := strings.Trim(der, `'"`)
		if valor == der { // no era un literal de texto
			return triNoSe
		}
		if evento == "" {
			return triNoSe // no sé en qué eventos dispara este workflow
		}
		igual := evento == valor
		if op == "!=" {
			igual = !igual
		}
		if igual {
			return triCierto
		}
		return triFalso
	}
	return triNoSe
}

func yTri(a, b tri) tri {
	if a == triFalso || b == triFalso {
		return triFalso
	}
	if a == triNoSe || b == triNoSe {
		return triNoSe
	}
	return triCierto
}

func oTri(a, b tri) tri {
	if a == triCierto || b == triCierto {
		return triCierto
	}
	if a == triNoSe || b == triNoSe {
		return triNoSe
	}
	return triFalso
}

func noTri(a tri) tri {
	switch a {
	case triCierto:
		return triFalso
	case triFalso:
		return triCierto
	}
	return triNoSe
}
