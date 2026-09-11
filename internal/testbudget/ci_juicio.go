package testbudget

// EL JUICIO SOBRE EL ci.yml.
//
// Está en código de producción y no en el _test.go a propósito: así los mismos jueces se pueden
// correr contra workflows SINTÉTICOS —cada sabotaje, uno por uno, con su forma exacta— y no sólo
// contra el archivo real del repo. Una guarda que sólo se ejerce contra el archivo bueno nunca
// se vio a sí misma en rojo.

import (
	"fmt"
	"regexp"
	"strings"
)

// ErroresDeTecho juzga de dónde saca el techo cada `go test` del workflow.
//
// Tres cosas, y las tres eran agujeros medidos:
//   - un `-timeout` que no sale de la política es un número tipeado, venga como venga escrito
//     (dos espacios, comillas simples, el `run:` en línea: todo eso ya está normalizado a argv
//     cuando llega acá);
//   - un `go test` que corre los tests de un paquete caro SIN `-timeout` cae en el default de
//     10m de Go — el defecto original en su forma más pura, y el que antes no era ni candidato
//     porque las líneas sin el flag se filtraban ANTES de mirar;
//   - si NINGÚN `go test` toma el techo de la política, el aparato está desconectado.
func ErroresDeTecho(f *FlujoCI, modulo string, caros []string) []error {
	gs := ComandosGoTest(f)
	if len(gs) == 0 {
		return []error{fmt.Errorf("%s no tiene NI UN comando `go test`: o el workflow cambió de "+
			"forma o esta guarda dejó de mirar lo que dice mirar", f.Ruta)}
	}
	var errs []error
	var derivados int
	for _, g := range gs {
		clase, valor := g.Techo()
		switch clase {
		case TechoDerivado:
			derivados++
		case TechoTipeado:
			detalle := "no sale de " + NombreArchivoPolitica
			if EsDuracionDeGo(valor) {
				detalle = "es un literal de duración tipeado a mano"
			}
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: `go test -timeout %s` %s.\n"+
				"    Usá -timeout \"$RACE_TIMEOUT\" (o ${{ env.RACE_TIMEOUT }}), que sale del paso "+
				"«Presupuesto de pruebas (política)».\n    comando: %s",
				f.Ruta, g.Paso.Job, g.Paso.Nombre, valor, detalle, g.Tuberia.Texto))
		case TechoAusente:
			if !g.CorreTests() {
				continue // guard de benchmark: no corre los tests del paquete.
			}
			for _, c := range caros {
				if g.Cubre(modulo, c) {
					errs = append(errs, fmt.Errorf("%s · job %q · paso %q: `go test` corre los "+
						"tests de %s SIN -timeout, así que se come el default de Go (10m POR "+
						"PAQUETE): el techo de la política no rige ahí y la corrida muere con "+
						"«panic: test timed out» culpando al test que justo estuviera corriendo.\n"+
						"    comando: %s",
						f.Ruta, g.Paso.Job, g.Paso.Nombre, c, g.Tuberia.Texto))
					break
				}
			}
		}
	}
	if derivados == 0 {
		errs = append(errs, fmt.Errorf("%s: ningún `go test` toma el techo de %s. El archivo de "+
			"política sigue en el repo y no lo lee nadie", f.Ruta, NombreArchivoPolitica))
	}
	return errs
}

// publicacion es un RACE_TIMEOUT que este workflow le deja al entorno del job, con TODO lo que
// hace falta para decir de dónde salió su valor.
type publicacion struct {
	paso        *PasoCI
	valor       string // el lado derecho, tal como estaba escrito
	via         string // "argv" o "cuerpo de heredoc"
	leyoLaPol   bool   // hubo un `source ./presupuesto-de-pruebas.env` ANTES, en este mismo paso
	pisadoCon   string // reasignación de RACE_TIMEOUT anterior a la publicación, si la hubo
	fuenteOpaca string // `source` de OTRO archivo antes de publicar: no sé qué le deja
}

