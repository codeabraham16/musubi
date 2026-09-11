package recalleval

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"sort"
	"testing"

	"musubi/internal/embedding"

	_ "modernc.org/sqlite"
)

// ¿UN VECTOR POR OBSERVACIÓN ALCANZA, O HAY QUE PARTIR EN TROZOS?
//
// RESPUESTA CORTA (2026-09-11): NO SE PUDO DEMOSTRAR QUE PARTIR COMPRE NADA, y el camino hasta ahí
// tiene dos hipótesis mías refutadas por medición. Este archivo conserva las tres mediciones porque
// la refutación es el resultado.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL MECANISMO ES REAL: LA DILUCIÓN EXISTE
//
// El proveedor estático (POTION) embebe con una MEDIA SIMPLE de los vectores de todos los tokens
// (internal/embedding/static.go: acumula, divide por n, normaliza L2). Sin atención, sin peso
// posicional, sin truncado. El corpus lo invita: p50=2.464 caracteres, p90=4.725, máximo 57.996,
// con el 65 % por encima de 2.000.
//
// Coseno medio al centroide del corpus, sobre los 1.975 vectores guardados:
//
//	0–1.000 car.  0,6263    3.000–5.000   0,8337
//	1.000–2.000   0,7458    5.000–10.000  0,8534
//	2.000–3.000   0,8081    10.000+       0,8721
//
// Monótono en los seis tramos. Los documentos largos SÍ convergen al centroide.
//
// ── HIPÓTESIS 1, REFUTADA: «el documento largo pierde su información interna» ────────────────
//
// Medido doc-contra-doc (mismo topic vs ajenos), la separación NO se achica con el largo: 0,1288 /
// 0,1325 / 0,1260 / 0,1463 / 0,1544 de corto a largo. Todo sube junto hacia el centroide y la RESTA
// sobrevive. Documentos del mismo tema se siguen pareciendo entre sí a cualquier largo.
//
// ── HIPÓTESIS 2, TAMBIÉN REFUTADA: «la consulta corta no llega al documento largo» ───────────
//
// Es lo que mide TestUnaConsultaCortaContraDocumentosLargos, y el número parecía contundente: por
// encima de 1.000 caracteres la separación cae a ~0, y entre 1.000 y 3.000 es NEGATIVA.
//
// Si la causa fuera el largo, cortar el documento a 1.000 caracteres —donde la separación sí
// existe— tendría que recuperarla. NO LA RECUPERA (TestElPrimerTrozoDevuelveLaSeparacion):
//
//	banda        doc entero   primer trozo de 1.000
//	1.000–3.000    -0,0145         -0,0157
//	3.000+         +0,0175         +0,0237
//
// El remedio propuesto, aplicado directamente, no mueve nada. La causa no es el largo.
//
// ── LO QUE SÍ EXPLICA LOS DATOS, Y POR QUÉ ESTE BANCO NO PUEDE CERRARLO ──────────────────────
//
// Los documentos naturalmente cortos separan porque SU TEXTO SE PARECE A SU PROPIA ETIQUETA: la
// consulta sale de `ConsultaDesdeTopico`, o sea de las palabras del `topic_key`. En una nota de 400
// caracteres esas palabras son una fracción grande del texto; en una de 2.500 son una línea.
//
// O sea que esto mide SOLAPAMIENTO ETIQUETA-TEXTO, no calidad de recuperación. Es el mismo techo
// que ya frenó el veredicto sobre MMR y sobre el peso léxico: en este banco las consultas Y las
// etiquetas salen las dos del `topic_key`. Tres items del plan, bloqueados por lo mismo.
//
// LO QUE LO DESTRABARÍA son consultas REALES con relevancia ELEGIDA. La mitad de la relevancia ya
// se empezó a registrar (`EtiquetadoPorExpansion`, v57). La mitad de las consultas no existe: el
// sistema no guarda las consultas del recall, y guardarlas es una decisión de privacidad, no de
// ingeniería.
//
// ── LO QUE QUEDA EN PIE, Y NO ES POCO ────────────────────────────────────────────────────────
//
// Una cosa SÍ quedó medida por dos caminos independientes: la señal vectorial aporta poco. El
// barrido de pesos (pesos_real_test.go) dice que `vector=0.5` rinde MÁS que `vector=1.0`, y que
// apagarla del todo cuesta -0,0264 de nDCG mientras apagar el léxico cuesta -0,1617. Acá se ve el
// otro lado del mismo hecho. Dos mediciones distintas coincidiendo es la única forma de creerle a
// una.

type docConVector struct {
	id    string
	topic string
	largo int
	vec   []float32
}

