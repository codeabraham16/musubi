package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// sembrarTipeo guarda cada contenido como una observación visible, sin proyecto.
func sembrarTipeo(t *testing.T, e *DbEngine, prefijo string, contenidos ...string) []string {
	t.Helper()
	ids := make([]string, 0, len(contenidos))
	for i, c := range contenidos {
		id := fmt.Sprintf("%s-%d", prefijo, i)
		if err := e.SaveObservation(id, "tipeo/"+prefijo, c, nil); err != nil {
			t.Fatalf("SaveObservation(%s): %v", id, err)
		}
		ids = append(ids, id)
	}
	return ids
}

// resumenDeCorrecciones deja las correcciones como «tipeado→buscado», en orden, para comparar.
func resumenDeCorrecciones(cs []Correccion) string {
	partes := make([]string, 0, len(cs))
	for _, c := range cs {
		partes = append(partes, c.Tipeado+"→"+c.Buscado)
	}
	return strings.Join(partes, ", ")
}

// TestCorrectorArreglaTransposicionYFaltaDeLetra: los tipeos de las dos clases que más comete el dueño
// se corrigen hacia la palabra que la memoria sí tiene, y la consulta reescrita conserva todo lo demás.
//
// Sabotaje: la transposición deja de invertir las letras (el candidato sale igual al término y se
// descarta).
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\t\tc[k], c[k+1] = c[k+1], c[k]\n"
// arnes: a="\t\tc[k], c[k+1] = c[k], c[k+1]\n"
//
// Sabotaje: «falta una letra» no prueba ninguna posición.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tfor i := 0; i <= len(r); i++ {\n"
// arnes: a="\tfor i := 0; i < 0; i++ {\n"
func TestCorrectorArreglaTransposicionYFaltaDeLetra(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc",
		"La información del fichaje quedó guardada en la base.",
		"Más información sobre el kiosko de la raspberry.",
		"El comando para reiniciar el servicio del kiosko.",
		"Otro comando útil en la raspberry del fichaje.",
	)
	q := "infromacion del comadno en la rasberry"
	got, cs := e.CorregirConsulta(context.Background(), q, ProjectScope{})
	if quiero := "infromacion→informacion, comadno→comando, rasberry→raspberry"; resumenDeCorrecciones(cs) != quiero {
		t.Fatalf("correcciones = %q, quería %q", resumenDeCorrecciones(cs), quiero)
	}
	if quiero := "informacion del comando en la raspberry"; got != quiero {
		t.Errorf("consulta corregida = %q, quería %q", got, quiero)
	}
}

// TestCorrectorEligeElTerminoExacto: el candidato se verifica como término EXACTO del índice. Un
// candidato que sólo es el comienzo de otra palabra no está vivo, y el df que decide es el del término
// exacto: «mejorar» está en 2 notas, aunque «mejorarlo» esté en otras 3. Las 20 notas con «memoria»
// son el vecino frecuente que NO está a un tipeo de «meorar», y no tiene que tirar.
//
// Sabotaje: la verificación de candidatos pasa a ser por prefijo.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tWHERE observations_fts MATCH '\"' || c.value || '\"' AND %s%s) FROM json_each(?) c`"
// arnes: a="\tWHERE observations_fts MATCH '\"' || c.value || '\"*' AND %s%s) FROM json_each(?) c`"
func TestCorrectorEligeElTerminoExacto(t *testing.T) {
	e := newTestEngine(t)
	var docs []string
	for i := 0; i < 20; i++ {
		docs = append(docs, fmt.Sprintf("Nota %d sobre la memoria del proyecto.", i))
	}
	docs = append(docs,
		"Hay que mejorar el recall del turno.",
		"Conviene mejorar la latencia del hook.",
		"El índice: mejorarlo cuesta poco.",
		"La consulta, mejorarlo después.",
		"El banco, mejorarlo con otra semilla.",
	)
	sembrarTipeo(t, e, "doc", docs...)
	got, cs := e.CorregirConsulta(context.Background(), "meorar el recall", ProjectScope{})
	if len(cs) != 1 || cs[0].Buscado != "mejorar" {
		t.Fatalf("correcciones = %q, quería meorar→mejorar", resumenDeCorrecciones(cs))
	}
	if cs[0].DF != 2 {
		t.Errorf("df de «mejorar» = %d, quería 2: el df tiene que ser el del término exacto, no el de un prefijo", cs[0].DF)
	}
	if got != "mejorar el recall" {
		t.Errorf("consulta corregida = %q", got)
	}
}

