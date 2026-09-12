package codeintel

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deriver_motor_test.go: LA GUARDA DE LA PRECONDICIÓN DE GraphDeriverVersion.
//
// QUÉ CUIDA. El índice incremental (internal/mcp/methods_codegraph.go:479-482) salta todo archivo
// cuyo src_fingerprint —sha256 del CONTENIDO, internal/memory/codepath.go:31— coincida con el del
// disco. La ÚNICA puerta que puede re-derivar un archivo que nadie tocó es que GraphDeriverVersion
// deje de coincidir con el sello guardado. Y esa constante se sube A MANO.
//
// El agujero: el derivador polyglot no es nuestro. treesit_on.go:79 llama
// `ts.NewParser(lang).Parse(src)` y le pide los spans a gotreesitter. Si sube la dependencia y
// nadie sube la constante, el motor nuevo se aplica SÓLO a los archivos que alguien edite; los
// demás quedan con el grafo del motor viejo para siempre, y nada lo declara.
//
// POR QUÉ go.mod Y NO debug.ReadBuildInfo(). MEDIDO en este mismo paquete el 2026-09-12: un binario
// de `go test` devuelve `ok=true` con `len(bi.Deps) == 0`. ReadBuildInfo no sirve como fuente de un
// test: no hay deps que leer. (Y aunque las hubiera, gotreesitter sólo se linkea con el build tag
// `treesitter`, que el CI por default no pone: la guarda quedaría muda justo donde tiene que hablar.)
// go.mod, en cambio, es un archivo del repo: se lee sin compilar nada y sin build tags.
//
// POR QUÉ NO ES UN ESPEJO. El hecho del mundo sale de go.mod —el archivo que edita Dependabot— y lo
// custodiado es motorTreeSitterDeclarado, en crosspkg.go. Dos archivos, dos motivos de edición
// distintos. La guarda NO clava ninguna versión en ningún lado: si lo hiciera habría que
// actualizarla a mano y no custodiaría nada.
//
// CÓMO SE SABOTEA ESTA GUARDA, Y POR QUÉ ALCANZA CON UN SOLO ARCHIVO. Medido el 2026-09-12, porque
// la respuesta intuitiva es la equivocada y cuesta tiempo: para verla ponerse roja alcanza con
// editar LA LÍNEA DE VERSIÓN DE go.mod. NO hace falta tocar go.sum, aunque quede desincronizado.
//
// El motivo es del repo y no del sabotaje: SIN BUILD TAGS, internal/codeintel no importa
// gotreesitter —lo reemplaza treesit_off.go—, así que Go nunca carga ese módulo y nunca verifica su
// entrada en go.sum. Comprobado dejando go.mod en v0.51.0 con go.sum conteniendo SÓLO las entradas
// de v0.52.0: `go build ./internal/codeintel/` compila y la guarda da rojo por SU PROPIA aserción.
//
// LA SALVEDAD, Y ES LA QUE IMPORTA SI AUTOMATIZÁS ESTO: con los tags de tree-sitter, ese mismo
// árbol NO COMPILA — `missing go.sum entry for module providing package
// github.com/odvcencio/gotreesitter`. O sea que el sabotaje de un archivo es válido para ESTA
// guarda, que corre sin tags en el job `test`, pero deja el árbol roto para el paso polyglot. Si
// después de sabotear corrés algo con tags, el rojo que vas a ver es de BUILD y no prueba nada
// (ver deploy/pruebas/sabotaje.sh, que compila antes y después justamente por esto). Restaurá
// go.mod antes de seguir, o sabotéalo con `go mod tidy` si vas a tocar el árbol con tags.
//
// Esa es también la propiedad rara de esta guarda: su sabotaje es EXACTAMENTE la edición que hace
// Dependabot. Prueba el caso real, no uno inventado.

const moduloMotorTreeSitter = "github.com/odvcencio/gotreesitter"

