package memory

import (
	"context"
	"testing"
)

// TestCodeMemoryProjectIsolationAndNoCollision valida el aislamiento multi-tenant de la memoria
// de código (Track 17, migración v13) Y la corrección del bug de colisión: con PRIMARY KEY(path)
// dos proyectos con el mismo path se pisaban el gist; ahora conviven por UNIQUE(path, project_id)
// y cada credencial lee el suyo (prefiriendo el propio sobre la fila sin atribuir).
func TestCodeMemoryProjectIsolationAndNoCollision(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	// Dos proyectos guardan el MISMO path — antes colisionaban en el ON CONFLICT(path).
	if err := e.SaveCodeMemoryFrom("crm", CodeMemory{Path: "auth.go", Gist: "crm gist", Tokens: 2}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveCodeMemoryFrom("web", CodeMemory{Path: "auth.go", Gist: "web gist", Tokens: 2}); err != nil {
		t.Fatal(err)
	}

	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	web := WithProjectScope(context.Background(), ProjectScope{ProjectID: "web"})

	// Cada uno lee el SUYO (no-colisión + aislamiento).
	if cm, ok, err := e.GetCodeMemoryCtx(crm, "auth.go"); err != nil || !ok || cm.Gist != "crm gist" {
		t.Errorf("crm debería leer 'crm gist', got ok=%v gist=%q err=%v", ok, cm.Gist, err)
	}
	if cm, ok, err := e.GetCodeMemoryCtx(web, "auth.go"); err != nil || !ok || cm.Gist != "web gist" {
		t.Errorf("web debería leer 'web gist', got ok=%v gist=%q err=%v", ok, cm.Gist, err)
	}

	// Con una fila sin atribuir ('') + una del proyecto: el proyecto PREFIERE la suya.
	if err := e.SaveCodeMemoryFrom("", CodeMemory{Path: "shared.go", Gist: "free gist", Tokens: 2}); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveCodeMemoryFrom("crm", CodeMemory{Path: "shared.go", Gist: "crm shared", Tokens: 2}); err != nil {
		t.Fatal(err)
	}
	if cm, _, _ := e.GetCodeMemoryCtx(crm, "shared.go"); cm.Gist != "crm shared" {
		t.Errorf("crm debería preferir su fila sobre la sin atribuir, leyó %q", cm.Gist)
	}
	// web (sin shared.go propio) cae a la fila sin atribuir.
	if cm, ok, _ := e.GetCodeMemoryCtx(web, "shared.go"); !ok || cm.Gist != "free gist" {
		t.Errorf("web debería ver la fila sin atribuir de shared.go, got ok=%v gist=%q", ok, cm.Gist)
	}
}
