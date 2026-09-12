package codeintel

import (
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
func TestGraphDeriverVersionAcusaElMotorDeTreeSitter(t *testing.T) {
	raiz := raizDelModulo(t)
	datos, err := os.ReadFile(filepath.Join(raiz, "go.mod"))
	if err != nil {
		t.Fatalf("no pude leer go.mod en %s: %v", raiz, err)
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
	// Tripwire de APLANADO: hoy es cierto por construcción (GraphDeriverVersion concatena la
	// constante), y deja de serlo en el instante en que alguien la reescriba como literal. Si el
	// motor no viaja DENTRO del valor que se compara contra el sello guardado, subirlo no re-deriva
	// nada y la guarda de arriba estaría custodiando un número que no decide.
	if !strings.Contains(GraphDeriverVersion, motorTreeSitterDeclarado) {
		t.Errorf("GraphDeriverVersion (%q) no contiene motorTreeSitterDeclarado (%q): el motor no "+
			"viaja en el sello, así que subirlo no dispara ninguna re-derivación",
			GraphDeriverVersion, motorTreeSitterDeclarado)
	}
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