// TestGraphDeriverVersionAcusaElMotorDeTreeSitter es la guarda. Roja cuando go.mod trae un motor que
// la constante no declara.
//
// MECANIZADA. El sabotaje es que la constante deje de acusar al go.mod. El arreglo es subir el
// número de revisión del sello, que es LITERALMENTE lo que el mensaje de error de esta prueba
// manda hacer: si la guarda castigara su propia instrucción, estaría premiando el defecto.
// Medidas las dos el 2026-09-12.
// Sabotaje que la pone roja: que `motorTreeSitterDeclarado` deje de acusar al `require` del go.mod.
// arnes: archivo="internal/codeintel/crosspkg.go"
// arnes: de="const motorTreeSitterDeclarado = \"v0.52.0\""
// arnes: a="const motorTreeSitterDeclarado = \"v0.51.0\""
// arnes: arreglo_de="const GraphDeriverVersion = \"5-crosspkg+ts-\""
// arnes: arreglo_a="const GraphDeriverVersion = \"6-crosspkg+ts-\""
func TestGraphDeriverVersionAcusaElMotorDeTreeSitter(t *testing.T) {
	raiz := raizDelModulo(t)
	datos, err := os.ReadFile(filepath.Join(raiz, "go.mod"))
	if err != nil {
		t.Fatalf("no pude leer go.mod en %s: %v", raiz, err)
	}
	// EL go.work TIENE PRECEDENCIA SOBRE EL go.mod Y ES LA FORMA MÁS COMÚN DE APUNTAR A UN FORK.
	// Medido el 2026-09-12: con un `replace` en go.work, `go list -m` contesta
	// `gotreesitter v0.52.0 => /tmp/.../forkts`, el build compila ESE código, y esta guarda —que
	// sólo leía go.mod— daba VERDE. El sello afirmaba acusar el motor y el motor era un directorio
	// local que nadie cruzó contra nada.
	//
	// No se intenta resolver qué versión es el fork: no hay forma honesta de saberlo desde un
	// archivo. Se FALLA CERRADA, que es la única respuesta correcta cuando la pregunta que la
	// guarda existe para contestar dejó de tener respuesta.
	if ruta, hay := trabajoQueDesviaElModulo(raiz, moduloMotorTreeSitter); hay {
		t.Fatalf(`HAY UN go.work QUE DESVÍA %s Y ESTA GUARDA NO PUEDE ACUSAR NADA.
  archivo: %s

El `+"`replace`"+` de un go.work tiene precedencia sobre el go.mod, así que el motor que se compila no es
el que declara el `+"`require`"+`, y el sello del derivador estaría mintiendo sobre qué derivó el grafo.

Si es un fork de trabajo local: sacá el go.work antes de indexar o de correr la suite. Si el fork
tiene que ser permanente, el sello tiene que llevar su identidad y no la del require.`,
			moduloMotorTreeSitter, ruta)
	}

	enElMundo, err := versionRequerida(string(datos), moduloMotorTreeSitter)
	if err != nil {
		// «No sé» NO es verde. Si el módulo desapareció, o hay un replace, la constante ya no puede
		// estar acusando nada verificable.
		t.Fatalf("no pude determinar la versión de %s desde %s/go.mod: %v",
			moduloMotorTreeSitter, raiz, err)
	}
	if motorTreeSitterDeclarado != enElMundo {
		t.Errorf(`GraphDeriverVersion no acusa el motor que el repo realmente usa.
  go.mod requiere : %s
  crosspkg.go declara: %s

QUÉ HACER: en internal/codeintel/crosspkg.go, poné motorTreeSitterDeclarado = %q y subí el número
de revisión de GraphDeriverVersion (%q → "4-..."). Eso hace que el próximo índice incremental
re-derive TODO una vez; sin eso, los archivos que nadie edite se quedan con el grafo del motor
viejo PARA SIEMPRE (el src_fingerprint es del contenido, no del derivador).`,
			enElMundo, motorTreeSitterDeclarado, enElMundo, GraphDeriverVersion)
	}
	// TRIPWIRE DE APLANADO. Pregunta por el FUENTE y no por el valor, y el motivo es que la
	// versión anterior —`strings.Contains(GraphDeriverVersion, motorTreeSitterDeclarado)`— estaba
	// en VERDE sobre exactamente el aplanado que decía cazar. Medido el 2026-09-12: reescrito el
	// sello como `const GraphDeriverVersion = "5-crosspkg+ts-v0.52.0+poly-on"`, esta prueba pasaba
	// y la del tag también, con tags; el literal CONTIENE el texto de la constante, porque salió
	// de concatenarla. `Contains` acierta por la misma razón por la que el aplanado es invisible.
	//
	// LO QUE DECIDE NO ES EL TEXTO SINO LA DERIVACIÓN: que cambiar la constante cambie el sello.
	// Eso es una propiedad de la DECLARACIÓN, no del valor —un `const` de Go no puede variar en
	// runtime, así que no hay forma honesta de preguntarlo desde el valor— y se lee del archivo
	// que la decide, con el mismo parser que la compila.
	ids := identificadoresDelSello(t)
	for _, quiero := range []string{"motorTreeSitterDeclarado", "motorPolyglot"} {
		if !contieneCadena(ids, quiero) {
			t.Errorf(`GraphDeriverVersion NO SE DERIVA de %s: el sello está aplanado.
  declaración en crosspkg.go referencia: %v
  valor actual                         : %q

Aplanado, el sello queda CONGELADO: subir %s no cambia el valor que se compara contra el sello
guardado, así que el próximo índice incremental no re-deriva nada y los archivos que nadie edite
se quedan con el grafo del motor viejo para siempre.

No alcanza con que el valor CONTENGA el texto de la constante —lo contiene igual estando aplanado,
porque salió de concatenarla—: tiene que REFERENCIARLA.`,
				quiero, ids, GraphDeriverVersion, quiero)
		}
	}
}

