package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// EL ONBOARDING NOMBRA CADA ESTADO QUE `musubi token list` PUEDE IMPRIMIR.
//
// docs/Server_Brain_Onboarding.md es donde un operador va a buscar qué significa lo que ve en el
// listado, y cuando se sumó «por vencer» siguió diciendo «muestra cuatro estados» y enumerando
// cuatro: quien abría el listado por la alerta `CredencialPorVencer` veía un estado que el doc no
// tenía, en un párrafo que además afirmaba que eran exactamente cuatro.
//
// Los estados NO se copian acá: salen de las constantes `Vencimiento*` de principals_admin.go,
// leídas con go/parser. Una lista escrita en la prueba envejecería junto con el doc y se darían la
// razón; así, el próximo estado que se agregue pone esto en rojo hasta que el doc lo nombre.
//
// Sabotaje que la pone roja: sacar «por vencer» de la enumeración del doc.
// arnes: archivo="docs/Server_Brain_Onboarding.md"
// arnes: de="`vigente` · `por vencer` · `VENCIDA`"
// arnes: a="`vigente` · `VENCIDA`"
//
// Sabotaje que la pone roja: que el doc vuelva a decir que son cuatro.
// arnes: archivo="docs/Server_Brain_Onboarding.md"
// arnes: de="muestra cinco estados**, y son cinco"
// arnes: a="muestra cuatro estados**, y son cuatro"
func TestElOnboardingNombraCadaEstadoDelListado(t *testing.T) {
	estados := estadosDeVencimientoDelCodigo(t)

	crudo, err := os.ReadFile(filepath.Join("..", "..", "docs", "Server_Brain_Onboarding.md"))
	if err != nil {
		t.Fatalf("no pude leer el doc de onboarding: %v", err)
	}
	doc := string(crudo)
	const cabeza = "- **`musubi token list` muestra "
	i := strings.Index(doc, cabeza)
	if i < 0 {
		t.Fatalf("el onboarding ya no tiene el párrafo %q que explica los estados del listado: o se "+
			"movió y esta prueba dejó de mirar, o se borró y nadie explica qué significa cada estado", cabeza)
	}
	parrafo := doc[i:]
	if j := strings.Index(parrafo[len(cabeza):], "\n- "); j >= 0 {
		parrafo = parrafo[:len(cabeza)+j]
	}

	// LA ENUMERACIÓN, NO EL PÁRRAFO: un estado explicado más abajo pero ausente de la lista `a · b`
	// deja la lista diciendo «son éstos» con uno de menos, que es el defecto que esto existe para ver.
	lista := regexp.MustCompile("(?:`[^`]+`\\s*·\\s*)+`[^`]+`").FindString(parrafo)
	if lista == "" {
		t.Fatalf("no encontré la enumeración de estados (`a` · `b` · …) en el párrafo del onboarding:\n%s", parrafo)
	}
	enDoc := map[string]bool{}
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(lista, -1) {
		enDoc[m[1]] = true
	}
	enCodigo := map[string]bool{}
	for _, e := range estados {
		enCodigo[e] = true
		if !enDoc[e] {
			t.Errorf("la enumeración del onboarding no nombra el estado `%s`, que `musubi token list` "+
				"imprime: quien lo vea en el listado no lo va a encontrar.\n  %s", e, lista)
		}
	}
	for e := range enDoc {
		if !enCodigo[e] {
			t.Errorf("la enumeración del onboarding nombra `%s`, que no es ningún estado del código (%v)", e, estados)
		}
	}
	palabras := map[int]string{3: "tres", 4: "cuatro", 5: "cinco", 6: "seis", 7: "siete", 8: "ocho"}
	cuantos, ok := palabras[len(estados)]
	if !ok {
		t.Fatalf("hay %d estados de vencimiento y esta prueba no sabe escribir ese número: agregalo a `palabras`", len(estados))
	}
	if !strings.Contains(parrafo, "muestra "+cuantos+" estados") {
		t.Errorf("el código tiene %d estados de vencimiento y el onboarding no dice «muestra %s estados»:\n%s",
			len(estados), cuantos, parrafo)
	}
}

// estadosDeVencimientoDelCodigo lee de principals_admin.go el valor de cada constante
// `Vencimiento*` de tipo texto.
func estadosDeVencimientoDelCodigo(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "principals_admin.go", nil, 0)
	if err != nil {
		t.Fatalf("no pude parsear principals_admin.go: %v", err)
	}
	var estados []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, sp := range g.Specs {
			vs := sp.(*ast.ValueSpec)
			for k, nombre := range vs.Names {
				if !strings.HasPrefix(nombre.Name, "Vencimiento") || k >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[k].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s: %v", nombre.Name, err)
				}
				estados = append(estados, v)
			}
		}
	}
	// El piso: eran cinco cuando se escribió esto. Menos es que el parseo dejó de mirar.
	if len(estados) < 5 {
		t.Fatalf("leí %d constantes Vencimiento* en principals_admin.go (%v) y eran cinco: el parseo "+
			"dejó de mirar, y un verde con menos estados no mediría nada", len(estados), estados)
	}
	return estados
}
