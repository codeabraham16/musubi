package mcp

// methods_skillusage.go expone los CONTADORES DEL ARSENAL (§7 del track «Forja global») como tool.
//
// SUS DOS CONSUMIDORES, que es lo que el track exige antes de escribir nada: (1) quien mantiene el
// arsenal, que hasta acá no tenía con qué decidir cuál skill vale la pena y cuál sólo ocupa
// contexto, y (2) el cuerpo, que muestra el arsenal y puede marcar las candidatas ahí mismo.

import (
	"context"
	"encoding/json"

	"musubi/internal/memory"
)

// skillUsageReader lo implementa el motor real. Interfaz y no *memory.DbEngine por la misma razón
// que usageReader: si el backend de turno no sabe de contadores, la tool lo dice en vez de romper.
type skillUsageReader interface {
	SkillUsage(ctx context.Context) ([]memory.SkillUsageRow, error)
}

// toolSkillUsage responde qué pasó con cada skill del arsenal cuando se activó.
func (s *McpServer) toolSkillUsage(ctx context.Context, _ json.RawMessage) (interface{}, *RpcError) {
	lector, ok := s.engine.(skillUsageReader)
	if !ok {
		return textResult("El motor de memoria de esta instancia no expone los contadores del arsenal."), nil
	}
	// scopedCtx acota al proyecto de la CREDENCIAL (Track 19): sin esto, un miembro del cerebro
	// central vería qué skills usa otro equipo y con qué frecuencia.
	filas, err := lector.SkillUsage(s.scopedCtx(ctx))
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al leer los contadores del arsenal: %v", err)
	}

	// EL ARSENAL MANDA, NO LOS CONTADORES. Una skill instalada que nunca matcheó tiene que
	// aparecer con 0: «0 activaciones» es la lectura más accionable de las tres, y una fila
	// ausente es indistinguible de «no hay dato».
	arsenal, err := s.resolver.LoadSkills()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo leer el arsenal de skills: %v", err)
	}

	contadas := make(map[string]memory.SkillUsageRow, len(filas))
	for _, f := range filas {
		contadas[f.Skill] = f
	}

	instaladas := make(map[string]bool, len(arsenal))
	out := make([]memory.SkillUsageRow, 0, len(arsenal))
	for _, sk := range arsenal {
		instaladas[sk.Name] = true
		fila := contadas[sk.Name] // el cero de SkillUsageRow ya es «nunca pasó nada»
		fila.Skill = sk.Name
		memory.MarcarCandidata(&fila)
		out = append(out, fila)
	}

	// Contadores de skills que ya se desinstalaron: son actividad real, así que se listan POR
	// NOMBRE en vez de desaparecer. Decir «hay 3» sin decir cuáles no deja hacer nada con el dato.
	huerfanas := make([]memory.SkillUsageRow, 0)
	for _, f := range filas {
		if !instaladas[f.Skill] {
			huerfanas = append(huerfanas, f)
		}
	}

	if (len(out) > 0 || len(huerfanas) > 0) && s.ledger == nil {
		return textResult(memory.FormatSkillUsage(out, huerfanas) +
			"\nOJO: el ledger está APAGADO en esta instancia (usage_ledger.enabled: false), " +
			"así que estos contadores no se están actualizando.\n"), nil
	}
	return textResult(memory.FormatSkillUsage(out, huerfanas)), nil
}
