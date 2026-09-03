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

// TestAddRevokeConservaExpires valida que dar de alta o revocar a OTRA persona no le borra el
// vencimiento a los que ya estaban. AddPrincipal/RemovePrincipal reescriben el archivo entero
// re-serializando principalEntry, así que un `expires` que no round-trippee por el YAML se
// perdería en silencio en el primer `musubi token new`: todas las credenciales acotadas volverían
// a valer para siempre y nadie lo notaría hasta que el contratista que se fue en marzo siga
// entrando en octubre.
//
// Sabotaje que la hace fallar: en principals.go, en principalEntry, cambiá la etiqueta del campo
// Expires por `yaml:"-"` — el alta de bob le borra el expires a alice.
func TestAddRevokeConservaExpires(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".musubi", "principals.yaml")
	if _, err := AddPrincipal(path, "alice", "crm", "writer"); err != nil {
		t.Fatalf("AddPrincipal alice: %v", err)
	}
	// El operador le pone el vencimiento a alice a mano: no hay comando que lo haga (la rotación
	// es de la Ola 2), así que el campo tiene que sobrevivir a las reescrituras del CLI.
	f, err := readPrincipalsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Principals[0].Expires = "2030-06-01T00:00:00Z"
	if err := writePrincipalsFile(path, f); err != nil {
		t.Fatal(err)
	}

	// Alta y baja de un tercero: las dos reescriben el archivo entero.
	if _, err := AddPrincipal(path, "bob", "crm", "reader"); err != nil {
		t.Fatalf("AddPrincipal bob: %v", err)
	}
	if found, err := RemovePrincipal(path, "bob"); err != nil || !found {
		t.Fatalf("RemovePrincipal bob: found=%v err=%v", found, err)
	}

	infos, err := ListPrincipalsInfo(path)
	if err != nil || len(infos) != 1 || infos[0].Name != "alice" {
		t.Fatalf("ListPrincipalsInfo: %+v err=%v", infos, err)
	}
	if infos[0].Expires != "2030-06-01T00:00:00Z" {
		t.Fatalf("el expires de alice debía sobrevivir al alta y la baja de bob, quedó %q", infos[0].Expires)
	}
}
