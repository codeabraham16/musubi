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