// ErroresDePublicacion juzga el nivel de adentro: que el valor que le llega a `go test` por
// $RACE_TIMEOUT salga del archivo de política y no esté tipeado en el camino.
//
// TRES BOCAS DEL MISMO CAÑO, y las tres estaban medidas:
//
//   - el `echo RACE_TIMEOUT=… >> $GITHUB_ENV` con el número a mano (el `source` queda intacto
//     arriba, así que cualquier chequeo de presencia sigue contento);
//   - el `env:` de YAML, en cualquiera de sus TRES niveles (workflow, job, paso), que pisa la
//     variable sin pasar por el archivo;
//   - la publicación que no viaja por un argv: un heredoc (`cat >> "$GITHUB_ENV" <<EOF`) o un
//     `| tee -a "$GITHUB_ENV"`.
//
// Y la cuarta, que es la que obliga a derivar en vez de reconocer: dejar el `source` intacto Y
// el `echo "RACE_TIMEOUT=$RACE_TIMEOUT"` intacto, y meter UNA línea en el medio que reasigne la
// variable. Todo lo que un chequeo de formas mira sigue exactamente igual. Por eso el paso se
// recorre EN ORDEN llevando de dónde viene el valor en cada punto.
func ErroresDePublicacion(f *FlujoCI) []error {
	var errs []error

	// (0) El `env:` de YAML. Ningún `env:` puede leer el archivo de política, así que declarar
	// el techo ahí es tipearlo — no hay forma buena de hacerlo salvo pasar el valor que ya viene.
	for _, e := range f.Envs {
		if e.Clave != nombreVarTecho || esReferenciaAlTecho(e.Valor) {
			continue
		}
		donde := "a nivel " + e.Nivel
		if e.Nivel == "job" {
			donde = fmt.Sprintf("en el job %q", e.Job)
		} else if e.Nivel == "paso" {
			donde = fmt.Sprintf("en el job %q, paso %q", e.Job, e.Paso)
		}
		errs = append(errs, fmt.Errorf("%s: hay un `env:` %s que declara %s=%s. Un `env:` de YAML "+
			"no puede leer %s: pisa el techo con un número tipeado y el paso que hace `source` "+
			"queda de adorno. El techo viaja por $GITHUB_ENV desde el paso que lee el archivo",
			f.Ruta, donde, e.Clave, e.Valor, NombreArchivoPolitica))
	}

	var pubs []publicacion
	for _, p := range f.Pasos {
		pubs = append(pubs, publicacionesDelPaso(p)...)
	}
	for _, p := range pubs {
		switch {
		case !esReferenciaAlTecho(p.valor):
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: publica %s=%s en $GITHUB_ENV "+
				"(%s) y ese valor no sale de %s. El `source` puede seguir escrito ahí arriba: lo "+
				"que rige es el valor que viaja",
				f.Ruta, p.paso.Job, p.paso.Nombre, nombreVarTecho, p.valor, p.via, NombreArchivoPolitica))
		case p.pisadoCon != "":
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: publica %s=%s en $GITHUB_ENV, "+
				"pero ANTES —en el mismo paso— reasigna %s=%s. El `source` y el `echo` quedan "+
				"intactos y lo que viaja es %s: el archivo de política no rige",
				f.Ruta, p.paso.Job, p.paso.Nombre, nombreVarTecho, p.valor, nombreVarTecho,
				p.pisadoCon, p.pisadoCon))
		case p.fuenteOpaca != "":
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: antes de publicar %s hace "+
				"`source ./%s`, que no es %s. No puedo derivar qué valor queda en la variable, y "+
				"un techo que no sé de dónde sale no puede pasar en verde",
				f.Ruta, p.paso.Job, p.paso.Nombre, nombreVarTecho, p.fuenteOpaca, NombreArchivoPolitica))
		case !p.leyoLaPol:
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: publica %s en $GITHUB_ENV sin "+
				"hacer `source ./%s` antes en el mismo paso, así que la variable llega vacía y "+
				"`go test` cae en su default de 10m",
				f.Ruta, p.paso.Job, p.paso.Nombre, nombreVarTecho, NombreArchivoPolitica))
		}
	}
	// Y el otro lado: todo job que USA el techo tiene que publicarlo antes.
	for _, g := range ComandosGoTest(f) {
		if clase, _ := g.Techo(); clase != TechoDerivado {
			continue
		}
		var publicado bool
		for _, p := range pubs {
			if p.paso.Job == g.Paso.Job && p.paso.Indice < g.Paso.Indice {
				publicado = true
				break
			}
		}
		if !publicado {
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: usa $RACE_TIMEOUT y ningún "+
				"paso anterior del job lo publica en $GITHUB_ENV: llegaría vacío y `go test` "+
				"caería en su default de 10m",
				f.Ruta, g.Paso.Job, g.Paso.Nombre))
		}
	}
	return errs
}

