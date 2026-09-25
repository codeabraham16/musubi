package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// TestElLinajeNoCruzaTenants: el linaje que acompaña a musubi_memory_expand respeta la frontera de
// la credencial en las dos direcciones. Una ficha de crm que salió de un blob de web no le enseña a
// un writer de crm el id del blob de web, y un blob de crm del que salió una ficha de web no le
// enseña esa ficha.
//
// Hace falta aparte porque las guardas de alcance que ya existen (scope_surfaces_test.go y
// read_surface_class_test.go) siembran observaciones SIN aristas: una fuga por el linaje les pasa
// por al lado. La dirección de control es el admin, que SÍ ve las dos puntas ajenas: sin él, un
// verde podría ser «el linaje no anda» y no «el linaje respeta el alcance».
//
// Sabotaje que la hace fallar: el linaje se calcula sin el alcance del ctx.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="sc, scArgs := projectScopeFrom(ctx).scopeClause(\"t\")"
// arnes: a="sc, scArgs := ProjectScope{}.scopeClause(\"t\")"
func TestElLinajeNoCruzaTenants(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	seed := func(origin, id, topic string) {
		if err := engine.SaveObservationTypedFrom(origin, "", id, topic, "contenido sin ids adentro", 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	seed("crm", "iso-ficha-crm", "design-corpus/propia")
	seed("crm", "iso-blob-crm", "ingested/web/propio")
	seed("web", "iso-ficha-web", "design-corpus/ajena")
	seed("web", "iso-blob-web", "ingested/web/ajeno")
	for _, a := range [][2]string{
		{"iso-ficha-crm", "iso-blob-crm"}, // control: lo propio se ve
		{"iso-ficha-crm", "iso-blob-web"}, // ida hacia otro tenant
		{"iso-ficha-web", "iso-blob-crm"}, // vuelta desde otro tenant
	} {
		if _, err := engine.UpsertObsRelation(memory.ObsRelation{SourceID: a[0], TargetID: a[1],
			Relation: memory.RelDerivedFrom, Status: memory.RelStatusResolved}); err != nil {
			t.Fatal(err)
		}
	}

	expandir := func(p *Principal, id string) string {
		raw, _ := json.Marshal(map[string]any{"ids": []string{id}})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_memory_expand", Arguments: raw})
		out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
		if rpcErr != nil {
			t.Fatalf("expandir %s: %+v", id, rpcErr)
		}
		return out.(CallToolResponse).Content[0].Text
	}

	writer := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}
	admin := &Principal{Name: "root", Role: RoleAdmin}

	ida := expandir(writer, "iso-ficha-crm")
	if !strings.Contains(ida, "iso-blob-crm") {
		t.Fatalf("control: la ficha de crm tiene que traer su fuente de crm, o esta prueba no mide nada: %s", ida)
	}
	if strings.Contains(ida, "iso-blob-web") {
		t.Errorf("FUGA: el writer de crm ve el id de un blob de web por el linaje: %s", ida)
	}
	vuelta := expandir(writer, "iso-blob-crm")
	if !strings.Contains(vuelta, "iso-ficha-crm") {
		t.Fatalf("control: el blob de crm tiene que traer su ficha de crm: %s", vuelta)
	}
	if strings.Contains(vuelta, "iso-ficha-web") {
		t.Errorf("FUGA: el writer de crm ve el id de una ficha de web por el linaje: %s", vuelta)
	}

	// El admin es federado: ve las puntas ajenas. Es lo que prueba que el silencio de arriba es el
	// alcance y no un linaje que no devuelve nada.
	if got := expandir(admin, "iso-ficha-crm"); !strings.Contains(got, "iso-blob-web") {
		t.Errorf("control: el admin federado tiene que ver la fuente de web: %s", got)
	}
	if got := expandir(admin, "iso-blob-crm"); !strings.Contains(got, "iso-ficha-web") {
		t.Errorf("control: el admin federado tiene que ver la ficha de web: %s", got)
	}
}
