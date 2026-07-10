package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTenancyFailClosedRequiresProject valida el fail-closed de Track 18: reader/writer DEBEN
// tener project_id (sin él resolverían a scope vacío ⇒ recall federado + escritura sin atribuir);
// solo 'admin' puede ser federado. Cubre AddPrincipal (CLI, guarda primaria) y loadPrincipals
// (YAML editado a mano, defensa en profundidad).
func TestTenancyFailClosedRequiresProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".musubi", "principals.yaml")

	// AddPrincipal: reader/writer sin proyecto ⇒ error.
	if _, err := AddPrincipal(path, "w1", "", "writer"); err == nil {
		t.Error("writer sin --project debía rechazarse")
	}
	if _, err := AddPrincipal(path, "r1", "   ", "reader"); err == nil {
		t.Error("reader con --project en blanco debía rechazarse")
	}
	// admin sin proyecto ⇒ OK (federado por diseño).
	if _, err := AddPrincipal(path, "root", "", "admin"); err != nil {
		t.Errorf("admin sin proyecto debía permitirse: %v", err)
	}
	// reader/writer CON proyecto ⇒ OK.
	if _, err := AddPrincipal(path, "alice", "crm", "writer"); err != nil {
		t.Errorf("writer con proyecto debía permitirse: %v", err)
	}

	// loadPrincipals: un YAML con writer sin project_id ⇒ error (defensa en profundidad).
	writeYAML := func(body string) string {
		p := filepath.Join(t.TempDir(), "reg.yaml")
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	badBody := "principals:\n  - name: x\n    token_sha256: \"" + hashToken("t") + "\"\n    role: writer\n"
	if _, err := loadPrincipals(writeYAML(badBody), ""); err == nil {
		t.Error("loadPrincipals: writer sin project_id debía fallar-closed")
	}
	// admin sin project_id en YAML ⇒ OK.
	adminBody := "principals:\n  - name: root\n    token_sha256: \"" + hashToken("t") + "\"\n    role: admin\n"
	if _, err := loadPrincipals(writeYAML(adminBody), ""); err != nil {
		t.Errorf("loadPrincipals: admin sin project debía cargar: %v", err)
	}
	// reader CON project_id ⇒ OK.
	okBody := "principals:\n  - name: bob\n    token_sha256: \"" + hashToken("t2") + "\"\n    project_id: web\n    role: reader\n"
	if _, err := loadPrincipals(writeYAML(okBody), ""); err != nil {
		t.Errorf("loadPrincipals: reader con project debía cargar: %v", err)
	}
}
