package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

func TestHashTokenDeterministic(t *testing.T) {
	h1 := hashToken("secreto")
	h2 := hashToken("secreto")
	if h1 != h2 {
		t.Fatal("hashToken no es determinista")
	}
	if hashToken("a") == hashToken("b") {
		t.Fatal("hashToken colisiona tokens distintos")
	}
	if len(hashToken("x")) != 64 {
		t.Fatalf("hashToken debe ser 64 hex, largo %d", len(hashToken("x")))
	}
}

func writeRegistry(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "principals.yaml")
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadPrincipalsMissingFile(t *testing.T) {
	// Sin archivo y sin token legacy ⇒ nil (modo legacy sin auth), sin error.
	reg, err := loadPrincipals(filepath.Join(t.TempDir(), "nope.yaml"), "")
	if err != nil || reg != nil {
		t.Fatalf("archivo ausente sin legacy: reg=%v err=%v, esperaba nil,nil", reg, err)
	}
	// Sin archivo pero con token legacy ⇒ registro con solo el legacy (admin).
	reg, err = loadPrincipals(filepath.Join(t.TempDir(), "nope.yaml"), "legacytok")
	if err != nil || reg == nil {
		t.Fatalf("archivo ausente con legacy: reg=%v err=%v", reg, err)
	}
	if p, ok := reg.resolve("legacytok"); !ok || p.Role != RoleAdmin {
		t.Fatalf("el token legacy debía resolver a admin: p=%v ok=%v", p, ok)
	}
}

func TestLoadPrincipalsValidAndResolve(t *testing.T) {
	aliceTok, bobTok := "alice-token", "bob-token"
	body := "principals:\n" +
		"  - name: alice\n    token_sha256: \"" + hashToken(aliceTok) + "\"\n    project_id: crm\n    role: writer\n" +
		"  - name: bob\n    token_sha256: \"" + hashToken(bobTok) + "\"\n    project_id: web\n    role: reader\n"
	reg, err := loadPrincipals(writeRegistry(t, body), "legacytok")
	if err != nil {
		t.Fatal(err)
	}
	p, ok := reg.resolve(aliceTok)
	if !ok || p.Name != "alice" || p.ProjectID != "crm" || p.Role != RoleWriter {
		t.Fatalf("alice mal resuelta: %+v ok=%v", p, ok)
	}
	if p, ok := reg.resolve(bobTok); !ok || p.Role != RoleReader {
		t.Fatalf("bob debía ser reader: %+v ok=%v", p, ok)
	}
	if p, ok := reg.resolve("legacytok"); !ok || p.Role != RoleAdmin {
		t.Fatalf("legacy debía seguir siendo admin junto al registro: %+v ok=%v", p, ok)
	}
	if _, ok := reg.resolve("token-desconocido"); ok {
		t.Fatal("un token desconocido no debía resolver")
	}
	if _, ok := reg.resolve(""); ok {
		t.Fatal("un token vacío no debía resolver")
	}
}

func TestLoadPrincipalsMalformed(t *testing.T) {
	cases := map[string]string{
		"rol inválido": "principals:\n  - name: x\n    token_sha256: \"" + hashToken("t") + "\"\n    role: superuser\n",
		"hash no-hex":  "principals:\n  - name: x\n    token_sha256: \"nohex\"\n    role: reader\n",
		"sin nombre":   "principals:\n  - token_sha256: \"" + hashToken("t") + "\"\n    role: reader\n",
		"sin rol":      "principals:\n  - name: x\n    token_sha256: \"" + hashToken("t") + "\"\n",
		"yaml roto":    "principals: [::::",
	}
	for name, body := range cases {
		if _, err := loadPrincipals(writeRegistry(t, body), ""); err == nil {
			t.Errorf("%s: esperaba error de carga, no hubo", name)
		}
	}
}

func TestCanCall(t *testing.T) {
	reader := &Principal{Role: RoleReader}
	writer := &Principal{Role: RoleWriter}
	admin := &Principal{Role: RoleAdmin}
	// reader: solo lectura.
	if !reader.canCall(true) || reader.canCall(false) {
		t.Error("reader debe poder leer y NO mutar")
	}
	// writer/admin: todo.
	if !writer.canCall(true) || !writer.canCall(false) || !admin.canCall(false) {
		t.Error("writer/admin deben poder todo")
	}
	// nil (stdio local): acceso pleno.
	var none *Principal
	if !none.canCall(false) {
		t.Error("sin principal (stdio) debe ser acceso pleno")
	}
}

