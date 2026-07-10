package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIsRemoteLegacyTenancy valida la condición que StrictTenancy rechaza y el WARNING hace
// visible (Track 18): un bind NO-loopback sin registro de principals real (nil o solo el bearer
// legacy) = un token con acceso total a todos los proyectos. Un bind loopback nunca dispara.
func TestIsRemoteLegacyTenancy(t *testing.T) {
	withPrincipals := &PrincipalRegistry{principals: []Principal{{Name: "a", Role: RoleWriter, ProjectID: "crm"}}}
	legacyOnly := &PrincipalRegistry{legacyHash: "deadbeef"}
	cases := []struct {
		name     string
		loopback bool
		reg      *PrincipalRegistry
		want     bool
	}{
		{"loopback sin registro", true, nil, false},
		{"loopback con legacy", true, legacyOnly, false},
		{"remoto sin registro", false, nil, true},
		{"remoto legacy-only", false, legacyOnly, true},
		{"remoto con principals", false, withPrincipals, false},
	}
	for _, c := range cases {
		if got := isRemoteLegacyTenancy(c.loopback, c.reg); got != c.want {
			t.Errorf("%s: isRemoteLegacyTenancy(%v)=%v, esperaba %v", c.name, c.loopback, got, c.want)
		}
	}
}

// TestLoadPrincipalsRejectsDuplicateNames valida la unicidad de nombres (Track 18): el nombre es
// la clave de la cuota por-principal, así que dos homónimos (case-insensitive) se rechazan.
func TestLoadPrincipalsRejectsDuplicateNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reg.yaml")
	body := "principals:\n" +
		"  - name: alice\n    token_sha256: \"" + hashToken("t1") + "\"\n    project_id: crm\n    role: writer\n" +
		"  - name: Alice\n    token_sha256: \"" + hashToken("t2") + "\"\n    project_id: web\n    role: reader\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPrincipals(path, ""); err == nil {
		t.Error("nombres duplicados (case-insensitive) debían rechazarse en la carga")
	}
}
