package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// callDesign invoca musubi_design con el prompt/target dados bajo el principal p (nil = stdio local)
// y devuelve el brief parseado. Falla el test si el dispatch devuelve error RPC.
func callDesign(t *testing.T, s *McpServer, p *Principal, prompt, target string) designBrief {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"prompt": prompt, "target": target})
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_design", Arguments: raw})
	ctx := context.Background()
	if p != nil {
		ctx = withPrincipal(ctx, p)
	}
	out, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr != nil {
		t.Fatalf("musubi_design: %+v", rpcErr)
	}
	resp := out.(CallToolResponse)
	var brief designBrief
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &brief); err != nil {
		t.Fatalf("parse brief: %v", err)
	}
	return brief
}

// TestDesignBriefTraeNucleoYAcervoScopeado valida las dos garantías del motor: (1) el brief SIEMPRE
// trae el núcleo estático (rol + principios + marca), aún sin acervo; (2) el corpus se lee SÓLO del
// tenant `musubi-design`, sin filtrar memoria de otros proyectos, sin importar el caller.
func TestDesignBriefTraeNucleoYAcervoScopeado(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	seed := func(origin, id, content string) {
		if err := engine.SaveObservationTypedFrom(origin, "", id, "diseno/patron", content, 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	// Un patrón en el acervo de diseño + un señuelo con el MISMO término en otro tenant.
	seed(designCorpusScope, "d1", "patron de dashboard con jerarquia fuerte y grilla de 12 columnas")
	seed("crm", "c1", "dashboard interno del crm de otro equipo, nada que ver con el acervo")

	// Sin embedder (NoopProvider) el motor cae al FTS: sirve para probar el scope del acervo.
	brief := callDesign(t, s, &Principal{Name: "alguien", Role: RoleWriter, ProjectID: "altura"}, "dashboard", "any")

	// (1) núcleo estático siempre presente.
	if brief.Role == "" || brief.Principles == "" || brief.Brand == "" {
		t.Errorf("el brief debe traer rol/principios/marca aunque el acervo esté vacío; role=%q principles=%q brand=%q",
			brief.Role, brief.Principles, brief.Brand)
	}
	if brief.CorpusScope != designCorpusScope {
		t.Errorf("corpus_scope=%q, esperaba %q", brief.CorpusScope, designCorpusScope)
	}

	// (2) el corpus salió SÓLO del acervo de diseño, no del tenant crm.
	ids := map[string]bool{}
	for _, h := range brief.Corpus {
		ids[h.ID] = true
	}
	if !ids["d1"] {
		t.Errorf("esperaba el patrón del acervo (d1) en el corpus; obtuve %v", ids)
	}
	if ids["c1"] {
		t.Errorf("FUGA DE TENANT: el corpus trajo memoria de crm (c1); el acervo debe scopear a %q", designCorpusScope)
	}
}

// TestDesignTargetOrientaLaEntrega valida que el target elige el bloque 'emit' correcto y que un
// target desconocido cae a 'any'.
func TestDesignTargetOrientaLaEntrega(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	cases := []struct {
		in       string
		wantTgt  string
		wantSubs string
	}{
		{"painter", "painter", "SPEC JSON"},
		{"web", "web", "Tailwind"},
		{"html", "html", "headless"},
		{"cualquier-cosa", "any", "portable"},
	}
	for _, c := range cases {
		brief := callDesign(t, s, nil, "una pantalla de login", c.in)
		if brief.Target != c.wantTgt {
			t.Errorf("target %q → %q, esperaba %q", c.in, brief.Target, c.wantTgt)
		}
		if !strings.Contains(brief.Emit, c.wantSubs) {
			t.Errorf("emit para %q no menciona %q: %q", c.wantTgt, c.wantSubs, brief.Emit)
		}
	}
}

// TestDesignEsLlamablePorReaderYCabina valida que, por ser readOnly, la puede invocar un principal
// write=none (la cabina) — el uso pedido: diseñar "estando donde sea", incluso sin poder mutar.
func TestDesignEsLlamablePorReaderYCabina(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Cabina: read=all, write=none. Antes fallaría si la tool no fuera readOnly.
	cabina := &Principal{Name: "cabina", Role: RoleReader, Read: ReadAll, Write: WriteNone}
	brief := callDesign(t, s, cabina, "un panel de administración", "web")
	if brief.Role == "" {
		t.Error("la cabina (write=none) debe poder llamar a musubi_design y recibir el brief")
	}
}

// callDesignBrand invoca musubi_design con prompt/target/brand bajo el principal p y devuelve el brief.
func callDesignBrand(t *testing.T, s *McpServer, p *Principal, prompt, target, brand string) designBrief {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"prompt": prompt, "target": target, "brand": brand})
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_design", Arguments: raw})
	ctx := context.Background()
	if p != nil {
		ctx = withPrincipal(ctx, p)
	}
	out, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr != nil {
		t.Fatalf("musubi_design: %+v", rpcErr)
	}
	var brief designBrief
	if err := json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &brief); err != nil {
		t.Fatalf("parse brief: %v", err)
	}
	return brief
}