// publicacionesDelPaso recorre el script del paso EN ORDEN y anota, para cada publicación, de
// dónde venía el valor en ESE punto. El orden es lo que decide: un `source` después del `echo`
// no lo alcanza, y una reasignación antes sí.
func publicacionesDelPaso(p *PasoCI) []publicacion {
	if strings.TrimSpace(p.Run) == "" {
		return nil
	}
	var out []publicacion
	var leyoLaPol bool
	var pisadoCon, fuenteOpaca string
	for _, t := range lexearShell(normalizarExpresionesGH(p.Run)) {
		escribe := laTuberiaEscribeAlEntornoDelJob(t)
		for _, c := range t.Cmds {
			if arch, ok := archivoDeSource(c); ok {
				if arch == NombreArchivoPolitica {
					leyoLaPol, pisadoCon, fuenteOpaca = true, "", ""
				} else {
					fuenteOpaca = arch
				}
			}
			for _, v := range asignacionesDelTecho(c) {
				if esReferenciaAlTecho(v) {
					continue // `RACE_TIMEOUT="$RACE_TIMEOUT"` no cambia nada
				}
				pisadoCon = v
			}
			if !escribe {
				continue
			}
			for _, v := range valoresPublicados(c) {
				out = append(out, publicacion{
					paso: p, valor: v.valor, via: v.via,
					leyoLaPol: leyoLaPol, pisadoCon: pisadoCon, fuenteOpaca: fuenteOpaca,
				})
			}
		}
	}
	return out
}

type valorPublicado struct{ valor, via string }

// reVerboPrintf es un verbo de formato de printf(1): `%s`, `%q`, `%-10s`…
var reVerboPrintf = regexp.MustCompile(`%[-+ #0-9.]*[a-zA-Z]`)

// valoresPublicados saca los RACE_TIMEOUT=… que este comando escribe, por argv o por heredoc.
func valoresPublicados(c Comando) []valorPublicado {
	var out []valorPublicado
	for i, a := range c.Argv {
		k, _, ok := strings.Cut(a, "=")
		if !ok || strings.TrimSpace(k) != nombreVarTecho {
			continue
		}
		bruto := c.Bruto[i]
		var comilla string
		if len(bruto) > 0 && (bruto[0] == '"' || bruto[0] == '\'') {
			comilla = string(bruto[0])
		}
		if _, vb, ok := strings.Cut(bruto, "="); ok {
			bruto = strings.TrimSuffix(vb, comilla)
		}
		valor := strings.TrimSuffix(strings.TrimSpace(bruto), `\n`)
		// Entre comillas SIMPLES el shell no expande: `'RACE_TIMEOUT=$RACE_TIMEOUT'` publica el
		// texto `$RACE_TIMEOUT`, no el techo. Se devuelven las comillas para que no se confunda
		// con una referencia y para que el mensaje muestre por qué.
		if comilla == "'" && strings.Contains(valor, "$") {
			valor = "'" + valor + "'"
		}
		// `printf 'RACE_TIMEOUT=%s\n' "$RACE_TIMEOUT"` publica el valor del ARGUMENTO, no el del
		// formato: es legítimo y hay que DERIVARLO. Rechazarlo sería un rojo sobre código sano,
		// y un rojo sobre código sano termina apagado.
		if v, ok := resolverVerboDePrintf(c, i, valor); ok {
			valor = v
		}
		out = append(out, valorPublicado{valor, "argv"})
	}
	for _, h := range c.Heredocs {
		for _, l := range strings.Split(h, "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(l), "=")
			if !ok || strings.TrimSpace(k) != nombreVarTecho {
				continue
			}
			out = append(out, valorPublicado{strings.TrimSpace(v), "cuerpo de heredoc"})
		}
	}
	return out
}

