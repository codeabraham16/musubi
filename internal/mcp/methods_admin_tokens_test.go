package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
)

// callAsPrincipal invoca una tool con un principal explícito en el contexto (para probar la
// autorización). El helper `call` usa principal nil = stdio local, que isAdmin() trata como
// confianza local (admin), así que no sirve para verificar el rechazo.
func callAsPrincipal(t *testing.T, s *McpServer, p *Principal, tool string, args map[string]any) (interface{}, *RpcError) {
	t.Helper()
	raw, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: raw})
	return s.handleToolsCall(withPrincipal(context.Background(), p), params)
}

// jsonOf parsea la respuesta JSON de una tool a un mapa.
func jsonOf(t *testing.T, res interface{}) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(textOf(t, res)), &out); err != nil {
		t.Fatalf("respuesta no es JSON: %v (%s)", err, textOf(t, res))
	}
	return out
}

// TestTokenAdminRoundTrip: como admin local (principal nil), mintear → listar → revocar → listar.
// El token viaja una sola vez; el listado refleja el alta y la baja.
func TestTokenAdminRoundTrip(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// Alta de un reader (read=own, write=none por default del rol).
	res, e := call(t, s, "musubi_token_new", map[string]any{"name": "alice", "project": "web", "role": "reader"})
	if e != nil {
		t.Fatalf("token_new error: %+v", e)
	}
	out := jsonOf(t, res)
	if tok, _ := out["token"].(string); !strings.HasPrefix(tok, "msb_") {
		t.Errorf("esperaba un token con prefijo msb_, obtuve %q", out["token"])
	}
	if out["read"] != "own" || out["write"] != "none" {
		t.Errorf("un reader debe quedar read=own write=none, obtuve read=%v write=%v", out["read"], out["write"])
	}

	// Listar: alice aparece con sus capacidades efectivas.
	res, e = call(t, s, "musubi_token_list", map[string]any{})
	if e != nil {
		t.Fatalf("token_list error: %+v", e)
	}
	principals, _ := jsonOf(t, res)["principals"].([]any)
	if !hasPrincipal(principals, "alice") {
		t.Fatalf("alice no aparece en el listado tras el alta: %v", principals)
	}

	// Revocar: found=true.
	res, e = call(t, s, "musubi_token_revoke", map[string]any{"name": "alice"})
	if e != nil {
		t.Fatalf("token_revoke error: %+v", e)
	}
	if found, _ := jsonOf(t, res)["found"].(bool); !found {
		t.Error("esperaba found=true al revocar a alice")
	}

	// Listar de nuevo: alice ya no está.
	res, _ = call(t, s, "musubi_token_list", map[string]any{})
	principals, _ = jsonOf(t, res)["principals"].([]any)
	if hasPrincipal(principals, "alice") {
		t.Errorf("alice sigue en el listado tras revocarla: %v", principals)
	}
}

func hasPrincipal(principals []any, name string) bool {
	for _, p := range principals {
		if m, ok := p.(map[string]any); ok && m["name"] == name {
			return true
		}
	}
	return false
}

// TestTokenAdminToolsRejectNonAdmin: un writer y un reader NO pueden mintear/listar/revocar. La
// guarda es server-side (isAdmin()), independiente de lo que muestre la UI del cliente.
func TestTokenAdminToolsRejectNonAdmin(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	writer := &Principal{Name: "dev", Role: RoleWriter, ProjectID: "web"}
	reader := &Principal{Name: "ojos", Role: RoleReader, ProjectID: "web"}

	for _, tc := range []struct {
		p    *Principal
		tool string
		args map[string]any
	}{
		{writer, "musubi_token_new", map[string]any{"name": "x", "project": "web"}},
		{writer, "musubi_token_list", map[string]any{}},
		{writer, "musubi_token_revoke", map[string]any{"name": "x"}},
		{reader, "musubi_token_new", map[string]any{"name": "x", "project": "web"}},
		{reader, "musubi_token_list", map[string]any{}},
		{reader, "musubi_token_revoke", map[string]any{"name": "x"}},
	} {
		_, e := callAsPrincipal(t, s, tc.p, tc.tool, tc.args)
		if e == nil {
			t.Errorf("%s: un %s no-admin no debería poder invocarla", tc.tool, tc.p.Role)
		} else if e.Code != codeUnauthorized {
			t.Errorf("%s (%s): esperaba unauthorized, obtuve code %d", tc.tool, tc.p.Role, e.Code)
		}
	}

	// Un admin explícito SÍ puede (mismo camino que el stdio local).
	admin := &Principal{Name: "root", Role: RoleAdmin}
	if _, e := callAsPrincipal(t, s, admin, "musubi_token_list", map[string]any{}); e != nil {
		t.Errorf("un admin debe poder listar: %+v", e)
	}
}

// TestTokenNewFailClosedTenancy: la tenancy fail-closed de AddPrincipalWithCaps se propaga como
// argumento inválido. Un writer sin proyecto se rechaza; una cabina (write=none) sin proyecto pasa.
func TestTokenNewFailClosedTenancy(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// writer (default) sin project → su escritura caería sin atribuir → rechazo.
	if _, e := call(t, s, "musubi_token_new", map[string]any{"name": "sinproj"}); e == nil {
		t.Error("esperaba rechazo: un writer sin project deja su escritura sin atribuir")
	} else if e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params, obtuve code %d", e.Code)
	}

	// cabina: read=all + write=none, sin project → válido (no muta, no necesita tenant).
	res, e := call(t, s, "musubi_token_new", map[string]any{"name": "cabina", "read": "all", "write": "none"})
	if e != nil {
		t.Fatalf("una cabina (write=none) sin project debe poder crearse: %+v", e)
	}
	if out := jsonOf(t, res); out["read"] != "all" || out["write"] != "none" {
		t.Errorf("la cabina debe quedar read=all write=none, obtuve read=%v write=%v", out["read"], out["write"])
	}
}

// TestTokenRevokeCannotRevokeSelf: un admin no puede revocarse a sí mismo (evita el lockout del
// único admin). Se rechaza ANTES de tocar el registro.
func TestTokenRevokeCannotRevokeSelf(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	admin := &Principal{Name: "root", Role: RoleAdmin}
	_, e := callAsPrincipal(t, s, admin, "musubi_token_revoke", map[string]any{"name": "root"})
	if e == nil {
		t.Fatal("esperaba rechazo al intentar auto-revocarse")
	}
	if e.Code != codeInvalidParams {
		t.Errorf("esperaba invalid params en auto-revoke, obtuve code %d", e.Code)
	}
}
