package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// autonomia_declarada_test.go cubre la escalera de autonomía DESDE LA SUPERFICIE MCP (F2).
//
// La capa de memoria ya tiene sus tests; éstos defienden lo otro, que es donde una capacidad se
// muere en silencio: que el cliente PUEDA declararla (el schema la anuncia y el campo llega hasta
// la fila) y que el freno se sienta a través de la tool, no sólo en Go. Una escalera que el motor
// respeta pero que ningún cliente sabe pedir es una capacidad desplegada e ininvocable — el modo
// de falla que ya nos pasó y que está en docs/failure-modes.md.

// N1: el schema ANUNCIA la escalera. Sin esto el agente nunca la usa: no la ve.
func TestAutonomiaAnunciadaEnElSchema(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	var work *Tool
	for i := range s.tools {
		if s.tools[i].Name == "musubi_work" {
			work = &s.tools[i].Tool
			break
		}
	}
	if work == nil {
		t.Fatal("no existe la tool musubi_work")
	}
	props := work.InputSchema.Properties
	for _, campo := range []string{"effect", "reviewer"} {
		if _, ok := props[campo]; !ok {
			t.Errorf("el schema no expone %q: el cliente no puede declararlo", campo)
		}
	}
	if !strings.Contains(props["action"].Description, "approve") {
		t.Error("la lista de actions no menciona approve: nadie va a firmar nada")
	}
	if u := props["units"].Description; !strings.Contains(u, "autonomy") {
		t.Errorf("plan no dice que las unidades llevan autonomy: %q", u)
	}
	// La descripción de la tool tiene que decir los tres niveles Y el default, que es lo que
	// evita que alguien crea que omitirlo frena algo.
	d := work.Description
	for _, must := range []string{"L1", "L2", "L3", "approve", "effect"} {
		if !strings.Contains(d, must) {
			t.Errorf("la descripción de musubi_work no menciona %q", must)
		}
	}
}

// N2: el nivel declarado en `plan` LLEGA a la unidad. Si se perdiera en el camino, todo el resto
// del sistema funcionaría igual de bien... sobre L3, sin que nadie se entere.
func TestAutonomiaViajaDesdePlan(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	res, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "plan", "batch": "esc",
		"units": []map[string]string{
			{"title": "mirar", "spec": "no toques", "autonomy": "L1"},
			{"title": "arreglar", "spec": "con revisión", "autonomy": "L2"},
			{"title": "rutina", "spec": "solo"},
		},
	})
	if e != nil {
		t.Fatalf("plan: %+v", e)
	}
	var b memory.WorkBatch
	if err := json.Unmarshal([]byte(textOf(t, res)), &b); err != nil {
		t.Fatalf("plan no devolvió WorkBatch: %v", err)
	}
	quiero := []string{memory.AutonomyReport, memory.AutonomyAssisted, memory.AutonomyUnattended}
	for i, q := range quiero {
		if b.Units[i].Autonomy != q {
			t.Errorf("unidad %d: autonomy=%q, esperaba %q", i, b.Units[i].Autonomy, q)
		}
	}
}

// N3: un nivel inválido se rechaza en el plan, con un error de parámetros y no un 500.
func TestAutonomiaInvalidaRechazadaPorLaTool(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	_, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "plan", "batch": "malo",
		"units":  []map[string]string{{"title": "x", "autonomy": "L9"}},
	})
	if e == nil {
		t.Fatal("un autonomy inválido debe rechazarse al postear")
	}
	if e.Code != codeInvalidParams {
		t.Errorf("code=%d, esperaba invalid params (%d)", e.Code, codeInvalidParams)
	}
}

