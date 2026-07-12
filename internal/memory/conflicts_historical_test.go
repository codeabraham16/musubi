package memory

import "testing"

// Un REGISTRO HISTÓRICO (un commit, un contrato SDD) es lo que PASÓ o lo que se ACORDÓ. No se
// puede des-hacer. Estos tests pinean las DOS mitades de esa idea — y la segunda importa más:
// la regla tiene que ser ASIMÉTRICA, no un martillo.

func TestNotaNoPuedeReemplazarUnRegistroHistorico(t *testing.T) { // S.c y S.d
	for _, tc := range []struct{ name, topic string }{
		{"un commit", CommitTopicKey},
		{"un contrato SDD", "sdd/mi-cambio/proposal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEngine(t)
			contenido := "La captura no debe guardar dos veces el mismo commit al mergear con squash."
			saveAt(t, e, "registro", tc.topic, contenido, "2026-01-01 10:00:00")
			saveAt(t, e, "nota", "notas/captura", contenido+" Vale para todos los repos.", "2026-01-02 10:00:00")

			rels, err := e.DetectRelations("nota", ConflictOptions{})
			if err != nil {
				t.Fatalf("DetectRelations error: %v", err)
			}
			if len(rels) != 0 {
				t.Fatalf("una nota NO puede reemplazar a lo que pasó ni a lo que se acordó: el único"+
					" veredicto posible sería imposible, así que no hay relación que proponer. Obtuve %+v", rels)
			}
		})
	}
}

// EL TEST QUE MÁS IMPORTA. El modo de fallo peligroso acá es el MARTILLO: una guarda que, por
// ancha, apague el caso valioso. Al revés SÍ importa — un commit es EVIDENCIA de que una nota
// quedó vieja.
func TestUnCommitSiPuedeVolverObsoletaUnaNota(t *testing.T) { // S.e
	e := newTestEngine(t)
	saveAt(t, e, "nota", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema.", "2026-01-01 10:00:00")
	saveAt(t, e, "commit", CommitTopicKey, "feat(db): usamos PostgreSQL como base de datos principal del sistema y migramos a SQLite.", "2026-06-01 10:00:00")

	rels, err := e.DetectRelations("commit", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("un commit SÍ puede volver obsoleta una nota: es EVIDENCIA de que envejeció." +
			" La guarda es asimétrica, no un martillo — si esto no detecta nada, se pasó de ancha")
	}
}

func TestCommitVsCommitSeSigueDetectando(t *testing.T) { // S.f — misma clase
	e := newTestEngine(t)
	saveAt(t, e, "c1", CommitTopicKey, "fix(captura): no guardar dos veces el mismo commit al mergear con squash", "2026-01-01 10:00:00")
	saveAt(t, e, "c2", CommitTopicKey, "fix(captura): no guardar dos veces el mismo commit al mergear con squash (#201)", "2026-01-02 10:00:00")

	rels, err := e.DetectRelations("c2", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("entre dos commits el parecido SÍ puede ser redundancia (son la misma clase): no debe taparse")
	}
}

func TestSDDVsSDDDeCambiosDistintosSeSigueDetectando(t *testing.T) { // S.g — misma clase
	e := newTestEngine(t)
	saveAt(t, e, "a", "sdd/cambio-a/design", "El detector DEBE proponer una relación pendiente cuando el coseno supera el piso.", "2026-01-01 10:00:00")
	saveAt(t, e, "b", "sdd/cambio-b/design", "El detector DEBE proponer una relación pendiente cuando el coseno supera el piso.", "2026-01-02 10:00:00")

	rels, err := e.DetectRelations("b", ConflictOptions{})
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("dos cambios SDD DISTINTOS son la misma clase: la guarda no debe taparlos")
	}
}

func TestLasGuardasNuncaOcultanNada(t *testing.T) { // S.h — el invariante de siempre
	e := newTestEngine(t)
	contenido := "La captura no debe guardar dos veces el mismo commit al mergear con squash."
	saveAt(t, e, "commit", CommitTopicKey, contenido, "2026-01-01 10:00:00")
	saveAt(t, e, "nota", "notas/captura", contenido+" Vale para todos los repos.", "2026-01-02 10:00:00")

	if _, err := e.DetectRelations("nota", ConflictOptions{}); err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	for _, id := range []string{"commit", "nota"} {
		var superseded *string
		var archived int
		if err := e.db.QueryRow(`SELECT superseded_by, archived FROM observations WHERE id=?`, id).
			Scan(&superseded, &archived); err != nil {
			t.Fatalf("consultando %s: %v", id, err)
		}
		if superseded != nil || archived != 0 {
			t.Errorf("%s quedó oculta: una guarda EVITA CREAR una relación, JAMÁS oculta memoria", id)
		}
	}
}
