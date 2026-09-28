package memory

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
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

// corregirSinApuro corre el corrector con un plazo que no depende de la máquina. Estas pruebas miden
// QUÉ corrige; el plazo tiene su prueba (TestCorrectorSeRindeAlPlazo) y su medición en el hook. Con
// el de producción, la consulta de cinco tipeos tarda ~5 ms acá, y bajo `-race` en CI puede pasarse
// de los 60 ms: la consulta volvería sin corregir y la prueba fallaría por la máquina. Y un sabotaje
// que hace más lento al corrector quedaría verde por el plazo, no por la lógica.
func corregirSinApuro(e *DbEngine, q string, alcance ProjectScope) (string, []Correccion) {
	return e.CorregirConsultaConPlazo(context.Background(), q, alcance, 30*time.Second)
}

// TestCorrectorArreglaTransposicionYFaltaDeLetra: los tipeos de las dos clases que más comete el dueño
// se corrigen hacia la palabra que la memoria sí tiene, y la consulta reescrita conserva todo lo demás.
//
// Sabotaje: la transposición deja de invertir las letras (el candidato sale igual al término y se
// descarta). Se invierte dos veces y no con `c[k], c[k+1] = c[k], c[k+1]`, que go vet rechaza por
// autoasignación y la prueba ni compila.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\t\tc[k], c[k+1] = c[k+1], c[k]\n"
// arnes: a="\t\tc[k], c[k+1] = c[k+1], c[k]\n\t\tc[k], c[k+1] = c[k+1], c[k]\n"
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
	got, cs := corregirSinApuro(e, q, ProjectScope{})
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
// arnes: de="\tWHERE observations_fts MATCH '\"' || c.value || '\"' AND ` + visibles + clausula + `) FROM json_each(?) c`"
// arnes: a="\tWHERE observations_fts MATCH '\"' || c.value || '\"*' AND ` + visibles + clausula + `) FROM json_each(?) c`"
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
	got, cs := corregirSinApuro(e, "meorar el recall", ProjectScope{})
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
	got, cs := corregirSinApuro(e, q, ProjectScope{})
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
	if got, cs := corregirSinApuro(e, q, ProjectScope{}); len(cs) != 0 || got != q {
		t.Fatalf("sustituyó: %q (consulta %q)", resumenDeCorrecciones(cs), got)
	}
}

// TestCorrectorRespetaElOrdenDeLasClases: las clases van en orden estricto —dos letras invertidas,
// una letra de menos, una de más— y gana la PRIMERA que tenga un candidato vivo, aunque una clase
// posterior tenga uno más frecuente. Las palabras son inventadas para que cada tipeo tenga un solo
// candidato por clase: «zrobax» es «zorbax» con dos letras invertidas (2 notas) y «zroba» con una de
// más (3 notas); «qlmore» es «qelmore» con una de menos (2 notas) y «qmore» con una de más (3). Y la
// tercera clase corrige sola cuando es la única con candidato: «zorbaxx» → «zorbax».
//
// Sabotaje: las clases se prueban al revés.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tfor _, clase := range [][]string{transposiciones(r), faltaUnaLetra(r), sobraUnaLetra(r)} {\n"
// arnes: a="\tfor _, clase := range [][]string{sobraUnaLetra(r), faltaUnaLetra(r), transposiciones(r)} {\n"
//
// Sabotaje: «sobra una letra» no quita ninguna (el candidato sale igual al término y se descarta).
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\t\tout = append(out, string(r[:i])+string(r[i+1:]))\n"
// arnes: a="\t\tout = append(out, string(r[:i])+string(r[i:]))\n"
func TestCorrectorRespetaElOrdenDeLasClases(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc",
		"El zorbax del banco.", "Otro zorbax en la tanda.",
		"La zroba de ayer.", "Una zroba nueva.", "La zroba vieja.",
		"El qelmore del hook.", "Otro qelmore del turno.",
		"El qmore del sync.", "Un qmore más.", "El último qmore.",
	)
	for _, c := range []struct{ q, quiero string }{
		{"el zrobax de hoy", "zrobax→zorbax"},   // invertidas antes que «sobra» (zroba, 3 notas)
		{"el qlmore de hoy", "qlmore→qelmore"},  // «falta» antes que «sobra» (qmore, 3 notas)
		{"el zorbaxx de hoy", "zorbaxx→zorbax"}, // sólo «sobra» tiene candidato
	} {
		if _, cs := corregirSinApuro(e, c.q, ProjectScope{}); resumenDeCorrecciones(cs) != c.quiero {
			t.Errorf("%q: correcciones = %q, quería %q", c.q, resumenDeCorrecciones(cs), c.quiero)
		}
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
	got, cs := corregirSinApuro(e, "infromacion rasberry servdior comadno fihcaje", ProjectScope{})
	// Contra el 4 literal y no contra la constante: si la prueba leyera el tope del código, cambiar el
	// tope cambiaría también lo que la prueba espera.
	if len(cs) != 4 {
		t.Fatalf("corrigió %d términos (%q), quería 4: el tope", len(cs), resumenDeCorrecciones(cs))
	}
	// Los cuatro más largos; entre «comadno» y «fihcaje» (7 runas) queda el primero de la consulta.
	if quiero := "informacion raspberry servidor comando fihcaje"; got != quiero {
		t.Errorf("consulta corregida = %q, quería %q", got, quiero)
	}
}

