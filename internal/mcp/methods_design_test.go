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

// TestDesignPrefiereTarjetasSobreBlobs valida F4: en el brief, una tarjeta CURADA (destilada) le gana el
// lugar a un artículo CRUDO (ingested/) que matchea la misma consulta — así el destilado se surfacea.
func TestDesignPrefiereTarjetasSobreBlobs(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Un blob crudo y una tarjeta curada, AMBOS con el término "tabla".
	if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "blob1", "ingested/article/x",
		"Artículo largo sobre diseño de tabla tabla tabla con muchas filas y columnas.", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "card1", "design-corpus/tabla-densa",
		"PATRÓN — TABLA DENSA: fila compacta, números a la derecha con tabular-nums. (de: tabla)", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	brief := callDesign(t, s, nil, "tabla", "web")
	if len(brief.Corpus) == 0 {
		t.Fatal("el corpus vino vacío")
	}
	// La tarjeta curada debe estar, y ANTES del blob (o el blob ni aparece dentro del límite).
	var idxCard, idxBlob = -1, -1
	for i, h := range brief.Corpus {
		if h.ID == "card1" {
			idxCard = i
		}
		if h.ID == "blob1" {
			idxBlob = i
		}
	}
	if idxCard < 0 {
		t.Errorf("la tarjeta curada (card1) debe aparecer en el corpus; ids=%v", corpusIDs(brief.Corpus))
	}
	if idxBlob >= 0 && idxCard > idxBlob {
		t.Errorf("la tarjeta curada debe ir ANTES del blob crudo (card en %d, blob en %d)", idxCard, idxBlob)
	}
}

// TestDesignMethodFallbackEstatico valida CAPA 2: sin sub-acervo `design-method/*`, los principios salen
// de la const de fallback y method_source es "static" — el brief nunca queda sin método.
func TestDesignMethodFallbackEstatico(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	brief := callDesign(t, s, nil, "un login", "web")
	if brief.MethodSource != "static" {
		t.Errorf("sin sub-acervo de método, method_source debe ser 'static'; fue %q", brief.MethodSource)
	}
	if brief.Principles != designPrinciples {
		t.Errorf("sin sub-acervo, los principios deben ser la const de fallback")
	}
}

// TestDesignMethodDelAcervoVivo valida CAPA 2: con tarjetas `design-method/*` sembradas, los principios
// salen del ACERVO (arbitrable), method_source es "corpus", y respetan el orden por importancia.
func TestDesignMethodDelAcervoVivo(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Método vivo: dos tarjetas, distinta importancia (la de mayor importancia va primero).
	if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "meth-color", "design-method/el-color-se-gana",
		"EL COLOR SE GANA: un acento dominante, el resto neutro; el color no se reparte.", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "meth-ia", "design-method/matar-look-de-ia",
		"MATAR EL LOOK DE IA: evitá el tell del momento (violeta-gradiente, crema+serif); el tell se mueve cada ~18 meses.", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	brief := callDesign(t, s, nil, "un login", "web")
	if brief.MethodSource != "corpus" {
		t.Fatalf("con sub-acervo sembrado, method_source debe ser 'corpus'; fue %q", brief.MethodSource)
	}
	// Desde F1+F2 el método del acervo viaja en `method[]` como MATERIAL CITADO, no dentro de
	// `principles` (que pasó a ser el nucleo estatico del codigo). La capacidad no cambia —el metodo
	// sigue saliendo del acervo y sigue siendo arbitrable— cambia quien AFIRMA cada bloque.
	if len(brief.Method) != 2 {
		t.Fatalf("esperaba las 2 tarjetas del acervo en method[]; hubo %d", len(brief.Method))
	}
	// Orden por importancia: la tarjeta de importancia 1.0 (color) va ANTES que la de 0.5 (ia).
	if !strings.Contains(brief.Method[0].Texto, "EL COLOR SE GANA") {
		t.Errorf("el metodo mas importante debe ir primero; primero fue %.60q", brief.Method[0].Texto)
	}
	if !strings.Contains(brief.Method[1].Texto, "MATAR EL LOOK DE IA") {
		t.Errorf("segunda tarjeta inesperada: %.60q", brief.Method[1].Texto)
	}
	// Cada item declara su procedencia: quien lee el brief tiene derecho a saber quien afirma que.
	for _, m := range brief.Method {
		if !strings.HasPrefix(m.Topic, designMethodPrefix) || m.Fuente != designCorpusScope {
			t.Errorf("material sin procedencia util: topic=%q fuente=%q", m.Topic, m.Fuente)
		}
	}
	// Y el nucleo estatico sigue estando SIEMPRE, venga o no el acervo.
	if !strings.Contains(brief.Principles, "JERARQU") {
		t.Error("principles debe traer el nucleo estatico del codigo")
	}
}

