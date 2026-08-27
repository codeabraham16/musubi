package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// read_surface_class_test.go SELLA POR CONTRATO la clase "superficie de lectura aislada por
// proyecto" (Track 19). La leccion de tres auditorias: scopear tool-por-tool siempre deja una
// hermana federada (detect_changes, telemetria, resolve_skills aparecieron una por auditoria). En
// vez de perseguirlas de a una, este archivo:
//   1. BARRE todas las superficies de lectura sobre tablas con project_id con datos cross-tenant
//      sembrados y falla si el marcador del otro tenant aparece (TestReadSurfaceClassIsolation).
//   2. Exige que TODA tool readOnly registrada este CLASIFICADA (cubierta por el barrido, o en la
//      allowlist de "no lee tablas scopeadas") — asi una tool de lectura nueva NO puede colarse sin
//      que un humano decida si necesita scope (TestEveryReadOnlyToolClassified).

// seedVictim siembra datos del proyecto "web" con marcadores distintivos en cada tabla scopeada.
func seedVictim(t *testing.T, e *memory.DbEngine) {
	t.Helper()
	if err := e.SaveObservationTypedFrom("web", "", "web-obs-1", "web/topic", "VICTIMOBS sobre shared/auth.go", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveTelemetryLogFrom("web", "shared/auth.go", "VICTIMTELEM boom", "fix"); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveSkillDecisionFrom("web", "web-skill", "WebSkill", "rejected", "r"); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveCodeMemoryFrom("web", memory.CodeMemory{Path: "shared/auth.go", Gist: "VICTIMGIST", Fingerprint: "h", Tokens: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SaveFactFrom("web", "SharedEntity", "relates_to", "VICTIMFACT", "", nil); err != nil {
		t.Fatal(err)
	}
	// Flota de web (track «Control de flota», S2): el INVENTARIO DE MÁQUINAS de otro proyecto
	// —cuántas tiene, cómo se llaman, en qué IP viven— es reconocimiento puro. Que se filtre es
	// peor que una fuga de memoria: le dibuja a un tenant el mapa de la infraestructura de otro.
	if victima, err := e.AltaDevice(fleet.Device{
		Name: "VICTIMDEVICE", ProjectID: "web", Tier: fleet.TierAgente,
		// La máquina víctima admite metrics Y exec: el barrido cubre las dos superficies, y
		// sin exec en el DEVICE la compuerta cortaría por C5 (el aparato) antes que por la
		// tenencia — probando otra vez la defensa equivocada.
		Caps: []fleet.Cap{fleet.CapMetrics, fleet.CapExec, fleet.CapScreen}, OS: "VICTIMOS",
	}, "token-del-device-victima"); err != nil {
		t.Fatal(err)
	} else {
		// Y su TELEMETRÍA (S4). El uso de recursos de la infraestructura de otro proyecto dice
		// más de su negocio que el nombre de las máquinas: cuántos servidores, qué tan cargados,
		// a qué hora. El marker va en `os`, que es lo del device que llega a la fila de métricas.
		if _, err := e.LatirDevice(victima.ID, time.Now(), `{"tomada":"2026-08-26T12:00:00Z","num_cpu":64,"load1":42.5,"mem_total":999,"mem_usada":1}`); err != nil {
			t.Fatal(err)
		}
		// Y su BITÁCORA DE EJECUCIÓN (S5). Qué comandos corre otro equipo en su infraestructura
		// es lo más revelador de todo el track: nombres de servicios, rutas, scripts internos.
		if _, err := e.EncolarComando(fleet.Comando{
			DeviceID: victima.ID, ProjectID: "web", Principal: "alguien",
			Argv: []string{"/opt/VICTIMSCRIPT.sh"}, Timeout: 30 * time.Second,
		}); err != nil {
			t.Fatal(err)
		}
		// Y su bitácora de PANTALLA (S6). Quién mira la pantalla de qué máquina en otro tenant
		// es información de personas, no sólo de infraestructura.
		if _, err := e.AbrirSesionPantalla(fleet.SesionPantalla{
			DeviceID: victima.ID, ProjectID: "web", Principal: "VICTIMMIRON",
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Ledger de uso de web (F0): el patrón de uso de OTRO proyecto —qué herramientas usa y con
	// qué frecuencia— es información de negocio y no debe cruzarse. El marker va en la columna
	// `tool`, que es lo único del ledger que llega a la respuesta.
	if err := e.RecordToolInvocations(context.Background(), []memory.ToolInvocation{
		{Tool: "VICTIMTOOL", Outcome: memory.OutcomeOK, ProjectID: "web"},
	}); err != nil {
		t.Fatal(err)
	}
	// Contadores del arsenal de web (§7 «Forja global»): qué skills usa OTRO equipo y con qué
	// frecuencia es el mismo tipo de información de negocio que el ledger de tools. El marker va
	// en `skill`, y llega a la respuesta porque las skills con contadores que ya no están
	// instaladas se listan POR NOMBRE en vez de contarse como un número.
	if err := e.RecordSkillEvents(context.Background(), []memory.SkillEvent{
		{Skill: "VICTIMSKILL", ProjectID: "web", Evidence: memory.EvidenciaGlob, Kind: memory.UsoResuelta},
	}); err != nil {
		t.Fatal(err)
	}
	// Grafo de código de web (Track 20 F2): VictimCaller --CALLS--> VictimCallee. Los markers son
	// el EXTREMO OPUESTO al que se consulta (así no se filtran por el eco del arg): code_graph pide
	// VictimCaller y el marker es VictimCallee; impact pide VictimCallee y el marker es VictimCaller.
	if err := e.UpsertPackageGraphFrom("web", []string{"shared/auth.go"},
		[]memory.GraphNode{
			{Key: "shared/auth.go#func:VictimCaller", Kind: "func", Name: "VictimCaller", Path: "shared/auth.go", SrcFingerprint: "h"},
			{Key: "shared/auth.go#func:VictimCallee", Kind: "func", Name: "VictimCallee", Path: "shared/auth.go", SrcFingerprint: "h"},
		},
		[]memory.GraphEdge{
			{FromKey: "shared/auth.go#func:VictimCaller", ToKey: "shared/auth.go#func:VictimCallee", Kind: "CALLS", Confidence: 1, Provenance: "EXTRACTED", SrcPath: "shared/auth.go", SrcFingerprint: "h"},
		}); err != nil {
		t.Fatal(err)
	}
}

// readSweepCase es una superficie de lectura y el marcador del tenant "web" que NO debe filtrar.
type readSweepCase struct {
	tool   string
	args   map[string]any
	marker string // string distintivo del tenant web que un atacante NO debe ver
}

// readSweepCases enumera las superficies de lectura marker-in-response cubiertas por el barrido.
// AGREGAR aca toda tool de lectura nueva que consulte una tabla con project_id (ver el guard de
// completitud abajo, que falla si una readOnly nueva no esta clasificada).
func readSweepCases() []readSweepCase {
	return []readSweepCase{
		{"musubi_recall", map[string]any{"query": "shared auth"}, "VICTIMOBS"},
		{"musubi_search_keyword", map[string]any{"query_text": "VICTIMOBS"}, "VICTIMOBS"},
		{"musubi_memory_expand", map[string]any{"ids": []string{"web-obs-1"}}, "VICTIMOBS"},
		{"musubi_recall_facts", map[string]any{"entity": "SharedEntity"}, "VICTIMFACT"},
		{"musubi_entity_context", map[string]any{"entity": "SharedEntity"}, "VICTIMFACT"},
		{"musubi_recall_code", map[string]any{"path": "shared/auth.go"}, "VICTIMGIST"},
		{"musubi_insights", map[string]any{}, "shared/auth.go"}, // hotspot file_path de la telemetria de web
		{"musubi_resolve_skills", map[string]any{"modified_files": []string{"auth.go"}}, "VICTIMTELEM"},
		{"musubi_code_graph", map[string]any{"symbol": "shared/auth.go#func:VictimCaller"}, "VictimCallee"},
		{"musubi_impact", map[string]any{"symbol": "shared/auth.go#func:VictimCallee"}, "VictimCaller"},
		{"musubi_map", map[string]any{}, "Victim"},
		{"musubi_tool_usage", map[string]any{}, "VICTIMTOOL"},
		{"musubi_skill_usage", map[string]any{}, "VICTIMSKILL"},
		// code_context: el weld deriva explained_by de la obs de web (topic_key web/topic) por el path.
		{"musubi_code_context", map[string]any{"symbol": "shared/auth.go#func:VictimCaller"}, "web/topic"},
		// grafos renderizables completos (Track 20): brain_graph lee observations (marker VICTIMOBS),
		// code_graph_viz lee code_graph_nodes (marker el nombre de nodo de web).
		{"musubi_brain_graph", map[string]any{}, "VICTIMOBS"},
		{"musubi_code_graph_viz", map[string]any{}, "VictimCaller"},
		// El inventario de la flota. Se pasa `project` a propósito: es el caso HOSTIL —el
		// atacante DECLARA el tenant ajeno— y fleetReadScopeFor tiene que ignorarlo por ser
		// read=own, mientras el admin federado (read=all) sí puede mirarlo.
		{"musubi_fleet_list", map[string]any{"project": "web"}, "VICTIMDEVICE"},
		// La telemetría del tenant ajeno. El caso hostil es el mismo: el atacante DECLARA el
		// proyecto de la víctima y la compuerta tiene que ignorarlo.
		{"musubi_fleet_metrics", map[string]any{"project": "web"}, "VICTIMOS"},
		// La bitácora del tenant ajeno. Mismo caso hostil: el atacante DECLARA el proyecto.
		{"musubi_fleet_log", map[string]any{"project": "web"}, "VICTIMSCRIPT"},
		{"musubi_fleet_sessions", map[string]any{"project": "web"}, "VICTIMMIRON"},
	}
}

func TestReadSurfaceClassIsolation(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	seedVictim(t, engine)

	respText := func(tool string, args map[string]any, p *Principal) string {
		raw, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: raw})
		out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
		if rpcErr != nil {
			t.Fatalf("%s: %+v", tool, rpcErr)
		}
		return out.(CallToolResponse).Content[0].Text
	}

	// AL ATACANTE SE LE DA LA CAPACIDAD DE FLOTA A PROPÓSITO, y es la diferencia entre un test
	// que prueba lo que dice y uno que pasa por la razón equivocada.
	//
	// Las tools de flota tienen DOS defensas encima: la tenencia (este barrido) y la compuerta
	// de capacidades de S3. Si el atacante no tuviera `metrics`, la compuerta lo frenaría
	// primero y el barrido quedaría verde AUNQUE la tenencia estuviera rota — probando la
	// defensa equivocada. Con `metrics: ["*"]` en la mano, lo único que se interpone entre él y
	// la telemetría de `web` es el aislamiento por proyecto, que es lo que este barrido existe
	// para custodiar.
	//
	// El admin federado necesita las mismas concesiones por el otro lado: sin ellas el control
	// («el dato existe y el filtro no rompe legacy») no podría verlo nunca. El rol admin NO
	// otorga capacidades de flota — ésa es la valla C1 del track — así que hay que declararlas.
	grantsDeFlota := map[fleet.Cap][]string{
		fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}, fleet.CapScreen: {"*"},
	}
	crm := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm", Fleet: grantsDeFlota} // atacante: otro proyecto
	admin := &Principal{Name: "root", Role: RoleAdmin, Fleet: grantsDeFlota}                   // federado: control

	for _, tc := range readSweepCases() {
		// El atacante (crm) NUNCA debe ver el marcador de web.
		if got := respText(tc.tool, tc.args, crm); strings.Contains(got, tc.marker) {
			t.Errorf("FUGA cross-tenant en %s: un writer/crm vio el marcador %q de web\nrespuesta: %s", tc.tool, tc.marker, got)
		}
		// El admin federado SÍ debe verlo (prueba que el dato existe y el filtro no rompe legacy).
		if got := respText(tc.tool, tc.args, admin); !strings.Contains(got, tc.marker) {
			t.Errorf("%s: el admin federado deberia ver el marcador %q de web (seed/legacy roto)\nrespuesta: %s", tc.tool, tc.marker, got)
		}
	}
}

// TestEveryReadOnlyToolClassified es el GUARD DE COMPLETITUD: toda tool readOnly registrada debe
// estar cubierta por el barrido de aislamiento, o declarada explicitamente como "no lee tablas
// scopeadas". Asi una tool de lectura nueva NO puede agregarse sin que alguien decida si necesita
// scope de proyecto — cierra el whack-a-mole por contrato, no por vigilancia.
func TestEveryReadOnlyToolClassified(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Cubiertas por el barrido de aislamiento (leen tablas con project_id).
	swept := map[string]bool{}
	for _, tc := range readSweepCases() {
		swept[tc.tool] = true
	}
	// Cubiertas por barrido/aislamiento en OTROS tests dedicados (por args especiales).
	for _, name := range []string{
		"musubi_search_semantic", // scope a nivel motor (scope_isolation_test); necesita embedder
		"musubi_conflicts",       // conflicts_isolation_test (JOIN a observations)
		"musubi_detect_changes",  // methods_detect_test (necesita git runner)
		"musubi_search_skills",   // behavior-bleed via GetSkillDecisionsCtx (Track 19); no marker-in-text
		"musubi_sync_pull",       // aislamiento por credencial verificado en sync_pull_test (TestToolSyncPullScoped)
		// readiness_medido_test (TestReadinessAcotadaAlProyecto). NO entra al barrido por marcador
		// porque no devuelve NINGÚN texto del dato: sólo agregados. Eso no la exime — filtrar
		// «cuántas veces invocó el otro equipo» es la misma información de negocio que filtrar QUÉ
		// invocó —, así que su test dedicado usa este mismo seedVictim y exige que las cuatro
		// dimensiones scopeadas den CERO para el tenant vecino y NO cero para el admin federado.
		"musubi_readiness",
		// musubi_design lee la tabla observations pero con scope FIJO al acervo COMPARTIDO
		// `musubi-design`, nunca al proyecto del caller — como el arsenal de skills, es un pozo de
		// conocimiento compartido a propósito, así que no hay dato por-tenant que aislar. NO entra al
		// barrido por marcador porque un admin federado tampoco vería el marker de web (el scope es
		// fijo, no federado): su test dedicado (TestDesignBriefTraeNucleoYAcervoScopeado) siembra un
		// señuelo en otro tenant y exige que el corpus salga SÓLO de `musubi-design`.
		"musubi_design",
	} {
		swept[name] = true
	}
	// readOnly que NO leen ninguna tabla con project_id (catalogo remoto, estado, salud, etc.):
	// no necesitan scope de proyecto. Si una de estas empieza a leer datos scopeados, MOVERLA arriba.
	noScopedRead := map[string]bool{
		"musubi_discover_skills": true, // catalogo remoto/marketplace
		"musubi_detect_stack":    true, // inspecciona el filesystem del proyecto local
		// Lee .musubi/skills/*.yaml del disco, igual que detect_stack. El arsenal del central es
		// COMPARTIDO a proposito (arsenal de empresa), asi que no hay dato de tenant que aislar. Lo
		// que si es por-proyecto son las DECISIONES sobre skills, y esta tool no las mira: si algun
		// dia filtrara por ellas, hay que MOVERLA al barrido de aislamiento.
		"musubi_list_skills": true,
		"musubi_tokens":      true, // ledger de la sesion
		"musubi_sync_status": true, // estado del outbox (no por-proyecto)
		"musubi_phase":       true, // pipeline de fases de la sesion
		"musubi_whoami":      true, // identidad del propio principal (nunca datos de otro tenant)
		// Contadores EN MEMORIA del proceso (F5): no lee ninguna tabla, así que no hay nada que
		// scopear. Y por invariante D5 nunca contiene un secreto, sólo conteos y TIPOS.
		"musubi_cognition_stats": true,
	}

	for i := range s.tools {
		tl := &s.tools[i]
		if !tl.readOnly {
			continue
		}
		if swept[tl.Name] || noScopedRead[tl.Name] {
			continue
		}
		t.Errorf("tool readOnly %q SIN clasificar: agregala al barrido de aislamiento (readSweepCases) si lee una tabla con project_id, o a noScopedRead si no", tl.Name)
	}
}