func leerVectores(t *testing.T, ruta string) []docConVector {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+ruta+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	filas, err := db.Query(`
		SELECT o.id, COALESCE(o.topic_key,''), length(o.content), e.vector
		FROM observations o JOIN embeddings e ON e.observation_id = o.id
		WHERE COALESCE(o.archived,0)=0 AND o.superseded_by IS NULL AND COALESCE(o.quarantined,0)=0
		  AND COALESCE(o.topic_key,'') != ''
		ORDER BY o.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer filas.Close()
	var out []docConVector
	for filas.Next() {
		var d docConVector
		var blob []byte
		if err := filas.Scan(&d.id, &d.topic, &d.largo, &blob); err != nil {
			t.Fatal(err)
		}
		d.vec = make([]float32, len(blob)/4)
		for i := range d.vec {
			d.vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
		}
		out = append(out, d)
	}
	return out
}

func coseno(a, b []float32) float64 {
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

// TestUnaConsultaCortaContraDocumentosLargos mide la SEPARACIÓN que consigue una consulta corta:
// cuánto más se parece a los documentos de su propio tema que a los ajenos, por tramo de largo.
//
// Si la separación se derrumba con el largo, partir en trozos compra algo concreto. Si se sostiene,
// la dilución es real pero no muerde, y partir sería resolver un problema que no existe — que es
// exactamente lo que este repo pide verificar antes de construir.
func TestUnaConsultaCortaContraDocumentosLargos(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	dirP := os.Getenv("MUSUBI_POTION_DIR")
	if ruta == "" || dirP == "" {
		t.Skip("faltan MUSUBI_FIXTURE_DB / MUSUBI_POTION_DIR")
	}
	prov, err := embedding.NewStaticProvider(dirP)
	if err != nil {
		t.Fatal(err)
	}
	docs := leerVectores(t, ruta)
	if len(docs) < 100 {
		t.Fatalf("sólo %d documentos con vector: muy pocos para medir", len(docs))
	}

	porTopico := map[string][]int{}
	for i, d := range docs {
		porTopico[d.topic] = append(porTopico[d.topic], i)
	}
	temas := make([]string, 0, len(porTopico))
	for tp, idxs := range porTopico {
		if len(idxs) >= 2 { // un tema de uno no tiene con qué comparar
			temas = append(temas, tp)
		}
	}
	sort.Strings(temas) // determinismo: el orden del mapa no puede mover el resultado
	if len(temas) < 20 {
		t.Fatalf("sólo %d temas con 2+ documentos: la medición no diría nada", len(temas))
	}
	t.Logf("%d documentos con vector · %d temas con 2+ documentos", len(docs), len(temas))

	type tramo struct {
		lo, hi  int
		propios []float64
		ajenos  []float64
	}
	tramos := []*tramo{{lo: 0, hi: 1000}, {lo: 1000, hi: 2000}, {lo: 2000, hi: 3000},
		{lo: 3000, hi: 5000}, {lo: 5000, hi: 1 << 30}}
	ubicar := func(l int) *tramo {
		for _, tr := range tramos {
			if l >= tr.lo && l < tr.hi {
				return tr
			}
		}
		return nil
	}

	rng := rand.New(rand.NewSource(7)) // semilla fija: la muestra de ajenos no puede cambiar entre corridas
	for _, tp := range temas {
		// La MISMA transformación que usa el fixture para fabricar consultas, no una propia: si
		// acá se inventara otra, esto mediría un recall que no existe.
		qv, err := prov.Embed(context.Background(), ConsultaDesdeTopico(tp))
		if err != nil {
			t.Fatal(err)
		}
		propios := map[int]bool{}
		for _, i := range porTopico[tp] {
			propios[i] = true
			if tr := ubicar(docs[i].largo); tr != nil {
				tr.propios = append(tr.propios, coseno(qv, docs[i].vec))
			}
		}
		// Ajenos: una muestra del resto, para que el «se parece a cualquier cosa» tenga la misma
		// composición por largo que el universo.
		for n := 0; n < 40; n++ {
			i := rng.Intn(len(docs))
			if propios[i] {
				continue
			}
			if tr := ubicar(docs[i].largo); tr != nil {
				tr.ajenos = append(tr.ajenos, coseno(qv, docs[i].vec))
			}
		}
	}

	media := func(xs []float64) float64 {
		if len(xs) == 0 {
			return 0
		}
		var s float64
		for _, x := range xs {
			s += x
		}
		return s / float64(len(xs))
	}

	t.Logf("%-18s %7s %7s  %9s %9s %12s", "largo del doc", "n prop", "n ajen", "propios", "ajenos", "SEPARACIÓN")
	for _, tr := range tramos {
		if len(tr.propios) < 10 {
			continue
		}
		p, a := media(tr.propios), media(tr.ajenos)
		hi := "∞"
		if tr.hi < 1<<29 {
			hi = itoa(tr.hi)
		}
		t.Logf("  %6d – %-9s %7d %7d  %9.4f %9.4f %12.4f", tr.lo, hi, len(tr.propios), len(tr.ajenos), p, a, p-a)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestElPrimerTrozoDevuelveLaSeparacion APLICA EL REMEDIO Y MIDE. Es la prueba que refutó la
// hipótesis 2, y por eso vale más que las otras dos juntas.
//
// El razonamiento era: si la separación existe por debajo de 1.000 caracteres y se pierde por
// encima, entonces embeber sólo el PRIMER TROZO de 1.000 de un documento largo debería
// recuperarla. Eso es exactamente lo que haría el troceo, en su forma más simple.
//
// No la recupera. Los dos números quedan casi idénticos, así que la causa no es el largo — y un
// troceo construido sobre esa premisa habría sido trabajo grande resolviendo un problema que no
// existe. Correr el remedio antes de construirlo cuesta una tarde; construirlo cuesta un track.
//
// Es caro (re-embebe todo el corpus dos veces, ~17 min) y por eso vive detrás del skip, como el
// resto de este paquete.
func TestElPrimerTrozoDevuelveLaSeparacion(t *testing.T) {
	ruta, dirP := os.Getenv("MUSUBI_FIXTURE_DB"), os.Getenv("MUSUBI_POTION_DIR")
	if ruta == "" || dirP == "" {
		t.Skip("faltan MUSUBI_FIXTURE_DB / MUSUBI_POTION_DIR")
	}
	prov, err := embedding.NewStaticProvider(dirP)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+ruta+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	filas, err := db.Query(`SELECT COALESCE(o.topic_key,''), o.content FROM observations o
		WHERE COALESCE(o.archived,0)=0 AND o.superseded_by IS NULL AND COALESCE(o.quarantined,0)=0
		  AND COALESCE(o.topic_key,'') != '' ORDER BY o.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer filas.Close()

	type parDeVectores struct {
		topic         string
		entero, trozo []float32
		largo         int
	}
	var docs []parDeVectores
	porTopico := map[string][]int{}
	for filas.Next() {
		var tp, cont string
		if err := filas.Scan(&tp, &cont); err != nil {
			t.Fatal(err)
		}
		r := []rune(cont)
		corte := r
		if len(corte) > 1000 {
			corte = corte[:1000] // el tamaño donde la separación TODAVÍA existe
		}
		ve, err := prov.Embed(context.Background(), cont)
		if err != nil {
			t.Fatal(err)
		}
		vt, err := prov.Embed(context.Background(), string(corte))
		if err != nil {
			t.Fatal(err)
		}
		porTopico[tp] = append(porTopico[tp], len(docs))
		docs = append(docs, parDeVectores{topic: tp, entero: ve, trozo: vt, largo: len(r)})
	}

	temas := []string{}
	for tp, ix := range porTopico {
		if len(ix) >= 2 {
			temas = append(temas, tp)
		}
	}
	sort.Strings(temas) // el orden del mapa no puede mover el resultado
	if len(temas) < 20 {
		t.Fatalf("sólo %d temas con 2+ documentos: no estoy midiendo nada", len(temas))
	}

	type acumulado struct{ pe, ae, pt, at []float64 }
	bandas := map[string]*acumulado{"1000-3000": {}, "3000+": {}}
	banda := func(l int) string {
		switch {
		case l < 1000:
			return "" // acá la separación ya existe: no hay nada que remediar
		case l < 3000:
			return "1000-3000"
		default:
			return "3000+"
		}
	}
	rng := rand.New(rand.NewSource(7))
	for _, tp := range temas {
		qv, err := prov.Embed(context.Background(), ConsultaDesdeTopico(tp))
		if err != nil {
			t.Fatal(err)
		}
		propio := map[int]bool{}
		for _, i := range porTopico[tp] {
			propio[i] = true
			if b := banda(docs[i].largo); b != "" {
				bandas[b].pe = append(bandas[b].pe, coseno(qv, docs[i].entero))
				bandas[b].pt = append(bandas[b].pt, coseno(qv, docs[i].trozo))
			}
		}
		for n := 0; n < 40; n++ {
			i := rng.Intn(len(docs))
			if propio[i] {
				continue
			}
			if b := banda(docs[i].largo); b != "" {
				bandas[b].ae = append(bandas[b].ae, coseno(qv, docs[i].entero))
				bandas[b].at = append(bandas[b].at, coseno(qv, docs[i].trozo))
			}
		}
	}
	med := func(xs []float64) float64 {
		if len(xs) == 0 {
			return 0
		}
		var s float64
		for _, x := range xs {
			s += x
		}
		return s / float64(len(xs))
	}
	t.Logf("%-12s %6s  %-22s %-22s", "banda", "n", "SEPARACIÓN doc entero", "SEPARACIÓN primer trozo")
	for _, b := range []string{"1000-3000", "3000+"} {
		a := bandas[b]
		t.Logf("  %-10s %6d  %-22.4f %-22.4f", b, len(a.pe), med(a.pe)-med(a.ae), med(a.pt)-med(a.at))
	}
}