// TestCorrectorNoTocaTerminosVivos: un término que la memoria tiene no se corrige, aunque haya una
// palabra a un tipeo de distancia. «investigue» está vivo como término exacto («investigué» es el
// mismo término del índice: el FTS pliega las tildes) y «deploys» está vivo por el prefijo de su raíz
// («deploy»), que es la cláusula del recall. Las dos tienen un vecino que el corrector elegiría si las
// diera por muertas: «investiguen» (una letra de menos) y «deploy» (una de más).
//
// Sabotaje: ningún término cuenta como vivo.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\treturn vivas[expr]\n"
// arnes: a="\treturn false\n"
//
// Sabotaje: la detección pierde la pasada del prefijo de raíz.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tvivos, err = e.expresionesVivas(ctx, prefijos, alcance)\n"
// arnes: a="\tvivos, err = map[string]bool{}, nil\n"
func TestCorrectorNoTocaTerminosVivos(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc",
		"El deploy del cerebro central.",
		"Otro deploy, ahora con la migración.",
		"El deployment quedó documentado.",
		"Ayer investigué el cursor del sync.",
		"Que investiguen el eco del backfill.",
		"Los que investiguen el grafo, avisen.",
	)
	q := "revisá los deploys que investigue ayer"
	got, cs := e.CorregirConsulta(context.Background(), q, ProjectScope{})
	if len(cs) != 0 || got != q {
		t.Fatalf("corrigió términos vivos: %q (consulta %q)", resumenDeCorrecciones(cs), got)
	}
}

// TestCorrectorNoUsaSustitucion: una letra cambiada por otra no se corrige, porque ahí el tipeo suele
// ser otra palabra válida: «dejemos» se queda, aunque «dejamos» esté en la memoria.
//
// Sabotaje: «falta una letra» reemplaza la letra en vez de agregarla (o sea, sustituye).
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\t\t\tout = append(out, string(r[:i])+string(l)+string(r[i:]))\n"
// arnes: a="\t\t\tout = append(out, string(r[:i])+string(l)+string(r[min(i+1, len(r)):]))\n"
func TestCorrectorNoUsaSustitucion(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc",
		"Lo dejamos para la próxima tanda.",
		"Ya lo dejamos anotado en pendientes.",
		"Dejamos el binario viejo en su lugar.",
	)
	q := "dejemos el binario"
	if got, cs := e.CorregirConsulta(context.Background(), q, ProjectScope{}); len(cs) != 0 || got != q {
		t.Fatalf("sustituyó: %q (consulta %q)", resumenDeCorrecciones(cs), got)
	}
}

// TestCorrectorTieneTope: con cinco tipeos, se corrigen cuatro — los más largos — y el quinto queda
// como vino.
//
// Sabotaje: el tope sube a 40.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tmaxCorreccionesDeTipeo = 4\n"
// arnes: a="\tmaxCorreccionesDeTipeo = 40\n"
func TestCorrectorTieneTope(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc",
		"La información del servidor y el comando del fichaje en la raspberry.",
		"Más información: el servidor corre el comando del fichaje en la raspberry.",
	)
	got, cs := e.CorregirConsulta(context.Background(), "infromacion rasberry servdior comadno fihcaje", ProjectScope{})
	if len(cs) != maxCorreccionesDeTipeo {
		t.Fatalf("corrigió %d términos (%q), el tope es %d", len(cs), resumenDeCorrecciones(cs), maxCorreccionesDeTipeo)
	}
	// Los cuatro más largos; entre «comadno» y «fihcaje» (7 runas) queda el primero de la consulta.
	if quiero := "informacion raspberry servidor comando fihcaje"; got != quiero {
		t.Errorf("consulta corregida = %q, quería %q", got, quiero)
	}
}

// TestCorrectorCorrigeEnSoloLectura: el corrector no escribe nada, así que anda sobre un motor abierto
// con NewDbEngineSoloLectura (query_only). Una implementación con fts5vocab necesitaría un CREATE, y
// ahí fallaría en silencio: la consulta volvería sin corregir.
//
// Sabotaje: la detección arma antes una tabla fts5vocab temporal.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tfilas, err := e.db.QueryContext(ctx, consulta, args...)\n\tif err != nil {\n\t\treturn nil, err\n"
// arnes: a="\tif _, err := e.db.ExecContext(ctx, \"CREATE VIRTUAL TABLE IF NOT EXISTS temp.voc USING fts5vocab(main, 'observations_fts', 'row')\"); err != nil {\n\t\treturn nil, err\n\t}\n\tfilas, err := e.db.QueryContext(ctx, consulta, args...)\n\tif err != nil {\n\t\treturn nil, err\n"
func TestCorrectorCorrigeEnSoloLectura(t *testing.T) {
	dir := dirSembrado(t)
	e, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	sembrarTipeo(t, e, "doc", "La información del fichaje.", "Más información del kiosko.")
	e.Close()

	ro, err := NewDbEngineSoloLectura(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSoloLectura: %v", err)
	}
	defer ro.Close()
	if _, cs := ro.CorregirConsulta(context.Background(), "infromacion", ProjectScope{}); len(cs) == 0 {
		t.Fatal("en sólo lectura no corrigió «infromacion»: el corrector necesita escribir")
	}
}

