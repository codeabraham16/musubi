package memory

import "testing"

func TestTopicExists(t *testing.T) {
	e := newTestEngine(t)

	ok, err := e.TopicExists("project/profile")
	if err != nil {
		t.Fatalf("TopicExists error: %v", err)
	}
	if ok {
		t.Error("no debería existir el topic en una DB vacía")
	}

	if err := e.SaveObservation("p1", "project/profile", "Perfil del proyecto: API en Go con SQLite.", nil); err != nil {
		t.Fatal(err)
	}
	ok, err = e.TopicExists("project/profile")
	if err != nil {
		t.Fatalf("TopicExists error: %v", err)
	}
	if !ok {
		t.Error("el topic debería existir tras guardar una observación con ese topic_key")
	}
}

func TestTopicExistsIgnoraArchivadas(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("p1", "project/profile", "perfil viejo", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = 'p1'`); err != nil {
		t.Fatal(err)
	}
	ok, err := e.TopicExists("project/profile")
	if err != nil {
		t.Fatalf("TopicExists error: %v", err)
	}
	if ok {
		t.Error("una observación archivada no debe contar como topic existente")
	}
}
