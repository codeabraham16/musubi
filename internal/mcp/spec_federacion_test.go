package mcp

// Federación opt-in por la TOOL (spec-graph F4): lo que el handler garantiza en el BORDE MCP, su sitio
// de declaración. El core (IsShared/ValidarShared/RenderProjection) y el motor (ProjectSpecShared/
// RetractSpecProjection/Reconcile) ya tienen su batería en specgraph/ y memory/; acá se fija lo que SÓLO
// vive en el handler: que create NO proyecta sin `shared` (R2/E1, el invariante primario, va primero) y
// el ciclo de vida completo create→update→delete a través del despacho real (handleToolsCall, con su
// candado write), no llamando al motor por atrás.
//
// Determinismo del id de proyección: SetProjectID("") fuerza el tenant propio del motor a "", así que la
// obs derivada queda en `spec-proj::<spec_id>` (specProjectionID con project_id vacío). Por eso el test
// afirma contra "spec-proj::"+id sin reconstruir la lógica del motor desde el paquete mcp.

import (
	"testing"

	"musubi/internal/memory"
)

// TestSpecCreateSinSharedNoProyecta fija R2/E1 —EL invariante primario de F4, «qué NO sale»— en su sitio
// de declaración: una spec creada SIN `shared` (o con `shared:false`), en un nodo con destino, no proyecta
// nada: ni obs `spec-proj:…` ni fila de outbox. La federación es opt-in; el default silencioso es local, y
// eso es lo que importa (qué NO sale de la máquina).
//
// Sabotaje que lo pone en ROJO: invertir la compuerta de IsShared (`v == "true"` → `v != "true"`), con lo
// que `shared:false` pasa a contar como federable y el create proyecta la obs derivada: existeObs la ve.
// La obs se escribe aunque el outbox esté apagado (el gate de outboxEnabled es sólo del enqueue), así que
// el sabotaje enrojece por existeObs con independencia del destino —no es vacuo—. Ataca exactamente la
// compuerta que decide qué sale de la máquina.
// arnes: archivo="internal/specgraph/parse.go"
// arnes: de="return ok && v == \"true\""
// arnes: a="return ok && v != \"true\""
func TestSpecCreateSinSharedNoProyecta(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	eng, ok := s.engine.(*memory.DbEngine)
	if !ok {
		t.Fatalf("el engine del test no es *memory.DbEngine: %T", s.engine)
	}
	eng.SetProjectID("") // tenant propio = "" ⇒ id determinista "spec-proj::<id>"

	// Base del outbox ANTES de crear: el afirmado es un DELTA (DirSembrado puede sembrar filas), así el
	// invariante «no encola nada» no depende de cuántas había.
	base, _, _, err := eng.OutboxStats()
	if err != nil {
		t.Fatal(err)
	}

	// shared:false explícito y shared ausente: los dos caminos del default local.
	casos := []struct {
		nombre string
		args   map[string]interface{}
		id     string
	}{
		{"shared-false", map[string]interface{}{
			"path": "specs/nf/SPEC.md", "id": "s-nofed", "type": "goal-and-requirements", "title": "No Fed",
			"shared": false,
		}, "s-nofed"},
		{"sin-shared", map[string]interface{}{
			"path": "specs/au/SPEC.md", "id": "s-ausente", "type": "goal-and-requirements", "title": "Ausente",
		}, "s-ausente"},
	}
	for _, c := range casos {
		if _, e := call(t, s, "musubi_spec_create", c.args); e != nil {
			t.Fatalf("[%s] create no debía fallar: %+v", c.nombre, e)
		}
		if existeObs(t, eng, "spec-proj::"+c.id) {
			t.Errorf("[%s] una spec no-shared NO debía crear la obs proyectada (R2)", c.nombre)
		}
	}
	// El carril de federación quedó intacto: ninguna fila de outbox NUEVA.
	pending, _, _, err := eng.OutboxStats()
	if err != nil {
		t.Fatal(err)
	}
	if pending != base {
		t.Errorf("una spec no-shared NO debía encolar nada al central: pending %d→%d (R2)", base, pending)
	}
}

// TestSpecSharedCicloDeVidaPorLaTool es cobertura de comportamiento del borde: el ciclo de vida completo
// de la federación por las tools reales (despacho handleToolsCall con candado write), de punta a punta. No
// lleva ancla mecanizada —las compuertas de create/update/delete están mecanizadas arriba, y los
// invariantes del motor en las unidades de memory/—; acá se fija que las transiciones ENCADENAN: nacer
// shared proyecta, des-federar retira, re-federar vuelve a proyectar, y borrar retira.
func TestSpecSharedCicloDeVidaPorLaTool(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	eng, ok := s.engine.(*memory.DbEngine)
	if !ok {
		t.Fatalf("el engine del test no es *memory.DbEngine: %T", s.engine)
	}
	eng.SetProjectID("")
	const projID = "spec-proj::s-fed"

	// Nace shared:true ⇒ proyecta.
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/fed/SPEC.md", "id": "s-fed", "type": "module-design", "title": "Federable",
		"shared": true,
	}); e != nil {
		t.Fatalf("create shared:true falló: %+v", e)
	}
	if !existeObs(t, eng, projID) {
		t.Fatal("una spec nacida shared:true debía proyectarse al central")
	}

	// update shared:false ⇒ des-federa (retira la proyección local, R9).
	if _, e := call(t, s, "musubi_spec_update", map[string]interface{}{
		"id": "s-fed", "shared": false,
	}); e != nil {
		t.Fatalf("update shared:false falló: %+v", e)
	}
	if existeObs(t, eng, projID) {
		t.Fatal("des-federar (shared:false) debía retirar la proyección local (R9)")
	}

	// update shared:true ⇒ re-federa (vuelve a proyectar por id determinista, R6).
	if _, e := call(t, s, "musubi_spec_update", map[string]interface{}{
		"id": "s-fed", "shared": true,
	}); e != nil {
		t.Fatalf("update shared:true falló: %+v", e)
	}
	if !existeObs(t, eng, projID) {
		t.Fatal("re-federar (shared:true) debía volver a proyectar")
	}

	// delete ⇒ retira SIEMPRE (no puede leer el frontmatter, ya no existe; es idempotente, R9).
	if _, e := call(t, s, "musubi_spec_delete", map[string]interface{}{"id": "s-fed"}); e != nil {
		t.Fatalf("delete falló: %+v", e)
	}
	if existeObs(t, eng, projID) {
		t.Fatal("borrar la spec debía retirar su proyección local (R9)")
	}
}
