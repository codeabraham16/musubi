package memory

import "testing"

// relations_backfill_test.go defiende las dos propiedades que hacen que el backfill sirva para
// calibrar: que el lex que escribe sea EXACTAMENTE el de producción, y que jamás pise un número
// que el detector ya había medido sobre el contenido de aquel momento.

// leerDesglose lee las dos columnas crudas. Va directo a SQL y no por PendingObsRelationsQueryCtx
// porque el backfill toca también relaciones RESUELTAS, que esa consulta no devuelve.
func leerDesglose(t *testing.T, e *DbEngine, relID string) (lex, cos *float64) {
	t.Helper()
	if err := e.db.QueryRow(
		`SELECT lex_score, cosine_score FROM observation_relations WHERE id = ?`, relID,
	).Scan(&lex, &cos); err != nil {
		t.Fatalf("leer desglose de %s: %v", relID, err)
	}
	return lex, cos
}

// B1: una relación sin desglose lo recibe, y el número es el de la función real.
func TestBackfillRellenaElDesgloseQueFaltaba(t *testing.T) {
	e := newTestEngine(t)
	r := relacionConSenales(t, e, "a", nil, nil)

	res, err := e.BackfillRelationScores(BackfillScoresOptions{})
	if err != nil {
		t.Fatalf("BackfillRelationScores: %v", err)
	}
	if res.Scanned != 1 || res.LexFilled != 1 {
		t.Fatalf("debería haber rellenado 1: %+v", res)
	}

	lex, cos := leerDesglose(t, e, r.ID)
	if lex == nil {
		t.Fatal("el lex sigue en NULL después del backfill")
	}
	// Sin embedder no hay coseno, y eso NO es un fallo: es el camino léxico histórico. Que quede
	// en NULL —y no en 0— es justamente la distinción que la tabla existe para conservar.
	if cos != nil {
		t.Errorf("sin embedder no debería haber coseno, vino %v", *cos)
	}
	esperado := Similarity("contenido de src-a", "contenido de tgt-a")
	if *lex != esperado {
		t.Errorf("el lex escrito (%v) no es el de Similarity (%v): la calibración mediría otra cosa", *lex, esperado)
	}
	if res.NoVector != 1 {
		t.Errorf("el par sin vector debería contarse como tal: %+v", res)
	}
}

// B2: lo ya medido no se pisa. Un score viejo describe el contenido de ENTONCES; el recomputado,
// el de hoy. Reemplazarlo destruiría la única medición fiel del par.
func TestBackfillNoPisaLoYaMedido(t *testing.T) {
	e := newTestEngine(t)
	r := relacionConSenales(t, e, "b", f(0.42), nil)

	if _, err := e.BackfillRelationScores(BackfillScoresOptions{}); err != nil {
		t.Fatalf("BackfillRelationScores: %v", err)
	}

	lex, _ := leerDesglose(t, e, r.ID)
	if lex == nil {
		t.Fatal("el lex medido se perdió: quedó en NULL, debía seguir en 0.42")
	}
	if *lex != 0.42 {
		t.Fatalf("el lex medido se pisó: quedó %v, debía seguir en 0.42", *lex)
	}
}

