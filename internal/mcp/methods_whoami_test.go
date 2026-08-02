package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

// whoamiResult invoca toolWhoami con (o sin) un principal y devuelve el JSON parseado.
func whoamiResult(t *testing.T, s *McpServer, p *Principal) map[string]any {
	t.Helper()
	ctx := context.Background()
	if p != nil {
		ctx = withPrincipal(ctx, p)
	}
	res, rpcErr := s.toolWhoami(ctx, nil)
	if rpcErr != nil {
		t.Fatalf("toolWhoami error: %v", rpcErr)
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("resultado inesperado: %#v", res)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &m); err != nil {
		t.Fatalf("json ilegible: %v", err)
	}
	return m
}

// La identidad sale del principal (sala de mando: ve todo, escribe lo suyo).
func TestWhoamiCentralPrincipal(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	p := &Principal{Name: "davantis", ProjectID: "musubi", Role: RoleWriter, Read: ReadAll, Write: WriteOwn}
	m := whoamiResult(t, s, p)
	if m["authenticated"] != true {
		t.Errorf("authenticated=%v, esperaba true", m["authenticated"])
	}
	if m["name"] != "davantis" {
		t.Errorf("name=%v, esperaba davantis", m["name"])
	}
	if m["project_id"] != "musubi" {
		t.Errorf("project_id=%v, esperaba musubi", m["project_id"])
	}
	if m["read"] != ReadAll || m["write"] != WriteOwn {
		t.Errorf("caps read=%v write=%v, esperaba all/own", m["read"], m["write"])
	}
}

// El admin legacy (token único sin persona) NO tiene autor: name vacío.
func TestWhoamiLegacyHasNoPerson(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	p := &Principal{Name: "legacy", Role: RoleAdmin, Read: ReadAll, Write: WriteAny}
	m := whoamiResult(t, s, p)
	if m["authenticated"] != true {
		t.Errorf("authenticated=%v, esperaba true", m["authenticated"])
	}
	if m["name"] != "" {
		t.Errorf("name=%v, esperaba vacío (legacy sin persona)", m["name"])
	}
}

// stdio local (sin principal): no autenticado, alcance local.
func TestWhoamiStdioLocal(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	m := whoamiResult(t, s, nil)
	if m["authenticated"] != false {
		t.Errorf("authenticated=%v, esperaba false", m["authenticated"])
	}
	if m["scope"] != "local" {
		t.Errorf("scope=%v, esperaba local", m["scope"])
	}
}