// resolverVerboDePrintf devuelve el argumento que llena el verbo del formato de un printf, si el
// valor publicado ES un verbo. Lo que decide es lo que sale, no lo que está escrito en el
// formato: `printf 'RACE_TIMEOUT=%s\n' 40m` publica un 40m tipeado y sigue siendo rojo.
func resolverVerboDePrintf(c Comando, iFormato int, valor string) (string, bool) {
	if len(c.Argv) == 0 || c.Argv[0] != "printf" || iFormato+1 >= len(c.Argv) {
		return "", false
	}
	v := strings.Trim(valor, `"'`)
	if !reVerboPrintf.MatchString(v) || reVerboPrintf.FindString(v) != v {
		return "", false
	}
	// Cuántos verbos hay ANTES de éste en el formato: ése es el desplazamiento del argumento.
	formato := c.Argv[iFormato]
	corte := strings.Index(formato, nombreVarTecho+"=")
	if corte < 0 {
		return "", false
	}
	antes := len(reVerboPrintf.FindAllString(strings.ReplaceAll(formato[:corte], "%%", ""), -1))
	i := iFormato + 1 + antes
	if i >= len(c.Argv) {
		return "", false
	}
	return strings.TrimSpace(c.Bruto[i]), true
}

var declaradoresDeVariable = map[string]bool{
	"export": true, "declare": true, "typeset": true, "readonly": true, "local": true,
}

// asignacionesDelTecho saca las asignaciones a RACE_TIMEOUT de un comando, y SÓLO ésas: un
// `echo "RACE_TIMEOUT=$X"` lleva el mismo texto y no asigna nada, así que sólo cuentan los
// tokens de PREFIJO (`VAR=v cmd`, o el comando entero cuando son todos asignaciones) y los que
// vienen detrás de un declarador.
func asignacionesDelTecho(c Comando) []string {
	var out []string
	desde := 0
	if len(c.Argv) > 0 && declaradoresDeVariable[c.Argv[0]] {
		desde = 1
		for i := desde; i < len(c.Argv); i++ {
			if v, ok := valorAsignadoAlTecho(c, i); ok {
				out = append(out, v)
			}
		}
		return out
	}
	for i := 0; i < len(c.Argv); i++ {
		k, _, ok := strings.Cut(c.Argv[i], "=")
		if !ok || !esNombreDeVariable(k) {
			break // ya no estamos en el prefijo de asignaciones
		}
		if v, ok := valorAsignadoAlTecho(c, i); ok {
			out = append(out, v)
		}
	}
	return out
}

func valorAsignadoAlTecho(c Comando, i int) (string, bool) {
	k, _, ok := strings.Cut(c.Argv[i], "=")
	if !ok || strings.TrimSpace(k) != nombreVarTecho {
		return "", false
	}
	_, vb, _ := strings.Cut(c.Bruto[i], "=")
	return strings.TrimSpace(vb), true
}

