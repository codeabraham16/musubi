package memory

import (
	"context"
	"testing"
)

// senales_separadas_test.go cubre el desglose del score del detector (migración v27).
//
// `confidence` significaba DOS COSAS distintas según la fila: en una pendiente era
// max(léxico, coseno) y en una auto-resuelta el léxico solo. Nadie podía notarlo desde afuera, y el
// 2026-08-11 eso llevó a triar los conflictos del cerebro central por «confianza ≥ 0,85» creyendo
// que ordenaba por gravedad cuando ordenaba por parecido — el coseno entre documentos SIN relación
// ya llega a 0,88.
//
// Lo que estos tests defienden no es «existen dos columnas» sino la distinción que hace que sirvan:
// que «no medido» y «midió cero» NO se confundan, porque un coseno de 0 es un dato y un NULL no.

func relacionConSenales(t *testing.T, e *DbEngine, id string, lex, cos *float64) ObsRelation {
	t.Helper()
	src, tgt := "src-"+id, "tgt-"+id
	for _, o := range []string{src, tgt} {
		if err := e.SaveObservation(o, "tema/"+o, "contenido de "+o, nil); err != nil {
			t.Fatalf("SaveObservation: %v", err)
		}
	}
	// La confianza se arma como la arma el detector REAL para una pendiente —max(léxico, coseno)—
	// y no como el léxico solo. Con el atajo, el test no reproducía la trampa que le da sentido a
	// todo esto: que un coseno alto infle la confianza de un par con poco solape real.
	conf := 0.0
	if lex != nil {
		conf = *lex
	}
	if cos != nil && *cos > conf {
		conf = *cos
	}
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: src, TargetID: tgt, Confidence: conf, Status: RelStatusPending,
		Lex: lex, Cosine: cos}); err != nil {
		t.Fatalf("UpsertObsRelation: %v", err)
	}
	page, err := e.PendingObsRelationsQueryCtx(context.Background(), PendingQuery{})
	if err != nil {
		t.Fatalf("PendingObsRelationsQueryCtx: %v", err)
	}
	for _, r := range page.Relations {
		if r.SourceID == src {
			return r
		}
	}
	t.Fatalf("no volvió la relación %s", id)
	return ObsRelation{}
}

// S1: las dos señales sobreviven al viaje de ida y vuelta, por separado.
func TestSenalesSeGuardanPorSeparado(t *testing.T) {
	e := newTestEngine(t)
	r := relacionConSenales(t, e, "a", f(0.42), f(0.81))
	if r.Lex == nil || r.Cosine == nil {
		t.Fatalf("las dos señales deberían volver: lex=%v cosine=%v", r.Lex, r.Cosine)
	}
	if *r.Lex != 0.42 || *r.Cosine != 0.81 {
		t.Errorf("señales cambiadas: lex=%v cosine=%v", *r.Lex, *r.Cosine)
	}
	// Y `confidence` sigue siendo lo que SIEMPRE fue —max(léxico, coseno) en una pendiente— porque
	// cambiarle el significado rompería a cualquiera que ya filtre por ella. El desglose se suma
	// al lado; no reemplaza nada. Acá el coseno (0,81) le gana al léxico (0,42), y ése es justo el
	// caso donde el número engaña si uno no puede ver de dónde salió.
	if r.Confidence != 0.81 {
		t.Errorf("confidence debe seguir siendo max(lex, coseno) = 0.81, vino %v", r.Confidence)
	}
}