// TestAuthzDispatch valida el gate de autorización en handleToolsCall: un reader es
// rechazado (codeUnauthorized) al invocar una tool que muta, pero un writer la ejecuta.
func TestAuthzDispatch(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	callWith := func(role, tool string, args map[string]any) *RpcError {
		argRaw, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: argRaw})
		ctx := withPrincipal(context.Background(), &Principal{Name: "p", Role: role})
		_, rpcErr := s.handleToolsCall(ctx, params)
		return rpcErr
	}

	// save_observation MUTA: reader denegado, writer permitido.
	saveArgs := map[string]any{"topic_key": "t/x", "content": "un hecho para guardar"}
	if e := callWith(RoleReader, "musubi_save_observation", saveArgs); e == nil || e.Code != codeUnauthorized {
		t.Fatalf("reader debía ser rechazado con codeUnauthorized, obtuve %+v", e)
	}
	if e := callWith(RoleWriter, "musubi_save_observation", saveArgs); e != nil {
		t.Fatalf("writer debía poder guardar, obtuve %+v", e)
	}
}

// TestResolveRechazaCredencialVencida valida el VENCIMIENTO de credenciales (Ola 0, "De Cuatro
// a Dos Mil"): un principal con `expires` en el pasado NO autentica aunque su hash siga en el
// archivo; con `expires` en el futuro autentica normal; sin `expires` no vence nunca (compat con
// todo registro anterior). El reloj se inyecta para que la prueba no dependa de la hora real.
//
// Sabotaje que la hace fallar: en resolve (principals.go) borrá el `if !match.Expires.IsZero()
// && !r.clock().Before(match.Expires) { return nil, false }` — la vencida vuelve a autenticar.
func TestResolveRechazaCredencialVencida(t *testing.T) {
	vencidaTok, vigenteTok, eternaTok := "vencida-token", "vigente-token", "eterna-token"
	body := "principals:\n" +
		"  - name: vencida\n    token_sha256: \"" + hashToken(vencidaTok) + "\"\n    project_id: crm\n    role: writer\n    expires: \"2026-03-01T00:00:00Z\"\n" +
		"  - name: vigente\n    token_sha256: \"" + hashToken(vigenteTok) + "\"\n    project_id: crm\n    role: writer\n    expires: \"2026-12-31T23:59:59Z\"\n" +
		"  - name: eterna\n    token_sha256: \"" + hashToken(eternaTok) + "\"\n    project_id: crm\n    role: reader\n"
	reg, err := loadPrincipals(writeRegistry(t, body), "")
	if err != nil {
		t.Fatal(err)
	}
	// "Hoy" es junio: marzo ya pasó, diciembre todavía no.
	reg.now = func() time.Time { return time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC) }

	if p, ok := reg.resolve(vencidaTok); ok {
		t.Fatalf("una credencial con expires en el pasado NO debía autenticar: %+v", p)
	}
	if p, ok := reg.resolve(vigenteTok); !ok || p.Name != "vigente" {
		t.Fatalf("una credencial con expires en el futuro debía autenticar: %+v ok=%v", p, ok)
	}
	if p, ok := reg.resolve(eternaTok); !ok || !p.Expires.IsZero() {
		t.Fatalf("una credencial sin expires no vence nunca: %+v ok=%v", p, ok)
	}

	// El vencimiento es un INSTANTE, no un día: en el segundo exacto ya no autentica (fail-closed
	// en el borde), y un segundo antes sí.
	reg.now = func() time.Time { return time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC) }
	if _, ok := reg.resolve(vigenteTok); ok {
		t.Fatal("en el instante exacto de expires la credencial ya debía estar vencida")
	}
	reg.now = func() time.Time { return time.Date(2026, 12, 31, 23, 59, 58, 0, time.UTC) }
	if _, ok := reg.resolve(vigenteTok); !ok {
		t.Fatal("un segundo antes de expires la credencial debía seguir vigente")
	}

	// Sin reloj inyectado se usa time.Now: la de marzo de 2026 ya venció en la realidad también.
	reg.now = nil
	if _, ok := reg.resolve(vencidaTok); ok {
		t.Fatal("con el reloj real, una credencial vencida en 2026-03 no debía autenticar")
	}
}