func esNombreDeVariable(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func archivoDeSource(c Comando) (string, bool) {
	if len(c.Argv) < 2 || (c.Argv[0] != "source" && c.Argv[0] != ".") {
		return "", false
	}
	return strings.TrimPrefix(strings.Trim(c.Argv[1], `"'`), "./"), true
}

// laTuberiaEscribeAlEntornoDelJob mira la TUBERÍA entera y no un comando suelto: en
// `echo … | tee -a "$GITHUB_ENV"` el que lleva el valor y el que escribe son comandos distintos.
func laTuberiaEscribeAlEntornoDelJob(t Tuberia) bool {
	for _, c := range t.Cmds {
		if redirigeAlEntornoDelJob(c) {
			return true
		}
		if len(c.Argv) > 0 && c.Argv[0] == "tee" {
			for _, a := range c.Argv[1:] {
				if esGithubEnv(a) {
					return true
				}
			}
		}
	}
	return false
}

func esGithubEnv(a string) bool {
	a = strings.Trim(strings.TrimSpace(a), `"'`)
	return a == "$GITHUB_ENV" || a == "${GITHUB_ENV}" || a == "${{env.GITHUB_ENV}}"
}

func redirigeAlEntornoDelJob(c Comando) bool {
	for i, a := range c.Argv {
		if (a == ">>" || a == ">") && i+1 < len(c.Argv) && esGithubEnv(c.Argv[i+1]) {
			return true
		}
	}
	return false
}

// RutaDelComandoDelGuard deriva la ruta con la que el ci.yml invoca al guard del margen.
func RutaDelComandoDelGuard(modulo string) string {
	return "." + strings.TrimPrefix(ImportDelGuard, modulo) + "/cmd/presupuesto"
}

// GoRun es un `go run <paquete>` del workflow, con la tubería donde vive: sin ella no se puede
// decir si su código de salida llega a algún lado.
type GoRun struct {
	Paso    *PasoCI
	Argv    []string
	Tuberia Tuberia
	EnCmd   int // posición del comando dentro de la tubería
}

// ErroresDelPasoDePresupuesto exige que el aparato SE EJECUTE Y PUEDA PONER ROJO EL JOB.
//
// El sabotaje original era borrar del ci.yml el paso «Presupuesto de pruebas (margen medido)»:
// todo el código seguía en el repo, con sus subtests en verde, y no corría nunca.
//
// No se ancla al NOMBRE del paso —un nombre se cambia sin cambiar nada— sino a la cadena que
// decide: la suite corre bajo -race sobre los paquetes caros, guarda su salida con `tee`, y
// algún paso POSTERIOR del MISMO job le pasa ESE archivo al comando del guard.
//
// Y «que corra» son TRES cosas, no una:
//
//  1. que el paso no esté desactivado — ni por `continue-on-error:`, ni por un `if:` que no se
//     cumple. Y no sólo el del paso: el `if:` y el `continue-on-error:` DEL JOB apagan igual.
//  2. que eso valga también para el paso que PRODUCE el dato. Un guard que corre sobre un log
//     que nadie escribió no mide nada, y ese paso no lo miraba nadie.
//  3. que el código de salida del guard llegue al paso: un `|| true` detrás, un `&` que lo manda
//     al fondo, o una tubería sin pipefail se lo comen enteros, y el comando sigue estando ahí
//     para cualquier chequeo que mire si está escrito.
func ErroresDelPasoDePresupuesto(f *FlujoCI, modulo string, caros []string) []error {
	var errs []error
	var suites []GoTest
	for _, g := range ComandosGoTest(f) {
		if _, hayRace := g.Flags["race"]; !hayRace || !g.CorreTests() {
			continue
		}
		for _, c := range caros {
			if g.Cubre(modulo, c) {
				suites = append(suites, g)
				break
			}
		}
	}
	if len(suites) == 0 {
		return []error{fmt.Errorf("%s no corre NI UNA vez los paquetes caros bajo `-race`: no hay "+
			"corrida que medir y el presupuesto quedó sin vigilar", f.Ruta)}
	}

	rutaCmd := RutaDelComandoDelGuard(modulo)
	for _, s := range suites {
		errs = append(errs, erroresDePasoApagado(f, s.Paso, "el paso que corre la suite bajo -race y produce el log que se mide")...)
		log, ok := destinoDeTee(s.Tuberia)
		if !ok {
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: la corrida bajo -race no guarda "+
				"su salida (`| tee <archivo>`), así que no queda de dónde medir el margen",
				f.Ruta, s.Paso.Job, s.Paso.Nombre))
			continue
		}
		var juez *GoRun
		for _, r := range comandosGoRun(f, rutaCmd) {
			if r.Paso.Job != s.Paso.Job || r.Paso.Indice <= s.Paso.Indice {
				continue
			}
			if v, ok := valorDeFlag(r.Argv, "salida"); ok && v == log {
				rr := r
				juez = &rr
				break
			}
		}
		if juez == nil {
			errs = append(errs, fmt.Errorf("%s · job %q: después del paso %q no hay ningún "+
				"`go run %s -salida %s`. El guard del margen está entero en el repo y no lo "+
				"ejecuta nadie: CI no puede ponerse rojo cuando el presupuesto se come",
				f.Ruta, s.Paso.Job, s.Paso.Nombre, rutaCmd, log))
			continue
		}
		errs = append(errs, erroresDePasoApagado(f, juez.Paso, "el paso del guard del margen")...)
		errs = append(errs, erroresDeCodigoDeSalida(f, *juez)...)
	}
	return errs
}

