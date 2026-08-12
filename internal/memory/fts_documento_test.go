package memory

import (
	"fmt"
	"strings"
	"testing"
)

// fts_documento_test.go cubre buildFTSQueryDeDocumento: el builder del pool léxico cuando la
// "consulta" es una observación entera.
//
// Lo que defiende no es "la consulta es más corta" sino la propiedad que hace que guardar deje de
// castigar a las notas largas: EL COSTO NO PUEDE CRECER CON EL LARGO DEL TEXTO. Antes crecía
// superlineal —36× de largo costaba 102× de tiempo— porque cada término del documento, repetido y
// todo, entraba al MATCH.

func terminosDe(q string) []string {
	if q == "" {
		return nil
	}
	return strings.Split(q, " OR ")
}

// D1 EL INVARIANTE CENTRAL: un documento gigante no produce una consulta gigante.
//
// Es la propiedad que hace que el costo sea plano. Sin tope, este test explota en proporción al
// texto — que es exactamente lo que pasaba en producción.
func TestConsultaDeDocumentoNoCreceConElTexto(t *testing.T) {
	var sb strings.Builder
	for i := 0; sb.Len() < 40000; i++ {
		fmt.Fprintf(&sb, "termino%d distinto del anterior ", i)
	}
	q := buildFTSQueryDeDocumento(sb.String())
	n := len(terminosDe(q))
	if n > topeTerminosDeDocumento {
		t.Fatalf("un documento de 40.000 caracteres produjo %d términos; el tope es %d", n, topeTerminosDeDocumento)
	}
	// Y el tope tiene que estar REALMENTE alcanzado: si diera 3 términos el tope no probaría nada.
	if n != topeTerminosDeDocumento {
		t.Errorf("con vocabulario de sobra se esperaban %d términos, vinieron %d", topeTerminosDeDocumento, n)
	}
}

// D2 `"x" OR "x"` es idénticamente `"x"`: deduplicar no cambia qué matchea, sólo saca trabajo.
func TestConsultaDeDocumentoNoRepiteTerminos(t *testing.T) {
	q := buildFTSQueryDeDocumento(strings.Repeat("candado despacho red ", 60))
	terms := terminosDe(q)
	if len(terms) != 3 {
		t.Fatalf("tres palabras repetidas 60 veces deberían dar 3 términos, dieron %d: %s", len(terms), q)
	}
	vistos := map[string]bool{}
	for _, x := range terms {
		if vistos[x] {
			t.Errorf("término repetido en la consulta: %s", x)
		}
		vistos[x] = true
	}
}

// D3 El criterio de recorte es explícito: gana lo que MÁS se repite, que es de lo que habla la nota.
//
// Sin esto, el recorte se quedaría con lo que aparece primero —el orden del documento—, que no es
// un criterio: sería tirar información sin saber cuál.
func TestConsultaDeDocumentoPrefiereLoQueSeRepite(t *testing.T) {
	var sb strings.Builder
	// 60 términos de relleno que aparecen UNA vez, y al final el tema real repetido.
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&sb, "relleno%d ", i)
	}
	sb.WriteString(strings.Repeat("cognicion ", 12))

	terms := terminosDe(buildFTSQueryDeDocumento(sb.String()))
	if len(terms) == 0 {
		t.Fatal("consulta vacía")
	}
	if terms[0] != `"cognicion"` {
		t.Errorf("el término más repetido debería ir primero; vino %s (consulta: %s…)", terms[0], strings.Join(terms[:min(5, len(terms))], " "))
	}
	// Y tiene que SOBREVIVIR al recorte aunque aparezca último en el documento.
	if !strings.Contains(buildFTSQueryDeDocumento(sb.String()), `"cognicion"`) {
		t.Error("el tema de la nota se perdió en el recorte")
	}
}

// D4 Determinismo: dos guardados del mismo texto tienen que dar el mismo pool. Con empates
// resueltos por orden de mapa, esto sale rojo de forma intermitente — que es la peor clase de rojo.
func TestConsultaDeDocumentoEsDeterminista(t *testing.T) {
	doc := strings.Repeat("alfa beta gama delta epsilon ", 40) + "unico"
	primera := buildFTSQueryDeDocumento(doc)
	for i := 0; i < 50; i++ {
		if q := buildFTSQueryDeDocumento(doc); q != primera {
			t.Fatalf("misma entrada, consulta distinta en la vuelta %d:\n  %s\n  %s", i, primera, q)
		}
	}
}

// D5 Lo que NO puede pasar: que por acotar la consulta el detector deje de ver un duplicado obvio.
//
// Éste es el test de CALIDAD, y es el que le da permiso al de velocidad. Una observación casi
// idéntica a otra tiene que seguir apareciendo en el pool léxico aunque el texto sea largo.
// El corpus está armado para que un tope DEMASIADO CHICO se note: el vocabulario más frecuente lo
// comparten TODAS las notas, y lo que distingue al duplicado son términos de frecuencia media. Con
// un tope sano esos términos entran y el duplicado aparece; con un tope de uno o dos, la consulta
// se queda con las palabras comunes, matchea las 60 notas de ruido por igual y el duplicado se cae
// del pool. Sin esa competencia el test pasaría con cualquier tope — pasó en el primer intento.
func TestElRecorteNoCiegaAlDetector(t *testing.T) {
	e := newTestEngine(t)

	comun := strings.Repeat("planta inventario lotes proveedores despachos jornada ", 30)

	// 60 notas de ruido (más que el pool de 50) con el MISMO vocabulario frecuente.
	for i := 0; i < 60; i++ {
		cuerpo := comun + fmt.Sprintf(" variante numero %d de la nota de ruido.", i)
		if err := e.SaveObservation(fmt.Sprintf("ruido%d", i), "otro/tema", cuerpo, nil); err != nil {
			t.Fatalf("SaveObservation ruido: %v", err)
		}
	}

	// La vieja: mismo vocabulario común, más lo que la hace ella.
	propio := strings.Repeat("portero privacidad secretos marcador reversible rehidratar ", 8)
	largo := comun + propio
	if err := e.SaveObservation("vieja", "cognicion/portero", largo, nil); err != nil {
		t.Fatalf("SaveObservation: %v", err)
	}

	nueva := largo + " Se agrega una línea al final, pero es la misma nota."
	if err := e.SaveObservation("nueva", "cognicion/portero", nueva, nil); err != nil {
		t.Fatalf("SaveObservation nueva: %v", err)
	}

	src, ok, err := e.loadObsRow("nueva")
	if err != nil || !ok {
		t.Fatalf("loadObsRow: %v (ok=%v)", err, ok)
	}
	cands, err := e.lexicalConflictCandidates(src, 50)
	if err != nil {
		t.Fatalf("lexicalConflictCandidates: %v", err)
	}
	for _, c := range cands {
		if c.id == "vieja" {
			return // la vio: el recorte no cegó al detector
		}
	}
	t.Fatalf("el casi-duplicado no entró al pool léxico (%d candidatas): acotar la consulta le sacó vista al detector", len(cands))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
