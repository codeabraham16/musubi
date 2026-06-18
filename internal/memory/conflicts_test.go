package memory

import (
	"context"
	"testing"
)

// saveAt guarda una observación con un created_at explícito (para controlar quién
// es "más nuevo" en los tests de supersede).
func saveAt(t *testing.T, e *DbEngine, id, topic, content, createdAt string) {
	t.Helper()
	if err := e.SaveObservation(id, topic, content, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET created_at=? WHERE id=?`, createdAt, id); err != nil {
		t.Fatal(err)
	}
}

func TestDetectRelationsSupersedeMismoTopic(t *testing.T) {
	e := newTestEngine(t)
	// Vieja y nueva casi idénticas, mismo topic: la nueva debe superseder a la vieja.
	saveAt(t, e, "old", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema.", "2026-01-01 10:00:00")
	saveAt(t, e, "new", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema productivo.", "2026-06-01 10:00:00")

	rels, err := e.DetectRelations("new", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("se esperaba al menos una relación detectada")
	}
	var sup *ObsRelation
	for i := range rels {
		if rels[i].Relation == RelSupersedes {
			sup = &rels[i]
		}
	}
	if sup == nil {
		t.Fatalf("se esperaba un supersede auto-resuelto, obtuve %+v", rels)
	}
	if sup.SourceID != "new" || sup.TargetID != "old" {
		t.Errorf("la nueva (source) debe superseder a la vieja (target): %+v", sup)
	}
	if sup.Status != RelStatusResolved || sup.ResolvedBy != "heuristic" {
		t.Errorf("el supersede de alta confianza debe auto-resolverse por heurística: %+v", sup)
	}
}

func TestDetectRelationsNoAutoSupersedeNueva(t *testing.T) {
	e := newTestEngine(t)
	// 'src' (la que pasamos a DetectRelations, p.ej. re-guardada por id) es MÁS VIEJA
	// que la candidata. Auto-superseder ocultaría 'src' pese a ser la recién tocada,
	// o peor, ocultaría contenido más nuevo. Debe quedar pending.
	saveAt(t, e, "src", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema.", "2026-01-01 10:00:00")
	saveAt(t, e, "cand", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema productivo.", "2026-06-01 10:00:00")

	rels, err := e.DetectRelations("src", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	for _, r := range rels {
		if r.Relation == RelSupersedes && r.Status == RelStatusResolved {
			t.Errorf("no debe auto-superseder cuando la candidata es más nueva: %+v", r)
		}
	}
	var sup string
	e.db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id='src'`).Scan(&sup)
	if sup != "" {
		t.Errorf("la observación 'src' no debe quedar oculta, superseded_by=%q", sup)
	}
}

func TestDetectRelationsMediaSimilitudQuedaPending(t *testing.T) {
	e := newTestEngine(t)
	saveAt(t, e, "a", "arch/api", "El servicio de autenticación valida tokens JWT con expiración corta.", "2026-01-01 10:00:00")
	saveAt(t, e, "b", "arch/api", "El servicio de autenticación valida tokens y además registra auditoría de accesos fallidos en disco.", "2026-06-01 10:00:00")

	rels, err := e.DetectRelations("b", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	hayPending := false
	for _, r := range rels {
		if r.Status == RelStatusPending {
			hayPending = true
		}
	}
	if !hayPending {
		t.Errorf("similitud media debe dejar la relación pendiente para el agente, obtuve %+v", rels)
	}
}

func TestDetectRelationsSinSimilitudNoRelaciona(t *testing.T) {
	e := newTestEngine(t)
	saveAt(t, e, "a", "arch/db", "Usamos PostgreSQL para persistencia.", "2026-01-01 10:00:00")
	saveAt(t, e, "b", "ui/theme", "El botón primario es de color verde menta en modo claro.", "2026-06-01 10:00:00")

	rels, err := e.DetectRelations("b", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) != 0 {
		t.Errorf("observaciones sin solapamiento no deben relacionarse, obtuve %+v", rels)
	}
}

func TestRecallExcluyeSuperseded(t *testing.T) {
	e := newTestEngine(t)
	saveAt(t, e, "old", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema.", "2026-01-01 10:00:00")
	saveAt(t, e, "new", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema productivo.", "2026-06-01 10:00:00")

	if _, err := e.DetectRelations("new", ConflictOptions{}); err != nil {
		t.Fatal(err)
	}

	res, err := e.Recall(context.Background(), "PostgreSQL base de datos", RecallOptions{TokenBudget: 500})
	if err != nil {
		t.Fatalf("Recall error: %v", err)
	}
	for _, it := range res.Items {
		if it.ID == "old" {
			t.Errorf("el recall no debe devolver la observación superseded 'old': %+v", res.Items)
		}
	}
	// La nueva sí debe estar.
	hayNew := false
	for _, it := range res.Items {
		if it.ID == "new" {
			hayNew = true
		}
	}
	if !hayNew {
		t.Errorf("el recall debe devolver la observación vigente 'new', items=%+v", res.Items)
	}
}

func TestKeywordSearchExcluyeSuperseded(t *testing.T) {
	e := newTestEngine(t)
	saveAt(t, e, "old", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema.", "2026-01-01 10:00:00")
	saveAt(t, e, "new", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema productivo.", "2026-06-01 10:00:00")
	if _, err := e.DetectRelations("new", ConflictOptions{}); err != nil {
		t.Fatal(err)
	}
	results, err := e.SearchObservationsFTS(context.Background(), "PostgreSQL", 20)
	if err != nil {
		t.Fatalf("SearchObservationsFTS error: %v", err)
	}
	for _, r := range results {
		if r.ID == "old" {
			t.Errorf("la búsqueda keyword no debe devolver superseded 'old': %+v", results)
		}
	}
}

func TestDetectRelationsNoSeRelacionaConsigoMisma(t *testing.T) {
	e := newTestEngine(t)
	saveAt(t, e, "solo", "arch/db", "Usamos PostgreSQL para persistencia del sistema.", "2026-01-01 10:00:00")
	rels, err := e.DetectRelations("solo", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) != 0 {
		t.Errorf("una observación no debe relacionarse consigo misma, obtuve %+v", rels)
	}
}
