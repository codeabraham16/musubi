package testbudget

// EL ci.yml SE PARSEA, NO SE GREPEA.
//
// La primera versión de esta guarda miraba LÍNEAS: pedía que un `go test` empezara renglón (o
// viniera después de un `;`/`&&`/`|`) y que en esa misma línea no apareciera un literal de
// duración pegado a `-timeout`. Doce sabotajes distintos quedaron VERDES contra eso, y todos por
// el mismo motivo: la guarda preguntaba por la FORMA DEL TEXTO y no por la FORMA QUE DECIDE.
//
//   - `run: go test -timeout 40m ./...` en una sola línea (la forma «inline» de YAML) no arranca
//     renglón ni viene después de un separador, así que el ancla no lo reconocía y el número
//     tipeado a mano pasaba.
//   - `-timeout  40m` (dos espacios) y `-timeout '40m'` no matcheaban el literal.
//   - SACAR el `-timeout` entero no era ni candidato: las líneas sin el flag se filtraban ANTES
//     de mirar, así que la guarda vigilaba que el número no se retipeara y NO que el techo
//     existiera.
//
// Agregar esas formas a una lista de formas malas sólo hubiera aplazado el problema: a una lista
// de formas malas siempre le falta la próxima. Lo que hay acá en cambio es un pipeline de
// derivación:
//
//	YAML (yaml.v3, el mismo parser que usa GitHub Actions)
//	  → jobs y pasos, con su `run` YA NORMALIZADO (bloque e inline dan el mismo string)
//	  → léxico de shell (comillas, comentarios, `;` `&&` `||` `|` `(` `)`, `$(...)`, redirecciones)
//	  → argv de cada comando de cada tubería
//	  → flags de `go test` parseados por NOMBRE, con su valor venga `-timeout X` o `-timeout=X`
//	  → el valor se CLASIFICA: ausente / referencia al techo de la política / cualquier otra cosa.
//
// Así las tres formas de arriba —y las que no se me ocurrieron— caen todas en el mismo lugar.

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PasoCI es un paso de un job, con lo que hace falta para juzgarlo.
type PasoCI struct {
	Job               string
	Indice            int // posición dentro del job, 0-based: sirve para el «antes que»
	Nombre            string
	Run               string
	Shell             string
	Si                string // `if:` del paso
	ContinuarConError string // `continue-on-error:` del paso, como texto
}

// Comando es UN comando (un eslabón de una tubería) con su argv.
//
// Argv trae los tokens con las comillas ya sacadas; Bruto los trae tal cual estaban escritos.
// Los dos hacen falta: la estructura se lee del primero, pero para decidir si `$RACE_TIMEOUT` se
// va a EXPANDIR hay que mirar el segundo (entre comillas simples no expande).
type Comando struct {
	Argv  []string
	Bruto []string
}

// Tuberia es una tubería completa (`a | b | c`) dentro de un paso.
type Tuberia struct {
	Paso  *PasoCI
	Cmds  []Comando
	Texto string
}

// FlujoCI es el workflow parseado.
type FlujoCI struct {
	Ruta     string
	Pasos    []*PasoCI
	Tuberias []Tuberia
}

type pasoYAML struct {
	Name            string `yaml:"name"`
	Run             string `yaml:"run"`
	Shell           string `yaml:"shell"`
	If              string `yaml:"if"`
	ContinueOnError any    `yaml:"continue-on-error"`
}

type flujoYAML struct {
	Jobs map[string]struct {
		Steps []pasoYAML `yaml:"steps"`
	} `yaml:"jobs"`
}

// reExpresionGH normaliza `${{ env.X }}` a `${{env.X}}`: es UN valor, no tres tokens, y el
// léxico de shell lo partiría por los espacios de adentro.
var reExpresionGH = regexp.MustCompile(`\$\{\{\s*(.*?)\s*\}\}`)

func normalizarExpresionesGH(s string) string {
	return reExpresionGH.ReplaceAllStringFunc(s, func(m string) string {
		sub := reExpresionGH.FindStringSubmatch(m)
		return "${{" + strings.Join(strings.Fields(sub[1]), " ") + "}}"
	})
}

// LeerCI parsea un workflow de GitHub Actions.
func LeerCI(ruta string) (*FlujoCI, error) {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", ruta, err)
	}
	return ParsearCI(ruta, string(b))
}