// TestCorrectorRespetaElAlcance: el corrector nunca ve vocabulario que el recall no podría devolver.
// «raspberry» sólo existe en el proyecto b: una consulta acotada al proyecto a no se corrige hacia
// ella, y la misma consulta federada sí (si no, la prueba no probaría nada).
//
// Sabotaje: el conteo de candidatos pierde la cláusula del alcance.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tclausula, argsAlcance := alcance.scopeClause(\"o\")\n\tconsulta := fmt.Sprintf(sqlDFDeCandidatos"
// arnes: a="\tclausula, argsAlcance := ProjectScope{}.scopeClause(\"o\")\n\tconsulta := fmt.Sprintf(sqlDFDeCandidatos"
func TestCorrectorRespetaElAlcance(t *testing.T) {
	e := newTestEngine(t)
	deB := sembrarTipeo(t, e, "b",
		"La raspberry del fichaje.",
		"Reiniciar la raspberry.",
		"La raspberry quedó sin red.",
	)
	for _, id := range deB {
		if _, err := e.db.Exec(`UPDATE observations SET project_id = ? WHERE id = ?`, "b", id); err != nil {
			t.Fatal(err)
		}
	}
	deA := sembrarTipeo(t, e, "a", "El kiosko de Altura.", "Otro kiosko en blanco.")
	for _, id := range deA {
		if _, err := e.db.Exec(`UPDATE observations SET project_id = ? WHERE id = ?`, "a", id); err != nil {
			t.Fatal(err)
		}
	}
	q := "la rasberry del kiosko"
	if got, cs := e.CorregirConsulta(context.Background(), q, ProjectScope{ProjectID: "a"}); len(cs) != 0 || got != q {
		t.Fatalf("el proyecto a se corrigió con vocabulario del b: %q", resumenDeCorrecciones(cs))
	}
	if _, cs := e.CorregirConsulta(context.Background(), q, ProjectScope{}); resumenDeCorrecciones(cs) != "rasberry→raspberry" {
		t.Fatalf("federada no corrigió: %q (la prueba no probaría nada)", resumenDeCorrecciones(cs))
	}
}

// TestCorrectorIgnoraInvisibles: lo que el recall no devuelve no cuenta, ni para decidir que un
// término está vivo ni para elegir un candidato. «infromacion» sólo aparece en una nota ARCHIVADA:
// sigue muerto, y se corrige. «kiosko» sólo aparece en notas archivadas: no es candidato de «kisoko».
//
// Sabotaje: la detección cuenta las notas invisibles.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="fmt.Sprintf(sqlExpresionesVivas, visibleObsPredicateDe(\"o\"), clausula)"
// arnes: a="fmt.Sprintf(sqlExpresionesVivas, \"1 = 1\", clausula)"
//
// Sabotaje: el conteo de candidatos cuenta las notas invisibles.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="fmt.Sprintf(sqlDFDeCandidatos, visibleObsPredicateDe(\"o\"), clausula)"
// arnes: a="fmt.Sprintf(sqlDFDeCandidatos, \"1 = 1\", clausula)"
func TestCorrectorIgnoraInvisibles(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "vis", "La información del fichaje.", "Más información del turno.")
	archivadas := sembrarTipeo(t, e, "arch",
		"Una nota vieja con infromacion mal escrita.",
		"El kiosko quedó en blanco.",
		"Otro kiosko sin red.",
	)
	for _, id := range archivadas {
		if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}
	got, cs := e.CorregirConsulta(context.Background(), "infromacion del kisoko", ProjectScope{})
	if quiero := "infromacion→informacion"; resumenDeCorrecciones(cs) != quiero {
		t.Fatalf("correcciones = %q, quería %q", resumenDeCorrecciones(cs), quiero)
	}
	if got != "informacion del kisoko" {
		t.Errorf("consulta corregida = %q", got)
	}
}
