package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// sembrarLlamadas mete invocaciones en el ledger del server de prueba. Usa el motor real (el que
// arma newTestServer), no un fake: el invariante que se prueba acá es que el HANDLER conecta el
// recorte de tenancy con la consulta, y con un fake el cable quedaría sin verificar.
func sembrarLlamadas(t *testing.T, s *McpServer, batch []memory.ToolInvocation) {
	t.Helper()
	e, ok := s.engine.(*memory.DbEngine)
	if !ok {
		t.Fatal("el server de prueba no trae el motor real")
	}
	if err := e.RecordToolInvocations(context.Background(), batch); err != nil {
		t.Fatalf("sembrar ledger: %v", err)
	}
}

// A7 — /api/actores exige credencial, igual que /api/stream y /metrics.
//
// No es simetría por prolijidad: el censo enumera las credenciales que existen y cuánto trabaja
// cada una. Servido sin auth, es un directorio de identidades del cerebro.
func TestA7ElCensoExigeCredencial(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	reg := &PrincipalRegistry{principals: []Principal{
		{Name: "cabina", Role: RoleReader, Read: ReadAll, Write: WriteNone, hash: hashToken("buen-token")},
	}}
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, registry: reg}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/actores")
	if err != nil {
		t.Fatalf("GET /api/actores: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/api/actores sin credencial dio %d, esperaba 401", resp.StatusCode)
	}
}

// A8 — el recorte de tenancy llega HASTA EL HTTP, no sólo hasta la consulta.
//
// A5 (internal/memory) prueba que la consulta sabe acotar. Éste prueba lo otro, que es lo que se
// rompe de verdad: que el handler le PASE el scope. Una consulta que sabe acotar y un handler que
// no le pasa el contexto da 200 con la fuga adentro, y ningún test de la capa de abajo lo ve.
func TestA8ElCensoPorHTTPRespetaLaTenancy(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarLlamadas(t, s, []memory.ToolInvocation{
		{Tool: "musubi_recall", Outcome: memory.OutcomeOK, Duration: time.Millisecond, Principal: "gio", ProjectID: "lastchaos"},
		{Tool: "musubi_recall", Outcome: memory.OutcomeOK, Duration: time.Millisecond, Principal: "davantis-altura", ProjectID: "altura"},
	})
	reg := &PrincipalRegistry{principals: []Principal{
		{Name: "gabriel", ProjectID: "altura", Read: ReadOwn, Write: WriteOwn, hash: hashToken("token-altura")},
		{Name: "mando", ProjectID: "", Read: ReadAll, Write: WriteAny, hash: hashToken("token-mando")},
	}}
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, registry: reg}))
	defer ts.Close()

	censo := func(token string) censoActores {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/actores?days=30", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/actores: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/actores dio %d", resp.StatusCode)
		}
		var c censoActores
		if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
			t.Fatalf("respuesta no parsea: %v", err)
		}
		return c
	}

	acotado := censo("token-altura")
	for _, a := range acotado.Actores {
		if a.Principal == "gio" {
			t.Fatalf("FUGA DE TENANCY POR HTTP: un principal de «altura» vio al actor de «lastchaos»: %+v", acotado.Actores)
		}
	}
	if len(acotado.Actores) != 1 {
		t.Fatalf("el acotado esperaba 1 actor, obtuvo %d: %+v", len(acotado.Actores), acotado.Actores)
	}

	// Y el contrapeso: sin este lado, un handler que devuelva SIEMPRE cero pasaría la mitad de
	// arriba. El federado tiene que ver a los dos.
	todo := censo("token-mando")
	if len(todo.Actores) != 2 {
		t.Fatalf("el federado esperaba 2 actores, obtuvo %d: %+v", len(todo.Actores), todo.Actores)
	}
}

// A9 — la taxonomía de sondeo viaja CON la respuesta.
//
// El panel tiene que poder decir «esto lo clasificó el servidor, y así». Si el panel mantuviera
// su propia lista, se separarían con el primer alta de tool y las dos vistas contarían distinto
// el mismo evento — sin que nada falle, que es la peor forma de estar mal.
func TestA9LaTaxonomiaDeSondeoViajaConElCenso(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, loopbackOnly: true}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/actores")
	if err != nil {
		t.Fatalf("GET /api/actores: %v", err)
	}
	defer resp.Body.Close()
	var c censoActores
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		t.Fatalf("respuesta no parsea: %v", err)
	}
	if len(c.Sondeo) != len(toolsDeSondeo) {
		t.Fatalf("la taxonomía llegó con %d tools, el servidor clasifica con %d", len(c.Sondeo), len(toolsDeSondeo))
	}
	for _, tool := range c.Sondeo {
		if !toolsDeSondeo[tool] {
			t.Errorf("la respuesta declara %q como sondeo y el servidor no lo clasifica así", tool)
		}
	}
}