// ParsearCI parsea el contenido de un workflow. Está separado de LeerCI para poder probar el
// juicio contra YAML sintético —incluidos los sabotajes— sin tocar el archivo real del repo.
func ParsearCI(ruta, contenido string) (*FlujoCI, error) {
	var f flujoYAML
	if err := yaml.Unmarshal([]byte(contenido), &f); err != nil {
		return nil, fmt.Errorf("%s no parsea como YAML: %w", ruta, err)
	}
	if len(f.Jobs) == 0 {
		return nil, fmt.Errorf("%s no declara ni un job: o cambió de forma o esta guarda dejó de "+
			"mirar lo que dice mirar", ruta)
	}
	flujo := &FlujoCI{Ruta: ruta}
	nombres := make([]string, 0, len(f.Jobs))
	for j := range f.Jobs {
		nombres = append(nombres, j)
	}
	sort.Strings(nombres)
	for _, job := range nombres {
		for i, s := range f.Jobs[job].Steps {
			p := &PasoCI{
				Job:    job,
				Indice: i,
				Nombre: s.Name,
				Run:    s.Run,
				Shell:  s.Shell,
				Si:     s.If,
			}
			if s.ContinueOnError != nil {
				p.ContinuarConError = strings.TrimSpace(fmt.Sprint(s.ContinueOnError))
			}
			flujo.Pasos = append(flujo.Pasos, p)
			if strings.TrimSpace(s.Run) == "" {
				continue
			}
			for _, t := range lexearShell(normalizarExpresionesGH(s.Run)) {
				t.Paso = p
				flujo.Tuberias = append(flujo.Tuberias, t)
			}
		}
	}
	if err := verificarCoberturaDelLexico(ruta, contenido, flujo); err != nil {
		return nil, err
	}
	return flujo, nil
}

// reGoTestCrudo cuenta `go test` en el texto pelado, para contrastar contra lo que vio el léxico.
var reGoTestCrudo = regexp.MustCompile(`\bgo\s+test\b`)

// verificarCoberturaDelLexico protege el modo de falla propio de esta guarda: que el parser deje
// de ver comandos y por eso no encuentre nada que objetar.
//
// Un `go test` que el léxico no reconoció es exactamente el agujero que se está cerrando, sólo
// que un nivel más adentro. Si el texto crudo tiene más `go test` que el argv parseado, esto es
// un error DURO: «no pude mirar» nunca puede salir por la misma puerta que «miré y está bien».
func verificarCoberturaDelLexico(ruta, contenido string, f *FlujoCI) error {
	var enTexto int
	for _, linea := range strings.Split(contenido, "\n") {
		s := strings.TrimSpace(linea)
		if strings.HasPrefix(s, "#") {
			continue
		}
		// Un `#` con espacio adelante, adentro de un `run:`, es comentario de shell.
		if i := strings.Index(s, " #"); i >= 0 {
			s = s[:i]
		}
		enTexto += len(reGoTestCrudo.FindAllString(s, -1))
	}
	vistos := len(ComandosGoTest(f))
	if vistos < enTexto {
		return fmt.Errorf("el léxico de %s reconoció %d comandos `go test` y el texto tiene %d: "+
			"el parser dejó de ver comandos, así que esta guarda quedaría verde por no haber mirado",
			ruta, vistos, enTexto)
	}
	return nil
}

// ---------------------------------------------------------------------------------------------
// Léxico de shell

type tokenLex struct {
	v     string
	bruto string
}