// TestCorrectorTopeSeQuedaConLosMasLargos: con cinco muertos corregibles, el que queda afuera del tope
// es el MÁS CORTO («cabel»), no el último de la consulta («rasbperry»). Esta consulta los trae del más
// corto al más largo, al revés que la de TestCorrectorTieneTope: sin el orden por largo, el tope se
// quedaría con los cuatro primeros y soltaría justo el más largo.
//
// Sabotaje: los muertos no se ordenan por largo antes del tope.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tsort.SliceStable(muertos, func(i, j int) bool {\n\t\treturn len([]rune(muertos[i].bajo)) > len([]rune(muertos[j].bajo))\n\t})\n"
// arnes: a=""
func TestCorrectorTopeSeQuedaConLosMasLargos(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc",
		"El cable del kiosko quedó flojo.",
		"Cambiamos el cable de la raspberry.",
		"El kiosko del fichaje reinicia solo.",
		"El fichaje pasa por el servidor central.",
		"El servidor y la raspberry hablan por el tailnet.",
	)
	got, cs := corregirSinApuro(e, "cabel kisoko fichjae servdior rasbperry", ProjectScope{})
	if quiero := "kisoko→kiosko, fichjae→fichaje, servdior→servidor, rasbperry→raspberry"; resumenDeCorrecciones(cs) != quiero {
		t.Fatalf("correcciones = %q, quería %q: el tope suelta al más corto", resumenDeCorrecciones(cs), quiero)
	}
	if quiero := "cabel kiosko fichaje servidor raspberry"; got != quiero {
		t.Errorf("consulta corregida = %q, quería %q", got, quiero)
	}
}

// TestCorrectorExigeLargoYDosNotas: los dos pisos del corrector. Un término de menos de 5 runas no se
// revisa aunque tenga un vecino vivo («acsa» está a una transposición de «casa», que está en dos
// notas), y un candidato que está en UNA sola nota no alcanza («zrobax» está a una transposición de
// «zorbax», que está en una). La misma forma, con el candidato en dos notas, sí se corrige («qlemore»
// → «qelmore»): si no, la prueba no probaría nada. Medido sobre los prompts reales, bajar el piso a
// una nota cambiaba la mitad de las veces hacia algo malo («comienza→comenza»).
//
// Sabotaje: se revisan también los términos de 3 runas.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tminRunasDeTipeo = 5\n"
// arnes: a="\tminRunasDeTipeo = 3\n"
//
// Sabotaje: alcanza con que el candidato esté en una nota.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tdfMinimoDeCandidato = 2\n"
// arnes: a="\tdfMinimoDeCandidato = 1\n"
func TestCorrectorExigeLargoYDosNotas(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc",
		"La casa del kiosko.",
		"Otra casa del fichaje.",
		"El zorbax quedó prendido.",
		"El qelmore del servidor.",
		"Reiniciar el qelmore.",
	)
	for _, c := range []struct{ q, quiero string }{
		{"la acsa", ""},
		{"el zrobax", ""},
		{"el qlemore", "qlemore→qelmore"},
	} {
		_, cs := corregirSinApuro(e, c.q, ProjectScope{})
		if got := resumenDeCorrecciones(cs); got != c.quiero {
			t.Errorf("%q: correcciones = %q, quería %q", c.q, got, c.quiero)
		}
	}
}

