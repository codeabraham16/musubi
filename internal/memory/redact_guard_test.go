package memory

import (
	"strings"
	"testing"
)

// obsContentByID lee el contenido persistido de una observación (helper de test, mismo paquete).
func obsContentByID(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var c string
	if err := e.db.QueryRow(`SELECT content FROM observations WHERE id=?`, id).Scan(&c); err != nil {
		t.Fatalf("leyendo content de %s: %v", id, err)
	}
	return c
}

const awsSecret = "AKIA1234567890ABCDEF"

func TestSaveSharedRedactsSecret(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("s1", "t", "clave "+awsSecret+" fin", 0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	got := obsContentByID(t, e, "s1")
	if strings.Contains(got, awsSecret) {
		t.Fatalf("una obs shared no debe conservar el secreto crudo: %q", got)
	}
	if !strings.Contains(got, "[REDACTED:aws-access-key]") {
		t.Fatalf("esperaba el secreto redactado, obtuve: %q", got)
	}
}

func TestSaveLocalKeepsSecretRaw(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("l1", "t", "clave "+awsSecret+" fin", nil); err != nil {
		t.Fatal(err)
	}
	got := obsContentByID(t, e, "l1")
	if !strings.Contains(got, awsSecret) {
		t.Fatalf("la memoria local NO debe redactarse (secretos ok en tu máquina): %q", got)
	}
}

func TestResaveSharedRowStaysRedacted(t *testing.T) {
	// El UPSERT preserva un 'shared' previo: un re-save por vía local NO debe re-abrir la fuga.
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("s2", "t", "primero "+awsSecret, 0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	// Re-save por la vía histórica (scope local por default) con OTRO secreto.
	const aws2 = "AKIAZZZZ9999YYYY8888"
	if err := e.SaveObservation("s2", "t", "segundo "+aws2, nil); err != nil {
		t.Fatal(err)
	}
	got := obsContentByID(t, e, "s2")
	if strings.Contains(got, aws2) {
		t.Fatalf("re-save de una fila shared debe seguir redactando: %q", got)
	}
	if !strings.Contains(got, "[REDACTED:aws-access-key]") {
		t.Fatalf("esperaba redacción tras el re-save, obtuve: %q", got)
	}
}

func TestPromoteRedactsSecret(t *testing.T) {
	e := newTestEngine(t)
	// Local crudo (con secreto), luego promover → debe redactar al cruzar a shared.
	if err := e.SaveObservation("p2", "t", "token ghp_abcdefghij0123456789abcdefghij012345 x", nil); err != nil {
		t.Fatal(err)
	}
	if got := obsContentByID(t, e, "p2"); !strings.Contains(got, "ghp_") {
		t.Fatalf("precondición: local debía quedar crudo, obtuve: %q", got)
	}
	if err := e.PromoteObservation("p2"); err != nil {
		t.Fatal(err)
	}
	got := obsContentByID(t, e, "p2")
	if strings.Contains(got, "ghp_abcdefghij0123456789abcdefghij012345") {
		t.Fatalf("promover debe redactar el secreto: %q", got)
	}
	if !strings.Contains(got, "[REDACTED:github-token]") {
		t.Fatalf("esperaba el token redactado tras promover, obtuve: %q", got)
	}
}