// erroresDePasoApagado dice si un paso NO va a correr —o si no se puede DEMOSTRAR que corra— por
// `continue-on-error:` o por `if:`, en el paso o en el job que lo contiene.
func erroresDePasoApagado(f *FlujoCI, p *PasoCI, papel string) []error {
	var errs []error
	if c := strings.ToLower(strings.TrimSpace(p.ContinuarConError)); c != "" && c != "false" {
		errs = append(errs, fmt.Errorf("%s · job %q · paso %q: %s lleva continue-on-error: %s, "+
			"así que su rojo no falla el job", f.Ruta, p.Job, p.Nombre, papel, p.ContinuarConError))
	}
	if c := strings.ToLower(strings.TrimSpace(p.ContinuarConErrorDelJob)); c != "" && c != "false" {
		errs = append(errs, fmt.Errorf("%s · job %q: el JOB que contiene %s lleva "+
			"continue-on-error: %s, así que ningún rojo de adentro falla la corrida",
			f.Ruta, p.Job, papel, p.ContinuarConErrorDelJob))
	}
	if ok, motivo := AnalizarSi(p.Si, f.Eventos); !ok {
		errs = append(errs, fmt.Errorf("%s · job %q · paso %q: %s — %s",
			f.Ruta, p.Job, p.Nombre, papel, motivo))
	}
	if ok, motivo := AnalizarSi(p.SiDelJob, f.Eventos); !ok {
		errs = append(errs, fmt.Errorf("%s · job %q: el `if:` del job que contiene %s — %s",
			f.Ruta, p.Job, papel, motivo))
	}
	return errs
}