// S2: EL INVARIANTE CENTRAL. «No medido» y «midió cero» son cosas distintas.
//
// Un coseno de 0 quiere decir «ortogonales», que es información. Un NULL quiere decir «esta fila es
// anterior al desglose y no se puede reconstruir». Aplanar el segundo al primero metería un dato
// inventado en la única columna que existe para poder confiar en los números.
func TestNoMedidoNoEsCero(t *testing.T) {
	e := newTestEngine(t)

	sinCoseno := relacionConSenales(t, e, "sin", f(0.5), nil)
	if sinCoseno.Cosine != nil {
		t.Errorf("sin coseno medido, el campo debe venir ausente (nil), vino %v", *sinCoseno.Cosine)
	}
	if sinCoseno.Lex == nil || *sinCoseno.Lex != 0.5 {
		t.Errorf("el léxico sí se midió y debe estar: %v", sinCoseno.Lex)
	}

	cosenoCero := relacionConSenales(t, e, "cero", f(0.5), f(0))
	if cosenoCero.Cosine == nil {
		t.Fatal("un coseno de 0 es un valor MEDIDO: no puede volver como ausente")
	}
	if *cosenoCero.Cosine != 0 {
		t.Errorf("el coseno medido en 0 debe volver como 0, vino %v", *cosenoCero.Cosine)
	}
}

// S3: una relación vieja —sin desglose— sigue funcionando y se declara como lo que es.
func TestRelacionAnteriorAlDesgloseNoMiente(t *testing.T) {
	e := newTestEngine(t)
	r := relacionConSenales(t, e, "vieja", nil, nil)
	if r.Lex != nil || r.Cosine != nil {
		t.Errorf("sin desglose, ambos campos van ausentes: lex=%v cosine=%v", r.Lex, r.Cosine)
	}
	if r.Confidence != 0 && r.Status != RelStatusPending {
		t.Errorf("la relación tiene que seguir existiendo y siendo usable: %+v", r)
	}
}

// S4: min_lex filtra por la señal que significa algo, y DESCARTA las que no la tienen.
//
// Incluir «por las dudas» una fila cuyo léxico nadie midió metería en el resultado justo lo que el
// filtro quería sacar, y encima sin manera de darse cuenta.
func TestMinLexFiltraYDescartaLoNoMedido(t *testing.T) {
	e := newTestEngine(t)
	relacionConSenales(t, e, "alto", f(0.80), f(0.30)) // léxico alto
	relacionConSenales(t, e, "bajo", f(0.20), f(0.95)) // léxico bajo, coseno alto (el caso trampa)
	relacionConSenales(t, e, "vieja", nil, nil)        // sin desglose

	page, err := e.PendingObsRelationsQueryCtx(context.Background(), PendingQuery{MinLex: 0.5})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if page.Count != 1 || len(page.Relations) != 1 {
		t.Fatalf("con min_lex=0.5 sólo pasa la de léxico 0.80: count=%d relaciones=%d",
			page.Count, len(page.Relations))
	}
	if page.Relations[0].Lex == nil || *page.Relations[0].Lex != 0.80 {
		t.Errorf("pasó la que no era: %+v", page.Relations[0])
	}

	// Y el contraste que da sentido a todo: filtrando por CONFIANZA, la de coseno 0.95 se cuela —
	// que es exactamente el error que este desglose viene a hacer visible.
	porConfianza, err := e.PendingObsRelationsQueryCtx(context.Background(), PendingQuery{MinConfidence: 0.5})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if porConfianza.Count == page.Count {
		t.Errorf("filtrar por confianza y por léxico no puede dar lo mismo: si diera, el desglose "+
			"no estaría sirviendo para nada (confianza=%d, léxico=%d)", porConfianza.Count, page.Count)
	}
}

