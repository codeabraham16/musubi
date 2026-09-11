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

// EL VENCIMIENTO ERA EL ÚNICO CAMPO DEL REGISTRO SIN CUSTODIO, Y SE PIERDE POR ESCRITURA AJENA.
//
// `AddPrincipal` y `RemovePrincipal` no editan una fila: leen el archivo entero, lo modifican en
// memoria y lo REESCRIBEN COMPLETO. O sea que un alta o una baja de OTRO principal pasa por encima
// de todos los campos de todos los demás. La mayoría tiene quien la mire —`read`/`write` los cubre
// `TestAddPrincipalWithCapsGuardaYPersiste`—; `expires` no tenía a nadie.
//
// Y el daño no es perder un dato: es que una credencial que VENCE se convierta en una ETERNA. El
// registro es lo único que hace vencer a un token `msb_`, así que blanquear ese campo no rompe
// nada visible —el archivo sigue siendo válido, el token sigue autenticando— y el sistema pasa a
// contestar «no vence» a una pregunta que nunca volvió a medir. Otra vez «no sé» con cara de «medí
// y está bien», en el eje que decide quién puede ejecutar comandos.
//
// EL SABOTAJE QUE AÍSLA ESTE AGUJERO, corrido contra main en e537936 antes de escribir esto: en
// `writePrincipalsFile`, ANTES del `yaml.Marshal`, poner
//
//	for i := range f.Principals { f.Principals[i].Expires = "" }
//
// que es literalmente «el alta o la baja de otro le borra el vencimiento a los que ya estaban»,
// sin tocar el camino de LECTURA. Con eso, `go test ./internal/mcp/ ./cmd/musubi/` daba
// `ok 136,651s` y `ok 50,004s`, EXIT 0 — el repo entero en VERDE sobre el defecto. Con esta prueba
// puesta, ROJA.
//
// ⚠️ NO SIRVE EL SABOTAJE OBVIO, y conviene dejarlo escrito para que nadie lo repita: ponerle
// `yaml:"-"` al campo lo agarran CINCO tests, porque rompe además la lectura. Un sabotaje que
// rompe de más no aísla nada: prueba que algo se dio cuenta, no que ESTA guarda mira acá.
//
// ¿ES `writePrincipalsFile` EL ÚNICO CAMINO? SÍ, MEDIDO — porque si hubiera otro escritor del
// archivo (una rotación, un `token new` que reescriba), tendría el mismo agujero y esta guarda no
// lo alcanzaría. Hay UN solo `os.WriteFile` del registro (principals_admin.go:109, adentro de esta
// función), DOS llamadores (`AddPrincipalWithCaps` y `RemovePrincipal`), y `AddPrincipal` delega en
// el primero. La rotación de flota nombra `principals.yaml` sólo en comentarios y no lo escribe.
// Es un embudo, así que custodiarlo acá los cubre a los tres.
//
// HERMANO BUSCADO: el mismo sabotaje sobre `Read`/`Write` en vez de `Expires` sí sale rojo
// (`TestAddPrincipalWithCapsGuardaYPersiste`). De los campos de `principalEntry` que round-trippean
// por `writePrincipalsFile`, `expires` era el único sin custodio.
//
// La prueba viene de una rama huérfana (26b7253) cuyo código NO se integró a propósito: main ya
// resolvió el vencimiento mejor —cuatro estados en vez de un bool, y la guarda en `porNombre()`,
// el lookup que la huérfana ni miraba—. Lo único que la huérfana tenía y main no era esta guarda.
func TestAddRevokeConservaExpires(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".musubi", "principals.yaml")
	if _, err := AddPrincipal(path, "alice", "crm", "writer"); err != nil {
		t.Fatalf("AddPrincipal alice: %v", err)
	}
	// El operador le pone el vencimiento a mano: no hay comando que lo escriba, así que el campo
	// tiene que sobrevivir a las reescrituras del CLI o no sirve para nada.
	f, err := readPrincipalsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Principals[0].Expires = "2030-06-01T00:00:00Z"
	if err := writePrincipalsFile(path, f); err != nil {
		t.Fatal(err)
	}

	// Alta y baja de un TERCERO: las dos reescriben el archivo entero, y ninguna tiene por qué
	// saber que alice existe.
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
		t.Fatalf("el expires de alice debía sobrevivir al alta y la baja de bob, quedó %q. "+
			"Un vencimiento que se pierde en una escritura ajena convierte una credencial que VENCE "+
			"en una ETERNA, y en silencio", infos[0].Expires)
	}
}
