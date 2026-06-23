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

func TestTopicDomainCounts(t *testing.T) {
	e := newTestEngine(t)
	saves := []struct{ id, topic string }{
		{"a", "roadmap/track-7"},
		{"b", "roadmap/track-8"},
		{"c", "roadmap/track-9"},
		{"d", "audit/full"},
		{"e", "audit/binario"},
		{"f", "tokens/brevity"},
		{"g", "project"}, // sin "/" -> dominio = el topic completo
	}
	for _, s := range saves {
		if err := e.SaveObservation(s.id, s.topic, "contenido de "+s.id, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Una archivada no debe contar.
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = 'c'`); err != nil {
		t.Fatal(err)
	}

	got, err := e.TopicDomainCounts()
	if err != nil {
		t.Fatalf("TopicDomainCounts error: %v", err)
	}
	// Esperado, ordenado por cantidad desc y desempate alfabético:
	// roadmap 2 (track-9 archivada), audit 2, project 1, tokens 1.
	want := []DomainCount{
		{"audit", 2},
		{"roadmap", 2},
		{"project", 1},
		{"tokens", 1},
	}
	if len(got) != len(want) {
		t.Fatalf("esperaba %d dominios, obtuve %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dominio %d: esperaba %+v, obtuve %+v (todo: %+v)", i, want[i], got[i], got)
		}
	}
}
