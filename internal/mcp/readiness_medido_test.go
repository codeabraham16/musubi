package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// readiness_medido_test.go cubre el estado de madurez DESDE LA SUPERFICIE MCP (F3).
//
// Dos cosas que la capa de memoria no puede defender sola: que la tool exista y se anuncie —una
// capacidad que nadie sabe invocar es una capacidad que no existe— y que el puntaje esté ACOTADO
// al proyecto de la credencial. Lo segundo no es cosmético: la tool agrega ledger, conflictos y
// grafo, o sea tres superficies que el repo ya vio filtrar entre tenants. Un puntaje calculado con
// el uso de otro equipo estaría mal Y además contaría cuánto trabaja ese equipo.

func readinessComo(t *testing.T, s *McpServer, p *Principal) memory.ReadinessReport {
	t.Helper()
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_readiness", Arguments: json.RawMessage(`{}`)})
	ctx := context.Background()
	if p != nil {
		ctx = withPrincipal(ctx, p)
	}
	out, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr != nil {
		t.Fatalf("readiness: %+v", rpcErr)
	}
	var rep memory.ReadinessReport
	if err := json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &rep); err != nil {
		t.Fatalf("decodear readiness: %v", err)
	}
	return rep
}

// M1: la tool está anunciada, es read-only y dice la regla que la hace distinta de un cuestionario.
//
// Read-only importa concretamente: la cabina (el CRM, el tablero del cuerpo) tiene write=none, y si
// la tool no estuviera marcada así, su consumidor principal se comería un rechazo por rol.
func TestReadinessAnunciadaYDeLectura(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	var found *toolEntry
	for i := range s.tools {
		if s.tools[i].Name == "musubi_readiness" {
			found = &s.tools[i]
			break
		}
	}
	if found == nil {
		t.Fatal("musubi_readiness no está en el registro: nadie puede invocarla")
	}
	if !found.readOnly {
		t.Error("musubi_readiness muta nada; marcarla read-only es lo que deja que la cabina (write=none) la llame")
	}
	d := found.Description
	for _, must := range []string{"no observada", "CERO", "evidencia"} {
		if !strings.Contains(strings.ToLower(d), strings.ToLower(must)) {
			t.Errorf("la descripción no menciona %q — la regla del cero y la evidencia son el punto entero: %q", must, d)
		}
	}
}