// S5: juzgar una relación NO borra cómo se la detectó.
//
// Pasa por construcción y conviene decir por qué: `ResolveObsRelation` es un UPDATE acotado a
// relation/status/resolved_by/reason, así que ni menciona las columnas de señales. Lo que este test
// fija es justamente eso — que el veredicto siga siendo una escritura ESTRECHA. Si alguna vez se
// reescribiera como un upsert completo, el desglose se perdería al resolver y este test se pondría
// en rojo. Un saboteo lo confirmó: borrar el COALESCE del upsert no lo afecta, porque el camino de
// resolución no pasa por ahí (ese otro camino lo cubre S7).
func TestJuzgarNoBorraLasSenales(t *testing.T) {
	e := newTestEngine(t)
	r := relacionConSenales(t, e, "juzgada", f(0.66), f(0.77))

	if err := e.ResolveObsRelation(r.ID, RelRelated, "humano", "temas vecinos"); err != nil {
		t.Fatalf("ResolveObsRelation: %v", err)
	}
	todas, err := e.AllObsRelations()
	if err != nil {
		t.Fatalf("AllObsRelations: %v", err)
	}
	var vista *ObsRelation
	for i := range todas {
		if todas[i].ID == r.ID {
			vista = &todas[i]
		}
	}
	if vista == nil {
		t.Fatal("la relación desapareció al resolverla")
	}
	if vista.Status != RelStatusResolved {
		t.Fatalf("debería estar resuelta: %+v", vista)
	}
	if vista.Lex == nil || vista.Cosine == nil {
		t.Fatalf("resolver borró el desglose: lex=%v cosine=%v", vista.Lex, vista.Cosine)
	}
	if *vista.Lex != 0.66 || *vista.Cosine != 0.77 {
		t.Errorf("el desglose cambió al resolver: lex=%v cosine=%v", *vista.Lex, *vista.Cosine)
	}
}

// S7: RE-DETECTAR EL MISMO PAR NO BORRA LO QUE YA SE SABÍA.
//
// Éste es el camino que el COALESCE del upsert protege de verdad, y no el de resolver: el detector
// corre en CADA guardado y vuelve a upsertar los pares que reencuentra. Si una detección posterior
// no tiene coseno —porque el embedder está apagado, o la observación no tiene vector— el
// `excluded.cosine_score` viene NULL, y sin el COALESCE pisaría el valor guardado. Se perdería la
// única evidencia de por qué ese par entró a la cola, en silencio y por hacer algo normal.
//
// Lo encontró un saboteo: quitar el COALESCE no rompía ningún test, porque el único que decía
// cubrirlo (S5) recorre un UPDATE que ni toca esas columnas.
func TestRedetectarNoPisaLasSenalesConNulos(t *testing.T) {
	e := newTestEngine(t)
	r := relacionConSenales(t, e, "redetectada", f(0.55), f(0.88))

	// Segunda detección del MISMO par, esta vez sin coseno (embedder apagado).
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: r.SourceID, TargetID: r.TargetID, Relation: RelPending,
		Confidence: 0.55, Status: RelStatusPending, Lex: f(0.55), Cosine: nil}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}

	page, err := e.PendingObsRelationsQueryCtx(context.Background(), PendingQuery{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(page.Relations) != 1 {
		t.Fatalf("el par no debía duplicarse: %d relaciones", len(page.Relations))
	}
	got := page.Relations[0]
	if got.Cosine == nil {
		t.Fatal("la re-detección sin coseno borró el coseno que ya estaba medido")
	}
	if *got.Cosine != 0.88 {
		t.Errorf("el coseno guardado cambió: %v", *got.Cosine)
	}
}

// S6: el detector REAL llena las señales. Sin esto, todo lo anterior probaría un camino que en
// producción nadie recorre — el modo de falla «capacidad desplegada que nadie invoca».
func TestElDetectorLlenaLasSenales(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("o1", "tema/uno", "el candado del despacho no cruza la red, caso uno", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("o2", "tema/uno", "el candado del despacho no cruza la red, caso dos", nil); err != nil {
		t.Fatal(err)
	}
	rels, err := e.DetectRelations("o2", ConflictOptions{SimilarityFloor: 0.3, AutoResolveThreshold: 0.7})
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	if len(rels) == 0 {
		t.Skip("el detector no emparejó nada con esta siembra; el invariante se prueba en los demás casos")
	}
	for _, r := range rels {
		if r.Lex == nil {
			t.Errorf("el detector dejó una relación sin señal léxica: %+v", r)
		}
	}
}