// TestDesignMarcaPorProyectoNoSeCruza valida la CAPA 3 (Musubi Renaissance F1): la marca activa se
// resuelve por el proyecto del principal, un cliente ve SU marca, Musubi ve la suya (índigo por default),
// y un proyecto SIN marca NO hereda la identidad de nadie (ni Musubi ni otro cliente) — no se cruza.
func TestDesignMarcaPorProyectoNoSeCruza(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Marca propia del cliente 'acme' (identidad distinta de Musubi).
	if err := engine.SaveObservationTypedFrom("acme", "", "acme-brand", brandTopicKey,
		"MARCA ACME: acento NARANJA #FF6A00, tipografía condensada, look industrial.", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	call := func(project string) designBrief {
		return callDesign(t, s, &Principal{Name: "x", Role: RoleWriter, ProjectID: project}, "un login", "web")
	}

	// El caller de 'acme' ve SU marca (source project), con el naranja.
	a := call("acme")
	if a.BrandSource != "project" || a.BrandScope != "acme" || !strings.Contains(a.Brand, "ACME") {
		t.Errorf("acme debe ver SU marca: source=%q scope=%q brand=%.50q", a.BrandSource, a.BrandScope, a.Brand)
	}
	// El caller de 'musubi' ve la marca Musubi por default (índigo real).
	m := call("musubi")
	if m.BrandSource != "default" || !strings.Contains(m.Brand, "6366F1") {
		t.Errorf("musubi debe ver la marca default índigo: source=%q brand=%.50q", m.BrandSource, m.Brand)
	}
	if strings.Contains(m.Brand, "ACME") || strings.Contains(m.Brand, "FF6A00") {
		t.Error("FUGA cross-marca: musubi vio la identidad de acme")
	}
	// Un proyecto SIN marca no hereda NI la de acme NI la de Musubi: marca neutra, source 'none'.
	o := call("otro")
	if o.BrandSource != "none" {
		t.Errorf("un proyecto sin marca debe ser 'none', fue %q", o.BrandSource)
	}
	// El leak real = heredar la marca APLICADA de otro cliente. (La marca neutra NOMBRA el índigo de
	// Musubi para decir "NO lo uses" — eso es una instrucción de no-cruce, no aplicar la identidad.)
	if strings.Contains(o.Brand, "ACME") || strings.Contains(o.Brand, "FF6A00") {
		t.Errorf("FUGA: un proyecto sin marca heredó la identidad de otro cliente: %.70q", o.Brand)
	}
}

// TestDesignBrandArgSoloReadAll valida que el arg `brand` (diseñar a nombre de otro proyecto) SÓLO lo
// respeta un principal read=all (la sala de mando); un writer acotado lo ignora y usa su propia marca.
func TestDesignBrandArgSoloReadAll(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	if err := engine.SaveObservationTypedFrom("acme", "", "acme-brand", brandTopicKey,
		"MARCA ACME: acento NARANJA #FF6A00.", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	// Sala de mando (read=all) pidiendo brand='acme' ⇒ obtiene la marca de acme.
	admin := &Principal{Name: "mando", Role: RoleAdmin}
	got := callDesignBrand(t, s, admin, "un login", "web", "acme")
	if got.BrandScope != "acme" || !strings.Contains(got.Brand, "ACME") {
		t.Errorf("un read=all con brand='acme' debe traer la marca de acme: scope=%q brand=%.50q", got.BrandScope, got.Brand)
	}
	// Writer acotado a 'otro' pidiendo brand='acme' ⇒ se IGNORA el arg, usa su propio scope.
	scoped := &Principal{Name: "w", Role: RoleWriter, ProjectID: "otro"}
	got2 := callDesignBrand(t, s, scoped, "un login", "web", "acme")
	if got2.BrandScope != "otro" {
		t.Errorf("un writer acotado NO puede declarar marca ajena: esperaba scope 'otro', fue %q", got2.BrandScope)
	}
	if strings.Contains(got2.Brand, "ACME") || strings.Contains(got2.Brand, "FF6A00") {
		t.Error("FUGA: un writer acotado obtuvo la marca de otro proyecto vía el arg brand")
	}
}

// TestDesignEmitRellenaTokens valida F2 (emisión multi-target): con tokens de marca, el `emit` sale
// RELLENO con los hex reales de ESA marca en el dialecto del target; una marca en prosa cae a la guía
// genérica sin valores; y el naranja de un cliente nunca se cruza con el índigo de Musubi.
func TestDesignEmitRellenaTokens(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Marca de 'acme' como DOC JSON de tokens (acento naranja).
	acme := `{"name":"Acme","palette":{"bg":"#101010","ink":"#FFFFFF","accent":"#FF6A00"},"radius":{"surface":6},"elevation":"raised","identity":"MARCA ACME industrial."}`
	if err := engine.SaveObservationTypedFrom("acme", "", "acme-brand", brandTopicKey, acme, 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	// Marca de 'proso' en PROSA (sin tokens estructurados).
	if err := engine.SaveObservationTypedFrom("proso", "", "proso-brand", brandTopicKey, "Marca en prosa, sin tokens estructurados.", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	call := func(project, target string) designBrief {
		return callDesign(t, s, &Principal{Name: "x", Role: RoleWriter, ProjectID: project}, "una pantalla", target)
	}

	// Musubi por default (web): el índigo real RELLENO en las variables CSS.
	m := call("musubi", "web")
	if !strings.Contains(m.Emit, "--accent: #6366F1") {
		t.Errorf("el emit web de Musubi debe rellenar el índigo real; emit=%.220q", m.Emit)
	}
	if m.BrandTokens == nil || m.BrandTokens.Palette["accent"] != "#6366F1" {
		t.Error("brand_tokens de Musubi debe traer accent índigo #6366F1")
	}
	// Musubi (painter): el mismo token mapeado al dialecto del cuerpo.
	if mp := call("musubi", "painter"); !strings.Contains(mp.Emit, "CORD(acento)=#6366F1") {
		t.Errorf("el emit painter de Musubi debe mapear CORD al índigo; emit=%.220q", mp.Emit)
	}
	// Acme (web): SU naranja relleno, y NUNCA el índigo de Musubi.
	a := call("acme", "web")
	if !strings.Contains(a.Emit, "--accent: #FF6A00") {
		t.Errorf("el emit web de acme debe rellenar SU naranja; emit=%.220q", a.Emit)
	}
	if strings.Contains(a.Emit, "#6366F1") {
		t.Error("FUGA: acme recibió el índigo de Musubi en el emit")
	}
	if a.BrandTokens == nil || a.BrandTokens.Palette["accent"] != "#FF6A00" {
		t.Error("brand_tokens de acme debe traer accent naranja #FF6A00")
	}
	// Marca en prosa: sin tokens ⇒ emit genérico (sin :root relleno), pero igual es source 'project'.
	p := call("proso", "web")
	if p.BrandTokens != nil {
		t.Error("una marca en prosa NO debe traer brand_tokens")
	}
	if strings.Contains(p.Emit, ":root {") {
		t.Errorf("una marca en prosa no debe rellenar tokens; emit=%.150q", p.Emit)
	}
	if p.BrandSource != "project" {
		t.Errorf("una marca en prosa igual tiene source 'project', fue %q", p.BrandSource)
	}
}