// N4: EL CICLO COMPLETO POR MCP. Una L2 se frena sin firma, la firma del propio dueño se rechaza,
// la de otro la destraba. Es el recorrido que va a hacer un agente de verdad.
func TestCicloL2PorLaTool(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "plan", "batch": "l2",
		"units":  []map[string]string{{"title": "arreglar", "spec": "con revisión", "autonomy": "L2"}},
	}); e != nil {
		t.Fatalf("plan: %+v", e)
	}
	res, e := call(t, s, "musubi_work", map[string]interface{}{"action": "claim", "batch": "l2", "agent": "obrero"})
	if e != nil {
		t.Fatalf("claim: %+v", e)
	}
	var c struct {
		Claimed bool            `json:"claimed"`
		Unit    memory.WorkUnit `json:"unit"`
	}
	if err := json.Unmarshal([]byte(textOf(t, res)), &c); err != nil || !c.Claimed {
		t.Fatalf("claim no entregó unidad: %v %s", err, textOf(t, res))
	}
	cerrar := func(efecto string) *RpcError {
		args := map[string]interface{}{
			"action": "complete", "id": c.Unit.ID, "agent": "obrero",
			"fencing_token": c.Unit.FencingToken, "result": "hecho",
		}
		if efecto != "" {
			args["effect"] = efecto
		}
		_, e := call(t, s, "musubi_work", args)
		return e
	}

	// Sin declarar efecto: se trata como apply y se frena. Es el caso del cliente viejo.
	if e := cerrar(""); e == nil {
		t.Fatal("una L2 sin firma no debe cerrar, ni siquiera omitiendo effect")
	} else if !strings.Contains(e.Message, "firma") {
		t.Errorf("el error debe explicar que falta la firma: %q", e.Message)
	}
	// El dueño no se firma solo.
	if _, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "approve", "id": c.Unit.ID, "reviewer": "obrero"}); e == nil {
		t.Error("el dueño no puede firmarse a sí mismo por la tool")
	}
	// Sin reviewer tampoco.
	if _, e := call(t, s, "musubi_work", map[string]interface{}{"action": "approve", "id": c.Unit.ID}); e == nil {
		t.Error("approve sin reviewer debe rechazarse")
	}
	// Con la firma de otro, cierra.
	if _, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "approve", "id": c.Unit.ID, "reviewer": "revisor"}); e != nil {
		t.Fatalf("approve: %+v", e)
	}
	if e := cerrar("apply"); e != nil {
		t.Fatalf("con firma vigente debe cerrar: %+v", e)
	}
	res, _ = call(t, s, "musubi_work", map[string]interface{}{"action": "status", "batch": "l2"})
	var b memory.WorkBatch
	json.Unmarshal([]byte(textOf(t, res)), &b)
	if b.Done != 1 {
		t.Errorf("la unidad debía quedar done: %+v", b)
	}
	if b.Units[0].ApprovedBy != "revisor" {
		t.Errorf("el status debe mostrar quién firmó, no sólo que se firmó: %+v", b.Units[0])
	}
}

// N5: una L1 reporta por la tool y el rechazo dice cómo salir. Un freno que no dice qué hacer
// deja al agente reintentando lo mismo hasta agotar el lease.
func TestL1ReportaPorLaTool(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "plan", "batch": "l1",
		"units":  []map[string]string{{"title": "auditar", "spec": "no toques nada", "autonomy": "L1"}},
	}); e != nil {
		t.Fatalf("plan: %+v", e)
	}
	res, _ := call(t, s, "musubi_work", map[string]interface{}{"action": "claim", "batch": "l1", "agent": "auditor"})
	var c struct {
		Unit memory.WorkUnit `json:"unit"`
	}
	json.Unmarshal([]byte(textOf(t, res)), &c)

	_, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "complete", "id": c.Unit.ID, "agent": "auditor",
		"fencing_token": c.Unit.FencingToken, "result": "lo arreglé", "effect": "apply"})
	if e == nil {
		t.Fatal("una L1 no puede cerrar declarando apply")
	}
	if !strings.Contains(e.Message, "effect=report") {
		t.Errorf("el rechazo debe indicar la salida (cerrar como report): %q", e.Message)
	}
	if _, e := call(t, s, "musubi_work", map[string]interface{}{
		"action": "complete", "id": c.Unit.ID, "agent": "auditor",
		"fencing_token": c.Unit.FencingToken, "result": "hallazgos: 3", "effect": "report"}); e != nil {
		t.Fatalf("una L1 debe poder cerrar reportando: %+v", e)
	}
}
