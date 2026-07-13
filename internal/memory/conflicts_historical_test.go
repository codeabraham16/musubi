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

// LA REGLA ENTERA, EN UNA TABLA. Antes vivía en tres guardas (G1, G2, G3) que se descubrieron por
// separado; resultaron ser TRES CARAS DE LA MISMA. La tabla es el contrato completo, y lo único que
// decide es la CLASE DEL DESTINO — por eso la columna `source` varía y nunca cambia el resultado,
// salvo cuando el destino deja de ser histórico.
//
// Los textos son deliberadamente CASI IDÉNTICOS: sin guarda, el detector los relacionaría sin dudar.
// Que no haya relación sólo puede venir de la guarda.
func TestSoloLasCreenciasSeReemplazan(t *testing.T) {
	const contenido = "La captura no debe guardar dos veces el mismo commit al mergear con squash."

	for _, tc := range []struct {
		name           string
		source         string // topic_key de la observación que se guarda ÚLTIMA (sobre la que corre la detección)
		target         string // topic_key de la que ya estaba
		quiereRelacion bool
	}{
		// El destino es un REGISTRO HISTÓRICO ⇒ jamás. Da igual quién pregunte.
		{"nota -> commit", "notas/captura", CommitTopicKey, false},
		{"nota -> contrato", "notas/captura", "sdd/mi-cambio/spec", false},
		{"commit -> commit", CommitTopicKey, CommitTopicKey, false},
		{"contrato -> contrato, MISMO cambio", "sdd/mi-cambio/design", "sdd/mi-cambio/spec", false},
		{"contrato -> contrato, cambios DISTINTOS", "sdd/cambio-b/spec", "sdd/cambio-a/spec", false},
		{"commit -> contrato", CommitTopicKey, "sdd/mi-cambio/spec", false},
		{"contrato -> commit", "sdd/mi-cambio/spec", CommitTopicKey, false},

		// El destino es una CREENCIA ⇒ sí se juzga. Acá vive todo el valor, y perderlo sería
		// convertir la regla en un martillo.
		{"commit -> nota (el commit es EVIDENCIA de que la nota envejeció)", CommitTopicKey, "notas/captura", true},
		{"contrato -> nota", "sdd/mi-cambio/spec", "notas/captura", true},
		{"nota -> nota (el ÚNICO par donde `supersedes` significa algo)", "notas/otra", "notas/captura", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEngine(t)
			saveAt(t, e, "target", tc.target, contenido, "2026-01-01 10:00:00")
			saveAt(t, e, "source", tc.source, contenido+" Vale para todos los repos.", "2026-01-02 10:00:00")

			rels, err := e.DetectRelations("source", ConflictOptions{})
			if err != nil {
				t.Fatalf("DetectRelations error: %v", err)
			}
			if got := len(rels) > 0; got != tc.quiereRelacion {
				if tc.quiereRelacion {
					t.Fatal("el destino es una CREENCIA y SÍ se puede reemplazar: la guarda se pasó de ancha," +
						" es un martillo")
				}
				t.Fatalf("el destino es un REGISTRO HISTÓRICO: no se puede des-hacer lo que pasó ni tachar lo"+
					" que se acordó, así que no hay veredicto que pedir. Obtuve %+v", rels)
			}
		})
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