// TestCorrectorNoRevisaTerminosLargos: un término de más de 40 runas no se revisa, aunque la memoria
// tenga en dos notas la palabra que está a una transposición; uno de 40 justas, sí (si no, la prueba no
// probaría nada). El techo es lo que acota el trabajo en Go, que el plazo no corta: sin él, un término
// de 3000 letras le hace armar a «falta una letra» 78.026 candidatos de 3001 runas antes de la primera
// consulta al índice, más de 200 MB.
//
// Sabotaje: el techo de largo se va.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de=" || len(r) > maxRunasDeTipeo"
// arnes: a=""
func TestCorrectorNoRevisaTerminosLargos(t *testing.T) {
	e := newTestEngine(t)
	justo := strings.Repeat("kiosko", 6) + "wxyz" // 40 runas
	largo := strings.Repeat("fichaje", 6)         // 42 runas
	sembrarTipeo(t, e, "doc",
		"El "+justo+" del turno.", "Otro "+justo+" del hook.",
		"El "+largo+" del turno.", "Otro "+largo+" del hook.",
	)
	// El tipeo de cada una: sus dos primeras letras invertidas.
	invertida := func(s string) string { return s[1:2] + s[:1] + s[2:] }
	q := invertida(justo) + " " + invertida(largo)
	got, cs := corregirSinApuro(e, q, ProjectScope{})
	if quiero := invertida(justo) + "→" + justo; resumenDeCorrecciones(cs) != quiero {
		t.Fatalf("correcciones = %q, quería %q: se revisa el de 40 runas y no el de 42", resumenDeCorrecciones(cs), quiero)
	}
	if quiero := justo + " " + invertida(largo); got != quiero {
		t.Errorf("consulta corregida = %q, quería %q", got, quiero)
	}

	// Y un término de 3000 letras no le cuesta memoria: ni se revisa. Con el plazo de producción, porque
	// lo que se mide es el trabajo en Go que el plazo no corta.
	q = "el " + strings.Repeat("qwzjx", 600)
	var antes, despues runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&antes)
	got, _ = e.CorregirConsulta(context.Background(), q, ProjectScope{})
	runtime.ReadMemStats(&despues)
	if got != q {
		t.Fatalf("la consulta de 3000 letras cambió: %q", got)
	}
	if alloc := despues.TotalAlloc - antes.TotalAlloc; alloc > 64<<20 {
		t.Errorf("una consulta de %d bytes alocó %d MB", len(q), alloc>>20)
	}
}

// TestCorrectorNoTocaTerminosConDigitos: un término con dígitos es un identificador (un host, una
// versión, un archivo), no una palabra mal tipeada. «kiosko2» está a una letra de sobra de «kiosko»,
// que la memoria tiene en dos notas, y aun así no se corrige.
//
// Sabotaje: se revisan también los términos con dígitos.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de=" || !todoLetras(r)"
// arnes: a=""
func TestCorrectorNoTocaTerminosConDigitos(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc",
		"El cable del kiosko quedó flojo.",
		"El kiosko del fichaje reinicia solo.",
	)
	q := "reinicia el kiosko2"
	if got, cs := corregirSinApuro(e, q, ProjectScope{}); len(cs) != 0 || got != q {
		t.Fatalf("corrigió un término con dígitos: %q (consulta %q)", resumenDeCorrecciones(cs), got)
	}
}

