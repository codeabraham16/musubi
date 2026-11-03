package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// conflicts_cobertura_test.go custodia la SALIDA de `musubi conflicts shadow`: que el JSON traiga
// la cobertura con sus claves en minúscula (R6) y que el camino del libro vacío declare el techo en
// vez de mandar a revisar la config (R7).
//
// Y, desde la verify de este expediente, la COSTURA: que el comando le PASE la cobertura a cada uno
// de los tres caminos. Los cuatro custodios de las piezas pasaban verdes con el cableado roto.

// TestShadowJSONTraeCoberturaYResumenConClavesEnMinuscula custodia R6 y la regla 6 del repo: un DTO
// sin tags explícitos serializa en mayúscula, el receptor que parsea en minúscula NO falla, y
// guarda los campos vacíos. Es una falla que se lee como un dato, así que el custodio no mira sólo
// que las claves esperadas estén: mira que NO haya ninguna que arranque en mayúscula.
// Sabotaje que la hace fallar: sacarle los tags JSON al sobre, que entonces emite las claves en mayúscula.
// arnes: archivo="cmd/musubi/conflicts.go"
// arnes: de="Coverage   memory.ShadowCoverageReport `json:\"coverage\"`\n\tByRelation []memory.ShadowAgreement    `json:\"by_relation\"`"
// arnes: a="Coverage   memory.ShadowCoverageReport\n\tByRelation []memory.ShadowAgreement"
func TestShadowJSONTraeCoberturaYResumenConClavesEnMinuscula(t *testing.T) {
	lex := 0.83
	payload := shadowJSON{
		Coverage: memory.ShadowCoverageReport{
			Relaciones: 1125, Redetectables: 895, Medibles: 0,
			FirmaAjena: 895, PuntaNoVisible: 230, AutoAplicadas: 2, Medidas: 0,
		},
		ByRelation: []memory.ShadowAgreement{
			{HeurRelation: "supersedes", Total: 2, Agreed: 1, Rate: 0.5, LexMin: &lex, LexMax: &lex},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raiz map[string]json.RawMessage
	if err := json.Unmarshal(b, &raiz); err != nil {
		t.Fatalf("Unmarshal de la raíz: %v", err)
	}
	for _, k := range []string{"coverage", "by_relation"} {
		if _, ok := raiz[k]; !ok {
			t.Errorf("falta la clave %q en la raíz; salieron %v", k, clavesDe(raiz))
		}
	}
	var cob map[string]json.RawMessage
	if err := json.Unmarshal(raiz["coverage"], &cob); err != nil {
		t.Fatalf("Unmarshal de coverage: %v", err)
	}
	for _, k := range []string{"relaciones", "redetectables", "medibles", "firma_ajena", "punta_no_visible", "auto_aplicadas", "medidas"} {
		if _, ok := cob[k]; !ok {
			t.Errorf("falta la clave %q en coverage; salieron %v", k, clavesDe(cob))
		}
	}
	for _, m := range []map[string]json.RawMessage{raiz, cob} {
		for k := range m {
			if k != strings.ToLower(k) {
				t.Errorf("la clave %q no está en minúscula: el DTO perdió su tag JSON", k)
			}
		}
	}
}

func clavesDe(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestLibroVacioDeclaraElTechoYNoCulpaALaConfig custodia R7, que es el defecto concreto que abrió
// este expediente: con el libro vacío el comando mandaba a revisar `conflicts.shadow.enabled` y el
// motor de cognición. El fixture siembra el techo cero porque es el caso del defecto: con techo
// cero encender las dos cosas no habría escrito una sola fila, y el operador busca el problema
// donde no está.
//
// ⚠️ ACÁ HUBO DOS NÚMEROS Y LOS DOS NACIERON VENCIDOS: primero «medido el 2026-11-02 el techo era
// CERO», después «son 4 pares de 1.138». Ese mismo día el techo fue 6, 4 y 0, y lo llevó a 0 el
// propio trabajo de este PR arbitrando relaciones por otro motivo. El techo es AUTO-CONSUMIBLE
// —cualquier arbitraje drena un `pending`, y un `heuristic` que se aplica oculta su target y se
// autoexcluye—, así que su estado estacionario es cero y un número suelto es la foto de un
// transitorio. Lo que este test custodia es la CONDUCTA ante el techo cero, no cuánto vale el
// corpus: por eso el fixture lo siembra en vez de leerlo.
// Sabotaje que la hace fallar: sacarle el `return` al techo cero, que entonces sigue y manda a encender la config.
// arnes: archivo="cmd/musubi/conflicts.go"
// arnes: de="fmt.Fprintln(w, \"procedencia, así que lo que falta no es el motor. El techo de hoy es CERO.\")\n\t\treturn"
// arnes: a="fmt.Fprintln(w, \"procedencia, así que lo que falta no es el motor. El techo de hoy es CERO.\")"
func TestLibroVacioDeclaraElTechoYNoCulpaALaConfig(t *testing.T) {
	t.Run("techo cero: no manda a la config", func(t *testing.T) {
		var buf bytes.Buffer
		imprimirLibroVacio(&buf, memory.ShadowCoverageReport{
			Relaciones: 1125, Redetectables: 895, Medibles: 0, FirmaAjena: 895,
			PuntaNoVisible: 230, AutoAplicadas: 2,
		})
		salida := buf.String()
		if !strings.Contains(salida, "no escribiría una sola fila") {
			t.Errorf("con techo cero la salida tiene que decir que encenderla no escribe nada; dijo:\n%s", salida)
		}
		if strings.Contains(salida, "conflicts.shadow.enabled") {
			t.Errorf("con techo cero NO puede mandar a la config: es la causa equivocada. Salida:\n%s", salida)
		}
		if !strings.Contains(salida, "895") {
			t.Errorf("la salida tiene que traer el denominador; dijo:\n%s", salida)
		}
	})

	t.Run("techo positivo: dice cuanto y manda a la config", func(t *testing.T) {
		var buf bytes.Buffer
		imprimirLibroVacio(&buf, memory.ShadowCoverageReport{
			Relaciones: 10, Redetectables: 9, Medibles: 4, FirmaAjena: 5, PuntaNoVisible: 1,
		})
		salida := buf.String()
		if !strings.Contains(salida, "el techo son 4 pares") {
			t.Errorf("con techo positivo tiene que decir el número; dijo:\n%s", salida)
		}
		if !strings.Contains(salida, "conflicts.shadow.enabled") {
			t.Errorf("con techo positivo SÍ corresponde mandar a la config; dijo:\n%s", salida)
		}
	})
}

// TestCoberturaNombraElMotivoDeCadaRecorte custodia R1 y R5: el número sin el motivo es justo lo
// que se lee como una falla. «895 afuera» parece el detector roto y es la guarda del upsert
// funcionando; «230 no re-detectables» parece corpus podrido y es trabajo cerrado; y el balde de la
// auto-resolución aplicada es el que evita que un cero de medibles mande a buscar un bug que no hay.
func TestCoberturaNombraElMotivoDeCadaRecorte(t *testing.T) {
	var buf bytes.Buffer
	imprimirCobertura(&buf, memory.ShadowCoverageReport{
		Relaciones: 1125, Redetectables: 895, Medibles: 0, FirmaAjena: 895,
		PuntaNoVisible: 230, AutoAplicadas: 2, Medidas: 0,
	})
	salida := buf.String()
	for _, motivo := range []string{
		"veredicto AJENO",                       // (a) la guarda lo protegió
		"archivada, superseded o en cuarentena", // (b) la detección no vuelve
		"auto-resoluciones del detector",        // (c) el subconjunto que explica el cero
		"conflicts backfill",                    // y qué sí se puede hacer con esa señal
	} {
		if !strings.Contains(salida, motivo) {
			t.Errorf("la cobertura no nombra %q; salida:\n%s", motivo, salida)
		}
	}
	// Los baldes en cero no se imprimen: una línea «0 quedan afuera por…» es ruido que compite con
	// el número que sí importa.
	var vacia bytes.Buffer
	imprimirCobertura(&vacia, memory.ShadowCoverageReport{})
	if strings.Contains(vacia.String(), "veredicto AJENO") {
		t.Errorf("con FirmaAjena=0 no corresponde imprimir ese balde; salida:\n%s", vacia.String())
	}
}

// TestPorcentajeMedibleSinDenominadorNoDiceNaN custodia el borde del corpus recién nacido: 0/0 en
// float es NaN, y un «(NaN%)» se lee como un error del comando en vez de como lo que es, un corpus
// sin nada que medir todavía.
func TestPorcentajeMedibleSinDenominadorNoDiceNaN(t *testing.T) {
	if got := porcentajeMedible(memory.ShadowCoverageReport{}); got != "" {
		t.Errorf("porcentajeMedible sin denominador = %q, esperaba vacío", got)
	}
	got := porcentajeMedible(memory.ShadowCoverageReport{Redetectables: 895, Medibles: 2})
	if !strings.Contains(got, "0.2") {
		t.Errorf("porcentajeMedible(2/895) = %q, esperaba que trajera 0.2", got)
	}
}

// TestEmitirShadowLlevaLaCoberturaALosTresCaminos custodia la COSTURA, que es lo que los otros
// cuatro custodios de este archivo NO miran: ellos miden las PIEZAS —el sobre, el helper del libro
// vacío, los motivos, el porcentaje— y ninguno mira que el comando se las pase. Medido en la verify
// de este expediente: las tres mutaciones del cableado dejaban `go test ./cmd/musubi/` ENTERO en
// verde, y una de ellas —pasarle un reporte en cero al libro vacío— hace que el comando vuelva a
// decir «techo CERO» teniendo techo 4, que es EXACTAMENTE el defecto que R7 vino a arreglar.
//
// Sabotaje que la hace fallar (1/3): emitir el array pelado en vez del sobre, o sea el `--json` de antes del cambio.
// arnes: archivo="cmd/musubi/conflicts.go"
// arnes: de="json.MarshalIndent(shadowJSON{Coverage: cob, ByRelation: res}, \"\", \"  \")"
// arnes: a="json.MarshalIndent(res, \"\", \"  \")"
//
// Sabotaje que la hace fallar (2/3): pasarle al libro vacío un reporte en CERO en vez del derivado.
// arnes: archivo="cmd/musubi/conflicts.go"
// arnes: de="imprimirLibroVacio(w, cob)"
// arnes: a="imprimirLibroVacio(w, memory.ShadowCoverageReport{})"
//
// Sabotaje que la hace fallar (3/3): derivar la cobertura y NO llevarla a la salida, que es la tabla de antes.
// arnes: archivo="cmd/musubi/conflicts.go"
// arnes: de="imprimirCobertura(w, cob)"
// arnes: a="imprimirCobertura(io.Discard, cob)"
func TestEmitirShadowLlevaLaCoberturaALosTresCaminos(t *testing.T) {
	lex := 0.83
	// Techo POSITIVO a propósito. Con Medibles en 0 las dos ramas del libro vacío dicen cosas
	// distintas por motivos distintos, y el custodio no podría distinguir «le llegó la cobertura»
	// de «le llegó un cero»: es la misma trampa de la permutación que vació al custodio de R4.
	cob := memory.ShadowCoverageReport{
		Relaciones: 10, Redetectables: 9, Medibles: 4, FirmaAjena: 5, PuntaNoVisible: 1,
	}
	res := []memory.ShadowAgreement{
		{HeurRelation: "supersedes", Total: 2, Agreed: 1, Rate: 0.5, LexMin: &lex, LexMax: &lex},
	}

	t.Run("json: objeto con la cobertura, no el array pelado", func(t *testing.T) {
		var buf bytes.Buffer
		emitirShadow(&buf, cob, res, true)
		var raiz map[string]json.RawMessage
		if err := json.Unmarshal(buf.Bytes(), &raiz); err != nil {
			t.Fatalf("el --json no emitió un objeto (%v); salida:\n%s", err, buf.String())
		}
		for _, k := range []string{"coverage", "by_relation"} {
			if _, ok := raiz[k]; !ok {
				t.Errorf("el --json salió sin %q; claves: %v", k, clavesDe(raiz))
			}
		}
		// Y LAS CIFRAS, no sólo las claves. Un sobre con `coverage` PRESENTE y en cero pasa el
		// chequeo de presencia y miente igual, que es el defecto que R7 vino a arreglar un salto
		// más abajo: declarar un techo que no es el del corpus. Medido en la verify de este
		// expediente: dos de los tres caminos de este custodio eran ciegos al VALOR, así que las
		// mutaciones que le pasaban un reporte vacío sobrevivían con el paquete entero en verde.
		var vuelta shadowJSON
		if err := json.Unmarshal(buf.Bytes(), &vuelta); err != nil {
			t.Fatalf("el sobre no deserializa de vuelta (%v); salida:\n%s", err, buf.String())
		}
		if vuelta.Coverage != cob {
			t.Errorf("la cobertura del sobre no es la que se le pasó:\n  salió    %+v\n  esperaba %+v",
				vuelta.Coverage, cob)
		}
		if len(vuelta.ByRelation) != len(res) {
			t.Fatalf("by_relation trajo %d filas, esperaba %d", len(vuelta.ByRelation), len(res))
		}
		// Comparación campo por campo y no sólo el largo: un array con la fila correcta en
		// cantidad y en cero en sus números es la misma mentira que el `coverage` vacío.
		if vuelta.ByRelation[0].HeurRelation != res[0].HeurRelation ||
			vuelta.ByRelation[0].Total != res[0].Total ||
			vuelta.ByRelation[0].Agreed != res[0].Agreed ||
			vuelta.ByRelation[0].Rate != res[0].Rate {
			t.Errorf("la fila de by_relation no es la que se le pasó:\n  salió    %+v\n  esperaba %+v",
				vuelta.ByRelation[0], res[0])
		}
	})

	t.Run("libro vacio: el techo que se imprime es el derivado", func(t *testing.T) {
		var buf bytes.Buffer
		// ⚠️ SLICE VACÍO NO-NIL, Y NO ES ESTILO: `ShadowAgreementByRelation` arranca con
		// `out := []ShadowAgreement{}` (internal/memory/shadow.go:113) y devuelve ESE slice
		// cuando no hay filas, así que un libro vacío NUNCA llega como nil al comando. Con `nil`
		// acá, mutar la guarda a `res == nil` dejaba este subtest VERDE mientras el camino de R7
		// pasaba a ser inalcanzable en el binario: el custodio seguía pasando y la conducta que
		// declara ya no existía. El fixture entra por la misma puerta que el comando o no mide.
		emitirShadow(&buf, cob, []memory.ShadowAgreement{}, false)
		salida := buf.String()
		if !strings.Contains(salida, "el techo son 4 pares") {
			t.Errorf("al libro vacío no le llegó la cobertura derivada; dijo:\n%s", salida)
		}
		if strings.Contains(salida, "no escribiría una sola fila") {
			t.Errorf("con techo 4 declaró techo CERO: le llegó un reporte vacío. Salida:\n%s", salida)
		}
	})

	t.Run("tabla: la cobertura va ANTES de las tasas", func(t *testing.T) {
		var buf bytes.Buffer
		emitirShadow(&buf, cob, res, false)
		salida := buf.String()
		iCob := strings.Index(salida, "Cobertura:")
		iTabla := strings.Index(salida, "VEREDICTO")
		if iCob < 0 {
			t.Fatalf("la tabla salió sin su denominador; salida:\n%s", salida)
		}
		if iTabla < 0 {
			t.Fatalf("no salió la tabla; salida:\n%s", salida)
		}
		// El ORDEN es parte de la afirmación, no una preferencia estética: con las tasas primero,
		// quien lee se forma la impresión con un «50 %» y recién después se entera del denominador.
		if iCob > iTabla {
			t.Errorf("la cobertura salió DESPUÉS de las tasas (%d > %d)", iCob, iTabla)
		}
		if !strings.Contains(salida, "supersedes") {
			t.Errorf("la tabla perdió su fila; salida:\n%s", salida)
		}
		// Las CIFRAS del denominador y de la tasa, por el mismo motivo que el sobre: «Cobertura:»
		// y «VEREDICTO» son ETIQUETAS y salen idénticas con un reporte en cero detrás, así que
		// buscarlas mide que la función se llamó y no que le llegó el dato.
		if !strings.Contains(salida, "4 de 9") {
			t.Errorf("la cobertura impresa no trae las cifras derivadas (4 de 9); salida:\n%s", salida)
		}
		// La tasa se imprime en PORCENTAJE: 0,5 tiene que salir «50», no «0». Sin esta línea, dejar
		// de multiplicar por 100 imprime «0%» en toda la tabla sin enrojecer nada.
		if !strings.Contains(salida, "50%") {
			t.Errorf("la tasa 0,5 no salió como 50%%; salida:\n%s", salida)
		}
	})
}

// baseConTechoPositivo siembra una base cuyo techo es POSITIVO y CONOCIDO: 4 pares medibles sobre 5
// re-detectables. El techo positivo es deliberado y no una comodidad —con techo cero las dos ramas
// del libro vacío dicen cosas distintas por motivos distintos, así que el custodio no podría
// distinguir «le llegó la cobertura» de «le llegó un cero»—, que es la misma trampa de la
// permutación que vació al custodio de R4 en este mismo expediente.
func baseConTechoPositivo(t *testing.T) string {
	t.Helper()
	dir := memtest.DirSembrado(t)
	e, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	par := func(src, tgt, relation, status, resolvedBy string) {
		for _, id := range []string{src, tgt} {
			if err := e.SaveObservationTyped(id, "cobertura/caller", "contenido de prueba para "+id,
				1.0, "", "shared", nil); err != nil {
				t.Fatalf("SaveObservationTyped(%s): %v", id, err)
			}
		}
		if _, err := e.UpsertObsRelation(memory.ObsRelation{
			SourceID: src, TargetID: tgt, Relation: relation,
			Status: status, ResolvedBy: resolvedBy, Confidence: 0.8,
		}); err != nil {
			t.Fatalf("UpsertObsRelation(%s->%s): %v", src, tgt, err)
		}
	}
	// 4 medibles: dos por PROPUESTA del detector y dos por su FIRMA.
	par("c-p1-src", "c-p1-tgt", memory.RelPending, memory.RelStatusPending, "")
	par("c-p2-src", "c-p2-tgt", memory.RelPending, memory.RelStatusPending, "")
	par("c-h1-src", "c-h1-tgt", memory.RelRelated, memory.RelStatusResolved, memory.RelResolvedByHeuristic)
	par("c-h2-src", "c-h2-tgt", memory.RelRelated, memory.RelStatusResolved, memory.RelResolvedByHeuristic)
	// 1 de firma ajena, para que Medibles y Redetectables NO sean el mismo número: si coincidieran,
	// una mutación que imprimiera el denominador en lugar del numerador pasaría verde.
	par("c-a1-src", "c-a1-tgt", memory.RelRelated, memory.RelStatusResolved, "agent")
	return dir
}

// salidaDe corre f con os.Stdout redirigido y devuelve lo que escribió.
//
// La lectura va en una goroutine A PROPÓSITO: un os.Pipe tiene buffer finito, y leer recién después
// de que f terminó funciona sólo mientras la salida quepa. Con la lectura concurrente el tamaño de
// la salida deja de ser una condición del test, que es lo que uno quiere de un andamio.
func salidaDe(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	leido := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		leido <- b
	}()
	viejo := os.Stdout
	os.Stdout = w
	f()
	os.Stdout = viejo
	w.Close()
	b := <-leido
	r.Close()
	return string(b)
}

// TestRunConflictsShadowCableaLaCoberturaALaSalida custodia la UNIÓN que ningún test alcanzaba:
// `runConflictsShadow` deriva la cobertura y se la pasa a `emitirShadow`, y ESE salto
// —cmd/musubi/conflicts.go:77— no tenía un solo caller en los tests. Medido en la verify de este
// expediente: cuatro mutaciones de esa línea sobrevivían con `go test ./cmd/musubi/` ENTERO en
// verde, y una de ellas —pasarle un reporte en cero al libro vacío— hace que el comando vuelva a
// decir «techo CERO» teniendo techo 4, que es EXACTAMENTE el defecto que R7 vino a arreglar. O sea
// que el defecto del expediente se podía reintroducir un salto más arriba del custodio que lo cuida.
//
// Ejerce el comando ENTERO sin tocar producción: `workspaceDir` mira MUSUBI_HOME antes que todo
// (cmd/musubi/setup.go:40), así que apuntarlo a un directorio sembrado hace que el comando abra ESA
// base y no la del repo. La alternativa era extraer una función de `runConflictsShadow` para poder
// llamarla, o sea cambiar producción para poder medirla; esto mide la que ya existe.
//
// Sabotaje que la hace fallar (1/3): llevarle una cobertura MUTILADA en vez de la derivada, que
// es el reporte en cero del defecto de R7. Conserva `cob.Medidas` a propósito y no por prolijidad:
// con `ShadowCoverageReport{}` pelado la variable `cob` queda sin usar, el paquete no COMPILA y el
// rojo lo firma el compilador en vez del custodio. Medido acá mismo: un sabotaje que no compila se
// lee idéntico a uno que funciona —rc=1 y ni un `--- FAIL`— y es la trampa que la skill llama
// «antes de concluir que rompió algo, verificá que el archivo haya cambiado de verdad».
// arnes: archivo="cmd/musubi/conflicts.go"
// arnes: de="emitirShadow(os.Stdout, cob, res, *asJSON)"
// arnes: a="emitirShadow(os.Stdout, memory.ShadowCoverageReport{Medidas: cob.Medidas}, res, *asJSON)"
// arnes: colision_ok="TestRunConflictsShadowCableaLaCoberturaALaSalida"
//
// Sabotaje que la hace fallar (2/3): descartar la salida, que deja el comando mudo con todo lo demás bien.
// arnes: archivo="cmd/musubi/conflicts.go"
// arnes: de="emitirShadow(os.Stdout, cob, res, *asJSON)"
// arnes: a="emitirShadow(io.Discard, cob, res, *asJSON)"
// arnes: colision_ok="TestRunConflictsShadowCableaLaCoberturaALaSalida"
//
// Sabotaje que la hace fallar (3/3): INVERTIR la bandera, que le da texto a quien pidió --json y
// JSON a quien pidió la tabla. Invertida y no clavada en `false` por lo mismo que arriba: con una
// constante, `asJSON` queda sin usar y el rojo es del compilador.
// arnes: archivo="cmd/musubi/conflicts.go"
// arnes: de="emitirShadow(os.Stdout, cob, res, *asJSON)"
// arnes: a="emitirShadow(os.Stdout, cob, res, !*asJSON)"
// arnes: colision_ok="TestRunConflictsShadowCableaLaCoberturaALaSalida"
func TestRunConflictsShadowCableaLaCoberturaALaSalida(t *testing.T) {
	t.Run("libro vacio: el techo que declara es el del corpus", func(t *testing.T) {
		t.Setenv("MUSUBI_HOME", baseConTechoPositivo(t))
		salida := salidaDe(t, func() { runConflictsShadow(nil) })

		if !strings.Contains(salida, "el techo son 4 pares") {
			t.Errorf("el comando no declaró el techo derivado; salida:\n%s", salida)
		}
		if strings.Contains(salida, "El techo de hoy es CERO") {
			t.Errorf("declaró techo CERO teniendo techo 4: no le llegó la cobertura. Salida:\n%s", salida)
		}
		if !strings.Contains(salida, "4 de 5") {
			t.Errorf("la cobertura no trae sus cifras (4 de 5); salida:\n%s", salida)
		}
	})

	t.Run("json: el sobre trae la cobertura derivada de la base", func(t *testing.T) {
		t.Setenv("MUSUBI_HOME", baseConTechoPositivo(t))
		salida := salidaDe(t, func() { runConflictsShadow([]string{"--json"}) })

		var sobre shadowJSON
		if err := json.Unmarshal([]byte(salida), &sobre); err != nil {
			t.Fatalf("el --json no emitió el sobre (%v); salida:\n%s", err, salida)
		}
		// Las CIFRAS, derivadas de la base sembrada: un sobre con las claves presentes y en cero
		// pasa cualquier chequeo de forma y miente igual.
		if sobre.Coverage.Medibles != 4 {
			t.Errorf("coverage.medibles = %d, esperaba 4 (sobre: %+v)", sobre.Coverage.Medibles, sobre.Coverage)
		}
		if sobre.Coverage.Redetectables != 5 {
			t.Errorf("coverage.redetectables = %d, esperaba 5 (sobre: %+v)", sobre.Coverage.Redetectables, sobre.Coverage)
		}
		if sobre.Coverage.FirmaAjena != 1 {
			t.Errorf("coverage.firma_ajena = %d, esperaba 1 (sobre: %+v)", sobre.Coverage.FirmaAjena, sobre.Coverage)
		}
	})

	t.Run("tabla: la cobertura va antes de las tasas y trae sus cifras", func(t *testing.T) {
		dir := baseConTechoPositivo(t)
		e, err := memory.NewDbEngine(dir)
		if err != nil {
			t.Fatalf("NewDbEngine: %v", err)
		}
		// Una fila medida, para que el comando tome el camino de la TABLA y no el del libro vacío.
		if err := e.SaveShadowVerdict(memory.ShadowVerdict{
			RelationID: "rel-caller", SourceID: "c-p1-src", TargetID: "c-p1-tgt",
			HeurRelation: memory.RelSupersedes, HeurStatus: memory.RelStatusResolved,
			JudgeRelation: memory.RelSupersedes, JudgeRaw: "supersedes", JudgeModel: "modelo-de-prueba",
		}); err != nil {
			t.Fatalf("SaveShadowVerdict: %v", err)
		}
		e.Close()

		t.Setenv("MUSUBI_HOME", dir)
		salida := salidaDe(t, func() { runConflictsShadow(nil) })

		iCob := strings.Index(salida, "Cobertura:")
		iTabla := strings.Index(salida, "VEREDICTO")
		if iCob < 0 {
			t.Fatalf("la tabla salió sin su denominador; salida:\n%s", salida)
		}
		if iTabla < 0 {
			t.Fatalf("no salió la tabla; salida:\n%s", salida)
		}
		if iCob > iTabla {
			t.Errorf("la cobertura salió DESPUÉS de las tasas (%d > %d); salida:\n%s", iCob, iTabla, salida)
		}
		if !strings.Contains(salida, "4 de 5") {
			t.Errorf("la cobertura no trae sus cifras (4 de 5); salida:\n%s", salida)
		}
		if !strings.Contains(salida, "supersedes") {
			t.Errorf("la tabla perdió su fila; salida:\n%s", salida)
		}
	})
}