// M2: EL PUNTAJE ESTÁ ACOTADO AL PROYECTO, en las CUATRO dimensiones que leen tablas scopeadas.
//
// Es la contrapartida de dejar esta tool fuera del barrido por marcador de read_surface_class_test:
// no devuelve texto del dato ajeno, sólo agregados, y por eso el barrido no la puede cazar. Pero un
// agregado ajeno filtra igual —«el equipo de al lado invocó 3.000 veces» es información de negocio—
// así que acá se usa el MISMO seedVictim y se exige que el vecino vea cero.
//
// El control positivo (el admin federado SÍ ve el dato) es lo que separa este test de uno que
// pasaría con la tool rota devolviendo ceros para todo el mundo.
func TestReadinessAcotadaAlProyecto(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// El sembrado canónico del barrido: observaciones, ledger, grafo de código y demás, todo de
	// "web". Se le suma una relación de conflicto entre dos observaciones de web.
	seedVictim(t, engine)
	if err := engine.SaveObservationTypedFrom("web", "", "web-obs-2", "web/topic",
		"otra cosa de web sobre shared/auth.go", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.UpsertObsRelation(memory.ObsRelation{
		SourceID: "web-obs-1", TargetID: "web-obs-2", Status: memory.RelStatusPending}); err != nil {
		t.Fatal(err)
	}

	vecino := readinessComo(t, s, &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"})
	federado := readinessComo(t, s, &Principal{Name: "root", Role: RoleAdmin})

	// El vecino no ve NADA de web: las cuatro dimensiones scopeadas quedan sin observar.
	for _, key := range []string{"uso", "confiabilidad", "coherencia", "grafo"} {
		d := dimensionPorClave(t, vecino, key)
		if d.Observed || d.Score != 0 {
			t.Errorf("FUGA cross-tenant en la dimensión %q: crm ve datos de web (observed=%v score=%v, evidencia %+v)",
				key, d.Observed, d.Score, d.Evidence)
		}
	}
	if n, _ := dimensionPorClave(t, vecino, "uso").Evidence["invocaciones"].(float64); n != 0 {
		t.Errorf("crm ve %v invocaciones de web: fuga del patrón de trabajo ajeno", n)
	}
	if n, _ := dimensionPorClave(t, vecino, "coherencia").Evidence["pendientes"].(float64); n != 0 {
		t.Errorf("crm ve %v conflictos de web", n)
	}
	if n, _ := dimensionPorClave(t, vecino, "grafo").Evidence["nodos"].(float64); n != 0 {
		t.Errorf("crm ve %v nodos del grafo de web", n)
	}

	// Control positivo: el dato existe y el federado sí lo ve. Sin esto, una tool rota que
	// devolviera ceros para todos pasaría el test de aislamiento con honores.
	for _, key := range []string{"uso", "coherencia", "grafo"} {
		if d := dimensionPorClave(t, federado, key); !d.Observed {
			t.Errorf("el admin federado debería ver la dimensión %q de web (seed roto o scope de más): %+v", key, d)
		}
	}
	if federado.Score <= vecino.Score {
		t.Errorf("el federado ve datos y el vecino no: su puntaje tiene que ser mayor (%v vs %v)",
			federado.Score, vecino.Score)
	}
}

// M3: la ventana del ledger se puede pedir y se respeta. Sin esto el parámetro sería decorativo:
// devolvería siempre lo mismo y el que lo pasa creería estar mirando otra ventana.
func TestReadinessRespetaLaVentana(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	eng, ok := s.engine.(*memory.DbEngine)
	if !ok {
		t.Skip("el server de test no corre sobre DbEngine")
	}
	lote := make([]memory.ToolInvocation, 0, 12)
	for i := 0; i < 12; i++ {
		lote = append(lote, memory.ToolInvocation{
			Tool:    []string{"musubi_recall", "musubi_doctor", "musubi_insights"}[i%3],
			Outcome: memory.OutcomeOK, Duration: time.Millisecond,
		})
	}
	if err := eng.RecordToolInvocations(context.Background(), lote); err != nil {
		t.Fatalf("RecordToolInvocations: %v", err)
	}

	pedir := func(days int) memory.ReadinessReport {
		t.Helper()
		args, _ := json.Marshal(map[string]interface{}{"days": days})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_readiness", Arguments: args})
		out, rpcErr := s.handleToolsCall(context.Background(), params)
		if rpcErr != nil {
			t.Fatalf("readiness(days=%d): %+v", days, rpcErr)
		}
		var rep memory.ReadinessReport
		json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &rep)
		return rep
	}
	if got := pedir(7).WindowDays; got != 7 {
		t.Errorf("pedí 7 días y el reporte dice %d", got)
	}
	if got := pedir(0).WindowDays; got != 30 {
		t.Errorf("sin ventana debería caer al default de 30, dice %d", got)
	}
}

// M4: una instalación virgen contesta 0 por la tool, con las cinco dimensiones nombradas como no
// observadas. Es el reporte que va a ver el que onboardea un proyecto nuevo, y tiene que decirle
// qué instrumentar — no un cero pelado.
func TestReadinessVirgenPorLaTool(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	rep := readinessComo(t, s, nil)
	if rep.Score != 0 {
		t.Errorf("instalación virgen debe puntuar 0, dio %v", rep.Score)
	}
	if len(rep.Unobserved) != 5 {
		t.Errorf("las 5 dimensiones deberían listarse como no observadas: %v", rep.Unobserved)
	}
	if !strings.Contains(rep.Note, "0") || !strings.Contains(strings.ToLower(rep.Note), "evidencia") {
		t.Errorf("la nota tiene que explicar la regla del cero y la evidencia: %q", rep.Note)
	}
	for _, d := range rep.Dimensions {
		if _, ok := d.Evidence["por_que_cero"]; !ok {
			t.Errorf("la dimensión %q no dice por qué puntúa 0", d.Key)
		}
	}
}

func dimensionPorClave(t *testing.T, rep memory.ReadinessReport, key string) memory.ReadinessDimension {
	t.Helper()
	for _, d := range rep.Dimensions {
		if d.Key == key {
			return d
		}
	}
	t.Fatalf("no existe la dimensión %q", key)
	return memory.ReadinessDimension{}
}