// identificadoresDelSello devuelve los identificadores que la declaración de GraphDeriverVersion
// REFERENCIA en crosspkg.go. Lee el fuente porque la propiedad que interesa —que el sello derive
// de las constantes del motor— vive en la declaración y no en el valor.
//
// `go test` corre con el cwd en el directorio del paquete, así que el archivo está al lado.
func identificadoresDelSello(t *testing.T) []string {
	t.Helper()
	datos, err := os.ReadFile("crosspkg.go")
	if err != nil {
		t.Fatalf("no pude leer crosspkg.go desde el paquete: %v", err)
	}
	ids, err := identificadoresDelSelloEn(string(datos))
	if err != nil {
		t.Fatalf("crosspkg.go: %v", err)
	}
	return ids
}

// identificadoresDelSelloEn es la unidad, separada del archivo a propósito: la decisión que hay que
// poder probar es «¿esta declaración deriva o está aplanada?», y probarla con el archivo real sólo
// mide el caso sano. Con la unidad se le puede dar un aplanado y ver que lo distingue —que es
// justo lo que el tripwire anterior no hacía.
func identificadoresDelSelloEn(fuente string) ([]string, error) {
	archivo, err := parser.ParseFile(token.NewFileSet(), "crosspkg.go", fuente, 0)
	if err != nil {
		return nil, fmt.Errorf("no parseó: %w", err)
	}
	for _, decl := range archivo.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, esp := range gen.Specs {
			val, ok := esp.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, nombre := range val.Names {
				if nombre.Name != "GraphDeriverVersion" || i >= len(val.Values) {
					continue
				}
				ids := []string{}
				ast.Inspect(val.Values[i], func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok {
						ids = append(ids, id.Name)
					}
					return true
				})
				return ids, nil
			}
		}
	}
	// «No lo encontré» NO es «está bien». Si la declaración cambió de forma, este tripwire dejó de
	// tener dónde apoyarse y hay que enterarse, no seguir en verde.
	return nil, fmt.Errorf("no encontré una declaración `const GraphDeriverVersion = ...` con valor")
}

func contieneCadena(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// raizDelModulo sube desde el directorio del test hasta encontrar go.mod. `go test` corre con el
// cwd en el directorio del paquete, así que esto funciona igual en el árbol principal y en un
// worktree, sin clavar ninguna ruta.
func raizDelModulo(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			t.Fatalf("no encontré go.mod subiendo desde el paquete")
		}
		dir = padre
	}
}

// versionRequerida saca del texto de un go.mod la versión REQUERIDA de un módulo.
//
// Devuelve error —nunca "" en silencio— cuando no puede contestar: módulo ausente, o un `replace`
// que lo desvía (en ese caso el `require` ya no dice qué código se compila, así que contestar sería
// inventar). Un cero que significa «no sé» es el modo de falla que esta función se prohíbe.
func versionRequerida(goMod, modulo string) (string, error) {
	enRequire, enReplace := false, false
	var version string
	for _, cruda := range strings.Split(goMod, "\n") {
		linea := cruda
		if i := strings.Index(linea, "//"); i >= 0 {
			linea = linea[:i]
		}
		linea = strings.TrimSpace(linea)
		if linea == "" {
			continue
		}
		if linea == ")" {
			enRequire, enReplace = false, false
			continue
		}
		campos := strings.Fields(linea)
		// GO ACEPTA LA RUTA DEL MÓDULO ENTRE COMILLAS, y sin esto la guarda no la reconoce.
		// Medido el 2026-09-12: `replace "github.com/odvcencio/gotreesitter" => …` es un go.mod
		// VÁLIDO (`go mod edit -json` lo parsea, `go build` sale 0) y el `campos[0] == modulo` de
		// abajo daba falso, así que el `replace` se colaba por el `continue` y la función devolvía
		// tranquila la versión del `require` — la guarda en VERDE con el motor apuntando a un fork.
		// Vale igual para `require "…" v1.2.3`, que daba un errAusente con diagnóstico falso.
		for i := range campos {
			campos[i] = strings.Trim(campos[i], `"`)
		}
		switch {
		case campos[0] == "require" && len(campos) == 2 && campos[1] == "(":
			enRequire = true
			continue
		case campos[0] == "replace" && len(campos) == 2 && campos[1] == "(":
			enReplace = true
			continue
		case campos[0] == "require":
			campos = campos[1:]
		case campos[0] == "replace":
			campos = campos[1:]
			if len(campos) > 0 && campos[0] == modulo {
				return "", errReplace
			}
			continue
		case campos[0] == "exclude" || campos[0] == "module" || campos[0] == "go" ||
			campos[0] == "toolchain" || campos[0] == "retract":
			continue
		case enReplace:
			if campos[0] == modulo {
				return "", errReplace
			}
			continue
		case !enRequire:
			continue
		}
		if len(campos) >= 2 && campos[0] == modulo {
			version = campos[1]
		}
	}
	if version == "" {
		return "", errAusente
	}
	return version, nil
}