// erroresDeCodigoDeSalida deriva si un rojo del guard llega al paso, en vez de preguntar por las
// formas conocidas de tragárselo.
//
// El estado del shell se lleva EN ORDEN (`set -e`, `set +e`, `set -o pipefail`, `set -euo
// pipefail`) porque es lo que decide, y el punto de partida sale del `shell:` del paso: GitHub
// corre `bash --noprofile --norc -eo pipefail {0}` para `shell: bash`, y `bash -e {0}` para el
// default. Un shell que no sé cómo trata un código de salida es ROJO, no verde.
func erroresDeCodigoDeSalida(f *FlujoCI, r GoRun) []error {
	p := r.Paso
	var errexit, pipefail bool
	switch strings.ToLower(strings.TrimSpace(p.Shell)) {
	case "":
		errexit, pipefail = true, false // `bash -e {0}`
	case "bash":
		errexit, pipefail = true, true // `bash --noprofile --norc -eo pipefail {0}`
	case "sh":
		errexit, pipefail = true, false // `sh -e {0}`
	default:
		return []error{fmt.Errorf("%s · job %q · paso %q: el paso del guard corre con `shell: %s` "+
			"y no sé cómo trata ese shell el código de salida del comando, así que no puedo "+
			"demostrar que un rojo del guard falle el job. Usá `shell: bash` o enseñale el shell "+
			"a erroresDeCodigoDeSalida", f.Ruta, p.Job, p.Nombre, p.Shell)}
	}

	var ts []Tuberia
	pos := -1
	for _, t := range f.Tuberias {
		if t.Paso != p {
			continue
		}
		if t.Indice == r.Tuberia.Indice {
			pos = len(ts)
		}
		ts = append(ts, t)
	}
	if pos < 0 {
		return nil
	}
	for i := 0; i < pos; i++ {
		aplicarSet(ts[i], &errexit, &pipefail)
	}
	g := ts[pos]

	var errs []error
	switch g.Separador {
	case "||":
		// `cmd || X` sólo se come el rojo si X NO falla. `|| exit 1` lo propaga y es legítimo;
		// `|| true` lo borra. La diferencia se deriva de X, no de la presencia del `||`.
		if !laSiguienteFalla(ts, pos) {
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: detrás del guard hay un `||` "+
				"cuyo lado derecho no falla, así que el rojo del guard no llega al paso y el "+
				"comando queda de adorno.\n    comando: %s", f.Ruta, p.Job, p.Nombre, g.Texto))
		}
	case "&":
		errs = append(errs, fmt.Errorf("%s · job %q · paso %q: el guard queda en segundo plano "+
			"(`&`): el paso no espera su resultado ni ve su código de salida.\n    comando: %s",
			f.Ruta, p.Job, p.Nombre, g.Texto))
	}
	if r.EnCmd != len(g.Cmds)-1 && !pipefail {
		errs = append(errs, fmt.Errorf("%s · job %q · paso %q: el guard está en el medio de una "+
			"tubería y este shell no tiene `pipefail`, así que el código que ve el paso es el del "+
			"ÚLTIMO comando y el rojo del guard se pierde.\n    comando: %s",
			f.Ruta, p.Job, p.Nombre, g.Texto))
	}
	if !errexit && !g.UltimaDelP && !elCodigoSeCapturaYSePropaga(ts, pos) {
		errs = append(errs, fmt.Errorf("%s · job %q · paso %q: cuando corre el guard hay un "+
			"`set +e` activo, quedan comandos después, y no encuentro que su código se capture "+
			"(`rc=$?`) y se propague (`exit \"$rc\"`): el código de salida del paso lo define el "+
			"último comando y el rojo del guard no llega a ningún lado", f.Ruta, p.Job, p.Nombre))
	}
	return errs
}

// laSiguienteFalla dice si la tubería que sigue a un `||` termina fallando (`exit N`, `false`).
func laSiguienteFalla(ts []Tuberia, pos int) bool {
	if pos+1 >= len(ts) || len(ts[pos+1].Cmds) == 0 {
		return false
	}
	argv := ts[pos+1].Cmds[0].Argv
	if len(argv) == 0 {
		return false
	}
	switch argv[0] {
	case "false":
		return true
	case "exit":
		return len(argv) < 2 || strings.Trim(argv[1], `"'`) != "0"
	}
	return false
}

// elCodigoSeCapturaYSePropaga reconoce el patrón que este repo ya usa en el paso de la suite:
// `set +e` / comando / `rc=$?` (o ${PIPESTATUS[0]}) / `set -e` / `exit "$rc"`. Es legítimo y da
// verde; lo que no da verde es que el código se pierda.
func elCodigoSeCapturaYSePropaga(ts []Tuberia, pos int) bool {
	if pos+1 >= len(ts) {
		return false
	}
	var variable string
	for _, c := range ts[pos+1].Cmds {
		for _, a := range c.Argv {
			k, v, ok := strings.Cut(a, "=")
			if !ok || !esNombreDeVariable(k) {
				continue
			}
			if strings.Contains(v, "$?") || strings.Contains(v, "PIPESTATUS") {
				variable = k
			}
		}
	}
	if variable == "" {
		return false
	}
	for i := pos + 2; i < len(ts); i++ {
		for _, c := range ts[i].Cmds {
			if len(c.Argv) < 2 || c.Argv[0] != "exit" {
				continue
			}
			if strings.Contains(c.Argv[1], "$"+variable) || strings.Contains(c.Argv[1], "${"+variable) {
				return true
			}
		}
	}
	return false
}

