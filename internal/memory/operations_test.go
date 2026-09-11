package memory

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

// newTestEngine crea un DbEngine respaldado por un directorio temporal autolimpiable.
func newTestEngine(t *testing.T) *DbEngine {
	t.Helper()
	engine, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	return engine
}

func TestRecentObservations(t *testing.T) {
	e := newTestEngine(t)
	// DB vacía: lista vacía, sin error.
	if got, err := e.RecentObservations(5); err != nil || len(got) != 0 {
		t.Fatalf("DB vacía: esperaba [] sin error, obtuve %v / %v", got, err)
	}

	for _, s := range []struct{ id, topic, content string }{
		{"a", "roadmap/track-11", "Track 11 — dashboard local en vivo de la memoria."},
		{"b", "tokens/brevity", "Brevedad del gobernador: recorta tokens de salida."},
		{"c", "audit/full", "Auditoría completa con el flujo multi-agente."},
	} {
		if err := e.SaveObservation(s.id, s.topic, s.content, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Una archivada no debe aparecer.
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = 'c'`); err != nil {
		t.Fatal(err)
	}

	got, err := e.RecentObservations(10)
	if err != nil {
		t.Fatalf("RecentObservations error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("esperaba 2 memorias activas, obtuve %d: %+v", len(got), got)
	}
	for _, c := range got {
		if c.TopicKey == "" || c.Gist == "" || c.CreatedAt == "" {
			t.Errorf("cada memoria debe traer tema, gist y fecha legibles, obtuve %+v", c)
		}
		if c.TopicKey == "audit/full" {
			t.Error("una observación archivada no debe aparecer en las recientes")
		}
	}

	// El límite recorta.
	if lim, _ := e.RecentObservations(1); len(lim) != 1 {
		t.Errorf("el límite debe recortar a 1, obtuve %d", len(lim))
	}
}

func TestSaveObservationWithoutEmbedding(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveObservation("obs1", "topic/a", "contenido de prueba", nil); err != nil {
		t.Fatalf("SaveObservation error: %v", err)
	}

	// Debe encontrarse por FTS pero NO tener embedding (búsqueda semántica vacía).
	fts, err := e.SearchObservationsFTS(context.Background(), "prueba", 5)
	if err != nil {
		t.Fatalf("SearchObservationsFTS error: %v", err)
	}
	if len(fts) != 1 || fts[0].ID != "obs1" {
		t.Fatalf("esperaba 1 resultado FTS obs1, obtuve %+v", fts)
	}

	sem, err := e.SearchObservations(context.Background(), []float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("SearchObservations error: %v", err)
	}
	if len(sem) != 0 {
		t.Fatalf("esperaba 0 resultados semánticos (sin embedding), obtuve %d", len(sem))
	}
}

func TestSaveObservationUpsertByID(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveObservation("dup", "t", "versión uno", []float32{1, 0}); err != nil {
		t.Fatalf("save 1 error: %v", err)
	}
	if err := e.SaveObservation("dup", "t", "versión dos", []float32{0, 1}); err != nil {
		t.Fatalf("save 2 error: %v", err)
	}

	res, err := e.SearchObservations(context.Background(), []float32{0, 1}, 5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("esperaba 1 observación tras upsert, obtuve %d", len(res))
	}
	if res[0].Content != "versión dos" {
		t.Errorf("esperaba contenido actualizado, obtuve %q", res[0].Content)
	}
}

func TestSearchObservationsOrderingAndLimit(t *testing.T) {
	e := newTestEngine(t)

	// Vector query = {1,0}. Similitud: a más alineado, mayor score.
	mustSave(t, e, "alto", []float32{1, 0})      // cos = 1
	mustSave(t, e, "medio", []float32{1, 1})     // cos ~ 0.707
	mustSave(t, e, "bajo", []float32{0, 1})      // cos = 0
	mustSave(t, e, "negativo", []float32{-1, 0}) // cos = -1

	res, err := e.SearchObservations(context.Background(), []float32{1, 0}, 2)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("esperaba 2 resultados por límite, obtuve %d", len(res))
	}
	if res[0].ID != "alto" || res[1].ID != "medio" {
		t.Errorf("orden incorrecto: %s, %s", res[0].ID, res[1].ID)
	}
}

func TestSearchObservationsSkipsDimensionMismatch(t *testing.T) {
	e := newTestEngine(t)

	mustSave(t, e, "dim2", []float32{1, 0})
	mustSave(t, e, "dim3", []float32{1, 0, 0})

	// Query de dimensión 3: solo "dim3" es comparable; "dim2" se ignora.
	res, err := e.SearchObservations(context.Background(), []float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res) != 1 || res[0].ID != "dim3" {
		t.Fatalf("esperaba solo dim3, obtuve %+v", res)
	}
}

func TestSearchObservationsNegativeLimitNoPanic(t *testing.T) {
	e := newTestEngine(t)
	mustSave(t, e, "x", []float32{1, 0})

	// limit negativo no debe panic: se interpreta como "sin límite".
	res, err := e.SearchObservations(context.Background(), []float32{1, 0}, -1)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("esperaba 1 resultado, obtuve %d", len(res))
	}
}

func TestFTSDeleteTriggerRemovesIndex(t *testing.T) {
	e := newTestEngine(t)
	mustSave(t, e, "borrable", []float32{1, 0})

	if _, err := e.db.Exec(`DELETE FROM observations WHERE id = ?`, "borrable"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	fts, err := e.SearchObservationsFTS(context.Background(), "contenido", 5)
	if err != nil {
		t.Fatalf("fts error: %v", err)
	}
	if len(fts) != 0 {
		t.Fatalf("esperaba 0 tras borrar (trigger AFTER DELETE), obtuve %d", len(fts))
	}
}

func mustSave(t *testing.T, e *DbEngine, id string, emb []float32) {
	t.Helper()
	if err := e.SaveObservation(id, "topic", "contenido "+id, emb); err != nil {
		t.Fatalf("SaveObservation(%s) error: %v", id, err)
	}
}

func TestSaveComputesDigest(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("d1", "t", "Esta es una observación de prueba. Con más texto detrás.", nil); err != nil {
		t.Fatalf("save error: %v", err)
	}
	var gist, hash string
	var tokens int
	if err := e.db.QueryRow(`SELECT gist, content_hash, tokens FROM observations WHERE id=?`, "d1").
		Scan(&gist, &hash, &tokens); err != nil {
		t.Fatalf("query error: %v", err)
	}
	if gist == "" || hash == "" || tokens == 0 {
		t.Errorf("digest no computado al guardar: gist=%q hash=%q tokens=%d", gist, hash, tokens)
	}
}

func TestSaveObservationDedupByHash(t *testing.T) {
	e := newTestEngine(t)

	id1, dup1, err := e.SaveObservationDeduped("t", "contenido idéntico", 1.0, nil)
	if err != nil {
		t.Fatalf("save 1 error: %v", err)
	}
	if dup1 {
		t.Error("el primer guardado no debería marcarse como duplicado")
	}

	// Mismo contenido tras normalizar espacios -> debe deduplicar.
	id2, dup2, err := e.SaveObservationDeduped("t", "  contenido   idéntico ", 1.0, nil)
	if err != nil {
		t.Fatalf("save 2 error: %v", err)
	}
	if !dup2 {
		t.Error("el segundo guardado debería detectarse como duplicado")
	}
	if id1 != id2 {
		t.Errorf("dedup debería devolver el mismo id: %s vs %s", id1, id2)
	}

	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&n); err != nil {
		t.Fatalf("count error: %v", err)
	}
	if n != 1 {
		t.Errorf("esperaba 1 fila tras dedup, obtuve %d", n)
	}
}

func TestSearchFTSOrdersByRelevance(t *testing.T) {
	e := newTestEngine(t)
	// 'rel' es muy relevante: término repetido en un doc corto.
	if err := e.SaveObservation("rel", "t", "singleton singleton singleton", nil); err != nil {
		t.Fatalf("save rel error: %v", err)
	}
	// 'irrel' menciona el término una sola vez en un doc largo.
	if err := e.SaveObservation("irrel", "t", "singleton "+strings.Repeat("relleno ", 60), nil); err != nil {
		t.Fatalf("save irrel error: %v", err)
	}

	res, err := e.SearchObservationsFTS(context.Background(), "singleton", 5)
	if err != nil {
		t.Fatalf("fts error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("esperaba 2 resultados, obtuve %d", len(res))
	}
	if res[0].ID != "rel" {
		t.Errorf("ORDER BY rank: esperaba el doc más relevante primero (rel), obtuve %s", res[0].ID)
	}
}

// TestDedupRevivelaArchivadaYRespetaLosOtrosEstados fija el arreglo del dedup ciego a la
// visibilidad: casar por content_hash contra una fila que el recall NO puede devolver convertía un
// "ya la tengo" en memoria perdida para siempre.
func TestDedupRevivelaArchivadaYRespetaLosOtrosEstados(t *testing.T) {
	const texto = "la observacion que se archiva y despues se vuelve a guardar textual"

	t.Run("archivada: revive y vuelve al recall", func(t *testing.T) {
		e, err := NewDbEngine(t.TempDir())
		if err != nil {
			t.Fatalf("NewDbEngine: %v", err)
		}
		defer e.Close()

		id, deduped, err := e.SaveObservationDeduped("t/revive", texto, 1.0, nil)
		if err != nil || deduped {
			t.Fatalf("primer save: id=%s deduped=%v err=%v", id, deduped, err)
		}
		if _, err := e.db.Exec(`UPDATE observations SET archived = 1, archived_at = datetime('now') WHERE id = ?`, id); err != nil {
			t.Fatalf("archivar: %v", err)
		}
		var seqAntes int64
		if err := e.db.QueryRow(`SELECT sync_seq FROM observations WHERE id = ?`, id).Scan(&seqAntes); err != nil {
			t.Fatalf("leer sync_seq previo: %v", err)
		}
		// Otra escritura mueve el MAX, para que el cursor de un espejo pueda haber pasado por
		// encima de la fila archivada — que es exactamente el caso que deja la memoria sin entregar.
		if err := e.SaveObservation("ruido", "t/otro", "otra observacion que mueve el maximo", nil); err != nil {
			t.Fatalf("mover el maximo: %v", err)
		}

		id2, deduped2, err := e.SaveObservationDeduped("t/revive", texto, 1.0, nil)
		if err != nil {
			t.Fatalf("segundo save: %v", err)
		}
		if id2 != id {
			t.Errorf("el id tiene que conservarse (ya viajó por el sync): era %s, volvió %s", id, id2)
		}
		if !deduped2 {
			t.Errorf("esperaba deduped=true: el contenido ya existía")
		}
		// LO QUE IMPORTA: que haya vuelto a ser visible para el recall.
		var visible int
		if err := e.db.QueryRow(
			`SELECT COUNT(*) FROM observations WHERE id = ? AND `+visibleObsPredicate, id).Scan(&visible); err != nil {
			t.Fatalf("consultar visibilidad: %v", err)
		}
		if visible != 1 {
			t.Error("la observación siguió oculta: el que guardó recibió 'ya la tengo' y se quedó sin ella")
		}
		// Y TIENE QUE MOVER EL CURSOR DEL SYNC. El pull entrante pagina por sync_seq: si revivir
		// no lo bumpea, la fila queda detrás del cursor de todo espejo que ya pasó por ese número
		// y para ese cliente la observación no vuelve a existir nunca.
		var seq int64
		if err := e.db.QueryRow(`SELECT sync_seq FROM observations WHERE id = ?`, id).Scan(&seq); err != nil {
			t.Fatalf("leer sync_seq: %v", err)
		}
		if seq <= seqAntes {
			t.Errorf("revivir no bumpeó sync_seq (%d -> %d): un espejo cuyo cursor ya pasó no la recibe nunca", seqAntes, seq)
		}
	})

	t.Run("cuarentena: NO se lava por la puerta de atras", func(t *testing.T) {
		e, err := NewDbEngine(t.TempDir())
		if err != nil {
			t.Fatalf("NewDbEngine: %v", err)
		}
		defer e.Close()

		id, _, err := e.SaveObservationDeduped("t/quar", texto, 1.0, nil)
		if err != nil {
			t.Fatalf("primer save: %v", err)
		}
		if _, err := e.db.Exec(`UPDATE observations SET archived = 1, quarantined = 1 WHERE id = ?`, id); err != nil {
			t.Fatalf("cuarentenar: %v", err)
		}
		if _, _, err := e.SaveObservationDeduped("t/quar", texto, 1.0, nil); err != nil {
			t.Fatalf("segundo save: %v", err)
		}
		var quar int
		e.db.QueryRow(`SELECT quarantined FROM observations WHERE id = ?`, id).Scan(&quar)
		if quar != 1 {
			t.Error("re-guardar el texto sacó la cuarentena sin pasar por CorroborateObservation")
		}
	})

	t.Run("superseded: no se deshace la cadena", func(t *testing.T) {
		e, err := NewDbEngine(t.TempDir())
		if err != nil {
			t.Fatalf("NewDbEngine: %v", err)
		}
		defer e.Close()

		id, _, err := e.SaveObservationDeduped("t/sup", texto, 1.0, nil)
		if err != nil {
			t.Fatalf("primer save: %v", err)
		}
		if _, err := e.db.Exec(
			`UPDATE observations SET archived = 1, superseded_by = 'otra-mas-nueva' WHERE id = ?`, id); err != nil {
			t.Fatalf("supersede: %v", err)
		}
		if _, _, err := e.SaveObservationDeduped("t/sup", texto, 1.0, nil); err != nil {
			t.Fatalf("segundo save: %v", err)
		}
		var sup sql.NullString
		e.db.QueryRow(`SELECT superseded_by FROM observations WHERE id = ?`, id).Scan(&sup)
		if !sup.Valid || sup.String != "otra-mas-nueva" {
			t.Error("re-guardar el texto deshizo un supersede que alguien decidió")
		}
	})
}