type errGoMod string

func (e errGoMod) Error() string { return string(e) }

const (
	errAusente = errGoMod("el módulo no aparece en ningún require de go.mod")
	errReplace = errGoMod("hay un `replace` para el módulo: el require ya no dice qué código se compila")
)

// TestVersionRequeridaParsea fija el PARSER contra las formas que go.mod realmente toma. Es un test
// del instrumento, no la guarda: la guarda de arriba come el go.mod REAL del repo.
func TestVersionRequeridaParsea(t *testing.T) {
	const m = "github.com/odvcencio/gotreesitter"
	casos := []struct {
		nombre, goMod, quiero, quieroErr string
	}{
		{"bloque", "module musubi\n\ngo 1.25\n\nrequire (\n\tgithub.com/a/b v1.0.0\n\t" + m + " v0.51.0\n)\n", "v0.51.0", ""},
		{"linea suelta", "module musubi\n\nrequire " + m + " v0.51.0\n", "v0.51.0", ""},
		{"indirect", "require (\n\t" + m + " v0.52.0 // indirect\n)\n", "v0.52.0", ""},
		{"comentado", "require (\n\t// " + m + " v9.9.9\n\t" + m + " v0.51.0\n)\n", "v0.51.0", ""},
		{"ausente", "require (\n\tgithub.com/a/b v1.0.0\n)\n", "", errAusente.Error()},
		{"replace suelto", "require (\n\t" + m + " v0.51.0\n)\n\nreplace " + m + " => ../local\n", "", errReplace.Error()},
		{"replace en bloque", "require (\n\t" + m + " v0.51.0\n)\n\nreplace (\n\t" + m + " => ../local\n)\n", "", errReplace.Error()},
		{"exclude no confunde", "exclude " + m + " v0.49.0\n\nrequire (\n\t" + m + " v0.51.0\n)\n", "v0.51.0", ""},
		// GO ACEPTA LA RUTA ENTRE COMILLAS EN LAS TRES POSICIONES, y las tres se colaban.
		// Medido el 2026-09-12: `replace "…" => …` es un go.mod válido que `go build` compila, y el
		// parser lo dejaba pasar porque comparaba el token CON las comillas puestas. El de `require`
		// era menos grave —fallaba cerrada— pero con un diagnóstico falso: decía «el módulo no
		// aparece en ningún require» sobre un go.mod donde aparece.
		{"require entrecomillado", "require (\n\t\"" + m + "\" v0.51.0\n)\n", "v0.51.0", ""},
		{"replace entrecomillado suelto", "require (\n\t" + m + " v0.51.0\n)\n\nreplace \"" + m + "\" => ../local\n", "", errReplace.Error()},
		{"replace entrecomillado en bloque", "require (\n\t" + m + " v0.51.0\n)\n\nreplace (\n\t\"" + m + "\" => ../local\n)\n", "", errReplace.Error()},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			v, err := versionRequerida(c.goMod, m)
			if c.quieroErr != "" {
				if err == nil || err.Error() != c.quieroErr {
					t.Fatalf("quería error %q, obtuve v=%q err=%v", c.quieroErr, v, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if v != c.quiero {
				t.Errorf("quería %q, obtuve %q", c.quiero, v)
			}
		})
	}
}

// trabajoQueDesviaElModulo busca un `go.work` desde la raíz del módulo hacia arriba y dice si tiene
// un `replace` para el módulo dado. Devuelve la ruta del archivo para poder nombrarlo en el error.
//
// Se busca HACIA ARRIBA porque así lo resuelve Go: un go.work en un directorio padre gobierna todos
// los módulos de abajo. Y se lee el archivo en vez de preguntarle a `go env GOWORK` para no atar
// esta guarda a que haya toolchain disponible — el mismo motivo por el que el resto de esta prueba
// lee go.mod a mano.
func trabajoQueDesviaElModulo(raiz, modulo string) (string, bool) {
	dir := raiz
	for {
		ruta := filepath.Join(dir, "go.work")
		if datos, err := os.ReadFile(ruta); err == nil {
			if _, errV := versionRequerida(string(datos), modulo); errV == errReplace {
				return ruta, true
			}
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			return "", false
		}
		dir = padre
	}
}

// TestElTripwireDeAplanadoVeLoQueContainsNoVeia — el control del tripwire nuevo, y está escrito
// para medir LA DIFERENCIA con el viejo, no para medirse a sí mismo.
//
// El tripwire anterior preguntaba `strings.Contains(GraphDeriverVersion, motorTreeSitterDeclarado)`
// y quedaba VERDE sobre el aplanado que decía cazar: el literal contiene el texto de la constante
// justamente porque salió de concatenarla. Cada subcaso de acá afirma las DOS cosas —qué contesta
// `Contains` y qué contesta el tripwire nuevo— porque si sólo afirmara la segunda, no se vería que
// la primera es la que estaba rota.
func TestElTripwireDeAplanadoVeLoQueContainsNoVeia(t *testing.T) {
	const cabecera = "package codeintel\n\nconst motorTreeSitterDeclarado = \"v0.52.0\"\nconst motorPolyglot = \"poly-on\"\n"

	casos := []struct {
		nombre        string
		decl          string
		valorResuelto string // lo que ese fuente compilaría, para preguntarle a `Contains`
		deriva        bool
	}{
		{
			nombre:        "sano: el sello concatena las dos constantes",
			decl:          `const GraphDeriverVersion = "5-crosspkg+ts-" + motorTreeSitterDeclarado + "+" + motorPolyglot`,
			valorResuelto: "5-crosspkg+ts-v0.52.0+poly-on",
			deriva:        true,
		},
		{
			nombre:        "APLANADO: el mismo valor, escrito como literal",
			decl:          `const GraphDeriverVersion = "5-crosspkg+ts-v0.52.0+poly-on"`,
			valorResuelto: "5-crosspkg+ts-v0.52.0+poly-on",
			deriva:        false,
		},
		{
			nombre:        "aplanado a medias: sólo el motor del tag sigue viajando",
			decl:          `const GraphDeriverVersion = "5-crosspkg+ts-v0.52.0+" + motorPolyglot`,
			valorResuelto: "5-crosspkg+ts-v0.52.0+poly-on",
			deriva:        false,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			ids, err := identificadoresDelSelloEn(cabecera + c.decl + "\n")
			if err != nil {
				t.Fatalf("no pude leer la declaración: %v", err)
			}
			deriva := contieneCadena(ids, "motorTreeSitterDeclarado") && contieneCadena(ids, "motorPolyglot")
			if deriva != c.deriva {
				t.Errorf("el tripwire dice deriva=%v y tendría que decir %v (referencia: %v)", deriva, c.deriva, ids)
			}

			// EL PUNTO DE TODO ESTO: los tres valores contienen el texto de la constante, así que
			// `Contains` contesta que sí en los tres — incluido el aplanado entero. Si esto alguna
			// vez deja de ser cierto, el tripwire viejo no era el instrumento ciego que creímos y
			// hay que releer por qué se cambió.
			if !strings.Contains(c.valorResuelto, "v0.52.0") {
				t.Fatalf("este subcaso ya no sirve de contraste: %q no contiene la versión, así que "+
					"`Contains` lo habría cazado y no demuestra ninguna ceguera", c.valorResuelto)
			}
		})
	}

	// Y la forma que el tripwire NO debe dejar pasar en silencio: que la declaración desaparezca.
	if _, err := identificadoresDelSelloEn("package codeintel\n\nconst Otra = \"x\"\n"); err == nil {
		t.Error("sin declaración de GraphDeriverVersion el lector contestó sin error: «no lo encontré» " +
			"se estaría leyendo como «está bien», que es el cero que significa «no sé»")
	}
}