// TestCorrectorSeRindeAlPlazo: si se vence el plazo, la consulta va como vino y sin correcciones —una
// corrección a medias no se aplica—. Con el plazo de producción la misma consulta sí se corrige (si
// no, la prueba no probaría nada).
//
// Sabotaje: el corrector no se pone plazo.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tctx, cancel := context.WithTimeout(ctx, plazo)\n"
// arnes: a="\tctx, cancel := context.WithCancel(ctx)\n"
func TestCorrectorSeRindeAlPlazo(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc", "La información del fichaje.", "Más información del kiosko.")
	q := "infromacion del fichaje"
	// Plazo 0 y no 1 ns: con 1 ns el timer del contexto puede no haber disparado cuando corre la primera
	// consulta (en Windows el reloj avanza a saltos) y la corrección terminaba igual. Con 0 el contexto
	// nace vencido en cualquier máquina.
	if got, cs := e.CorregirConsultaConPlazo(context.Background(), q, ProjectScope{}, 0); len(cs) != 0 || got != q {
		t.Fatalf("con el plazo vencido corrigió: %q (consulta %q)", resumenDeCorrecciones(cs), got)
	}
	if _, cs := e.CorregirConsulta(context.Background(), q, ProjectScope{}); resumenDeCorrecciones(cs) != "infromacion→informacion" {
		t.Fatalf("con el plazo de producción corrigió %q: la prueba no probaría nada", resumenDeCorrecciones(cs))
	}
}

