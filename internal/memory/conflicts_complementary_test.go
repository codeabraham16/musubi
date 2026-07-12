package memory

import "testing"

// Los pares de estos tests son deliberadamente CASI IDÉNTICOS: si la guarda no estuviera, el
// detector los relacionaría sin dudar. Que no haya relación sólo puede venir de la guarda.

func TestGuardaHermanosDelMismoCambioSDD(t *testing.T) { // S.a
	e := newTestEngine(t)
	saveAt(t, e, "spec", "sdd/mi-cambio/spec", "El detector DEBE proponer una relación pendiente cuando el coseno supera el piso.", "2026-01-01 10:00:00")
	saveAt(t, e, "design", "sdd/mi-cambio/design", "El detector propone una relación pendiente cuando el coseno supera el piso configurado.", "2026-01-02 10:00:00")

	rels, err := e.DetectRelations("design", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) != 0 {
		t.Fatalf("dos fases del MISMO cambio SDD son complementarias, no duplicadas: no debería haber relación, obtuve %+v", rels)
	}
}

func TestGuardaNoTapaCambiosSDDDistintos(t *testing.T) { // S.b — el test que más importa
	e := newTestEngine(t)
	saveAt(t, e, "a", "sdd/cambio-a/design", "El detector DEBE proponer una relación pendiente cuando el coseno supera el piso.", "2026-01-01 10:00:00")
	saveAt(t, e, "b", "sdd/cambio-b/design", "El detector DEBE proponer una relación pendiente cuando el coseno supera el piso.", "2026-01-02 10:00:00")

	rels, err := e.DetectRelations("b", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("dos cambios DISTINTOS que dicen lo mismo SÍ merecen un veredicto: la guarda se pasó de ancha")
	}
}

func TestGuardaCommitVsSuPropioSDD(t *testing.T) { // S.c — en AMBOS órdenes (D3)
	for _, tc := range []struct {
		name          string
		primero, dupe string // quién se guarda primero, y sobre quién corre la detección
	}{
		{"el commit llega ultimo", "sdd/mi-cambio/proposal", "git-commit"},
		{"el SDD llega ultimo", "git-commit", "sdd/mi-cambio/proposal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEngine(t)
			content := "fix(captura): no guardar dos veces el mismo commit al mergear con squash"
			saveAt(t, e, "primero", tc.primero, content, "2026-01-01 10:00:00")
			saveAt(t, e, "segundo", tc.dupe, content+" (#201)", "2026-01-02 10:00:00")

			rels, err := e.DetectRelations("segundo", ConflictOptions{})
			if err != nil {
				t.Fatalf("DetectRelations error: %v", err)
			}
			if len(rels) != 0 {
				t.Fatalf("el commit es el EVENTO y el contrato SDD es el ACUERDO: ninguno reemplaza al otro, no debería haber relación. Obtuve %+v", rels)
			}
		})
	}
}

func TestGuardaNoTapaCommitVsCommit(t *testing.T) { // S.d
	e := newTestEngine(t)
	saveAt(t, e, "c1", CommitTopicKey, "fix(captura): no guardar dos veces el mismo commit al mergear con squash", "2026-01-01 10:00:00")
	saveAt(t, e, "c2", CommitTopicKey, "fix(captura): no guardar dos veces el mismo commit al mergear con squash (#201)", "2026-01-02 10:00:00")

	rels, err := e.DetectRelations("c2", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("entre dos commits el parecido SÍ puede ser redundancia: la guarda no debe taparlo")
	}
}

func TestGuardaNoTapaCommitVsNota(t *testing.T) { // S.e
	e := newTestEngine(t)
	saveAt(t, e, "nota", "captura/dedup", "La captura no debe guardar dos veces el mismo commit al mergear con squash.", "2026-01-01 10:00:00")
	saveAt(t, e, "commit", CommitTopicKey, "fix(captura): no guardar dos veces el mismo commit al mergear con squash", "2026-01-02 10:00:00")

	rels, err := e.DetectRelations("commit", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("un commit y una nota común son comparables: la guarda no debe taparlos")
	}
}

func TestGuardaNoOcultaNadaJamas(t *testing.T) { // S.f — el invariante R0
	e := newTestEngine(t)
	saveAt(t, e, "spec", "sdd/mi-cambio/spec", "El detector DEBE proponer una relación pendiente cuando el coseno supera el piso.", "2026-01-01 10:00:00")
	saveAt(t, e, "design", "sdd/mi-cambio/design", "El detector propone una relación pendiente cuando el coseno supera el piso configurado.", "2026-01-02 10:00:00")

	if _, err := e.DetectRelations("design", ConflictOptions{}); err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	// Una guarda evita CREAR una relación. Nunca archiva, nunca marca superseded.
	for _, id := range []string{"spec", "design"} {
		var superseded *string
		var archived int
		err := e.db.QueryRow(`SELECT superseded_by, archived FROM observations WHERE id=?`, id).Scan(&superseded, &archived)
		if err != nil {
			t.Fatalf("consultando %s: %v", id, err)
		}
		if superseded != nil {
			t.Errorf("%s quedó superseded (%q): una guarda JAMÁS debe ocultar memoria", id, *superseded)
		}
		if archived != 0 {
			t.Errorf("%s quedó archivada: una guarda JAMÁS debe ocultar memoria", id)
		}
	}
}

func TestSddChange(t *testing.T) { // T12
	for _, tc := range []struct{ topicKey, want string }{
		{"sdd/mi-cambio/spec", "mi-cambio"},
		{"sdd/mi-cambio/archive", "mi-cambio"},
		{"sdd/mi-cambio", ""}, // sin la barra de la fase NO es un contrato SDD
		{"sdd/", ""},
		{"sdd//spec", ""}, // cambio vacío
		{"git-commit", ""},
		{"notas/x/y", ""},
		{"", ""},
	} {
		if got := sddChange(tc.topicKey); got != tc.want {
			t.Errorf("sddChange(%q) = %q, quería %q", tc.topicKey, got, tc.want)
		}
	}
}