// aplicarSet actualiza errexit/pipefail con los `set` de una tubería.
func aplicarSet(t Tuberia, errexit, pipefail *bool) {
	for _, c := range t.Cmds {
		if len(c.Argv) == 0 || c.Argv[0] != "set" {
			continue
		}
		for i := 1; i < len(c.Argv); i++ {
			a := c.Argv[i]
			if len(a) < 2 || (a[0] != '-' && a[0] != '+') {
				continue
			}
			enciende := a[0] == '-'
			for k := 1; k < len(a); k++ {
				switch a[k] {
				case 'e':
					*errexit = enciende
				case 'o':
					if i+1 < len(c.Argv) {
						switch c.Argv[i+1] {
						case "pipefail":
							*pipefail = enciende
						case "errexit":
							*errexit = enciende
						}
						i++
					}
				}
			}
		}
	}
}

func comandosGoRun(f *FlujoCI, paquete string) []GoRun {
	var out []GoRun
	for _, t := range f.Tuberias {
		for ci, c := range t.Cmds {
			if len(c.Argv) < 3 || c.Argv[0] != "go" || c.Argv[1] != "run" {
				continue
			}
			for _, a := range c.Argv[2:] {
				if strings.TrimSuffix(a, "/") == strings.TrimSuffix(paquete, "/") {
					out = append(out, GoRun{Paso: t.Paso, Argv: c.Argv, Tuberia: t, EnCmd: ci})
					break
				}
			}
		}
	}
	return out
}

func destinoDeTee(t Tuberia) (string, bool) {
	for _, c := range t.Cmds[1:] {
		if len(c.Argv) == 0 || c.Argv[0] != "tee" {
			continue
		}
		for _, a := range c.Argv[1:] {
			if !strings.HasPrefix(a, "-") {
				return a, true
			}
		}
	}
	return "", false
}

// valorDeFlag saca el valor de un flag de un argv cualquiera, en sus dos formas.
func valorDeFlag(argv []string, nombre string) (string, bool) {
	for i, a := range argv {
		if !strings.HasPrefix(a, "-") {
			continue
		}
		n := strings.TrimLeft(a, "-")
		if k, v, ok := strings.Cut(n, "="); ok {
			if k == nombre {
				return v, true
			}
			continue
		}
		if n == nombre && i+1 < len(argv) {
			return argv[i+1], true
		}
	}
	return "", false
}

// ErroresDeTechoEnPaquetesCaros es el subconjunto del juicio que vale para CUALQUIER workflow,
// no sólo para ci.yml: el que corra los TESTS de un paquete caro tiene que tomar el techo de la
// política.
//
// EL HERMANO DEL ci.yml. La guarda del techo se ancla a ci.yml porque ahí es donde el número
// estaba tipeado, pero .github/workflows tiene más archivos y nada impedía que apareciera otro
// `go test ./...` sin techo en el de al lado. La excepción de bench-scale.yml sigue siendo
// legítima y sale sola de la forma que decide: sus comandos son `-run=NONE -bench=…`, o sea que
// no corren los tests del paquete y su techo mide otra cosa.
func ErroresDeTechoEnPaquetesCaros(f *FlujoCI, modulo string, caros []string) []error {
	var errs []error
	for _, g := range ComandosGoTest(f) {
		if !g.CorreTests() {
			continue
		}
		var caro string
		for _, c := range caros {
			if g.Cubre(modulo, c) {
				caro = c
				break
			}
		}
		if caro == "" {
			continue
		}
		clase, valor := g.Techo()
		if clase == TechoDerivado {
			continue
		}
		comoEsta := "sin -timeout, o sea con el default de Go de 10m POR PAQUETE"
		if clase == TechoTipeado {
			comoEsta = "con el techo tipeado a mano (`-timeout " + valor + "`)"
		}
		errs = append(errs, fmt.Errorf("%s · job %q · paso %q: corre los tests de %s %s. "+
			"El techo tiene que salir de %s.\n    comando: %s",
			f.Ruta, g.Paso.Job, g.Paso.Nombre, caro, comoEsta, NombreArchivoPolitica, g.Tuberia.Texto))
	}
	return errs
}
