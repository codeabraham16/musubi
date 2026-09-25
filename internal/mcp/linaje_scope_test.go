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

// TestLasFuentesDelBriefSeExpandenConLaMismaCredencial: los ids que musubi_design le anuncia a una
// credencial —las `fuentes` de una ficha— se pueden expandir con ESA credencial, aunque sea un
// writer de otro proyecto. musubi_design lee el acervo con un scope fijo; memory_expand leía con el
// de la credencial, y a un writer de crm le devolvía `[]` sin error ni aviso: la nota del brief le
// prometía «expandir uno trae el artículo entero» y no se cumplía justo para él.
//
// La otra mitad es la que cuida la frontera: abrir el acervo no abre otros tenants. El mismo writer
// sigue sin poder expandir una nota de web, y el linaje del artículo no le nombra la ficha de web que
// también salió de él.
//
// Sabotaje que la hace fallar: memory_expand se queda con el alcance pelado de la credencial.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="Federate: fed, Acervo: designCorpusScope})"
// arnes: a="Federate: fed})"
//
// Sabotaje que la hace fallar: la cláusula del acervo abre todos los tenants.
// arnes: archivo="internal/memory/scope.go"
// arnes: de="OR (%s = ? AND %s))"
// arnes: a="OR 1 OR (%s = ? AND %s))"
func TestLasFuentesDelBriefSeExpandenConLaMismaCredencial(t *testing.T) {
	s := acervoDePatrones(t, map[string]string{
		"design-corpus/tabla-densa": "Para una tabla densa, la primera columna ancla la lectura y las numéricas van a la derecha.",
	})
	seed := func(origin, id, topic, texto string) {
		if err := s.engine.SaveObservationTypedFrom(origin, "", id, topic, texto, 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	derivar := func(ficha, blob string) {
		if _, err := s.engine.UpsertObsRelation(memory.ObsRelation{SourceID: ficha, TargetID: blob,
			Relation: memory.RelDerivedFrom, Status: memory.RelStatusResolved}); err != nil {
			t.Fatal(err)
		}
	}
	// acervoDePatrones numera los ids desde «pata»; con una sola entrada, la ficha es «pata».
	seed(designCorpusScope, "fuente-de-pata", "ingested/web/manual", "Un manual de otra cosa: kerning y ligaduras.")
	seed("crm", "nota-de-crm", "notas/propia", "una nota propia del writer")
	seed("web", "nota-de-web", "notas/ajena", "una nota de otro tenant")
	seed("web", "ficha-de-web", "design-corpus/ajena", "una ficha de otro tenant")
	derivar("pata", "fuente-de-pata")
	derivar("ficha-de-web", "fuente-de-pata")

	writer := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}
	expandir := func(ids ...string) string {
		raw, _ := json.Marshal(map[string]any{"ids": ids})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_memory_expand", Arguments: raw})
		out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), writer), params)
		if rpcErr != nil {
			t.Fatalf("expandir %v: %+v", ids, rpcErr)
		}
		return out.(CallToolResponse).Content[0].Text
	}

	b := callDesign(t, s, writer, "tabla densa", "web")
	var fuentes []string
	for _, p := range b.Corpus {
		if p.ID == "pata" {
			fuentes = p.Fuentes
		}
	}
	if strings.Join(fuentes, ",") != "fuente-de-pata" {
		t.Fatalf("control: el brief tiene que anunciarle al writer de crm la fuente de la ficha, o no hay nada que expandir; fuentes=%v corpus=%+v", fuentes, b.Corpus)
	}

	articulo := expandir(fuentes...)
	if !strings.Contains(articulo, "kerning y ligaduras") {
		t.Errorf("el writer de crm tiene que poder expandir la fuente que el brief le anunció: %s", articulo)
	}
	if !strings.Contains(articulo, `"destilado_en"`) || !strings.Contains(articulo, `"pata"`) {
		t.Errorf("el artículo expandido tiene que traer la ficha del acervo que salió de él: %s", articulo)
	}
	if strings.Contains(articulo, "ficha-de-web") {
		t.Errorf("FUGA: por el linaje del acervo, el writer de crm ve el id de una ficha de web: %s", articulo)
	}
	if got := expandir("nota-de-crm"); !strings.Contains(got, "una nota propia del writer") {
		t.Errorf("control: lo propio se sigue expandiendo: %s", got)
	}
	if got := expandir("nota-de-web"); strings.Contains(got, "nota-de-web") {
		t.Errorf("FUGA: abrir el acervo abrió otro tenant; el writer de crm expandió una nota de web: %s", got)
	}
}

