package memory

import "testing"

// Dedup SEMÁNTICO (M1/Q4 + M2). La restricción que manda: los embeddings estáticos NO evalúan
// predicados ("usamos X" y "ya NO usamos X" tienen coseno alto), así que el coseno NUNCA puede
// auto-ocultar memoria. De ahí el AND-gate: auto-resolver exige léxico Y coseno altos.

func testOpts() ConflictOptions {
	return ConflictOptions{
		SimilarityFloor:      0.3,
		AutoResolveThreshold: 0.7,
		CandidatePool:        10,
		CosineFloor:          0.85,
		CosineAutoThreshold:  0.90,
	}.withDefaults()
}

func f(v float64) *float64 { return &v }

// src más nueva y mismo topic: es la configuración que HOY auto-supersede si el léxico es alto.
func supersedePair() (obsRow, obsRow) {
	src := obsRow{id: "new", topicKey: "t", content: "a", createdAt: "2026-01-02"}
	cand := obsRow{id: "old", topicKey: "t", content: "b", createdAt: "2026-01-01"}
	return src, cand
}

// D.c — un casi-duplicado REAL (léxico alto + coseno que corrobora) sigue auto-resolviendo.
// Los casi-duplicados medidos en la base real dan coseno ~0.99, así que el AND-gate no los rompe.
func TestGateAutoResolveSurvivesWhenBothSignalsAgree(t *testing.T) {
	src, cand := supersedePair()
	rel := decideRelation(src, cand, 0.9, f(0.99), testOpts())
	if rel.Relation != RelSupersedes || rel.Status != RelStatusResolved {
		t.Errorf("léxico 0.9 + coseno 0.99 debe auto-resolver supersedes, obtuve %s/%s", rel.Relation, rel.Status)
	}
}

// D.b — AND-gate: léxico alto pero el coseno NO corrobora ⇒ DEGRADA a pending, no auto-suprime.
func TestGateDegradesWhenCosineDoesNotCorroborate(t *testing.T) {
	src, cand := supersedePair()
	rel := decideRelation(src, cand, 0.9, f(0.5), testOpts())
	if rel.Status != RelStatusPending {
		t.Errorf("léxico alto + coseno bajo NO debe auto-resolver: obtuve %s/%s", rel.Relation, rel.Status)
	}
	if rel.Relation == RelSupersedes && rel.Status == RelStatusResolved {
		t.Error("una auto-supresión acá ocultaría memoria sin corroboración semántica")
	}
}

// D.a — EL FALSO NEGATIVO QUE SE CIERRA: duplicado semántico (mismo significado, otras palabras).
// Léxico por debajo del piso, coseno alto. Hoy esto se IGNORA por completo; ahora va a pending.
func TestGateSurfacesSemanticDuplicateAsPending(t *testing.T) {
	opts := testOpts()
	src, cand := supersedePair()
	lex, cos := 0.05, f(0.93) // otras palabras, mismo significado

	if !relevantPair(lex, cos, opts) {
		t.Fatal("un duplicado semántico (léxico bajo, coseno alto) DEBE entrar al pool; hoy se descarta en silencio")
	}
	rel := decideRelation(src, cand, lex, cos, opts)
	if rel.Status != RelStatusPending {
		t.Errorf("el duplicado semántico debe quedar pending para que lo juzgue el agente, obtuve %s/%s", rel.Relation, rel.Status)
	}
	// Nunca auto-resolver por coseno solo: sería auto-ocultar por parecido de tema.
	if rel.Status == RelStatusResolved {
		t.Error("el coseno NO puede auto-resolver solo (no evalúa predicados: negación ⇒ coseno alto)")
	}
}

// R8 — por debajo de AMBOS pisos, el par se ignora (no entra al pool).
func TestGateIgnoresPairBelowBothFloors(t *testing.T) {
	if relevantPair(0.1, f(0.4), testOpts()) {
		t.Error("léxico bajo + coseno bajo no debe generar relación")
	}
}

// D.d / R7 — sin coseno (embedder apagado o sin vector) el veredicto es el LÉXICO histórico.
func TestGateWithoutCosineIsHistorical(t *testing.T) {
	opts := testOpts()
	src, cand := supersedePair()

	// Léxico alto ⇒ auto-supersede, igual que siempre (el coseno no está para exigir corroboración).
	if rel := decideRelation(src, cand, 0.9, nil, opts); rel.Relation != RelSupersedes || rel.Status != RelStatusResolved {
		t.Errorf("sin coseno, léxico alto debe auto-resolver como siempre: %s/%s", rel.Relation, rel.Status)
	}
	// Léxico medio ⇒ pending, igual que siempre.
	if rel := decideRelation(src, cand, 0.5, nil, opts); rel.Status != RelStatusPending {
		t.Errorf("sin coseno, léxico medio debe ser pending: %s/%s", rel.Relation, rel.Status)
	}
	// Léxico bajo ⇒ ni siquiera entra al pool.
	if relevantPair(0.1, nil, opts) {
		t.Error("sin coseno, léxico bajo no debe generar relación")
	}
}