// TestCorrectorCandidatoMiraElPlazo: la fase de los candidatos también respeta el plazo. Con el
// contexto ya vencido, mejorCandidato no puede devolver un candidato sin error. TestCorrectorSeRindeAlPlazo
// no llega a mirar esta fase: con plazo 0 muere antes, en terminosMuertos.
//
// Sabotaje: la consulta de los candidatos no lleva el plazo.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tfilas, err := e.db.QueryContext(ctx, consulta, args...)\n\tif err != nil {\n\t\treturn \"\", 0, err\n"
// arnes: a="\tfilas, err := e.db.QueryContext(context.Background(), consulta, args...)\n\tif err != nil {\n\t\treturn \"\", 0, err\n"
func TestCorrectorCandidatoMiraElPlazo(t *testing.T) {
	e := newTestEngine(t)
	sembrarTipeo(t, e, "doc",
		"El cable del kiosko quedó flojo.",
		"El kiosko del fichaje reinicia solo.",
	)
	if buscado, _, err := e.mejorCandidato(context.Background(), "kisoko", ProjectScope{}); err != nil || buscado != "kiosko" {
		t.Fatalf("con el contexto vivo, mejorCandidato(kisoko) = %q, %v; quería kiosko (la prueba no probaría nada)", buscado, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if buscado, df, err := e.mejorCandidato(ctx, "kisoko", ProjectScope{}); err == nil {
		t.Fatalf("con el contexto vencido devolvió %q (df %d) sin error: la fase de candidatos no mira el plazo", buscado, df)
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
	if _, cs := corregirSinApuro(ro, "infromacion", ProjectScope{}); len(cs) == 0 {
		t.Fatal("en sólo lectura no corrigió «infromacion»: el corrector necesita escribir")
	}
}

// TestCorrectorRespetaElAlcance: el corrector nunca ve vocabulario que el recall no podría devolver,
// y el alcance vale en las DOS mitades. Para elegir: «raspberry» sólo existe en el proyecto b, así que
// una consulta acotada al proyecto a no se corrige hacia ella, y la misma consulta federada sí (si no,
// la prueba no probaría nada). Para decidir qué está muerto: el proyecto b guardó el tipeo «fichjae»
// tal cual, y eso no puede darlo por vivo en el proyecto a, que tiene «fichaje» y no el tipeo.
//
// Sabotaje: el conteo de candidatos pierde la cláusula del alcance.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tclausula, argsAlcance := alcance.scopeClause(\"o\")\n\tconsulta := sqlDFDeCandidatos("
// arnes: a="\tclausula, argsAlcance := ProjectScope{}.scopeClause(\"o\")\n\tconsulta := sqlDFDeCandidatos("
//
// Sabotaje: la detección de términos muertos pierde la cláusula del alcance.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="\tclausula, argsAlcance := alcance.scopeClause(\"o\")\n\tconsulta := sqlExpresionesVivas("
// arnes: a="\tclausula, argsAlcance := ProjectScope{}.scopeClause(\"o\")\n\tconsulta := sqlExpresionesVivas("
func TestCorrectorRespetaElAlcance(t *testing.T) {
	e := newTestEngine(t)
	delProyecto := func(proyecto string, contenidos ...string) {
		t.Helper()
		for _, id := range sembrarTipeo(t, e, proyecto, contenidos...) {
			if _, err := e.db.Exec(`UPDATE observations SET project_id = ? WHERE id = ?`, proyecto, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	delProyecto("b",
		"La raspberry del fichjae.",
		"Reiniciar la raspberry.",
		"La raspberry quedó sin red.",
	)
	delProyecto("a",
		"El kiosko de Altura.",
		"Otro kiosko en blanco.",
		"El fichaje de la mañana.",
		"El fichaje quedó en cero.",
	)
	q := "la rasberry del kiosko"
	if got, cs := corregirSinApuro(e, q, ProjectScope{ProjectID: "a"}); len(cs) != 0 || got != q {
		t.Fatalf("el proyecto a se corrigió con vocabulario del b: %q", resumenDeCorrecciones(cs))
	}
	if _, cs := corregirSinApuro(e, q, ProjectScope{}); resumenDeCorrecciones(cs) != "rasberry→raspberry" {
		t.Fatalf("federada no corrigió: %q (la prueba no probaría nada)", resumenDeCorrecciones(cs))
	}
	got, cs := corregirSinApuro(e, "el fichjae de hoy", ProjectScope{ProjectID: "a"})
	if resumenDeCorrecciones(cs) != "fichjae→fichaje" || got != "el fichaje de hoy" {
		t.Fatalf("en el proyecto a, «fichjae» quedó %q (consulta %q): lo dio por vivo una nota del b", resumenDeCorrecciones(cs), got)
	}
	// Y federada, la nota del b sí lo deja vivo: el tipeo es una palabra de la memoria que se ve.
	if _, cs := corregirSinApuro(e, "el fichjae de hoy", ProjectScope{}); len(cs) != 0 {
		t.Fatalf("federada corrigió «fichjae», que está en una nota visible: %q", resumenDeCorrecciones(cs))
	}
}

// TestCorrectorIgnoraInvisibles: lo que el recall no devuelve no cuenta, ni para decidir que un
// término está vivo ni para elegir un candidato. «infromacion» sólo aparece en una nota ARCHIVADA:
// sigue muerto, y se corrige. «kiosko» sólo aparece en notas archivadas: no es candidato de «kisoko».
//
// Sabotaje: la detección cuenta las notas invisibles.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="sqlExpresionesVivas(visibleObsPredicateDe(\"o\"), clausula)"
// arnes: a="sqlExpresionesVivas(\"1 = 1\", clausula)"
//
// Sabotaje: el conteo de candidatos cuenta las notas invisibles.
// arnes: archivo="internal/memory/tipeo.go"
// arnes: de="sqlDFDeCandidatos(visibleObsPredicateDe(\"o\"), clausula)"
// arnes: a="sqlDFDeCandidatos(\"1 = 1\", clausula)"
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
	got, cs := corregirSinApuro(e, "infromacion del kisoko", ProjectScope{})
	if quiero := "infromacion→informacion"; resumenDeCorrecciones(cs) != quiero {
		t.Fatalf("correcciones = %q, quería %q", resumenDeCorrecciones(cs), quiero)
	}
	if got != "informacion del kisoko" {
		t.Errorf("consulta corregida = %q", got)
	}
}
