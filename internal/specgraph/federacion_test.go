package specgraph

// Federación opt-in (spec-graph F4): el vocabulario `shared` y el render de la proyección, en su
// sitio de declaración (el core). Lo que se fija acá son los dos invariantes que el core garantiza y
// que el motor/handler NO pueden re-garantizar sin él:
//   - R3: `shared` DEBE ser booleano al guardar; un valor fuera del dialecto se rechaza, no se
//     coacciona a local en silencio.
//   - R8/E7: el texto federable se arma de los campos PARSEADOS, así que la huella de las anclas de
//     F2 —que vive DENTRO del frontmatter— no puede viajar en la proyección (garantía estructural).
// Cada ancla «Sabotaje» lleva su directiva `// arnes:` mecanizada (archivo/de/a) glued debajo, para
// que el censo del arnés (internal/mcp/arnes_censo_test.go) la cuente como mecanizada, no como deuda.

import (
	"strings"
	"testing"
)

// TestValidarSharedRechazaNoBooleano fija R3 en su sitio: `shared` presente DEBE ser "true"/"false"
// (el frontmatter ya normalizó el booleano YAML a string); cualquier otra cosa es ERROR. Ausente es
// válido (default local, opt-in R2). No coaccionar es deliberado: si "banana" pasara como local, el
// autor que quiso federar nunca se enteraría; una relajación que lo tratara como `true` federaría basura.
//
// Sabotaje que lo pone en ROJO: neutralizar la cláusula de no-booleano de ValidarShared agregándole
// `&& false`. Por precedencia (`&&` liga más que `||`) la guarda queda en `!isStr`, así que un string
// fuera del dialecto ("banana", isStr=true) pasa SIN error y el caso banana se pone rojo.
// arnes: archivo="internal/specgraph/parse.go"
// arnes: de="if !isStr || (s != \"true\" && s != \"false\") {"
// arnes: a="if !isStr || (s != \"true\" && s != \"false\") && false {"
func TestValidarSharedRechazaNoBooleano(t *testing.T) {
	// Booleanos normalizados: válidos.
	for _, v := range []string{"true", "false"} {
		if err := ValidarShared(Frontmatter{FieldShared: v}); err != nil {
			t.Errorf("shared=%q debía ser válido: %v", v, err)
		}
	}
	// Ausente: válido (el default es local, R2).
	if err := ValidarShared(Frontmatter{FieldID: "x"}); err != nil {
		t.Errorf("shared ausente debía ser válido (default local): %v", err)
	}
	// String fuera del dialecto: ERROR, no coacción a local.
	if err := ValidarShared(Frontmatter{FieldShared: "banana"}); err == nil {
		t.Error("shared=\"banana\" debía rechazarse, no coaccionarse a local en silencio (R3)")
	}
	// No-string (lista): ERROR.
	if err := ValidarShared(Frontmatter{FieldShared: []string{"x"}}); err == nil {
		t.Error("shared como lista debía rechazarse (R3)")
	}
}

// TestRenderProjectionNoFiltraLaHuellaDeAnclas fija R8/E7: el texto federable se construye de id/type/
// title/cuerpo PARSEADOS, nunca del SPEC.md crudo ni del frontmatter serializado. Por eso la huella de
// las anclas de F2 —que vive dentro del frontmatter (`ruta#símbolo=digest`)— no puede colarse en la
// proyección: es una garantía ESTRUCTURAL de cómo se arma el texto, no un filtro que alguien recuerda.
//
// Sabotaje que lo pone en ROJO: que RenderProjection, además del title, vuelque las anclas crudas. El
// digest aparece en la salida y el test lo detecta. Es el único camino por el que la huella viajaría.
// arnes: archivo="internal/specgraph/parse.go"
// arnes: de="sb.WriteString(title)"
// arnes: a="sb.WriteString(title); for _, an := range List(fm, FieldAnchors) { sb.WriteString(an) }"
func TestRenderProjectionNoFiltraLaHuellaDeAnclas(t *testing.T) {
	const digest = "abc123deadbeefhuella"
	fm := Frontmatter{
		FieldID:      "s-dig",
		FieldType:    "module-design",
		FieldTitle:   "Título de la spec",
		FieldAnchors: []string{"internal/x/y.go#Foo=" + digest},
	}
	out := RenderProjection(fm, "cuerpo de la spec")

	if strings.Contains(out, digest) {
		t.Errorf("la huella de las anclas (F2) NO debe viajar en la proyección (R8/E7); salida:\n%s", out)
	}
	// Y el contenido útil SÍ viaja (id/type/title/cuerpo), o el render no serviría para nada.
	for _, want := range []string{"s-dig", "module-design", "Título de la spec", "cuerpo de la spec"} {
		if !strings.Contains(out, want) {
			t.Errorf("la proyección debía contener %q; salida:\n%s", want, out)
		}
	}
}
