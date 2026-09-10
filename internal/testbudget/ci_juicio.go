package testbudget

// EL JUICIO SOBRE EL ci.yml.
//
// Está en código de producción y no en el _test.go a propósito: así los mismos jueces se pueden
// correr contra workflows SINTÉTICOS —cada sabotaje, uno por uno, con su forma exacta— y no sólo
// contra el archivo real del repo. Una guarda que sólo se ejerce contra el archivo bueno nunca
// se vio a sí misma en rojo.

import (
	"fmt"
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

// publicacion es un `RACE_TIMEOUT=… >> $GITHUB_ENV` encontrado en el workflow.
type publicacion struct {
	paso  *PasoCI
	valor string // el lado derecho, tal como estaba escrito
}

// ErroresDePublicacion juzga el nivel de adentro: que el valor que se publica en $GITHUB_ENV
// salga del `source` del archivo de política y no esté tipeado ahí.
//
// El sabotaje que cierra dejaba el `source` INTACTO —así que cualquier chequeo de presencia o de
// conteo de líneas seguía contento— y publicaba el número a mano en el entorno del job. Lo que
// rige no es que el `source` esté escrito: es qué valor viaja.
func ErroresDePublicacion(f *FlujoCI) []error {
	var errs []error
	var pubs []publicacion
	for _, t := range f.Tuberias {
		for _, c := range t.Cmds {
			if !redirigeAlEntornoDelJob(c) {
				continue
			}
			for i, a := range c.Argv {
				k, _, ok := strings.Cut(a, "=")
				if !ok || strings.TrimSpace(k) != "RACE_TIMEOUT" {
					continue
				}
				bruto := c.Bruto[i]
				if _, vb, ok := strings.Cut(bruto, "="); ok {
					bruto = strings.TrimSuffix(vb, `"`)
				}
				pubs = append(pubs, publicacion{paso: t.Paso, valor: strings.TrimSpace(bruto)})
			}
		}
	}
	for _, p := range pubs {
		if !referenciasAlTecho[p.valor] && !referenciasAlTecho[`"`+p.valor+`"`] {
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: publica RACE_TIMEOUT=%s en "+
				"$GITHUB_ENV y ese valor no sale de %s. El `source` puede seguir escrito ahí "+
				"arriba: lo que rige es el valor que viaja",
				f.Ruta, p.paso.Job, p.paso.Nombre, p.valor, NombreArchivoPolitica))
			continue
		}
		if !elPasoLeeLaPolitica(p.paso) {
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: publica RACE_TIMEOUT en "+
				"$GITHUB_ENV sin hacer `source ./%s` en el mismo paso, así que la variable llega "+
				"vacía y `go test` cae en su default de 10m",
				f.Ruta, p.paso.Job, p.paso.Nombre, NombreArchivoPolitica))
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

func redirigeAlEntornoDelJob(c Comando) bool {
	for i, a := range c.Argv {
		if (a == ">>" || a == ">") && i+1 < len(c.Argv) {
			d := strings.Trim(c.Argv[i+1], `"`)
			if d == "$GITHUB_ENV" || d == "${GITHUB_ENV}" {
				return true
			}
		}
	}
	return false
}

func elPasoLeeLaPolitica(p *PasoCI) bool {
	for _, t := range lexearShell(normalizarExpresionesGH(p.Run)) {
		for _, c := range t.Cmds {
			if len(c.Argv) < 2 || (c.Argv[0] != "source" && c.Argv[0] != ".") {
				continue
			}
			if strings.TrimPrefix(c.Argv[1], "./") == NombreArchivoPolitica {
				return true
			}
		}
	}
	return false
}

// RutaDelComandoDelGuard deriva la ruta con la que el ci.yml invoca al guard del margen.
func RutaDelComandoDelGuard(modulo string) string {
	return "." + strings.TrimPrefix(ImportDelGuard, modulo) + "/cmd/presupuesto"
}

// GoRun es un `go run <paquete>` del workflow.
type GoRun struct {
	Paso *PasoCI
	Argv []string
}

// ErroresDelPasoDePresupuesto exige que el aparato SE EJECUTE.
//
// El sabotaje que cierra era borrar del ci.yml el paso «Presupuesto de pruebas (margen medido)»:
// todo el código seguía en el repo, con sus trece subtests en verde, y no corría nunca. Nada
// pedía que ese paso existiera.
//
// No se ancla al NOMBRE del paso —un nombre se cambia sin cambiar nada— sino a la cadena que
// decide: la suite corre bajo -race sobre los paquetes caros, guarda su salida con `tee`, y
// algún paso POSTERIOR del MISMO job le pasa ESE archivo al comando del guard, sin estar
// desactivado.
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
		if c := strings.ToLower(juez.Paso.ContinuarConError); c != "" && c != "false" {
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: el paso del guard lleva "+
				"continue-on-error: %s, así que su rojo no falla el job",
				f.Ruta, juez.Paso.Job, juez.Paso.Nombre, juez.Paso.ContinuarConError))
		}
		if si := strings.ToLower(strings.TrimSpace(juez.Paso.Si)); si == "false" || si == "${{false}}" {
			errs = append(errs, fmt.Errorf("%s · job %q · paso %q: el paso del guard está apagado "+
				"con `if: %s`", f.Ruta, juez.Paso.Job, juez.Paso.Nombre, juez.Paso.Si))
		}
	}
	return errs
}

func comandosGoRun(f *FlujoCI, paquete string) []GoRun {
	var out []GoRun
	for _, t := range f.Tuberias {
		for _, c := range t.Cmds {
			if len(c.Argv) < 3 || c.Argv[0] != "go" || c.Argv[1] != "run" {
				continue
			}
			for _, a := range c.Argv[2:] {
				if strings.TrimSuffix(a, "/") == strings.TrimSuffix(paquete, "/") {
					out = append(out, GoRun{Paso: t.Paso, Argv: c.Argv})
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
