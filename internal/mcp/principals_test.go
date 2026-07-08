package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