// R9 — cosine_floor = 0 apaga el coseno: interruptor de rollback por config.
func TestCosineFloorZeroDisablesCosine(t *testing.T) {
	opts := ConflictOptions{CosineFloor: 0}.withDefaults()
	if opts.cosineEnabled() {
		t.Error("cosine_floor = 0 debe apagar el coseno (rollback al dedup léxico histórico)")
	}
}

// M2 END-TO-END: el pool SEMÁNTICO encuentra el duplicado escrito con OTRAS palabras. Con el pool
// sólo-FTS de antes, estas dos observaciones no comparten casi vocabulario ⇒ la candidata nunca
// entraba al pool ⇒ el duplicado era INVISIBLE. Ahora entra por coseno y queda pending.
func TestDetectRelationsFindsSemanticDuplicateViaVectorPool(t *testing.T) {
	e := newTestEngine(t)
	e.SetVectorModelID("static:tabla@abc")

	// Vectores casi paralelos (coseno ~0.99) pero textos con vocabulario distinto (Jaccard bajo).
	if err := e.SaveObservation("orig", "infra", "usamos NordVPN en la laptop del trabajo", []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("dupe", "infra", "la maquina corre un tunel cifrado comercial", []float32{0.99, 0.14, 0}); err != nil {
		t.Fatal(err)
	}

	// Precondición: léxicamente NO se parecen (por eso hoy es invisible).
	if lex := Similarity(
		"usamos NordVPN en la laptop del trabajo",
		"la maquina corre un tunel cifrado comercial",
	); lex >= 0.3 {
		t.Fatalf("precondición: los textos deberían ser léxicamente distintos, Jaccard=%.2f", lex)
	}

	rels, err := e.DetectRelations("dupe", testOpts())
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}

	var found *ObsRelation
	for i := range rels {
		if rels[i].TargetID == "orig" {
			found = &rels[i]
		}
	}
	if found == nil {
		t.Fatal("el pool semántico debía traer el duplicado por sinonimia; sin él es un falso negativo silencioso")
	}
	if found.Status != RelStatusPending {
		t.Errorf("el duplicado semántico debe quedar PENDING (lo juzga el agente), obtuve %s/%s", found.Relation, found.Status)
	}
}

// Sin vectores, DetectRelations no debe encontrar nada acá: es exactamente el comportamiento
// histórico (el falso negativo) y demuestra que el test de arriba mide el pool semántico, no otra cosa.
func TestDetectRelationsMissesSemanticDuplicateWithoutVectors(t *testing.T) {
	e := newTestEngine(t) // sin SetVectorModelID ⇒ sin coseno
	if err := e.SaveObservation("orig", "infra", "usamos NordVPN en la laptop del trabajo", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("dupe", "infra", "la maquina corre un tunel cifrado comercial", nil); err != nil {
		t.Fatal(err)
	}
	rels, err := e.DetectRelations("dupe", testOpts())
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	for _, r := range rels {
		if r.TargetID == "orig" {
			t.Fatal("sin vectores el camino léxico NO puede ver este duplicado: el test de arriba estaría midiendo otra cosa")
		}
	}
}

// D.e / R0 — EL INVARIANTE, como property test sobre una grilla densa: para CUALQUIER par
// (lex, cos), el gate con coseno NUNCA produce una auto-supresión que el gate léxico-puro no
// produjera. Es lo que garantiza que agregar semántica no pueda hacer desaparecer memoria.
func TestR0CosineNeverCreatesNewAutoSuppression(t *testing.T) {
	opts := testOpts()
	src, cand := supersedePair() // la configuración que MÁS fácil auto-suprime

	autoSuppresses := func(rel ObsRelation) bool {
		return rel.Relation == RelSupersedes && rel.Status == RelStatusResolved
	}

	for lexI := 0; lexI <= 100; lexI++ {
		lex := float64(lexI) / 100
		historical := autoSuppresses(decideRelation(src, cand, lex, nil, opts))

		for cosI := 0; cosI <= 100; cosI++ {
			cos := float64(cosI) / 100
			withCosine := autoSuppresses(decideRelation(src, cand, lex, &cos, opts))

			if withCosine && !historical {
				t.Fatalf("R0 VIOLADO: con lex=%.2f cos=%.2f el coseno CREÓ una auto-supresión que el gate léxico no hacía", lex, cos)
			}
		}
	}
}