// TestLoadPrincipalsRechazaExpiresIlegible valida que un `expires` que no es RFC3339 rechaza el
// archivo ENTERO, con el mismo criterio fail-closed que un rol o un hash inválidos: ignorar el
// campo sería fail-open (el operador cree que acotó la credencial y el token sigue valiendo para
// siempre). Una fecha legible pero ya PASADA, en cambio, carga bien (es la que resolve niega).
//
// Sabotaje que la hace fallar: en loadPrincipals (principals.go) reemplazá el `return nil,
// fmt.Errorf("principal %q: expires ilegible ...")` por `expires = time.Time{}` (o quitá el
// bloque que parsea p.Expires) — el archivo con la fecha rota carga sin error.
func TestLoadPrincipalsRechazaExpiresIlegible(t *testing.T) {
	entry := func(expires string) string {
		return "principals:\n  - name: x\n    token_sha256: \"" + hashToken("t") + "\"\n    project_id: crm\n    role: reader\n    expires: " + expires + "\n"
	}
	ilegibles := map[string]string{
		"fecha sin hora":    `"2026-12-31"`,
		"formato humano":    `"31/12/2026 23:59"`,
		"texto":             `"nunca"`,
		"epoch":             `1767225599`,
		"sin zona horaria":  `"2026-12-31T23:59:59"`,
		"basura tras fecha": `"2026-12-31T23:59:59Z manana"`,
	}
	for name, exp := range ilegibles {
		_, err := loadPrincipals(writeRegistry(t, entry(exp)), "")
		if err == nil {
			t.Errorf("%s (%s): esperaba error de carga, no hubo", name, exp)
			continue
		}
		if !strings.Contains(err.Error(), "expires ilegible") {
			t.Errorf("%s: el error debía señalar el expires ilegible, fue: %v", name, err)
		}
	}
	// Legibles: futuro, pasado y con offset. Todas cargan.
	for name, exp := range map[string]string{
		"futuro":     `"2030-01-01T00:00:00Z"`,
		"pasado":     `"2020-01-01T00:00:00Z"`,
		"con offset": `"2026-12-31T20:59:59-03:00"`,
	} {
		if _, err := loadPrincipals(writeRegistry(t, entry(exp)), ""); err != nil {
			t.Errorf("%s (%s): una fecha RFC3339 legible debía cargar, falló: %v", name, exp, err)
		}
	}
}

// TestListPrincipalsInfoMuestraExpires valida que el listado (`musubi token list` y
// musubi_token_list) expone el vencimiento y marca las que ya vencieron: sin eso el operador vería
// como vigente a alguien a quien el server ya le cierra la puerta.
//
// Sabotaje que la hace fallar: en ListPrincipalsInfo (principals_admin.go) borrá el
// `info.Expired = true` — la vencida se lista como vigente.
func TestListPrincipalsInfoMuestraExpires(t *testing.T) {
	body := "principals:\n" +
		"  - name: vencida\n    token_sha256: \"" + hashToken("a") + "\"\n    project_id: crm\n    role: writer\n    expires: \"2020-01-01T00:00:00Z\"\n" +
		"  - name: vigente\n    token_sha256: \"" + hashToken("b") + "\"\n    project_id: crm\n    role: writer\n    expires: \"2100-01-01T00:00:00Z\"\n" +
		"  - name: eterna\n    token_sha256: \"" + hashToken("c") + "\"\n    project_id: crm\n    role: reader\n"
	infos, err := ListPrincipalsInfo(writeRegistry(t, body))
	if err != nil || len(infos) != 3 {
		t.Fatalf("ListPrincipalsInfo: %+v err=%v", infos, err)
	}
	want := map[string]PrincipalInfo{
		"vencida": {Expires: "2020-01-01T00:00:00Z", Expired: true},
		"vigente": {Expires: "2100-01-01T00:00:00Z", Expired: false},
		"eterna":  {Expires: "", Expired: false},
	}
	for _, in := range infos {
		w := want[in.Name]
		if in.Expires != w.Expires || in.Expired != w.Expired {
			t.Errorf("%s: expires=%q expired=%v, esperaba expires=%q expired=%v", in.Name, in.Expires, in.Expired, w.Expires, w.Expired)
		}
	}
}