// B3: si alguna de las dos puntas dejó de ser visible, el par no entra. Calibrar umbrales con
// pares donde una observación ya está archivada o supersedida sería calibrar contra memoria que
// el sistema decidió no mostrar más.
func TestBackfillIgnoraLosParesConUnaPuntaInvisible(t *testing.T) {
	e := newTestEngine(t)
	r := relacionConSenales(t, e, "c", nil, nil)
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = ?`, "tgt-c"); err != nil {
		t.Fatalf("archivar el target: %v", err)
	}

	res, err := e.BackfillRelationScores(BackfillScoresOptions{})
	if err != nil {
		t.Fatalf("BackfillRelationScores: %v", err)
	}
	if res.Scanned != 0 {
		t.Errorf("no debería haber escaneado nada: %+v", res)
	}
	if lex, _ := leerDesglose(t, e, r.ID); lex != nil {
		t.Errorf("se le escribió desglose a un par con una punta invisible: %v", *lex)
	}
}

// B3b: `Limit` cuenta pares ÚTILES, no filas leídas. Es la razón por la que el filtro de
// visibilidad está también en el SELECT y no sólo en loadObsRow: sin él, un límite chico sobre una
// base con memoria archivada gastaría el cupo en descartes y el backfill parecería no avanzar.
// El par invisible se fuerza más nuevo para que el ORDER BY lo ponga primero, que es el caso malo.
func TestBackfillLimitNoGastaElCupoEnDescartes(t *testing.T) {
	e := newTestEngine(t)
	invisible := relacionConSenales(t, e, "f", nil, nil)
	visible := relacionConSenales(t, e, "g", nil, nil)
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = ?`, "tgt-f"); err != nil {
		t.Fatalf("archivar el target: %v", err)
	}
	for id, ts := range map[string]string{invisible.ID: "2030-01-01", visible.ID: "2020-01-01"} {
		if _, err := e.db.Exec(`UPDATE observation_relations SET updated_at = ? WHERE id = ?`, ts, id); err != nil {
			t.Fatalf("fijar updated_at: %v", err)
		}
	}

	res, err := e.BackfillRelationScores(BackfillScoresOptions{Limit: 1})
	if err != nil {
		t.Fatalf("BackfillRelationScores: %v", err)
	}
	if res.Scanned != 1 {
		t.Fatalf("el cupo de 1 se gastó en el par invisible: %+v", res)
	}
	if lex, _ := leerDesglose(t, e, visible.ID); lex == nil {
		t.Error("el par visible se quedó sin desglose: el límite lo consumió un descarte")
	}
}

// B4: el ensayo cuenta lo mismo que escribiría, y no escribe. Es lo que permite mirar el alcance
// del backfill sobre una base de producción antes de tocarla.
func TestBackfillDryRunCuentaSinEscribir(t *testing.T) {
	e := newTestEngine(t)
	r := relacionConSenales(t, e, "d", nil, nil)

	seco, err := e.BackfillRelationScores(BackfillScoresOptions{DryRun: true})
	if err != nil {
		t.Fatalf("BackfillRelationScores (dry-run): %v", err)
	}
	if lex, _ := leerDesglose(t, e, r.ID); lex != nil {
		t.Fatalf("el dry-run escribió: %v", *lex)
	}

	mojado, err := e.BackfillRelationScores(BackfillScoresOptions{})
	if err != nil {
		t.Fatalf("BackfillRelationScores: %v", err)
	}
	if seco.Scanned != mojado.Scanned || seco.LexFilled != mojado.LexFilled {
		t.Errorf("el ensayo prometió %+v y la corrida hizo %+v", seco, mojado)
	}
}

// B5: correrlo de nuevo no ESCRIBE nada, aunque vuelva a mirar. La distinción importa: sin
// embedder el coseno queda en NULL para siempre y el par sigue matcheando el filtro, así que la
// promesa que se puede sostener es «no reescribe», no «no recorre». Lo que rompería de verdad es
// que la segunda corrida volviera a estampar un lex —ahí un backfill agendado iría pisando el
// desglose cada vez que el contenido cambiara un poco.
func TestBackfillNoReescribeEnLaSegundaCorrida(t *testing.T) {
	e := newTestEngine(t)
	r := relacionConSenales(t, e, "e", nil, nil)

	if _, err := e.BackfillRelationScores(BackfillScoresOptions{}); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	primerLex, _ := leerDesglose(t, e, r.ID)

	segunda, err := e.BackfillRelationScores(BackfillScoresOptions{})
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}
	if segunda.LexFilled != 0 || segunda.CosineFilled != 0 {
		t.Errorf("la segunda corrida volvió a escribir: %+v", segunda)
	}
	segundoLex, _ := leerDesglose(t, e, r.ID)
	if primerLex == nil || segundoLex == nil || *primerLex != *segundoLex {
		t.Errorf("el desglose cambió entre corridas: %v -> %v", primerLex, segundoLex)
	}
}
