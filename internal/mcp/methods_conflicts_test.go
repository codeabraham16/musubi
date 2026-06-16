package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

func saveObs(t *testing.T, s *McpServer, id, topic, content string) (interface{}, *RpcError) {
	t.Helper()
	return call(t, s, "musubi_save_observation", map[string]interface{}{
		"id": id, "topic_key": topic, "content": content,
	})
}

func TestSaveAutoSupersedeSurfaceado(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := saveObs(t, s, "old", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema."); e != nil {
		t.Fatal(e)
	}
	res, e := saveObs(t, s, "new", "arch/db", "Usamos PostgreSQL como base de datos principal del sistema productivo.")
	if e != nil {
		t.Fatal(e)
	}
	txt := textOf(t, res)
	if !strings.Contains(txt, "id:") {
		t.Errorf("la respuesta debe seguir confirmando el id, obtuve: %q", txt)
	}
	if !strings.Contains(strings.ToLower(txt), "reemplaz") {
		t.Errorf("un supersede auto-resuelto debe surfacearse en la respuesta, obtuve: %q", txt)
	}
}

func TestSavePendingPideVeredicto(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := saveObs(t, s, "a", "arch/api", "El servicio de autenticación valida tokens JWT con expiración corta."); e != nil {
		t.Fatal(e)
	}
	res, e := saveObs(t, s, "b", "arch/api", "El servicio de autenticación valida tokens y además registra auditoría de accesos fallidos en disco.")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(textOf(t, res), "musubi_judge") {
		t.Errorf("una relación pendiente debe pedir veredicto vía musubi_judge, obtuve: %q", textOf(t, res))
	}
}

func TestConflictsYJudge(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := saveObs(t, s, "a", "arch/api", "El servicio de autenticación valida tokens JWT con expiración corta."); e != nil {
		t.Fatal(e)
	}
	if _, e := saveObs(t, s, "b", "arch/api", "El servicio de autenticación valida tokens y además registra auditoría de accesos fallidos en disco."); e != nil {
		t.Fatal(e)
	}

	// Listar conflictos pendientes.
	res, e := call(t, s, "musubi_conflicts", map[string]interface{}{})
	if e != nil {
		t.Fatalf("musubi_conflicts error: %+v", e)
	}
	var listed struct {
		Count     int                  `json:"count"`
		Relations []memory.ObsRelation `json:"relations"`
	}
	if err := json.Unmarshal([]byte(textOf(t, res)), &listed); err != nil {
		t.Fatalf("respuesta de conflicts no es JSON: %v\n%s", err, textOf(t, res))
	}
	if listed.Count < 1 || len(listed.Relations) < 1 {
		t.Fatalf("esperaba al menos 1 conflicto pendiente, obtuve %+v", listed)
	}
	relID := listed.Relations[0].ID

	// Emitir veredicto.
	if _, e := call(t, s, "musubi_judge", map[string]interface{}{
		"relation_id": relID, "relation": "not_conflict",
	}); e != nil {
		t.Fatalf("musubi_judge error: %+v", e)
	}

	// Ya no debe quedar pendiente.
	res2, _ := call(t, s, "musubi_conflicts", map[string]interface{}{})
	var after struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal([]byte(textOf(t, res2)), &after)
	if after.Count != 0 {
		t.Errorf("tras juzgar no debe quedar pendiente, quedan %d", after.Count)
	}
}

func TestJudgeInvalidParams(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_judge", map[string]interface{}{"relation": "related"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("musubi_judge sin relation_id debe dar invalid params, obtuve %+v", e)
	}
	if _, e := call(t, s, "musubi_judge", map[string]interface{}{"relation_id": "x", "relation": "inventada"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("musubi_judge con relación inválida debe dar invalid params, obtuve %+v", e)
	}
}
