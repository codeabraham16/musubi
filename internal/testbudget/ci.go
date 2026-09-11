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
//	  → disparadores (`on:`), `env:` de los tres niveles, jobs y pasos, con su `run` YA
//	    NORMALIZADO (bloque e inline dan el mismo string)
//	  → heredocs sacados aparte (su cuerpo NO son comandos, y era otra boca del mismo caño)
//	  → léxico de shell (comillas, comentarios, `;` `&&` `||` `|` `(` `)`, `$(...)`, redirecciones)
//	    conservando el SEPARADOR con que termina cada tubería (`|| true` se come un rojo)
//	  → argv de cada comando de cada tubería
//	  → flags de `go test` parseados por NOMBRE, con su valor venga `-timeout X` o `-timeout=X`
//	  → el valor se CLASIFICA: ausente / referencia al techo de la política / cualquier otra cosa.
//
// Así las formas de arriba —y las que no se me ocurrieron— caen todas en el mismo lugar.
//
// Y LO QUE EL PARSER NO ENTIENDE ES ROJO, NO VERDE. Un mini-intérprete de GitHub Actions siempre
// va a tener un agujero; lo que no puede tener es un agujero SILENCIOSO. Por eso hay dos
// contrastes contra el texto crudo —uno por `go test`, otro por RACE_TIMEOUT— que se ponen rojos
// cuando el archivo nombra algo que el parser no llegó a leer: «no pude mirar» nunca puede salir
// por la misma puerta que «miré y está bien».

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// nombreVarTecho es la variable por la que viaja el techo desde el archivo de política hasta el
// `go test` que lo consume. Todo lo que la nombre tiene que pasar por acá.
const nombreVarTecho = "RACE_TIMEOUT"

// PasoCI es un paso de un job, con lo que hace falta para juzgarlo.
type PasoCI struct {
	Job               string
	Indice            int // posición dentro del job, 0-based: sirve para el «antes que»
	Nombre            string
	Run               string
	Shell             string // el `shell:` EFECTIVO: el del paso, o el `defaults.run.shell` que herede
	Si                string // `if:` del paso
	ContinuarConError string // `continue-on-error:` del paso, como texto
	Env               map[string]string

	// Lo del JOB que decide sobre el paso: un `if:` o un `continue-on-error:` puesto un nivel
	// más arriba apaga el paso igual de bien, y no estaba mirado.
	SiDelJob                string
	ContinuarConErrorDelJob string
}

// Comando es UN comando (un eslabón de una tubería) con su argv.
//
// Argv trae los tokens con las comillas ya sacadas; Bruto los trae tal cual estaban escritos.
// Los dos hacen falta: la estructura se lee del primero, pero para decidir si `$RACE_TIMEOUT` se
// va a EXPANDIR hay que mirar el segundo (entre comillas simples no expande).
//
// Heredocs son los cuerpos de los `<<EOF` que este comando lleva pegados. NO son comandos: son
// datos que el comando escribe, y ahí es donde se puede publicar el techo a mano sin que ningún
// argv lo muestre.
type Comando struct {
	Argv     []string
	Bruto    []string
	Heredocs []string
}

// Tuberia es una tubería completa (`a | b | c`) dentro de un paso.
//
// Separador es el operador con el que TERMINA la tubería: `||`, `&&`, `;`, salto de línea, `&`,
// o vacío cuando es la última y no hay operador detrás. Sin
// él, `cmd || true` y `cmd` son indistinguibles para el parser — y son lo contrario en lo único
// que importa acá, que es si el rojo del comando llega al paso.
type Tuberia struct {
	Paso       *PasoCI
	Cmds       []Comando
	Texto      string
	Separador  string
	Indice     int // posición de la tubería dentro del script del paso
	UltimaDelP bool
}

// DeclaracionEnv es un `env:` de YAML, con el nivel donde estaba escrito.
//
// LOS TRES NIVELES. `env:` existe a nivel workflow, job y paso, y cualquiera de los tres pisa la
// variable SIN pasar por el archivo de política. El parser no tenía el campo, así que
// `env: { RACE_TIMEOUT: 20m }` era invisible en los tres.
type DeclaracionEnv struct {
	Nivel string // "workflow", "job" o "paso"
	Job   string
	Paso  string
	Clave string
	Valor string
}

