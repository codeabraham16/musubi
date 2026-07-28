package memory

import "testing"

// TestIngestSharedSyncSeqAtomic: tras ingerir, la fila SIEMPRE queda con sync_seq>0 (nunca 0, que la
// haría invisible al pull), y cada ingest sube el sync_seq monótonamente — todo en un solo statement
// atómico (fix de atomicidad de la auditoría v0.98.0).
func TestIngestSharedSyncSeqAtomic(t *testing.T) {
	e := newTestEngine(t)
	seq := func(id string) int {
		t.Helper()
		var s int
		if err := e.db.QueryRow(`SELECT sync_seq FROM observations WHERE id=?`, id).Scan(&s); err != nil {
			t.Fatalf("seq(%s): %v", id, err)
		}
		return s
	}

	if _, err := e.IngestShared(SharedObs{ID: "s1", TopicKey: "t", Content: "uno", Importance: 1, ProjectID: "acme"}); err != nil {
		t.Fatal(err)
	}
	if s := seq("s1"); s <= 0 {
		t.Fatalf("sync_seq debe ser > 0 tras ingest (nunca 0 = invisible al pull), got %d", s)
	}

	if _, err := e.IngestShared(SharedObs{ID: "s2", TopicKey: "t", Content: "dos", Importance: 1, ProjectID: "acme"}); err != nil {
		t.Fatal(err)
	}
	if seq("s2") <= seq("s1") {
		t.Errorf("cada ingest debe subir el sync_seq: s2(%d) debe ser > s1(%d)", seq("s2"), seq("s1"))
	}

	// Re-ingest (update) de s1 debe re-bumpear por encima del máximo actual (que una edición de una
	// obs shared ya sincronizada se re-entregue al pull).
	if _, err := e.IngestShared(SharedObs{ID: "s1", TopicKey: "t", Content: "uno editado", Importance: 1, ProjectID: "acme"}); err != nil {
		t.Fatal(err)
	}
	if seq("s1") <= seq("s2") {
		t.Errorf("re-ingest (update) debe re-bumpear s1 por encima de s2: s1(%d) > s2(%d)", seq("s1"), seq("s2"))
	}
}
