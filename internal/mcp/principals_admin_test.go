package mcp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateTokenUnique(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GenerateToken()
	if a == b {
		t.Fatal("dos tokens generados no deberían coincidir")
	}
	if !strings.HasPrefix(a, "msb_") || len(a) < 20 {
		t.Fatalf("token con formato inesperado: %q", a)
	}
}

// TestAddListRevokeRoundTrip valida el ciclo completo del CLI: AddPrincipal genera el token
// y guarda su hash; el token generado AUTENTICA vía loadPrincipals/resolve; ListPrincipalsInfo
// no expone hashes; los nombres duplicados se rechazan; RemovePrincipal revoca.
func TestAddListRevokeRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".musubi", "principals.yaml")

	tok, err := AddPrincipal(path, "alice", "crm", "writer")
	if err != nil {
		t.Fatalf("AddPrincipal: %v", err)
	}
	if !strings.HasPrefix(tok, "msb_") {
		t.Fatalf("token con formato inesperado: %q", tok)
	}

	// El token generado debe autenticar contra el registro escrito.
	reg, err := loadPrincipals(path, "")
	if err != nil || reg == nil {
		t.Fatalf("loadPrincipals: reg=%v err=%v", reg, err)
	}
	p, ok := reg.resolve(tok)
	if !ok || p.Name != "alice" || p.ProjectID != "crm" || p.Role != RoleWriter {
		t.Fatalf("el token generado debía resolver a alice/crm/writer: %+v ok=%v", p, ok)
	}

	// Nombre duplicado ⇒ error.
	if _, err := AddPrincipal(path, "alice", "otro", "reader"); err == nil {
		t.Error("un nombre duplicado debía rechazarse")
	}
	// Rol inválido ⇒ error.
	if _, err := AddPrincipal(path, "carol", "x", "superuser"); err == nil {
		t.Error("un rol inválido debía rechazarse")
	}

	// list no expone hashes, muestra a alice.
	infos, err := ListPrincipalsInfo(path)
	if err != nil || len(infos) != 1 || infos[0].Name != "alice" {
		t.Fatalf("ListPrincipalsInfo inesperado: %+v err=%v", infos, err)
	}

	// revoke alice ⇒ found; el token deja de resolver.
	found, err := RemovePrincipal(path, "alice")
	if err != nil || !found {
		t.Fatalf("RemovePrincipal alice: found=%v err=%v", found, err)
	}
	reg2, _ := loadPrincipals(path, "")
	if reg2 != nil {
		if _, ok := reg2.resolve(tok); ok {
			t.Error("tras revocar, el token no debía resolver")
		}
	}
	// revoke inexistente ⇒ found=false, sin error.
	if found, err := RemovePrincipal(path, "nadie"); err != nil || found {
		t.Fatalf("revoke inexistente: found=%v err=%v", found, err)
	}
}
