package mcp

// alertas_umbral_alcanzable_test.go — que el umbral y el plazo de una alerta sean ALCANZABLES.
//
// ═════════════════════════════════════════════════════════════════════════════════════════════
// LOS TRES CABOS, MEDIDOS: LA GUARDA MIRABA LA FORMA Y LA FORMA NO DECIDE NADA
//
// `alertas_cobertura_sla_test.go` cerró el cabo de que NADIE leyera la cobertura de servicios.
// Después un saboteador reprodujo el mismo defecto —una alerta que no puede disparar nunca— de
// tres maneras que esa guarda no ve, y las tres quedaron en VERDE:
//
//	(1) UMBRAL INALCANZABLE.  `delta(musubi:project_service_up:cobertura30d[6h]) < -5`
//	    La cobertura es un ratio en [0,1] ⇒ su `delta` vive en [-1,1] y `< -5` no dispara jamás.
//	    Es la confusión MÁS PLAUSIBLE que hay: el `summary` dice «cayó más de 5 puntos», así que
//	    quien pase de ratio a porcentaje escribe exactamente eso, sin mala fe.
//	(2) PLAZO INALCANZABLE.  `for: 30d` en vez de `for: 30m`.
//	    La condición tendría que sostenerse treinta días sobre una ventana de seis horas. La
//	    guarda vieja NO LEÍA el campo `for` en ninguna de las dos alertas, aunque su propio
//	    mensaje de error le ordenaba al autor calcar `for: 30m`.
//	(3) OTRA FUNCIÓN, MISMO REGEX.  `deriv(...)` en vez de `delta(...)`, umbral intacto.
//	    `deriv` es POR SEGUNDO: -0,05/s son -180 por hora sobre un ratio [0,1]. Inalcanzable. Y lo
//	    bendecía el propio regex de la guarda, que aceptaba la alternación `delta|idelta|deriv`:
//	    la lista de formas buenas era, ella misma, el agujero.
//
// Y ESTÁN EN N DE N CAMINOS: los tres se aplican igual a `CoberturaDelSlaSeCayo`, la alerta de
// MÁQUINAS que ya estaba en producción, no sólo a la hermana nueva de servicios.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// CÓMO SE CIERRA: DEJANDO DE MIRAR LA FORMA
//
// Agregar `deriv` a una lista negra no cierra nada — la próxima función tampoco va a estar en la
// lista. Así que esta guarda no pregunta CÓMO está escrita la expresión: la PARSEA y calcula el
// RANGO DE VALORES que puede tomar, con aritmética de intervalos sobre el árbol:
//
//	musubi_fleet_device_up                       [0,1]      declarado en deploy/rangos-de-series.yml
//	musubi:device_up:norm      = max by(...)     [0,1]      un max no sale del rango de su entrada
//	musubi:device_up:cobertura30d
//	    = count_over_time(:norm[30d]) / 8640     [0,1]      30d ÷ `interval: 5m` = 8640 muestras
//	musubi:project_up:cobertura30d = min by(...) [0,1]
//	delta(...[6h])                               [-1,1]     diferencia de dos valores del rango
//	deriv(...[6h])   = delta ÷ 21600 s      [-4,6e-5 , 4,6e-5]
//
// Con el rango en la mano, «¿este umbral puede cruzarse?» es una comparación y no una opinión:
// `< -0,05` cae adentro de [-1,1] (alcanzable) y no cae adentro de [-4,6e-5 , 4,6e-5] (jamás).
// El mismo cálculo mata el `-5` y el `deriv` SIN NOMBRAR NI A UNO NI AL OTRO.
//
// El 8640 tampoco se copia: sale de dividir la ventana del `count_over_time` por el `interval:`
// del grupo. Si mañana el grupo pasa a `interval: 1m`, la cuenta se mueve sola.
//
// LO ÚNICO QUE NO SE PUEDE DERIVAR DEL REPO es el rango de las métricas CRUDAS, que las produce
// código Go. Eso se DECLARA en deploy/rangos-de-series.yml, y una serie sin rango declarado es
// ROJO — nunca verde. «No pude medir» y «medí y está bien» no pueden ser el mismo resultado: ése
// es el defecto que este archivo entero existe para no repetir.
// ═════════════════════════════════════════════════════════════════════════════════════════════

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// 1. LECTURA DEL REPO
// ─────────────────────────────────────────────────────────────────────────────────────────────

// reglaGrabada es un `- record:` con el `interval:` del grupo que lo evalúa. El intervalo es
// parte del significado: `count_over_time(X[30d])` no vale lo mismo a 5m que a 1m.
type reglaGrabada struct {
	Nombre    string
	Expr      string
	Grupo     string
	Intervalo time.Duration
}

// alertaCruda lleva el `for:` además de la expresión, que es lo que la guarda vieja no miraba.
type alertaCruda struct {
	Nombre     string
	Expr       string
	Plazo      string
	TienePlazo bool
	Archivo    string
}

func reglasGrabadasDelRepo(t *testing.T) map[string]reglaGrabada {
	t.Helper()
	ruta := filepath.Join("..", "..", "deploy", "musubi-recording.yml")
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("falta %s: %v", ruta, err)
	}
	var a struct {
		Groups []struct {
			Name     string `yaml:"name"`
			Interval string `yaml:"interval"`
			Rules    []struct {
				Record string `yaml:"record"`
				Expr   string `yaml:"expr"`
			} `yaml:"rules"`
		} `yaml:"groups"`
	}
	if err := yaml.Unmarshal(crudo, &a); err != nil {
		t.Fatalf("%s no es YAML válido: %v", ruta, err)
	}
	out := map[string]reglaGrabada{}
	for _, g := range a.Groups {
		iv, err := duracionProm(g.Interval)
		if err != nil {
			t.Fatalf("el grupo %q de musubi-recording.yml no declara un `interval:` que se pueda "+
				"leer (%q): sin él no se puede saber cuántas muestras entran en una ventana, y la "+
				"alcanzabilidad de todo lo que cuelga de un `count_over_time` deja de ser medible",
				g.Name, g.Interval)
		}
		for _, r := range g.Rules {
			if r.Record == "" {
				continue
			}
			out[r.Record] = reglaGrabada{Nombre: r.Record, Expr: r.Expr, Grupo: g.Name, Intervalo: iv}
		}
	}
	return out
}

func alertasCrudasDelRepo(t *testing.T) []alertaCruda {
	t.Helper()
	var out []alertaCruda
	for _, f := range archivosDeAlertasDelRepo(t) {
		ruta := filepath.Join("..", "..", "deploy", f)
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatalf("falta %s: %v", ruta, err)
		}
		var a struct {
			Groups []struct {
				Rules []struct {
					Alert string    `yaml:"alert"`
					Expr  string    `yaml:"expr"`
					For   yaml.Node `yaml:"for"`
				} `yaml:"rules"`
			} `yaml:"groups"`
		}
		if err := yaml.Unmarshal(crudo, &a); err != nil {
			t.Fatalf("%s no es YAML válido: %v", ruta, err)
		}
		for _, g := range a.Groups {
			for _, r := range g.Rules {
				if r.Alert == "" {
					continue
				}
				out = append(out, alertaCruda{
					Nombre: r.Alert, Expr: r.Expr,
					Plazo: r.For.Value, TienePlazo: r.For.Kind != 0,
					Archivo: f,
				})
			}
		}
	}
	return out
}