// FlujoCI es el workflow parseado.
type FlujoCI struct {
	Ruta     string
	Eventos  []string // los disparadores de `on:`
	Envs     []DeclaracionEnv
	Pasos    []*PasoCI
	Tuberias []Tuberia
}

type mapaEnv map[string]any

type defaultsYAML struct {
	Run struct {
		Shell string `yaml:"shell"`
	} `yaml:"run"`
}

type pasoYAML struct {
	Name            string  `yaml:"name"`
	Run             string  `yaml:"run"`
	Shell           string  `yaml:"shell"`
	If              string  `yaml:"if"`
	ContinueOnError any     `yaml:"continue-on-error"`
	Env             mapaEnv `yaml:"env"`
}

type jobYAML struct {
	If              string       `yaml:"if"`
	ContinueOnError any          `yaml:"continue-on-error"`
	Env             mapaEnv      `yaml:"env"`
	Defaults        defaultsYAML `yaml:"defaults"`
	Steps           []pasoYAML   `yaml:"steps"`
}

type flujoYAML struct {
	On       any                `yaml:"on"`
	Env      mapaEnv            `yaml:"env"`
	Defaults defaultsYAML       `yaml:"defaults"`
	Jobs     map[string]jobYAML `yaml:"jobs"`
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
	flujo := &FlujoCI{Ruta: ruta, Eventos: eventosDe(f.On)}
	flujo.Envs = append(flujo.Envs, declaracionesEnv("workflow", "", "", f.Env)...)

	nombres := make([]string, 0, len(f.Jobs))
	for j := range f.Jobs {
		nombres = append(nombres, j)
	}
	sort.Strings(nombres)
	for _, job := range nombres {
		jb := f.Jobs[job]
		flujo.Envs = append(flujo.Envs, declaracionesEnv("job", job, "", jb.Env)...)
		shellPorDefecto := primeroNoVacio(jb.Defaults.Run.Shell, f.Defaults.Run.Shell)
		for i, s := range jb.Steps {
			p := &PasoCI{
				Job:                     job,
				Indice:                  i,
				Nombre:                  s.Name,
				Run:                     s.Run,
				Shell:                   primeroNoVacio(s.Shell, shellPorDefecto),
				Si:                      s.If,
				Env:                     aplanarEnv(s.Env),
				SiDelJob:                jb.If,
				ContinuarConErrorDelJob: comoTexto(jb.ContinueOnError),
			}
			p.ContinuarConError = comoTexto(s.ContinueOnError)
			flujo.Envs = append(flujo.Envs, declaracionesEnv("paso", job, s.Name, s.Env)...)
			flujo.Pasos = append(flujo.Pasos, p)
			if strings.TrimSpace(s.Run) == "" {
				continue
			}
			ts := lexearShell(normalizarExpresionesGH(s.Run))
			for k := range ts {
				ts[k].Paso = p
				ts[k].Indice = k
				ts[k].UltimaDelP = k == len(ts)-1
			}
			flujo.Tuberias = append(flujo.Tuberias, ts...)
		}
	}
	if err := verificarCoberturaDelLexico(ruta, contenido, flujo); err != nil {
		return nil, err
	}
	if err := verificarCoberturaDelTecho(ruta, contenido, flujo); err != nil {
		return nil, err
	}
	return flujo, nil
}

func primeroNoVacio(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func comoTexto(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func aplanarEnv(m mapaEnv) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = comoTexto(v)
	}
	return out
}

func declaracionesEnv(nivel, job, paso string, m mapaEnv) []DeclaracionEnv {
	if len(m) == 0 {
		return nil
	}
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	out := make([]DeclaracionEnv, 0, len(claves))
	for _, k := range claves {
		out = append(out, DeclaracionEnv{Nivel: nivel, Job: job, Paso: paso, Clave: k, Valor: comoTexto(m[k])})
	}
	return out
}