// TestDelAcervoAjenoSoloSeExpandeLoVisible: abrir el acervo para seguir las `fuentes` de una ficha no
// abre su cuarentena ni lo que el afilador fundió. musubi_design nunca anuncia esas filas, pero la
// hidratación por id no filtra visibilidad: sin la cláusula, un writer de otro proyecto que conociera
// un id podía leer una propuesta de LLM sin corroborar o una ficha ya archivada del acervo.
//
// Lo PROPIO no cambia, y eso también se mira: la credencial sigue expandiendo su fila archivada como
// antes. El control es el admin federado, que ve las tres filas del acervo: sin él, un silencio podría
// ser «esas filas no existen» y no «la frontera las tapa».
//
// Sabotaje que la hace fallar: la cláusula del acervo deja de exigir visibilidad.
// arnes: archivo="internal/memory/scope.go"
// arnes: de="vis := visibleObsPredicate"
// arnes: a="vis := \"1\""
func TestDelAcervoAjenoSoloSeExpandeLoVisible(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	seed := func(origin, id, topic, texto string) {
		if err := engine.SaveObservationTypedFrom(origin, "", id, topic, texto, 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	seed(designCorpusScope, "acervo-visible", "design-corpus/visible", "TEXTO-ACERVO-VISIBLE")
	seed(designCorpusScope, "acervo-fundida", "design-corpus/fundida", "TEXTO-ACERVO-FUNDIDA")
	if ok, err := engine.ArchiveAsDuplicate(designCorpusScope, "acervo-fundida", "acervo-visible"); err != nil || !ok {
		t.Fatalf("no se pudo fundir la ficha del acervo: ok=%v err=%v", ok, err)
	}
	enCuarentena, err := engine.ProposeObservation(designCorpusScope, "destilador", "design-corpus/propuesta",
		"TEXTO-ACERVO-CUARENTENA", "prueba", 0.5, "semantic", nil)
	if err != nil {
		t.Fatal(err)
	}
	seed("crm", "propia-canonica", "notas/canonica", "TEXTO-PROPIA-CANONICA")
	seed("crm", "propia-archivada", "notas/archivada", "TEXTO-PROPIA-ARCHIVADA")
	if ok, err := engine.ArchiveAsDuplicate("crm", "propia-archivada", "propia-canonica"); err != nil || !ok {
		t.Fatalf("no se pudo fundir la nota propia: ok=%v err=%v", ok, err)
	}

	expandir := func(p *Principal, ids ...string) string {
		raw, _ := json.Marshal(map[string]any{"ids": ids})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_memory_expand", Arguments: raw})
		out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
		if rpcErr != nil {
			t.Fatalf("expandir %v: %+v", ids, rpcErr)
		}
		return out.(CallToolResponse).Content[0].Text
	}
	writer := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}
	admin := &Principal{Name: "root", Role: RoleAdmin}
	delAcervo := []string{"acervo-visible", "acervo-fundida", enCuarentena}

	todo := expandir(admin, delAcervo...)
	for _, texto := range []string{"TEXTO-ACERVO-VISIBLE", "TEXTO-ACERVO-FUNDIDA", "TEXTO-ACERVO-CUARENTENA"} {
		if !strings.Contains(todo, texto) {
			t.Fatalf("control: el admin federado tiene que expandir %s, o esta prueba no mide nada: %s", texto, todo)
		}
	}

	got := expandir(writer, delAcervo...)
	if !strings.Contains(got, "TEXTO-ACERVO-VISIBLE") {
		t.Errorf("la ficha visible del acervo se tiene que seguir expandiendo desde otro proyecto: %s", got)
	}
	if strings.Contains(got, "TEXTO-ACERVO-FUNDIDA") {
		t.Errorf("FUGA: un writer de crm expandió una ficha del acervo que el afilador ya fundió: %s", got)
	}
	if strings.Contains(got, "TEXTO-ACERVO-CUARENTENA") {
		t.Errorf("FUGA: un writer de crm expandió una propuesta del acervo en cuarentena: %s", got)
	}
	if propia := expandir(writer, "propia-archivada"); !strings.Contains(propia, "TEXTO-PROPIA-ARCHIVADA") {
		t.Errorf("lo propio no cambia: la credencial sigue expandiendo su nota archivada: %s", propia)
	}
}