// lexearShell parte un script en tuberías y comandos con su argv.
//
// No pretende ser un shell: pretende que NINGUNA forma de escribir el mismo comando se le
// escape. Maneja comillas simples y dobles, escapes, comentarios, continuación de línea,
// `$(...)` como parte de un token, redirecciones, y los separadores `\n ; & && || | ( )`.
func lexearShell(script string) []Tuberia {
	var (
		out     []Tuberia
		tuberia []Comando
		argv    []tokenLex
		cur     tokenLex
		enTok   bool
	)
	finTok := func() {
		if enTok {
			argv = append(argv, cur)
			cur, enTok = tokenLex{}, false
		}
	}
	finCmd := func() {
		finTok()
		if len(argv) > 0 {
			var c Comando
			for _, t := range argv {
				c.Argv = append(c.Argv, t.v)
				c.Bruto = append(c.Bruto, t.bruto)
			}
			tuberia = append(tuberia, c)
			argv = nil
		}
	}
	finTuberia := func() {
		finCmd()
		if len(tuberia) > 0 {
			out = append(out, Tuberia{Cmds: tuberia, Texto: textoDe(tuberia)})
			tuberia = nil
		}
	}
	agregar := func(v, bruto string) {
		cur.v += v
		cur.bruto += bruto
		enTok = true
	}

	r := []rune(script)
	tope := func(i int) int {
		if i > len(r) {
			return len(r)
		}
		return i
	}
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\'':
			j := i + 1
			for j < len(r) && r[j] != '\'' {
				j++
			}
			agregar(string(r[i+1:tope(j)]), string(r[i:tope(j+1)]))
			i = j
		case c == '"':
			var v, bruto strings.Builder
			bruto.WriteRune('"')
			j := i + 1
			for j < len(r) && r[j] != '"' {
				if r[j] == '\\' && j+1 < len(r) {
					bruto.WriteRune(r[j])
					bruto.WriteRune(r[j+1])
					v.WriteRune(r[j+1])
					j += 2
					continue
				}
				bruto.WriteRune(r[j])
				v.WriteRune(r[j])
				j++
			}
			if j < len(r) {
				bruto.WriteRune('"')
			}
			agregar(v.String(), bruto.String())
			i = j
		case c == '\\' && i+1 < len(r):
			if r[i+1] == '\n' { // continuación de línea
				i++
				continue
			}
			agregar(string(r[i+1]), string(r[i:i+2]))
			i++
		case c == '$' && i+1 < len(r) && r[i+1] == '(':
			prof, j := 0, i+1
			for ; j < len(r); j++ {
				if r[j] == '(' {
					prof++
				} else if r[j] == ')' {
					prof--
					if prof == 0 {
						break
					}
				}
			}
			agregar(string(r[i:tope(j+1)]), string(r[i:tope(j+1)]))
			i = j
		case c == '#' && !enTok:
			for i < len(r) && r[i] != '\n' {
				i++
			}
			finTuberia()
		case c == ' ' || c == '\t' || c == '\r':
			finTok()
		case c == '\n' || c == ';' || c == '(' || c == ')':
			finTuberia()
		case c == '&':
			if i+1 < len(r) && r[i+1] == '&' {
				i++
			}
			finTuberia()
		case c == '|':
			if i+1 < len(r) && r[i+1] == '|' {
				i++
				finTuberia()
				continue
			}
			finCmd()
		case c == '>' || c == '<':
			// Redirección: el destino es un token aparte, no un argumento del comando.
			finTok()
			agregar(string(c), string(c))
			if i+1 < len(r) && r[i+1] == c {
				agregar(string(c), string(c))
				i++
			}
			finTok()
			// `2>&1`: el `&` de un duplicado de descriptor NO es el separador `&`. Confundirlos
			// partía la tubería justo antes del `| tee …`, y con eso se perdía el archivo donde
			// la corrida guarda su salida — el dato del que cuelga todo el guard del margen.
			if i+1 < len(r) && r[i+1] == '&' {
				i++
				agregar("&", "&")
			}
		default:
			agregar(string(c), string(c))
		}
	}
	finTuberia()
	return out
}

func textoDe(t []Comando) string {
	partes := make([]string, 0, len(t))
	for _, c := range t {
		partes = append(partes, strings.Join(c.Bruto, " "))
	}
	return strings.Join(partes, " | ")
}

// ---------------------------------------------------------------------------------------------
// `go test` y sus flags

// flagsConValorSeparado son los flags de `go test` cuyo valor puede venir en el token siguiente.
// El parseo por `=` no necesita lista; ésta sólo evita confundir el VALOR de un flag con un
// patrón de paquete.
var flagsConValorSeparado = map[string]bool{
	"timeout": true, "run": true, "bench": true, "benchtime": true, "count": true,
	"tags": true, "coverprofile": true, "covermode": true, "cpu": true, "parallel": true,
	"gcflags": true, "ldflags": true, "exec": true, "o": true, "fuzztime": true, "fuzz": true,
}

// GoTest es un `go test` del workflow, ya parseado.
type GoTest struct {
	Tuberia  Tuberia
	Paso     *PasoCI
	Flags    map[string]string // valor sin comillas; "" para los booleanos
	Brutos   map[string]string // el mismo valor tal cual estaba escrito
	Paquetes []string          // patrones de paquete
}

