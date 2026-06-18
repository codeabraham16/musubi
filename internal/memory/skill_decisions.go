package memory

import (
	"fmt"
)

// SkillDecision representa una fila de la tabla skill_decisions.
// Es un log append-only: cada llamada a SaveSkillDecision crea una fila nueva.
type SkillDecision struct {
	// ID es el identificador autoincremental de la fila.
	ID int
	// SkillID es el identificador slug de la skill (ej. "go-gin").
	SkillID string
	// Name es el nombre legible de la skill.
	Name string
	// Decision es la decisión tomada: "accepted" o "rejected".
	Decision string
	// Reason es la justificación opcional de la decisión.
	Reason string
	// CreatedAt es la marca temporal ISO-8601 del momento del registro.
	CreatedAt string
}

// SaveSkillDecision inserta una nueva fila en skill_decisions con los datos dados.
// La validación del valor de decision (accepted/rejected) es responsabilidad del
// handler MCP; la capa de base de datos acepta cualquier valor.
func (e *DbEngine) SaveSkillDecision(skillID, name, decision, reason string) error {
	const q = `INSERT INTO skill_decisions (skill_id, name, decision, reason)
	            VALUES (?, ?, ?, ?)`
	_, err := e.db.Exec(q, skillID, name, decision, reason)
	if err != nil {
		return fmt.Errorf("error al guardar decisión de skill: %w", err)
	}
	return nil
}

// GetSkillDecisions devuelve todas las filas de skill_decisions en orden de inserción.
func (e *DbEngine) GetSkillDecisions() ([]SkillDecision, error) {
	const q = `SELECT id, skill_id, name, decision, reason, created_at
	           FROM skill_decisions
	           ORDER BY id ASC`
	rows, err := e.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("error al consultar decisiones de skills: %w", err)
	}
	defer rows.Close()

	var decisiones []SkillDecision
	for rows.Next() {
		var d SkillDecision
		if err := rows.Scan(&d.ID, &d.SkillID, &d.Name, &d.Decision, &d.Reason, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("error al escanear decisión: %w", err)
		}
		decisiones = append(decisiones, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar decisiones de skills: %w", err)
	}
	return decisiones, nil
}