// rangosDeclaradosDeMetricasCrudas lee deploy/rangos-de-series.yml. Es lo ÚNICO que no se deriva.
func rangosDeclaradosDeMetricasCrudas(t *testing.T) map[string]rangoNumerico {
	t.Helper()
	ruta := filepath.Join("..", "..", "deploy", "rangos-de-series.yml")
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("falta %s: %v\n"+
			"  Es la tabla donde se DECLARA el rango de las métricas crudas. Sin ella no se puede "+
			"derivar el rango de ninguna serie grabada, y entonces no se puede decidir si el umbral "+
			"de una alerta es alcanzable. Su ausencia es ROJO y no gris a propósito.", ruta, err)
	}
	var a struct {
		Rangos map[string]struct {
			Min    *float64 `yaml:"min"`
			Max    *float64 `yaml:"max"`
			Unidad string   `yaml:"unidad"`
			Porque string   `yaml:"porque"`
		} `yaml:"rangos"`
	}
	if err := yaml.Unmarshal(crudo, &a); err != nil {
		t.Fatalf("%s no es YAML válido: %v", ruta, err)
	}
	out := map[string]rangoNumerico{}
	for nombre, d := range a.Rangos {
		if d.Min == nil || d.Max == nil {
			t.Errorf("%s declara %s sin `min` o sin `max`.\n"+
				"  Una declaración a medias no sirve para acotar nada: es «no pude medir» "+
				"disfrazado de dato.", ruta, nombre)
			continue
		}
		if *d.Min > *d.Max {
			t.Errorf("%s declara %s con min (%g) mayor que max (%g)", ruta, nombre, *d.Min, *d.Max)
			continue
		}
		if len(strings.TrimSpace(d.Porque)) < 40 {
			t.Errorf("%s declara %s sin decir POR QUÉ ese es su rango.\n"+
				"  El rango es un contrato con el exportador que lo emite; sin el argumento al lado, "+
				"la próxima persona no sabe cuándo dejó de ser cierto.", ruta, nombre)
		}
		out[nombre] = rangoNumerico{lo: *d.Min, hi: *d.Max}
	}
	if len(out) == 0 {
		t.Fatalf("%s no declaró ni un rango. Un verde acá no significaría nada.", ruta)
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// 2. DURACIONES DE PROMETHEUS
// ─────────────────────────────────────────────────────────────────────────────────────────────

var reUnidadDuracion = regexp.MustCompile(`^(\d+)(ms|[smhdwy])`)

// duracionProm entiende `30m`, `6h`, `30d`, `1w`, `1h30m`. Se escribe acá y no se usa
// `model.ParseDuration` para no arrastrar la dependencia entera de Prometheus a una prueba.
func duracionProm(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("duración vacía")
	}
	if s == "0" {
		return 0, nil
	}
	var total time.Duration
	resto := s
	for resto != "" {
		m := reUnidadDuracion.FindStringSubmatch(resto)
		if m == nil {
			return 0, fmt.Errorf("no entiendo la duración %q (se cortó en %q)", s, resto)
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}
		var u time.Duration
		switch m[2] {
		case "ms":
			u = time.Millisecond
		case "s":
			u = time.Second
		case "m":
			u = time.Minute
		case "h":
			u = time.Hour
		case "d":
			u = 24 * time.Hour
		case "w":
			u = 7 * 24 * time.Hour
		case "y":
			u = 365 * 24 * time.Hour
		}
		total += time.Duration(n) * u
		resto = resto[len(m[0]):]
	}
	return total, nil
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// 3. UN PARSER CHICO DE PROMQL (el subconjunto que este repo usa)
//
// No se mira texto: se construye un árbol. Lo que el parser NO entiende es un ERROR, nunca un
// silencio — un constructo desconocido no puede pasar por «alcanzable».
// ─────────────────────────────────────────────────────────────────────────────────────────────

type tokenProm struct {
	tipo  string // "num" | "ident" | "op" | "(" | ")" | "," | "rango" | "llaves"
	texto string
}

func esInicioIdent(c byte) bool {
	return c == '_' || c == ':' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func esCuerpoIdent(c byte) bool { return esInicioIdent(c) || (c >= '0' && c <= '9') }

func lexProm(s string) ([]tokenProm, error) {
	var out []tokenProm
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '{':
			prof, j := 0, i
			for j < len(s) {
				if s[j] == '{' {
					prof++
				} else if s[j] == '}' {
					prof--
					if prof == 0 {
						j++
						break
					}
				}
				j++
			}
			if prof != 0 {
				return nil, fmt.Errorf("`{` sin cerrar")
			}
			out = append(out, tokenProm{"llaves", s[i:j]})
			i = j
		case c == '[':
			j := strings.IndexByte(s[i:], ']')
			if j < 0 {
				return nil, fmt.Errorf("`[` sin cerrar")
			}
			out = append(out, tokenProm{"rango", s[i+1 : i+j]})
			i += j + 1
		case (c >= '0' && c <= '9') || (c == '.' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9'):
			j := i
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == '.') {
				j++
			}
			if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
				k := j + 1
				if k < len(s) && (s[k] == '+' || s[k] == '-') {
					k++
				}
				if k < len(s) && s[k] >= '0' && s[k] <= '9' {
					for k < len(s) && s[k] >= '0' && s[k] <= '9' {
						k++
					}
					j = k
				}
			}
			out = append(out, tokenProm{"num", s[i:j]})
			i = j
		case esInicioIdent(c):
			j := i
			for j < len(s) && esCuerpoIdent(s[j]) {
				j++
			}
			out = append(out, tokenProm{"ident", s[i:j]})
			i = j
		default:
			if i+1 < len(s) {
				switch s[i : i+2] {
				case "==", "!=", "<=", ">=":
					out = append(out, tokenProm{"op", s[i : i+2]})
					i += 2
					continue
				}
			}
			switch c {
			case '(', ')', ',':
				out = append(out, tokenProm{string(c), string(c)})
				i++
			case '+', '-', '*', '/', '%', '^', '<', '>':
				out = append(out, tokenProm{"op", string(c)})
				i++
			default:
				return nil, fmt.Errorf("carácter que no entiendo: %q", string(c))
			}
		}
	}
	return out, nil
}

// nodoProm es el árbol. `tipo` dice qué es y `hijos` cuelga lo que haya.
type nodoProm struct {
	tipo    string // "num" | "serie" | "llamada" | "agregacion" | "binario" | "cmp" | "conjunto"
	num     float64
	nombre  string // nombre de serie, de función, de agregación
	op      string // operador binario o de comparación
	ventana string // `[6h]` si el selector la trae
	hijos   []*nodoProm
}

type analizadorProm struct {
	toks []tokenProm
	i    int
}

func (p *analizadorProm) fin() bool { return p.i >= len(p.toks) }
func (p *analizadorProm) mirar() tokenProm {
	if p.fin() {
		return tokenProm{}
	}
	return p.toks[p.i]
}
func (p *analizadorProm) comer() tokenProm { t := p.mirar(); p.i++; return t }

var agregacionesProm = map[string]bool{
	"sum": true, "min": true, "max": true, "avg": true, "count": true, "group": true,
	"stddev": true, "stdvar": true, "topk": true, "bottomk": true, "quantile": true,
	"count_values": true,
}

