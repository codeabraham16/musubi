package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// TestToolSyncPullABAcotadoAlProyecto valida que musubi_sync_pull_ab sirve el snapshot del A/B del
// decididor (tabla decididor_ab) ACOTADO al proyecto de la CREDENCIAL: un writer de un tenant NO ve
// las filas de otro. Es el gemelo del aislamiento de TestToolSyncPullScoped para la tabla de
// contadores del A/B, y el ÚNICO test que ejerce el recorte a nivel de la TOOL (R1 de la spec
// ab-lift-bajada); memory/decididor_ab_test prueba el merge y la aritmética, no el handler.
//
// El brazo on/off es irrelevante para el aislamiento: lo que se prueba es que el filtro por
// project_id corta, así que los dos tenants se siembran en el mismo brazo.
//
// Sabotaje: servir el pull con el ctx sin scopear ⇒ scopeClause no filtra y las filas de web se fugan ⇒ rojo.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="rows, err := lector.DecididorAB(s.scopedCtx(ctx))"
// arnes: a="rows, err := lector.DecididorAB(ctx)"
func TestToolSyncPullABAcotadoAlProyecto(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	// Dos tenants con una ocasión cada uno en el A/B. 'acme' es la credencial que pulirá; 'web' es el
	// ajeno que NO debe cruzar. exigio distinto sólo para que las dos filas no sean idénticas.
	must(engine.RegistrarOportunidadDecididor(context.Background(), "acme", memory.BrazoOn, true))
	must(engine.RegistrarOportunidadDecididor(context.Background(), "web", memory.BrazoOn, false))

	pullAB := func(p *Principal) []memory.DecididorABRow {
		args, _ := json.Marshal(map[string]any{})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_sync_pull_ab", Arguments: args})
		out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
		if rpcErr != nil {
			t.Fatalf("pull_ab: %+v", rpcErr)
		}
		var resp struct {
			Rows []memory.DecididorABRow `json:"rows"`
		}
		if uErr := json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &resp); uErr != nil {
			t.Fatalf("unmarshal respuesta: %v", uErr)
		}
		return resp.Rows
	}

	acme := &Principal{Name: "ana", Role: RoleWriter, ProjectID: "acme"}

	// La credencial 'acme' ve SU fila y sólo la suya: la de 'web' queda del otro lado del recorte.
	rows := pullAB(acme)
	if len(rows) != 1 {
		t.Fatalf("pull_ab de acme esperaba 1 fila (sólo acme), obtuve %d: %+v", len(rows), rows)
	}
	if rows[0].ProjectID != "acme" {
		t.Errorf("la fila del pull esperaba project_id=acme, obtuve %q", rows[0].ProjectID)
	}
	for _, r := range rows {
		if r.ProjectID == "web" {
			t.Error("fuga cross-tenant: la fila de 'web' apareció en el pull_ab de 'acme'")
		}
	}
}