// ComandosGoTest saca del flujo todos los `go test`, vengan como vengan.
func ComandosGoTest(f *FlujoCI) []GoTest {
	var out []GoTest
	for _, t := range f.Tuberias {
		for _, c := range t.Cmds {
			if len(c.Argv) < 2 || c.Argv[0] != "go" || c.Argv[1] != "test" {
				continue
			}
			g := GoTest{Tuberia: t, Paso: t.Paso, Flags: map[string]string{}, Brutos: map[string]string{}}
			for i := 2; i < len(c.Argv); i++ {
				a := c.Argv[i]
				if a == "" || !strings.HasPrefix(a, "-") {
					g.Paquetes = append(g.Paquetes, a)
					continue
				}
				nombre := strings.TrimLeft(a, "-")
				if k, v, ok := strings.Cut(nombre, "="); ok {
					_, vb, _ := strings.Cut(c.Bruto[i], "=")
					g.Flags[k] = v
					g.Brutos[k] = vb
					continue
				}
				if flagsConValorSeparado[nombre] && i+1 < len(c.Argv) {
					g.Flags[nombre] = c.Argv[i+1]
					g.Brutos[nombre] = c.Bruto[i+1]
					i++
					continue
				}
				g.Flags[nombre] = ""
				g.Brutos[nombre] = ""
			}
			out = append(out, g)
		}
	}
	return out
}

// referenciasAlTecho son las ÚNICAS formas aceptadas de tomar el techo de la política. Es una
// lista de formas BUENAS —canónica y cerrada—, no una lista de formas malas: cualquier cosa que
// no esté acá (un literal, otra variable, una comilla simple que impide la expansión, un vacío)
// cae del lado del error.
var referenciasAlTecho = map[string]bool{
	"$RACE_TIMEOUT":           true,
	"${RACE_TIMEOUT}":         true,
	"${{env.RACE_TIMEOUT}}":   true,
	`"$RACE_TIMEOUT"`:         true,
	`"${RACE_TIMEOUT}"`:       true,
	`"${{env.RACE_TIMEOUT}}"`: true,
}

// ClaseDeTecho dice de dónde sale el valor de un `-timeout`.
type ClaseDeTecho int

const (
	TechoAusente  ClaseDeTecho = iota // no hay -timeout: `go test` cae en su default de 10m
	TechoDerivado                     // sale de $RACE_TIMEOUT, o sea del archivo de política
	TechoTipeado                      // un literal de duración, o cualquier otra cosa
)

func (c ClaseDeTecho) String() string {
	switch c {
	case TechoAusente:
		return "ausente"
	case TechoDerivado:
		return "derivado"
	default:
		return "tipeado"
	}
}

// Techo clasifica el `-timeout` de un `go test`. Devuelve también el valor tal como se escribió.
func (g GoTest) Techo() (ClaseDeTecho, string) {
	bruto, ok := g.Brutos["timeout"]
	if !ok {
		return TechoAusente, ""
	}
	if referenciasAlTecho[strings.TrimSpace(bruto)] {
		return TechoDerivado, bruto
	}
	return TechoTipeado, bruto
}

// EsDuracionDeGo dice si el texto es un literal de duración. Sólo se usa para escribir un error
// más preciso: la decisión NO depende de esto (si dependiera, la próxima forma de tipear el
// número se escaparía).
func EsDuracionDeGo(s string) bool {
	s = strings.Trim(strings.TrimSpace(s), `"'`)
	if s == "" {
		return false
	}
	_, err := time.ParseDuration(s)
	return err == nil
}

// CorreTests dice si el comando ejecuta los TESTS del paquete. Un `-bench` con `-run=NONE` no
// los corre: es un guard de benchmark, mide otra cosa y tiene su propio presupuesto.
func (g GoTest) CorreTests() bool {
	if _, hayBench := g.Flags["bench"]; hayBench {
		if r, ok := g.Flags["run"]; ok && (r == "NONE" || r == "^$" || r == "XXX") {
			return false
		}
	}
	return true
}

// Cubre dice si los patrones de paquete del comando alcanzan al paquete dir (relativo a la raíz
// del módulo, p. ej. "internal/mcp").
func (g GoTest) Cubre(modulo, dir string) bool {
	for _, p := range g.Paquetes {
		if patronCubre(modulo, p, dir) {
			return true
		}
	}
	return false
}

func patronCubre(modulo, patron, dir string) bool {
	patron = strings.TrimSpace(patron)
	if patron == "" {
		return false
	}
	patron = strings.TrimPrefix(patron, modulo+"/")
	patron = strings.TrimPrefix(patron, "./")
	patron = strings.TrimSuffix(patron, "/")
	dir = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(dir, "./"), modulo+"/"), "/")
	if patron == "..." || patron == modulo {
		return true
	}
	if resto, ok := strings.CutSuffix(patron, "/..."); ok {
		return dir == resto || strings.HasPrefix(dir, resto+"/")
	}
	return patron == dir
}