// TestDesignMethodExcluidoDelCorpus valida que el sub-acervo del método NO se duplica: una tarjeta
// `design-method/*` que matchea el pedido viaja como `method[]`, NUNCA como patrón del corpus.
func TestDesignMethodExcluidoDelCorpus(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Método y patrón, ambos con el término "jerarquia" (con NoopProvider el recall cae al FTS).
	if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "meth-jer", "design-method/jerarquia",
		"JERARQUIA: una sola cosa manda por pantalla, jerarquia jerarquia.", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "card-jer", "design-corpus/jerarquia-tabla",
		"PATRÓN — jerarquia en tablas: el total manda, jerarquia jerarquia.", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	brief := callDesign(t, s, nil, "jerarquia", "web")
	// El método aparece en Principles...
	if brief.MethodSource != "corpus" || !strings.Contains(brief.Principles, "una sola cosa manda") {
		t.Errorf("la tarjeta de método debe servir como Principles; source=%q principles=%.120q", brief.MethodSource, brief.Principles)
	}
	// ...pero NUNCA en el corpus de patrones (evita la duplicación).
	for _, h := range brief.Corpus {
		if h.ID == "meth-jer" {
			t.Errorf("la tarjeta design-method/ NO debe aparecer en el corpus; ids=%v", corpusIDs(brief.Corpus))
		}
	}
}

// TestDesignMethodNoStarveaCorpus valida el fix de la revisión adversarial: las tarjetas del método
// (design-method/*) viven en el MISMO tenant y son densas en keywords de diseño, pero NO deben copar el
// pool de búsqueda y dejar el corpus de patrones vacío con degraded en falso. El pool se agranda para
// dejar lugar a los patrones reales tras excluir el método.
func TestDesignMethodNoStarveaCorpus(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Muchas tarjetas de método, todas densas en el término de la consulta (starvan el pool viejo de 18).
	for i := 0; i < 20; i++ {
		id := "m" + string(rune('a'+i))
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", id, "design-method/regla-"+id,
			"jerarquia densa contraste grilla: regla de metodo numero jerarquia densa", 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	// UN patrón real del corpus con el mismo término: no debe quedar sepultado ni excluido.
	if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "patron1", "design-corpus/tabla-jerarquia",
		"PATRÓN — jerarquia densa en tablas: el total manda, jerarquia densa contraste grilla.", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	brief := callDesign(t, s, nil, "jerarquia densa contraste grilla", "web")
	if brief.Degraded {
		t.Error("el corpus NO debe quedar degraded en falso: las tarjetas de método no deben vaciar el pool")
	}
	found := false
	for _, h := range brief.Corpus {
		if h.ID == "patron1" {
			found = true
		}
		if strings.HasPrefix(h.TopicKey, designMethodPrefix) {
			t.Errorf("una tarjeta de método se coló en el corpus: %s", h.TopicKey)
		}
	}
	if !found {
		t.Errorf("el patrón real (patron1) debe sobrevivir en el corpus pese a las 20 tarjetas de método; ids=%v", corpusIDs(brief.Corpus))
	}
}

func corpusIDs(hits []searchHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.ID
	}
	return out
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