func parsearProm(expr string) (*nodoProm, error) {
	toks, err := lexProm(expr)
	if err != nil {
		return nil, err
	}
	p := &analizadorProm{toks: toks}
	n, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if !p.fin() {
		return nil, fmt.Errorf("sobró %q después de parsear", p.mirar().texto)
	}
	return n, nil
}

// saltarModificadorDeJoin se come `on(...)`, `ignoring(...)`, `group_left(...)`, `group_right(...)`.
func (p *analizadorProm) saltarModificadorDeJoin() error {
	for {
		t := p.mirar()
		if t.tipo != "ident" {
			return nil
		}
		switch t.texto {
		case "on", "ignoring", "group_left", "group_right":
			p.comer()
			if p.mirar().tipo == "(" {
				if err := p.saltarGrupoParen(); err != nil {
					return err
				}
			}
		default:
			return nil
		}
	}
}

func (p *analizadorProm) saltarGrupoParen() error {
	if p.mirar().tipo != "(" {
		return fmt.Errorf("esperaba `(` y encontré %q", p.mirar().texto)
	}
	prof := 0
	for !p.fin() {
		t := p.comer()
		if t.tipo == "(" {
			prof++
		} else if t.tipo == ")" {
			prof--
			if prof == 0 {
				return nil
			}
		}
	}
	return fmt.Errorf("`(` sin cerrar")
}

func (p *analizadorProm) parseOr() (*nodoProm, error) {
	n, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.mirar().tipo == "ident" && p.mirar().texto == "or" {
		p.comer()
		if err := p.saltarModificadorDeJoin(); err != nil {
			return nil, err
		}
		d, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		n = &nodoProm{tipo: "conjunto", op: "or", hijos: []*nodoProm{n, d}}
	}
	return n, nil
}

func (p *analizadorProm) parseAnd() (*nodoProm, error) {
	n, err := p.parseCmp()
	if err != nil {
		return nil, err
	}
	for p.mirar().tipo == "ident" && (p.mirar().texto == "and" || p.mirar().texto == "unless") {
		op := p.comer().texto
		if err := p.saltarModificadorDeJoin(); err != nil {
			return nil, err
		}
		d, err := p.parseCmp()
		if err != nil {
			return nil, err
		}
		n = &nodoProm{tipo: "conjunto", op: op, hijos: []*nodoProm{n, d}}
	}
	return n, nil
}

func esOpComparacion(s string) bool {
	switch s {
	case "<", ">", "<=", ">=", "==", "!=":
		return true
	}
	return false
}

func (p *analizadorProm) parseCmp() (*nodoProm, error) {
	n, err := p.parseSuma()
	if err != nil {
		return nil, err
	}
	for p.mirar().tipo == "op" && esOpComparacion(p.mirar().texto) {
		op := p.comer().texto
		if p.mirar().tipo == "ident" && p.mirar().texto == "bool" {
			p.comer()
		}
		if err := p.saltarModificadorDeJoin(); err != nil {
			return nil, err
		}
		d, err := p.parseSuma()
		if err != nil {
			return nil, err
		}
		n = &nodoProm{tipo: "cmp", op: op, hijos: []*nodoProm{n, d}}
	}
	return n, nil
}

func (p *analizadorProm) parseSuma() (*nodoProm, error) {
	n, err := p.parseProducto()
	if err != nil {
		return nil, err
	}
	for p.mirar().tipo == "op" && (p.mirar().texto == "+" || p.mirar().texto == "-") {
		op := p.comer().texto
		if err := p.saltarModificadorDeJoin(); err != nil {
			return nil, err
		}
		d, err := p.parseProducto()
		if err != nil {
			return nil, err
		}
		n = &nodoProm{tipo: "binario", op: op, hijos: []*nodoProm{n, d}}
	}
	return n, nil
}

func (p *analizadorProm) parseProducto() (*nodoProm, error) {
	n, err := p.parseUnario()
	if err != nil {
		return nil, err
	}
	for p.mirar().tipo == "op" && (p.mirar().texto == "*" || p.mirar().texto == "/" || p.mirar().texto == "%") {
		op := p.comer().texto
		if err := p.saltarModificadorDeJoin(); err != nil {
			return nil, err
		}
		d, err := p.parseUnario()
		if err != nil {
			return nil, err
		}
		n = &nodoProm{tipo: "binario", op: op, hijos: []*nodoProm{n, d}}
	}
	return n, nil
}

func (p *analizadorProm) parseUnario() (*nodoProm, error) {
	if p.mirar().tipo == "op" && (p.mirar().texto == "-" || p.mirar().texto == "+") {
		op := p.comer().texto
		n, err := p.parseUnario()
		if err != nil {
			return nil, err
		}
		if op == "-" {
			return &nodoProm{tipo: "binario", op: "-",
				hijos: []*nodoProm{{tipo: "num", num: 0}, n}}, nil
		}
		return n, nil
	}
	return p.parsePrimario()
}

