package memory

import (
	"testing"
)

// TestInitSchemaIdempotente verifica que initSchema se puede llamar dos veces
// sin error (idempotencia mediante CREATE TABLE IF NOT EXISTS).
func TestInitSchemaIdempotente(t *testing.T) {
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	// Llamar initSchema una segunda vez no debe fallar.
	if err := engine.initSchema(); err != nil {
		t.Errorf("segunda llamada a initSchema falló: %v", err)
	}
}

// TestSaveAndGetSkillDecisions verifica que SaveSkillDecision inserta filas y
// GetSkillDecisions las recupera.
func TestSaveAndGetSkillDecisions(t *testing.T) {
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	// Guardar dos decisiones.
	if err := engine.SaveSkillDecision("go-gin", "Go — Gin", "accepted", "alta utilidad"); err != nil {
		t.Fatalf("SaveSkillDecision accepted: %v", err)
	}
	if err := engine.SaveSkillDecision("go-echo", "Go — Echo", "rejected", "redundante con gin"); err != nil {
		t.Fatalf("SaveSkillDecision rejected: %v", err)
	}

	decisiones, err := engine.GetSkillDecisions()
	if err != nil {
		t.Fatalf("GetSkillDecisions error: %v", err)
	}
	if len(decisiones) != 2 {
		t.Fatalf("esperaba 2 decisiones, obtuve %d", len(decisiones))
	}

	// Verificar campos de la primera fila.
	d := decisiones[0]
	if d.SkillID != "go-gin" {
		t.Errorf("skill_id incorrecto: %q", d.SkillID)
	}
	if d.Name != "Go — Gin" {
		t.Errorf("name incorrecto: %q", d.Name)
	}
	if d.Decision != "accepted" {
		t.Errorf("decision incorrecto: %q", d.Decision)
	}
	if d.Reason != "alta utilidad" {
		t.Errorf("reason incorrecto: %q", d.Reason)
	}
	if d.CreatedAt == "" {
		t.Error("created_at no debe estar vacío")
	}
}

// TestSaveSkillDecisionAppendOnly verifica que varias llamadas con el mismo
// skill_id crean filas separadas (log de sólo-inserción).
func TestSaveSkillDecisionAppendOnly(t *testing.T) {
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	for i := 0; i < 3; i++ {
		if err := engine.SaveSkillDecision("go-gin", "Go — Gin", "accepted", "vuelta"); err != nil {
			t.Fatalf("vuelta %d SaveSkillDecision error: %v", i, err)
		}
	}

	decisiones, err := engine.GetSkillDecisions()
	if err != nil {
		t.Fatalf("GetSkillDecisions error: %v", err)
	}
	if len(decisiones) != 3 {
		t.Errorf("esperaba 3 filas (append-only), obtuve %d", len(decisiones))
	}
}

// TestSkillDecisionNoValidationAtDBLayer verifica que la capa de DB acepta
// cualquier valor de decision (la validación es responsabilidad del handler MCP).
func TestSkillDecisionNoValidationAtDBLayer(t *testing.T) {
	engine, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	// "maybe" no es válido en el handler, pero la DB debe aceptarlo sin error.
	if err := engine.SaveSkillDecision("x", "X", "maybe", ""); err != nil {
		t.Errorf("la capa DB no debe validar el campo decision, obtuvo error: %v", err)
	}
}
