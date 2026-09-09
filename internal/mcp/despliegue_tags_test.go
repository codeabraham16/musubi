package mcp

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// LOS TAGS DE COMPILACIÓN ESTÁN ESCRITOS DOS VECES, Y ESO DECIDE QUÉ ENTIENDE EL BINARIO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ESTO ES UNA PRUEBA Y NO UN COMENTARIO EN LOS DOS ARCHIVOS
//
// `-tags treesitter` (más los `grammar_subset_*`) es lo que hace que el binario derive el grafo de
// código de TS/TSX/JS/Python. Sin ellos entiende Go y nada más.
//
// La lista vive en dos lugares que no se pueden leer entre sí: `deploy/construir.sh` (bash, el
// binario de desarrollo y el que se redespliega al cerebro) y `.github/workflows/release.yml`
// (Actions, el binario que se publica). Hasta hoy sólo el segundo los tenía, así que había DOS
// binarios que imprimían la misma versión y entendían lenguajes distintos — y la diferencia no se
// veía por ningún lado. Medido sobre el binario del árbol: cero apariciones de gotreesitter.
//
// Que dos listas separadas digan lo mismo es una convención, y una convención no se custodia sola.
// Es la misma forma de defecto que `musubi-alerts.yml` documenta sobre sus dos archivos de reglas:
// «se copian juntos, así que envejecen juntos» — hasta que alguien copia uno solo.
//
// Sabotaje que la hace fallar: sacar un grammar_subset de cualquiera de los dos; agregar una
// gramática nueva a uno y olvidarse del otro; sacar el `-tags` de construir.sh.
func TestLosTagsDeConstruirIgualanAlosDelRelease(t *testing.T) {
	construir := leerParaTags(t, "../../deploy/construir.sh")
	release := leerParaTags(t, "../../.github/workflows/release.yml")

	deConstruir := extraerTags(construir, regexp.MustCompile(`(?m)^TAGS='([^']*)'`))
	if len(deConstruir) == 0 {
		t.Fatal("no encontré la asignación `TAGS='...'` en deploy/construir.sh. Si se renombró la variable, actualizá esta guarda; si se BORRÓ, el binario local volvió a entender sólo Go y hay que devolverla")
	}

	deRelease := extraerTags(release, regexp.MustCompile(`-tags '([^']*)'`))
	if len(deRelease) == 0 {
		t.Fatal("no encontré `-tags '...'` en .github/workflows/release.yml: el binario que se PUBLICA dejaría de entender TS/JS/Python, y esta guarda no puede verificar nada")
	}

	if strings.Join(deConstruir, " ") != strings.Join(deRelease, " ") {
		t.Errorf("los tags de compilación DIFIEREN y por lo tanto el binario local y el publicado entienden lenguajes distintos, imprimiendo la misma versión:\n  construir.sh: %v\n  release.yml:  %v\nfaltan en construir.sh: %v\nfaltan en release.yml:  %v",
			deConstruir, deRelease, tagsFaltantes(deRelease, deConstruir), tagsFaltantes(deConstruir, deRelease))
	}

	// Y el tag que de verdad decide: sin `treesitter` los grammar_subset_* no hacen nada, así que
	// una lista que los tenga a todos MENOS ése pasaría la comparación de arriba estando rota.
	if !tagPresente(deConstruir, "treesitter") {
		t.Error("la lista de tags no incluye `treesitter`: los grammar_subset_* por sí solos no linkean nada y el binario entiende sólo Go")
	}
}

func leerParaTags(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no pude leer %s: %v", ruta, err)
	}
	return string(b)
}

// extraerTags extrae la lista y la ORDENA: el orden de los tags no cambia el binario, así que exigir
// que coincida haría fallar la guarda por algo que no es un defecto.
func extraerTags(contenido string, re *regexp.Regexp) []string {
	m := re.FindStringSubmatch(contenido)
	if m == nil {
		return nil
	}
	tags := strings.Fields(m[1])
	sort.Strings(tags)
	return tags
}

func tagsFaltantes(tiene, contra []string) []string {
	set := map[string]bool{}
	for _, t := range contra {
		set[t] = true
	}
	var out []string
	for _, t := range tiene {
		if !set[t] {
			out = append(out, t)
		}
	}
	return out
}

func tagPresente(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