func (p *analizadorProm) parsePrimario() (*nodoProm, error) {
	t := p.mirar()
	switch t.tipo {
	case "num":
		p.comer()
		v, err := strconv.ParseFloat(t.texto, 64)
		if err != nil {
			return nil, err
		}
		return &nodoProm{tipo: "num", num: v}, nil
	case "(":
		p.comer()
		n, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.mirar().tipo != ")" {
			return nil, fmt.Errorf("esperaba `)` y encontré %q", p.mirar().texto)
		}
		p.comer()
		return n, nil
	case "ident":
		nombre := p.comer().texto
		// Agregación: `max by(a,b) (expr)` o `max(expr) by(a,b)`.
		if agregacionesProm[nombre] {
			if p.mirar().tipo == "ident" && (p.mirar().texto == "by" || p.mirar().texto == "without") {
				p.comer()
				if err := p.saltarGrupoParen(); err != nil {
					return nil, err
				}
			}
			if p.mirar().tipo != "(" {
				return nil, fmt.Errorf("la agregación %s no viene seguida de `(`", nombre)
			}
			p.comer()
			var args []*nodoProm
			for {
				a, err := p.parseOr()
				if err != nil {
					return nil, err
				}
				args = append(args, a)
				if p.mirar().tipo == "," {
					p.comer()
					continue
				}
				break
			}
			if p.mirar().tipo != ")" {
				return nil, fmt.Errorf("la agregación %s no cierra su `(`", nombre)
			}
			p.comer()
			if p.mirar().tipo == "ident" && (p.mirar().texto == "by" || p.mirar().texto == "without") {
				p.comer()
				if err := p.saltarGrupoParen(); err != nil {
					return nil, err
				}
			}
			return &nodoProm{tipo: "agregacion", nombre: nombre, hijos: args}, nil
		}
		// Llamada a función.
		if p.mirar().tipo == "(" {
			p.comer()
			var args []*nodoProm
			if p.mirar().tipo != ")" {
				for {
					a, err := p.parseOr()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if p.mirar().tipo == "," {
						p.comer()
						continue
					}
					break
				}
			}
			if p.mirar().tipo != ")" {
				return nil, fmt.Errorf("la función %s no cierra su `(`", nombre)
			}
			p.comer()
			return &nodoProm{tipo: "llamada", nombre: nombre, hijos: args}, nil
		}
		// Selector de serie, con o sin `{...}` y con o sin `[ventana]`.
		n := &nodoProm{tipo: "serie", nombre: nombre}
		if p.mirar().tipo == "llaves" {
			p.comer()
		}
		if p.mirar().tipo == "rango" {
			n.ventana = p.comer().texto
		}
		return n, nil
	}
	return nil, fmt.Errorf("no esperaba %q acá", t.texto)
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// 4. ARITMÉTICA DE INTERVALOS SOBRE EL ÁRBOL
// ─────────────────────────────────────────────────────────────────────────────────────────────

type rangoNumerico struct{ lo, hi float64 }

func (r rangoNumerico) String() string { return fmt.Sprintf("[%g, %g]", r.lo, r.hi) }

type calculadoraDeRango struct {
	grabadas  map[string]reglaGrabada
	crudas    map[string]rangoNumerico
	visitando map[string]bool
	intervalo time.Duration // intervalo de muestreo del grupo cuya regla se está expandiendo

	// consultadas anota qué métricas CRUDAS hizo falta mirar para llegar al resultado. Es lo que
	// vuelve LOAD-BEARING a deploy/rangos-de-series.yml: un rango declarado que ninguna derivación
	// consultó es una declaración que nadie está usando, y se lee como si alguien la hubiera
	// revisado. Ver `huerfanas` en la guarda.
	consultadas map[string]bool
}

func nuevaCalculadora(grabadas map[string]reglaGrabada, crudas map[string]rangoNumerico) *calculadoraDeRango {
	return &calculadoraDeRango{
		grabadas: grabadas, crudas: crudas,
		visitando: map[string]bool{}, consultadas: map[string]bool{},
	}
}

func (c *calculadoraDeRango) ventanaDe(n *nodoProm) (time.Duration, error) {
	if n.tipo != "serie" || n.ventana == "" {
		return 0, fmt.Errorf("esperaba un selector con ventana `[...]`")
	}
	return duracionProm(n.ventana)
}

func (c *calculadoraDeRango) rango(n *nodoProm) (rangoNumerico, error) {
	switch n.tipo {
	case "num":
		return rangoNumerico{n.num, n.num}, nil

	case "serie":
		if r, ok := c.crudas[n.nombre]; ok {
			if c.consultadas != nil {
				c.consultadas[n.nombre] = true
			}
			return r, nil
		}
		g, ok := c.grabadas[n.nombre]
		if !ok {
			return rangoNumerico{}, fmt.Errorf(
				"no sé qué rango de valores puede tomar %s: ni la graba musubi-recording.yml ni "+
					"la declara deploy/rangos-de-series.yml.\n"+
					"       Sin el rango no se puede decidir si un umbral es alcanzable, y «no pude "+
					"medir» no puede pasar por «está bien»: declarala en deploy/rangos-de-series.yml",
				n.nombre)
		}
		if c.visitando[n.nombre] {
			return rangoNumerico{}, fmt.Errorf("ciclo de recording rules en %s", n.nombre)
		}
		c.visitando[n.nombre] = true
		defer delete(c.visitando, n.nombre)
		sub, err := parsearProm(g.Expr)
		if err != nil {
			return rangoNumerico{}, fmt.Errorf("no pude parsear la recording rule %s (%q): %v", n.nombre, g.Expr, err)
		}
		anterior := c.intervalo
		c.intervalo = g.Intervalo
		defer func() { c.intervalo = anterior }()
		r, err := c.rango(sub)
		if err != nil {
			return rangoNumerico{}, fmt.Errorf("%s: %w", n.nombre, err)
		}
		return r, nil

	case "agregacion":
		switch n.nombre {
		case "min", "max", "avg", "group", "quantile", "topk", "bottomk", "stddev":
			// Ninguna de éstas sale del rango de sus entradas (stddev no puede superar el ancho).
			arg := n.hijos[len(n.hijos)-1]
			r, err := c.rango(arg)
			if err != nil {
				return rangoNumerico{}, err
			}
			if n.nombre == "stddev" {
				return rangoNumerico{0, r.hi - r.lo}, nil
			}
			if n.nombre == "group" {
				return rangoNumerico{1, 1}, nil
			}
			return r, nil
		default:
			return rangoNumerico{}, fmt.Errorf(
				"no sé acotar la agregación %s: su resultado depende de cuántas series entren, "+
					"que el repo no dice. Si hace falta, enseñásela a calculadoraDeRango", n.nombre)
		}

	case "llamada":
		switch n.nombre {
		case "avg_over_time", "min_over_time", "max_over_time", "last_over_time",
			"quantile_over_time", "median_over_time":
			return c.rango(n.hijos[len(n.hijos)-1])
		case "present_over_time", "absent", "absent_over_time":
			return rangoNumerico{1, 1}, nil
		case "abs":
			r, err := c.rango(n.hijos[0])
			if err != nil {
				return rangoNumerico{}, err
			}
			if r.lo >= 0 {
				return r, nil
			}
			if r.hi <= 0 {
				return rangoNumerico{-r.hi, -r.lo}, nil
			}
			return rangoNumerico{0, math.Max(-r.lo, r.hi)}, nil
		case "vector", "scalar":
			return c.rango(n.hijos[0])
		case "count_over_time":
			// EL 8640 NO SE COPIA: sale de la ventana dividida por el `interval:` del grupo.
			//
			// SE EVALÚA IGUAL EL RANGO DEL ARGUMENTO aunque el conteo no dependa de sus valores:
			// un eslabón de la cadena que la guarda no sabe acotar es una cadena que nadie midió,
			// y un `count_over_time` se convierte en un `avg_over_time` cambiando una palabra. Si
			// no se exigiera acá, el rango declarado de las métricas crudas no lo consultaría
			// NADIE en la cadena de cobertura —medido: sacar `musubi_fleet_device_up` de
			// deploy/rangos-de-series.yml dejaba la guarda en verde— y la declaración sería
			// decorado.
			if _, err := c.rango(n.hijos[0]); err != nil {
				return rangoNumerico{}, err
			}
			w, err := c.ventanaDe(n.hijos[0])
			if err != nil {
				return rangoNumerico{}, fmt.Errorf("count_over_time: %v", err)
			}
			if c.intervalo <= 0 {
				return rangoNumerico{}, fmt.Errorf(
					"count_over_time sin un `interval:` conocido: no se puede saber cuántas muestras " +
						"entran en la ventana")
			}
			return rangoNumerico{0, float64(w) / float64(c.intervalo)}, nil
		case "delta", "idelta":
			r, err := c.rango(n.hijos[0])
			if err != nil {
				return rangoNumerico{}, err
			}
			// La diferencia entre dos valores del rango, en las MISMAS unidades de la serie.
			return rangoNumerico{r.lo - r.hi, r.hi - r.lo}, nil
		case "deriv", "rate", "irate":
			// LO MISMO PERO POR SEGUNDO. Acá muere el sabotaje (3): no porque la función se llame
			// `deriv` sino porque dividir por los segundos de la ventana achica el rango cuatro
			// órdenes de magnitud y el umbral se queda afuera.
			r, err := c.rango(n.hijos[0])
			if err != nil {
				return rangoNumerico{}, err
			}
			w, err := c.ventanaDe(n.hijos[0])
			if err != nil {
				return rangoNumerico{}, fmt.Errorf("%s: %v", n.nombre, err)
			}
			seg := w.Seconds()
			if seg <= 0 {
				return rangoNumerico{}, fmt.Errorf("%s con ventana de cero segundos", n.nombre)
			}
			return rangoNumerico{(r.lo - r.hi) / seg, (r.hi - r.lo) / seg}, nil
		case "increase":
			r, err := c.rango(n.hijos[0])
			if err != nil {
				return rangoNumerico{}, err
			}
			return rangoNumerico{0, r.hi - r.lo}, nil
		default:
			return rangoNumerico{}, fmt.Errorf(
				"no sé qué rango devuelve la función %s. Enseñásela a calculadoraDeRango antes de "+
					"usarla en una alerta: una función que la guarda no entiende no puede pasar por "+
					"«umbral alcanzable»", n.nombre)
		}

	case "binario":
		a, err := c.rango(n.hijos[0])
		if err != nil {
			return rangoNumerico{}, err
		}
		b, err := c.rango(n.hijos[1])
		if err != nil {
			return rangoNumerico{}, err
		}
		switch n.op {
		case "+":
			return rangoNumerico{a.lo + b.lo, a.hi + b.hi}, nil
		case "-":
			return rangoNumerico{a.lo - b.hi, a.hi - b.lo}, nil
		case "*":
			return extremos(a.lo*b.lo, a.lo*b.hi, a.hi*b.lo, a.hi*b.hi), nil
		case "/":
			if b.lo <= 0 && b.hi >= 0 {
				return rangoNumerico{}, fmt.Errorf("división por un rango que incluye el 0 (%v)", b)
			}
			return extremos(a.lo/b.lo, a.lo/b.hi, a.hi/b.lo, a.hi/b.hi), nil
		default:
			return rangoNumerico{}, fmt.Errorf("no sé acotar el operador %q", n.op)
		}

	case "cmp":
		// Un filtro no cambia los valores: los recorta. Se intersecta con la constante si la hay.
		a, err := c.rango(n.hijos[0])
		if err != nil {
			return rangoNumerico{}, err
		}
		b, err := c.rango(n.hijos[1])
		if err != nil {
			return rangoNumerico{}, err
		}
		if b.lo != b.hi {
			return a, nil
		}
		switch n.op {
		case "<", "<=":
			return rangoNumerico{a.lo, math.Min(a.hi, b.hi)}, nil
		case ">", ">=":
			return rangoNumerico{math.Max(a.lo, b.lo), a.hi}, nil
		}
		return a, nil

	case "conjunto":
		a, err := c.rango(n.hijos[0])
		if err != nil {
			return rangoNumerico{}, err
		}
		// El lado derecho se evalúa SIEMPRE, aunque `and`/`unless` no usen sus valores: si la
		// guarda no sabe acotarlo, no puede afirmar que entendió la expresión. Mismo motivo que
		// en `count_over_time`.
		b, err := c.rango(n.hijos[1])
		if err != nil {
			return rangoNumerico{}, err
		}
		if n.op == "or" {
			return rangoNumerico{math.Min(a.lo, b.lo), math.Max(a.hi, b.hi)}, nil
		}
		// `and` / `unless` filtran por etiquetas y devuelven los valores de la izquierda.
		return a, nil
	}
	return rangoNumerico{}, fmt.Errorf("nodo %q que no sé evaluar", n.tipo)
}

func extremos(vs ...float64) rangoNumerico {
	lo, hi := vs[0], vs[0]
	for _, v := range vs[1:] {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	return rangoNumerico{lo, hi}
}

// recorrer visita cada nodo del árbol.
func recorrer(n *nodoProm, f func(*nodoProm)) {
	if n == nil {
		return
	}
	f(n)
	for _, h := range n.hijos {
		recorrer(h, f)
	}
}

// ventanasDe devuelve todas las ventanas `[...]` del árbol.
func ventanasDe(n *nodoProm) ([]time.Duration, error) {
	var out []time.Duration
	var err error
	recorrer(n, func(x *nodoProm) {
		if x.tipo == "serie" && x.ventana != "" {
			d, e := duracionProm(x.ventana)
			if e != nil {
				err = e
				return
			}
			out = append(out, d)
		}
	})
	return out, err
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// 5. LAS GUARDAS
// ─────────────────────────────────────────────────────────────────────────────────────────────

// alertasEnMira son las que leen alguna serie GRABADA por musubi-recording.yml: sobre esas se
// puede derivar el rango, así que sobre esas se exige que el umbral sea alcanzable.
func alertasEnMira(t *testing.T) ([]alertaCruda, map[string]reglaGrabada) {
	t.Helper()
	grabadas := reglasGrabadasDelRepo(t)
	if len(grabadas) < 10 {
		t.Fatalf("sólo leí %d recording rules de musubi-recording.yml; el parseo dejó de mirar y un "+
			"verde acá no significaría nada", len(grabadas))
	}
	todas := alertasCrudasDelRepo(t)
	if len(todas) < 10 {
		t.Fatalf("sólo leí %d alertas de %v; el barrido dejó de mirar", len(todas), archivosDeAlertasDelRepo(t))
	}
	var mira []alertaCruda
	for _, a := range todas {
		for nombre := range grabadas {
			if nombraLaSerie(a.Expr, nombre) {
				mira = append(mira, a)
				break
			}
		}
	}
	sort.Slice(mira, func(i, j int) bool { return mira[i].Nombre < mira[j].Nombre })
	// UN CERO ACÁ NO ES «TODO BIEN». Son dos desde que existe la hermana de servicios; si
	// aparecen cero es que alguien borró las alertas de cobertura o que el detector dejó de ver.
	if len(mira) < 2 {
		t.Fatalf("sólo %d alerta(s) leen una serie grabada por musubi-recording.yml.\n"+
			"  Eran dos —`CoberturaDelSlaSeCayo` y `CoberturaDelSlaDeServiciosSeCayo`—: o se borró "+
			"una, o este detector dejó de mirar. En los dos casos es «no pude medir», no «está bien».",
			len(mira))
	}
	return mira, grabadas
}

// alertasSinUmbralNumerico es la escapatoria ESCRITA para una alerta sobre una serie grabada que
// no compara contra ningún número (`absent(...)`, por ejemplo). Vacía hoy, y a propósito: si
// mañana hace falta una, el motivo queda acá y no en la cabeza de nadie.
var alertasSinUmbralNumerico = map[string]string{}

// EL UMBRAL DE CADA ALERTA SOBRE UNA SERIE GRABADA TIENE QUE SER ALCANZABLE.
//
// Sabotajes que la hacen fallar (los tres, verificados):
//   - `delta(musubi:project_service_up:cobertura30d[6h]) < -5`   (umbral fuera del rango)
//   - `deriv(...)` en vez de `delta(...)` con el umbral intacto  (unidades por segundo)
//   - cualquiera de los dos sobre `CoberturaDelSlaSeCayo`, la alerta de máquinas
func TestElUmbralDeCadaAlertaSobreUnaSerieGrabadaEsAlcanzable(t *testing.T) {
	mira, grabadas := alertasEnMira(t)
	crudas := rangosDeclaradosDeMetricasCrudas(t)

	usadas := map[string]bool{}
	evaluadas := 0
	for _, a := range mira {
		arbol, err := parsearProm(a.Expr)
		if err != nil {
			t.Errorf("%s (%s): no pude parsear su expresión %q: %v\n"+
				"  Una expresión que la guarda no entiende no puede darse por buena: enseñale la "+
				"forma nueva al parser de alertas_umbral_alcanzable_test.go.", a.Nombre, a.Archivo, a.Expr, err)
			continue
		}
		var comparaciones []*nodoProm
		recorrer(arbol, func(x *nodoProm) {
			if x.tipo == "cmp" {
				comparaciones = append(comparaciones, x)
			}
		})

		conUmbral := 0
		for _, cmp := range comparaciones {
			calcI := nuevaCalculadora(grabadas, crudas)
			izq, errI := calcI.rango(cmp.hijos[0])
			calcD := nuevaCalculadora(grabadas, crudas)
			der, errD := calcD.rango(cmp.hijos[1])
			for _, c := range []*calculadoraDeRango{calcI, calcD} {
				for m := range c.consultadas {
					usadas[m] = true
				}
			}

			op, serie, umbral := cmp.op, izq, der
			switch {
			case errI != nil && errD != nil:
				conUmbral++ // la comparación EXISTE; lo que falló fue derivar su rango.
				t.Errorf("%s (%s): no pude derivar el rango de ninguno de los dos lados de la "+
					"comparación en %q.\n    izquierda: %v\n    derecha:   %v",
					a.Nombre, a.Archivo, a.Expr, errI, errD)
				continue
			case errI == nil && errD == nil && der.lo == der.hi:
				// forma normal: <expresión> OP <constante>
			case errI == nil && errD == nil && izq.lo == izq.hi:
				op, serie, umbral = espejarOp(cmp.op), der, izq
			case errI != nil || errD != nil:
				conUmbral++ // ídem: la comparación existe, la cadena es la que no se pudo acotar.
				t.Errorf("%s (%s): no pude derivar el rango de un lado de la comparación en %q.\n"+
					"    izquierda: %v\n    derecha:   %v\n"+
					"  Un lado que la guarda no sabe acotar no puede pasar por «umbral alcanzable».",
					a.Nombre, a.Archivo, a.Expr, errI, errD)
				continue
			default:
				// Comparación entre dos vectores: no hay umbral numérico que juzgar.
				continue
			}
			conUmbral++
			evaluadas++
			u := umbral.lo
			alcanzable, siempre := juzgarUmbral(op, serie, u)
			if !alcanzable {
				t.Errorf("%s (%s) TIENE UN UMBRAL QUE NO PUEDE CRUZARSE NUNCA: `%s %s %g`\n"+
					"  Ese lado sólo puede valer %v —derivado de musubi-recording.yml y de "+
					"deploy/rangos-de-series.yml, no copiado de ningún lado—, así que la condición es "+
					"falsa por construcción y esta alerta NO VA A SONAR JAMÁS.\n"+
					"  Una alarma inalcanzable se ve exactamente igual que una que no tiene nada que "+
					"decir: no falla, no se queja y no avisa.\n"+
					"  Revisá las UNIDADES antes que el número: la cobertura es un RATIO en [0,1] "+
					"(«5 puntos» son 0,05, no 5) y `deriv`/`rate` son POR SEGUNDO, así que un umbral "+
					"pensado para `delta` queda cuatro órdenes de magnitud afuera.",
					a.Nombre, a.Archivo, textoDe(cmp.hijos[0]), cmp.op, u, serie)
			} else if siempre {
				t.Errorf("%s (%s) tiene un umbral que se cumple SIEMPRE: `%s %s %g`, con ese lado "+
					"acotado en %v.\n"+
					"  Una alerta que dispara siempre entrena a ignorar el canal —este repo ya lo "+
					"pagó con trece `MaquinaCaida`—.",
					a.Nombre, a.Archivo, textoDe(cmp.hijos[0]), cmp.op, u, serie)
			}
		}
		if conUmbral == 0 {
			if motivo, ok := alertasSinUmbralNumerico[a.Nombre]; ok && len(strings.TrimSpace(motivo)) >= 40 {
				continue
			}
			t.Errorf("%s (%s) lee una serie grabada y no compara contra ningún número: %q\n"+
				"  Sin umbral no hay nada que juzgar, y esta guarda no puede decir si la alerta puede "+
				"disparar. Si es a propósito —un `absent(...)`, por ejemplo—, escribí el motivo en "+
				"`alertasSinUmbralNumerico`; un caso sin motivo escrito es «no pude medir», no «está bien».",
				a.Nombre, a.Archivo, a.Expr)
		}
	}

	if evaluadas == 0 {
		t.Fatalf("no se evaluó ni un umbral. La guarda quedó muda y su verde no significa nada.")
	}
	// LA TABLA DECLARADA NO PUEDE PODRIRSE, Y TIENE QUE ESTAR SOSTENIENDO ALGO. `usadas` no se
	// llena mirando el texto de las reglas: se llena con las métricas que la DERIVACIÓN tuvo que
	// consultar de verdad para llegar al rango. Una entrada que ninguna derivación consultó no
	// está sosteniendo ninguna cuenta, y se lee como si alguien la hubiera revisado.
	var huerfanas []string
	for nombre := range crudas {
		if !usadas[nombre] {
			huerfanas = append(huerfanas, nombre)
		}
	}
	sort.Strings(huerfanas)
	if len(huerfanas) > 0 {
		t.Errorf("deploy/rangos-de-series.yml declara el rango de %v y ninguna derivación de umbral "+
			"tuvo que consultarlo.\n"+
			"  Una declaración que no sostiene ninguna cuenta es peor que ninguna: se lee como "+
			"revisada y no lo está. Borrala, o revisá por qué la cadena de la alerta dejó de pasar "+
			"por esa métrica.", huerfanas)
	}
	if len(usadas) == 0 {
		t.Fatalf("la derivación no consultó NI UNA métrica cruda declarada. O la cadena de las "+
			"alertas de cobertura ya no llega a las series crudas, o el rastreo dejó de anotar: en "+
			"los dos casos el rango declarado no está sosteniendo nada y el verde no significa nada. "+
			"(%d declaradas)", len(crudas))
	}
}

func espejarOp(op string) string {
	switch op {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	}
	return op
}

// juzgarUmbral dice si la condición `serie OP umbral` puede cumplirse alguna vez (alcanzable) y si
// se cumple para todo valor posible (siempre).
func juzgarUmbral(op string, serie rangoNumerico, u float64) (alcanzable, siempre bool) {
	switch op {
	case "<":
		return serie.lo < u, serie.hi < u
	case "<=":
		return serie.lo <= u, serie.hi <= u
	case ">":
		return serie.hi > u, serie.lo > u
	case ">=":
		return serie.hi >= u, serie.lo >= u
	case "==":
		return serie.lo <= u && u <= serie.hi, serie.lo == u && serie.hi == u
	case "!=":
		return !(serie.lo == u && serie.hi == u), !(serie.lo <= u && u <= serie.hi)
	}
	return true, false
}

// textoDe reconstruye lo justo para que el mensaje de error se lea.
func textoDe(n *nodoProm) string {
	switch n.tipo {
	case "num":
		return strconv.FormatFloat(n.num, 'g', -1, 64)
	case "serie":
		if n.ventana != "" {
			return n.nombre + "[" + n.ventana + "]"
		}
		return n.nombre
	case "llamada", "agregacion":
		var partes []string
		for _, h := range n.hijos {
			partes = append(partes, textoDe(h))
		}
		return n.nombre + "(" + strings.Join(partes, ", ") + ")"
	case "binario", "cmp", "conjunto":
		return textoDe(n.hijos[0]) + " " + n.op + " " + textoDe(n.hijos[1])
	}
	return "?"
}

// EL PLAZO (`for:`) TAMBIÉN TIENE QUE SER ALCANZABLE.
//
// Una condición medida sobre una ventana de 6 h no puede sostenerse 30 días: cuando la caída sale
// de la ventana, la condición se apaga sola. O sea que `for` ≥ ventana es una alerta muda, y se ve
// idéntica a una sana.
//
// Sabotaje que la hace fallar (verificado): cambiar `for: 30m` por `for: 30d` en cualquiera de las
// dos alertas de cobertura.
func TestElPlazoDeCadaAlertaSobreUnaSerieGrabadaEsAlcanzable(t *testing.T) {
	mira, _ := alertasEnMira(t)
	revisadas, conVentana := 0, 0
	for _, a := range mira {
		arbol, err := parsearProm(a.Expr)
		if err != nil {
			t.Errorf("%s (%s): no pude parsear %q: %v", a.Nombre, a.Archivo, a.Expr, err)
			continue
		}
		vents, err := ventanasDe(arbol)
		if err != nil {
			t.Errorf("%s (%s): no pude leer una de sus ventanas: %v", a.Nombre, a.Archivo, err)
			continue
		}
		// EL TECHO DEL PLAZO SALE DE LA VENTANA, Y UNA ALERTA SIN VENTANA NO TIENE TECHO.
		//
		// Lo que hace inalcanzable a un `for:` es que la condición SE APAGUE SOLA: `delta(X[6h])`
		// vuelve a cero cuando la caída sale de esas seis horas, así que un `for` de 30d nunca se
		// cumple. Una alerta de NIVEL —`musubi:device_up:cobertura30d < 0.5`— no tiene ese
		// mecanismo: el nivel se queda donde está indefinidamente, así que CUALQUIER `for` finito
		// se cumple. Su techo es infinito, y eso es una MEDICIÓN, no un encogimiento de hombros.
		//
		// La versión anterior de esta guarda trataba «no hay ventana» como error y le agregaba un
		// juicio de producto («una alerta de NIVEL sobre el SLA no es accionable»). Medido: eso
		// rechazaba en ROJO una alerta perfectamente sana —`musubi:device_up:cobertura30d < 0.5`
		// con `for: 2h`— sin ninguna escapatoria escrita. Un falso positivo así no se discute: se
		// apaga la guarda entera, y con ella los tres sabotajes que sí agarra.
		//
		// Que las DOS alertas de cobertura tengan que ser de CAÍDA y no de NIVEL ya lo sostiene
		// `TestCadaCoberturaDeSlaPorProyectoTieneUnaAlertaQueLaLee` (verificado: cambiarlas a
		// `musubi:project_service_up:cobertura30d < 0.95` da rojo ahí). Ésa es la guarda que
		// decide la FORMA; ésta decide las MAGNITUDES y no tiene por qué opinar de la otra cosa.
		conTecho := len(vents) > 0
		var menor time.Duration
		if conTecho {
			menor = vents[0]
			for _, v := range vents[1:] {
				if v < menor {
					menor = v
				}
			}
		}
		if !a.TienePlazo {
			donde := "Sin ventana en la expresión el plazo es la única defensa contra el ruido"
			if conTecho {
				donde = fmt.Sprintf("Sobre una ventana de %s el plazo es parte de la decisión", menor)
			}
			t.Errorf("%s (%s) no declara `for:`.\n"+
				"  %s: sin él la alerta dispara en la primera evaluación que cruce el umbral. "+
				"Declaralo, aunque sea `for: 0s`.", a.Nombre, a.Archivo, donde)
			continue
		}
		plazo, err := duracionProm(a.Plazo)
		if err != nil {
			t.Errorf("%s (%s): no entiendo su `for: %s`: %v", a.Nombre, a.Archivo, a.Plazo, err)
			continue
		}
		revisadas++
		if !conTecho {
			// Plazo finito contra techo infinito: se cumple. Medido y sano, no salteado.
			continue
		}
		conVentana++
		if plazo >= menor {
			t.Errorf("%s (%s) TIENE UN PLAZO QUE NO PUEDE CUMPLIRSE: `for: %s` sobre una ventana de %s.\n"+
				"  La condición se mide sobre %s: cuando la caída sale de esa ventana, la condición se "+
				"apaga sola. Para que `for` se cumpla habría que sostenerla %s seguidos, o sea que esta "+
				"alerta NO VA A SONAR JAMÁS y se ve idéntica a una sana.\n"+
				"  El plazo tiene que ser ESTRICTAMENTE menor que la ventana más chica de la expresión.",
				a.Nombre, a.Archivo, a.Plazo, menor, menor, a.Plazo)
		}
	}
	if revisadas == 0 {
		t.Fatalf("no se revisó ni un `for:`. La guarda quedó muda.")
	}
	// Y ADEMÁS: que se hayan revisado plazos no alcanza. La comparación que agarra el sabotaje del
	// `for: 30d` es plazo-contra-ventana, y ésa sólo corre sobre alertas CON ventana. Si un día
	// todas las alertas en mira son de nivel, `revisadas` sigue subiendo y esta guarda no midió
	// nada de lo que existe para medir. Son dos —las de cobertura, ambas `delta(...[6h])`—.
	if conVentana < 2 {
		t.Fatalf("sólo %d alerta(s) con ventana llegaron a la comparación plazo-contra-ventana.\n"+
			"  Eran dos —`CoberturaDelSlaSeCayo` y `CoberturaDelSlaDeServiciosSeCayo`, las dos sobre "+
			"`[6h]`—. Menos que eso es «no pude medir», no «está bien»: el sabotaje del `for: 30d` "+
			"sólo lo agarra esta comparación.", conVentana)
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// 6. Y LAS SERIES QUE NO LAS LEE NADIE
//
// `musubi:project_up:min30d` es la métrica insignia —«el peor equipo del cliente», la que le
// contesta a un cliente enojado— y no la referencia NADIE fuera de su propia definición. Igual
// `musubi:device_up:avg7d`. Una serie que no lee nadie no es necesariamente un defecto; una serie
// que no lee nadie SIN QUE ESTÉ ESCRITO POR QUÉ, sí: la próxima auditoría vuelve a descubrir lo
// mismo y no sabe si ya se decidió.
//
// Así que la decisión vive acá, ejecutable: toda serie grabada tiene que tener un lector —una
// alerta o otra recording rule— o un motivo escrito de por qué no lo tiene.
// ─────────────────────────────────────────────────────────────────────────────────────────────

// seriesSinLectorConMotivo: por qué estas series NO tienen alerta. Cada entrada es una decisión
// tomada, con su argumento, no una excepción para que la prueba pase.
var seriesSinLectorConMotivo = map[string]string{
	"musubi:project_up:min30d": "" +
		"EL PEOR EQUIPO DEL CLIENTE, Y NO LE CORRESPONDE ALERTA. (a) Una alerta de NIVEL sobre un " +
		"promedio de 30 días no es accionable: ningún arreglo baja un mes de historia, la única " +
		"salida es esperar, y una alarma que no se apaga actuando es cómo se apaga un canal (A79, " +
		"trece MaquinaCaida). Lo accionable —una máquina caída AHORA— ya lo avisa MaquinaCaida con " +
		"90 s de latencia, no con 30 días. (b) Una alerta de CAÍDA sería inalcanzable por " +
		"construcción: seis horas de caída mueven un promedio de 30 días 0,008, o sea por debajo de " +
		"cualquier umbral honesto — exactamente la clase de umbral que " +
		"TestElUmbralDeCadaAlertaSobreUnaSerieGrabadaEsAlcanzable existe para rechazar. (c) La falla " +
		"que esta serie SÍ tuvo —el 10,2 % fantasma por los duplicados de `instance`, siendo " +
		"84,7 %— es una pérdida de MEDICIÓN, y ésa está vigilada: desciende de " +
		"`musubi:device_up:norm`, igual que `musubi:project_up:cobertura30d`, así que si la medición " +
		"se cae, CoberturaDelSlaSeCayo suena. Lo que le falta a esta serie no es una alerta: es el " +
		"CONSUMIDOR del reporte, que todavía no existe.",
	"musubi:device_up:avg7d": "" +
		"UN PROMEDIO NO TIENE UMBRAL HONESTO. El mismo 99 % es «una máquina rota siete días» y " +
		"«siete máquinas con un hipo»: un umbral sobre el promedio no distingue las dos, y la " +
		"accionable de las dos ya la avisa MaquinaCaida por máquina. Existe como DIAGNÓSTICO para " +
		"leer al lado de avg30d —una caída reciente se ve en 7 días y se diluye en 30—, no como " +
		"número a vigilar. Su pérdida de medición la cubre la cobertura, que sale del mismo " +
		"`musubi:device_up:norm`.",
	"musubi:device_up:sla30d": "" +
		"NO LLEVA ALERTA PORQUE SU DISEÑO YA ES LA ALERTA: la serie DESAPARECE donde la cobertura " +
		"cae debajo de 0,95, así que un panel dibuja un hueco en vez de un número que no se puede " +
		"sostener. Y su desaparición se anuncia ANTES: CoberturaDelSlaSeCayo mira la cobertura que " +
		"la condiciona. Es el número que se publica, no un síntoma que haya que perseguir.",
	"musubi:project_up:sla30d": "" +
		"Mismo argumento que musubi:device_up:sla30d: la ausencia de la serie ES el aviso, y la " +
		"caída de la cobertura que la condiciona la vigila CoberturaDelSlaSeCayo. Es el número que " +
		"se le firma al cliente; alertar sobre su nivel sería alertar sobre el SLA vendido, que se " +
		"discute en una reunión y no a las tres de la mañana.",
	"musubi:service_up:sla30d": "" +
		"Mismo argumento, del lado de los servicios: desaparece donde la cobertura de servicios cae " +
		"debajo de 0,95, y esa caída la vigila CoberturaDelSlaDeServiciosSeCayo. Además su nivel " +
		"está dominado por servicios ociosos por diseño (A70), así que un umbral sobre él sería " +
		"cierto y no informaría.",
}

// TODA SERIE GRABADA TIENE UN LECTOR, O UN MOTIVO ESCRITO DE POR QUÉ NO LO TIENE.
//
// Sabotaje que la hace fallar: agregar un `- record:` nuevo a musubi-recording.yml sin darle
// lector ni entrada acá; o borrar el motivo de `musubi:project_up:min30d`.
func TestCadaSerieGrabadaTieneUnLectorOUnMotivoEscrito(t *testing.T) {
	grabadas := reglasGrabadasDelRepo(t)
	if len(grabadas) < 10 {
		t.Fatalf("sólo leí %d recording rules; el parseo dejó de mirar", len(grabadas))
	}
	alertas := alertasCrudasDelRepo(t)
	if len(alertas) < 10 {
		t.Fatalf("sólo leí %d alertas; el barrido dejó de mirar", len(alertas))
	}

	leePorAlerta := map[string]bool{}
	leePorRegla := map[string]bool{}
	for nombre := range grabadas {
		for _, a := range alertas {
			if nombraLaSerie(a.Expr, nombre) {
				leePorAlerta[nombre] = true
				break
			}
		}
		for otro, g := range grabadas {
			if otro == nombre {
				continue
			}
			if nombraLaSerie(g.Expr, nombre) {
				leePorRegla[nombre] = true
				break
			}
		}
	}

	var sinMotivo []string
	for nombre := range grabadas {
		if leePorAlerta[nombre] || leePorRegla[nombre] {
			continue
		}
		if len(strings.TrimSpace(seriesSinLectorConMotivo[nombre])) < 120 {
			sinMotivo = append(sinMotivo, nombre)
		}
	}
	sort.Strings(sinMotivo)
	if len(sinMotivo) > 0 {
		t.Errorf("estas series grabadas no las lee NADIE —ni una alerta ni otra recording rule— y no "+
			"dicen por qué: %v\n"+
			"  Una serie que nadie lee puede ser una decisión o puede ser un cabo suelto, y desde "+
			"afuera se ven igual: eso fue `musubi:project_service_up:cobertura30d`, grabada desde A93 "+
			"y sin un solo lector.\n"+
			"  Dale un lector, o escribí el argumento en `seriesSinLectorConMotivo` "+
			"(internal/mcp/alertas_umbral_alcanzable_test.go) para que la próxima auditoría lo "+
			"encuentre y no vuelva a decidirlo desde cero.", sinMotivo)
	}

	// LA TABLA TAMPOCO PUEDE PODRIRSE, en las dos direcciones.
	for nombre, motivo := range seriesSinLectorConMotivo {
		if _, ok := grabadas[nombre]; !ok {
			t.Errorf("`seriesSinLectorConMotivo` explica por qué %s no tiene alerta y musubi-recording.yml "+
				"ya no la graba. Un motivo sobre una serie inexistente se lee como revisado y no lo está.", nombre)
			continue
		}
		if leePorAlerta[nombre] {
			t.Errorf("`seriesSinLectorConMotivo` dice que %s no tiene alerta, y sí la tiene. "+
				"Sacala de la tabla: un motivo que miente es peor que ninguno.", nombre)
		}
		if len(strings.TrimSpace(motivo)) < 120 {
			t.Errorf("el motivo de %s es demasiado corto para ser un argumento (%d caracteres). "+
				"La tabla existe para que la próxima auditoría no tenga que decidir de nuevo.",
				nombre, len(strings.TrimSpace(motivo)))
		}
	}

	// Y EL PISO: si el cruce no encontró ni un lector, no midió nada.
	if len(leePorAlerta) == 0 {
		t.Fatalf("ninguna serie grabada la lee una alerta. O se borraron las alertas de cobertura o "+
			"este cruce dejó de mirar; en los dos casos es «no pude medir». (%d series, %d alertas)",
			len(grabadas), len(alertas))
	}
}