// eventosDe normaliza el `on:` en sus tres formas: escalar, lista y mapa.
func eventosDe(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, e := range t {
			out = append(out, comoTexto(e))
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(t))
		for k := range t {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	return nil
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
	// Un `go test` adentro de un heredoc SÍ se ejecuta (`bash <<EOF`), y este parser no lo puede
	// juzgar: el cuerpo son datos, no argv. No entenderlo es rojo, no verde.
	for _, t := range f.Tuberias {
		for _, c := range t.Cmds {
			for _, h := range c.Heredocs {
				if reGoTestCrudo.MatchString(h) {
					return fmt.Errorf("%s · job %q · paso %q: hay un `go test` adentro del cuerpo de "+
						"un heredoc. Esta guarda parsea argv, no cuerpos de heredoc, así que no puede "+
						"decir con qué techo corre: escribilo como un comando o esta guarda queda "+
						"verde por no haber podido mirar", ruta, t.Paso.Job, t.Paso.Nombre)
				}
			}
		}
	}
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

// verificarCoberturaDelTecho es el MISMO contraste, sobre la variable en vez de sobre el comando.
//
// Es lo que hace que este mini-intérprete no tenga agujeros silenciosos. Los cinco sabotajes de
// la ronda 3 eran todos «el techo publicado de una forma que el parser no lee»: un `env:` de
// YAML, un heredoc, un `with:` de una action. Enumerar esas formas nunca termina — siempre falta
// la próxima. Lo que sí termina es preguntar: ¿el archivo nombra RACE_TIMEOUT en algún lugar que
// yo no llegué a leer? Si la respuesta es sí, esto es ROJO y dice dónde.
func verificarCoberturaDelTecho(ruta, contenido string, f *FlujoCI) error {
	var enTexto int
	for _, linea := range strings.Split(contenido, "\n") {
		s := strings.TrimSpace(linea)
		if strings.HasPrefix(s, "#") {
			continue
		}
		enTexto += strings.Count(s, nombreVarTecho)
	}
	var leido int
	for _, p := range f.Pasos {
		for _, s := range []string{p.Run, p.Nombre, p.Si, p.ContinuarConError, p.Shell} {
			leido += strings.Count(s, nombreVarTecho)
		}
	}
	vistosJobs := map[string]bool{}
	for _, p := range f.Pasos {
		if vistosJobs[p.Job] {
			continue
		}
		vistosJobs[p.Job] = true
		leido += strings.Count(p.SiDelJob, nombreVarTecho) + strings.Count(p.ContinuarConErrorDelJob, nombreVarTecho)
	}
	for _, e := range f.Envs {
		leido += strings.Count(e.Clave, nombreVarTecho) + strings.Count(e.Valor, nombreVarTecho)
	}
	if leido < enTexto {
		return fmt.Errorf("%s nombra %s %d veces y este parser sólo llegó a leer %d de ellas: hay "+
			"una construcción de GitHub Actions que no entiendo tocando el techo (un `with:` de una "+
			"action, una clave nueva, un `defaults:`). Un techo que no sé de dónde sale no puede "+
			"pasar en verde: agregá la construcción a ParsearCI o sacála del workflow",
			ruta, nombreVarTecho, enTexto, leido)
	}
	return nil
}

// ---------------------------------------------------------------------------------------------
// Heredocs
//
// El cuerpo de un `<<EOF` NO son comandos: son datos. Lexearlo como comandos daba dos errores a
// la vez —inventaba comandos que nadie corre y perdía de vista lo que el comando ESCRIBE—, y ahí
// vivía una boca entera del caño: `cat >> "$GITHUB_ENV" <<EOF` / `RACE_TIMEOUT=40m` / `EOF`
// publica el techo a mano sin que aparezca en ningún argv.
//
// Se sacan ANTES de lexear y se cuelgan del comando que los abrió.

const (
	prefijoHeredoc  = "\x00hd:"
	sufijoHeredoc   = "\x00"
	marcaHereString = "\x00hs\x00"
)

var reAperturaHeredoc = regexp.MustCompile(`<<-?[ \t]*(?:'([^']*)'|"([^"]*)"|([A-Za-z_][A-Za-z0-9_]*))`)

// extraerHeredocs devuelve el script con los cuerpos sacados (y el operador reemplazado por una
// marca que el léxico conserva como un token) más los cuerpos, en orden de apertura.
func extraerHeredocs(script string) (string, []string) {
	lineas := strings.Split(script, "\n")
	var salida []string
	var cuerpos []string
	for i := 0; i < len(lineas); i++ {
		// `<<<` es una here-string, no un heredoc: se saca de la vista para que el regex no lo
		// confunda con `<<` + palabra.
		l := strings.ReplaceAll(lineas[i], "<<<", marcaHereString)
		var delims []string
		l = reAperturaHeredoc.ReplaceAllStringFunc(l, func(m string) string {
			g := reAperturaHeredoc.FindStringSubmatch(m)
			d := g[1] + g[2] + g[3]
			if d == "" {
				return m
			}
			delims = append(delims, d)
			cuerpos = append(cuerpos, "")
			return " " + prefijoHeredoc + strconv.Itoa(len(cuerpos)-1) + sufijoHeredoc + " "
		})
		salida = append(salida, strings.ReplaceAll(l, marcaHereString, "<<<"))
		if len(delims) == 0 {
			continue
		}
		base := len(cuerpos) - len(delims)
		for k, d := range delims {
			var cuerpo []string
			for i+1 < len(lineas) {
				i++
				if strings.TrimSpace(lineas[i]) == d {
					break
				}
				cuerpo = append(cuerpo, lineas[i])
			}
			cuerpos[base+k] = strings.Join(cuerpo, "\n")
		}
	}
	return strings.Join(salida, "\n"), cuerpos
}

func indiceHeredoc(tok string) (int, bool) {
	resto, ok := strings.CutPrefix(tok, prefijoHeredoc)
	if !ok {
		return 0, false
	}
	resto, ok = strings.CutSuffix(resto, sufijoHeredoc)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(resto)
	if err != nil {
		return 0, false
	}
	return n, true
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
// `$(...)` como parte de un token, redirecciones, heredocs, y los separadores `\n ; & && || | ( )`
// —guardando CUÁL fue, porque `cmd || true` no es `cmd`.
func lexearShell(script string) []Tuberia {
	script, cuerpos := extraerHeredocs(script)
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
	finTuberia := func(sep string) {
		finCmd()
		if len(tuberia) > 0 {
			out = append(out, Tuberia{Cmds: tuberia, Separador: sep})
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
			finTuberia("#")
		case c == ' ' || c == '\t' || c == '\r':
			finTok()
		case c == '\n' || c == ';' || c == '(' || c == ')':
			finTuberia(string(c))
		case c == '&':
			sep := "&"
			if i+1 < len(r) && r[i+1] == '&' {
				i++
				sep = "&&"
			}
			finTuberia(sep)
		case c == '|':
			if i+1 < len(r) && r[i+1] == '|' {
				i++
				finTuberia("||")
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
	finTuberia("")
	for ti := range out {
		for ci := range out[ti].Cmds {
			colgarHeredocs(&out[ti].Cmds[ci], cuerpos)
		}
		out[ti].Texto = textoDe(out[ti].Cmds)
	}
	return out
}

// colgarHeredocs saca del argv las marcas que dejó extraerHeredocs y les cuelga su cuerpo.
func colgarHeredocs(c *Comando, cuerpos []string) {
	var argv, bruto []string
	for k, a := range c.Argv {
		if n, ok := indiceHeredoc(a); ok {
			if n < len(cuerpos) {
				c.Heredocs = append(c.Heredocs, cuerpos[n])
			}
			continue
		}
		argv = append(argv, a)
		bruto = append(bruto, c.Bruto[k])
	}
	c.Argv, c.Bruto = argv, bruto
}

func textoDe(t []Comando) string {
	partes := make([]string, 0, len(t))
	for _, c := range t {
		p := strings.Join(c.Bruto, " ")
		for _, h := range c.Heredocs {
			p += " <<EOF{" + strings.ReplaceAll(strings.TrimSpace(h), "\n", " ; ") + "}"
		}
		partes = append(partes, p)
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

// esReferenciaAlTecho acepta el valor con o sin las comillas dobles de afuera.
func esReferenciaAlTecho(bruto string) bool {
	b := strings.TrimSpace(bruto)
	return referenciasAlTecho[b] || referenciasAlTecho[`"`+b+`"`]
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
